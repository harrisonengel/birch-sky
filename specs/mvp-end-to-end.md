# MVP End-to-End Spec

## Goal

Wire all existing components into a single working system:
**Frontend chat UI -> Agent Harness HTTP service -> OpenSearch (real data) -> Market Platform API (purchases, buy orders) -> Postgres/MinIO**

Data is simple/seed but lives in real systems (Postgres, OpenSearch, MinIO).

## Changes

### 1. Stub Payment Processor
When no `STRIPE_SECRET_KEY` is configured, use a stub that auto-succeeds.
This lets the full purchase flow work in dev without Stripe credentials.

**File:** `internal/payments/stripe.go` — add `StubProcessor` implementing `PaymentProcessor`.

### 2. Agent Harness HTTP Service
Lift `harness/runner.py:execute()` into a FastAPI endpoint.

- `POST /api/run` — `{starting_context, user_input, max_turns}` → `{response}`
- Config from env vars (`ANTHROPIC_API_KEY`, `OPENSEARCH_URL`, `MODEL_NAME`)
- Add `fastapi` + `uvicorn` to dependencies
- New file: `harness/api.py`

### 3. Full-Stack Docker Compose
Extend `src/market-platform/docker-compose.yml` to include:
- `market-platform` server (Go binary)
- `agent-harness` service (Python FastAPI)
- All infra (Postgres, OpenSearch, MinIO) — already present

### 4. Seed Data CLI
Go CLI using cobra at `src/market-platform/cmd/iecli/`.
- `iecli seed` creates sellers, listings, uploads sample data via the HTTP API
- Simple datasets: pricing data CSV, satellite imagery metadata JSON, etc.

### 5. Frontend Wiring
Replace mock data with real API calls:
- Chat query → `POST /api/v1/search` on market-platform
- Result cards populated from search response
- Buy button → `POST /api/v1/purchases` + `POST /api/v1/purchases/{id}/confirm`
- Buy request form → `POST /api/v1/buy-orders`
- New module: `src/api-client.js` — thin wrapper around fetch to market-platform

### 6. Vite Proxy
Add proxy config in `vite.config.js` to forward `/api/` to market-platform `:8080`
and `/agent/` to harness `:8000`.
