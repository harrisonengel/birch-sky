package harness

import (
	"errors"
	"fmt"
	"strings"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/brief"
)

// Template captures everything the harness needs to know about one
// objective type. Adding a new objective is one new Template plus a
// matching set of registered tools — no engine code changes required.
type Template struct {
	// ID is the objective identifier the buyer references in the
	// brief, e.g. "healthcare.provider.verification".
	ID string

	// Description is a human-readable explanation surfaced via
	// `GET /v1/objectives` so buyers know what each template does.
	Description string

	// SystemPrompt is the heavily prescriptive instruction the LLM
	// receives at the top of every conversation.
	SystemPrompt string

	// RequiredSubjectFields are the keys the brief.Subject must carry
	// for this template to run. The validator returns
	// SUBJECT_MISSING_FIELDS if any are absent.
	RequiredSubjectFields []string

	// AllowedTools is the closed set of tool IDs this template may
	// invoke through the proxy.
	AllowedTools []string

	// DefaultBudgetCents is used when the buyer omits a budget.
	DefaultBudgetCents int

	// TurnLimit caps the number of LLM calls before the loop is
	// forcibly terminated.
	TurnLimit int

	// SubjectIDField names the subject field used as the canonical
	// scope for tool calls (e.g. "provider_id"). The proxy uses this
	// to enforce that every tool call targets the brief's subject and
	// nothing else.
	SubjectIDField string
}

// Validate checks that a brief is structurally compatible with this
// template before the engine runs anything expensive.
func (t *Template) Validate(b *brief.Brief) error {
	for _, field := range t.RequiredSubjectFields {
		if _, ok := b.SubjectField(field); !ok {
			return fmt.Errorf("template %s: missing required subject field %q", t.ID, field)
		}
	}
	return nil
}

// AllowedToolSet returns the AllowedTools list as a map for the
// proxy's authorization check.
func (t *Template) AllowedToolSet() map[string]bool {
	out := make(map[string]bool, len(t.AllowedTools))
	for _, id := range t.AllowedTools {
		out[id] = true
	}
	return out
}

// Registry is the lookup table from objective ID to template.
type Registry struct {
	templates map[string]*Template
}

// NewRegistry constructs a registry pre-populated with the MVP
// objective(s).
func NewRegistry() *Registry {
	r := &Registry{templates: map[string]*Template{}}
	r.Register(&Template{
		ID:                    "healthcare.provider.verification",
		Description:           "Verify a healthcare provider's address, phone, and network status against the exchange's directory adapters.",
		SystemPrompt:          strings.TrimSpace(healthcareProviderPrompt),
		RequiredSubjectFields: []string{"provider_id"},
		AllowedTools:          []string{"provider_directory"},
		DefaultBudgetCents:    50,
		TurnLimit:             10,
		SubjectIDField:        "provider_id",
	})
	return r
}

// Register adds (or replaces) a template in the registry.
func (r *Registry) Register(t *Template) {
	r.templates[t.ID] = t
}

// ErrUnknownObjective is returned by Lookup when no template matches.
var ErrUnknownObjective = errors.New("harness: unknown objective")

// Lookup returns the template for the given objective ID.
func (r *Registry) Lookup(id string) (*Template, error) {
	t, ok := r.templates[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownObjective, id)
	}
	return t, nil
}

// All returns every registered template. Used by `GET /v1/objectives`.
func (r *Registry) All() []*Template {
	out := make([]*Template, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out
}

const healthcareProviderPrompt = `
You are a verification agent operating inside a secure sandbox for The
Information Exchange. You are verifying healthcare provider records.

You MUST:
- Use only the tools provided to you.
- Investigate only the subject specified in the user message.
- Produce your final answer as a single JSON verdict that conforms to
  the verdict schema. Free text is not permitted.
- Never include raw seller data in your output. Only categorical
  assessments and counts.

If you cannot reach a verdict with the available tools and budget,
emit an INSUFFICIENT_DATA verdict with an appropriate insufficient_reason
token.
`
