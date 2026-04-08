# harness — the agentic loop and template registry

The harness is the heart of the sandbox. Its job is to take a brief,
select a `Template`, run an LLM-driven loop with controlled tool
access, and emit a fixed-schema verdict.

## Pieces

- `Template` — the per-objective config: system prompt, allowed
  tools, default budget, turn limit, required subject fields.
- `Registry` — name → template lookup. The MVP registers one:
  `healthcare.provider.verification`.
- `Engine` — owns the loop. `Run(ctx, job)` produces a `Verdict`.
- `Worker` — pops jobs from the queue and feeds them to the engine.

## Loop semantics

```
build initial transcript
for turn := 0; turn < template.TurnLimit; turn++ {
    if budget exhausted → return INSUFFICIENT_DATA(BUDGET_EXHAUSTED)
    reply := llm.Turn(transcript)
    switch reply.Kind {
    case TOOL_USE:
        if tool not in template.AllowedTools → return verdict with NO_MATCHING_SOURCES
        result := toolproxy.Call(...)
        append result to transcript
    case VERDICT:
        validate against schema; return
    case TEXT:
        append; loop
    }
}
return INSUFFICIENT_DATA(TURN_LIMIT_EXCEEDED)
```

The MVP loop is deliberately simple — no repetition detection, no
correction-prompt retries — but the seams for both are obvious. Add
them when the real LLM client lands.
