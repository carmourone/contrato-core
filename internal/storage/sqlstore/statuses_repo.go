package sqlstore

import (
	"context"
	"database/sql"

	"contrato/internal/storage"
)

type StatusesRepo struct{ q querier }

func (r *StatusesRepo) Create(ctx context.Context, s storage.Status) (storage.Status, error) {
	row := r.q.QueryRowContext(ctx, `
INSERT INTO statuses(tenant_id, domain, name) VALUES($1,$2,$3)
ON CONFLICT (tenant_id, domain, name) DO UPDATE SET name=EXCLUDED.name
RETURNING tenant_id, domain, name
`, s.TenantID, s.Domain, s.Name)
	var out storage.Status
	if err := row.Scan(&out.TenantID, &out.Domain, &out.Name); err != nil {
		return storage.Status{}, err
	}
	return out, nil
}

func (r *StatusesRepo) GetByName(ctx context.Context, tenantID, domain, name string) (storage.Status, error) {
	row := r.q.QueryRowContext(ctx, `SELECT tenant_id, domain, name FROM statuses WHERE tenant_id=$1 AND domain=$2 AND name=$3`, tenantID, domain, name)
	var out storage.Status
	if err := row.Scan(&out.TenantID, &out.Domain, &out.Name); err != nil {
		if err == sql.ErrNoRows { return storage.Status{}, storage.ErrNotFound }
		return storage.Status{}, err
	}
	return out, nil
}
