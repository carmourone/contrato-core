package modelio

import (
	"encoding/json"
	"fmt"
)

type LintIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type LintReport struct {
	OK     bool        `json:"ok"`
	Issues []LintIssue `json:"issues,omitempty"`
}

type LintMode string

const (
	LintModeEnabledModel LintMode = "enabled_model"
	LintModeBootstrap    LintMode = "bootstrap"
)

func LintBundle(b Bundle, mode LintMode) LintReport {
	issues := []LintIssue{}

	typeSet := map[string]bool{}
	for _, t := range b.Types {
		typeSet[t.Domain+":"+t.Name] = true
	}

	reqTypes := []struct{ domain, name string }{
		{"node", "capability"},
		{"node", "policy"},
		{"edge", "provides"},
		{"edge", "requires"},
		{"edge", "governs"},
	}
	if mode != LintModeBootstrap {
		reqTypes = append(reqTypes, struct{ domain, name string }{"node", "actor"})
	}
	for _, rt := range reqTypes {
		if !typeSet[rt.domain+":"+rt.name] {
			issues = append(issues, LintIssue{Code: "MISSING_TYPE", Message: fmt.Sprintf("missing required type %s:%s", rt.domain, rt.name)})
		}
	}

	caps := []Node{}
	pols := []Node{}
	actors := []Node{}
	idSet := map[string]bool{}
	for _, n := range b.Nodes {
		if n.ID != "" {
			idSet[n.ID] = true
		}
		switch n.Type {
		case "capability":
			caps = append(caps, n)
		case "policy":
			pols = append(pols, n)
		case "actor":
			actors = append(actors, n)
		}
	}

	if mode != LintModeBootstrap && len(actors) == 0 {
		issues = append(issues, LintIssue{Code: "NO_ACTORS", Message: "enabled model should include at least one actor node"})
	}
	if len(pols) == 0 {
		issues = append(issues, LintIssue{Code: "NO_POLICIES", Message: "enabled model should include at least one policy node"})
	}
	if mode != LintModeBootstrap && len(caps) == 0 {
		issues = append(issues, LintIssue{Code: "NO_CAPABILITIES", Message: "enabled model should include at least one capability node"})
	}

	providers := map[string]bool{}
	governed := map[string]bool{}
	for _, e := range b.Edges {
		if e.FromID != "" && !idSet[e.FromID] {
			issues = append(issues, LintIssue{Code: "BAD_REF", Message: "edge.from_id not found", Path: e.FromID})
		}
		if e.ToID != "" && !idSet[e.ToID] {
			issues = append(issues, LintIssue{Code: "BAD_REF", Message: "edge.to_id not found", Path: e.ToID})
		}
		switch e.Type {
		case "provides":
			providers[e.ToID] = true
		case "governs":
			governed[e.ToID] = true
		}
	}

	if mode != LintModeBootstrap {
		for _, c := range caps {
			if c.ID == "" {
				continue
			}
			if !providers[c.ID] {
				issues = append(issues, LintIssue{Code: "CAP_NO_PROVIDER", Message: "capability has no provider (missing provides edge)", Path: c.ID})
			}
			if !governed[c.ID] {
				issues = append(issues, LintIssue{Code: "CAP_NO_POLICY", Message: "capability has no governing policy (missing governs edge)", Path: c.ID})
			}
			if c.Blob["ref"] == nil {
				issues = append(issues, LintIssue{Code: "CAP_NO_REF", Message: "capability missing blob.ref", Path: c.ID})
			}
			if c.Blob["job"] == nil {
				issues = append(issues, LintIssue{Code: "CAP_NO_JOB", Message: "capability missing blob.job", Path: c.ID})
			}
		}
	}

	ok := len(issues) == 0
	return LintReport{OK: ok, Issues: issues}
}

func (r LintReport) JSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}
