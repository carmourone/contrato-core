-- Properties table: unified properties + metrics + config + evidence
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
