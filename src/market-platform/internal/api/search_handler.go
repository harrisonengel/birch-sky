package api

import (
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type SearchHandler struct {
	svc *service.TurnMarketService
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := validateSearchRequest(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, SearchResponse(*resp))
}
