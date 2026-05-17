package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"

	"contrato/internal/api/httpapi"
	"contrato/internal/authn"
	authnnoop "contrato/internal/authn/providers/noop"
	"contrato/internal/authz"
	authznoop "contrato/internal/authz/providers/noop"
	"contrato/internal/config"
	"contrato/internal/migrate"
	"contrato/internal/storage"
	pg "contrato/internal/storage/drivers/postgres"
)

type App struct {
	cfg   config.Config
	store storage.Store
	authn authn.Provider
	authz authz.Engine
	http  *http.Server
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	if cfg.PostgresDSN == "" {
		return nil, errors.New("CONTRATO_PG_DSN is required")
	}

	var an authn.Provider
	switch cfg.AuthN.Provider {
	case "noop":
		an = &authnnoop.Provider{ActorID: "service:dev", UserID: "user:dev"}
	default:
		return nil, fmt.Errorf("unknown authn provider %q", cfg.AuthN.Provider)
	}

	var az authz.Engine
	switch cfg.AuthZ.Provider {
	case "noop":
		if cfg.AuthZ.NoopAllow && cfg.Env != "dev" {
			return nil, fmt.Errorf("refusing noop authz allow outside dev (CONTRATO_ENV=%s)", cfg.Env)
		}
		az = &authznoop.Engine{Allow: cfg.AuthZ.NoopAllow}
	default:
		return nil, fmt.Errorf("unknown authz provider %q", cfg.AuthZ.Provider)
	}

	drv := pg.Driver{Migrate: migrate.Run}
	st, err := drv.Open(ctx, map[string]any{"dsn": cfg.PostgresDSN})
	if err != nil {
		return nil, err
	}

	features := cfg.License.Features()
	a := &App{cfg: cfg, store: st, authn: an, authz: az}
	a.http = httpapi.NewServer(cfg.HTTPAddr, st, an, az, features)
	return a, nil
}

func (a *App) HTTPServer() *http.Server { return a.http }

func (a *App) Close() error {
	if a.store != nil {
		_ = a.store.Close()
	}
	return nil
}
