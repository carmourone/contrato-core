package authz

import "context"

type Subject struct{ Type, ID string }
type Object struct{ Type, ID string }

type CheckRequest struct {
	Subject  Subject
	Relation string
	Object   Object
	Context  map[string]any
}

type CheckResponse struct {
	Allowed bool
	Reason  string
}

type Tuple struct {
	Subject  Subject
	Relation string
	Object   Object
}

type Engine interface {
	Name() string
	Check(ctx context.Context, req CheckRequest) (CheckResponse, error)
	WriteTuples(ctx context.Context, writes []Tuple, deletes []Tuple) error
}
