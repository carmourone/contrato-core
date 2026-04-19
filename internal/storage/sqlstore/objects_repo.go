package sqlstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"contrato/internal/storage"
)

type ObjectsRepo struct{ q querier }

// Objects are append-only, versioned blobs associated with a graph node id (stable id across node versions).
// Store either bytes (inline) or an external URL (exturl). At least one must be present.
func (r *ObjectsRepo) GetByNodeID(ctx context.Context, tenantID, nodeID string) (storage.Object, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, id, node_id, bytes, exturl, etag, version, expires_at, created_at, updated_at
FROM objects
WHERE tenant_id=$1 AND node_id=$2
ORDER BY version DESC
LIMIT 1
`, tenantID, nodeID)

	var out storage.Object
	var expires sql.NullTime
	var created time.Time
	var updated time.Time
	var bytes []byte
	var exturl sql.NullString

	if err := row.Scan(&out.TenantID, &out.ID, &out.NodeID, &bytes, &exturl, &out.ETag, &out.Version, &expires, &created, &updated); err != nil {
		if err == sql.ErrNoRows { return storage.Object{}, storage.ErrNotFound }
		return storage.Object{}, err
	}
	out.Bytes = bytes
	if exturl.Valid { out.ExtURL = exturl.String }
	out.CreatedAt = created
	out.UpdatedAt = updated
	if expires.Valid { out.ExpiresAt = expires.Time }
	if !out.ExpiresAt.IsZero() && time.Now().After(out.ExpiresAt) {
		return storage.Object{}, storage.ErrNotFound
	}
	return out, nil
}

func (r *ObjectsRepo) Put(ctx context.Context, obj storage.Object, opts storage.ObjectPutOptions) (storage.Object, error) {
	if obj.TenantID == "" || obj.NodeID == "" {
		return storage.Object{}, errors.New("tenant_id and node_id are required")
	}
	if len(obj.Bytes) == 0 && obj.ExtURL == "" {
		return storage.Object{}, errors.New("either bytes or exturl must be provided")
	}

	// optimistic concurrency: check latest version if requested
	if opts.ExpectedVersion != nil {
		latest, err := LatestVersion(ctx, r.q, "objects", obj.TenantID, "node_id", obj.NodeID)
		if err != nil { return storage.Object{}, err }
		if latest != *opts.ExpectedVersion { return storage.Object{}, storage.ErrConflict }
	}

	payload := obj.Bytes
	if len(payload) == 0 {
		payload = []byte(obj.ExtURL)
	}
	sum := sha256.Sum256(payload)
	etag := hex.EncodeToString(sum[:])

	if opts.IfMatchETag != "" && opts.IfMatchETag != etag {
		return storage.Object{}, storage.ErrConflict
	}

	var expiresAt sql.NullTime
	if opts.TTL > 0 {
		expiresAt = sql.NullTime{Time: time.Now().Add(opts.TTL), Valid: true}
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM objects
  WHERE tenant_id=$1 AND node_id=$2
)
INSERT INTO objects(tenant_id, node_id, bytes, exturl, etag, version, expires_at)
SELECT $1,$2,$3,$4,$5, next.v, $6
FROM next
RETURNING tenant_id, id, node_id, bytes, exturl, etag, version, expires_at, created_at, updated_at
`, obj.TenantID, obj.NodeID, nullBytes(obj.Bytes), nullString(obj.ExtURL), etag, expiresAt)

	var out storage.Object
	var expires sql.NullTime
	var created time.Time
	var updated time.Time
	var bytes []byte
	var exturl sql.NullString

	if err := row.Scan(&out.TenantID, &out.ID, &out.NodeID, &bytes, &exturl, &out.ETag, &out.Version, &expires, &created, &updated); err != nil {
		return storage.Object{}, err
	}
	out.Bytes = bytes
	if exturl.Valid { out.ExtURL = exturl.String }
	out.CreatedAt = created
	out.UpdatedAt = updated
	if expires.Valid { out.ExpiresAt = expires.Time }
	return out, nil
}

func (r *ObjectsRepo) DeleteByNodeID(ctx context.Context, tenantID, nodeID string) error {
	_, err := r.q.ExecContext(ctx, `
DELETE FROM objects
WHERE tenant_id=$1 AND node_id=$2
`, tenantID, nodeID)
	return err
}

// helpers for nullable inserts
func nullString(s string) sql.NullString {
	if s == "" { return sql.NullString{Valid: false} }
	return sql.NullString{String: s, Valid: true}
}
func nullBytes(b []byte) []byte {
	if len(b)==0 { return nil }
	return b
}
