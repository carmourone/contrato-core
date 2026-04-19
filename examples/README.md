## Bootstrap core ontology

```bash
go run ./cmd/contrato --bootstrap examples/core/core_ontology_v0_1.json --tenant demo
```

# Demo model bundles

These JSON files are importable/exportable model bundles.

## Import
Assuming you have `CONTRATO_PG_DSN` set:

```bash
go run ./cmd/contrato --import examples/models/service_recovery.json --enable
go run ./cmd/contrato --import examples/models/expense_approval.json --enable
go run ./cmd/contrato --import examples/models/john_wick.json --enable
```

## Export (latest enabled)
```bash
go run ./cmd/contrato --export /tmp/model.json --tenant demo
```

Notes:
- Demo bundles include contract statuses: request, active, in_progress, fulfilled

- Import will create the tenant if missing.
- Import will insert any missing `types`/`statuses` keys referenced by the bundle.
- Import creates a new `model_versions` row (append-only). With `--enable`, it is created as enabled.

These demos include explicit `policy` nodes linked via `applies_to` edges.

```bash
go run ./cmd/contrato --import examples/models/moneyball.json --enable
```
Moneyball example demonstrates analytics-driven policy application to team construction.

Each `policy` node includes an `opa` section with `input_schema`, `output_schema`, and a small `decision_examples` array. Reason codes and supported obligations are stored as policy `properties` for machine interpretation.

To include contracts in an export/import bundle, use `--with-contracts`.
