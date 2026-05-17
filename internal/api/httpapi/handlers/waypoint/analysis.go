package waypoint

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/dominikbraun/graph"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

type StoreWithDB interface {
	storage.Store
	DB() *sql.DB
}

type Handler struct{ Store StoreWithDB }

type Gap struct {
	Kind           string `json:"kind"`
	CapabilityID   string `json:"capability_id,omitempty"`
	CapabilityType string `json:"capability_type,omitempty"`
	ProviderCount  int    `json:"provider_count,omitempty"`
	Description    string `json:"description"`
}

type GapsResponse struct {
	TenantID     string `json:"tenant_id"`
	ModelID      string `json:"model_id"`
	ModelVersion int    `json:"model_version"`
	Gaps         []Gap  `json:"gaps"`
}

func (h *Handler) Gaps(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID := q.Get("tenant_id")
	if tenantID == "" {
		respond.BadRequest(w, "tenant_id required")
		return
	}

	mv, err := h.Store.ModelVersions().GetLatestEnabled(r.Context(), tenantID)
	if err != nil {
		respond.StorageError(w, err)
		return
	}

	gaps, err := h.detectGaps(r.Context(), tenantID, mv)
	if err != nil {
		respond.InternalError(w, err)
		return
	}
	if gaps == nil {
		gaps = []Gap{}
	}
	respond.OK(w, GapsResponse{
		TenantID:     tenantID,
		ModelID:      mv.ModelID,
		ModelVersion: mv.Version,
		Gaps:         gaps,
	})
}

func (h *Handler) detectGaps(ctx context.Context, tenantID string, mv storage.ModelVersion) ([]Gap, error) {
	db := h.Store.DB()

	nRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  id, type
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenantID, mv.ModelID, mv.Version)
	if err != nil {
		return nil, err
	}
	defer nRows.Close()

	type node struct{ id, typ string }
	var nodes []node
	for nRows.Next() {
		var n node
		if err := nRows.Scan(&n.id, &n.typ); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := nRows.Err(); err != nil {
		return nil, err
	}

	eRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
  from_id, to_id, type
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
`, tenantID, mv.ModelID, mv.Version)
	if err != nil {
		return nil, err
	}
	defer eRows.Close()

	type edge struct{ from, to, typ string }
	providerCount := map[string]int{}

	for eRows.Next() {
		var e edge
		if err := eRows.Scan(&e.from, &e.to, &e.typ); err != nil {
			return nil, err
		}
		if e.typ == "provides" {
			providerCount[e.to]++
		}
	}
	if err := eRows.Err(); err != nil {
		return nil, err
	}

	pRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, owner_type, owner_id, key)
  owner_id, key, value
FROM properties
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3 AND owner_type='node'
ORDER BY tenant_id, owner_type, owner_id, key, version DESC
`, tenantID, mv.ModelID, mv.Version)
	if err != nil {
		return nil, err
	}
	defer pRows.Close()

	unavailable := map[string]bool{}
	for pRows.Next() {
		var ownerID, key string
		var valJSON []byte
		if err := pRows.Scan(&ownerID, &key, &valJSON); err != nil {
			return nil, err
		}
		if key == "availability" {
			var v any
			if err := json.Unmarshal(valJSON, &v); err == nil {
				if b, ok := v.(bool); ok && !b {
					unavailable[ownerID] = true
				}
			}
		}
	}

	var gaps []Gap
	for _, n := range nodes {
		if n.typ != "capability" {
			continue
		}
		pc := providerCount[n.id]
		switch {
		case pc == 0:
			gaps = append(gaps, Gap{
				Kind:           "no_provider",
				CapabilityID:   n.id,
				CapabilityType: n.typ,
				ProviderCount:  0,
				Description:    "No actor provides this capability",
			})
		case pc == 1:
			gaps = append(gaps, Gap{
				Kind:           "single_provider",
				CapabilityID:   n.id,
				CapabilityType: n.typ,
				ProviderCount:  1,
				Description:    "Only one provider — single point of failure",
			})
		}
		if unavailable[n.id] {
			gaps = append(gaps, Gap{
				Kind:          "unavailable",
				CapabilityID:  n.id,
				ProviderCount: pc,
				Description:   "Capability is marked unavailable",
			})
		}
	}

	return gaps, nil
}

type PathsResponse struct {
	TenantID     string     `json:"tenant_id"`
	ModelID      string     `json:"model_id"`
	ModelVersion int        `json:"model_version"`
	From         string     `json:"from"`
	To           string     `json:"to"`
	Paths        [][]string `json:"paths"`
}

func (h *Handler) Paths(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID, from, to := q.Get("tenant_id"), q.Get("from"), q.Get("to")
	if tenantID == "" || from == "" || to == "" {
		respond.BadRequest(w, "tenant_id, from, and to required")
		return
	}

	mv, err := h.Store.ModelVersions().GetLatestEnabled(r.Context(), tenantID)
	if err != nil {
		respond.StorageError(w, err)
		return
	}

	db := h.Store.DB()

	nRows, err := db.QueryContext(r.Context(), `
SELECT DISTINCT ON (tenant_id, id) id
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenantID, mv.ModelID, mv.Version)
	if err != nil {
		respond.InternalError(w, err)
		return
	}
	defer nRows.Close()

	g := graph.New(func(s string) string { return s }, graph.Directed())
	for nRows.Next() {
		var id string
		if err := nRows.Scan(&id); err != nil {
			respond.InternalError(w, err)
			return
		}
		_ = g.AddVertex(id)
	}
	if err := nRows.Err(); err != nil {
		respond.InternalError(w, err)
		return
	}

	eRows, err := db.QueryContext(r.Context(), `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
  from_id, to_id
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
`, tenantID, mv.ModelID, mv.Version)
	if err != nil {
		respond.InternalError(w, err)
		return
	}
	defer eRows.Close()

	for eRows.Next() {
		var f, t string
		if err := eRows.Scan(&f, &t); err != nil {
			respond.InternalError(w, err)
			return
		}
		if err := g.AddEdge(f, t); err != nil && err != graph.ErrEdgeAlreadyExists {
			respond.InternalError(w, err)
			return
		}
	}
	if err := eRows.Err(); err != nil {
		respond.InternalError(w, err)
		return
	}

	paths, err := graph.AllPathsBetween(g, from, to)
	if err != nil {
		// no path found is not an error — return empty
		paths = [][]string{}
	}
	if paths == nil {
		paths = [][]string{}
	}
	respond.OK(w, PathsResponse{
		TenantID:     tenantID,
		ModelID:      mv.ModelID,
		ModelVersion: mv.Version,
		From:         from,
		To:           to,
		Paths:        paths,
	})
}
