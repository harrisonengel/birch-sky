package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type SellerHandler struct {
	svc *service.ListingService
}

func (h *SellerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateSellerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	seller, err := h.svc.CreateSeller(r.Context(), req.Name, req.Email)
	if err != nil {
		if err.Error() == "seller with email "+req.Email+" already exists" {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, CreateSellerResponse(*seller))
}

func (h *SellerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	seller, err := h.svc.GetSeller(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if seller == nil {
		respondError(w, http.StatusNotFound, "seller not found")
		return
	}
	respondJSON(w, http.StatusOK, GetSellerResponse(*seller))
}

type ListingHandler struct {
	svc *service.ListingService
}

func (h *ListingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateListingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	listing, err := h.svc.CreateListing(r.Context(), service.CreateListingInput{
		SellerID:    req.SellerID,
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		PriceCents:  req.PriceCents,
		Currency:    req.Currency,
		Tags:        req.Tags,
	})
	if err != nil {
		if err.Error() == "seller not found" {
			respondError(w, http.StatusBadRequest, "seller not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, CreateListingResponse(*listing))
}

func (h *ListingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	listing, err := h.svc.GetListing(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if listing == nil {
		respondError(w, http.StatusNotFound, "listing not found")
		return
	}
	respondJSON(w, http.StatusOK, GetListingResponse(*listing))
}

func (h *ListingHandler) List(w http.ResponseWriter, r *http.Request) {
	req := parseListListingsRequest(r)
	filter := domain.ListingFilter{
		SellerID: req.SellerID,
		Status:   req.Status,
		Category: req.Category,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	listings, total, err := h.svc.ListListings(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ListListingsResponse{
		Data:   listings,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (h *ListingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateListingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	listing, err := h.svc.UpdateListing(r.Context(), id, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if listing == nil {
		respondError(w, http.StatusNotFound, "listing not found")
		return
	}
	respondJSON(w, http.StatusOK, UpdateListingResponse(*listing))
}

func (h *ListingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteListing(r.Context(), id); err != nil {
		respondError(w, http.StatusNotFound, "listing not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ListingHandler) Upload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// 100MB max
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	bucket := "market-data"
	if err := h.svc.UploadData(r.Context(), id, bucket, file, header.Size, header.Header.Get("Content-Type"), header.Filename); err != nil {
		if err.Error() == "listing not found" {
			respondError(w, http.StatusNotFound, "listing not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, UploadListingResponse{Status: "uploaded"})
}
