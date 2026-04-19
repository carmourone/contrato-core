# Graph Resolution Pipeline (Contrato)

Contrato resolves a contract request against a *versioned* model snapshot and produces:

1. A deterministic **resolution plan** (bounded traversal)
2. A resolved subgraph + properties + derived facts
3. Attached policies for OPA evaluation
4. A normalized decision payload: `action`, `obligations`, `reasons`

## Principles

- **Deterministic**: same inputs + same model_version => same plan + same result
- **Bounded**: traversal limits prevent runaway graph walks
- **Replayable**: resolution is pinned to `(tenant_id, model_id, model_version)`
- **Separation of concerns**:
  - Contrato: resolve, attach policy, derive facts, build OPA input
  - OPA: decide `allow|deny|...` and obligations/reasons

## Suggested core ontology (minimum)

Nodes:
- `actor`
- `capability`
- `policy`

Edges:
- `provides` (actor -> capability)
- `requires` (capability -> capability/resource)
- `governs` (policy -> capability)

properties:
- stored in `properties` table (typed columns)
- used for metrics, thresholds, config, and evidence

## OPA handoff shape

OPA input should include:
- pinned model ref
- resolved nodes/edges/properties subset
- derived facts

OPA output should return:
- `action`
- `obligations[]`
- `reasons[]` (stable codes)

## Next implementation steps

- Implement a concrete resolver that:
  - resolves the capability path from request
  - loads only required nodes/edges/params
  - attaches governing policies via `governs`
  - derives common facts (e.g., thresholds met, budget available)
- Add a `contrato resolve` CLI command that outputs:
  - plan JSON
  - resolved result JSON
