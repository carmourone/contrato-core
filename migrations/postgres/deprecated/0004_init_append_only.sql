-- Postgres v0 init: tenant explicit, TEXT type/status constrained by registries, append-only versioning.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL UNIQUE,
  version     INTEGER NOT NULL DEFAULT 1,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS types (
  tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain     TEXT NOT NULL,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, domain, name)
);

CREATE TABLE IF NOT EXISTS statuses (
  tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain     TEXT NOT NULL,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, domain, name)
);

CREATE TABLE IF NOT EXISTS model_versions (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  model_id     UUID NOT NULL DEFAULT gen_random_uuid(),
  version      INTEGER NOT NULL,
  status       TEXT NOT NULL,
  change_note  TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, model_id, version),
  CONSTRAINT fk_model_version_status FOREIGN KEY (tenant_id, 'model_version', status)
    REFERENCES statuses(tenant_id, domain, name)
);

CREATE INDEX IF NOT EXISTS idx_model_versions_latest
  ON model_versions (tenant_id, model_id, version DESC);

CREATE INDEX IF NOT EXISTS idx_model_versions_enabled
  ON model_versions (tenant_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS properties (
  tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_type  TEXT NOT NULL,
  owner_id    UUID NOT NULL,
  key         TEXT NOT NULL,
  value       JSONB NOT NULL DEFAULT '{}'::jsonb,
  model_id    UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version     INTEGER NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, owner_type, owner_id, key, version),
  FOREIGN KEY (tenant_id, model_id, model_version) REFERENCES model_versions(tenant_id, model_id, version),
  FOREIGN KEY (tenant_id, model_id, model_version) REFERENCES model_versions(tenant_id, model_id, version)
);
CREATE INDEX IF NOT EXISTS idx_properties_latest
  ON properties (tenant_id, owner_type, owner_id, key, version DESC);

CREATE TABLE IF NOT EXISTS properties (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_type    TEXT NOT NULL,
  owner_id      UUID NOT NULL,
  key           TEXT NOT NULL,

  value_float   DOUBLE PRECISION NULL,
  value_int     BIGINT NULL,
  value_text    TEXT NULL,
  value_json    JSONB NULL,
  value_bytes   BYTEA NULL,

  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,

  version       INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, owner_type, owner_id, key, version),
  FOREIGN KEY (tenant_id, model_id, model_version) REFERENCES model_versions(tenant_id, model_id, version),

  CONSTRAINT chk_parameters_one_value CHECK (
    (CASE WHEN value_float IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN value_int   IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN value_text  IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN value_json  IS NULL THEN 0 ELSE 1 END) +
    (CASE WHEN value_bytes IS NULL THEN 0 ELSE 1 END)
    <= 1
  )
);
CREATE INDEX IF NOT EXISTS idx_parameters_latest
  ON properties (tenant_id, owner_type, owner_id, key, version DESC);

CREATE TABLE IF NOT EXISTS contracts (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id           UUID NOT NULL DEFAULT gen_random_uuid(),
  domain       TEXT NOT NULL DEFAULT 'contract',
  type         TEXT NOT NULL,
  status       TEXT NOT NULL,
  version      INTEGER NOT NULL,
  blob         BYTEA NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id, version),
  CONSTRAINT ck_contract_domain CHECK (domain = 'contract'),
  CONSTRAINT fk_contract_type FOREIGN KEY (tenant_id, domain, type)
    REFERENCES types(tenant_id, domain, name),
  CONSTRAINT fk_contract_model FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version),
  CONSTRAINT fk_contract_status FOREIGN KEY (tenant_id, domain, status)
    REFERENCES statuses(tenant_id, domain, name)
);
CREATE INDEX IF NOT EXISTS idx_contracts_latest
  ON contracts (tenant_id, id, version DESC);
CREATE INDEX IF NOT EXISTS idx_contracts_by_type_latest
  ON contracts (tenant_id, domain, type, id, version DESC);

CREATE TABLE IF NOT EXISTS reason_codes (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain       TEXT NOT NULL DEFAULT 'contract',
  code         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, domain, code)
);

CREATE TABLE IF NOT EXISTS contract_reasons (
  tenant_id        UUID NOT NULL,
  contract_id      UUID NOT NULL,
  contract_version INTEGER NOT NULL,
  domain           TEXT NOT NULL DEFAULT 'contract',
  code             TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, contract_id, contract_version, domain, code),
  FOREIGN KEY (tenant_id, contract_id, contract_version)
    REFERENCES contracts(tenant_id, id, version) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, domain, code)
    REFERENCES reason_codes(tenant_id, domain, code)
);

CREATE TABLE IF NOT EXISTS contract_obligations (
  tenant_id        UUID NOT NULL,
  contract_id      UUID NOT NULL,
  contract_version INTEGER NOT NULL,
  obligation       TEXT NOT NULL,
  params           JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, contract_id, contract_version, obligation),
  FOREIGN KEY (tenant_id, contract_id, contract_version)
    REFERENCES contracts(tenant_id, id, version) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, 'obligation', obligation)
    REFERENCES types(tenant_id, domain, name)
);

CREATE TABLE IF NOT EXISTS objects (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id       UUID NOT NULL, -- node stable id (graph_nodes.id)
  bytes         BYTEA NULL,
  exturl        TEXT NULL,
  etag          TEXT NOT NULL DEFAULT '',
  version       INTEGER NOT NULL,
  expires_at    TIMESTAMPTZ NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_objects_bytes_or_url CHECK (
    bytes IS NOT NULL OR exturl IS NOT NULL
  )
);
CREATE INDEX IF NOT EXISTS idx_objects_node_latest
  ON objects (tenant_id, node_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_objects_tenant_expires
  ON objects (tenant_id, expires_at);

CREATE TABLE IF NOT EXISTS graph_nodes (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id           UUID NOT NULL DEFAULT gen_random_uuid(),
  domain       TEXT NOT NULL DEFAULT 'node',
  type         TEXT NOT NULL,
  model_id     UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version      INTEGER NOT NULL,
  blob         BYTEA NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id, version),
  CONSTRAINT ck_node_domain CHECK (domain = 'node'),
  CONSTRAINT fk_node_model FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version),
  CONSTRAINT fk_node_type FOREIGN KEY (tenant_id, domain, type)
    REFERENCES types(tenant_id, domain, name)
);
CREATE INDEX IF NOT EXISTS idx_nodes_latest
  ON graph_nodes (tenant_id, id, version DESC);
CREATE INDEX IF NOT EXISTS idx_nodes_by_type_latest
  ON graph_nodes (tenant_id, domain, type, id, version DESC);

CREATE TABLE IF NOT EXISTS graph_edges (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  from_id      UUID NOT NULL,
  to_id        UUID NOT NULL,
  domain       TEXT NOT NULL DEFAULT 'edge',
  type         TEXT NOT NULL,
  model_id     UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version      INTEGER NOT NULL,
  blob         BYTEA NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, from_id, to_id, domain, type, version),
  CONSTRAINT ck_edge_domain CHECK (domain = 'edge'),
  CONSTRAINT fk_edge_model FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version),
  CONSTRAINT fk_edge_type FOREIGN KEY (tenant_id, domain, type)
    REFERENCES types(tenant_id, domain, name)
);
CREATE INDEX IF NOT EXISTS idx_edges_latest
  ON graph_edges (tenant_id, from_id, to_id, domain, type, version DESC);
CREATE INDEX IF NOT EXISTS idx_edges_by_from_latest
  ON graph_edges (tenant_id, from_id, domain, type, to_id, version DESC);
