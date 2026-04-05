package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
)

type ListingService struct {
	listingRepo *postgres.ListingRepo
	sellerRepo  *postgres.SellerRepo
	objStore    objectstore.ObjectStore
	indexer     *search.Indexer
}

func NewListingService(listingRepo *postgres.ListingRepo, sellerRepo *postgres.SellerRepo, objStore objectstore.ObjectStore, indexer *search.Indexer) *ListingService {
	return &ListingService{
		listingRepo: listingRepo,
		sellerRepo:  sellerRepo,
		objStore:    objStore,
		indexer:     indexer,
	}
}

func (s *ListingService) CreateSeller(ctx context.Context, name, email string) (*domain.Seller, error) {
	existing, err := s.sellerRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("seller with email %s already exists", email)
	}
	return s.sellerRepo.Create(ctx, name, email)
}

func (s *ListingService) GetSeller(ctx context.Context, id string) (*domain.Seller, error) {
	return s.sellerRepo.GetByID(ctx, id)
}

func (s *ListingService) CreateListing(ctx context.Context, sellerID, title, description, category string, priceCents int, currency string, tags json.RawMessage) (*domain.Listing, error) {
	// Validate seller exists
	seller, err := s.sellerRepo.GetByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if seller == nil {
		return nil, fmt.Errorf("seller not found")
	}

	if tags == nil {
		tags = json.RawMessage("[]")
	}
	if currency == "" {
		currency = "usd"
	}

	listing := &domain.Listing{
		SellerID:    sellerID,
		Title:       title,
		Description: description,
		Category:    category,
		PriceCents:  priceCents,
		Currency:    currency,
		Tags:        tags,
		Status:      domain.ListingStatusActive,
	}

	created, err := s.listingRepo.Create(ctx, listing)
	if err != nil {
		return nil, err
	}

	// Index in OpenSearch (no data file yet, so no content_text)
	if err := s.indexer.IndexListing(ctx, created, nil, ""); err != nil {
		// Log but don't fail the creation
		fmt.Printf("warning: failed to index listing %s: %v\n", created.ID, err)
	}

	return created, nil
}

func (s *ListingService) GetListing(ctx context.Context, id string) (*domain.Listing, error) {
	return s.listingRepo.GetByID(ctx, id)
}

func (s *ListingService) ListListings(ctx context.Context, filter domain.ListingFilter) ([]domain.Listing, int, error) {
	return s.listingRepo.List(ctx, filter)
}

func (s *ListingService) UpdateListing(ctx context.Context, id string, updates map[string]interface{}) (*domain.Listing, error) {
	updated, err := s.listingRepo.Update(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, nil
	}

	// Re-index
	if err := s.indexer.IndexListing(ctx, updated, nil, ""); err != nil {
		fmt.Printf("warning: failed to re-index listing %s: %v\n", id, err)
	}

	return updated, nil
}

func (s *ListingService) DeleteListing(ctx context.Context, id string) error {
	if err := s.listingRepo.SoftDelete(ctx, id); err != nil {
		return err
	}

	// Remove from search index
	if err := s.indexer.Engine().DeleteListing(ctx, id); err != nil {
		fmt.Printf("warning: failed to remove listing %s from index: %v\n", id, err)
	}

	return nil
}

func (s *ListingService) UploadData(ctx context.Context, listingID, bucket string, reader io.ReadSeeker, size int64, contentType, filename string) error {
	listing, err := s.listingRepo.GetByID(ctx, listingID)
	if err != nil {
		return err
	}
	if listing == nil {
		return fmt.Errorf("listing not found")
	}

	key := fmt.Sprintf("listings/%s/%s", listingID, filename)

	if err := s.objStore.Upload(ctx, bucket, key, reader, size, contentType); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	// Determine format from content type
	dataFormat := contentType
	if err := s.listingRepo.UpdateDataRef(ctx, listingID, key, dataFormat, size); err != nil {
		return err
	}

	// Re-read data for content extraction and re-index
	reader.Seek(0, io.SeekStart)
	listing.DataRef = key
	listing.DataFormat = dataFormat
	listing.DataSizeBytes = size

	if err := s.indexer.IndexListing(ctx, listing, reader, dataFormat); err != nil {
		fmt.Printf("warning: failed to re-index listing %s after upload: %v\n", listingID, err)
	}

	return nil
}
