package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type GraphHandler struct{ Store storage.Store }

// --- Nodes ---

func (h *GraphHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID, modelID := q.Get("tenant_id"), q.Get("model_id")
	if tenantID == "" || modelID == "" {
		respond.BadRequest(w, "tenant_id and model_id required")
		return
	}
	// use export-style query via BeginTx for read-only consistency
	// For now, get a single node list via the graph repo's OutEdges isn't right;
	// model-scoped node listing goes through the DB directly.
	// We re-use the store's underlying DB access via a read tx.
	tx, err := h.Store.BeginTx(r.Context(), storage.TxOptions{ReadOnly: true})
	if err != nil {
		respond.InternalError(w, err)
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	nodeID := q.Get("id")
	if nodeID != "" {
		n, err := tx.Graph().GetNode(r.Context(), tenantID, nodeID)
		if err != nil {
			respond.StorageError(w, err)
			return
		}
		respond.OK(w, n)
		return
	}
	respond.Error(w, http.StatusNotImplemented, "list all nodes requires model_id cursor paging (not yet implemented)")
}

func (h *GraphHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var n storage.Node
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	created, err := h.Store.Graph().PutNode(r.Context(), n, storage.PutOptions{})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.Created(w, created)
}

func (h *GraphHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	id := chi.URLParam(r, "id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}
	n, err := h.Store.Graph().GetNode(r.Context(), tenantID, id)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, n)
}

func (h *GraphHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	var n storage.Node
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	n.ID = chi.URLParam(r, "id")
	var opts storage.PutOptions
	if v := r.URL.Query().Get("expected_version"); v != "" {
		var ev int
		if err := json.Unmarshal([]byte(v), &ev); err == nil {
			opts.ExpectedVersion = &ev
		}
	}
	updated, err := h.Store.Graph().PutNode(r.Context(), n, opts)
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, updated)
}

// --- Edges ---

func (h *GraphHandler) ListEdges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID, fromID, domain, typ := q.Get("tenant_id"), q.Get("from"), q.Get("domain"), q.Get("type")
	if tenantID == "" || fromID == "" {
		respond.BadRequest(w, "tenant_id and from required")
		return
	}
	edges, _, err := h.Store.Graph().OutEdges(r.Context(), tenantID, fromID, domain, typ, storage.Page{Limit: 100})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, edges)
}

func (h *GraphHandler) CreateEdge(w http.ResponseWriter, r *http.Request) {
	var e storage.Edge
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	created, err := h.Store.Graph().PutEdge(r.Context(), e, storage.PutOptions{})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.Created(w, created)
}

func (h *GraphHandler) UpdateEdge(w http.ResponseWriter, r *http.Request) {
	var e storage.Edge
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	e.FromID = chi.URLParam(r, "from")
	e.ToID = chi.URLParam(r, "to")
	e.Type = chi.URLParam(r, "type")
	updated, err := h.Store.Graph().PutEdge(r.Context(), e, storage.PutOptions{})
	if err != nil {
		respond.StorageError(w, err)
		return
	}
	respond.OK(w, updated)
}
