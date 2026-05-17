package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"contrato/internal/api/httpapi/handlers"
	"contrato/internal/api/httpapi/handlers/frontier"
	"contrato/internal/api/httpapi/handlers/waypoint"
	"contrato/internal/api/httpapi/respond"
	"contrato/internal/authn"
	"contrato/internal/authz"
	"contrato/internal/config"
	"contrato/internal/storage"
)

type storeWithDB interface {
	storage.Store
	DB() *sql.DB
}

func NewServer(addr string, st storage.Store, an authn.Provider, az authz.Engine, features config.Features) *http.Server {
	sdb, _ := st.(storeWithDB)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(withAuth(an, az))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Health(r.Context()); err != nil {
			respond.Error(w, http.StatusServiceUnavailable, "unhealthy")
			return
		}
		respond.OK(w, map[string]string{"status": "ok"})
	})

	r.Route("/v0", func(r chi.Router) {
		// model versions — all personas
		mv := &handlers.ModelVersionsHandler{Store: st}
		r.Get("/model-versions", mv.List)
		r.Post("/model-versions", mv.Create)
		r.Get("/model-versions/{model_id}", mv.Get)
		r.Patch("/model-versions/{model_id}/status", mv.SetStatus)

		// graph — all personas
		gh := &handlers.GraphHandler{Store: st}
		r.Get("/graph/nodes", gh.ListNodes)
		r.Post("/graph/nodes", gh.CreateNode)
		r.Get("/graph/nodes/{id}", gh.GetNode)
		r.Put("/graph/nodes/{id}", gh.UpdateNode)

		r.Get("/graph/edges", gh.ListEdges)
		r.Post("/graph/edges", gh.CreateEdge)
		r.Put("/graph/edges/{from}/{to}/{type}", gh.UpdateEdge)

		// properties — all personas
		ph := &handlers.PropertiesHandler{Store: st}
		r.Get("/properties/{owner_type}/{owner_id}/{key}", ph.Get)
		r.Put("/properties/{owner_type}/{owner_id}/{key}", ph.Put)

		// contracts — all personas
		ch := &handlers.ContractsHandler{Store: st}
		r.Get("/contracts", ch.List)
		r.Post("/contracts", ch.Create)
		r.Get("/contracts/{id}", ch.Get)
		r.Patch("/contracts/{id}/status", ch.SetStatus)

		// Frontier persona
		if features.Frontier && sdb != nil {
			fh := &frontier.Handler{Store: sdb}
			r.Get("/analysis/holes", fh.Holes)
			r.Get("/analysis/boundary", fh.Boundary)
			r.Get("/analysis/implied-edges", fh.ImpliedEdges)
		}

		// Waypoint persona
		if features.Waypoint && sdb != nil {
			wh := &waypoint.Handler{Store: sdb}
			r.Get("/analysis/gaps", wh.Gaps)
			r.Get("/analysis/paths", wh.Paths)
		}

		// Meridian persona (resolve/decide — resolver not yet implemented)
		if features.Meridian {
			r.Post("/resolve", func(w http.ResponseWriter, r *http.Request) {
				respond.Error(w, http.StatusNotImplemented, "resolver not yet implemented")
			})
			r.Post("/simulate", func(w http.ResponseWriter, r *http.Request) {
				respond.Error(w, http.StatusNotImplemented, "simulator not yet implemented")
			})
		}
	})

	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}
