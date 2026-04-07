package api

import (
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

// InitiatePurchaseRequest is the JSON body of POST /api/v1/purchases.
type InitiatePurchaseRequest struct {
	BuyerID   string `json:"buyer_id"`
	ListingID string `json:"listing_id"`
}

func (r *InitiatePurchaseRequest) Validate() error {
	if r.BuyerID == "" {
		return errMissingField("buyer_id")
	}
	if r.ListingID == "" {
		return errMissingField("listing_id")
	}
	return nil
}

// InitiatePurchaseResponse is the JSON body returned from POST
// /api/v1/purchases.
type InitiatePurchaseResponse = service.InitiatePurchaseResponse

// ConfirmPurchaseResponse is the JSON body returned from POST
// /api/v1/purchases/{id}/confirm.
type ConfirmPurchaseResponse = domain.Ownership

// GetPurchaseResponse is the JSON body returned from GET /api/v1/purchases/{id}.
type GetPurchaseResponse = domain.Transaction

// ListOwnershipRequest captures query parameters for GET /api/v1/ownership.
type ListOwnershipRequest struct {
	BuyerID string
}

func (r *ListOwnershipRequest) Validate() error {
	if r.BuyerID == "" {
		return errMissingField("buyer_id")
	}
	return nil
}

func parseListOwnershipRequest(r *http.Request) ListOwnershipRequest {
	return ListOwnershipRequest{
		BuyerID: queryString(r, "buyer_id"),
	}
}

// ListOwnershipResponse is the JSON body returned from GET /api/v1/ownership.
type ListOwnershipResponse = []domain.Ownership

// DownloadOwnershipRequest captures query parameters for GET
// /api/v1/ownership/{listingID}/download. The listing ID is taken from
// the URL path; only buyer_id arrives via query string.
type DownloadOwnershipRequest struct {
	BuyerID string
}

func (r *DownloadOwnershipRequest) Validate() error {
	if r.BuyerID == "" {
		return errMissingField("buyer_id")
	}
	return nil
}

func parseDownloadOwnershipRequest(r *http.Request) DownloadOwnershipRequest {
	return DownloadOwnershipRequest{
		BuyerID: queryString(r, "buyer_id"),
	}
}

// DownloadOwnershipResponse is the JSON body returned from GET
// /api/v1/ownership/{listingID}/download.
type DownloadOwnershipResponse struct {
	DownloadURL string `json:"download_url"`
}
