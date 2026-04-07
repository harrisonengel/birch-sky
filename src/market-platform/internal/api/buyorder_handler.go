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

// Create handles POST /api/v1/buy-orders.
//
// NOTE: The buyer_id in the request body is trusted on faith today. A
// real auth flow (OAuth or similar) that ties the request to an
// authenticated buyer identity is required before MVP; see the follow-up
// GitHub issue on buyer authentication.
func (h *BuyOrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBuyOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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
	respondJSON(w, http.StatusCreated, CreateBuyOrderResponse(*created))
}

func (h *BuyOrderHandler) List(w http.ResponseWriter, r *http.Request) {
	req := parseListBuyOrdersRequest(r)
	filter := domain.BuyOrderFilter{
		BuyerID:  req.BuyerID,
		Status:   req.Status,
		Category: req.Category,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	orders, total, err := h.svc.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ListBuyOrdersResponse{
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
	respondJSON(w, http.StatusOK, GetBuyOrderResponse(*order))
}

func (h *BuyOrderHandler) Fill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req FillBuyOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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
	respondJSON(w, http.StatusOK, FillBuyOrderResponse(*order))
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
