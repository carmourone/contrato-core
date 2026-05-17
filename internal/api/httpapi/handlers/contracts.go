package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type ContractsHandler struct{ Store storage.Store }

func (h *ContractsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID, domain, typ := q.Get("tenant_id"), q.Get("domain"), q.Get("type")
	if tenantID == "" || typ == "" {
		respond.BadRequest(w, "tenant_id and type required")
		return
	}
	recs, _, err := h.Store.Contracts().ListByType(r.Context(), tenantID, domain, typ, storage.Page{Limit: 50})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, recs)
}

func (h *ContractsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rec storage.ContractRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	created, err := h.Store.Contracts().Put(r.Context(), rec, storage.PutOptions{})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.Created(w, created)
}

func (h *ContractsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	id := chi.URLParam(r, "id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	rec, err := h.Store.Contracts().Get(r.Context(), tenantID, id)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, rec)
}

func (h *ContractsHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID        string `json:"tenant_id"`
		Status          string `json:"status"`
		ExpectedVersion *int   `json:"expected_version,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	id := chi.URLParam(r, "id")

	rec, err := h.Store.Contracts().Get(r.Context(), body.TenantID, id)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	rec.Status = body.Status
	updated, err := h.Store.Contracts().Put(r.Context(), rec, storage.PutOptions{ExpectedVersion: body.ExpectedVersion})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, updated)
}
