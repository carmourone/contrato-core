package sqlstore

import (
	"context"
	"database/sql"

	"contrato/internal/storage"
)

type querier interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type Tx struct {
	tx     *sql.Tx
	cap    storage.CapSet
	cancel context.CancelFunc
}

func (t *Tx) Tenants() storage.TenantRepo             { return &TenantsRepo{q: t.tx} }
func (t *Tx) Types() storage.TypeRepo                 { return &TypesRepo{q: t.tx} }
func (t *Tx) Statuses() storage.StatusRepo            { return &StatusesRepo{q: t.tx} }
func (t *Tx) Properties() storage.PropertyRepo        { return &PropertiesRepo{q: t.tx} }
func (t *Tx) ModelVersions() storage.ModelVersionRepo { return &ModelVersionsRepo{q: t.tx} }
func (t *Tx) Contracts() storage.ContractRepo         { return &ContractsRepo{q: t.tx} }
func (t *Tx) Objects() storage.ObjectRepo             { return &ObjectsRepo{q: t.tx} }
func (t *Tx) Graph() storage.GraphRepo                { return &GraphRepo{q: t.tx} }

func (t *Tx) Commit(ctx context.Context) error {
	err := t.tx.Commit()
	if t.cancel != nil {
		t.cancel()
	}
	return err
}

func (t *Tx) Rollback(ctx context.Context) error {
	err := t.tx.Rollback()
	if t.cancel != nil {
		t.cancel()
	}
	return err
}
