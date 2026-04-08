package llm

import (
	"context"
	"testing"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

func TestStubFirstTurnEmitsToolCall(t *testing.T) {
	stub := NewStubClient()
	transcript := []Message{
		{Role: RoleSystem, Content: "you are a verifier"},
		{Role: RoleUser, Content: "subject:hello-001"},
	}
	reply, err := stub.Turn(context.Background(), transcript)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if reply.Kind != ReplyToolUse {
		t.Fatalf("expected ReplyToolUse, got %v", reply.Kind)
	}
	if reply.ToolID != "provider_directory" {
		t.Fatalf("unexpected tool id %q", reply.ToolID)
	}
	if reply.ToolParams["subject_id"] != "hello-001" {
		t.Fatalf("unexpected subject_id: %v", reply.ToolParams)
	}
}

func TestStubSecondTurnEmitsVerdict(t *testing.T) {
	stub := NewStubClient()
	transcript := []Message{
		{Role: RoleUser, Content: "subject:hello-001"},
		{Role: RoleTool, ToolID: "provider_directory", ToolResult: []map[string]any{{
			"address_match": true,
			"phone_match":   true,
			"in_network":    true,
		}}},
	}
	reply, err := stub.Turn(context.Background(), transcript)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if reply.Kind != ReplyVerdict || reply.Verdict == nil {
		t.Fatalf("expected verdict reply, got %+v", reply)
	}
	if reply.Verdict.Assessment != verdict.AssessmentConfirmed {
		t.Fatalf("expected CONFIRMED, got %v", reply.Verdict.Assessment)
	}
}

func TestStubVerdictNoRecords(t *testing.T) {
	stub := NewStubClient()
	transcript := []Message{
		{Role: RoleUser, Content: "subject:missing"},
		{Role: RoleTool, ToolID: "provider_directory", ToolResult: []map[string]any{}},
	}
	reply, err := stub.Turn(context.Background(), transcript)
	if err != nil {
		t.Fatalf("Turn: %v", err)
	}
	if reply.Verdict.Assessment != verdict.AssessmentInsufficientData {
		t.Fatalf("expected INSUFFICIENT_DATA, got %v", reply.Verdict.Assessment)
	}
}
