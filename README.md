# Contrato (v0 skeleton)

This revision:
- `type` / `status` columns are **TEXT** and **FK constrained** to tenant-scoped registries:
  - `types(tenant_id, domain, name)`
  - `statuses(tenant_id, domain, name)`

Note: `types` and `statuses` are treated as stable registries (no version column). Rename is done as insert + bulk update + delete.
- Any column ending in `_id` is **UUID**.
- Most mutable tables use **append-only versioning**:
  - logical key + `version` (int)
  - writes insert a new version; reads fetch `MAX(version)`
- `metrics` is a specialised typed KV store (like `properties`) that links to a UUID owner:
  - `owner_type` (text, e.g. "node", "edge", "contract")
  - `owner_id` (uuid)
  - value stored in exactly one of float/int/text/jsonb

## Run
Set:
- `CONTRATO_PG_DSN` (required)
Then:
- `make run`

> AuthN/AuthZ remain NOOP for v0 (guarded).

## Model versioning (replay)

- `model_versions` is append-only: `(tenant_id, model_id, version)`.
- Status is constrained via `statuses` with domain `model_version` and values `draft|enabled|disabled`.
- Domain tables (contracts, nodes, edges, properties, metrics) carry `model_id` + `model_version` FK so you can replay decisions using the exact model.
- Typical flow: write a batch of graph/property changes -> create a new `model_versions` row -> reference it from subsequent writes.

## Model import/export (CLI)

Run the binary in CLI mode using flags:

```bash
# Import a model bundle (creates a new model_version)
go run ./cmd/contrato --import examples/models/service_recovery.json --enable

# Export latest enabled model bundle for a tenant
go run ./cmd/contrato --export /tmp/model.json --tenant demo
```

If neither `--import` nor `--export` is provided, the HTTP service starts normally.

## Contract lifecycle status

Suggested baseline `contract` statuses (in `statuses` registry, domain `contract`):
- `request`
- `active`
- `in_progress`
- `fulfilled`

Because `statuses` is registry-driven, you can extend this per tenant/domain later (e.g. `rejected`, `cancelled`).

## Naming conventions

### Domains
- `node` and `edge` are structural domains.
- `contract` is the contract record domain.
- `model_version` is the model version lifecycle domain.

### Types
Use lower_snake_case in the `types` registry, scoped by `(tenant_id, domain)`.
Examples:
- nodes: `service`, `person`, `policy`, `runbook`, `resource`
- edges: `depends_on`, `owns`, `applies_to`, `governs`

### Recommended node naming fields
IDs are UUIDs in storage, but put a stable human name in the node blob as `name`, and optionally a namespace-style `ref`:
- `ref`: `svc:payments-api`, `user:manager`, `policy:expense-approval:v1`

### Recommended edge blob conventions
Keep edge semantics in `type` and use blob only for properties:
- `type=applies_to`, blob: `{ "scope": "tenant" }`
- `type=depends_on`, blob: `{ "latency_budget_ms": 250 }`

## Contract decision model

Contracts can carry an explicit decision outcome:

- `action`: one of `allow | deny | defer | delegate | require_approval | escalate`
- `obligations`: zero or more follow-up requirements (e.g. `log`, `notify`, `create_approval_task`)
- `reasons`: stable reason codes (machine-readable) supporting audit and analytics

Storage:
- `contracts.action` (nullable) is constrained via `types` with domain `contract_action`.
- `contract_obligations` stores obligations per contract version (FK to `types` domain `obligation`).
- `reason_codes` + `contract_reasons` store stable reason codes per contract version.

This keeps contracts replayable (append-only) while allowing you to reproduce the exact decision context.

## Tenant and model onboarding

Bootstrap a tenant with the core ontology bundle (types/statuses + minimal actor/capability/policy):

```bash
go run ./cmd/contrato --bootstrap examples/core/core_ontology_v0_1.json --tenant demo
```

Then import a domain model bundle (service recovery, expense approval, etc):

```bash
go run ./cmd/contrato --import examples/models/service_recovery.json --enable --tenant demo
```

You can lint a bundle before importing/enabling:

```bash
go run ./cmd/contrato --lint examples/models/service_recovery.json
```

## Graph Resolution Pipeline

Contrato is built around a deterministic, bounded graph resolution pipeline that produces a resolution plan, resolved subgraph/properties, attached policies, and an OPA-ready input/output contract.

See: `docs/graph_resolution_pipeline.md`
