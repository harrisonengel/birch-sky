package api

import (
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
)

// CreateBuyOrderRequest is the JSON body of POST /api/v1/buy-orders.
type CreateBuyOrderRequest struct {
	BuyerID       string                  `json:"buyer_id"`
	Query         string                  `json:"query"`
	Criteria      domain.BuyOrderCriteria `json:"criteria"`
	MaxPriceCents int                     `json:"max_price_cents"`
	Currency      string                  `json:"currency"`
	Category      string                  `json:"category"`
}

func (r *CreateBuyOrderRequest) Validate() error {
	if r.BuyerID == "" {
		return errMissingField("buyer_id")
	}
	if r.Query == "" {
		return errMissingField("query")
	}
	if r.MaxPriceCents <= 0 {
		return errInvalidField("max_price_cents", "must be positive")
	}
	return nil
}

// CreateBuyOrderResponse is the JSON body returned from POST /api/v1/buy-orders.
type CreateBuyOrderResponse = domain.BuyOrder

// GetBuyOrderResponse is the JSON body returned from GET /api/v1/buy-orders/{id}.
type GetBuyOrderResponse = domain.BuyOrder

// ListBuyOrdersRequest captures query parameters for GET /api/v1/buy-orders.
type ListBuyOrdersRequest struct {
	BuyerID  string
	Status   string
	Category string
	Limit    int
	Offset   int
}

func parseListBuyOrdersRequest(r *http.Request) ListBuyOrdersRequest {
	return ListBuyOrdersRequest{
		BuyerID:  queryString(r, "buyer_id"),
		Status:   queryString(r, "status"),
		Category: queryString(r, "category"),
		Limit:    queryInt(r, "limit", 20),
		Offset:   queryInt(r, "offset", 0),
	}
}

// ListBuyOrdersResponse is the JSON body returned from GET /api/v1/buy-orders.
type ListBuyOrdersResponse = PaginatedResponse

// FillBuyOrderRequest is the JSON body of POST /api/v1/buy-orders/{id}/fill.
type FillBuyOrderRequest struct {
	ListingID string `json:"listing_id"`
}

func (r *FillBuyOrderRequest) Validate() error {
	if r.ListingID == "" {
		return errMissingField("listing_id")
	}
	return nil
}

// FillBuyOrderResponse is the JSON body returned from
// POST /api/v1/buy-orders/{id}/fill.
type FillBuyOrderResponse = domain.BuyOrder
