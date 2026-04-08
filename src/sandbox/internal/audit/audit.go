// Package audit captures every interaction inside a sandbox job.
//
// The trust scoring layer of the exchange consumes audit records to
// build longitudinal accuracy scores. To make that downstream usage
// frictionless, every interesting event during a job run flows
// through the same `Logger` interface: job lifecycle, LLM turns, tool
// calls, and the final verdict.
//
// The MVP `MemoryLogger` keeps everything in a slice. A
// Postgres-backed logger drops in later behind the same interface.
package audit

import (
	"sync"
	"time"
)

// EventKind tags an audit record. Every record stored by a Logger
// carries one of these.
type EventKind string

const (
	EventJobCreated   EventKind = "JOB_CREATED"
	EventJobStarted   EventKind = "JOB_STARTED"
	EventJobCompleted EventKind = "JOB_COMPLETED"
	EventJobFailed    EventKind = "JOB_FAILED"
	EventLLMTurn      EventKind = "LLM_TURN"
	EventToolCall     EventKind = "TOOL_CALL"
	EventVerdict      EventKind = "VERDICT"
)

// Record is one append-only audit row. Fields beyond Kind are
// optional and only set for the events that need them.
type Record struct {
	Kind         EventKind `json:"kind"`
	JobID        string    `json:"job_id"`
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message,omitempty"`
	TurnNumber   int       `json:"turn_number,omitempty"`
	ToolID       string    `json:"tool_id,omitempty"`
	QueryShape   string    `json:"query_shape,omitempty"`
	ResponseSize int       `json:"response_size,omitempty"`
	CostCents    int       `json:"cost_cents,omitempty"`
	LatencyMS    int       `json:"latency_ms,omitempty"`
}

// Logger is the audit sink. Implementations must be goroutine-safe.
type Logger interface {
	Append(r Record)
	ByJob(jobID string) []Record
}

// MemoryLogger is the in-memory implementation used by the MVP. The
// production logger writes append-only rows to Postgres or DynamoDB
// behind the same interface.
type MemoryLogger struct {
	mu      sync.Mutex
	records []Record
}

func NewMemoryLogger() *MemoryLogger { return &MemoryLogger{} }

func (l *MemoryLogger) Append(r Record) {
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
}

func (l *MemoryLogger) ByJob(jobID string) []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Record
	for _, r := range l.records {
		if r.JobID == jobID {
			out = append(out, r)
		}
	}
	return out
}

// All returns every record. Test/debug helper, not part of the
// production interface.
func (l *MemoryLogger) All() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Record, len(l.records))
	copy(out, l.records)
	return out
}
