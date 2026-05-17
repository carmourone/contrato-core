package frontier

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/dominikbraun/graph"

	"contrato/internal/api/httpapi/respond"
	"contrato/internal/storage"
)

// StoreWithDB gives the frontier handler access to raw SQL for bulk graph reads.
type StoreWithDB interface {
	storage.Store
	DB() *sql.DB
}

type Handler struct{ Store StoreWithDB }

type gNode struct {
	ID     string
	Type   string
	Domain string
	Blob   map[string]any
}

type gEdge struct {
	FromID string
	ToID   string
	Type   string
	Domain string
}

func (h *Handler) loadGraph(ctx context.Context, tenantID, modelID string, modelVersion int) ([]gNode, []gEdge, error) {
	db := h.Store.DB()

	nRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id) id, domain, type, blob
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenantID, modelID, modelVersion)
	if err != nil {
		return nil, nil, err
	}
	defer nRows.Close()
	var nodes []gNode
	for nRows.Next() {
		var n gNode
		var blob []byte
		if err := nRows.Scan(&n.ID, &n.Domain, &n.Type, &blob); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(blob, &n.Blob)
		nodes = append(nodes, n)
	}
	if err := nRows.Err(); err != nil {
		return nil, nil, err
	}

	eRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type) from_id, to_id, domain, type
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
`, tenantID, modelID, modelVersion)
	if err != nil {
		return nil, nil, err
	}
	defer eRows.Close()
	var edges []gEdge
	for eRows.Next() {
		var e gEdge
		if err := eRows.Scan(&e.FromID, &e.ToID, &e.Domain, &e.Type); err != nil {
			return nil, nil, err
		}
		edges = append(edges, e)
	}
	return nodes, edges, eRows.Err()
}

func buildGraph(nodes []gNode, edges []gEdge) (graph.Graph[string, gNode], error) {
	g := graph.New(func(n gNode) string { return n.ID }, graph.Directed())
	for _, n := range nodes {
		if err := g.AddVertex(n); err != nil {
			return nil, err
		}
	}
	for _, e := range edges {
		// ignore duplicate edges — multigraph via type is represented as attributes
		if err := g.AddEdge(e.FromID, e.ToID,
			graph.EdgeAttribute("type", e.Type),
			graph.EdgeAttribute("domain", e.Domain),
		); err != nil && err != graph.ErrEdgeAlreadyExists {
			return nil, err
		}
	}
	return g, nil
}

// --- Hole detection ---

type Hole struct {
	Kind        string `json:"kind"`
	NodeID      string `json:"node_id,omitempty"`
	NodeType    string `json:"node_type,omitempty"`
	Description string `json:"description"`
}

type HolesResponse struct {
	TenantID     string `json:"tenant_id"`
	ModelID      string `json:"model_id"`
	ModelVersion int    `json:"model_version"`
	Holes        []Hole `json:"holes"`
}

func (h *Handler) Holes(w http.ResponseWriter, r *http.Request) {
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

	nodes, edges, err := h.loadGraph(r.Context(), tenantID, mv.ModelID, mv.Version)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	g, err := buildGraph(nodes, edges)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	holes := detectHoles(nodes, edges, g)
	respond.OK(w, HolesResponse{
		TenantID:     tenantID,
		ModelID:      mv.ModelID,
		ModelVersion: mv.Version,
		Holes:        holes,
	})
}

func detectHoles(nodes []gNode, edges []gEdge, g graph.Graph[string, gNode]) []Hole {
	var holes []Hole

	// index edges for quick lookup
	toEdges := map[string][]gEdge{}
	fromEdges := map[string][]gEdge{}
	for _, e := range edges {
		toEdges[e.ToID] = append(toEdges[e.ToID], e)
		fromEdges[e.FromID] = append(fromEdges[e.FromID], e)
	}

	adj, _ := g.AdjacencyMap()
	pred, _ := g.PredecessorMap()

	for _, n := range nodes {
		inDeg := len(pred[n.ID])
		outDeg := len(adj[n.ID])

		if inDeg == 0 && outDeg == 0 {
			holes = append(holes, Hole{
				Kind:        "isolated_node",
				NodeID:      n.ID,
				NodeType:    n.Type,
				Description: "Node has no edges (completely isolated)",
			})
			continue
		}

		if n.Type == "capability" {
			hasProvider := false
			for _, e := range toEdges[n.ID] {
				if e.Type == "provides" {
					hasProvider = true
					break
				}
			}
			if !hasProvider {
				holes = append(holes, Hole{
					Kind:        "capability_no_provider",
					NodeID:      n.ID,
					NodeType:    n.Type,
					Description: "Capability has no provider (no 'provides' edge pointing to it)",
				})
			}
			hasPolicy := false
			for _, e := range toEdges[n.ID] {
				if e.Type == "governs" {
					hasPolicy = true
					break
				}
			}
			if !hasPolicy {
				holes = append(holes, Hole{
					Kind:        "capability_no_policy",
					NodeID:      n.ID,
					NodeType:    n.Type,
					Description: "Capability has no governing policy (no 'governs' edge)",
				})
			}
		}

		if n.Type == "policy" {
			hasCap := false
			for _, e := range fromEdges[n.ID] {
				if e.Type == "governs" {
					hasCap = true
					break
				}
			}
			if !hasCap {
				holes = append(holes, Hole{
					Kind:        "policy_governs_nothing",
					NodeID:      n.ID,
					NodeType:    n.Type,
					Description: "Policy node governs no capability (dead policy)",
				})
			}
		}

		if n.Type == "actor" && outDeg == 0 {
			holes = append(holes, Hole{
				Kind:        "actor_no_capabilities",
				NodeID:      n.ID,
				NodeType:    n.Type,
				Description: "Actor has no outgoing edges (no capability assignments)",
			})
		}
	}

	// structural holes: SCCs with size 1 that bridge others are articulation-like.
	// Use StronglyConnectedComponents to find isolated singleton SCCs.
	sccs, err := graph.StronglyConnectedComponents(g)
	if err == nil {
		// find singleton SCCs that have both incoming and outgoing inter-SCC edges
		// (these are structural bridges/holes in the capability graph)
		sccOf := map[string]int{}
		for i, scc := range sccs {
			for _, id := range scc {
				sccOf[id] = i
			}
		}
		interEdges := map[string]bool{} // nodeID → bridges different SCCs
		for _, e := range edges {
			if sccOf[e.FromID] != sccOf[e.ToID] {
				interEdges[e.FromID] = true
				interEdges[e.ToID] = true
			}
		}
		for _, scc := range sccs {
			if len(scc) == 1 && interEdges[scc[0]] {
				v, _ := g.Vertex(scc[0])
				holes = append(holes, Hole{
					Kind:        "structural_hole",
					NodeID:      scc[0],
					NodeType:    v.Type,
					Description: "Bridges otherwise disconnected capability clusters",
				})
			}
		}
	}

	if holes == nil {
		holes = []Hole{}
	}
	return holes
}

// --- Boundary detection ---

type BoundaryNode struct {
	NodeID      string `json:"node_id"`
	NodeType    string `json:"node_type"`
	InDegree    int    `json:"in_degree"`
	OutDegree   int    `json:"out_degree"`
	Description string `json:"description"`
}

type BoundaryResponse struct {
	TenantID      string         `json:"tenant_id"`
	ModelID       string         `json:"model_id"`
	ModelVersion  int            `json:"model_version"`
	BoundaryNodes []BoundaryNode `json:"boundary_nodes"`
}

func (h *Handler) Boundary(w http.ResponseWriter, r *http.Request) {
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

	nodes, edges, err := h.loadGraph(r.Context(), tenantID, mv.ModelID, mv.Version)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	g, err := buildGraph(nodes, edges)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	boundary := detectBoundary(nodes, g)
	if boundary == nil {
		boundary = []BoundaryNode{}
	}
	respond.OK(w, BoundaryResponse{
		TenantID:      tenantID,
		ModelID:       mv.ModelID,
		ModelVersion:  mv.Version,
		BoundaryNodes: boundary,
	})
}

func detectBoundary(nodes []gNode, g graph.Graph[string, gNode]) []BoundaryNode {
	adj, _ := g.AdjacencyMap()
	pred, _ := g.PredecessorMap()

	var boundary []BoundaryNode
	for _, n := range nodes {
		inDeg := len(pred[n.ID])
		outDeg := len(adj[n.ID])
		if inDeg+outDeg > 1 {
			continue
		}
		desc := "Leaf node (degree 1)"
		if inDeg == 0 && outDeg == 1 {
			desc = "Source: no inbound edges — nothing leads to this node"
		} else if inDeg == 1 && outDeg == 0 {
			desc = "Sink: no outbound edges — nothing follows from this node"
		}
		boundary = append(boundary, BoundaryNode{
			NodeID:    n.ID,
			NodeType:  n.Type,
			InDegree:  inDeg,
			OutDegree: outDeg,
			Description: desc,
		})
	}
	return boundary
}

// --- Implied edge detection ---

type ImpliedEdge struct {
	FromID      string `json:"from_id"`
	ToID        string `json:"to_id"`
	Type        string `json:"type"`
	Basis       string `json:"basis"`
	ViaNodeID   string `json:"via_node_id,omitempty"`
	ViaNodeType string `json:"via_node_type,omitempty"`
}

type ImpliedEdgesResponse struct {
	TenantID     string        `json:"tenant_id"`
	ModelID      string        `json:"model_id"`
	ModelVersion int           `json:"model_version"`
	ImpliedEdges []ImpliedEdge `json:"implied_edges"`
}

func (h *Handler) ImpliedEdges(w http.ResponseWriter, r *http.Request) {
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

	nodes, edges, err := h.loadGraph(r.Context(), tenantID, mv.ModelID, mv.Version)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	// TransitiveReduction gives us the minimal graph with same reachability.
	// Edges present in the full graph but absent from the reduction are redundant.
	// Edges implied by transitivity (A→B→C but no A→C) are what we surface.
	g, err := buildGraph(nodes, edges)
	if err != nil {
		respond.InternalError(w, err)
		return
	}

	implied := detectImpliedEdges(nodes, edges, g)
	if implied == nil {
		implied = []ImpliedEdge{}
	}
	respond.OK(w, ImpliedEdgesResponse{
		TenantID:     tenantID,
		ModelID:      mv.ModelID,
		ModelVersion: mv.Version,
		ImpliedEdges: implied,
	})
}

func detectImpliedEdges(nodes []gNode, edges []gEdge, g graph.Graph[string, gNode]) []ImpliedEdge {
	type edgeKey struct{ from, to, typ string }
	edgeSet := make(map[edgeKey]bool, len(edges))
	nodeByID := make(map[string]gNode, len(nodes))

	provides := map[string][]string{}  // actor → []capabilityID
	requires := map[string][]string{}  // capabilityID → []capabilityID
	delegates := map[string][]string{} // actor → []actor

	for _, n := range nodes {
		nodeByID[n.ID] = n
	}
	for _, e := range edges {
		edgeSet[edgeKey{e.FromID, e.ToID, e.Type}] = true
		switch e.Type {
		case "provides":
			provides[e.FromID] = append(provides[e.FromID], e.ToID)
		case "requires":
			requires[e.FromID] = append(requires[e.FromID], e.ToID)
		case "can_delegate_to":
			delegates[e.FromID] = append(delegates[e.FromID], e.ToID)
		}
	}

	// Use AllPathsBetween from the library to verify reachability for candidate implied edges,
	// ensuring we only surface edges that are genuinely implied by multi-hop paths.
	var implied []ImpliedEdge

	// Rule: actor -[provides]-> capability -[requires]-> dep
	//       implies actor -[provides]-> dep
	for actor, caps := range provides {
		for _, cap := range caps {
			for _, dep := range requires[cap] {
				k := edgeKey{actor, dep, "provides"}
				if edgeSet[k] {
					continue
				}
				// confirm the path actually exists in the graph
				paths, err := graph.AllPathsBetween(g, actor, dep)
				if err != nil || len(paths) == 0 {
					continue
				}
				via := nodeByID[cap]
				implied = append(implied, ImpliedEdge{
					FromID:      actor,
					ToID:        dep,
					Type:        "provides",
					ViaNodeID:   cap,
					ViaNodeType: via.Type,
					Basis:       "provides a capability that requires this dependency",
				})
			}
		}
	}

	// Rule: actor -[can_delegate_to]-> delegate -[provides]-> capability
	//       implies actor -[provides]-> capability via delegation
	for actor, dests := range delegates {
		for _, delegate := range dests {
			for _, cap := range provides[delegate] {
				k := edgeKey{actor, cap, "provides"}
				if edgeSet[k] {
					continue
				}
				via := nodeByID[delegate]
				implied = append(implied, ImpliedEdge{
					FromID:      actor,
					ToID:        cap,
					Type:        "provides",
					ViaNodeID:   delegate,
					ViaNodeType: via.Type,
					Basis:       "can delegate to an actor that provides this capability",
				})
			}
		}
	}

	return implied
}
