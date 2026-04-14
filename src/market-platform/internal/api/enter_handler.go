package api

import (
	"net/http"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
)

type EnterHandler struct {
	svc *service.TurnMarketService
}

func (h *EnterHandler) Enter(w http.ResponseWriter, r *http.Request) {
	var req EnterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := validateEnterRequest(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Enter(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, EnterResponse(*resp))
}
