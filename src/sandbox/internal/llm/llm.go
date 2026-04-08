// Package llm defines the seam where the harness talks to a language
// model and ships a deterministic stub that exercises the seam end to
// end without any API key.
//
// The MVP harness drives an "agentic loop" by repeatedly asking the
// model what to do next. Each call returns a `Reply` that is either a
// tool call (the model wants the harness to execute a tool), a
// verdict (the model has reached its conclusion and emitted the
// fixed-schema JSON), or a text turn (used by the stub for negative
// tests). Anything outside of these three shapes is rejected by the
// harness loop controller.
package llm

import (
	"context"
	"errors"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

// Role tags messages in a transcript.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one entry in the transcript fed to the model.
type Message struct {
	Role    Role
	Content string
	// ToolID and ToolResult are populated for tool-result messages
	// that the harness feeds back into the model after running a
	// tool call.
	ToolID     string
	ToolResult any
}

// ReplyKind is the discriminator on `Reply`.
type ReplyKind string

const (
	ReplyText    ReplyKind = "TEXT"
	ReplyToolUse ReplyKind = "TOOL_USE"
	ReplyVerdict ReplyKind = "VERDICT"
)

// Reply is one round-trip's response from the model.
type Reply struct {
	Kind ReplyKind

	// Text is set for ReplyText.
	Text string

	// Tool use fields are set for ReplyToolUse.
	ToolID     string
	ToolParams map[string]any

	// Verdict is set for ReplyVerdict.
	Verdict *verdict.Verdict

	// Cost metering — every reply contributes to the per-job budget.
	CostCents int
}

// Client is the seam where a real model implementation plugs in. The
// Anthropic-backed client implements this interface; the stub below
// also implements it for offline development and testing.
type Client interface {
	Turn(ctx context.Context, transcript []Message) (Reply, error)
}

// StubClient is the deterministic LLM used by the MVP. It mirrors the
// "happy path" of a healthcare provider verification:
//
//   Turn 1: emit a tool call against provider_directory for the
//           subject ID in the transcript context.
//   Turn 2: emit a verdict matching the tool result the harness
//           feeds back.
//
// The harness asks for the subject ID via a single user message at
// the start of the transcript with content `subject:<id>`.
type StubClient struct{}

// NewStubClient builds the deterministic stub.
func NewStubClient() *StubClient { return &StubClient{} }

// ErrEmptyTranscript is returned if the harness somehow calls Turn
// with no messages — that should never happen in normal flow.
var ErrEmptyTranscript = errors.New("llm.StubClient: empty transcript")

// Turn implements the Client interface for the stub. The stub looks
// at the transcript to decide what state of the conversation it is
// in:
//
//   * If no tool result has been fed back yet → emit the tool call.
//   * If a tool result is present → emit the verdict that matches it.
func (s *StubClient) Turn(_ context.Context, transcript []Message) (Reply, error) {
	if len(transcript) == 0 {
		return Reply{}, ErrEmptyTranscript
	}

	subjectID := extractSubjectID(transcript)
	toolResult := lastToolResult(transcript)

	if toolResult == nil {
		// First turn: ask for the provider directory record.
		return Reply{
			Kind:      ReplyToolUse,
			ToolID:    "provider_directory",
			ToolParams: map[string]any{
				"subject_id": subjectID,
			},
			CostCents: 1,
		}, nil
	}

	// Second turn: turn the tool result into a categorical verdict.
	v := stubVerdictFromRecords(toolResult)
	return Reply{
		Kind:      ReplyVerdict,
		Verdict:   v,
		CostCents: 1,
	}, nil
}

func extractSubjectID(transcript []Message) string {
	for _, m := range transcript {
		if m.Role != RoleUser {
			continue
		}
		const prefix = "subject:"
		if len(m.Content) > len(prefix) && m.Content[:len(prefix)] == prefix {
			return m.Content[len(prefix):]
		}
	}
	return ""
}

func lastToolResult(transcript []Message) any {
	for i := len(transcript) - 1; i >= 0; i-- {
		if transcript[i].Role == RoleTool {
			return transcript[i].ToolResult
		}
	}
	return nil
}

// stubVerdictFromRecords converts the proxy result into the canned
// hello-world verdict. Real LLMs do this work themselves; the stub
// hard-codes the mapping so the test path is deterministic.
func stubVerdictFromRecords(toolResult any) *verdict.Verdict {
	records, _ := toolResult.([]map[string]any)
	if len(records) == 0 {
		return &verdict.Verdict{
			Assessment:         verdict.AssessmentInsufficientData,
			Confidence:         verdict.ConfidenceLow,
			SourcesConsulted:   []string{"provider_directory"},
			Agreement:          verdict.AgreementSummary{Unavailable: 1, TotalQueried: 1},
			RecommendedAction:  verdict.ActionResubmitMoreBudget,
			InsufficientReason: verdict.ReasonNoMatchingSources,
		}
	}

	rec := records[0]
	agreed := 0
	disagreed := 0
	for _, key := range []string{"address_match", "phone_match", "in_network"} {
		if v, ok := rec[key].(bool); ok {
			if v {
				agreed++
			} else {
				disagreed++
			}
		}
	}

	assessment := verdict.AssessmentConfirmed
	confidence := verdict.ConfidenceHigh
	if disagreed > 0 && agreed == 0 {
		assessment = verdict.AssessmentRefuted
	} else if disagreed > 0 {
		confidence = verdict.ConfidenceMedium
	}

	return &verdict.Verdict{
		Assessment:        assessment,
		Confidence:        confidence,
		SourcesConsulted:  []string{"provider_directory"},
		Agreement:         verdict.AgreementSummary{Agreed: agreed, Disagreed: disagreed, TotalQueried: agreed + disagreed},
		RecommendedAction: verdict.ActionNone,
	}
}
