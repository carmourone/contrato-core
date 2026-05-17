package sqlstore

import (
	"context"
	"database/sql"
	"time"

	pgvector "github.com/pgvector/pgvector-go"

	"contrato/internal/storage"
)

type EmbeddingsRepo struct{ q *sql.DB }

func (r *EmbeddingsRepo) SetNodeEmbedding(ctx context.Context, tenantID, nodeID, model string, vec []float32) error {
	_, err := r.q.ExecContext(ctx, `
INSERT INTO node_embeddings (tenant_id, node_id, model, embedding, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (tenant_id, node_id) DO UPDATE
    SET model=EXCLUDED.model, embedding=EXCLUDED.embedding, updated_at=now()
`, tenantID, nodeID, model, pgvector.NewVector(vec))
	return err
}

func (r *EmbeddingsRepo) SearchNodes(ctx context.Context, tenantID string, vec []float32, limit int) ([]storage.NodeMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.q.QueryContext(ctx, `
WITH latest AS (
    SELECT DISTINCT ON (tenant_id, id)
        tenant_id, id, domain, type, model_id, model_version, version, blob, created_at, updated_at
    FROM graph_nodes
    WHERE tenant_id = $1
    ORDER BY tenant_id, id, version DESC
)
SELECT l.tenant_id, l.id, l.domain, l.type, l.model_id, l.model_version, l.version,
       l.blob, l.created_at, l.updated_at,
       1 - (ne.embedding <=> $2) AS similarity
FROM node_embeddings ne
JOIN latest l ON l.tenant_id = ne.tenant_id AND l.id = ne.node_id
WHERE ne.tenant_id = $1
ORDER BY ne.embedding <=> $2
LIMIT $3
`, tenantID, pgvector.NewVector(vec), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.NodeMatch
	for rows.Next() {
		var m storage.NodeMatch
		if err := rows.Scan(
			&m.TenantID, &m.ID, &m.Domain, &m.Type,
			&m.ModelID, &m.ModelVersion, &m.Version,
			&m.Blob, &m.CreatedAt, &m.UpdatedAt,
			&m.Similarity,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *EmbeddingsRepo) SetEdgeEmbedding(ctx context.Context, tenantID, fromID, toID, domain, typ, model string, vec []float32) error {
	_, err := r.q.ExecContext(ctx, `
INSERT INTO edge_embeddings (tenant_id, from_id, to_id, domain, type, model, embedding, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (tenant_id, from_id, to_id, domain, type) DO UPDATE
    SET model=EXCLUDED.model, embedding=EXCLUDED.embedding, updated_at=now()
`, tenantID, fromID, toID, domain, typ, model, pgvector.NewVector(vec))
	return err
}

func (r *EmbeddingsRepo) SearchEdges(ctx context.Context, tenantID string, vec []float32, limit int) ([]storage.EdgeMatch, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.q.QueryContext(ctx, `
WITH latest AS (
    SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
        tenant_id, from_id, to_id, domain, type, model_id, model_version, version, blob, created_at, updated_at
    FROM graph_edges
    WHERE tenant_id = $1
    ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
)
SELECT l.tenant_id, l.from_id, l.to_id, l.domain, l.type,
       l.model_id, l.model_version, l.version,
       l.blob, l.created_at, l.updated_at,
       1 - (ee.embedding <=> $2) AS similarity
FROM edge_embeddings ee
JOIN latest l ON l.tenant_id = ee.tenant_id
    AND l.from_id = ee.from_id
    AND l.to_id   = ee.to_id
    AND l.domain  = ee.domain
    AND l.type    = ee.type
WHERE ee.tenant_id = $1
ORDER BY ee.embedding <=> $2
LIMIT $3
`, tenantID, pgvector.NewVector(vec), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.EdgeMatch
	for rows.Next() {
		var m storage.EdgeMatch
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&m.TenantID, &m.FromID, &m.ToID, &m.Domain, &m.Type,
			&m.ModelID, &m.ModelVersion, &m.Version,
			&m.Blob, &createdAt, &updatedAt,
			&m.Similarity,
		); err != nil {
			return nil, err
		}
		m.CreatedAt = createdAt
		m.UpdatedAt = updatedAt
		out = append(out, m)
	}
	return out, rows.Err()
}
