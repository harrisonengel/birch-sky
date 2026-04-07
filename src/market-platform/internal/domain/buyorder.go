package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type BuyOrderStatus string

const (
	BuyOrderStatusOpen      BuyOrderStatus = "open"
	BuyOrderStatusFilled    BuyOrderStatus = "filled"
	BuyOrderStatusCancelled BuyOrderStatus = "cancelled"
	BuyOrderStatusExpired   BuyOrderStatus = "expired"
)

// BuyOrderCriteria is the structured form of the buyer's evaluation
// requirements attached to a buy order. It deliberately starts small but
// is a struct (not a stringly-typed blob) so we can grow it without
// breaking callers.
//
// In the future this struct will carry the full state of an agent session
// that the marketplace can spin up on its own infrastructure to evaluate
// candidate listings on the buyer's behalf — the AgentSessionState field
// is the placeholder for that payload.
type BuyOrderCriteria struct {
	// RequiredTags lists tags every candidate listing must carry.
	RequiredTags []string `json:"required_tags,omitempty"`
	// RequiredFormats restricts acceptable data formats (e.g. "csv", "json").
	RequiredFormats []string `json:"required_formats,omitempty"`
	// MinFreshnessDays, when non-zero, requires the listing's data to have
	// been updated within this many days.
	MinFreshnessDays int `json:"min_freshness_days,omitempty"`
	// AgentSessionState is reserved for the serialized agent session that
	// the platform will use to evaluate candidate listings. The exact shape
	// is owned by the buyer agent platform layer and is opaque to the
	// market service today.
	AgentSessionState json.RawMessage `json:"agent_session_state,omitempty"`
}

// Value implements driver.Valuer so the criteria can be persisted to a
// JSONB column.
func (c BuyOrderCriteria) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner so the criteria can be read back out of a
// JSONB column.
func (c *BuyOrderCriteria) Scan(src interface{}) error {
	if src == nil {
		*c = BuyOrderCriteria{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported type for BuyOrderCriteria: %T", src)
	}
	if len(b) == 0 {
		*c = BuyOrderCriteria{}
		return nil
	}
	return json.Unmarshal(b, c)
}

type BuyOrder struct {
	ID                string           `db:"id" json:"id"`
	BuyerID           string           `db:"buyer_id" json:"buyer_id"`
	Query             string           `db:"query" json:"query"`
	Criteria          BuyOrderCriteria `db:"criteria" json:"criteria"`
	MaxPriceCents     int              `db:"max_price_cents" json:"max_price_cents"`
	Currency          string           `db:"currency" json:"currency"`
	Category          string           `db:"category" json:"category"`
	Status            BuyOrderStatus   `db:"status" json:"status"`
	FilledByListingID *string          `db:"filled_by_listing_id" json:"filled_by_listing_id,omitempty"`
	CreatedAt         time.Time        `db:"created_at" json:"created_at"`
	ExpiresAt         *time.Time       `db:"expires_at" json:"expires_at,omitempty"`
}

type BuyOrderFilter struct {
	BuyerID  string
	Status   string
	Category string
	Limit    int
	Offset   int
}
