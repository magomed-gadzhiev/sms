# Cycle 1 — Contract Findings

**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle1-contract.md`
**Branch:** `review/api-v2-cycle1-contract`

## A1 — HTTP OpenAPI ↔ code drift

**Severity:** major (contract drift misleads integrators)

### Source of truth

**OpenAPI file:** `api/openapi/openapi.yaml` (765 lines, last commit 2026-04-14).
**Router file:** `internal/gateway/client/router/router.go` (last commit 2026-04-12).

### Drift summary

| Drift type | Count |
|---|---|
| In openapi, not in router | 1 |
| In router, not in openapi | 22 |
| Response status code mismatches | 2 |
| Response body schema mismatches | 2 |

### Details

**In openapi, not in router:**

- `GET /metrics` — documented as "Prometheus metrics" but no route is registered in `router.go`; the Prometheus handler is likely registered directly on the HTTP server or monitoring package, outside the client router. Clients following the spec would get a 404 from the client gateway.

**In router, not in openapi (22 routes — all undocumented):**

Docs routes (4, no-auth, arguably internal):
- `GET /docs` — Redoc UI, documented nowhere
- `GET /docs/swagger` — Swagger UI
- `GET /docs/grpc` — gRPC docs page
- `GET /docs/openapi.yaml` — serves the spec itself

Webhooks group (5):
- `POST /api/v1/webhooks` — create webhook subscription
- `GET /api/v1/webhooks` — list webhook subscriptions
- `GET /api/v1/webhooks/{id}` — get webhook subscription
- `PUT /api/v1/webhooks/{id}` — update webhook subscription
- `DELETE /api/v1/webhooks/{id}` — delete webhook subscription

Lookup / HLR group (3):
- `POST /api/v1/lookup` — single number lookup
- `POST /api/v1/lookup/bulk` — bulk number lookup
- `GET /api/v1/lookup/history` — lookup history

Templates group (6):
- `POST /api/v1/templates` — create template
- `GET /api/v1/templates` — list templates
- `GET /api/v1/templates/{id}` — get template
- `PUT /api/v1/templates/{id}` — update template
- `DELETE /api/v1/templates/{id}` — delete template
- `GET /api/v1/templates/{id}/audit` — template audit log

Cascade group (4):
- `POST /api/v1/cascade/deliveries` — create cascade delivery
- `GET /api/v1/cascade/deliveries` — list cascade deliveries
- `GET /api/v1/cascade/deliveries/{id}` — get cascade delivery
- `GET /api/v1/cascade/stats` — cascade stats

**Mismatches (path/method identical, but contract differs):**

None — the paths that exist in both use correct method + path template. All mismatches are response-level (see below).

### Response-schema drift

Five endpoints present in both OpenAPI and router, checked against handler source:

1. **POST /api/v1/sms/send** — OpenAPI declares `202 Accepted`; handler calls `respondJSON(w, http.StatusOK, response)` = **200 OK**. Mismatch on status code. Body shape is consistent with `SendSMSResponse` schema.

2. **POST /api/v1/sms/batch** — OpenAPI declares `202 Accepted`; handler calls `respondJSON(w, http.StatusOK, response)` = **200 OK**. Same mismatch. Body shape matches `SendBatchResponse`.

3. **GET /api/v1/account/balance** — OpenAPI schema declares `{balance, currency}` (2 fields). Handler returns `{client_id, balance, currency, updated_at}` (4 fields). Extra fields `client_id` and `updated_at` are undocumented but non-breaking for consumers.

4. **GET /api/v1/account/stats** — OpenAPI schema declares `{total: integer, counts: object<string,integer>}`. Handler returns `{client_id, from, to, group_by, groups: [{key, stats:{total_sent, total_delivered, ...}}], totals: {...}}`. **Complete schema mismatch** — different structure, different field names. The OpenAPI schema is a flat counts-by-status shape; the actual response is a time-series analytics object. This is the most severe individual mismatch.

5. **DELETE /api/v1/sms/{id}** — OpenAPI declares `204 No Content`; handler calls `w.WriteHeader(http.StatusNoContent)`. **Match.** No drift.

### Decisions

The drift count exceeds 5 missing routes in one direction (22 routes undocumented), which triggers the large-drift threshold. Recommendation: create a separate plan `docs/superpowers/plans/2026-04-21-fix-openapi-drift.md` to cover:
- Add OpenAPI entries for all 22 undocumented routes (webhooks, lookup, templates, cascade)
- Fix `POST /sms/send` and `POST /sms/batch` status codes from 202 to 200 (or fix the handlers to return 202)
- Fix `GET /account/stats` schema to match actual response shape
- Clarify `GET /metrics` route registration (client router vs. monitoring server)

The /docs/* routes may be intentionally excluded from the public contract spec (internal UI routes) — document this decision explicitly.

None of these are fixed in this PR. All are flagged in the matrix as `plan:docs/superpowers/plans/2026-04-21-fix-openapi-drift.md`.

## A1-grpc — gRPC proto ↔ implementation

_TODO: Task 3_

## A2 — error format consistency

_TODO: Task 4_

## A3 — idempotency of write operations

_TODO: Task 5_

## Summary of critical / major findings

_TODO: Task 6_
