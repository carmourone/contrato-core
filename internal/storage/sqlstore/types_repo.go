package sqlstore

import (
	"context"
	"database/sql"

	"contrato/internal/storage"
)

type TypesRepo struct{ q querier }

func (r *TypesRepo) Create(ctx context.Context, t storage.Type) (storage.Type, error) {
	row := r.q.QueryRowContext(ctx, `
INSERT INTO types(tenant_id, domain, name) VALUES($1,$2,$3)
ON CONFLICT (tenant_id, domain, name) DO NOTHING
RETURNING tenant_id, domain, name
`, t.TenantID, t.Domain, t.Name)
	var out storage.Type
	if err := row.Scan(&out.TenantID, &out.Domain, &out.Name); err != nil { return storage.Type{}, err }
	return out, nil
}

func (r *TypesRepo) GetByName(ctx context.Context, tenantID, domain, name string) (storage.Type, error) {
	row := r.q.QueryRowContext(ctx, `SELECT tenant_id, domain, name FROM types WHERE tenant_id=$1 AND domain=$2 AND name=$3`, tenantID, domain, name)
	var out storage.Type
	if err := row.Scan(&out.TenantID, &out.Domain, &out.Name); err != nil {
		if err == sql.ErrNoRows { return storage.Type{}, storage.ErrNotFound }
		return storage.Type{}, err
	}
	return out, nil
}
