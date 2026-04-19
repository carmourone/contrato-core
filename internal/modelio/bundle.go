package modelio

import "time"

type Bundle struct {
	FormatVersion int        `json:"format_version"`
	Tenant        Tenant     `json:"tenant"`
	Model         Model      `json:"model"`
	Types         []Type     `json:"types"`
	Statuses      []Status   `json:"statuses"`
	Nodes         []Node     `json:"nodes"`
	Edges         []Edge     `json:"edges"`
	Properties    []Property `json:"properties"`
	Metrics       []Metric   `json:"metrics,omitempty"` // deprecated (backward-compatible)
}

type Tenant struct {
	Name string `json:"name"`
}

type Model struct {
	ModelID     string    `json:"model_id,omitempty"`
	Version     int       `json:"version,omitempty"`
	Status      string    `json:"status,omitempty"` // draft|enabled|disabled
	ChangeNote  string    `json:"change_note,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type Type struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

type Status struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

type Node struct {
	ID     string   `json:"id"`
	Domain string   `json:"domain,omitempty"` // default "node"
	Type   string   `json:"type"`
	Blob   jsonRaw  `json:"blob,omitempty"`
}

type Edge struct {
	FromID  string  `json:"from_id"`
	ToID    string  `json:"to_id"`
	Domain  string  `json:"domain,omitempty"` // default "edge"
	Type    string  `json:"type"`
	Blob    jsonRaw `json:"blob,omitempty"`
}

type Property struct {
	OwnerType string  `json:"owner_type"` // "node"|"edge"|"contract"|etc
	OwnerID   string  `json:"owner_id"`
	Key       string  `json:"key"`
	Value     jsonRaw `json:"value"`
}

// Property is the unified typed key/value storage for config, evidence, and metrics.
// Exactly one of Float/Int/Text/JSON/BytesB64 should be set.

// Metric is deprecated; kept for backward-compatible imports.
type Metric struct {
	OwnerType string   `json:"owner_type"`
	OwnerID   string   `json:"owner_id"`
	Key       string   `json:"key"`
	Float     *float64 `json:"float,omitempty"`
	Int       *int64   `json:"int,omitempty"`
	Text      *string  `json:"text,omitempty"`
	JSON      *jsonRaw `json:"json,omitempty"`
}

type jsonRaw = map[string]any
