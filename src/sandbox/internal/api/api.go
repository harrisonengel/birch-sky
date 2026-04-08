// Package api implements the public HTTP surface of the sandbox.
//
// The router is plain `net/http`. Auth is a single static API key
// checked by middleware so the seam for swapping in JWT later is one
// function. Handlers translate HTTP into calls against the jobstore,
// the harness registry, and the audit logger — they hold no state of
// their own.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/brief"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/harness"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/jobstore"
)

// Server holds the dependencies a request needs. Construct one in
// `cmd/sandbox-server/main.go` and call `Routes()` to mount it.
type Server struct {
	APIKey   string
	Store    jobstore.Store
	Queue    jobstore.Queue
	Registry *harness.Registry
	Audit    audit.Logger
}

// Routes returns an http.Handler with every public endpoint mounted.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /v1/objectives", s.requireAuth(s.listObjectives))
	mux.HandleFunc("POST /v1/briefs", s.requireAuth(s.submitBrief))
	mux.HandleFunc("GET /v1/jobs/{id}", s.requireAuth(s.getJob))
	mux.HandleFunc("GET /v1/audit/{id}", s.requireAuth(s.getAudit))
	return mux
}

// requireAuth is the static-API-key middleware. Replace with JWT
// verification by swapping the contents of this function.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.APIKey == "" {
			next(w, r)
			return
		}
		if r.Header.Get("X-API-Key") != s.APIKey {
			writeError(w, http.StatusUnauthorized, "missing or invalid api key")
			return
		}
		next(w, r)
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// objectiveResponse mirrors a Template for buyers. Internal-only
// fields (like the system prompt) are stripped.
type objectiveResponse struct {
	ID                    string   `json:"id"`
	Description           string   `json:"description"`
	RequiredSubjectFields []string `json:"required_subject_fields"`
	AllowedTools          []string `json:"allowed_tools"`
	DefaultBudgetCents    int      `json:"default_budget_cents"`
	TurnLimit             int      `json:"turn_limit"`
}

func (s *Server) listObjectives(w http.ResponseWriter, _ *http.Request) {
	templates := s.Registry.All()
	out := make([]objectiveResponse, 0, len(templates))
	for _, t := range templates {
		out = append(out, objectiveResponse{
			ID:                    t.ID,
			Description:           t.Description,
			RequiredSubjectFields: t.RequiredSubjectFields,
			AllowedTools:          t.AllowedTools,
			DefaultBudgetCents:    t.DefaultBudgetCents,
			TurnLimit:             t.TurnLimit,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"objectives": out})
}

type submitResponse struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
}

func (s *Server) submitBrief(w http.ResponseWriter, r *http.Request) {
	var b brief.Brief
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.Registry.Lookup(b.Objective); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := s.Store.Create(r.Context(), b)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Audit.Append(audit.Record{Kind: audit.EventJobCreated, JobID: job.ID})

	if err := s.Queue.Push(r.Context(), job.ID); err != nil {
		_ = s.Store.Fail(context.Background(), job.ID, "queue push failed: "+err.Error())
		writeError(w, http.StatusServiceUnavailable, "queue unavailable")
		return
	}

	writeJSON(w, http.StatusAccepted, submitResponse{JobID: job.ID, State: string(job.State)})
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}
	job, err := s.Store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, jobstore.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}
	records := s.Audit.ByJob(id)
	writeJSON(w, http.StatusOK, map[string]any{"job_id": id, "records": records})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(msg)})
}
