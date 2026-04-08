# Models Sandbox Service — Design

This is the top-level design document. The full spec lives at
`specs/features/models-sandbox/mvp-sandbox-design.md`.

## What this is

The Models Sandbox is the trust enforcement layer of The Information
Exchange. Buyers submit a structured *agent brief* describing what
they want to know; the sandbox executes that brief on their behalf
inside a controlled harness with scoped data access, and returns a
fixed-schema verdict. Raw seller data never leaves the system.

## Architecture (MVP scaffold)

Single Go binary on `:8090`, containing:

- **API Gateway** (`internal/api`) — `net/http` REST endpoints for
  brief submission, job polling, objective discovery, audit lookup.
- **Job Store + Queue** (`internal/jobstore`) — in-memory map +
  channel queue. Interfaces shaped for a Postgres + SQS swap.
- **Harness Engine** (`internal/harness`) — template registry, worker
  pool, agentic loop, verdict extractor.
- **Tool Proxy** (`internal/toolproxy`) — the only path the harness
  uses to touch data. Enforces template-scoped tool authorization,
  cost metering, and audit logging.
- **LLM Client** (`internal/llm`) — interface plus a deterministic
  stub. The Anthropic client drops in behind the same interface.
- **Audit Logger** (`internal/audit`) — append-only record of every
  job, LLM turn, and tool call.

A Cobra CLI (`cmd/sandbox-cli`) exercises the API end to end and ships
a `demo` subcommand that runs the canned hello-world flow.

## Why so simple

The MVP brief asks for "ultra simple test/hello world data objects
and logic to fill in the gaps". The point of this scaffold is to make
every contract and seam in the architecture explicit so the real
implementations (Anthropic LLM, healthcare data sources, Postgres +
SQS, ECS deployment) are localized changes.

See `specs/features/models-sandbox/mvp-sandbox-design.md` for the
extension checklist.
