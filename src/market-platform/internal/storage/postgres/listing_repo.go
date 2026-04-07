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

type ListingRepo struct {
	db *sqlx.DB
}

func NewListingRepo(db *sqlx.DB) *ListingRepo {
	return &ListingRepo{db: db}
}

func (r *ListingRepo) Create(ctx context.Context, l *domain.Listing) (*domain.Listing, error) {
	result := &domain.Listing{}
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO listings (seller_id, title, description, category, price_cents, currency, data_ref, data_format, data_size_bytes, tags, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, seller_id, title, description, category, price_cents, currency, data_ref, data_format, data_size_bytes, tags, status, created_at, updated_at`,
		l.SellerID, l.Title, l.Description, l.Category, l.PriceCents, l.Currency,
		l.DataRef, l.DataFormat, l.DataSizeBytes, l.Tags, l.Status,
	).StructScan(result)
	if err != nil {
		return nil, fmt.Errorf("create listing: %w", err)
	}
	return result, nil
}

func (r *ListingRepo) GetByID(ctx context.Context, id string) (*domain.Listing, error) {
	listing := &domain.Listing{}
	err := r.db.GetContext(ctx, listing,
		`SELECT id, seller_id, title, description, category, price_cents, currency, data_ref, data_format, data_size_bytes, tags, status, created_at, updated_at
		 FROM listings WHERE id = $1 AND status != 'deleted'`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get listing: %w", err)
	}
	return listing, nil
}

func (r *ListingRepo) List(ctx context.Context, f domain.ListingFilter) ([]domain.Listing, int, error) {
	where := []string{"status != 'deleted'"}
	args := []interface{}{}
	argIdx := 1

	if f.SellerID != "" {
		where = append(where, fmt.Sprintf("seller_id = $%d", argIdx))
		args = append(args, f.SellerID)
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
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM listings WHERE %s", whereClause)
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
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
		`SELECT id, seller_id, title, description, category, price_cents, currency, data_ref, data_format, data_size_bytes, tags, status, created_at, updated_at
		 FROM listings WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var listings []domain.Listing
	if err := r.db.SelectContext(ctx, &listings, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list listings: %w", err)
	}
	if listings == nil {
		listings = []domain.Listing{}
	}
	return listings, total, nil
}

func (r *ListingRepo) Update(ctx context.Context, id string, updates domain.ListingUpdate) (*domain.Listing, error) {
	sets := []string{}
	args := []interface{}{}
	argIdx := 1

	addSet := func(col string, val interface{}) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	if updates.Title != nil {
		addSet("title", *updates.Title)
	}
	if updates.Description != nil {
		addSet("description", *updates.Description)
	}
	if updates.Category != nil {
		addSet("category", *updates.Category)
	}
	if updates.PriceCents != nil {
		addSet("price_cents", *updates.PriceCents)
	}
	if updates.Tags != nil {
		addSet("tags", *updates.Tags)
	}
	if updates.Status != nil {
		addSet("status", *updates.Status)
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	query := fmt.Sprintf(
		`UPDATE listings SET %s WHERE id = $%d AND status != 'deleted'
		 RETURNING id, seller_id, title, description, category, price_cents, currency, data_ref, data_format, data_size_bytes, tags, status, created_at, updated_at`,
		strings.Join(sets, ", "), argIdx)
	args = append(args, id)

	listing := &domain.Listing{}
	err := r.db.QueryRowxContext(ctx, query, args...).StructScan(listing)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update listing: %w", err)
	}
	return listing, nil
}

func (r *ListingRepo) SoftDelete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE listings SET status = 'deleted', updated_at = now() WHERE id = $1 AND status != 'deleted'`, id)
	if err != nil {
		return fmt.Errorf("soft delete listing: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *ListingRepo) UpdateDataRef(ctx context.Context, id, dataRef, dataFormat string, sizeBytes int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE listings SET data_ref = $1, data_format = $2, data_size_bytes = $3, updated_at = now() WHERE id = $4`,
		dataRef, dataFormat, sizeBytes, id)
	if err != nil {
		return fmt.Errorf("update data ref: %w", err)
	}
	return nil
}
