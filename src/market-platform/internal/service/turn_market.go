package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
)

type TurnMarketService struct {
	engine   search.SearchEngine
	embedder search.Embedder // optional; only consulted when the engine is not in pipeline mode
}

func NewTurnMarketService(engine search.SearchEngine, embedder search.Embedder) *TurnMarketService {
	return &TurnMarketService{engine: engine, embedder: embedder}
}

type EnterRequest struct {
	Query         string `json:"query"`
	Category      string `json:"category,omitempty"`
	MaxPriceCents *int   `json:"max_price_cents,omitempty"`
	Mode          string `json:"mode,omitempty"` // hybrid, text, vector (default: hybrid)
	PerPage       int    `json:"per_page,omitempty"`
}

type EnterResponse struct {
	Results []search.SearchResult `json:"results"`
	Total   int                   `json:"total"`
	Mode    string                `json:"mode"`
}

func (s *TurnMarketService) Enter(ctx context.Context, req EnterRequest) (*EnterResponse, error) {
	mode := req.Mode
	if mode == "" {
		mode = "hybrid"
	}
	size := req.PerPage
	if size <= 0 {
		size = 20
	}

	filters := search.SearchFilters{
		Category:      req.Category,
		MaxPriceCents: req.MaxPriceCents,
	}

	switch mode {
	case "text":
		results, err := s.engine.TextSearch(ctx, req.Query, filters, size)
		if err != nil {
			return nil, err
		}
		return &EnterResponse{Results: results, Total: len(results), Mode: mode}, nil

	case "vector":
		results, err := s.semanticSearch(ctx, req.Query, filters, size)
		if err != nil {
			return nil, err
		}
		return &EnterResponse{Results: results, Total: len(results), Mode: mode}, nil

	default: // hybrid
		return s.hybridEnter(ctx, req.Query, filters, size)
	}
}

// semanticSearch picks the right vector path: server-side neural query
// when the engine has an ML pipeline, client-side embed+kNN otherwise.
func (s *TurnMarketService) semanticSearch(ctx context.Context, query string, filters search.SearchFilters, size int) ([]search.SearchResult, error) {
	if s.engine.PipelineMode() {
		return s.engine.SemanticSearch(ctx, query, filters, size)
	}
	if s.embedder == nil {
		return nil, fmt.Errorf("vector search not configured: no ML pipeline and no embedder")
	}
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.engine.VectorSearch(ctx, embedding, filters, size)
}

func (s *TurnMarketService) hybridEnter(ctx context.Context, query string, filters search.SearchFilters, size int) (*EnterResponse, error) {
	// Issue text and vector searches in parallel
	var textResults, vectorResults []search.SearchResult
	var textErr, vectorErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		textResults, textErr = s.engine.TextSearch(ctx, query, filters, size)
	}()
	go func() {
		defer wg.Done()
		vectorResults, vectorErr = s.semanticSearch(ctx, query, filters, size)
	}()
	wg.Wait()

	if textErr != nil {
		return nil, textErr
	}
	if vectorErr != nil {
		return nil, vectorErr
	}

	// Reciprocal Rank Fusion (k=60)
	merged := rrfMerge(textResults, vectorResults, 60, size)
	return &EnterResponse{Results: merged, Total: len(merged), Mode: "hybrid"}, nil
}

func rrfMerge(textResults, vectorResults []search.SearchResult, k, maxResults int) []search.SearchResult {
	scores := map[string]float64{}
	resultMap := map[string]search.SearchResult{}

	for rank, r := range textResults {
		scores[r.ListingID] += 1.0 / float64(k+rank+1)
		resultMap[r.ListingID] = r
	}
	for rank, r := range vectorResults {
		scores[r.ListingID] += 1.0 / float64(k+rank+1)
		if _, exists := resultMap[r.ListingID]; !exists {
			resultMap[r.ListingID] = r
		}
	}

	type scored struct {
		id    string
		score float64
	}
	var ranked []scored
	for id, score := range scores {
		ranked = append(ranked, scored{id, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	results := make([]search.SearchResult, 0, maxResults)
	for i, s := range ranked {
		if i >= maxResults {
			break
		}
		r := resultMap[s.id]
		r.Score = s.score
		results = append(results, r)
	}
	return results
}
