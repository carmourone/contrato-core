-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL UNIQUE,
  version    INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
  tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  model_id    UUID NOT NULL DEFAULT gen_random_uuid(),
  version     INTEGER NOT NULL,
  status      TEXT NOT NULL,
  change_note TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, model_id, version)
);
CREATE INDEX IF NOT EXISTS idx_model_versions_enabled
  ON model_versions(tenant_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS graph_nodes (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id            UUID NOT NULL DEFAULT gen_random_uuid(),
  domain        TEXT NOT NULL DEFAULT 'node',
  type          TEXT NOT NULL,
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version       INTEGER NOT NULL,
  blob          JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id, version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_latest
  ON graph_nodes(tenant_id, id, version DESC);

CREATE TABLE IF NOT EXISTS graph_edges (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  from_id       UUID NOT NULL,
  to_id         UUID NOT NULL,
  domain        TEXT NOT NULL DEFAULT 'edge',
  type          TEXT NOT NULL,
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version       INTEGER NOT NULL,
  blob          JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, from_id, to_id, domain, type, version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_graph_edges_latest
  ON graph_edges(tenant_id, from_id, to_id, domain, type, version DESC);
CREATE INDEX IF NOT EXISTS idx_graph_edges_from
  ON graph_edges(tenant_id, from_id, domain, type);

CREATE TABLE IF NOT EXISTS properties (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_type    TEXT NOT NULL,
  owner_id      UUID NOT NULL,
  key           TEXT NOT NULL,
  value         JSONB NOT NULL DEFAULT '{}',
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version       INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, owner_type, owner_id, key, version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_properties_latest
  ON properties(tenant_id, owner_type, owner_id, key, version DESC);

CREATE TABLE IF NOT EXISTS contracts (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id            UUID NOT NULL DEFAULT gen_random_uuid(),
  domain        TEXT NOT NULL DEFAULT 'contract',
  type          TEXT NOT NULL,
  status        TEXT NOT NULL,
  action        TEXT,
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  version       INTEGER NOT NULL,
  blob          JSONB NOT NULL DEFAULT '{}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id, version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_contracts_latest
  ON contracts(tenant_id, id, version DESC);
CREATE INDEX IF NOT EXISTS idx_contracts_by_type
  ON contracts(tenant_id, domain, type, id, version DESC);

CREATE TABLE IF NOT EXISTS objects (
  tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id    UUID NOT NULL,
  bytes      BYTEA,
  exturl     TEXT,
  etag       TEXT NOT NULL DEFAULT '',
  version    INTEGER NOT NULL,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_objects_content CHECK (bytes IS NOT NULL OR exturl IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_objects_node_latest
  ON objects(tenant_id, node_id, version DESC);

CREATE TABLE IF NOT EXISTS reason_codes (
  tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain      TEXT NOT NULL DEFAULT 'contract',
  code        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
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
  params           JSONB NOT NULL DEFAULT '{}',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, contract_id, contract_version, obligation),
  FOREIGN KEY (tenant_id, contract_id, contract_version)
    REFERENCES contracts(tenant_id, id, version) ON DELETE CASCADE
);
