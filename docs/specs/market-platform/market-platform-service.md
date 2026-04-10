# Market Platform Service — Implementation Spec

## Context

The Information Exchange needs its core backend: the Market Platform. This service stores seller data, enables search, processes purchases, and exposes tools for buyer agents. No Go code exists yet — this is greenfield under `src/market-platform/`. The goal is a locally-testable service where basic flows (list data, search, purchase, buy orders) can be exercised via curl and MCP client.

**Why hybrid search from day one**: The buyer agent's ability to find precisely the right data is the core value proposition. Text search alone misses semantic matches (e.g. "consumer electronics pricing" should match "retail gadget cost analysis"). Vector search alone loses precision on exact terms and structured filters. The combination — text via BM25F (`combined_fields`) plus kNN vector search, fused with Reciprocal Rank Fusion — gives agents the best chance of finding relevant data. OpenSearch 3.x supports all of this natively. We accept the extra complexity of an embedding pipeline because search quality is not a "nice to have" — it's the product.

### Key architectural constraints from existing docs

- **Three-tier architecture**: `[Web Front End] → [Buyers Agent Platform] → [Market Platform]`. The market platform is accessed by the agent platform, not directly by end users.
- **`buyer_id` is an opaque string** — buyers live on the agent platform. Auth will be Cognito JWT validated in middleware, but for this MVP the middleware accepts a `buyer_id` header/field without validation.
- **Data can be files or structured** — the `mvp_architecture.md` draft schema has both SQL and document storage paths. For MVP, we store files in MinIO and metadata in Postgres.
- **Trust engine is deferred** — see [`specs/features/trust/mvp-trust-decision.md`](../../../specs/features/trust/mvp-trust-decision.md) for the full decision. In summary: sellers are manually onboarded (no self-serve, no automated scoring), every transaction captures rich metadata to seed the future trust engine, and buyer refutations trigger an operator email rather than automated processing.

## Tech Stack

| Concern | Choice |
|---------|--------|
| Language | Go |
| Router | chi v5 |
| Database | PostgreSQL 16 |
| Search | OpenSearch 3.x (3.5 preferred, 3.3 minimum; text + kNN vector) |
| Embeddings | AWS Bedrock Titan Embeddings v2:0 (1024-dim) |
| Object storage | MinIO (S3-compatible) |
| Payments | Stripe test mode |
| LLM | Claude API (`anthropic-sdk-go`) for `analyze_data` |
| Local infra | Docker Compose |
| API style | REST/JSON + MCP server (SSE) |
| Service root | `src/market-platform/` |

## Directory Structure

Every directory has a `design.md` that captures decisions, principles, and constraints for that area. The top-level `design.md` is this spec. These are living documents — code changes that affect design assumptions should update the relevant `design.md`.

```
src/market-platform/
├── design.md                       # This spec — top-level service design
├── cmd/server/main.go              # Entry point, wiring
├── internal/
│   ├── design.md                   # Directory guide: what lives in internal and why
│   ├── config/config.go            # Env-based config
│   ├── domain/                     # Pure types, no deps
│   │   ├── design.md               # Type design principles, key decisions
│   │   ├── listing.go
│   │   ├── transaction.go
│   │   └── buyorder.go
│   ├── storage/
│   │   ├── design.md               # Storage layer decisions (why sqlx, interface contracts)
│   │   ├── postgres/
│   │   │   ├── design.md           # Postgres-specific patterns (migrations, scanning, txns)
│   │   │   ├── db.go               # Connection + migrate
│   │   │   ├── listing_repo.go
│   │   │   ├── seller_repo.go
│   │   │   ├── transaction_repo.go
│   │   │   ├── buyorder_repo.go
│   │   │   └── migrations/
│   │   │       ├── design.md       # Migration conventions + ordering rules
│   │   │       ├── 001_initial.up.sql
│   │   │       └── 001_initial.down.sql
│   │   └── objectstore/minio.go
│   ├── search/
│   │   ├── design.md               # Hybrid search architecture, embedding pipeline, scoring
│   │   ├── opensearch.go           # Client + query construction
│   │   ├── embedder.go             # Bedrock Titan embedding client
│   │   ├── indexer.go              # Content extraction + indexing (text + vectors)
│   │   └── mapping.go             # Index schema (text fields + knn_vector)
│   ├── payments/stripe.go
│   ├── service/
│   │   ├── design.md               # Service layer patterns, orchestration rules
│   │   ├── listing_service.go
│   │   ├── search_service.go
│   │   ├── purchase_service.go
│   │   └── buyorder_service.go
│   ├── api/
│   │   ├── design.md               # API conventions, error format, pagination
│   │   ├── router.go
│   │   ├── middleware.go
│   │   ├── listing_handler.go
│   │   ├── search_handler.go
│   │   ├── purchase_handler.go
│   │   ├── buyorder_handler.go
│   │   ├── request.go
│   │   └── response.go
│   └── mcp/
│       ├── design.md               # MCP tool contracts, agent-facing behavior
│       ├── server.go               # SSE on :8081
│       └── tools.go                # 3 tools
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── go.mod
└── .env.example
```

## Database Schema (single migration: `001_initial`)

```sql
-- sellers
CREATE TABLE sellers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- listings
CREATE TABLE listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id UUID NOT NULL REFERENCES sellers(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    price_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    data_ref TEXT NOT NULL,           -- S3/MinIO object key
    data_format TEXT NOT NULL DEFAULT '',
    data_size_bytes BIGINT NOT NULL DEFAULT 0,
    tags JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',  -- active | inactive | deleted
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_listings_seller ON listings(seller_id);
CREATE INDEX idx_listings_status ON listings(status);
CREATE INDEX idx_listings_category ON listings(category);

-- transactions
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id TEXT NOT NULL,           -- opaque, from agent platform
    listing_id UUID NOT NULL REFERENCES listings(id),
    amount_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    stripe_payment_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending | completed | failed
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    -- trust engine seed data (see specs/features/trust/mvp-trust-decision.md)
    buyer_agent_query TEXT,           -- original query issued to the buyer agent
    agent_analysis_summary TEXT       -- what analyze_data returned to the agent
);

-- ownership
CREATE TABLE ownership (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id TEXT NOT NULL,
    listing_id UUID NOT NULL REFERENCES listings(id),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    data_ref TEXT NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(buyer_id, listing_id)
);

-- buy_orders
CREATE TABLE buy_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id TEXT NOT NULL,
    query TEXT NOT NULL,
    criteria TEXT NOT NULL DEFAULT '{}',  -- JSON
    max_price_cents INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    category TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',  -- open | filled | cancelled | expired
    filled_by_listing_id UUID REFERENCES listings(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ
);
```

## API Endpoints (`/api/v1`)

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Liveness |
| `GET` | `/ready` | Readiness (PG, OpenSearch, MinIO) |
| **Listings** | | |
| `POST` | `/listings` | Create listing |
| `GET` | `/listings` | List (paginated, filterable) |
| `GET` | `/listings/{id}` | Get one |
| `PUT` | `/listings/{id}` | Update metadata |
| `DELETE` | `/listings/{id}` | Soft-delete |
| `POST` | `/listings/{id}/upload` | Upload data file (multipart) |
| **Search** | | |
| `POST` | `/search` | Hybrid text + vector search w/ filters |
| **Purchases** | | |
| `POST` | `/purchases` | Initiate (Stripe payment intent) |
| `POST` | `/purchases/{id}/confirm` | Confirm, record ownership |
| `GET` | `/purchases/{id}` | Status |
| `GET` | `/ownership` | List owned (`?buyer_id=X`) |
| `GET` | `/ownership/{listing_id}/download` | Presigned download URL |
| **Buy Orders** | | |
| `POST` | `/buy-orders` | Create |
| `GET` | `/buy-orders` | List |
| `GET` | `/buy-orders/{id}` | Get one |
| `POST` | `/buy-orders/{id}/fill` | Seller fills with listing |
| `DELETE` | `/buy-orders/{id}` | Cancel |

## MCP Tools (SSE on `:8081`)

Using `github.com/mark3labs/mcp-go`:

1. **`search_marketplace`** — natural language search with optional category/price filters. Delegates to search service.
2. **`get_listing`** — full public metadata for a listing by ID.
3. **`analyze_data`** — buyer agent asks questions about a dataset; the service loads the data from MinIO, sends it to Claude API with the questions, returns answers without revealing raw data. This is the Arrow's paradox resolver.

## Hybrid Search Design

### Why hybrid
Text search (BM25) is precise on exact terms and structured queries. Vector search (kNN) catches semantic similarity that keyword matching misses. For a data marketplace where buyer agents issue natural-language queries like "consumer electronics pricing trends in Southeast Asia", neither alone is sufficient. Combined, they cover both precision and recall.

### Trade-offs accepted
- **Embedding latency**: Each listing ingestion requires a Bedrock API call (~100-300ms). Acceptable for write-path; listings are ingested infrequently relative to searches.
- **Embedding cost**: Titan Embeddings v2 is ~$0.02/1M tokens. At MVP scale (hundreds of listings), cost is negligible.
- **Local dev**: For local development, the embedder interface has a fallback that generates deterministic pseudo-embeddings from text hashing, so Docker Compose doesn't require AWS credentials for basic testing. Real Bedrock embeddings are used when `AWS_REGION` + credentials are configured.

### OpenSearch Index: `listings`

**Text analysis**: Custom `listing_analyzer` — standard tokenizer → lowercase → stop words → snowball stemmer.

**Index mapping** (key fields):
```json
{
  "settings": {
    "index": { "knn": true },
    "analysis": { "analyzer": { "listing_analyzer": { ... } } }
  },
  "mappings": {
    "properties": {
      "title":        { "type": "text", "analyzer": "listing_analyzer" },
      "description":  { "type": "text", "analyzer": "listing_analyzer" },
      "tags":         { "type": "text", "analyzer": "listing_analyzer" },
      "content_text": { "type": "text", "analyzer": "listing_analyzer" },
      "embedding":    { "type": "knn_vector", "dimension": 1024, "method": {
                          "name": "hnsw", "space_type": "cosinesimil",
                          "engine": "nmslib" } },
      "category":     { "type": "keyword" },
      "status":       { "type": "keyword" },
      "price_cents":  { "type": "integer" },
      "listing_id":   { "type": "keyword" }
    }
  }
}
```

### Text search: `combined_fields`

Uses `combined_fields` query type (BM25F ranking across multiple fields treated as one combined field). This is the correct choice for our schema because the fields (`title`, `description`, `tags`, `content_text`) share the same analyzer and represent different aspects of the same document — BM25F handles the per-field length normalization properly rather than the ad-hoc boosting of `best_fields`.

```json
{
  "combined_fields": {
    "query": "<user query>",
    "fields": ["title^3", "description^2", "tags^2", "content_text"],
    "operator": "or"
  }
}
```

### Vector search: kNN

At query time, the user's query is embedded via the same Titan model. A kNN query retrieves the top-K nearest neighbors:

```json
{
  "knn": {
    "embedding": {
      "vector": ["<query_embedding>"],
      "k": 20
    }
  }
}
```

### Fusion: Reciprocal Rank Fusion (RRF)

OpenSearch 3.x may support hybrid search pipelines with built-in normalization, but for clarity and control we implement RRF application-side:

1. Issue text query and kNN query as two separate OpenSearch requests (in parallel).
2. For each result, compute RRF score: `score = Σ 1/(k + rank_i)` where `k=60` (standard constant) and `rank_i` is the result's rank in each list.
3. Return merged results sorted by RRF score.

This is simple, robust, and doesn't require tuning a linear combination weight. The `SearchEngine` interface hides this behind a single `Search()` call.

### Embedding pipeline

**`Embedder` interface** in `internal/search/embedder.go`:
```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float64, error)
}
```

Two implementations:
- `BedrockEmbedder`: calls AWS Bedrock `amazon.titan-embed-text-v2:0`, returns 1024-dim vector. Used when AWS credentials are available.
- `LocalEmbedder`: deterministic hash-based pseudo-embeddings for local dev/test. Produces consistent vectors so basic search works without AWS, but semantic similarity is not meaningful.

### Content extraction for indexing

`content_text` is extracted from uploaded files during ingestion:
- **CSV**: column headers + first 50 sample rows
- **JSON**: key paths + sample values (depth-limited)
- **Plain text**: full content, truncated at 50KB

The concatenation of `title + description + tags + content_text` is sent to the embedder to produce the listing's embedding vector. This is stored alongside the text fields in the index.

## Key Interfaces

All constructor-injected in `main.go`. No DI framework.

```go
type ObjectStore interface {
    Upload(ctx, bucket, key string, reader io.Reader, size int64, contentType string) error
    Download(ctx, bucket, key string) (io.ReadCloser, error)
    PresignedGetURL(ctx, bucket, key string, expiry time.Duration) (string, error)
    Delete(ctx, bucket, key string) error
}

type Embedder interface {
    Embed(ctx context.Context, text string) ([]float64, error)
}

type SearchEngine interface {
    EnsureIndex(ctx) error
    IndexListing(ctx, listing domain.Listing, contentText string, embedding []float64) error
    DeleteListing(ctx, listingID string) error
    TextSearch(ctx, query string, filters SearchFilters) ([]SearchResult, error)
    VectorSearch(ctx, embedding []float64, filters SearchFilters) ([]SearchResult, error)
}

type PaymentProcessor interface {
    CreatePaymentIntent(ctx, amountCents int, currency string) (clientSecret string, paymentID string, err error)
    ConfirmPayment(ctx, paymentID string) error
}

type DataAnalyzer interface {
    Analyze(ctx, dataRef string, questions []string) ([]string, error)
}
```

## Go Libraries (for `go.mod`)

```
github.com/go-chi/chi/v5
github.com/jmoiron/sqlx
github.com/golang-migrate/migrate/v4
github.com/opensearch-project/opensearch-go/v4
github.com/minio/minio-go/v7
github.com/stripe/stripe-go/v76
github.com/mark3labs/mcp-go
github.com/google/uuid
github.com/anthropics/anthropic-sdk-go
github.com/aws/aws-sdk-go-v2
github.com/aws/aws-sdk-go-v2/config
github.com/aws/aws-sdk-go-v2/service/bedrockruntime
```

---

## Build Plan — Agent Assignments

This section breaks the implementation into work streams assigned to agent personas. Each agent is loaded via its corresponding `/skill` (e.g., `/programmer`, `/quality-assurance`, `/security`). Work streams are designed for maximum parallelism.

### Team Roles

| Agent | Skill | Responsibility |
|-------|-------|---------------|
| **Programmer A** (Infra Lead) | `/programmer` | Skeleton, config, Docker, migrations, main.go wiring |
| **Programmer B** (Data Layer) | `/programmer` | Domain types, Postgres repos, MinIO object store, listing service |
| **Programmer C** (Search) | `/programmer` | OpenSearch client, embedder, indexer, search service, hybrid search |
| **Programmer D** (Commerce) | `/programmer` | Stripe integration, purchase flow, buy orders, MCP server |
| **QA Engineer** | `/quality-assurance` | Test infrastructure, integration tests, validation code — starts Phase 1, runs in parallel throughout |
| **Security Reviewer** | `/security` | Threat model, security review of each phase's output, attack surface analysis |

### Dependency Graph

```
                    ┌──────────────────────────────────────────────────────────┐
                    │            QA Engineer (parallel throughout)              │
                    │  Phase 1: test infra + helpers                           │
                    │  Phase 2: listing CRUD tests                             │
                    │  Phase 3: search integration tests                       │
                    │  Phase 4-5: purchase + buy order tests                   │
                    │  Phase 6: MCP tool tests                                 │
                    │  Phase 7: end-to-end smoke test suite                    │
                    └──────────────────────────────────────────────────────────┘

Phase 1                Phase 2              Phase 3           Phase 4-6         Phase 7
Programmer A ──────►  Programmer B ──────► Programmer C       Programmer D       All
(skeleton)            (data layer)         (search)           (commerce + MCP)   (smoke test)
                          │                    │                  │
                          └───────► Programmer D starts ◄─────────┘
                                    after Phase 2 done
                                    (needs repos + domain types)

Security Reviewer: reviews after Phase 1, Phase 3, Phase 4, Phase 6
```

### Phase 1: Skeleton
**Assigned to**: Programmer A

**Deliverables**:
- `go.mod` (`go mod init github.com/harrisonengel/birch-sky/src/market-platform`)
- `docker-compose.yml`: postgres:16-alpine, opensearchproject/opensearch:3.5.0 (single-node, security disabled, `knn.plugin.enabled: true`), minio/minio:latest
- `internal/config/config.go`: env vars for DB URL, OpenSearch URL, MinIO endpoint/creds, Stripe key, Claude API key, AWS region/credentials (Bedrock), server ports. Bedrock config optional — when absent, local embedder is used.
- `cmd/server/main.go`: parse config → connect PG → run migrations → create chi router → mount `/health` and `/ready` → start HTTP server on `:8080`
- `Makefile` targets: `docker-up-infra`, `docker-down`, `run`, `build`, `migrate`, `seed`, `test`
- `internal/storage/postgres/migrations/001_initial.up.sql` and `.down.sql` — all 5 tables
- `Dockerfile` (multi-stage Go build)
- `.env.example`
- `design.md` files: top-level, `internal/`, `internal/storage/`, `internal/storage/postgres/`, `internal/storage/postgres/migrations/`

**Verify**: `make docker-up-infra && make run` → `curl localhost:8080/health` returns `{"status":"ok"}`, `curl localhost:8080/ready` checks all 3 dependencies.

**Starts**: Immediately
**Blocks**: All other phases

---

### Phase 1 (parallel): QA Test Infrastructure
**Assigned to**: QA Engineer

**Deliverables** (starts in parallel with Phase 1, doesn't need Phase 1 complete):
- Test directory structure: `src/market-platform/tests/` with `integration/` and `helpers/` subdirs
- `tests/helpers/testdb.go`: Postgres test container setup (use `testcontainers-go`), creates isolated DB per test suite, runs migrations
- `tests/helpers/testopensearch.go`: OpenSearch test container, index setup/teardown
- `tests/helpers/testminio.go`: MinIO test container, bucket creation
- `tests/helpers/fixtures.go`: Factory functions for creating test sellers, listings, transactions with sensible defaults
- `tests/helpers/assertions.go`: Custom assertion helpers (e.g., `AssertJSONResponse`, `AssertListingEqual`)
- `tests/integration/health_test.go`: Health and readiness endpoint tests (validates Phase 1 output)
- Test matrix document: `tests/TEST_PLAN.md` — full test case matrix for all endpoints, organized by phase

**Verify**: `go test ./tests/...` runs and the health integration test passes against Docker containers.

**Starts**: Immediately (parallel with Programmer A)
**Blocks**: Nothing — other phases' tests build on this infrastructure

---

### Phase 2: Listings CRUD + MinIO
**Assigned to**: Programmer B

**Deliverables**:
- `internal/domain/listing.go`: `Seller`, `Listing` structs, `ListingStatus` and `Category` enums
- `internal/storage/postgres/seller_repo.go`: Create, GetByID, GetByEmail
- `internal/storage/postgres/listing_repo.go`: Create, GetByID, List (filters: status, category, seller_id; pagination via limit/offset), Update, SoftDelete
- `internal/storage/objectstore/minio.go`: `ObjectStore` interface + MinIO implementation — Upload, Download, PresignedGetURL, Delete
- `internal/service/listing_service.go`: orchestrates repo + object store. Upload extracts content text for later indexing, stores file in MinIO.
- `internal/api/router.go`: chi router setup, v1 route group
- `internal/api/middleware.go`: request logging, JSON content-type, recovery, CORS
- `internal/api/request.go`, `response.go`: shared JSON helpers
- `internal/api/listing_handler.go`: all listing REST handlers
- `design.md` files: `internal/domain/`, `internal/api/`, `internal/service/`

**Verify**: `curl -X POST localhost:8080/api/v1/listings -d '{"seller_id":"...","title":"Test","description":"Test data","price_cents":1000}'` → 201. `GET /api/v1/listings` returns it. File upload to MinIO works.

**Starts**: After Phase 1 complete
**Blocks**: Phases 3, 4, 5

---

### Phase 2 (parallel): QA Listing Tests
**Assigned to**: QA Engineer

**Deliverables** (starts as soon as Phase 2 domain types are merged):
- `tests/integration/listing_test.go`:
  - Create listing → 201, returns valid UUID
  - Create listing with missing required fields → 400 with clear error
  - Get listing by ID → 200 with full metadata
  - Get nonexistent listing → 404
  - List listings with no filters → paginated results
  - List listings filtered by category, status, seller_id
  - Pagination: page=1 vs page=2 return different results
  - Update listing metadata → 200, fields changed
  - Soft-delete listing → subsequent GET returns 404 (or status=deleted)
  - Upload file → stored in MinIO, data_ref populated
  - Upload to nonexistent listing → 404
  - Create listing with negative price_cents → 400
  - Create listing with empty title → 400
- `tests/integration/seller_test.go`:
  - Create seller → 201
  - Duplicate email → 409 or appropriate error
  - Get seller by ID → 200

**Verify**: `go test ./tests/integration/ -run TestListing` all pass against running service.

---

### Phase 3: Hybrid Search (OpenSearch + Embeddings)
**Assigned to**: Programmer C

**Deliverables**:
- `internal/search/mapping.go`: index mapping with text fields (custom `listing_analyzer`) + `knn_vector` (1024-dim, HNSW/cosinesimil/nmslib)
- `internal/search/embedder.go`: `Embedder` interface + `BedrockEmbedder` (Titan v2) + `LocalEmbedder` (hash-based fallback)
- `internal/search/opensearch.go`: `SearchEngine` interface + client — `EnsureIndex`, `IndexListing`, `DeleteListing`, `TextSearch` (BM25F `combined_fields`), `VectorSearch` (kNN)
- `internal/search/indexer.go`: content extraction (CSV/JSON/text) + embedding generation + calls `IndexListing`. Called from `ListingService` after upload.
- `internal/service/search_service.go`: orchestrates hybrid — issues `TextSearch` and `VectorSearch` in parallel, fuses with RRF (k=60), applies post-filters, returns merged results with scores and highlights
- `internal/api/search_handler.go`: `POST /search` — accepts `{"query":"...","category":"...","max_price_cents":N,"mode":"hybrid|text|vector"}` (default: hybrid)
- `internal/search/design.md`: documents hybrid architecture, `combined_fields` choice, RRF strategy, embedding model, local fallback
- Wire indexing into `ListingService.Upload` — after MinIO store, index into OpenSearch

**Verify**:
- Upload a listing with data file, then `POST /search {"query":"test data"}` returns results with RRF scores
- `POST /search {"query":"test data","mode":"text"}` returns BM25F-only results
- Semantic query `{"query":"retail gadget cost analysis"}` matches listing titled "Consumer Electronics Pricing" (demonstrates vector value — requires Bedrock or is verified in integration test with local embedder for basic functionality)

**Starts**: After Phase 2 complete
**Blocks**: Phase 6 (MCP search tool)

---

### Phase 3 (parallel): QA Search Tests
**Assigned to**: QA Engineer

**Deliverables**:
- `tests/integration/search_test.go`:
  - Empty index → search returns 0 results, not error
  - Upload 3 listings, search by exact title → top result matches
  - Search with category filter → only matching category returned
  - Search with max_price_cents filter → no results above threshold
  - Search with mode=text → results returned (no vector scores)
  - Search with mode=vector → results returned (no text scores)
  - Search with mode=hybrid → results from both pipelines, RRF-fused
  - Empty query string → 400
  - Pagination: per_page=1 returns 1 result with correct total count
  - Deleted/inactive listings excluded from search results
  - Upload CSV file → content_text extracted and searchable (search for column name)
  - Upload JSON file → key paths searchable
- `tests/integration/search_relevance_test.go`:
  - Seed 5 listings with distinct topics. Query for one topic. Assert the correct listing is rank 1.
  - Synonym-like query (e.g. "housing prices" for a "real estate pricing" listing) returns relevant result in top 3

---

### Phase 4: Purchase Flow
**Assigned to**: Programmer D

**Deliverables**:
- `internal/domain/transaction.go`: `Transaction`, `Ownership` structs, `TransactionStatus` enum
- `internal/payments/stripe.go`: `PaymentProcessor` interface + Stripe implementation — `CreatePaymentIntent`, `ConfirmPayment`
- `internal/storage/postgres/transaction_repo.go`: Create, GetByID, UpdateStatus
- Ownership repo (in transaction_repo.go or separate): Create, Exists, ListByBuyer
- `internal/service/purchase_service.go`: initiate (check already-owned, create intent + pending txn), confirm (verify Stripe, record ownership, generate presigned URL), status check
- `internal/api/purchase_handler.go`: all purchase + ownership REST handlers

**Verify**: `curl -X POST localhost:8080/api/v1/purchases -d '{"buyer_id":"test","listing_id":"..."}'` → Stripe client secret returned. Confirm endpoint records ownership.

**Starts**: After Phase 2 complete (needs repos + domain types)
**Blocks**: Phase 6 (MCP needs full service layer)

---

### Phase 5: Buy Orders
**Assigned to**: Programmer D (sequential after Phase 4)

**Deliverables**:
- `internal/domain/buyorder.go`: `BuyOrder` struct, `BuyOrderStatus` enum
- `internal/storage/postgres/buyorder_repo.go`: Create, GetByID, List (filterable by status, buyer_id, category), UpdateStatus, Fill
- `internal/service/buyorder_service.go`: create, list, get, fill (validates listing exists, updates status), cancel
- `internal/api/buyorder_handler.go`: all buy order REST handlers

**Verify**: Create a buy order, list it, fill with a listing ID, verify status changes to `filled`.

**Starts**: After Phase 4 (same programmer, sequential)
**Blocks**: Phase 6

---

### Phase 4-5 (parallel): QA Commerce Tests
**Assigned to**: QA Engineer

**Deliverables**:
- `tests/integration/purchase_test.go`:
  - Initiate purchase → 201, returns Stripe client secret + transaction ID
  - Initiate purchase for already-owned listing → returns already_owned: true, no charge
  - Confirm purchase → 200, ownership recorded
  - Confirm already-confirmed purchase → idempotent or 409
  - Get purchase status → pending/completed/failed
  - Download owned data → presigned URL returned
  - Download unowned data → 403
  - Purchase nonexistent listing → 404
  - List ownership → returns all buyer's owned listings
  - Purchase with invalid buyer_id format → still works (opaque string)
- `tests/integration/buyorder_test.go`:
  - Create buy order → 201
  - List buy orders filtered by status → correct filtering
  - Fill buy order with valid listing → status becomes filled
  - Fill already-filled buy order → 409
  - Cancel buy order → status becomes cancelled
  - Fill cancelled buy order → 409
  - Create buy order with max_price_cents = 0 → 400
  - List buy orders by buyer_id → only that buyer's orders

---

### Phase 6: MCP Server
**Assigned to**: Programmer D (sequential after Phase 5)

**Deliverables**:
- `internal/mcp/server.go`: SSE transport on `:8081` using `mcp-go`
- `internal/mcp/tools.go`: 3 tool definitions:
  - `search_marketplace`: params `{query, category?, max_price_cents?}` → delegates to `SearchService`
  - `get_listing`: params `{listing_id}` → delegates to `ListingRepo.GetByID`
  - `analyze_data`: params `{listing_id, questions[]}` → loads data from MinIO, calls Claude API, returns answers
- `DataAnalyzer` interface + implementation using `anthropic-sdk-go`
- `internal/mcp/design.md`
- MCP server started in `main.go` alongside HTTP server (separate goroutine)

**Verify**: Connect an MCP client to `http://localhost:8081/sse`, call `search_marketplace` with a query, get results.

**Starts**: After Phases 3, 4, 5 complete
**Blocks**: Phase 7

---

### Phase 6 (parallel): QA MCP Tests
**Assigned to**: QA Engineer

**Deliverables**:
- `tests/integration/mcp_test.go`:
  - Connect to MCP SSE endpoint → successful handshake
  - Call `search_marketplace` with valid query → returns listing results
  - Call `search_marketplace` with empty query → appropriate error
  - Call `get_listing` with valid ID → returns listing metadata
  - Call `get_listing` with invalid ID → error response
  - Call `analyze_data` with valid listing + questions → returns LLM-generated answers
  - Call `analyze_data` with nonexistent listing → error
  - Verify `analyze_data` response does NOT contain raw data (spot check for data leakage)

---

### Security Reviews

**Assigned to**: Security Reviewer

The security reviewer operates asynchronously, reviewing each phase's output before the next phase begins. Reviews are documented in `docs/specs/market-platform/security-reviews/`.

#### Review 1: Post-Phase 1 (Infrastructure)
**Focus**: Docker Compose security, default credentials, network exposure, migration safety
- Are default passwords acceptable for local dev? (Yes, but flag for production config)
- Is OpenSearch security disabled safely? (Acceptable for local-only)
- Migration: does `001_initial.down.sql` drop tables safely?

#### Review 2: Post-Phase 2 (Data Layer)
**Focus**: Input validation, SQL injection surface, file upload risks
- SQL parameterization in all repos (sqlx named params)
- File upload: size limits, content-type validation, filename sanitization
- MinIO bucket policy: no public access
- `buyer_id` as opaque string: what happens if someone sends a SQL injection as buyer_id?

#### Review 3: Post-Phase 3 (Search)
**Focus**: Information leakage via search, embedding pipeline security
- Do search results or highlights leak data beyond public metadata?
- Can a crafted search query cause OpenSearch injection?
- Embedding API: are credentials properly scoped? Bedrock IAM policy should be minimal.
- `content_text` extraction: can a malicious CSV/JSON cause code execution?

#### Review 4: Post-Phase 4 (Purchase Flow)
**Focus**: Payment integrity, ownership bypass, data access controls
- Can a buyer access data without completing payment? (presigned URL generation timing)
- Stripe webhook vs. confirm-endpoint: is there a TOCTOU race?
- Ownership uniqueness constraint: can it be bypassed with concurrent requests?
- Is the presigned URL lifetime appropriate? (should be short, ~5 minutes)

#### Review 5: Post-Phase 6 (MCP Server)
**Focus**: Agent sandbox boundary, data exfiltration via MCP, analyze_data leakage
- `analyze_data`: does the LLM prompt prevent raw data from being returned? (Prompt injection risk — a crafted dataset could manipulate the LLM into returning raw data)
- MCP SSE: is there authentication? (Not in MVP — flag as pre-production requirement)
- Can an MCP client enumerate all listings via repeated search calls? (By design, yes — public metadata is searchable. Raw data is the protected asset.)
- Tool input validation: oversized questions array, extremely long query strings

---

### Phase 7: Seed Data + End-to-End Smoke Test
**Assigned to**: Programmer A + QA Engineer (joint)

**Deliverables**:
- `cmd/seed/main.go`: creates 2 demo sellers, 5-6 listings with sample CSV/JSON files matching demo categories (consumer electronics pricing, real estate data, SaaS metrics, etc.)
- `make seed` target in Makefile
- `tests/integration/e2e_test.go` (QA Engineer):
  - Full flow: seed data → search → find listing → initiate purchase → confirm → download → verify data matches uploaded file
  - Buy order flow: search fails → create buy order → seller fills → buyer purchases filled listing
  - MCP flow: connect → search_marketplace → get_listing → analyze_data → verify answers are relevant

**Verify**: `make docker-up-infra && make run && make seed` then run `go test ./tests/integration/ -run TestE2E`.

**Starts**: After Phase 6 complete
**Blocks**: Nothing — this is the final validation

---

### Phase 8: Architect Skill Update
**Assigned to**: Programmer A

**File**: `.claude/skills/architect/SKILL.md`

Add to Priorities section:

```markdown
# Search Quality as Core Architecture

The buyer agent's ability to find precisely the right data is the product — not a feature.
Every search-related design decision must weigh:
- **Precision vs. recall**: Hybrid search (text BM25F + vector kNN with RRF fusion) is the baseline. Text search uses `combined_fields` for proper BM25F ranking across shared-analyzer fields. Never regress to `best_fields` or `multi_match` without justification.
- **Embedding pipeline cost/latency**: Embeddings add write-path latency and AWS cost. This is acceptable because search quality directly determines whether buyers find data to purchase. Budget for it.
- **Local dev parity**: Search must be testable locally without AWS credentials. Maintain the `Embedder` interface with a local fallback, but never let the fallback mask a production search regression.
- **Relevance tuning is ongoing**: The search pipeline (analyzer, boosts, fusion constant k, embedding model) is a living system. Design for observability — log query/result pairs to enable offline relevance evaluation.

# Platform Versioning Discipline

This is a greenfield project. Always use the latest stable version of every platform, library, and tool unless there is a specific documented reason not to.
- **Always specify the target version** when referencing any platform in specs, designs, and design.md files. Never write "OpenSearch" — write "OpenSearch 3.5". Never write "PostgreSQL" — write "PostgreSQL 16". This makes version assumptions explicit and auditable.
- **Re-evaluate versions at the start of each new feature or phase.** The tools that agents rely on (search engines, vector stores, embedding models, MCP implementations) have evolved rapidly. Features available in the latest release may eliminate custom code or unlock better approaches.
- **Document why** if you pin to an older version. "Latest wasn't available on AWS yet" is a valid reason; "we've always used this version" is not.
```

**Verify**: Load `/architect` and ask about search — it should reference `combined_fields`, RRF fusion, and embedding pipeline trade-offs.

---

## Execution Timeline (Parallel Swimlanes)

```
Time ──────────────────────────────────────────────────────────────►

Prog A:    [Phase 1: Skeleton]──────────────────────────────────────────────[Phase 7: Seed]──[Phase 8: Skill]
Prog B:                        [Phase 2: Data Layer]
Prog C:                                              [Phase 3: Search]
Prog D:                                              [Phase 4: Purchase][Phase 5: BuyOrders][Phase 6: MCP]
QA:        [Phase 1: Test Infra][Phase 2: Tests]     [Phase 3: Tests]  [Phase 4-5: Tests]  [Phase 6: Tests][Phase 7: E2E]
Security:              [Review 1]          [Review 2]          [Review 3]        [Review 4]         [Review 5]
```

**Critical path**: Phase 1 → Phase 2 → Phase 3 → Phase 6 → Phase 7

**Maximum parallelism after Phase 2**: Programmer C (search) and Programmer D (commerce) work simultaneously, QA writes tests for each as deliverables land.

**Security reviews are non-blocking by default** — findings are filed as issues. Critical findings (e.g., data leakage in search, payment bypass) block the next phase.

## design.md Convention

Every directory in the project gets a `design.md` that serves as the authoritative reference for that area. These are not documentation afterthoughts — they are living design documents that code changes are validated against.

**Rules**:
- The `design.md` at `src/market-platform/design.md` is this spec (the full service design).
- Subdirectory `design.md` files are lightweight: key decisions, interface contracts, patterns used, and constraints. Typically 20-60 lines.
- When code changes affect a design decision, the `design.md` must be updated in the same commit.
- `design.md` files are written during each phase alongside the code, not retroactively.

**Phase 1 creates**: top-level `design.md`, `internal/design.md`, `internal/storage/design.md`, `internal/storage/postgres/design.md`, `internal/storage/postgres/migrations/design.md`
**Phase 2 creates**: `internal/domain/design.md`, `internal/api/design.md`, `internal/service/design.md`
**Phase 3 creates**: `internal/search/design.md`
**Phase 6 creates**: `internal/mcp/design.md`
