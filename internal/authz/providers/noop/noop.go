package noop

import (
	"context"

	"contrato/internal/authz"
)

type Engine struct{ Allow bool }

func (e *Engine) Name() string { return "noop" }

func (e *Engine) Check(ctx context.Context, req authz.CheckRequest) (authz.CheckResponse, error) {
	_ = ctx
	_ = req
	if e.Allow {
		return authz.CheckResponse{Allowed: true, Reason: "noop allow"}, nil
	}
	return authz.CheckResponse{Allowed: false, Reason: "noop deny"}, nil
}

func (e *Engine) WriteTuples(ctx context.Context, writes []authz.Tuple, deletes []authz.Tuple) error {
	_ = ctx
	_ = writes
	_ = deletes
	return nil
}
