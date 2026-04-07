# Market Platform Service — Design

This is the top-level design document. The full spec lives at `docs/specs/market-platform/market-platform-service.md`.

## Architecture

Three-tier: Web Front End → Buyers Agent Platform → **Market Platform** (this service).

- **HTTP API** on `:8080` — REST/JSON for listings, search, purchases, buy orders
- **MCP Server** on `:8081` — SSE transport, 3 tools for buyer agents
- **PostgreSQL 16** — relational data (sellers, listings, transactions, ownership, buy orders)
- **OpenSearch 3.x** — hybrid text + vector search with BM25F and kNN
- **MinIO** — S3-compatible object storage for data files
- **Stripe** — payment processing (test mode for MVP)
- **AWS Bedrock Titan v2** — 1024-dim embeddings (optional; local fallback for dev)

## Key Decisions

- `buyer_id` is an opaque string from the agent platform. No auth validation in MVP.
- Hybrid search from day one — search quality is the product.
- Constructor injection for all dependencies, no DI framework.
- Migrations embedded in binary via `embed.FS`.
- Every directory has a `design.md` capturing local decisions.
