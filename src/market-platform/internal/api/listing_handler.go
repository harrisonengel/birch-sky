package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type SellerHandler struct {
	svc *service.ListingService
}

func (h *SellerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.Email == "" {
		respondError(w, http.StatusBadRequest, "name and email are required")
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
	respondJSON(w, http.StatusCreated, seller)
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
	respondJSON(w, http.StatusOK, seller)
}

type ListingHandler struct {
	svc *service.ListingService
}

func (h *ListingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SellerID    string          `json:"seller_id"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Category    string          `json:"category"`
		PriceCents  int             `json:"price_cents"`
		Currency    string          `json:"currency"`
		Tags        json.RawMessage `json:"tags"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.SellerID == "" {
		respondError(w, http.StatusBadRequest, "seller_id is required")
		return
	}
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Description == "" {
		respondError(w, http.StatusBadRequest, "description is required")
		return
	}
	if req.PriceCents < 0 {
		respondError(w, http.StatusBadRequest, "price_cents must be non-negative")
		return
	}

	listing, err := h.svc.CreateListing(r.Context(), req.SellerID, req.Title, req.Description, req.Category, req.PriceCents, req.Currency, req.Tags)
	if err != nil {
		if err.Error() == "seller not found" {
			respondError(w, http.StatusBadRequest, "seller not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, listing)
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
	respondJSON(w, http.StatusOK, listing)
}

func (h *ListingHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.ListingFilter{
		SellerID: queryString(r, "seller_id"),
		Status:   queryString(r, "status"),
		Category: queryString(r, "category"),
		Limit:    queryInt(r, "limit", 20),
		Offset:   queryInt(r, "offset", 0),
	}

	listings, total, err := h.svc.ListListings(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:   listings,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (h *ListingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updates map[string]interface{}
	if err := decodeJSON(r, &updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	listing, err := h.svc.UpdateListing(r.Context(), id, updates)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if listing == nil {
		respondError(w, http.StatusNotFound, "listing not found")
		return
	}
	respondJSON(w, http.StatusOK, listing)
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

	respondJSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
}
