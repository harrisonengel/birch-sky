package api

import "github.com/harrisonengel/birch-sky/src/market-platform/internal/service"

// EnterRequest is the JSON body of POST /api/v1/enter.
//
// It is a thin alias over the service-layer EnterRequest because the API
// and service contract for entering the exchange are intentionally identical:
// entering the exchange is the marketplace's product surface, and we don't
// want a translation layer between what callers send and what the engine
// receives.
type EnterRequest = service.EnterRequest

// EnterResponse is the JSON body returned from POST /api/v1/enter.
type EnterResponse = service.EnterResponse

// Validate enforces required fields on the request.
func validateEnterRequest(r *EnterRequest) error {
	if r.Query == "" {
		return errMissingField("query")
	}
	return nil
}
