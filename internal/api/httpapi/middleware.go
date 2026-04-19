package httpapi

import (
	"context"
	"net/http"

	"contrato/internal/authn"
	"contrato/internal/authz"
)

type ctxKey string

const authnKey ctxKey = "authnctx"

func withAuth(an authn.Provider, az authz.Engine, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ac, err := an.Authenticate(ctx, r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}

		dec, err := az.Check(ctx, authz.CheckRequest{
			Subject:  authz.Subject{Type: ac.Actor.Type, ID: ac.Actor.ID},
			Relation: "access",
			Object:   authz.Object{Type: "system", ID: "contrato"},
			Context:  map[string]any{"path": r.URL.Path, "method": r.Method},
		})
		if err != nil || !dec.Allowed {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
			return
		}

		ctx = context.WithValue(ctx, authnKey, ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AuthnFromContext(ctx context.Context) (authn.Context, bool) {
	v := ctx.Value(authnKey)
	if v == nil {
		return authn.Context{}, false
	}
	ac, ok := v.(authn.Context)
	return ac, ok
}
