// Package jobstore holds the per-job state machine and the dispatch
// queue between the API gateway and the harness workers.
//
// The MVP ships an in-memory `Store` and a channel-backed `Queue`.
// Both are wrapped behind small interfaces so the Postgres + SQS
// versions drop in later without touching callers.
package jobstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/brief"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

// State is the lifecycle of a single sandbox job.
type State string

const (
	StateQueued    State = "QUEUED"
	StateRunning   State = "RUNNING"
	StateCompleted State = "COMPLETED"
	StateFailed    State = "FAILED"
	StateTimeout   State = "TIMEOUT"
)

// Job is the persisted record for one sandbox run.
type Job struct {
	ID          string           `json:"id"`
	Brief       brief.Brief      `json:"brief"`
	State       State            `json:"state"`
	Verdict     *verdict.Verdict `json:"verdict,omitempty"`
	Error       string           `json:"error,omitempty"`
	CostCents   int              `json:"cost_cents"`
	CreatedAt   time.Time        `json:"created_at"`
	StartedAt   *time.Time       `json:"started_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
}

// Store persists job state. Implementations must be goroutine-safe.
type Store interface {
	Create(ctx context.Context, b brief.Brief) (*Job, error)
	Get(ctx context.Context, id string) (*Job, error)
	UpdateState(ctx context.Context, id string, state State) error
	Complete(ctx context.Context, id string, v *verdict.Verdict, costCents int) error
	Fail(ctx context.Context, id string, reason string) error
}

// Queue dispatches job IDs from the API gateway to the harness
// workers. Implementations must be goroutine-safe.
type Queue interface {
	Push(ctx context.Context, jobID string) error
	// Pop blocks until a job is available or ctx is cancelled.
	Pop(ctx context.Context) (string, error)
}

// ErrNotFound is returned by Store.Get when no job exists.
var ErrNotFound = errors.New("jobstore: job not found")

// MemoryStore is the in-memory implementation of Store. Postgres swap
// later, same interface.
type MemoryStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: make(map[string]*Job)}
}

func (s *MemoryStore) Create(_ context.Context, b brief.Brief) (*Job, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	job := &Job{
		ID:        id,
		Brief:     b,
		State:     StateQueued,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.jobs[id] = job
	s.mu.Unlock()
	return cloneJob(job), nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneJob(job), nil
}

func (s *MemoryStore) UpdateState(_ context.Context, id string, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	job.State = state
	if state == StateRunning && job.StartedAt == nil {
		now := time.Now().UTC()
		job.StartedAt = &now
	}
	return nil
}

func (s *MemoryStore) Complete(_ context.Context, id string, v *verdict.Verdict, costCents int) error {
	if v == nil {
		return errors.New("jobstore: verdict is nil")
	}
	if err := v.Validate(); err != nil {
		return fmt.Errorf("jobstore: verdict invalid: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	verdictCopy := *v
	job.Verdict = &verdictCopy
	job.CostCents = costCents
	job.State = StateCompleted
	job.CompletedAt = &now
	return nil
}

func (s *MemoryStore) Fail(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	job.State = StateFailed
	job.Error = reason
	job.CompletedAt = &now
	return nil
}

func cloneJob(j *Job) *Job {
	out := *j
	if j.Verdict != nil {
		v := *j.Verdict
		out.Verdict = &v
	}
	return &out
}

func newID() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "job_" + hex.EncodeToString(buf[:]), nil
}

// ChannelQueue is the in-memory implementation of Queue.
type ChannelQueue struct {
	ch chan string
}

func NewChannelQueue(buffer int) *ChannelQueue {
	if buffer <= 0 {
		buffer = 64
	}
	return &ChannelQueue{ch: make(chan string, buffer)}
}

func (q *ChannelQueue) Push(ctx context.Context, jobID string) error {
	select {
	case q.ch <- jobID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *ChannelQueue) Pop(ctx context.Context) (string, error) {
	select {
	case id := <-q.ch:
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Close stops accepting new pushes. Pop drains remaining items, then
// returns ctx.Err() once empty and ctx is cancelled.
func (q *ChannelQueue) Close() {
	close(q.ch)
}
