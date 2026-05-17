package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type EmbeddingsHandler struct{ Store storage.Store }

type setEmbeddingRequest struct {
	Model     string    `json:"model"`
	Embedding []float32 `json:"embedding"`
}

type searchRequest struct {
	Embedding []float32 `json:"embedding"`
	Limit     int       `json:"limit"`
}

func (h *EmbeddingsHandler) SetNodeEmbedding(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	nodeID := chi.URLParam(r, "id")

	var req setEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	if len(req.Embedding) == 0 {
		respond.BadRequest(w, "embedding must be a non-empty float array")
		return
	}

	if err := h.Store.Embeddings().SetNodeEmbedding(r.Context(), tenantID, nodeID, req.Model, req.Embedding); err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.NoContent(w)
}

func (h *EmbeddingsHandler) SearchNodes(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	limit := queryInt(r, "limit", 10)

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	if len(req.Embedding) == 0 {
		respond.BadRequest(w, "embedding must be a non-empty float array")
		return
	}
	if req.Limit > 0 {
		limit = req.Limit
	}

	matches, err := h.Store.Embeddings().SearchNodes(r.Context(), tenantID, req.Embedding, limit)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	if matches == nil {
		matches = []storage.NodeMatch{}
	}
	respond.OK(w, matches)
}

func (h *EmbeddingsHandler) SetEdgeEmbedding(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	fromID := chi.URLParam(r, "from")
	toID := chi.URLParam(r, "to")
	domain := chi.URLParam(r, "domain")
	typ := chi.URLParam(r, "type")

	var req setEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	if len(req.Embedding) == 0 {
		respond.BadRequest(w, "embedding must be a non-empty float array")
		return
	}

	if err := h.Store.Embeddings().SetEdgeEmbedding(r.Context(), tenantID, fromID, toID, domain, typ, req.Model, req.Embedding); err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.NoContent(w)
}

func (h *EmbeddingsHandler) SearchEdges(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	limit := queryInt(r, "limit", 10)

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	if len(req.Embedding) == 0 {
		respond.BadRequest(w, "embedding must be a non-empty float array")
		return
	}
	if req.Limit > 0 {
		limit = req.Limit
	}

	matches, err := h.Store.Embeddings().SearchEdges(r.Context(), tenantID, req.Embedding, limit)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	if matches == nil {
		matches = []storage.EdgeMatch{}
	}
	respond.OK(w, matches)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
