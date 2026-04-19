package postgres

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"

	"contrato/internal/storage"
	sqlstore "contrato/internal/storage/sqlstore"
)

type Driver struct {
	Migrate func(ctx context.Context, db *sql.DB) error
}

func (d Driver) Name() string { return "postgres" }

func (d Driver) Open(ctx context.Context, cfg map[string]any) (storage.Store, error) {
	dsn, _ := cfg["dsn"].(string)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if d.Migrate != nil {
		if err := d.Migrate(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	caps := storage.NewCapSet(storage.CapTx, storage.CapDoc, storage.CapKV, storage.CapCAS, storage.CapGraph, storage.CapTTL)
	return sqlstore.NewStore(db, caps), nil
}
