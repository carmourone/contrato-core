package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type ModelVersionsHandler struct{ Store storage.Store }

func (h *ModelVersionsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	modelID := r.URL.Query().Get("model_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	mvs, _, err := h.Store.ModelVersions().List(r.Context(), tenantID, modelID, storage.Page{Limit: 50})
	if err != nil {
		respond.InternalError(w, err)
		return
	}
	respond.OK(w, mvs)
}

func (h *ModelVersionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var mv storage.ModelVersion
	if err := json.NewDecoder(r.Body).Decode(&mv); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	created, err := h.Store.ModelVersions().Create(r.Context(), mv)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.Created(w, created)
}

func (h *ModelVersionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	modelID := chi.URLParam(r, "model_id")
	if tenantID == "" || modelID == "" {
		respond.BadRequest(w, "tenant_id and model_id required")
		return
	}
	mvs, _, err := h.Store.ModelVersions().List(r.Context(), tenantID, modelID, storage.Page{Limit: 1})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	if len(mvs) == 0 {
		respond.NotFound(w)
		return
	}
	respond.OK(w, mvs[0])
}

func (h *ModelVersionsHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID string `json:"tenant_id"`
		ModelID  string `json:"model_id"`
		Version  int    `json:"version"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}

	// fetch current then re-create with new status — append-only means we insert a new version
	mv, err := h.Store.ModelVersions().Get(r.Context(), body.TenantID, body.ModelID, body.Version)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	mv.Status = body.Status
	updated, err := h.Store.ModelVersions().Create(r.Context(), mv)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, updated)
}
