package sqlstore

import (
	"context"
	"database/sql"
	"errors"

	"contrato/internal/storage"
)

type PropertiesRepo struct{ q querier }

func (r *PropertiesRepo) Get(ctx context.Context, tenantID, ownerType, ownerID, key string) (storage.Property, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, owner_type, owner_id, key, value, model_id, model_version, version
FROM properties
WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
ORDER BY version DESC
LIMIT 1
`, tenantID, ownerType, ownerID, key)

	var p storage.Property
	if err := row.Scan(&p.TenantID, &p.OwnerType, &p.OwnerID, &p.Key, &p.ValueJSON, &p.ModelID, &p.ModelVersion, &p.Version); err != nil {
		if err == sql.ErrNoRows { return storage.Property{}, storage.ErrNotFound }
		return storage.Property{}, err
	}
	return p, nil
}

func (r *PropertiesRepo) Put(ctx context.Context, p storage.Property, opts storage.PutOptions) (storage.Property, error) {
	if p.TenantID == "" || p.OwnerType == "" || p.OwnerID == "" || p.Key == "" {
		return storage.Property{}, errors.New("tenant_id, owner_type, owner_id, key required")
	}
	if len(p.ValueJSON) == 0 { p.ValueJSON = []byte(`{}`) }
	if p.ModelID == "" || p.ModelVersion == 0 {
		mv, err := (&ModelVersionsRepo{q: r.q}).GetLatestEnabled(ctx, p.TenantID)
		if err != nil { return storage.Property{}, errors.New("model_id/model_version required (no enabled model)") }
		p.ModelID, p.ModelVersion = mv.ModelID, mv.Version
	}

	if opts.ExpectedVersion != nil {
		row := r.q.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version),0) FROM properties
WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
`, p.TenantID, p.OwnerType, p.OwnerID, p.Key)
		latest, err := scanNullInt(row)
		if err != nil { return storage.Property{}, err }
		if latest != *opts.ExpectedVersion { return storage.Property{}, storage.ErrConflict }
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM properties
  WHERE tenant_id=$1 AND owner_type=$2 AND owner_id=$3 AND key=$4
)
INSERT INTO properties(tenant_id, owner_type, owner_id, key, value, model_id, model_version, version)
SELECT $1,$2,$3,$4,$5,$6,$7, next.v
FROM next
RETURNING tenant_id, owner_type, owner_id, key, value, model_id, model_version, version
`, p.TenantID, p.OwnerType, p.OwnerID, p.Key, p.ValueJSON, p.ModelID, p.ModelVersion)

	var out storage.Property
	if err := row.Scan(&out.TenantID, &out.OwnerType, &out.OwnerID, &out.Key, &out.ValueJSON, &out.ModelID, &out.ModelVersion, &out.Version); err != nil {
		return storage.Property{}, err
	}
	return out, nil
}
