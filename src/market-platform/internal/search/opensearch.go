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
	Category     string
	Status       string
	MaxPriceCents *int
}

type SearchResult struct {
	ListingID string  `json:"listing_id"`
	Score     float64 `json:"score"`
	Title     string  `json:"title"`
	Description string `json:"description"`
	Category  string  `json:"category"`
	PriceCents int    `json:"price_cents"`
}

type SearchEngine interface {
	EnsureIndex(ctx context.Context) error
	IndexListing(ctx context.Context, listingID, title, description, category, status string, priceCents int, tags string, contentText string, embedding []float64) error
	DeleteListing(ctx context.Context, listingID string) error
	TextSearch(ctx context.Context, query string, filters SearchFilters, size int) ([]SearchResult, error)
	VectorSearch(ctx context.Context, embedding []float64, filters SearchFilters, size int) ([]SearchResult, error)
}

type OpenSearchEngine struct {
	baseURL    string
	httpClient *http.Client
}

func NewOpenSearchEngine(baseURL string) (*OpenSearchEngine, error) {
	return &OpenSearchEngine{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}, nil
}

func (e *OpenSearchEngine) EnsureIndex(ctx context.Context) error {
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

	// Create index
	body, _ := json.Marshal(IndexMapping)
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

func (e *OpenSearchEngine) IndexListing(ctx context.Context, listingID, title, description, category, status string, priceCents int, tags string, contentText string, embedding []float64) error {
	doc := map[string]interface{}{
		"listing_id":   listingID,
		"title":        title,
		"description":  description,
		"category":     category,
		"status":       status,
		"price_cents":  priceCents,
		"tags":         tags,
		"content_text": contentText,
		"embedding":    embedding,
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
							"embedding": map[string]interface{}{
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
			ListingID: getString(hit.Source, "listing_id"),
			Score:     hit.Score,
			Title:     getString(hit.Source, "title"),
			Description: getString(hit.Source, "description"),
			Category:  getString(hit.Source, "category"),
			PriceCents: getInt(hit.Source, "price_cents"),
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
