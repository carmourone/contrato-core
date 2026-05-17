package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	pgmigrations "contrato/migrations/postgres"
)

func Run(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(pgmigrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("migrate: set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
