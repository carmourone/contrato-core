package resolution

// The resolver is deterministic and produces a bounded resolution plan.
// The plan can be executed to fetch/derive facts and to attach policies.

type StepKind string

const (
	StepLoadNode      StepKind = "load_node"
	StepLoadEdges     StepKind = "load_edges"
	StepLoadParams    StepKind = "load_parameters"
	StepDeriveFacts   StepKind = "derive_facts"
	StepAttachPolicy  StepKind = "attach_policy"
	StepOPAInput      StepKind = "build_opa_input"
)

// Step is a single deterministic operation. Steps are append-only in the plan.
type Step struct {
	Kind StepKind
	Ref  string            // stable reference for traceability (e.g. "actor", "capability_path[0]")
	Args map[string]any
}

type Plan struct {
	Model   ModelRef
	Steps   []Step
	Limits  Limits
	Notes   []string
}

type Limits struct {
	MaxNodes int
	MaxEdges int
	MaxDepth int
}

// Result contains the resolved graph subset + derived facts + policies to hand to OPA.
type Result struct {
	Model    ModelRef
	Nodes    []ResolvedNode
	Edges    []ResolvedEdge
	Params   []ResolvedProperty
	Policies []ResolvedPolicy
	Facts    map[string]any
}

type ResolvedNode struct {
	NodeID string
	Type   string
	Blob   []byte
}

type ResolvedEdge struct {
	FromID string
	ToID   string
	Type   string
	Blob   []byte
}

type ResolvedProperty struct {
	OwnerType string
	OwnerID   string
	Key   string
	// value fields are captured as raw to preserve type fidelity
	Float *float64
	Int   *int64
	Text  *string
	JSON  []byte
	Bytes []byte
}

type ResolvedPolicy struct {
	PolicyNodeID string
	Package      string
	Rule         string
	InputSchema  map[string]any
	OutputSchema map[string]any
}
