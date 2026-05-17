package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type PropertiesHandler struct{ Store storage.Store }

func (h *PropertiesHandler) Get(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID := q.Get("tenant_id")
	ownerType := chi.URLParam(r, "owner_type")
	ownerID := chi.URLParam(r, "owner_id")
	key := chi.URLParam(r, "key")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	p, err := h.Store.Properties().Get(r.Context(), tenantID, ownerType, ownerID, key)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, p)
}

func (h *PropertiesHandler) Put(w http.ResponseWriter, r *http.Request) {
	var p storage.Property
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	p.OwnerType = chi.URLParam(r, "owner_type")
	p.OwnerID = chi.URLParam(r, "owner_id")
	p.Key = chi.URLParam(r, "key")

	var opts storage.PutOptions
	if v := r.URL.Query().Get("expected_version"); v != "" {
		var ev int
		if err := json.Unmarshal([]byte(v), &ev); err == nil {
			opts.ExpectedVersion = &ev
		}
	}
	updated, err := h.Store.Properties().Put(r.Context(), p, opts)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, updated)
}
