package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"contrato/internal/storage"
)

type ParametersRepo struct{ q querier }

func (r *ParametersRepo) Get(ctx context.Context, tenantID, ownerType, ownerID, key string) (storage.Parameter, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, owner_type, owner_id, key, value_float, value_int, value_text, value_json, value_bytes, model_id, model_version, version, created_at, updated_at
FROM parameters
WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
ORDER BY version DESC
LIMIT 1
`, tenantID, ownerType, ownerID, key)

	var m storage.Parameter
	var vf sql.NullFloat64
	var vi sql.NullInt64
	var vt sql.NullString
	var vj []byte
	var created time.Time
	var updated time.Time
	if err := row.Scan(&m.TenantID, &m.OwnerType, &m.OwnerID, &m.Key, &vf, &vi, &vt, &vj, &m.ModelID, &m.ModelVersion, &m.Version, &created); err != nil {
		if err == sql.ErrNoRows { return storage.Parameter{}, storage.ErrNotFound }
		return storage.Parameter{}, err
	}
	if vf.Valid { m.Float = &vf.Float64 }
	if vi.Valid { m.Int = &vi.Int64 }
	if vt.Valid { m.Text = &vt.String }
	m.JSON = vj
	m.CreatedAt = created
	return m, nil
}

func (r *ParametersRepo) Put(ctx context.Context, m storage.Parameter, opts storage.PutOptions) (storage.Parameter, error) {
	if m.TenantID == "" || m.OwnerType == "" || m.OwnerID == "" || m.Key == "" {
		return storage.Parameter{}, errors.New("tenant_id, owner_type, owner_id, key required")
	}

	var vf any = nil
	var vi any = nil
	var vt any = nil
	var vj any = nil
	if m.Float != nil { vf = *m.Float }
	if m.Int != nil { vi = *m.Int }
	if m.Text != nil { vt = *m.Text }
	if len(m.JSON) > 0 { vj = m.JSON }

	if m.ModelID == "" || m.ModelVersion == 0 {
		mv, err := (&ModelVersionsRepo{q: r.q}).GetLatestEnabled(ctx, m.TenantID)
		if err != nil { return storage.Parameter{}, errors.New("model_id/model_version required (no enabled model)") }
		m.ModelID, m.ModelVersion = mv.ModelID, mv.Version
	}

	if opts.ExpectedVersion != nil {
		row := r.q.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version),0) FROM parameters
WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
`, m.TenantID, m.OwnerType, m.OwnerID, m.Key)
		latest, err := scanNullInt(row)
		if err != nil { return storage.Parameter{}, err }
		if latest != *opts.ExpectedVersion { return storage.Parameter{}, storage.ErrConflict }
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM parameters
  WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
)
INSERT INTO parameters(tenant_id, owner_type, owner_id, key, value_float, value_int, value_text, value_json, value_bytes, model_id, model_version, version)
SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10, next.v
FROM next
RETURNING tenant_id, owner_type, owner_id, key, value_float, value_int, value_text, value_json, value_bytes, model_id, model_version, version, created_at, updated_at
`, m.TenantID, m.OwnerType, m.OwnerID, m.Key, vf, vi, vt, vj, vb, m.ModelID, m.ModelVersion)

	var out storage.Parameter
	var ovf sql.NullFloat64
	var ovi sql.NullInt64
	var ovt sql.NullString
	var ovj []byte
	var ovb []byte
	var created time.Time
	var updated time.Time
	var updated time.Time
	if err := row.Scan(&out.TenantID, &out.OwnerType, &out.OwnerID, &out.Key, &ovf, &ovi, &ovt, &ovj, &ovb, &out.ModelID, &out.ModelVersion, &out.Version, &created, &updated); err != nil {
		return storage.Parameter{}, err
	}
	if ovf.Valid { out.Float = &ovf.Float64 }
	if ovi.Valid { out.Int = &ovi.Int64 }
	if ovt.Valid { out.Text = &ovt.String }
	out.JSON = ovj
	out.Bytes = ovb
	out.CreatedAt = created
	out.UpdatedAt = updated
	return out, nil
}
