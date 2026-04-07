package api

import "github.com/harrisonengel/birch-sky/src/market-platform/internal/service"

// SearchRequest is the JSON body of POST /api/v1/search.
//
// It is a thin alias over the service-layer SearchRequest because the API
// and service contract for search are intentionally identical: search is
// the marketplace's product surface, and we don't want a translation layer
// between what callers send and what the engine receives.
type SearchRequest = service.SearchRequest

// SearchResponse is the JSON body returned from POST /api/v1/search.
type SearchResponse = service.SearchResponse

// Validate enforces required fields on the request.
func validateSearchRequest(r *SearchRequest) error {
	if r.Query == "" {
		return errMissingField("query")
	}
	return nil
}
