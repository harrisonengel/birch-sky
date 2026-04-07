package domain

import (
	"encoding/json"
	"time"
)

type ListingStatus string

const (
	ListingStatusActive   ListingStatus = "active"
	ListingStatusInactive ListingStatus = "inactive"
	ListingStatusDeleted  ListingStatus = "deleted"
)

type Seller struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Listing struct {
	ID            string          `db:"id" json:"id"`
	SellerID      string          `db:"seller_id" json:"seller_id"`
	Title         string          `db:"title" json:"title"`
	Description   string          `db:"description" json:"description"`
	Category      string          `db:"category" json:"category"`
	PriceCents    int             `db:"price_cents" json:"price_cents"`
	Currency      string          `db:"currency" json:"currency"`
	DataRef       string          `db:"data_ref" json:"data_ref"`
	DataFormat    string          `db:"data_format" json:"data_format"`
	DataSizeBytes int64           `db:"data_size_bytes" json:"data_size_bytes"`
	Tags          json.RawMessage `db:"tags" json:"tags"`
	Status        ListingStatus   `db:"status" json:"status"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

type ListingFilter struct {
	SellerID string
	Status   string
	Category string
	Limit    int
	Offset   int
}

// ListingUpdate carries an editable subset of Listing fields. Pointer
// fields distinguish "not provided" from "set to zero value", which is
// what callers need when patching a record.
type ListingUpdate struct {
	Title       *string          `json:"title,omitempty"`
	Description *string          `json:"description,omitempty"`
	Category    *string          `json:"category,omitempty"`
	PriceCents  *int             `json:"price_cents,omitempty"`
	Tags        *json.RawMessage `json:"tags,omitempty"`
	Status      *ListingStatus   `json:"status,omitempty"`
}
