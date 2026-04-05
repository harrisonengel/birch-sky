package api

import (
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type SearchHandler struct {
	svc *service.SearchService
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req service.SearchRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}
