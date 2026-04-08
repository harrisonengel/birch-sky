# audit — append-only sandbox transaction log

The audit logger captures every event the trust scoring layer will
later need to evaluate sandbox runs:

- `Job` — job created / queued / failed / completed
- `LLMTurn` — one LLM round trip (turn number, token count, cost)
- `ToolCall` — one call through the tool proxy (tool ID, query shape,
  response size, cost, latency)
- `Verdict` — the final verdict, with cost totals

The MVP `MemoryLogger` keeps records in a slice for inspection. The
production version writes append-only rows to a Postgres audit
schema. Either way, the API surface is the same.

The logger never records raw seller data or raw query parameters by
default — only the *shape* of the access. A future dispute-resolution
flow can opt into encrypted content-level audit; that is out of scope
for the MVP.
