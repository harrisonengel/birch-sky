package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type SearchFilters struct {
	Category      string
	Status        string
	MaxPriceCents *int
}

type SearchResult struct {
	ListingID   string  `json:"listing_id"`
	Score       float64 `json:"score"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	PriceCents  int     `json:"price_cents"`
	SellerName  string  `json:"seller_name"`
}

// SearchEngine is the abstract index/search surface used by the
// service layer. Implementations may handle embedding either
// client-side (passing pre-computed vectors via IndexListing) or
// server-side (via an OpenSearch ingest pipeline + neural query).
type SearchEngine interface {
	EnsureIndex(ctx context.Context) error
	IndexListing(ctx context.Context, listingID, title, description, category, status string, priceCents int, tags string, contentText string, embedding []float64, sellerName string) error
	DeleteListing(ctx context.Context, listingID string) error
	TextSearch(ctx context.Context, query string, filters SearchFilters, size int) ([]SearchResult, error)
	VectorSearch(ctx context.Context, embedding []float64, filters SearchFilters, size int) ([]SearchResult, error)
	// SemanticSearch runs a vector-space search from raw text. Engines
	// in pipeline mode use OpenSearch's neural query (server embeds
	// the query text); engines without a model fall back to embedding
	// the query externally — but that fallback is the caller's job
	// (the service can still call VectorSearch directly with a
	// pre-computed vector). When pipeline mode is disabled this
	// returns ErrNoSemanticSearch.
	SemanticSearch(ctx context.Context, queryText string, filters SearchFilters, size int) ([]SearchResult, error)
	// PipelineMode reports whether server-side embeddings are wired
	// up. Callers (e.g. Indexer, TurnMarketService) use this to
	// decide whether to compute embeddings client-side.
	PipelineMode() bool
}

// ErrNoSemanticSearch is returned by SemanticSearch on engines that
// don't have an OpenSearch ML pipeline configured.
var ErrNoSemanticSearch = fmt.Errorf("semantic search not configured: no ML pipeline model id")

type OpenSearchEngine struct {
	baseURL    string
	httpClient *http.Client
	ml         *MLClient // optional; non-nil enables server-side embeddings
}

// NewOpenSearchEngine creates a basic engine. The caller is
// responsible for either calling EnsureIndex (which will set up the
// index with no ingest pipeline) or first calling EnableMLPipeline to
// bootstrap the OpenSearch ML pipeline.
func NewOpenSearchEngine(baseURL string) (*OpenSearchEngine, error) {
	return &OpenSearchEngine{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}, nil
}

// EnableMLPipeline boots the ML Commons setup (model + ingest
// pipeline) and tells the engine to rely on server-side embeddings
// from now on. Idempotent — safe to call again on warm clusters.
func (e *OpenSearchEngine) EnableMLPipeline(ctx context.Context) error {
	ml := NewMLClient(e.baseURL)
	if err := ml.SetupModel(ctx); err != nil {
		return err
	}
	e.ml = ml
	return nil
}

// PipelineMode reports whether server-side embeddings are configured.
func (e *OpenSearchEngine) PipelineMode() bool {
	return e.ml != nil && e.ml.ModelID() != ""
}

// MLModelID returns the id of the deployed embedding model, or "" if
// pipeline mode is off. Useful for diagnostics / CLI status output.
func (e *OpenSearchEngine) MLModelID() string {
	if e.ml == nil {
		return ""
	}
	return e.ml.ModelID()
}

func (e *OpenSearchEngine) EnsureIndex(ctx context.Context) error {
	pipeline := ""
	if e.PipelineMode() {
		pipeline = MLPipelineName
	}

	// Check if index exists
	req, _ := http.NewRequestWithContext(ctx, "HEAD", e.baseURL+"/"+IndexName, nil)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil // index exists
	}

	body, _ := json.Marshal(IndexMappingFor(pipeline))
	req, _ = http.NewRequestWithContext(ctx, "PUT", e.baseURL+"/"+IndexName, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// IndexListing writes a document. In pipeline mode the embedding
// argument is ignored (and may be nil) — the OpenSearch ingest
// pipeline computes the vector from the embedding_text field. In
// non-pipeline (client-side) mode, the caller must pass a
// pre-computed embedding of length EmbeddingDimension.
func (e *OpenSearchEngine) IndexListing(ctx context.Context, listingID, title, description, category, status string, priceCents int, tags string, contentText string, embedding []float64, sellerName string) error {
	doc := map[string]interface{}{
		"listing_id":  listingID,
		"title":       title,
		"description": description,
		"category":    category,
		"status":      status,
		"price_cents": priceCents,
		"tags":        tags,
		"content_text": contentText,
		"seller_name": sellerName,
	}

	if e.PipelineMode() {
		// Hand raw text to the pipeline. Joining all fields keeps
		// the embedding aligned with the same content the BM25 side
		// matches against, which improves hybrid fusion quality.
		doc[MLEmbeddingTextField] = strings.Join([]string{title, description, tags, contentText}, " ")
	} else {
		doc[MLEmbeddingField] = embedding
	}

	body, _ := json.Marshal(doc)
	req, _ := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/%s/_doc/%s", e.baseURL, IndexName, listingID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Force refresh for immediate search availability
	q := req.URL.Query()
	q.Set("refresh", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("index listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index listing failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (e *OpenSearchEngine) DeleteListing(ctx context.Context, listingID string) error {
	req, _ := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/%s/_doc/%s?refresh=true", e.baseURL, IndexName, listingID), nil)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete listing from index: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (e *OpenSearchEngine) TextSearch(ctx context.Context, query string, filters SearchFilters, size int) ([]SearchResult, error) {
	if size <= 0 {
		size = 20
	}

	searchQuery := map[string]interface{}{
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"combined_fields": map[string]interface{}{
							"query":    query,
							"fields":   []string{"title^3", "description^2", "tags^2", "content_text"},
							"operator": "or",
						},
					},
				},
				"filter": buildFilters(filters),
			},
		},
	}

	return e.executeSearch(ctx, searchQuery)
}

func (e *OpenSearchEngine) VectorSearch(ctx context.Context, embedding []float64, filters SearchFilters, size int) ([]SearchResult, error) {
	if size <= 0 {
		size = 20
	}

	searchQuery := map[string]interface{}{
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"knn": map[string]interface{}{
							MLEmbeddingField: map[string]interface{}{
								"vector": embedding,
								"k":      size,
							},
						},
					},
				},
				"filter": buildFilters(filters),
			},
		},
	}

	return e.executeSearch(ctx, searchQuery)
}

// SemanticSearch issues a `neural` query that asks OpenSearch to embed
// the query text with the deployed model and run kNN against
// MLEmbeddingField. Returns ErrNoSemanticSearch if the engine isn't in
// pipeline mode.
func (e *OpenSearchEngine) SemanticSearch(ctx context.Context, queryText string, filters SearchFilters, size int) ([]SearchResult, error) {
	if !e.PipelineMode() {
		return nil, ErrNoSemanticSearch
	}
	if size <= 0 {
		size = 20
	}

	searchQuery := map[string]interface{}{
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"neural": map[string]interface{}{
							MLEmbeddingField: map[string]interface{}{
								"query_text": queryText,
								"model_id":   e.ml.ModelID(),
								"k":          size,
							},
						},
					},
				},
				"filter": buildFilters(filters),
			},
		},
	}

	return e.executeSearch(ctx, searchQuery)
}

func buildFilters(filters SearchFilters) []interface{} {
	f := []interface{}{
		map[string]interface{}{
			"term": map[string]interface{}{
				"status": "active",
			},
		},
	}

	if filters.Category != "" {
		f = append(f, map[string]interface{}{
			"term": map[string]interface{}{
				"category": filters.Category,
			},
		})
	}
	if filters.MaxPriceCents != nil {
		f = append(f, map[string]interface{}{
			"range": map[string]interface{}{
				"price_cents": map[string]interface{}{
					"lte": *filters.MaxPriceCents,
				},
			},
		})
	}
	return f
}

func (e *OpenSearchEngine) executeSearch(ctx context.Context, searchQuery map[string]interface{}) ([]SearchResult, error) {
	body, _ := json.Marshal(searchQuery)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/%s/_search", e.baseURL, IndexName),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var osResp struct {
		Hits struct {
			Hits []struct {
				ID     string                 `json:"_id"`
				Score  float64                `json:"_score"`
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&osResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	results := make([]SearchResult, 0, len(osResp.Hits.Hits))
	for _, hit := range osResp.Hits.Hits {
		r := SearchResult{
			ListingID:   getString(hit.Source, "listing_id"),
			Score:       hit.Score,
			Title:       getString(hit.Source, "title"),
			Description: getString(hit.Source, "description"),
			Category:    getString(hit.Source, "category"),
			PriceCents:  getInt(hit.Source, "price_cents"),
			SellerName:  getString(hit.Source, "seller_name"),
		}
		results = append(results, r)
	}
	return results, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}
