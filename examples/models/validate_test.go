package models_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// bundle mirrors the importable JSON bundle structure — only the fields we validate.
type bundle struct {
	FormatVersion int `json:"format_version"`
	Types         []struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
	} `json:"types"`
	Nodes []struct {
		ID   string         `json:"id"`
		Type string         `json:"type"`
		Blob map[string]any `json:"blob"`
	} `json:"nodes"`
	Edges []struct {
		FromID string `json:"from_id"`
		ToID   string `json:"to_id"`
		Type   string `json:"type"`
	} `json:"edges"`
	Contracts []struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Action string `json:"action"`
	} `json:"contracts"`
	Properties []map[string]any `json:"properties"`
}

func loadBundle(t *testing.T, name string) bundle {
	t.Helper()
	path := filepath.Join(".", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var b bundle
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return b
}

var modelFiles = []string{"john_wick.json", "moneyball.json"}

func TestExampleModelsParseCleanly(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			if b.FormatVersion != 1 {
				t.Errorf("expected format_version 1, got %d", b.FormatVersion)
			}
		})
	}
}

func TestNoInvalidPropertyFields(t *testing.T) {
	// Properties must use "value" (JSONB). The old schema had value_float /
	// value_int which don't exist in the storage layer.
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			for i, p := range b.Properties {
				if _, ok := p["value_float"]; ok {
					t.Errorf("property[%d] uses forbidden field value_float (use \"value\" instead)", i)
				}
				if _, ok := p["value_int"]; ok {
					t.Errorf("property[%d] uses forbidden field value_int (use \"value\" instead)", i)
				}
				if _, ok := p["value"]; !ok {
					t.Errorf("property[%d] is missing required field \"value\"", i)
				}
			}
		})
	}
}

func TestAllContractTypesDeclared(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)

			declared := make(map[string]bool)
			for _, ty := range b.Types {
				if ty.Domain == "contract" {
					declared[ty.Name] = true
				}
			}
			for i, c := range b.Contracts {
				if c.Type != "" && !declared[c.Type] {
					t.Errorf("contract[%d] type %q not declared in types array", i, c.Type)
				}
			}
		})
	}
}

func TestNoDuplicateNodeIDs(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			seen := make(map[string]int)
			for _, nd := range b.Nodes {
				seen[nd.ID]++
			}
			for id, count := range seen {
				if count > 1 {
					t.Errorf("node id %q appears %d times", id, count)
				}
			}
		})
	}
}

func TestAllEdgeEndpointsExist(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			nodeIDs := make(map[string]bool, len(b.Nodes))
			for _, nd := range b.Nodes {
				nodeIDs[nd.ID] = true
			}
			for i, eg := range b.Edges {
				if !nodeIDs[eg.FromID] {
					t.Errorf("edge[%d] from_id %q not found in nodes", i, eg.FromID)
				}
				if !nodeIDs[eg.ToID] {
					t.Errorf("edge[%d] to_id %q not found in nodes", i, eg.ToID)
				}
			}
		})
	}
}

func TestAllEdgeTypesDeclared(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			declared := make(map[string]bool)
			for _, ty := range b.Types {
				if ty.Domain == "edge" {
					declared[ty.Name] = true
				}
			}
			for i, eg := range b.Edges {
				if !declared[eg.Type] {
					t.Errorf("edge[%d] type %q not declared in types array", i, eg.Type)
				}
			}
		})
	}
}

func TestAllNodeTypesDeclared(t *testing.T) {
	for _, f := range modelFiles {
		f := f
		t.Run(f, func(t *testing.T) {
			b := loadBundle(t, f)
			declared := make(map[string]bool)
			for _, ty := range b.Types {
				if ty.Domain == "node" {
					declared[ty.Name] = true
				}
			}
			for i, nd := range b.Nodes {
				if !declared[nd.Type] {
					t.Errorf("node[%d] type %q not declared in types array", i, nd.Type)
				}
			}
		})
	}
}

func TestMoneyballHasRealPlayerNames(t *testing.T) {
	b := loadBundle(t, "moneyball.json")
	wantPlayers := []string{"Scott Hatteberg", "David Justice", "Jeremy Giambi", "Chad Bradford", "Barry Zito"}
	nameSet := make(map[string]bool)
	for _, nd := range b.Nodes {
		if nd.Type == "player" || nd.Type == "person" {
			if name, ok := nd.Blob["name"].(string); ok {
				nameSet[name] = true
			}
		}
	}
	for _, want := range wantPlayers {
		if !nameSet[want] {
			t.Errorf("expected real player %q in moneyball model", want)
		}
	}
}

func TestMoneyballCloserGapExists(t *testing.T) {
	// "Closer Coverage" must exist and be marked unavailable (availability=false)
	// to demonstrate the Waypoint gap detection scenario.
	b := loadBundle(t, "moneyball.json")

	// Find the Closer Coverage node ID.
	closerID := ""
	for _, nd := range b.Nodes {
		if name, _ := nd.Blob["name"].(string); name == "Closer Coverage" {
			closerID = nd.ID
			break
		}
	}
	if closerID == "" {
		t.Fatal("moneyball model must contain a 'Closer Coverage' capability node")
	}

	// Find the availability property for it.
	for _, p := range b.Properties {
		ownerID, _ := p["owner_id"].(string)
		key, _ := p["key"].(string)
		if ownerID == closerID && key == "availability" {
			avail, ok := p["value"].(bool)
			if !ok || avail {
				t.Errorf("Closer Coverage availability should be false (gap scenario), got %v", p["value"])
			}
			return
		}
	}
	t.Error("no availability property found for Closer Coverage node")
}

func TestJohnWickHasCanDelegateTo(t *testing.T) {
	b := loadBundle(t, "john_wick.json")
	for _, eg := range b.Edges {
		if eg.Type == "can_delegate_to" {
			return
		}
	}
	t.Error("john_wick model must contain at least one can_delegate_to edge for implied-edge detection")
}

func TestJohnWickPoliciesHaveOPASource(t *testing.T) {
	b := loadBundle(t, "john_wick.json")
	for _, nd := range b.Nodes {
		if nd.Type != "policy" {
			continue
		}
		if _, ok := nd.Blob["source"]; !ok {
			name, _ := nd.Blob["name"].(string)
			t.Errorf("policy node %q is missing 'source' field (OPA Rego)", name)
		}
	}
}
