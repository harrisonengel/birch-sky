package helpers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
)

func CreateTestSeller(t *testing.T, repo *postgres.SellerRepo, name, email string) *domain.Seller {
	t.Helper()
	seller, err := repo.Create(context.Background(), name, email)
	if err != nil {
		t.Fatalf("create test seller: %v", err)
	}
	return seller
}

func CreateTestListing(t *testing.T, repo *postgres.ListingRepo, sellerID, title, description, category string, priceCents int) *domain.Listing {
	t.Helper()
	tags, _ := json.Marshal([]string{"test"})
	listing, err := repo.Create(context.Background(), &domain.Listing{
		SellerID:    sellerID,
		Title:       title,
		Description: description,
		Category:    category,
		PriceCents:  priceCents,
		Currency:    "usd",
		Tags:        tags,
		Status:      domain.ListingStatusActive,
	})
	if err != nil {
		t.Fatalf("create test listing: %v", err)
	}
	return listing
}

func CreateTestTransaction(t *testing.T, repo *postgres.TransactionRepo, buyerID, listingID string, amountCents int) *domain.Transaction {
	t.Helper()
	txn, err := repo.Create(context.Background(), &domain.Transaction{
		BuyerID:     buyerID,
		ListingID:   listingID,
		AmountCents: amountCents,
		Currency:    "usd",
		Status:      domain.TransactionStatusPending,
	})
	if err != nil {
		t.Fatalf("create test transaction: %v", err)
	}
	return txn
}

func CreateTestBuyOrder(t *testing.T, repo *postgres.BuyOrderRepo, buyerID, query, category string, maxPriceCents int) *domain.BuyOrder {
	t.Helper()
	bo, err := repo.Create(context.Background(), &domain.BuyOrder{
		BuyerID:       buyerID,
		Query:         query,
		Criteria:      domain.BuyOrderCriteria{},
		MaxPriceCents: maxPriceCents,
		Currency:      "usd",
		Category:      category,
		Status:        domain.BuyOrderStatusOpen,
	})
	if err != nil {
		t.Fatalf("create test buy order: %v", err)
	}
	return bo
}
