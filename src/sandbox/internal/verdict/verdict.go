// Package verdict defines the fixed schema returned to buyers.
//
// A verdict is the only thing that crosses the sandbox boundary on the
// way out. Every field is either categorical (a closed enum) or a
// count. There are no free-text fields — that is the structural
// guarantee that raw seller data cannot be smuggled out via prompt
// injection or LLM creativity.
package verdict

import (
	"errors"
	"fmt"
	"strings"
)

type Assessment string

const (
	AssessmentConfirmed        Assessment = "CONFIRMED"
	AssessmentRefuted          Assessment = "REFUTED"
	AssessmentInsufficientData Assessment = "INSUFFICIENT_DATA"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "LOW"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceHigh   Confidence = "HIGH"
)

type RecommendedAction string

const (
	ActionNone                  RecommendedAction = "NONE"
	ActionPurchaseUpdatedRecord RecommendedAction = "PURCHASE_UPDATED_RECORD"
	ActionResubmitMoreBudget    RecommendedAction = "RESUBMIT_WITH_MORE_BUDGET"
)

type InsufficientReason string

const (
	ReasonNone              InsufficientReason = ""
	ReasonBudgetExhausted   InsufficientReason = "BUDGET_EXHAUSTED"
	ReasonTurnLimitExceeded InsufficientReason = "TURN_LIMIT_EXCEEDED"
	ReasonNoMatchingSources InsufficientReason = "NO_MATCHING_SOURCES"
	ReasonSubjectMissing    InsufficientReason = "SUBJECT_MISSING_FIELDS"
)

// AgreementSummary captures how many sources agreed vs disagreed
// vs were unavailable. The exchange's trust scoring layer reads these
// counts; the buyer reads the categorical assessment.
type AgreementSummary struct {
	Agreed       int `json:"agreed"`
	Disagreed    int `json:"disagreed"`
	Unavailable  int `json:"unavailable"`
	TotalQueried int `json:"total_queried"`
}

// Verdict is the fixed-schema response a buyer receives.
type Verdict struct {
	Assessment         Assessment         `json:"assessment"`
	Confidence         Confidence         `json:"confidence"`
	SourcesConsulted   []string           `json:"sources_consulted"`
	Agreement          AgreementSummary   `json:"agreement_summary"`
	RecommendedAction  RecommendedAction  `json:"recommended_action"`
	InsufficientReason InsufficientReason `json:"insufficient_reason,omitempty"`
	CostCents          int                `json:"cost_cents"`
	TurnsUsed          int                `json:"turns_used"`
}

// Validate checks the verdict against the closed enums. Anything that
// fails this check is rejected by the harness extractor.
func (v *Verdict) Validate() error {
	switch v.Assessment {
	case AssessmentConfirmed, AssessmentRefuted, AssessmentInsufficientData:
	default:
		return fmt.Errorf("verdict: invalid assessment %q", v.Assessment)
	}
	switch v.Confidence {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return fmt.Errorf("verdict: invalid confidence %q", v.Confidence)
	}
	switch v.RecommendedAction {
	case ActionNone, ActionPurchaseUpdatedRecord, ActionResubmitMoreBudget:
	default:
		return fmt.Errorf("verdict: invalid recommended_action %q", v.RecommendedAction)
	}
	switch v.InsufficientReason {
	case ReasonNone, ReasonBudgetExhausted, ReasonTurnLimitExceeded, ReasonNoMatchingSources, ReasonSubjectMissing:
	default:
		return fmt.Errorf("verdict: invalid insufficient_reason %q", v.InsufficientReason)
	}
	if v.Assessment == AssessmentInsufficientData && v.InsufficientReason == ReasonNone {
		return errors.New("verdict: INSUFFICIENT_DATA requires an insufficient_reason")
	}
	if v.Agreement.TotalQueried < 0 {
		return errors.New("verdict: agreement.total_queried must be non-negative")
	}
	for _, s := range v.SourcesConsulted {
		if strings.TrimSpace(s) == "" {
			return errors.New("verdict: sources_consulted contains an empty entry")
		}
	}
	return nil
}
