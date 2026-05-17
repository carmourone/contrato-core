package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"contrato/internal/storage"
)

type ContractsRepo struct{ q querier }

func (r *ContractsRepo) Get(ctx context.Context, tenantID, id string) (storage.ContractRecord, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, id, domain, type, status, action, model_id, model_version, version, blob, created_at
FROM contracts
WHERE tenant_id=$1 AND id=$2
ORDER BY version DESC
LIMIT 1
`, tenantID, id)
	return scanContract(row)
}

func (r *ContractsRepo) GetVersion(ctx context.Context, tenantID, id string, version int) (storage.ContractRecord, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, id, domain, type, status, action, model_id, model_version, version, blob, created_at
FROM contracts
WHERE tenant_id=$1 AND id=$2 AND version=$3
`, tenantID, id, version)
	return scanContract(row)
}

func (r *ContractsRepo) Put(ctx context.Context, rec storage.ContractRecord, opts storage.PutOptions) (storage.ContractRecord, error) {
	if rec.TenantID == "" { return storage.ContractRecord{}, errors.New("tenant_id required") }
	if rec.Domain == "" { rec.Domain = "contract" }
	if rec.Type == "" { return storage.ContractRecord{}, errors.New("type required") }
	if rec.Status == "" { return storage.ContractRecord{}, errors.New("status required") }
	if len(rec.Blob) == 0 { rec.Blob = []byte(`{}`) }

	if rec.ModelID == "" || rec.ModelVersion == 0 {
		mv, err := (&ModelVersionsRepo{q: r.q}).GetLatestEnabled(ctx, rec.TenantID)
		if err != nil { return storage.ContractRecord{}, errors.New("model_id/model_version required (no enabled model)") }
		rec.ModelID, rec.ModelVersion = mv.ModelID, mv.Version
	}

	if rec.ID == "" {
		row := r.q.QueryRowContext(ctx, `
INSERT INTO contracts(tenant_id, domain, type, status, action, model_id, model_version, version, blob)
VALUES($1,$2,$3,$4,$5,$6,$7,1,$8)
RETURNING tenant_id, id, domain, type, status, action, model_id, model_version, version, blob, created_at
`, rec.TenantID, rec.Domain, rec.Type, rec.Status, nullableString(rec.Action), rec.ModelID, rec.ModelVersion, rec.Blob)
		return scanContract(row)
	}

	if opts.ExpectedVersion != nil {
		row := r.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM contracts WHERE tenant_id=$1 AND id=$2`, rec.TenantID, rec.ID)
		latest, err := scanNullInt(row)
		if err != nil { return storage.ContractRecord{}, err }
		if latest != *opts.ExpectedVersion { return storage.ContractRecord{}, storage.ErrConflict }
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM contracts
  WHERE tenant_id=$1 AND id=$2
)
INSERT INTO contracts(tenant_id, id, domain, type, status, action, model_id, model_version, version, blob)
SELECT $1,$2,$3,$4,$5,$6,$7,$8, next.v, $9
FROM next
RETURNING tenant_id, id, domain, type, status, action, model_id, model_version, version, blob, created_at
`, rec.TenantID, rec.ID, rec.Domain, rec.Type, rec.Status, nullableString(rec.Action), rec.ModelID, rec.ModelVersion, rec.Blob)
	return scanContract(row)
}

func (r *ContractsRepo) ListByType(ctx context.Context, tenantID, domain, typ string, page storage.Page) ([]storage.ContractRecord, string, error) {
	limit := page.Limit
	if limit <= 0 || limit > 200 { limit = 50 }
	if domain == "" { domain = "contract" }

	rows, err := r.q.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  tenant_id, id, domain, type, status, action, model_id, model_version, version, blob, created_at
FROM contracts
WHERE tenant_id=$1 AND domain=$2 AND type=$3
ORDER BY tenant_id, id, version DESC
LIMIT $4
`, tenantID, domain, typ, limit)
	if err != nil { return nil, "", err }
	defer rows.Close()

	recs := []storage.ContractRecord{}
	for rows.Next() {
		var rec storage.ContractRecord
		var created time.Time
		var action sql.NullString
		if err := rows.Scan(&rec.TenantID, &rec.ID, &rec.Domain, &rec.Type, &rec.Status, &action, &rec.ModelID, &rec.ModelVersion, &rec.Version, &rec.Blob, &created); err != nil {
			return nil, "", err
		}
		if action.Valid { rec.Action = action.String }
		rec.CreatedAt = created
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil { return nil, "", err }
	return recs, "", nil
}

func scanContract(row *sql.Row) (storage.ContractRecord, error) {
	var rec storage.ContractRecord
	var created time.Time
	var action sql.NullString
	if err := row.Scan(&rec.TenantID, &rec.ID, &rec.Domain, &rec.Type, &rec.Status, &action, &rec.ModelID, &rec.ModelVersion, &rec.Version, &rec.Blob, &created); err != nil {
		if err == sql.ErrNoRows { return storage.ContractRecord{}, storage.ErrNotFound }
		return storage.ContractRecord{}, err
	}
	if action.Valid { rec.Action = action.String }
	rec.CreatedAt = created
	return rec, nil
}

func nullableString(s string) sql.NullString {
	if s == "" { return sql.NullString{} }
	return sql.NullString{String: s, Valid: true}
}
