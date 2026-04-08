# toolproxy — the only door between the harness and data

The harness never touches a data source directly. Every call goes
through `Proxy`, which:

1. Validates that the calling session (a brief + its template) is
   authorized to use the requested tool. The template declares its
   `AllowedTools` list; anything else is rejected.
2. Routes to the matching `Source` implementation.
3. Strips fields the template hasn't declared as relevant.
4. Meters cost (flat per-call for the MVP).
5. Appends an audit record via the `audit.Logger`.

Each data source is a Go struct that satisfies `Source`. The MVP ships
one source: `provider_directory`, which returns a single fake
healthcare provider record. Adding a new source is one struct + one
line of registration in `NewProxy`.

The proxy never returns raw seller data to the harness verbatim. The
`Result.Records` slice carries normalized field maps so the harness
can produce a categorical verdict without ever seeing — or being able
to copy — raw documents.
