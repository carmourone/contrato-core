package noop

import (
	"context"

	"contrato/internal/authn"
)

type Provider struct {
	ActorID string
	UserID  string
}

func (p *Provider) Name() string { return "noop" }

func (p *Provider) Authenticate(ctx context.Context, req any) (authn.Context, error) {
	_ = ctx
	_ = req
	actor := authn.Principal{Type: "service", ID: firstNonEmpty(p.ActorID, "service:dev"), Claims: map[string]any{"env": "dev"}}
	var user *authn.Principal
	if p.UserID != "" {
		u := authn.Principal{Type: "user", ID: p.UserID, Claims: map[string]any{"env": "dev"}}
		user = &u
	}
	return authn.Context{Actor: actor, User: user}, nil
}

func firstNonEmpty(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
