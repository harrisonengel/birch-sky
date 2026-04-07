package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/jmoiron/sqlx"
)

type TransactionRepo struct {
	db *sqlx.DB
}

func NewTransactionRepo(db *sqlx.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) Create(ctx context.Context, t *domain.Transaction) (*domain.Transaction, error) {
	result := &domain.Transaction{}
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO transactions (buyer_id, listing_id, amount_cents, currency, stripe_payment_id, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, buyer_id, listing_id, amount_cents, currency, stripe_payment_id, status, created_at, completed_at`,
		t.BuyerID, t.ListingID, t.AmountCents, t.Currency, t.StripePaymentID, t.Status,
	).StructScan(result)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	return result, nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	txn := &domain.Transaction{}
	err := r.db.GetContext(ctx, txn,
		`SELECT id, buyer_id, listing_id, amount_cents, currency, stripe_payment_id, status, created_at, completed_at
		 FROM transactions WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	return txn, nil
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	query := `UPDATE transactions SET status = $1`
	if status == domain.TransactionStatusCompleted {
		query += `, completed_at = now()`
	}
	query += ` WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}
