package harness

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/brief"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/jobstore"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/llm"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/toolproxy"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

func helloJob(t *testing.T, providerID string) *jobstore.Job {
	t.Helper()
	subject, _ := json.Marshal(map[string]string{"provider_id": providerID})
	return &jobstore.Job{
		ID: "job_test",
		Brief: brief.Brief{
			Objective: "healthcare.provider.verification",
			Subject:   subject,
			BuyerID:   "buyer_test",
		},
		State:     jobstore.StateQueued,
		CreatedAt: time.Now(),
	}
}

func newTestEngine() (*Engine, *audit.MemoryLogger) {
	logger := audit.NewMemoryLogger()
	proxy := toolproxy.NewProxy(logger, toolproxy.NewProviderDirectorySource())
	registry := NewRegistry()
	engine := NewEngine(registry, llm.NewStubClient(), proxy, logger)
	return engine, logger
}

func TestEngineHelloWorld(t *testing.T) {
	engine, logger := newTestEngine()
	job := helloJob(t, "hello-001")

	v, cost, err := engine.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v == nil {
		t.Fatal("nil verdict")
	}
	if v.Assessment != verdict.AssessmentConfirmed {
		t.Fatalf("expected CONFIRMED, got %v", v.Assessment)
	}
	if v.TurnsUsed != 2 {
		t.Fatalf("expected 2 turns, got %d", v.TurnsUsed)
	}
	if cost == 0 {
		t.Fatal("expected non-zero cost")
	}

	records := logger.ByJob("job_test")
	gotKinds := map[audit.EventKind]int{}
	for _, r := range records {
		gotKinds[r.Kind]++
	}
	if gotKinds[audit.EventLLMTurn] != 2 {
		t.Fatalf("expected 2 LLM turns audited, got %d", gotKinds[audit.EventLLMTurn])
	}
	if gotKinds[audit.EventToolCall] != 1 {
		t.Fatalf("expected 1 tool call audited, got %d", gotKinds[audit.EventToolCall])
	}
}

func TestEngineMissingSubjectField(t *testing.T) {
	engine, _ := newTestEngine()
	subject, _ := json.Marshal(map[string]string{"wrong_field": "x"})
	job := &jobstore.Job{
		ID: "job_missing",
		Brief: brief.Brief{
			Objective: "healthcare.provider.verification",
			Subject:   subject,
			BuyerID:   "buyer",
		},
	}
	v, _, err := engine.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.InsufficientReason != verdict.ReasonSubjectMissing {
		t.Fatalf("expected SUBJECT_MISSING_FIELDS, got %v", v.InsufficientReason)
	}
}

func TestEngineUnknownObjective(t *testing.T) {
	engine, _ := newTestEngine()
	subject, _ := json.Marshal(map[string]string{"provider_id": "x"})
	job := &jobstore.Job{
		ID: "job_unknown",
		Brief: brief.Brief{
			Objective: "doesnt.exist",
			Subject:   subject,
			BuyerID:   "buyer",
		},
	}
	if _, _, err := engine.Run(context.Background(), job); err == nil {
		t.Fatal("expected error for unknown objective")
	}
}

func TestEngineSubjectNotInDirectory(t *testing.T) {
	engine, _ := newTestEngine()
	job := helloJob(t, "ghost")
	v, _, err := engine.Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.Assessment != verdict.AssessmentInsufficientData {
		t.Fatalf("expected INSUFFICIENT_DATA, got %v", v.Assessment)
	}
}
