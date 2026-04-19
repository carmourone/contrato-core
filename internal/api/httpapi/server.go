package httpapi

import (
	"net/http"

	"contrato/internal/authn"
	"contrato/internal/authz"
	"contrato/internal/storage"
)

func NewServer(addr string, st storage.Store, an authn.Provider, az authz.Engine) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unhealthy"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/v0/contracts", withAuth(an, az, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("contracts endpoint stub"))
	})))

	return &http.Server{Addr: addr, Handler: mux}
}
