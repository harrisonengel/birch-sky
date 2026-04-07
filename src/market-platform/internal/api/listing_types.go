package api

import (
	"encoding/json"
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
)

// CreateSellerRequest is the JSON body of POST /api/v1/sellers.
type CreateSellerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Validate enforces required fields. It is called by the handler before
// any service work. Validation lives on the typed object so a reader can
// see the contract from one file.
func (r *CreateSellerRequest) Validate() error {
	if r.Name == "" {
		return errMissingField("name")
	}
	if r.Email == "" {
		return errMissingField("email")
	}
	return nil
}

// CreateSellerResponse is the JSON body returned from POST /api/v1/sellers.
// It is the persisted seller record.
type CreateSellerResponse = domain.Seller

// GetSellerResponse is the JSON body returned from GET /api/v1/sellers/{id}.
type GetSellerResponse = domain.Seller

// CreateListingRequest is the JSON body of POST /api/v1/listings.
type CreateListingRequest struct {
	SellerID    string          `json:"seller_id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	PriceCents  int             `json:"price_cents"`
	Currency    string          `json:"currency"`
	Tags        json.RawMessage `json:"tags"`
}

func (r *CreateListingRequest) Validate() error {
	if r.SellerID == "" {
		return errMissingField("seller_id")
	}
	if r.Title == "" {
		return errMissingField("title")
	}
	if r.Description == "" {
		return errMissingField("description")
	}
	if r.PriceCents < 0 {
		return errInvalidField("price_cents", "must be non-negative")
	}
	return nil
}

// CreateListingResponse is the JSON body returned from POST /api/v1/listings.
type CreateListingResponse = domain.Listing

// GetListingResponse is the JSON body returned from GET /api/v1/listings/{id}.
type GetListingResponse = domain.Listing

// ListListingsRequest captures the query parameters of GET /api/v1/listings.
// It is populated from the URL by parseListListingsRequest.
type ListListingsRequest struct {
	SellerID string
	Status   string
	Category string
	Limit    int
	Offset   int
}

func parseListListingsRequest(r *http.Request) ListListingsRequest {
	return ListListingsRequest{
		SellerID: queryString(r, "seller_id"),
		Status:   queryString(r, "status"),
		Category: queryString(r, "category"),
		Limit:    queryInt(r, "limit", 20),
		Offset:   queryInt(r, "offset", 0),
	}
}

// ListListingsResponse is the JSON body returned from GET /api/v1/listings.
// Data is always a slice of Listing — typed via PaginatedResponse[Listing].
type ListListingsResponse = PaginatedResponse

// UpdateListingRequest is the JSON body of PUT /api/v1/listings/{id}. Every
// field is optional; only provided fields are updated.
type UpdateListingRequest = domain.ListingUpdate

// UpdateListingResponse is the JSON body returned from PUT /api/v1/listings/{id}.
type UpdateListingResponse = domain.Listing

// UploadListingResponse is the JSON body returned from
// POST /api/v1/listings/{id}/upload.
type UploadListingResponse struct {
	Status string `json:"status"`
}
