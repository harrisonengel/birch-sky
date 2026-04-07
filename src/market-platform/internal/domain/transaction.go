package domain

import "time"

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

type Transaction struct {
	ID              string            `db:"id" json:"id"`
	BuyerID         string            `db:"buyer_id" json:"buyer_id"`
	ListingID       string            `db:"listing_id" json:"listing_id"`
	AmountCents     int               `db:"amount_cents" json:"amount_cents"`
	Currency        string            `db:"currency" json:"currency"`
	StripePaymentID *string           `db:"stripe_payment_id" json:"stripe_payment_id,omitempty"`
	Status          TransactionStatus `db:"status" json:"status"`
	CreatedAt       time.Time         `db:"created_at" json:"created_at"`
	CompletedAt     *time.Time        `db:"completed_at" json:"completed_at,omitempty"`
}

type Ownership struct {
	ID            string    `db:"id" json:"id"`
	BuyerID       string    `db:"buyer_id" json:"buyer_id"`
	ListingID     string    `db:"listing_id" json:"listing_id"`
	TransactionID string    `db:"transaction_id" json:"transaction_id"`
	DataRef       string    `db:"data_ref" json:"data_ref"`
	AcquiredAt    time.Time `db:"acquired_at" json:"acquired_at"`
}
