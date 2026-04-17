# Local Compose Demo

## Goal

Replace the current scripted-overlay demo with a single `docker compose up` that
boots every runtime service and lets the frontend exercise the real backend
end-to-end over pre-loaded data. No more `MOCK_RESULTS` fallback, no more
`(Using demo data — backend not connected)` path.

## Scope

In scope (runtime services):

- `src/market-platform/` — Go marketplace API (postgres-backed, OpenSearch for
  search, MinIO for object storage)
- `src/harness/` — Python buyer-agent sandbox runner (moved from `harness/`)
- `src/agent_prepper/` — Python pre-entry clarification service (moved from `agent_prepper/`)
- `src/` frontend (Vite dev server)
- Infra: postgres, opensearch, minio (already in
  `src/market-platform/docker-compose.yml`)

Out of scope: `orchestrator/`, `arch/`, `fin/`, `scripts/`.

## Decisions (from conversation)

1. **Auth:** frontend dev-mode bypass. A new `VITE_AUTH_MODE=local` flag short-
   circuits the Cognito gate in `src/main.js` and assigns a stub user so
   `api-client.js` still has a `buyerID`. No local Cognito mock.
2. **Seed data:** compose pre-loads real data. A `seed` service (oneshot
   container) runs after market-platform is healthy and uses the existing
   cobra CLI under `src/market-platform/cmd/` to create sellers, listings,
   and push them through the OpenSearch embedding pipeline. Data lives in
   `src/market-platform/testdata/seed/` as yaml/json fixtures.
3. **Strip from `demo-flow.js`:**
   - `MOCK_RESULTS` fallback branch in `enterBackend` / `runFlow`
   - `backendAvailable` state and the "(Using demo data …)" message
   - `#demo-mode` toggle UI and the `forcedPath` / `no-results` forced path
   - Purchase/buy-request catch-block fallback overlays (real calls only;
     surface errors honestly)
4. **Keep:** scene animation (`agentRunTo` / `buildingWork` / `agentRunBack`)
   as pure visual layer timed to the real request lifecycle. User will iterate.

## Work plan

### Phase 1 — Move Python services under `src/`

- `git mv harness src/harness`
- `git mv agent_prepper src/agent_prepper`
- Update any import paths / Dockerfile references (both Dockerfiles are
  self-contained; check orchestrator/ and scripts/ for path references but
  do NOT modify orchestrator behavior — just leave a note if imports break,
  since orchestrator is out of scope)
- Update each service's Dockerfile build context expectations

### Phase 2 — Top-level `docker-compose.yml`

New file at repo root. Composes:

- `postgres`, `opensearch`, `minio` (lifted from
  `src/market-platform/docker-compose.yml`)
- `market-platform` (build: `src/market-platform`)
- `harness` (build: `src/harness`)
- `agent-prepper` (build: `src/agent_prepper`)
- `frontend` (new Dockerfile: `src/Dockerfile.frontend` or root `Dockerfile.frontend` — runs `npm run dev -- --host 0.0.0.0`)
- `seed` (build: `src/market-platform`, depends_on market-platform healthy,
  runs the seed CLI command, exits 0)

All services on a shared network. Ports exposed:
- 5173 frontend
- 8080 market-platform
- 8081 harness
- 8082 agent-prepper
- 9200 opensearch, 5432 postgres, 9000/9001 minio

### Phase 3 — Frontend dev-mode bypass

- `src/main.js`: read `import.meta.env.VITE_AUTH_MODE`. If `== 'local'`, skip
  `ensureSession` / `showAuthModal` and set a stub buyer identity via a new
  `src/auth/local-bypass.js` helper that sessionStorage-seeds a buyer id.
- `.env.local` gets `VITE_AUTH_MODE=local` for compose runs. Real deploys
  leave it unset and Cognito still works.
- Remove `?e2e=1` URL-param bypass OR keep it — keep, since Playwright still
  needs it and it's orthogonal.

### Phase 4 — Strip scripted fallbacks from `demo-flow.js`

- Delete the `MOCK_RESULTS` import and fallback branch.
- Delete `backendAvailable` state and the demo-data message.
- Delete the `#demo-toggle` DOM injection and `getPath()` — `runFlow` always
  takes the real-backend path.
- In `onBuyClick` / `onBuyRequestSubmit`, remove the silent-success catch
  block; instead surface a real error overlay on failure.
- `mock-data.js` stays (still used by `generateBuyRequest` for draft text),
  but `MOCK_RESULTS` export is removed. Update `mock-data.test.js`.
- E2E tests that rely on `#demo-mode` toggle (`e2e/flows/`) need to migrate
  to hitting the real stack; note any that break and fix in the same PR.

### Phase 5 — Seed fixtures + CLI wiring

- `src/market-platform/testdata/seed/*.yaml` — sellers, listings covering
  enough query variety to make the happy-path and no-results-path both
  demonstrable against real data.
- `src/market-platform/cmd/seed` (new cobra subcommand) reads the fixtures
  and calls the same internal APIs that the HTTP handlers use, so the
  embedding pipeline runs for each listing.
- Document in repo `README.md`: `docker compose up` → wait for `seed` to
  exit clean → open `http://localhost:5173`.

## Open risks

- **OpenSearch boot time.** ML model deploy can take minutes on first run;
  seed service must wait on it, not just on market-platform. Add a poll.
- **Harness networking.** Harness spawns buyer agents that need network
  access to market-platform; verify the compose network resolves service
  names inside the harness sandbox.
- **Cognito env vars.** `.env.local` currently has real Cognito values. With
  `VITE_AUTH_MODE=local` those are ignored at runtime, but we should avoid
  loading them into the frontend container's build if possible.

## Non-goals

- Production-ready compose (no TLS, no secrets management, no resource
  limits tuned).
- Replacing Cognito long-term.
- Touching `orchestrator/`, CI, or deployment tooling.
