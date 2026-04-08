package jobstore

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/brief"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/verdict"
)

func sampleBrief() brief.Brief {
	return brief.Brief{
		Objective: "healthcare.provider.verification",
		Subject:   json.RawMessage(`{"provider_id":"hello-001"}`),
		BuyerID:   "buyer-1",
	}
}

func TestStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	job, err := store.Create(ctx, sampleBrief())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.State != StateQueued {
		t.Fatalf("expected QUEUED, got %v", job.State)
	}

	if err := store.UpdateState(ctx, job.ID, StateRunning); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != StateRunning || got.StartedAt == nil {
		t.Fatalf("expected RUNNING with StartedAt set, got %+v", got)
	}

	v := &verdict.Verdict{
		Assessment:        verdict.AssessmentConfirmed,
		Confidence:        verdict.ConfidenceHigh,
		SourcesConsulted:  []string{"provider_directory"},
		Agreement:         verdict.AgreementSummary{Agreed: 1, TotalQueried: 1},
		RecommendedAction: verdict.ActionNone,
	}
	if err := store.Complete(ctx, job.ID, v, 5); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, _ = store.Get(ctx, job.ID)
	if got.State != StateCompleted || got.Verdict == nil || got.CostCents != 5 {
		t.Fatalf("unexpected job after Complete: %+v", got)
	}
}

func TestStoreFail(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	job, _ := store.Create(ctx, sampleBrief())
	if err := store.Fail(ctx, job.ID, "boom"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	got, _ := store.Get(ctx, job.ID)
	if got.State != StateFailed || got.Error != "boom" {
		t.Fatalf("unexpected failed job: %+v", got)
	}
}

func TestQueueRoundTrip(t *testing.T) {
	q := NewChannelQueue(2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := q.Push(ctx, "job_a"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	id, err := q.Pop(ctx)
	if err != nil || id != "job_a" {
		t.Fatalf("Pop = %q, err = %v", id, err)
	}
}

func TestQueueConcurrent(t *testing.T) {
	q := NewChannelQueue(8)
	ctx := context.Background()

	var wg sync.WaitGroup
	got := make(chan string, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := q.Pop(ctx)
			if err == nil {
				got <- id
			}
		}()
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		_ = q.Push(ctx, id)
	}
	wg.Wait()
	close(got)
	if len(got) != 4 {
		t.Fatalf("expected 4 popped, got %d", len(got))
	}
}
