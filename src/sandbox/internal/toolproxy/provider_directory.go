package toolproxy

import (
	"context"
	"strings"
)

// ProviderDirectorySource is the hello-world data source for the MVP.
// It returns a single canned healthcare provider record for any
// subject ID it knows about, and zero records for everything else.
//
// The point of this source is *not* to be a realistic verifier — it
// is the smallest possible thing that lets the harness exercise the
// full proxy → source → result path. Replacing it with a real
// healthcare adapter is one struct + one line in NewProxy.
type ProviderDirectorySource struct {
	// directory is the canned data the source serves. Keyed by the
	// subject ID the harness queries with.
	directory map[string]map[string]any
}

// NewProviderDirectorySource builds the source with one hello-world
// record. Add more entries here (or load from disk) to grow the
// fixture set.
func NewProviderDirectorySource() *ProviderDirectorySource {
	return &ProviderDirectorySource{
		directory: map[string]map[string]any{
			"hello-001": {
				"provider_id":      "hello-001",
				"npi":              "1234567890",
				"address_match":    true,
				"phone_match":      true,
				"in_network":       true,
				"last_verified_at": "2026-04-01T00:00:00Z",
			},
		},
	}
}

func (s *ProviderDirectorySource) ID() string     { return "provider_directory" }
func (s *ProviderDirectorySource) CostCents() int { return 1 }

func (s *ProviderDirectorySource) Query(_ context.Context, subjectID string, _ map[string]any) ([]map[string]any, error) {
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return nil, nil
	}
	rec, ok := s.directory[subjectID]
	if !ok {
		return nil, nil
	}
	// Return a copy so the harness cannot mutate the underlying
	// fixture.
	out := make(map[string]any, len(rec))
	for k, v := range rec {
		out[k] = v
	}
	return []map[string]any{out}, nil
}
