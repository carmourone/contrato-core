package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"contrato/internal/storage"
)

type GraphRepo struct{ q querier }

func (r *GraphRepo) GetNode(ctx context.Context, tenantID, id string) (storage.Node, error) {
	row := r.q.QueryRowContext(ctx, `
SELECT tenant_id, id, domain, type, model_id, model_version, version, blob, created_at
FROM graph_nodes
WHERE tenant_id=$1 AND id=$2
ORDER BY version DESC
LIMIT 1
`, tenantID, id)

	var n storage.Node
	var created time.Time
	if err := row.Scan(&n.TenantID, &n.ID, &n.Domain, &n.Type, &n.ModelID, &n.ModelVersion, &n.Version, &n.Blob, &created); err != nil {
		if err == sql.ErrNoRows { return storage.Node{}, storage.ErrNotFound }
		return storage.Node{}, err
	}
	n.CreatedAt = created
	return n, nil
}

func (r *GraphRepo) PutNode(ctx context.Context, n storage.Node, opts storage.PutOptions) (storage.Node, error) {
	if n.TenantID == "" { return storage.Node{}, errors.New("tenant_id required") }
	if n.Domain == "" { n.Domain = "node" }
	if n.Type == "" { return storage.Node{}, errors.New("type required") }
	if len(n.Blob) == 0 { n.Blob = []byte(`{}`) }

	if n.ModelID == "" || n.ModelVersion == 0 {
		mv, err := (&ModelVersionsRepo{q: r.q}).GetLatestEnabled(ctx, n.TenantID)
		if err != nil { return storage.Node{}, errors.New("model_id/model_version required (no enabled model)") }
		n.ModelID, n.ModelVersion = mv.ModelID, mv.Version
	}

	if n.ID == "" {
		row := r.q.QueryRowContext(ctx, `
INSERT INTO graph_nodes(tenant_id, domain, type, model_id, model_version, version, blob)
VALUES($1,$2,$3,$4,$5,1,$6)
RETURNING tenant_id, id, domain, type, version, blob, created_at
`, n.TenantID, n.Domain, n.Type, n.ModelID, n.ModelVersion, n.Blob)
		return scanNode(row)
	}

	if opts.ExpectedVersion != nil {
		row := r.q.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM graph_nodes WHERE tenant_id=$1 AND id=$2`, n.TenantID, n.ID)
		latest, err := scanNullInt(row)
		if err != nil { return storage.Node{}, err }
		if latest != *opts.ExpectedVersion { return storage.Node{}, storage.ErrConflict }
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM graph_nodes
  WHERE tenant_id=$1 AND id=$2
)
INSERT INTO graph_nodes(tenant_id, id, domain, type, model_id, model_version, version, blob)
SELECT $1,$2,$3,$4,$5,$6, next.v, $7
FROM next
RETURNING tenant_id, id, domain, type, version, blob, created_at
`, n.TenantID, n.ID, n.Domain, n.Type, n.ModelID, n.ModelVersion, n.Blob)
	return scanNode(row)
}

func (r *GraphRepo) PutEdge(ctx context.Context, e storage.Edge, opts storage.PutOptions) (storage.Edge, error) {
	if e.TenantID == "" { return storage.Edge{}, errors.New("tenant_id required") }
	if e.Domain == "" { e.Domain = "edge" }
	if e.Type == "" { return storage.Edge{}, errors.New("type required") }
	if e.FromID == "" || e.ToID == "" { return storage.Edge{}, errors.New("from_id and to_id required") }
	if len(e.Blob) == 0 { e.Blob = []byte(`{}`) }

	if e.ModelID == "" || e.ModelVersion == 0 {
		mv, err := (&ModelVersionsRepo{q: r.q}).GetLatestEnabled(ctx, e.TenantID)
		if err != nil { return storage.Edge{}, errors.New("model_id/model_version required (no enabled model)") }
		e.ModelID, e.ModelVersion = mv.ModelID, mv.Version
	}

	if opts.ExpectedVersion != nil {
		row := r.q.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version),0)
FROM graph_edges
WHERE tenant_id=$1 AND from_id=$2 AND to_id=$3 AND domain=$4 AND type=$5
`, e.TenantID, e.FromID, e.ToID, e.Domain, e.Type)
		latest, err := scanNullInt(row)
		if err != nil { return storage.Edge{}, err }
		if latest != *opts.ExpectedVersion { return storage.Edge{}, storage.ErrConflict }
	}

	row := r.q.QueryRowContext(ctx, `
WITH next AS (
  SELECT COALESCE(MAX(version),0)+1 AS v
  FROM graph_edges
  WHERE tenant_id=$1 AND from_id=$2 AND to_id=$3 AND domain=$4 AND type=$5
)
INSERT INTO graph_edges(tenant_id, from_id, to_id, domain, type, model_id, model_version, version, blob)
SELECT $1,$2,$3,$4,$5,$6,$7, next.v, $8
FROM next
RETURNING tenant_id, from_id, to_id, domain, type, model_id, model_version, version, blob, created_at
`, e.TenantID, e.FromID, e.ToID, e.Domain, e.Type, e.ModelID, e.ModelVersion, e.Blob)

	return scanEdge(row)
}

func (r *GraphRepo) OutEdges(ctx context.Context, tenantID, fromID, domain, typ string, page storage.Page) ([]storage.Edge, string, error) {
	limit := page.Limit
	if limit <= 0 || limit > 500 { limit = 100 }
	if domain == "" { domain = "edge" }

	rows, err := r.q.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
  tenant_id, from_id, to_id, domain, type, version, blob, created_at
FROM graph_edges
WHERE tenant_id=$1 AND from_id=$2 AND domain=$3 AND type=$4
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
LIMIT $5
`, tenantID, fromID, domain, typ, limit)
	if err != nil { return nil, "", err }
	defer rows.Close()

	edges := []storage.Edge{}
	for rows.Next() {
		var e storage.Edge
		var created time.Time
		if err := rows.Scan(&e.TenantID, &e.FromID, &e.ToID, &e.Domain, &e.Type, &e.ModelID, &e.ModelVersion, &e.Version, &e.Blob, &created); err != nil {
			return nil, "", err
		}
		e.CreatedAt = created
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil { return nil, "", err }
	return edges, "", nil
}

func scanNode(row *sql.Row) (storage.Node, error) {
	var n storage.Node
	var created time.Time
	if err := row.Scan(&n.TenantID, &n.ID, &n.Domain, &n.Type, &n.ModelID, &n.ModelVersion, &n.Version, &n.Blob, &created); err != nil {
		if err == sql.ErrNoRows { return storage.Node{}, storage.ErrNotFound }
		return storage.Node{}, err
	}
	n.CreatedAt = created
	return n, nil
}

func scanEdge(row *sql.Row) (storage.Edge, error) {
	var e storage.Edge
	var created time.Time
	if err := row.Scan(&e.TenantID, &e.FromID, &e.ToID, &e.Domain, &e.Type, &e.ModelID, &e.ModelVersion, &e.Version, &e.Blob, &created); err != nil {
		if err == sql.ErrNoRows { return storage.Edge{}, storage.ErrNotFound }
		return storage.Edge{}, err
	}
	e.CreatedAt = created
	return e, nil
}
