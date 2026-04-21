# Cycle 1 — Contract Coverage Matrix

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle1-contract.md`
**Phase completed:** skeleton
**OK rows with test_file:** 0

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` / `major` / `minor`
- `action`: `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`

## Notes on A1 row scope

One row per HTTP endpoint (not per role) for the A1 axis. OpenAPI is a single document covering all client roles equally — per-role duplication would add noise without information. Role is fixed as `client` for all A1 rows.

## Matrix

| surface | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| POST /api/v1/sms/send | http | client | A1 | DRIFT | — | Status code: openapi=202, code=200 | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/sms/batch | http | client | A1 | DRIFT | — | Status code: openapi=202, code=200 | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/sms/status/{id} | http | client | A1 | OK | — | — | — | — |
| GET /api/v1/sms/history | http | client | A1 | OK | — | — | — | — |
| GET /api/v1/sms/scheduled | http | client | A1 | OK | — | — | — | — |
| DELETE /api/v1/sms/{id} | http | client | A1 | OK | — | — | — | — |
| GET /api/v1/account/balance | http | client | A1 | DRIFT | — | Extra fields client_id, updated_at undocumented | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/account/stats | http | client | A1 | DRIFT | — | Schema completely wrong: openapi={total,counts}, code returns time-series analytics object | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /health | http | client | A1 | OK | — | — | — | — |
| GET /health/live | http | client | A1 | OK | — | — | — | — |
| GET /health/ready | http | client | A1 | OK | — | — | — | — |
| GET /metrics | http | client | A1 | MISSING | — | In openapi, not in client router (registered elsewhere) | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /docs | http | client | A1 | MISSING | — | In router, not in openapi | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /docs/swagger | http | client | A1 | MISSING | — | In router, not in openapi | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /docs/grpc | http | client | A1 | MISSING | — | In router, not in openapi | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /docs/openapi.yaml | http | client | A1 | MISSING | — | In router, not in openapi | minor | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/webhooks | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/webhooks | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/webhooks/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| PUT /api/v1/webhooks/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| DELETE /api/v1/webhooks/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/lookup | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/lookup/bulk | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/lookup/history | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/templates | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/templates | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/templates/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| PUT /api/v1/templates/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| DELETE /api/v1/templates/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/templates/{id}/audit | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| POST /api/v1/cascade/deliveries | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/cascade/deliveries | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/cascade/deliveries/{id} | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |
| GET /api/v1/cascade/stats | http | client | A1 | MISSING | — | In router, not in openapi | major | plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md |

## Machine-readable

См. `2026-04-21-cycle1-contract-matrix.csv`.
