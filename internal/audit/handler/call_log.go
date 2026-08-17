package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listCallLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.service.ListCallLogs(r.Context(), 500)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func (h *Handler) listMyCallLogs(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	logs, err := h.service.ListMyCallLogs(r.Context(), principal.MemberID, 200)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}
