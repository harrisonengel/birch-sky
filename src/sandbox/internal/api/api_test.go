package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/harness"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/jobstore"
)

func newTestServer() *Server {
	return &Server{
		APIKey:   "test-key",
		Store:    jobstore.NewMemoryStore(),
		Queue:    jobstore.NewChannelQueue(8),
		Registry: harness.NewRegistry(),
		Audit:    audit.NewMemoryLogger(),
	}
}

func do(t *testing.T, h http.Handler, method, path, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	srv := newTestServer()
	rec := do(t, srv.Routes(), http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestListObjectivesAuth(t *testing.T) {
	srv := newTestServer()
	rec := do(t, srv.Routes(), http.MethodGet, "/v1/objectives", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}
	rec = do(t, srv.Routes(), http.MethodGet, "/v1/objectives", "", "test-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with key, got %d", rec.Code)
	}
	var body struct {
		Objectives []objectiveResponse `json:"objectives"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Objectives) == 0 {
		t.Fatal("expected at least one objective")
	}
}

func TestSubmitAndGetJob(t *testing.T) {
	srv := newTestServer()
	h := srv.Routes()

	briefJSON := `{
		"objective": "healthcare.provider.verification",
		"subject": {"provider_id": "hello-001"},
		"buyer_id": "buyer-1"
	}`

	rec := do(t, h, http.MethodPost, "/v1/briefs", briefJSON, "test-key")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var sub submitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sub); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sub.JobID == "" {
		t.Fatal("expected job_id")
	}
	if sub.State != string(jobstore.StateQueued) {
		t.Fatalf("expected QUEUED, got %s", sub.State)
	}

	// Drain the queue so we can confirm the API truly enqueued it.
	gotID, err := srv.Queue.Pop(context.Background())
	if err != nil {
		t.Fatalf("queue pop: %v", err)
	}
	if gotID != sub.JobID {
		t.Fatalf("queue had %q, want %q", gotID, sub.JobID)
	}

	rec = do(t, h, http.MethodGet, "/v1/jobs/"+sub.JobID, "", "test-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSubmitInvalidObjective(t *testing.T) {
	srv := newTestServer()
	rec := do(t, srv.Routes(), http.MethodPost, "/v1/briefs", `{"objective":"unknown","subject":{"x":"y"},"buyer_id":"b"}`, "test-key")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
