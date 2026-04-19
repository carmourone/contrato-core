package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"contrato/internal/storage"
)

type ModelVersionsRepo struct{ q querier }

func (r *ModelVersionsRepo) Create(ctx context.Context, mv storage.ModelVersion) (storage.ModelVersion, error) {
	if mv.TenantID == "" { return storage.ModelVersion{}, errors.New("tenant_id required") }
	if mv.Status == "" { mv.Status = "draft" }
	if mv.ChangeNote == "" { mv.ChangeNote = "" }

	if mv.ModelID == "" {
		row := r.q.QueryRowContext(ctx, `
INSERT INTO model_versions(tenant_id, status, change_note, version)
VALUES($1,$2,$3,1)
RETURNING tenant_id, model_id, version, status, change_note, created_at
`, mv.TenantID, mv.Status, mv.ChangeNote)
		return scanModelVersion(row)
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM model_versions
  WHERE tenant_id=$1 AND model_id=$2
)
INSERT INTO model_versions(tenant_id, model_id, version, status, change_note)
SELECT $1,$2, next.v, $3, $4
FROM next
RETURNING tenant_id, model_id, version, status, change_note, created_at
`, mv.TenantID, mv.ModelID, mv.Status, mv.ChangeNote)
	return scanModelVersion(row)
}

func (r *ModelVersionsRepo) Get(ctx context.Context, tenantID, modelID string, version int) (storage.ModelVersion, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, model_id, version, status, change_note, created_at
FROM model_versions
WHERE tenant_id=$1 AND model_id=$2 AND version=$3
`, tenantID, modelID, version)
	return scanModelVersion(row)
}

func (r *ModelVersionsRepo) GetLatestEnabled(ctx context.Context, tenantID string) (storage.ModelVersion, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, model_id, version, status, change_note, created_at
FROM model_versions
WHERE tenant_id=$1 AND status='enabled'
ORDER BY created_at DESC
LIMIT 1
`, tenantID)
	return scanModelVersion(row)
}

func (r *ModelVersionsRepo) List(ctx context.Context, tenantID string, modelID string, page storage.Page) ([]storage.ModelVersion, string, error) {
	limit := page.Limit
	if limit <= 0 || limit > 200 { limit = 50 }

	var rows *sql.Rows
	var err error
	if modelID == "" {
		rows, err = r.q.QueryContext(ctx, `
SELECT tenant_id, model_id, version, status, change_note, created_at
FROM model_versions
WHERE tenant_id=$1
ORDER BY model_id, version DESC
LIMIT $2
`, tenantID, limit)
	} else {
		rows, err = r.q.QueryContext(ctx, `
SELECT tenant_id, model_id, version, status, change_note, created_at
FROM model_versions
WHERE tenant_id=$1 AND model_id=$2
ORDER BY model_id, version DESC
LIMIT $3
`, tenantID, modelID, limit)
	}
	if err != nil { return nil, "", err }
	defer rows.Close()

	out := []storage.ModelVersion{}
	for rows.Next() {
		var mv storage.ModelVersion
		var created time.Time
		if err := rows.Scan(&mv.TenantID, &mv.ModelID, &mv.Version, &mv.Status, &mv.ChangeNote, &created); err != nil {
			return nil, "", err
		}
		mv.CreatedAt = created
		out = append(out, mv)
	}
	if err := rows.Err(); err != nil { return nil, "", err }
	return out, "", nil
}

func scanModelVersion(row *sql.Row) (storage.ModelVersion, error) {
	var mv storage.ModelVersion
	var created time.Time
	if err := row.Scan(&mv.TenantID, &mv.ModelID, &mv.Version, &mv.Status, &mv.ChangeNote, &created); err != nil {
		if err == sql.ErrNoRows { return storage.ModelVersion{}, storage.ErrNotFound }
		return storage.ModelVersion{}, err
	}
	mv.CreatedAt = created
	return mv, nil
}
