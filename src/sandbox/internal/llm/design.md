# llm — model client interface and stub

`Client` is the seam where the real LLM plugs in. It has one method:

```go
Turn(ctx, transcript) (Reply, error)
```

A `Reply` is one of three things:

- a tool call (the LLM wants the harness to execute a tool)
- a verdict (the LLM has reached its conclusion and emitted the
  fixed-schema JSON)
- a text-only response (only used by the stub for repetition tests)

`StubClient` ships with a deterministic two-turn script that exercises
the entire harness without an API key:

1. Turn 1 → emit a tool call against `provider_directory` for the
   subject in the transcript context.
2. Turn 2 → emit a verdict that matches the tool output.

The Anthropic-backed client lives behind the same interface and can
be wired up in the server binary when `ANTHROPIC_API_KEY` is set.
