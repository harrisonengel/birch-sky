package service

import (
	"context"
	"fmt"
	"time"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/payments"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
)

type PurchaseService struct {
	transactionRepo *postgres.TransactionRepo
	ownershipRepo   *postgres.OwnershipRepo
	listingRepo     *postgres.ListingRepo
	payments        payments.PaymentProcessor
	objStore        objectstore.ObjectStore
	bucket          string
}

func NewPurchaseService(txnRepo *postgres.TransactionRepo, ownRepo *postgres.OwnershipRepo, listingRepo *postgres.ListingRepo, payments payments.PaymentProcessor, objStore objectstore.ObjectStore, bucket string) *PurchaseService {
	return &PurchaseService{
		transactionRepo: txnRepo,
		ownershipRepo:   ownRepo,
		listingRepo:     listingRepo,
		payments:        payments,
		objStore:        objStore,
		bucket:          bucket,
	}
}

type InitiatePurchaseResponse struct {
	TransactionID string `json:"transaction_id"`
	ClientSecret  string `json:"client_secret,omitempty"`
	AlreadyOwned  bool   `json:"already_owned"`
}

func (s *PurchaseService) Initiate(ctx context.Context, buyerID, listingID string) (*InitiatePurchaseResponse, error) {
	// Check listing exists
	listing, err := s.listingRepo.GetByID(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if listing == nil {
		return nil, fmt.Errorf("listing not found")
	}

	// Check already owned
	owned, err := s.ownershipRepo.Exists(ctx, buyerID, listingID)
	if err != nil {
		return nil, err
	}
	if owned {
		return &InitiatePurchaseResponse{AlreadyOwned: true}, nil
	}

	// Create Stripe payment intent
	clientSecret, paymentID, err := s.payments.CreatePaymentIntent(ctx, listing.PriceCents, listing.Currency)
	if err != nil {
		return nil, fmt.Errorf("payment: %w", err)
	}

	// Create pending transaction
	txn := &domain.Transaction{
		BuyerID:         buyerID,
		ListingID:       listingID,
		AmountCents:     listing.PriceCents,
		Currency:        listing.Currency,
		StripePaymentID: &paymentID,
		Status:          domain.TransactionStatusPending,
	}
	created, err := s.transactionRepo.Create(ctx, txn)
	if err != nil {
		return nil, err
	}

	return &InitiatePurchaseResponse{
		TransactionID: created.ID,
		ClientSecret:  clientSecret,
	}, nil
}

func (s *PurchaseService) Confirm(ctx context.Context, transactionID string) (*domain.Ownership, error) {
	txn, err := s.transactionRepo.GetByID(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	if txn == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if txn.Status == domain.TransactionStatusCompleted {
		// Idempotent: return existing ownership
		existing, err := s.ownershipRepo.GetByBuyerAndListing(ctx, txn.BuyerID, txn.ListingID)
		if err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Verify payment with Stripe
	if txn.StripePaymentID != nil {
		if err := s.payments.ConfirmPayment(ctx, *txn.StripePaymentID); err != nil {
			s.transactionRepo.UpdateStatus(ctx, transactionID, domain.TransactionStatusFailed)
			return nil, fmt.Errorf("payment confirmation failed: %w", err)
		}
	}

	// Get listing for data_ref
	listing, err := s.listingRepo.GetByID(ctx, txn.ListingID)
	if err != nil {
		return nil, err
	}

	// Record ownership
	ownership := &domain.Ownership{
		BuyerID:       txn.BuyerID,
		ListingID:     txn.ListingID,
		TransactionID: transactionID,
		DataRef:       listing.DataRef,
	}
	created, err := s.ownershipRepo.Create(ctx, ownership)
	if err != nil {
		return nil, err
	}

	// Mark transaction complete
	s.transactionRepo.UpdateStatus(ctx, transactionID, domain.TransactionStatusCompleted)

	return created, nil
}

func (s *PurchaseService) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return s.transactionRepo.GetByID(ctx, id)
}

func (s *PurchaseService) ListOwnership(ctx context.Context, buyerID string) ([]domain.Ownership, error) {
	return s.ownershipRepo.ListByBuyer(ctx, buyerID)
}

func (s *PurchaseService) DownloadURL(ctx context.Context, buyerID, listingID string) (string, error) {
	ownership, err := s.ownershipRepo.GetByBuyerAndListing(ctx, buyerID, listingID)
	if err != nil {
		return "", err
	}
	if ownership == nil {
		return "", fmt.Errorf("not owned")
	}

	url, err := s.objStore.PresignedGetURL(ctx, s.bucket, ownership.DataRef, 5*time.Minute)
	if err != nil {
		return "", err
	}
	return url, nil
}
