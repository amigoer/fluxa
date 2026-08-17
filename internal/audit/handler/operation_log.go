package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
)

func (h *Handler) listOperationLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.ListOperationLogs(r.Context(), 500)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}
