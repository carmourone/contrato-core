package frontier

import (
	"testing"
)

// --- helpers ---

func n(id, typ string) gNode { return gNode{ID: id, Type: typ, Domain: "node"} }
func e(from, to, typ string) gEdge { return gEdge{FromID: from, ToID: to, Type: typ, Domain: "edge"} }

func mustGraph(t *testing.T, nodes []gNode, edges []gEdge) ([]gNode, []gEdge, interface{ String() string }) {
	t.Helper()
	g, err := buildGraph(nodes, edges)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	// return a thin wrapper so callers can pass g directly to detect* functions
	_ = g
	return nodes, edges, nil
}

// holesOfKind filters holes by kind for assertions.
func holesOfKind(holes []Hole, kind string) []Hole {
	var out []Hole
	for _, h := range holes {
		if h.Kind == kind {
			out = append(out, h)
		}
	}
	return out
}

func hasHoleFor(holes []Hole, kind, nodeID string) bool {
	for _, h := range holes {
		if h.Kind == kind && h.NodeID == nodeID {
			return true
		}
	}
	return false
}

func hasBoundaryFor(bns []BoundaryNode, nodeID string) bool {
	for _, b := range bns {
		if b.NodeID == nodeID {
			return true
		}
	}
	return false
}

func hasImplied(edges []ImpliedEdge, fromID, toID, typ string) bool {
	for _, e := range edges {
		if e.FromID == fromID && e.ToID == toID && e.Type == typ {
			return true
		}
	}
	return false
}

// --- buildGraph ---

func TestBuildGraph(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		g, err := buildGraph(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		adj, _ := g.AdjacencyMap()
		if len(adj) != 0 {
			t.Errorf("expected empty graph, got %d vertices", len(adj))
		}
	})

	t.Run("single_node", func(t *testing.T) {
		nodes := []gNode{n("a", "capability")}
		g, err := buildGraph(nodes, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		adj, _ := g.AdjacencyMap()
		if len(adj) != 1 {
			t.Errorf("expected 1 vertex, got %d", len(adj))
		}
	})

	t.Run("duplicate_edge_ignored", func(t *testing.T) {
		nodes := []gNode{n("a", "actor"), n("b", "capability")}
		edges := []gEdge{
			e("a", "b", "provides"),
			e("a", "b", "provides"), // duplicate — must not error
		}
		if _, err := buildGraph(nodes, edges); err != nil {
			t.Fatalf("duplicate edge should be silently ignored: %v", err)
		}
	})
}

// --- detectHoles ---

func TestDetectHoles(t *testing.T) {
	t.Run("empty_graph", func(t *testing.T) {
		g, _ := buildGraph(nil, nil)
		holes := detectHoles(nil, nil, g)
		if len(holes) != 0 {
			t.Errorf("expected no holes, got %v", holes)
		}
	})

	t.Run("isolated_node", func(t *testing.T) {
		nodes := []gNode{n("cap1", "capability")}
		g, _ := buildGraph(nodes, nil)
		holes := detectHoles(nodes, nil, g)
		if !hasHoleFor(holes, "isolated_node", "cap1") {
			t.Errorf("expected isolated_node for cap1, got %v", holes)
		}
	})

	t.Run("capability_no_provider", func(t *testing.T) {
		// Policy governs capability but no actor provides it.
		nodes := []gNode{n("cap1", "capability"), n("pol1", "policy")}
		edges := []gEdge{e("pol1", "cap1", "governs")}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "capability_no_provider", "cap1") {
			t.Errorf("expected capability_no_provider for cap1, got %v", holes)
		}
	})

	t.Run("capability_no_policy", func(t *testing.T) {
		// Actor provides capability but no policy governs it.
		nodes := []gNode{n("act1", "actor"), n("cap1", "capability")}
		edges := []gEdge{e("act1", "cap1", "provides")}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "capability_no_policy", "cap1") {
			t.Errorf("expected capability_no_policy for cap1, got %v", holes)
		}
	})

	t.Run("capability_well_formed", func(t *testing.T) {
		// Capability with both provider and governing policy → no capability holes.
		nodes := []gNode{n("act1", "actor"), n("cap1", "capability"), n("pol1", "policy")}
		edges := []gEdge{
			e("act1", "cap1", "provides"),
			e("pol1", "cap1", "governs"),
		}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if hasHoleFor(holes, "capability_no_provider", "cap1") {
			t.Error("capability_no_provider should not fire for well-formed capability")
		}
		if hasHoleFor(holes, "capability_no_policy", "cap1") {
			t.Error("capability_no_policy should not fire for well-formed capability")
		}
	})

	t.Run("policy_governs_nothing", func(t *testing.T) {
		// Policy applies_to a service but has no governs edge → dead policy.
		nodes := []gNode{n("pol1", "policy"), n("svc1", "service")}
		edges := []gEdge{e("pol1", "svc1", "applies_to")}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "policy_governs_nothing", "pol1") {
			t.Errorf("expected policy_governs_nothing for pol1, got %v", holes)
		}
	})

	t.Run("actor_no_capabilities", func(t *testing.T) {
		// Actor only has an incoming edge, no outgoing → no capability assignments.
		nodes := []gNode{n("act1", "actor"), n("svc1", "service")}
		edges := []gEdge{e("svc1", "act1", "owns")}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "actor_no_capabilities", "act1") {
			t.Errorf("expected actor_no_capabilities for act1, got %v", holes)
		}
	})

	t.Run("actor_with_provides_not_flagged", func(t *testing.T) {
		nodes := []gNode{n("act1", "actor"), n("cap1", "capability"), n("pol1", "policy")}
		edges := []gEdge{
			e("act1", "cap1", "provides"),
			e("pol1", "cap1", "governs"),
		}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if hasHoleFor(holes, "actor_no_capabilities", "act1") {
			t.Error("actor_no_capabilities should not fire when actor has provides edges")
		}
	})

	t.Run("structural_hole", func(t *testing.T) {
		// B bridges two otherwise separate groups: {A,D} → B → {C}.
		// In a DAG every node is its own singleton SCC, so the implementation
		// marks any node that appears in a cross-SCC edge as a structural hole.
		// B (interior) is definitely one; A and D (sources) are also flagged
		// because the implementation marks both edge endpoints.
		nodes := []gNode{
			n("A", "service"), n("B", "capability"), n("C", "capability"), n("D", "actor"),
		}
		edges := []gEdge{
			e("A", "B", "depends_on"),
			e("B", "C", "requires"),
			e("D", "B", "provides"),
		}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "structural_hole", "B") {
			t.Errorf("expected structural_hole for B (bridges disconnected groups), got %v", holes)
		}
		// C is a sink (outDeg=0) — it only has an incoming inter-SCC edge,
		// so it is NOT in interEdges as a "from" node, but the current impl
		// marks both endpoints → C is also flagged. Just assert B is present.
		structuralHoles := holesOfKind(holes, "structural_hole")
		if len(structuralHoles) == 0 {
			t.Error("expected at least one structural_hole")
		}
	})

	t.Run("moneyball_closer_gap", func(t *testing.T) {
		// Mirrors the moneyball model: Closer Coverage has no provides edge.
		// Ground Ball Bullpen has one provider (single_provider caught by Waypoint,
		// but frontier detects the policy gap).
		nodes := []gNode{
			n("act-beane",    "actor"),
			n("cap-team",     "capability"),
			n("cap-closer",   "capability"),
			n("pol-payroll",  "policy"),
		}
		edges := []gEdge{
			e("act-beane",   "cap-team",   "provides"),
			e("pol-payroll", "cap-team",   "governs"),
			e("cap-team",    "cap-closer", "requires"),
			// cap-closer has no provides edge and no governs edge
		}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "capability_no_provider", "cap-closer") {
			t.Errorf("cap-closer should be flagged capability_no_provider, got %v", holes)
		}
		if !hasHoleFor(holes, "capability_no_policy", "cap-closer") {
			t.Errorf("cap-closer should be flagged capability_no_policy, got %v", holes)
		}
	})

	t.Run("john_wick_marker_debt_gap", func(t *testing.T) {
		// In the John Wick model, Marker Debt Resolution has no provider.
		// Mirrors the actual graph structure.
		nodes := []gNode{
			n("act-winston", "actor"),
			n("cap-sanc",    "capability"),
			n("cap-enforce", "capability"),
			n("cap-marker",  "capability"),
			n("pol-cont",    "policy"),
			n("pol-ht",      "policy"),
		}
		edges := []gEdge{
			e("act-winston", "cap-sanc",    "provides"),
			e("act-winston", "cap-enforce", "provides"),
			e("pol-cont",    "cap-sanc",    "governs"),
			e("pol-ht",      "cap-enforce", "governs"),
			e("pol-ht",      "cap-marker",  "governs"),
			e("cap-enforce", "cap-sanc",    "requires"),
			e("cap-marker",  "cap-enforce", "requires"),
			// cap-marker has no provides edge → capability_no_provider
		}
		g, _ := buildGraph(nodes, edges)
		holes := detectHoles(nodes, edges, g)
		if !hasHoleFor(holes, "capability_no_provider", "cap-marker") {
			t.Errorf("cap-marker should have capability_no_provider, got %v", holes)
		}
		// cap-sanc and cap-enforce are well-formed — should not be flagged.
		if hasHoleFor(holes, "capability_no_provider", "cap-sanc") {
			t.Error("cap-sanc should not be flagged capability_no_provider")
		}
		if hasHoleFor(holes, "capability_no_provider", "cap-enforce") {
			t.Error("cap-enforce should not be flagged capability_no_provider")
		}
	})
}

// --- detectBoundary ---

func TestDetectBoundary(t *testing.T) {
	t.Run("source_node", func(t *testing.T) {
		// Source: outDeg=1, inDeg=0 → total degree 1.
		nodes := []gNode{n("src", "service"), n("dst", "capability")}
		edges := []gEdge{e("src", "dst", "depends_on")}
		g, _ := buildGraph(nodes, edges)
		bns := detectBoundary(nodes, g)
		if !hasBoundaryFor(bns, "src") {
			t.Errorf("source node should be a boundary node, got %v", bns)
		}
		for _, b := range bns {
			if b.NodeID == "src" && b.Description == "" {
				t.Error("boundary description should not be empty")
			}
		}
	})

	t.Run("sink_node", func(t *testing.T) {
		// Sink: inDeg=1, outDeg=0 → total degree 1.
		nodes := []gNode{n("src", "actor"), n("dst", "capability")}
		edges := []gEdge{e("src", "dst", "provides")}
		g, _ := buildGraph(nodes, edges)
		bns := detectBoundary(nodes, g)
		if !hasBoundaryFor(bns, "dst") {
			t.Errorf("sink node should be a boundary node, got %v", bns)
		}
	})

	t.Run("interior_node_not_boundary", func(t *testing.T) {
		// Interior: inDeg=1, outDeg=1 → total degree 2 → not a boundary node.
		nodes := []gNode{n("a", "actor"), n("b", "capability"), n("c", "capability")}
		edges := []gEdge{e("a", "b", "provides"), e("b", "c", "requires")}
		g, _ := buildGraph(nodes, edges)
		bns := detectBoundary(nodes, g)
		if hasBoundaryFor(bns, "b") {
			t.Error("interior node with degree 2 should not be a boundary node")
		}
	})

	t.Run("hub_not_boundary", func(t *testing.T) {
		// Hub: high in+out degree → not a boundary node.
		nodes := []gNode{
			n("hub", "capability"),
			n("a", "actor"), n("b", "actor"), n("c", "capability"), n("d", "capability"),
		}
		edges := []gEdge{
			e("a",   "hub", "provides"),
			e("b",   "hub", "provides"),
			e("hub", "c",   "requires"),
			e("hub", "d",   "requires"),
		}
		g, _ := buildGraph(nodes, edges)
		bns := detectBoundary(nodes, g)
		if hasBoundaryFor(bns, "hub") {
			t.Error("hub node with degree 4 should not be a boundary node")
		}
	})

	t.Run("isolated_is_boundary", func(t *testing.T) {
		// Isolated node (degree 0) is included as a boundary node.
		nodes := []gNode{n("lone", "person")}
		g, _ := buildGraph(nodes, nil)
		bns := detectBoundary(nodes, g)
		if !hasBoundaryFor(bns, "lone") {
			t.Errorf("isolated node should appear as boundary node, got %v", bns)
		}
	})

	t.Run("chain_endpoints", func(t *testing.T) {
		// A → B → C: A is source boundary, C is sink boundary, B is interior.
		nodes := []gNode{n("A", "actor"), n("B", "capability"), n("C", "capability")}
		edges := []gEdge{e("A", "B", "provides"), e("B", "C", "requires")}
		g, _ := buildGraph(nodes, edges)
		bns := detectBoundary(nodes, g)
		if !hasBoundaryFor(bns, "A") {
			t.Error("A (source) should be a boundary node")
		}
		if !hasBoundaryFor(bns, "C") {
			t.Error("C (sink) should be a boundary node")
		}
		if hasBoundaryFor(bns, "B") {
			t.Error("B (interior, degree 2) should not be a boundary node")
		}
	})
}

// --- detectImpliedEdges ---

func TestDetectImpliedEdges(t *testing.T) {
	t.Run("empty_graph", func(t *testing.T) {
		g, _ := buildGraph(nil, nil)
		implied := detectImpliedEdges(nil, nil, g)
		if len(implied) != 0 {
			t.Errorf("expected no implied edges, got %v", implied)
		}
	})

	t.Run("provides_requires_chain", func(t *testing.T) {
		// actor -[provides]-> capA -[requires]-> capB
		// implies actor -[provides]-> capB (no direct edge exists)
		nodes := []gNode{n("actor1", "actor"), n("capA", "capability"), n("capB", "capability")}
		edges := []gEdge{
			e("actor1", "capA", "provides"),
			e("capA",   "capB", "requires"),
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		if !hasImplied(implied, "actor1", "capB", "provides") {
			t.Errorf("expected implied provides actor1→capB, got %v", implied)
		}
	})

	t.Run("provides_requires_chain_direct_exists", func(t *testing.T) {
		// Same chain but actor already directly provides capB → no implied edge.
		nodes := []gNode{n("actor1", "actor"), n("capA", "capability"), n("capB", "capability")}
		edges := []gEdge{
			e("actor1", "capA", "provides"),
			e("capA",   "capB", "requires"),
			e("actor1", "capB", "provides"), // direct already exists
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		for _, ie := range implied {
			if ie.FromID == "actor1" && ie.ToID == "capB" {
				t.Errorf("should not produce implied edge when direct already exists: %v", ie)
			}
		}
	})

	t.Run("delegation_chain", func(t *testing.T) {
		// actor1 -[can_delegate_to]-> actor2 -[provides]-> cap1
		// implies actor1 -[provides]-> cap1 via delegation
		nodes := []gNode{n("actor1", "actor"), n("actor2", "actor"), n("cap1", "capability")}
		edges := []gEdge{
			e("actor1", "actor2", "can_delegate_to"),
			e("actor2", "cap1",   "provides"),
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		if !hasImplied(implied, "actor1", "cap1", "provides") {
			t.Errorf("expected implied provides actor1→cap1 via delegation, got %v", implied)
		}
		// check the via node is actor2
		for _, ie := range implied {
			if ie.FromID == "actor1" && ie.ToID == "cap1" && ie.ViaNodeID != "actor2" {
				t.Errorf("implied edge should be via actor2, got via=%s", ie.ViaNodeID)
			}
		}
	})

	t.Run("delegation_chain_direct_exists", func(t *testing.T) {
		// actor1 already directly provides cap1 → no implied delegation edge.
		nodes := []gNode{n("actor1", "actor"), n("actor2", "actor"), n("cap1", "capability")}
		edges := []gEdge{
			e("actor1", "actor2", "can_delegate_to"),
			e("actor2", "cap1",   "provides"),
			e("actor1", "cap1",   "provides"), // direct
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		for _, ie := range implied {
			if ie.FromID == "actor1" && ie.ToID == "cap1" {
				t.Errorf("should not produce implied edge when direct already exists: %v", ie)
			}
		}
	})

	t.Run("moneyball_delegation", func(t *testing.T) {
		// Beane can_delegate_to DePodesta; DePodesta provides Statistical Modelling.
		// Beane does NOT directly provide Statistical Modelling → implied edge.
		nodes := []gNode{
			n("act-beane",     "actor"),
			n("act-depodesta", "actor"),
			n("cap-team",      "capability"),
			n("cap-stats",     "capability"),
		}
		edges := []gEdge{
			e("act-beane",     "cap-team",      "provides"),
			e("act-beane",     "act-depodesta", "can_delegate_to"),
			e("act-depodesta", "cap-stats",     "provides"),
			e("cap-team",      "cap-stats",     "requires"),
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)

		// Via delegation: Beane can provide cap-stats through DePodesta.
		if !hasImplied(implied, "act-beane", "cap-stats", "provides") {
			t.Errorf("expected implied provides act-beane→cap-stats, got %v", implied)
		}
	})

	t.Run("john_wick_delegation_suppressed", func(t *testing.T) {
		// Winston can_delegate_to Charon; Charon provides Sanctuary.
		// Winston ALSO directly provides Sanctuary → no implied edge generated.
		nodes := []gNode{
			n("act-winston", "actor"),
			n("act-charon",  "actor"),
			n("cap-sanc",    "capability"),
		}
		edges := []gEdge{
			e("act-winston", "cap-sanc",   "provides"),
			e("act-charon",  "cap-sanc",   "provides"),
			e("act-winston", "act-charon", "can_delegate_to"),
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		for _, ie := range implied {
			if ie.FromID == "act-winston" && ie.ToID == "cap-sanc" {
				t.Errorf("delegation-implied edge should be suppressed when direct exists: %v", ie)
			}
		}
	})

	t.Run("no_false_positives_for_non_provides", func(t *testing.T) {
		// governs and applies_to edges should never produce implied provides edges.
		nodes := []gNode{
			n("pol1", "policy"),
			n("cap1", "capability"),
			n("cap2", "capability"),
		}
		edges := []gEdge{
			e("pol1", "cap1", "governs"),
			e("cap1", "cap2", "requires"),
		}
		g, _ := buildGraph(nodes, edges)
		implied := detectImpliedEdges(nodes, edges, g)
		for _, ie := range implied {
			if ie.Type == "provides" && ie.FromID == "pol1" {
				t.Errorf("policy should not produce implied provides edges: %v", ie)
			}
		}
	})
}
