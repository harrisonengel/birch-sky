package brief

import (
	"encoding/json"
	"testing"
)

func TestBriefValidate(t *testing.T) {
	cases := []struct {
		name    string
		brief   Brief
		wantErr bool
	}{
		{
			name: "valid brief",
			brief: Brief{
				Objective: "healthcare.provider.verification",
				Subject:   json.RawMessage(`{"provider_id":"hello-001"}`),
				BuyerID:   "buyer-1",
			},
		},
		{
			name: "missing objective",
			brief: Brief{
				Subject: json.RawMessage(`{"provider_id":"hello-001"}`),
				BuyerID: "buyer-1",
			},
			wantErr: true,
		},
		{
			name: "missing buyer_id",
			brief: Brief{
				Objective: "healthcare.provider.verification",
				Subject:   json.RawMessage(`{"provider_id":"hello-001"}`),
			},
			wantErr: true,
		},
		{
			name: "subject not an object",
			brief: Brief{
				Objective: "healthcare.provider.verification",
				Subject:   json.RawMessage(`"hello"`),
				BuyerID:   "buyer-1",
			},
			wantErr: true,
		},
		{
			name: "negative budget",
			brief: Brief{
				Objective:   "healthcare.provider.verification",
				Subject:     json.RawMessage(`{"provider_id":"x"}`),
				BuyerID:     "buyer-1",
				BudgetCents: -1,
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.brief.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestSubjectField(t *testing.T) {
	b := Brief{
		Subject: json.RawMessage(`{"provider_id":"hello-001","extra":42}`),
	}
	id, ok := b.SubjectField("provider_id")
	if !ok || id != "hello-001" {
		t.Fatalf("SubjectField provider_id = %q, ok = %v", id, ok)
	}
	if _, ok := b.SubjectField("missing"); ok {
		t.Fatal("expected missing field to return ok=false")
	}
	if _, ok := b.SubjectField("extra"); ok {
		t.Fatal("expected non-string field to return ok=false")
	}
}
