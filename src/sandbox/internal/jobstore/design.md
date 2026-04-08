# jobstore — persistent job state and dispatch queue

`Store` is the per-job source of truth: brief, status, verdict,
counters, timestamps. The MVP uses a goroutine-safe in-memory map. A
Postgres-backed implementation behind the same interface drops in
later.

`Queue` is the dispatch handoff between the API gateway and the
harness workers. The MVP uses a buffered Go channel. An SQS-backed
implementation behind the same interface drops in later.

States:

```
QUEUED → RUNNING → COMPLETED
                 ↘ FAILED
                 ↘ TIMEOUT
```
