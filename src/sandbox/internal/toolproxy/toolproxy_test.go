package toolproxy

import (
	"context"
	"errors"
	"testing"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
)

func TestProxyAuthorizes(t *testing.T) {
	logger := audit.NewMemoryLogger()
	proxy := NewProxy(logger, NewProviderDirectorySource())

	allowed := map[string]bool{"provider_directory": true}
	res, err := proxy.Call(context.Background(), Call{
		JobID:        "job_test",
		ToolID:       "provider_directory",
		SubjectID:    "hello-001",
		AllowedTools: allowed,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(res.Records))
	}
	if res.CostCents != 1 {
		t.Fatalf("expected cost 1, got %d", res.CostCents)
	}
	if got := logger.ByJob("job_test"); len(got) != 1 || got[0].Kind != audit.EventToolCall {
		t.Fatalf("expected 1 audit record, got %+v", got)
	}
}

func TestProxyRejectsUnauthorized(t *testing.T) {
	logger := audit.NewMemoryLogger()
	proxy := NewProxy(logger, NewProviderDirectorySource())

	_, err := proxy.Call(context.Background(), Call{
		JobID:        "job_x",
		ToolID:       "provider_directory",
		SubjectID:    "hello-001",
		AllowedTools: map[string]bool{"some_other_tool": true},
	})
	if !errors.Is(err, ErrToolNotAuthorized) {
		t.Fatalf("expected ErrToolNotAuthorized, got %v", err)
	}
}

func TestProxyRejectsUnknownTool(t *testing.T) {
	logger := audit.NewMemoryLogger()
	proxy := NewProxy(logger, NewProviderDirectorySource())
	_, err := proxy.Call(context.Background(), Call{
		JobID:        "job_y",
		ToolID:       "ghost_tool",
		AllowedTools: map[string]bool{"ghost_tool": true},
	})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected ErrToolNotFound, got %v", err)
	}
}

func TestProviderDirectoryUnknownSubject(t *testing.T) {
	src := NewProviderDirectorySource()
	got, err := src.Query(context.Background(), "missing", nil)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 records, got %d", len(got))
	}
}
