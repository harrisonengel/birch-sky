# brief — agent brief schema

The `Brief` is the only thing a buyer can submit. The harness owns
runtime; the buyer owns intent. A brief carries:

- `Objective` — a registered objective ID (template selector)
- `Subject` — entity to investigate, validated per template
- `BudgetCents` — optional cap, falls back to template default
- `BuyerID` — opaque ID for the audit trail

Validation is structural only. Per-objective subject validation is the
template's responsibility (it knows which subject fields it needs).
