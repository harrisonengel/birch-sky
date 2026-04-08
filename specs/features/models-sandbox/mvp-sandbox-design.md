# Models Sandbox MVP — Design

The Models Sandbox is the trust enforcement layer of The Information
Exchange. Buyers submit a structured *agent brief* describing what they
want to know; the sandbox executes that brief on their behalf inside a
controlled harness with scoped data access, and returns a fixed-schema
verdict. Raw seller data never crosses the boundary — only categorical
assessments and recommended actions do.

This document is the implementation spec for the MVP scaffolding under
`src/sandbox/`. The longer architectural narrative lives in the design
brief that opened this work.

## Goals

1. Stand up a runnable, end-to-end skeleton of every component named in
   the architecture (API gateway, job store, queue, harness engine,
   tool proxy, audit log, verdict delivery).
2. Use the simplest possible "hello world" data and logic at every
   layer so the contracts are clear and the seams are obvious.
3. Make every extension point explicit: adding a new objective type, a
   new tool, a new data source, or a new LLM client must be a
   localized change.
4. Ship with a Cobra CLI (`sandbox-cli`) that exercises the API end to
   end so a human or another agent can drive a brief through the
   system.

## Non-goals (deliberately deferred)

- AWS deployment, ECS, Postgres, SQS, RDS — the MVP uses an in-process
  store and a Go channel queue. The interfaces are shaped so a
  Postgres + SQS implementation drops in cleanly later.
- Real LLM calls — the harness ships with a deterministic stub LLM
  that drives the agentic loop through one tool call and produces a
  verdict. The `llm.Client` interface is the seam where the real
  Anthropic client plugs in.
- Network isolation, IAM, secrets management — the MVP runs as a
  single binary on localhost.
- Authentication beyond a static API key.
- Multi-tenant cost metering — budget enforcement is implemented at
  the harness loop level but uses a single hard-coded ceiling.

## Repository layout

```
src/sandbox/
  design.md                   — top-level design pointer
  go.mod
  cmd/
    sandbox-server/main.go    — single-binary: API gateway + worker pool
    sandbox-cli/main.go       — Cobra CLI for end-to-end exercise
  internal/
    api/                      — HTTP handlers + router
    audit/                    — append-only audit logger
    brief/                    — agent brief schema and validation
    config/                   — env-driven config
    harness/                  — engine, template registry, loop, worker
    jobstore/                 — in-memory job store + channel queue
    llm/                      — LLM client interface + deterministic stub
    toolproxy/                — tool proxy + scoped data source adapters
    verdict/                  — fixed verdict schema and validation
```

Each subdirectory has its own `design.md` capturing the local
contract.

## Component contracts

### Brief and verdict (`internal/brief`, `internal/verdict`)

The brief is the *only* thing the buyer can submit. It carries:

- `objective` — string identifier mapped to a harness template
- `subject` — the entity to investigate (free-form JSON, validated per
  template)
- `budget_cents` — optional cap; falls back to template default
- `buyer_id` — opaque ID for audit trail

The verdict is the *only* thing the buyer can receive. It carries:

- `assessment` — categorical (`CONFIRMED`, `REFUTED`, `INSUFFICIENT_DATA`)
- `confidence` — categorical tier (`LOW`, `MEDIUM`, `HIGH`)
- `sources_consulted` — list of source IDs (no content)
- `agreement_summary` — count of sources that agreed/disagreed
- `recommended_action` — categorical hint
  (`NONE`, `PURCHASE_UPDATED_RECORD`, `RESUBMIT_WITH_MORE_BUDGET`)
- `insufficient_reason` — short categorical token, never free text
- `cost_cents`, `turns_used`

Free text fields are not permitted. The verdict extractor refuses any
field outside the schema.

### Job store and queue (`internal/jobstore`)

- `Store` is a goroutine-safe in-memory map keyed by job ID. Drop-in
  Postgres replacement implements the same interface.
- `Queue` is a Go channel of job IDs. Workers `Pop()`; the API
  `Push()`es. Drop-in SQS replacement implements the same interface.
- States: `QUEUED`, `RUNNING`, `COMPLETED`, `FAILED`, `TIMEOUT`.

### Tool proxy (`internal/toolproxy`)

- `Proxy` is the only call path the harness uses to touch data.
- Each `Source` is a small Go struct that returns hello-world
  records — for the MVP, a directory of one fake healthcare provider.
- The proxy enforces:
  - per-session tool authorization (template declares its allowed
    tools, the proxy rejects anything else)
  - per-call cost metering (every call costs a flat `cost_cents`)
  - audit logging (every call is appended to the audit log)

### Harness engine (`internal/harness`)

- `Template` describes one objective type: system prompt, allowed
  tools, output schema, default budget, turn limit.
- `Registry` is a map of objective name → template. The MVP registers
  one: `healthcare.provider.verification`.
- `Engine.Run(ctx, job)` executes a job:
  1. Look up the template by `job.Brief.Objective`.
  2. Construct the initial LLM prompt.
  3. Loop: call LLM → if tool call, route through `toolproxy` → feed
     response back → repeat.
  4. Stop when the LLM emits a verdict, the budget is exhausted, or
     the turn limit is hit.
  5. Validate the verdict against the schema; on failure, retry once
     with a correction prompt.
  6. Write the audit summary, return the verdict.
- `Worker.Start()` pops jobs from the queue and runs them. Concurrency
  is configurable via `--workers`.

### LLM client (`internal/llm`)

- `Client` interface: one method, `Turn(ctx, transcript) (Reply, error)`.
- `StubClient` implements a deterministic two-turn script:
  1. First turn: emit a tool call against the provider directory.
  2. Second turn: emit a verdict that matches the tool output.

  This is enough to exercise every code path end to end without
  needing an API key. The real Anthropic client implements the same
  interface.

### Audit log (`internal/audit`)

- `Logger` exposes `Job(...)`, `LLMTurn(...)`, `ToolCall(...)`,
  `Verdict(...)`.
- The MVP `MemoryLogger` keeps records in a slice for inspection. A
  Postgres-backed logger drops in later.

### API gateway (`internal/api`)

Endpoints (plain `net/http`):

- `POST /v1/briefs` — submit a brief, get back `{ "job_id": "..." }`.
- `GET  /v1/jobs/{id}` — return job status and (when complete) the
  verdict.
- `GET  /v1/objectives` — list registered templates and their required
  subject fields.
- `GET  /v1/audit/{job_id}` — return the audit trail for one job
  (helpful for the demo; would be locked down in prod).
- `GET  /healthz` — liveness.

Auth is a static API key in `X-API-Key`. The interface for swapping in
JWT verification later is a single middleware.

## CLI (`cmd/sandbox-cli`)

Cobra commands:

- `sandbox-cli objectives` — list registered objective types
- `sandbox-cli submit --objective <id> --subject <json> [--budget N]` —
  submit a brief, print the job ID
- `sandbox-cli get <job-id>` — print status / verdict
- `sandbox-cli wait <job-id>` — poll until terminal, print verdict
- `sandbox-cli demo` — run the canned hello-world flow:
  submit → wait → print verdict and audit summary

All commands talk to the API over HTTP. No direct in-process access.

## Demo flow

1. `sandbox-server` starts on `:8090` with one worker and the stub LLM.
2. `sandbox-cli demo` submits a healthcare provider verification brief
   for the canned subject `provider:hello-001`.
3. The harness selects the `healthcare.provider.verification` template,
   runs the stub LLM through one tool call against the provider
   directory adapter, and emits a verdict:
   `{ assessment: CONFIRMED, confidence: HIGH, sources_consulted: [provider_directory], recommended_action: NONE }`.
4. The CLI fetches the verdict and the audit trail for the job and
   prints both.

This is the minimum bar that proves every layer is wired correctly.

## Extension checklist

Adding a new objective type:

1. Add a `Template` to `harness.NewRegistry`.
2. Add any new tools to `toolproxy.NewProxy` and authorize them in the
   template's `AllowedTools`.
3. (If the LLM stub is being used for tests) extend `StubClient` with a
   script for the new objective.

Adding a new data source:

1. Create a new `Source` implementation under `internal/toolproxy`.
2. Register it in `toolproxy.NewProxy`.
3. Authorize it in any template that should be allowed to call it.

Swapping the in-memory store/queue for AWS:

1. Implement `jobstore.Store` against Postgres (`*sql.DB`).
2. Implement `jobstore.Queue` against SQS.
3. Wire them in `cmd/sandbox-server/main.go` behind a config flag.

Swapping the stub LLM for Anthropic:

1. Implement `llm.Client` using `github.com/anthropics/anthropic-sdk-go`
   (already vendored by the market-platform module).
2. Wire it in `cmd/sandbox-server/main.go` when `ANTHROPIC_API_KEY` is
   set.

## Testing

- `internal/brief`, `internal/verdict` — schema validation unit tests.
- `internal/jobstore` — store + queue concurrency tests.
- `internal/toolproxy` — authorization + audit unit tests.
- `internal/harness` — full engine run against the stub LLM, asserting
  the verdict schema and the audit trail.
- `internal/api` — handler tests with `httptest`.

The Go test suite runs with `go test ./...` from `src/sandbox/`. No
external services required.
