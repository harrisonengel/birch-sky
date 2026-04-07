package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/jmoiron/sqlx"
)

type OwnershipRepo struct {
	db *sqlx.DB
}

func NewOwnershipRepo(db *sqlx.DB) *OwnershipRepo {
	return &OwnershipRepo{db: db}
}

func (r *OwnershipRepo) Create(ctx context.Context, o *domain.Ownership) (*domain.Ownership, error) {
	result := &domain.Ownership{}
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO ownership (buyer_id, listing_id, transaction_id, data_ref)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, buyer_id, listing_id, transaction_id, data_ref, acquired_at`,
		o.BuyerID, o.ListingID, o.TransactionID, o.DataRef,
	).StructScan(result)
	if err != nil {
		return nil, fmt.Errorf("create ownership: %w", err)
	}
	return result, nil
}

func (r *OwnershipRepo) Exists(ctx context.Context, buyerID, listingID string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM ownership WHERE buyer_id = $1 AND listing_id = $2`,
		buyerID, listingID)
	if err != nil {
		return false, fmt.Errorf("check ownership: %w", err)
	}
	return count > 0, nil
}

func (r *OwnershipRepo) ListByBuyer(ctx context.Context, buyerID string) ([]domain.Ownership, error) {
	var ownerships []domain.Ownership
	err := r.db.SelectContext(ctx, &ownerships,
		`SELECT id, buyer_id, listing_id, transaction_id, data_ref, acquired_at
		 FROM ownership WHERE buyer_id = $1 ORDER BY acquired_at DESC`, buyerID)
	if err != nil {
		return nil, fmt.Errorf("list ownership: %w", err)
	}
	if ownerships == nil {
		ownerships = []domain.Ownership{}
	}
	return ownerships, nil
}

func (r *OwnershipRepo) GetByBuyerAndListing(ctx context.Context, buyerID, listingID string) (*domain.Ownership, error) {
	o := &domain.Ownership{}
	err := r.db.GetContext(ctx, o,
		`SELECT id, buyer_id, listing_id, transaction_id, data_ref, acquired_at
		 FROM ownership WHERE buyer_id = $1 AND listing_id = $2`, buyerID, listingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ownership: %w", err)
	}
	return o, nil
}
