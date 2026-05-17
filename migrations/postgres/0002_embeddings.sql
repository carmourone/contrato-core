-- +goose Up

-- pgvector extension: requires superuser (or rds_superuser on RDS).
-- If the migration fails here, run this manually as a superuser first:
--   CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS vector;

-- Embeddings are stored as a mutable upsert table separate from the
-- append-only graph tables. Each node/edge has at most one embedding row.
-- The `model` column records which embedding model produced the vector
-- so callers can guard against mixing dimensions across models.

CREATE TABLE IF NOT EXISTS node_embeddings (
    tenant_id   TEXT         NOT NULL,
    node_id     TEXT         NOT NULL,
    model       TEXT         NOT NULL DEFAULT '',
    embedding   vector       NOT NULL,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, node_id)
);

-- Uncomment and set the dimension that matches your embedding model, then
-- recreate to get HNSW approximate nearest-neighbour speed:
--   nomic-embed-text / all-mpnet-base-v2  → 768
--   all-MiniLM-L6-v2                      → 384
--   mxbai-embed-large / bge-m3            → 1024
-- CREATE INDEX node_embeddings_hnsw ON node_embeddings
--     USING hnsw ((embedding::vector(768)) vector_cosine_ops);

CREATE TABLE IF NOT EXISTS edge_embeddings (
    tenant_id   TEXT         NOT NULL,
    from_id     TEXT         NOT NULL,
    to_id       TEXT         NOT NULL,
    domain      TEXT         NOT NULL DEFAULT '',
    type        TEXT         NOT NULL DEFAULT '',
    model       TEXT         NOT NULL DEFAULT '',
    embedding   vector       NOT NULL,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, from_id, to_id, domain, type)
);

-- CREATE INDEX edge_embeddings_hnsw ON edge_embeddings
--     USING hnsw ((embedding::vector(768)) vector_cosine_ops);
