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

| MessagingService/SendMessage | grpc | client | A1-grpc | OK | — | — | — | — |
| MessagingService/SendBatch | grpc | client | A1-grpc | OK | — | — | — | — |
| MessagingService/GetMessageStatus | grpc | client | A1-grpc | OK | — | — | — | — |
| MessagingService/GetMessageHistory | grpc | client | A1-grpc | OK | — | — | — | — |
| MessagingService/ProcessDLR | grpc | client | A1-grpc | OK | — | — | — | — |
| MessagingService/CancelMessage | grpc | client | A1-grpc | BROKEN | — | No implementation in server.go; falls through to UnimplementedMessagingServiceServer → codes.Unimplemented | major | plan:docs/superpowers/plans/2026-04-21-fix-grpc-unimplemented-methods.md |
| MessagingService/ListScheduledMessages | grpc | client | A1-grpc | BROKEN | — | No implementation in server.go; falls through to UnimplementedMessagingServiceServer → codes.Unimplemented | major | plan:docs/superpowers/plans/2026-04-21-fix-grpc-unimplemented-methods.md |
| GetBalance (dead method) | grpc | client | A1-grpc | BROKEN | — | Method in server.go but not part of MessagingServiceServer interface; never reachable via any registered service | minor | in-PR removal (≤50 lines) |
| GetStatistics (dead method) | grpc | client | A1-grpc | BROKEN | — | Method in server.go but not part of MessagingServiceServer interface; never reachable via any registered service | minor | in-PR removal (≤50 lines) |
| proto source — all 21 v1 packages | grpc | client | A1-grpc | BROKEN | — | v1 dirs contain only .pb.go; no .proto source → codegen pipeline broken | major | plan:docs/superpowers/plans/2026-04-21-fix-missing-proto-sources.md |
| grpc_health_v1 service | grpc | client | A1-grpc | MISSING | — | Standard gRPC health protocol not registered; only HTTP health endpoints exist | minor | inline fix in dead-methods removal PR |

| account.go | http | client | A2 | DRIFT | — | Uses respondError helper; envelope is {"error":{code,message,details}} — nested, not flat; no request_id | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| sms.go | http | client | A2 | DRIFT | — | Uses respondError helper; same envelope nesting/request_id gaps; otherwise consistent | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| lookup.go | http | client | A2 | DRIFT | — | Uses respondError helper; same envelope nesting/request_id gaps | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| templates.go | http | client | A2 | DRIFT | — | Uses respondError helper; same envelope nesting/request_id gaps | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| webhooks.go | http | client | A2 | DRIFT | — | Uses respondError helper; same envelope nesting/request_id gaps | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| cascade.go | http | client | A2 | BROKEN | — | Auth errors use http.Error() — plain text, not JSON; downstream errors use respondGRPCError; split error format on same handler | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| common.go (helpers) | http | client | A2 | DRIFT | — | response.Error() produces {"error":{code,message,details}} — nested; no request_id field; single fix here propagates to all 5 consistent handlers | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| grpc-external (server.go) | grpc | client | A2 | DRIFT | — | Uses status.Error(codes.X, msg) consistently; no status.WithDetails(); downstream errors pass through unmapped — leaks backend codes | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |
| SMPP handler | smpp | client | A2 | DRIFT | — | ESME codes scattered inline; no centralized bizError→ESME table; validation errors and decode errors both return ESME_RINVCMDLEN (semantically incorrect for validation) | major | plan:docs/superpowers/plans/2026-04-21-fix-error-contract.md |

| POST /api/v1/sms/send | http | client | A3 | BROKEN | — | No Idempotency-Key middleware; no handler-level check; no downstream dedup — retry = double send + double charge | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/sms/batch | http | client | A3 | BROKEN | — | Same as /sms/send — no dedup at any layer | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/webhooks | http | client | A3 | BROKEN | — | No idempotency; duplicate POST creates duplicate webhook subscription | major | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/lookup | http | client | A3 | BROKEN | — | No idempotency; retry creates duplicate lookup_log entry | major | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/lookup/bulk | http | client | A3 | BROKEN | — | Same as single lookup | major | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/templates | http | client | A3 | DRIFT | — | Partial protection: UNIQUE(client_id, name) prevents exact-name duplicate; not idempotency (different request = same name → 409, not replay) | minor | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| POST /api/v1/cascade/deliveries | http | client | A3 | BROKEN | — | No idempotency at gateway; downstream billing guard protects billing only for pipeline retries, not client retries | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| MessagingService/SendMessage | grpc | client | A3 | BROKEN | — | No idempotency-key metadata; no field in proto; retry = double message + double charge | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| MessagingService/SendBatch | grpc | client | A3 | BROKEN | — | Same as SendMessage — no dedup at any layer | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |
| SMPP submit_sm | smpp | client | A3 | BROKEN | — | user_message_reference TLV 0x0204 not read; uuid.New() per PDU; no Redis dedup on sequence; retry = double send + double charge | critical | plan:docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md |

## Machine-readable

См. `2026-04-21-cycle1-contract-matrix.csv`.
