package opa

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/rego"

	"contrato/internal/resolution"
)

// Evaluator runs Rego policies embedded in policy node blobs against a resolved result.
type Evaluator struct{}

func New() *Evaluator { return &Evaluator{} }

// Eval evaluates all policies in result against req and merges their decisions.
// Each ResolvedPolicy is matched to a node in result.Nodes by PolicyNodeID; the
// node blob must contain a "source" field with the Rego module text.
// OPA is expected to return an object with "action", "reasons", and "obligations".
func (e *Evaluator) Eval(ctx context.Context, req resolution.ContractRequest, result resolution.Result) (resolution.Decision, error) {
	nodeBlobs := make(map[string]map[string]any, len(result.Nodes))
	for _, n := range result.Nodes {
		var m map[string]any
		if err := json.Unmarshal(n.Blob, &m); err == nil {
			nodeBlobs[n.NodeID] = m
		}
	}

	input := buildInput(req, result)

	var decision resolution.Decision

	for _, pol := range result.Policies {
		blob, ok := nodeBlobs[pol.PolicyNodeID]
		if !ok {
			return resolution.Decision{}, fmt.Errorf("opa: policy node %s not found in resolved nodes", pol.PolicyNodeID)
		}
		src, ok := blob["source"].(string)
		if !ok || src == "" {
			return resolution.Decision{}, fmt.Errorf("opa: policy node %s has no blob.source", pol.PolicyNodeID)
		}

		query := fmt.Sprintf("data.%s.%s", pol.Package, pol.Rule)
		r := rego.New(
			rego.Query(query),
			rego.Module(pol.PolicyNodeID+".rego", src),
			rego.Input(input),
		)

		rs, err := r.Eval(ctx)
		if err != nil {
			return resolution.Decision{}, fmt.Errorf("opa: eval policy %s: %w", pol.PolicyNodeID, err)
		}
		if len(rs) == 0 || len(rs[0].Expressions) == 0 {
			return resolution.Decision{}, fmt.Errorf("opa: policy %s returned no result", pol.PolicyNodeID)
		}

		out, ok := rs[0].Expressions[0].Value.(map[string]any)
		if !ok {
			return resolution.Decision{}, fmt.Errorf("opa: policy %s result is not an object", pol.PolicyNodeID)
		}

		if err := mergeDecision(&decision, out); err != nil {
			return resolution.Decision{}, fmt.Errorf("opa: merge policy %s: %w", pol.PolicyNodeID, err)
		}
	}

	return decision, nil
}

func buildInput(req resolution.ContractRequest, result resolution.Result) map[string]any {
	nodes := make([]map[string]any, len(result.Nodes))
	for i, n := range result.Nodes {
		var blob map[string]any
		_ = json.Unmarshal(n.Blob, &blob)
		nodes[i] = map[string]any{"node_id": n.NodeID, "type": n.Type, "blob": blob}
	}

	edges := make([]map[string]any, len(result.Edges))
	for i, e := range result.Edges {
		var blob map[string]any
		_ = json.Unmarshal(e.Blob, &blob)
		edges[i] = map[string]any{"from_id": e.FromID, "to_id": e.ToID, "type": e.Type, "blob": blob}
	}

	params := make([]map[string]any, len(result.Params))
	for i, p := range result.Params {
		params[i] = map[string]any{
			"owner_type": p.OwnerType,
			"owner_id":   p.OwnerID,
			"key":        p.Key,
			"float":      p.Float,
			"int":        p.Int,
			"text":       p.Text,
		}
	}

	return map[string]any{
		"model": map[string]any{
			"tenant_id":     result.Model.TenantID,
			"model_id":      result.Model.ModelID,
			"model_version": result.Model.ModelVersion,
		},
		"request": map[string]any{
			"id":        req.ID,
			"requested": req.Requested,
			"at":        req.At,
			"context":   req.Context,
			"actor":     map[string]any{"node_id": req.Actor.NodeID, "type": req.Actor.Type},
			"subject":   map[string]any{"node_id": req.Subject.NodeID, "type": req.Subject.Type},
			"resource":  map[string]any{"node_id": req.Resource.NodeID, "type": req.Resource.Type},
		},
		"nodes":  nodes,
		"edges":  edges,
		"params": params,
		"facts":  result.Facts,
	}
}

// mergeDecision folds one OPA policy result into the running decision.
// A deny from any policy wins; the first non-empty action otherwise wins.
func mergeDecision(d *resolution.Decision, out map[string]any) error {
	if action, ok := out["action"].(string); ok && action != "" {
		if d.Action == "" || action == "deny" {
			d.Action = action
		}
	}

	if reasons, ok := out["reasons"].([]any); ok {
		for _, r := range reasons {
			if s, ok := r.(string); ok {
				d.Reasons = append(d.Reasons, s)
			}
		}
	}

	if obligations, ok := out["obligations"].([]any); ok {
		for _, o := range obligations {
			if m, ok := o.(map[string]any); ok {
				ob := resolution.Obligation{}
				if name, ok := m["name"].(string); ok {
					ob.Name = name
				}
				if props, ok := m["properties"].(map[string]any); ok {
					ob.Properties = props
				}
				d.Obligations = append(d.Obligations, ob)
			}
		}
	}

	return nil
}
