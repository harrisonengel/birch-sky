package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type BuyOrderHandler struct {
	svc *service.BuyOrderService
}

func (h *BuyOrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuyerID       string `json:"buyer_id"`
		Query         string `json:"query"`
		Criteria      string `json:"criteria"`
		MaxPriceCents int    `json:"max_price_cents"`
		Currency      string `json:"currency"`
		Category      string `json:"category"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.BuyerID == "" || req.Query == "" {
		respondError(w, http.StatusBadRequest, "buyer_id and query are required")
		return
	}
	if req.MaxPriceCents <= 0 {
		respondError(w, http.StatusBadRequest, "max_price_cents must be positive")
		return
	}

	bo := &domain.BuyOrder{
		BuyerID:       req.BuyerID,
		Query:         req.Query,
		Criteria:      req.Criteria,
		MaxPriceCents: req.MaxPriceCents,
		Currency:      req.Currency,
		Category:      req.Category,
	}

	created, err := h.svc.Create(r.Context(), bo)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (h *BuyOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.BuyOrderFilter{
		BuyerID:  queryString(r, "buyer_id"),
		Status:   queryString(r, "status"),
		Category: queryString(r, "category"),
		Limit:    queryInt(r, "limit", 20),
		Offset:   queryInt(r, "offset", 0),
	}

	orders, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:   orders,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (h *BuyOrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.svc.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		respondError(w, http.StatusNotFound, "buy order not found")
		return
	}
	respondJSON(w, http.StatusOK, order)
}

func (h *BuyOrderHandler) Fill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		ListingID string `json:"listing_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ListingID == "" {
		respondError(w, http.StatusBadRequest, "listing_id is required")
		return
	}

	order, err := h.svc.Fill(r.Context(), id, req.ListingID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			respondError(w, http.StatusNotFound, msg)
			return
		}
		if strings.Contains(msg, "cannot fill") {
			respondError(w, http.StatusConflict, msg)
			return
		}
		respondError(w, http.StatusInternalServerError, msg)
		return
	}
	respondJSON(w, http.StatusOK, order)
}

func (h *BuyOrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Cancel(r.Context(), id); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			respondError(w, http.StatusNotFound, msg)
			return
		}
		if strings.Contains(msg, "cannot cancel") {
			respondError(w, http.StatusConflict, msg)
			return
		}
		respondError(w, http.StatusInternalServerError, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
