// Package brief defines the agent brief schema submitted by buyers.
//
// A brief is the only thing a buyer can submit to the sandbox. It is
// deliberately small and structural: an objective ID that selects a
// harness template, a subject blob that the template knows how to
// interpret, an optional budget, and the opaque buyer ID used for
// audit tracking. Free-form prompts are not allowed — the harness
// owns the runtime and the LLM context.
package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Brief is the buyer-submitted intent. The sandbox owns everything
// downstream of this struct.
type Brief struct {
	Objective   string          `json:"objective"`
	Subject     json.RawMessage `json:"subject"`
	BudgetCents int             `json:"budget_cents,omitempty"`
	BuyerID     string          `json:"buyer_id"`
}

// Validate runs structural checks. Per-objective subject validation
// is performed later by the matching harness template.
func (b *Brief) Validate() error {
	if strings.TrimSpace(b.Objective) == "" {
		return errors.New("brief: objective is required")
	}
	if strings.TrimSpace(b.BuyerID) == "" {
		return errors.New("brief: buyer_id is required")
	}
	if len(b.Subject) == 0 {
		return errors.New("brief: subject is required")
	}
	// The subject must be valid JSON object — anything else cannot be
	// safely fed to a template.
	var probe map[string]any
	if err := json.Unmarshal(b.Subject, &probe); err != nil {
		return fmt.Errorf("brief: subject must be a JSON object: %w", err)
	}
	if b.BudgetCents < 0 {
		return errors.New("brief: budget_cents must be non-negative")
	}
	return nil
}

// SubjectField looks up a single string field from the subject blob.
// Templates use this to validate they have what they need.
func (b *Brief) SubjectField(name string) (string, bool) {
	var fields map[string]any
	if err := json.Unmarshal(b.Subject, &fields); err != nil {
		return "", false
	}
	v, ok := fields[name]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
