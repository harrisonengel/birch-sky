package domain

import "time"

type BuyOrderStatus string

const (
	BuyOrderStatusOpen      BuyOrderStatus = "open"
	BuyOrderStatusFilled    BuyOrderStatus = "filled"
	BuyOrderStatusCancelled BuyOrderStatus = "cancelled"
	BuyOrderStatusExpired   BuyOrderStatus = "expired"
)

type BuyOrder struct {
	ID                string         `db:"id" json:"id"`
	BuyerID           string         `db:"buyer_id" json:"buyer_id"`
	Query             string         `db:"query" json:"query"`
	Criteria          string         `db:"criteria" json:"criteria"`
	MaxPriceCents     int            `db:"max_price_cents" json:"max_price_cents"`
	Currency          string         `db:"currency" json:"currency"`
	Category          string         `db:"category" json:"category"`
	Status            BuyOrderStatus `db:"status" json:"status"`
	FilledByListingID *string        `db:"filled_by_listing_id" json:"filled_by_listing_id,omitempty"`
	CreatedAt         time.Time      `db:"created_at" json:"created_at"`
	ExpiresAt         *time.Time     `db:"expires_at" json:"expires_at,omitempty"`
}

type BuyOrderFilter struct {
	BuyerID  string
	Status   string
	Category string
	Limit    int
	Offset   int
}
