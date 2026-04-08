# verdict — fixed verdict schema

The `Verdict` is the only thing that crosses the sandbox boundary on
the way out. It is intentionally categorical and small. Free-text
fields are not permitted — every value is an enum or a count, never
raw seller data.

The verdict extractor in `harness` validates the LLM's output against
this schema. Anything outside the schema is rejected.
