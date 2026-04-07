# Architecture Annotations

_Last updated: 2026-04-07. Maintained by architect agent + humans._

## Services

### web_frontend
- **QPS**: unknown
- **SLA**: unknown
- **Costs**: unknown
- **Notes**: Vite + vanilla JS demo. Entry at `src/main.js` wires `initScene`, `initChat`, `initDemoFlow`. Currently consumes only `src/mock-data.js` — no real network calls. Grep for `fetch(`/`axios`/`grpc` in `src/` returned zero hits as of bootstrap. Plays the role of "Web Front End" in the three-tier architecture from `docs/plans/mvp_architecture.md`.

### buyers_agent_platform
- **QPS**: unknown
- **SLA**: unknown
- **Costs**: unknown
- **Notes**: Planned only — no code in repo. Per `mvp_architecture.md`, will host LLM-based "forgetful" buyer agents in a sandbox with access to market platform tools (search + analyze). Spec for this service does not yet exist; only the market platform is specced.

### market_platform
- **QPS**: unknown
- **SLA**: unknown
- **Costs**: unknown
- **Notes**: Planned only — `src/market-platform/` directory does not exist yet. Detailed implementation spec at `docs/specs/market-platform/market-platform-service.md`. Stack: Go + chi v5, Postgres 16, OpenSearch 3.x (text + kNN vector), MinIO, Stripe test mode, AWS Bedrock Titan v2 embeddings (1024-dim), Claude API for `analyze_data`, REST + MCP/SSE on :8081. Hybrid search (BM25F + kNN, RRF fusion) is intentional from day one — search quality is the product.

## Cross-cutting Constraints

- **Three-tier boundary**: end users never call market_platform directly. Path is `web_frontend → buyers_agent_platform → market_platform`. The purchase flow is documented as an exception (frontend → market_platform direct) since payment doesn't need to traverse the agent.
- **buyer_id is opaque**: market_platform does not own buyer identity; it accepts an opaque `buyer_id` from the agent platform. Auth will be Cognito JWT in middleware later.
- **Two environments from day one**: Test and Prod, fully separated. Demos run on Test.

## Open Questions

- [ ] Confirm all low-confidence edges by checking call sites once Go code lands
- [ ] Add QPS, cost, and SLA data for each service
- [ ] Design spec for buyers_agent_platform (currently only narrative in `mvp_architecture.md`)
- [ ] Resolve purchase flow: does it really bypass the agent platform, or should it route through it for audit?
- [ ] Decide buy-order fulfillment design (currently TBD; demo uses cron + HTTP)

## Decision Log

- **2026-04-07** — Bootstrapped architecture layer. Current state: only `web_frontend` (Vite demo with mock data) physically exists. All backend nodes are inferred from `docs/plans/mvp_architecture.md` and `docs/specs/market-platform/` and marked `confidence: low`. They should be promoted to `high` as code lands.
