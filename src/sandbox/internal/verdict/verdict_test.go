package verdict

import "testing"

func TestVerdictValidate(t *testing.T) {
	good := Verdict{
		Assessment:        AssessmentConfirmed,
		Confidence:        ConfidenceHigh,
		SourcesConsulted:  []string{"provider_directory"},
		Agreement:         AgreementSummary{Agreed: 1, TotalQueried: 1},
		RecommendedAction: ActionNone,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("expected good verdict to validate, got %v", err)
	}

	bad := []Verdict{
		{Assessment: "MAYBE", Confidence: ConfidenceHigh, RecommendedAction: ActionNone},
		{Assessment: AssessmentConfirmed, Confidence: "SORTA", RecommendedAction: ActionNone},
		{Assessment: AssessmentConfirmed, Confidence: ConfidenceHigh, RecommendedAction: "BUY"},
		{Assessment: AssessmentInsufficientData, Confidence: ConfidenceLow, RecommendedAction: ActionNone},
		{
			Assessment:        AssessmentConfirmed,
			Confidence:        ConfidenceHigh,
			RecommendedAction: ActionNone,
			SourcesConsulted:  []string{""},
		},
	}
	for i, v := range bad {
		if err := v.Validate(); err == nil {
			t.Fatalf("case %d: expected error, got nil", i)
		}
	}
}
