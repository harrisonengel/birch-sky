package service

import (
	"context"
	"fmt"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
)

type BuyOrderService struct {
	buyOrderRepo *postgres.BuyOrderRepo
	listingRepo  *postgres.ListingRepo
}

func NewBuyOrderService(buyOrderRepo *postgres.BuyOrderRepo, listingRepo *postgres.ListingRepo) *BuyOrderService {
	return &BuyOrderService{
		buyOrderRepo: buyOrderRepo,
		listingRepo:  listingRepo,
	}
}

func (s *BuyOrderService) Create(ctx context.Context, bo *domain.BuyOrder) (*domain.BuyOrder, error) {
	if bo.Status == "" {
		bo.Status = domain.BuyOrderStatusOpen
	}
	if bo.Currency == "" {
		bo.Currency = "usd"
	}
	return s.buyOrderRepo.Create(ctx, bo)
}

func (s *BuyOrderService) Get(ctx context.Context, id string) (*domain.BuyOrder, error) {
	return s.buyOrderRepo.GetByID(ctx, id)
}

func (s *BuyOrderService) List(ctx context.Context, filter domain.BuyOrderFilter) ([]domain.BuyOrder, int, error) {
	return s.buyOrderRepo.List(ctx, filter)
}

func (s *BuyOrderService) Fill(ctx context.Context, orderID, listingID string) (*domain.BuyOrder, error) {
	order, err := s.buyOrderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("buy order not found")
	}
	if order.Status != domain.BuyOrderStatusOpen {
		return nil, fmt.Errorf("buy order is %s, cannot fill", order.Status)
	}

	// Verify listing exists
	listing, err := s.listingRepo.GetByID(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if listing == nil {
		return nil, fmt.Errorf("listing not found")
	}

	if err := s.buyOrderRepo.Fill(ctx, orderID, listingID); err != nil {
		return nil, err
	}

	return s.buyOrderRepo.GetByID(ctx, orderID)
}

func (s *BuyOrderService) Cancel(ctx context.Context, id string) error {
	order, err := s.buyOrderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if order == nil {
		return fmt.Errorf("buy order not found")
	}
	if order.Status != domain.BuyOrderStatusOpen {
		return fmt.Errorf("buy order is %s, cannot cancel", order.Status)
	}
	return s.buyOrderRepo.UpdateStatus(ctx, id, domain.BuyOrderStatusCancelled)
}
