package search

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
)

// fakeEngine records IndexListing calls and exposes a configurable
// PipelineMode flag so we can exercise both indexer code paths
// without spinning up OpenSearch.
type fakeEngine struct {
	pipeline   bool
	indexed    int
	lastEmbed  []float64
	embedderUsed bool
}

func (f *fakeEngine) PipelineMode() bool                                          { return f.pipeline }
func (f *fakeEngine) EnsureIndex(context.Context) error                           { return nil }
func (f *fakeEngine) DeleteListing(context.Context, string) error                 { return nil }
func (f *fakeEngine) TextSearch(context.Context, string, SearchFilters, int) ([]SearchResult, error) {
	return nil, nil
}
func (f *fakeEngine) VectorSearch(context.Context, []float64, SearchFilters, int) ([]SearchResult, error) {
	return nil, nil
}
func (f *fakeEngine) SemanticSearch(context.Context, string, SearchFilters, int) ([]SearchResult, error) {
	return nil, nil
}
func (f *fakeEngine) IndexListing(_ context.Context, _, _, _, _, _ string, _ int, _, _ string, embedding []float64, _ string) error {
	f.indexed++
	f.lastEmbed = embedding
	return nil
}

type countingEmbedder struct{ calls int }

func (c *countingEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	c.calls++
	return []float64{0.1, 0.2, 0.3}, nil
}

func TestIndexer_PipelineModeSkipsClientEmbed(t *testing.T) {
	engine := &fakeEngine{pipeline: true}
	emb := &countingEmbedder{}
	idx := NewIndexer(engine, emb)

	listing := &domain.Listing{
		ID: "abc", Title: "t", Description: "d", Category: "c",
		Status: "active", PriceCents: 100,
		Tags: json.RawMessage(`["x"]`),
	}
	if err := idx.IndexListing(context.Background(), listing, nil, "", "seller"); err != nil {
		t.Fatalf("index: %v", err)
	}
	if emb.calls != 0 {
		t.Errorf("expected embedder to be skipped in pipeline mode, got %d calls", emb.calls)
	}
	if engine.lastEmbed != nil {
		t.Errorf("expected nil embedding when pipeline mode is on, got %v", engine.lastEmbed)
	}
}

func TestIndexer_NoPipelineUsesEmbedder(t *testing.T) {
	engine := &fakeEngine{pipeline: false}
	emb := &countingEmbedder{}
	idx := NewIndexer(engine, emb)

	listing := &domain.Listing{
		ID: "abc", Title: "t", Description: "d", Category: "c",
		Status: "active", PriceCents: 100,
		Tags: json.RawMessage(`["x"]`),
	}
	if err := idx.IndexListing(context.Background(), listing, nil, "", "seller"); err != nil {
		t.Fatalf("index: %v", err)
	}
	if emb.calls != 1 {
		t.Errorf("expected one embedder call, got %d", emb.calls)
	}
	if len(engine.lastEmbed) == 0 {
		t.Errorf("expected non-empty embedding in client-side mode")
	}
}

func TestIndexer_NoPipelineNoEmbedderErrors(t *testing.T) {
	engine := &fakeEngine{pipeline: false}
	idx := NewIndexer(engine, nil)

	listing := &domain.Listing{ID: "abc", Title: "t", Status: "active"}
	if err := idx.IndexListing(context.Background(), listing, nil, ""); err == nil {
		t.Fatalf("expected error when neither pipeline nor embedder is available")
	}
}
