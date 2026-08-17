package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/quota"
	"github.com/amigoer/fluxa/internal/provider/repo"
)

func (h *Handler) getDepartmentQuotaPool(w http.ResponseWriter, r *http.Request) {
	departmentID := chi.URLParam(r, "departmentId")
	balance, err := h.service.DepartmentQuotaBalance(r.Context(), departmentID)
	if errors.Is(err, repo.ErrNotFound) {
		// A department with no pool yet is an empty pool, not an error --
		// but it has to serialize like every other balance, so the client
		// isn't parsing two different shapes off one endpoint.
		httpx.JSON(w, http.StatusOK, quota.Balance{})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, balance)
}

type setQuotaPoolRequest struct {
	TotalCents int64 `json:"totalCents"`
}

func (h *Handler) setDepartmentQuotaPool(w http.ResponseWriter, r *http.Request) {
	departmentID := chi.URLParam(r, "departmentId")
	var req setQuotaPoolRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.SetDepartmentQuotaPool(r.Context(), departmentID, req.TotalCents); err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, nil)
}
