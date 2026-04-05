package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/jmoiron/sqlx"
)

type BuyOrderRepo struct {
	db *sqlx.DB
}

func NewBuyOrderRepo(db *sqlx.DB) *BuyOrderRepo {
	return &BuyOrderRepo{db: db}
}

func (r *BuyOrderRepo) Create(ctx context.Context, bo *domain.BuyOrder) (*domain.BuyOrder, error) {
	result := &domain.BuyOrder{}
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO buy_orders (buyer_id, query, criteria, max_price_cents, currency, category, status, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, buyer_id, query, criteria, max_price_cents, currency, category, status, filled_by_listing_id, created_at, expires_at`,
		bo.BuyerID, bo.Query, bo.Criteria, bo.MaxPriceCents, bo.Currency, bo.Category, bo.Status, bo.ExpiresAt,
	).StructScan(result)
	if err != nil {
		return nil, fmt.Errorf("create buy order: %w", err)
	}
	return result, nil
}

func (r *BuyOrderRepo) GetByID(ctx context.Context, id string) (*domain.BuyOrder, error) {
	bo := &domain.BuyOrder{}
	err := r.db.GetContext(ctx, bo,
		`SELECT id, buyer_id, query, criteria, max_price_cents, currency, category, status, filled_by_listing_id, created_at, expires_at
		 FROM buy_orders WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get buy order: %w", err)
	}
	return bo, nil
}

func (r *BuyOrderRepo) List(ctx context.Context, f domain.BuyOrderFilter) ([]domain.BuyOrder, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if f.BuyerID != "" {
		where = append(where, fmt.Sprintf("buyer_id = $%d", argIdx))
		args = append(args, f.BuyerID)
		argIdx++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, f.Category)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM buy_orders WHERE %s", whereClause), args...); err != nil {
		return nil, 0, fmt.Errorf("count buy orders: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(
		`SELECT id, buyer_id, query, criteria, max_price_cents, currency, category, status, filled_by_listing_id, created_at, expires_at
		 FROM buy_orders WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var orders []domain.BuyOrder
	if err := r.db.SelectContext(ctx, &orders, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list buy orders: %w", err)
	}
	if orders == nil {
		orders = []domain.BuyOrder{}
	}
	return orders, total, nil
}

func (r *BuyOrderRepo) UpdateStatus(ctx context.Context, id string, status domain.BuyOrderStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE buy_orders SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("update buy order status: %w", err)
	}
	return nil
}

func (r *BuyOrderRepo) Fill(ctx context.Context, id, listingID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE buy_orders SET status = 'filled', filled_by_listing_id = $1 WHERE id = $2 AND status = 'open'`,
		listingID, id)
	if err != nil {
		return fmt.Errorf("fill buy order: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("buy order not open or not found")
	}
	return nil
}
