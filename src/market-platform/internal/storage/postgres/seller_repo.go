package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/jmoiron/sqlx"
)

type SellerRepo struct {
	db *sqlx.DB
}

func NewSellerRepo(db *sqlx.DB) *SellerRepo {
	return &SellerRepo{db: db}
}

func (r *SellerRepo) Create(ctx context.Context, name, email string) (*domain.Seller, error) {
	seller := &domain.Seller{}
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO sellers (name, email) VALUES ($1, $2) RETURNING id, name, email, created_at`,
		name, email,
	).StructScan(seller)
	if err != nil {
		return nil, fmt.Errorf("create seller: %w", err)
	}
	return seller, nil
}

func (r *SellerRepo) GetByID(ctx context.Context, id string) (*domain.Seller, error) {
	seller := &domain.Seller{}
	err := r.db.GetContext(ctx, seller, `SELECT id, name, email, created_at FROM sellers WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get seller: %w", err)
	}
	return seller, nil
}

func (r *SellerRepo) GetByEmail(ctx context.Context, email string) (*domain.Seller, error) {
	seller := &domain.Seller{}
	err := r.db.GetContext(ctx, seller, `SELECT id, name, email, created_at FROM sellers WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get seller by email: %w", err)
	}
	return seller, nil
}
