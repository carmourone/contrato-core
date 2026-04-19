package resolution

import "time"

// ModelRef pins resolution to a specific, replayable model snapshot.
type ModelRef struct {
	TenantID     string
	ModelID      string
	ModelVersion int
}

// ContractRequest is the minimal input used by the resolver.
type ContractRequest struct {
	ID        string
	TenantID  string
	Model     ModelRef
	Actor     EntityRef
	Subject   EntityRef
	Resource  EntityRef
	Requested string            // requested action verb (e.g. "approve_expense", "recover_service")
	Context   map[string]any    // opaque input payload
	At        time.Time
}

// EntityRef references a node in the model graph.
type EntityRef struct {
	NodeID string
	Type   string
}

// Decision is the normalized contract decision payload intended for OPA handoff and storage.
type Decision struct {
	Action      string       // allow|deny|defer|delegate|require_approval|escalate
	Obligations []Obligation
	Reasons     []string     // stable reason codes
	Facts       map[string]any // derived facts (optional, for audit/replay)
}

// Obligation is a post-decision requirement (log, notify, create approval task, etc).
type Obligation struct {
	Name   string
	Properties map[string]any
}
