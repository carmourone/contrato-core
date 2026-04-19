-- Contrato unified initial schema (single init)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tenants
CREATE TABLE IF NOT EXISTS tenants (
  id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE
);

-- Registries
CREATE TABLE IF NOT EXISTS types (
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain    TEXT NOT NULL,            -- e.g. node, edge, contract_action, obligation
  name      TEXT NOT NULL,            -- e.g. capability, provides
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, domain, name)
);

CREATE TABLE IF NOT EXISTS statuses (
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  domain    TEXT NOT NULL,            -- e.g. contract, model_version
  name      TEXT NOT NULL,            -- e.g. enabled, request
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, domain, name)
);

-- Model versions (append-only)
CREATE TABLE IF NOT EXISTS model_versions (
  tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  model_id     UUID NOT NULL DEFAULT gen_random_uuid(),
  version      INTEGER NOT NULL,
  status_domain TEXT NOT NULL DEFAULT 'model_version',
  status       TEXT NOT NULL,
  change_note  TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, model_id, version),
  FOREIGN KEY (tenant_id, status_domain, status)
    REFERENCES statuses(tenant_id, domain, name)
);

-- Nodes (append-only by model_version)
CREATE TABLE IF NOT EXISTS nodes (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id            UUID NOT NULL DEFAULT gen_random_uuid(),
  type_domain   TEXT NOT NULL DEFAULT 'node',
  type          TEXT NOT NULL,
  blob          JSONB NOT NULL DEFAULT '{}'::jsonb,
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id, model_id, model_version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, type_domain, type)
    REFERENCES types(tenant_id, domain, name)
);

-- Edges (append-only by model_version)
CREATE TABLE IF NOT EXISTS edges (
  tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  from_id       UUID NOT NULL,
  to_id         UUID NOT NULL,
  type_domain   TEXT NOT NULL DEFAULT 'edge',
  type          TEXT NOT NULL,
  blob          JSONB NOT NULL DEFAULT '{}'::jsonb,
  model_id      UUID NOT NULL,
  model_version INTEGER NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, from_id, to_id, type, model_id, model_version),
  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, type_domain, type)
    REFERENCES types(tenant_id, domain, name)
);

-- Objects: large artifacts referenced from nodes via node_id
CREATE TABLE IF NOT EXISTS objects (
  tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id    UUID NOT NULL,
  bytes      BYTEA,
  exturl     TEXT,
  content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, node_id),
  CHECK (bytes IS NOT NULL OR exturl IS NOT NULL)
);

-- Unified properties (typed values; at most one populated)
CREATE TABLE IF NOT EXISTS properties (
  tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_type  TEXT NOT NULL, -- node|edge|contract|model|policy
  owner_id    UUID NOT NULL,
  key         TEXT NOT NULL,

  value_float DOUBLE PRECISION,
  value_int   BIGINT,
  value_text  TEXT,
  value_json  JSONB,
  value_bytes BYTEA,

  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, owner_type, owner_id, key),

  CHECK (
    (value_float IS NOT NULL)::int +
    (value_int   IS NOT NULL)::int +
    (value_text  IS NOT NULL)::int +
    (value_json  IS NOT NULL)::int +
    (value_bytes IS NOT NULL)::int <= 1
  )
);

CREATE INDEX IF NOT EXISTS idx_properties_owner
  ON properties(tenant_id, owner_type, owner_id);

-- Contracts (append-only)
CREATE TABLE IF NOT EXISTS contracts (
  tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id             UUID NOT NULL DEFAULT gen_random_uuid(),
  domain         TEXT NOT NULL DEFAULT 'contract',
  type_domain    TEXT NOT NULL DEFAULT 'contract',
  type           TEXT NOT NULL,
  status_domain  TEXT NOT NULL DEFAULT 'contract',
  status         TEXT NOT NULL,
  action_domain  TEXT NOT NULL DEFAULT 'contract_action',
  action         TEXT NULL,

  model_id       UUID NOT NULL,
  model_version  INTEGER NOT NULL,

  version        INTEGER NOT NULL,
  blob           JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY (tenant_id, id, version),

  FOREIGN KEY (tenant_id, model_id, model_version)
    REFERENCES model_versions(tenant_id, model_id, version) ON DELETE RESTRICT,

  FOREIGN KEY (tenant_id, type_domain, type)
    REFERENCES types(tenant_id, domain, name),

  FOREIGN KEY (tenant_id, status_domain, status)
    REFERENCES statuses(tenant_id, domain, name),

  FOREIGN KEY (tenant_id, action_domain, action)
    REFERENCES types(tenant_id, domain, name)
);

-- Stable reason codes + attachments
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
  obligation_domain TEXT NOT NULL DEFAULT 'obligation',
  obligation       TEXT NOT NULL,
  params           JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, contract_id, contract_version, obligation),
  FOREIGN KEY (tenant_id, contract_id, contract_version)
    REFERENCES contracts(tenant_id, id, version) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, obligation_domain, obligation)
    REFERENCES types(tenant_id, domain, name)
);
