package sqlstore

import (
	"context"
	"database/sql"

	"contrato/internal/storage"
)

type TenantsRepo struct{ q querier }

func (r *TenantsRepo) Create(ctx context.Context, name string) (storage.Tenant, error) {
	row := r.q.QueryRowContext(ctx, `
INSERT INTO tenants(name) VALUES($1)
ON CONFLICT (name) DO UPDATE SET updated_at=now()
RETURNING id, name, version
`, name)
	var t storage.Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Version); err != nil { return storage.Tenant{}, err }
	return t, nil
}

func (r *TenantsRepo) Get(ctx context.Context, id string) (storage.Tenant, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, name, version FROM tenants WHERE id=$1`, id)
	var t storage.Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Version); err != nil {
		if err == sql.ErrNoRows { return storage.Tenant{}, storage.ErrNotFound }
		return storage.Tenant{}, err
	}
	return t, nil
}

func (r *TenantsRepo) GetByName(ctx context.Context, name string) (storage.Tenant, error) {
	row := r.q.QueryRowContext(ctx, `SELECT id, name, version FROM tenants WHERE name=$1`, name)
	var t storage.Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Version); err != nil {
		if err == sql.ErrNoRows { return storage.Tenant{}, storage.ErrNotFound }
		return storage.Tenant{}, err
	}
	return t, nil
}
