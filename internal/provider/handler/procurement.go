package handler

import (
	"net/http"

	"github.com/amigoer/fluxa/internal/platform/httpx"
	"github.com/amigoer/fluxa/internal/provider/types"
	"github.com/amigoer/fluxa/internal/rbac"
)

func (h *Handler) listProcurement(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	records, err := h.service.ListProcurementRecords(r.Context(), principal.OrgID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, records)
}

func (h *Handler) recordProcurement(w http.ResponseWriter, r *http.Request) {
	principal, _ := rbac.FromContext(r.Context())
	var rec types.ProcurementRecord
	if !decodeJSON(w, r, &rec) {
		return
	}
	rec.RecordedByMemberID = principal.MemberID
	created, err := h.service.RecordProcurement(r.Context(), rec)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}
