package authn

import "context"

type Principal struct {
	Type   string
	ID     string
	Claims map[string]any
}

type Context struct {
	Actor    Principal
	User     *Principal
	TenantID string
}

type Provider interface {
	Name() string
	Authenticate(ctx context.Context, req any) (Context, error)
}
