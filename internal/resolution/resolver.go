package resolution

import "context"

// Resolver builds a deterministic plan and then executes it against storage.
// Policy evaluation is intentionally separated: this returns everything needed
// to call OPA, but does not embed an OPA engine here.
type Resolver interface {
	BuildPlan(ctx context.Context, req ContractRequest, limits Limits) (Plan, error)
	Execute(ctx context.Context, plan Plan) (Result, error)
}
