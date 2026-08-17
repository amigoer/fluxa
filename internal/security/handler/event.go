package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
)

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context(), 200)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, events)
}
