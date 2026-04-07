package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type PurchaseHandler struct {
	svc *service.PurchaseService
}

func (h *PurchaseHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req InitiatePurchaseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Initiate(r.Context(), req.BuyerID, req.ListingID)
	if err != nil {
		if err.Error() == "listing not found" {
			respondError(w, http.StatusNotFound, "listing not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, InitiatePurchaseResponse(*resp))
}

func (h *PurchaseHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ownership, err := h.svc.Confirm(r.Context(), id)
	if err != nil {
		if err.Error() == "transaction not found" {
			respondError(w, http.StatusNotFound, "transaction not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ConfirmPurchaseResponse(*ownership))
}

func (h *PurchaseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txn, err := h.svc.GetTransaction(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if txn == nil {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}
	respondJSON(w, http.StatusOK, GetPurchaseResponse(*txn))
}

func (h *PurchaseHandler) ListOwnership(w http.ResponseWriter, r *http.Request) {
	req := parseListOwnershipRequest(r)
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ownerships, err := h.svc.ListOwnership(r.Context(), req.BuyerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ListOwnershipResponse(ownerships))
}

func (h *PurchaseHandler) Download(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listingID")
	req := parseDownloadOwnershipRequest(r)
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	url, err := h.svc.DownloadURL(r.Context(), req.BuyerID, listingID)
	if err != nil {
		if err.Error() == "not owned" {
			respondError(w, http.StatusForbidden, "you do not own this listing")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, DownloadOwnershipResponse{DownloadURL: url})
}

