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

**Severity:** major (external gRPC surface contains 2 permanently-unimplemented RPCs, 2 dead unreachable methods that compile but are not reachable via any registered service, and all v1 packages lack `.proto` source for reproducible regeneration)

### A1-grpc.1 — Unimplemented RPCs

Both `CancelMessage` and `ListScheduledMessages` are declared in `MessagingService_ServiceDesc` (7 methods total) and are wired through the generated dispatcher, but `internal/gateway/client/grpc/server.go` provides no override — so the call falls through to the embedded `UnimplementedMessagingServiceServer` stubs.

**CancelMessage:** declared in `api/proto/messagingv1/messaging_grpc.pb.go:357`. No override in `server.go`. Falls through to `UnimplementedMessagingServiceServer.CancelMessage` (line 176 of `messaging_grpc.pb.go`) which returns `codes.Unimplemented`. The HTTP equivalent `DELETE /api/v1/sms/{id}` works and is confirmed OK in Task 2.

**ListScheduledMessages:** declared in `api/proto/messagingv1/messaging_grpc.pb.go:362`. No override in `server.go`. Falls through to `UnimplementedMessagingServiceServer.ListScheduledMessages` (line 179). The HTTP equivalent `GET /api/v1/sms/scheduled` is also in the router.

No comment in `server.go` explains the omission. There is no test for these two RPCs in `server_test.go`. The HTTP path works — this is a gRPC-only gap, not an underlying service gap.

**Decision:** create `plan:docs/superpowers/plans/2026-04-21-fix-grpc-unimplemented-methods.md` to implement both RPCs following the same proxy pattern used for `SendMessage`/`GetMessageStatus`/etc. Alternatively, if gRPC exposure of cancel/list-scheduled is intentionally deferred, add a comment in `server.go` and document in the gRPC API surface docs. Leaving the unimplemented stubs silently returning `codes.Unimplemented` without documentation is the worst option.

### A1-grpc.2 — Dead methods in server.go

`GetBalance` (lines 145–167) and `GetStatistics` (lines 191–215) exist in `internal/gateway/client/grpc/server.go` with `billingv1`/`analyticsv1` types. These are NOT part of `MessagingServiceServer` interface and are NOT registered with any `RegisterXxxServer` call.

`cmd/client-gateway/main.go` line 207 registers only:
```
messagingv1.RegisterMessagingServiceServer(grpcServer, grpcHandlers)
```

No `billingv1.RegisterBillingServiceServer` or `analyticsv1.RegisterAnalyticsServiceServer` call exists anywhere in `cmd/client-gateway/`.

**Evidence:** `server.go:145–167` (`GetBalance`), `server.go:191–215` (`GetStatistics`). Both methods are fully tested in `server_test.go` (lines 512–669) with mock clients — confirming they were implemented intentionally, but never wired.

**Impact:** Misleads any developer reading `server.go` into believing these services are exposed. The `Server` struct holds `billingClient` and `analyticsClient` fields that are injected at `NewServer` call site (`main.go:200–204`) but serve only these unreachable methods. No gRPC client can call `GetBalance` or `GetStatistics` through the external gRPC port.

**Decision:** Removal is the clean answer (≤50 lines, ε-ok for in-PR). If there is an intent to eventually register `billingv1`/`analyticsv1` externally, at minimum add a `// TODO: register BillingServiceServer` comment and open a tracking issue. Removing now prevents the next developer from debugging a "why does this not work" mystery.

### A1-grpc.3 — Missing .proto source

Output of `find api/proto -name "*.proto" | sort` (20 files total):

```
api/proto/analytics/analytics.proto
api/proto/audit/audit.proto
api/proto/auth/auth.proto
api/proto/billing/billing.proto
api/proto/campaign/campaign.proto
api/proto/cascade/cascade.proto
api/proto/client/client.proto
api/proto/company/company.proto
api/proto/contact/contact.proto
api/proto/link/link.proto
api/proto/messaging/messaging.proto
api/proto/network_analytics/network_analytics.proto
api/proto/provider/provider.proto
api/proto/routing/routing.proto
api/proto/sender-name/sender_name.proto
api/proto/smpp/smpp.proto
api/proto/sms.proto
api/proto/tarification/tarification.proto
api/proto/template/template.proto
api/proto/webhook/webhook.proto
```

**Packages with .proto source (legacy, non-v1 dirs):** all 20 listed above — `analytics`, `audit`, `auth`, `billing`, `campaign`, `cascade`, `client`, `company`, `contact`, `link`, `messaging`, `network_analytics`, `provider`, `routing`, `sender-name`, `smpp`, root `sms.proto`, `tarification`, `template`, `webhook`.

**Packages without .proto source (v1 dirs — only .pb.go):** all 21 v1 packages — `analyticsv1`, `auditv1`, `authv1`, `billingv1`, `campaignv1`, `cascadev1`, `clientproviderv1`, `clientv1`, `companyv1`, `contactv1`, `linkv1`, `messagingv1`, `networkanalyticsv1`, `providerv1`, `routingv1`, `sendernamev1`, `smppv1`, `smsv1`, `tarificationv1`, `templatev1`, `webhookv1`.

The pattern is clear: there is a legacy `api/proto/<name>/` tree with `.proto` source and a parallel `api/proto/<name>v1/` tree containing only generated `.pb.go` files. The `messaging_grpc.pb.go` metadata field confirms the source was `messaging/messaging.proto`, meaning the v1 packages were generated from the legacy protos and the output was copied to separate v1 dirs — but the source proto itself was never moved there.

Some v1 dirs also have duplicate subdirs (e.g., `api/proto/messagingv1/messaging/messaging_grpc.pb.go` alongside `api/proto/messagingv1/messaging_grpc.pb.go`) suggesting partial tooling experiments left artifacts.

**Impact:** Any change to gRPC contracts in the v1 packages requires either modifying the legacy `.proto` (and re-running codegen with correct output path) or editing `.pb.go` directly (which is a category error — generated files must not be hand-edited). Currently the codegen pipeline is broken for v1: you cannot confidently regenerate without knowing which `protoc` invocation produced the existing output.

**Decision:** create `plan:docs/superpowers/plans/2026-04-21-fix-missing-proto-sources.md` to either: (a) move/copy `.proto` files into corresponding v1 dirs and fix the `go_package` option so `protoc` outputs to the correct location, or (b) establish a `buf.gen.yaml` / `Makefile` target that documents the exact codegen command. The duplicate nested subdirs should also be cleaned up. This is a tooling debt item, not an emergency — but any future contract change is blocked until resolved.

### A1-grpc.4 — Reflection and health

**Reflection:** present, but conditional. `cmd/client-gateway/main.go:212–215`:
```go
if cfg.Service.Env == "development" {
    reflection.Register(grpcServer)
}
```
In production (`cfg.Service.Env != "development"`), reflection is disabled. This is a reasonable security posture for production. Acceptable as-is. No action needed beyond documenting that grpcurl/grpc-health-probe must use `--plaintext` without reflection in production.

**gRPC health service (`grpc_health_v1`):** absent. There is an HTTP health checker (`monitoring.NewHealthChecker`) wired to HTTP routes (`/health`, `/health/live`, `/health/ready`), but no `google.golang.org/grpc/health` package is imported and no `RegisterHealthServer` call exists in `cmd/client-gateway/main.go`. Load balancers and orchestrators (Kubernetes, Envoy) that use the gRPC health protocol for pod readiness checks cannot use the standard probe. They would need to fall back to HTTP or exec probes.

**Decision:** minor gap. Recommend adding `grpc_health_v1` registration in a follow-up PR — it is ~10 lines and widely expected in production gRPC services. No separate plan needed; can be an inline fix in the same PR that removes dead methods.

### Decisions summary

| Issue | Severity | Action |
|---|---|---|
| 2 Unimplemented RPCs (CancelMessage, ListScheduledMessages) | major | plan:docs/superpowers/plans/2026-04-21-fix-grpc-unimplemented-methods.md |
| 2 Dead methods (GetBalance, GetStatistics) | minor | in-PR removal candidate (≤50 lines) |
| Missing .proto source for all 21 v1 packages | major | plan:docs/superpowers/plans/2026-04-21-fix-missing-proto-sources.md |
| Reflection dev-only | acceptable | no action; document in gRPC surface docs |
| gRPC health service absent | minor | inline fix in dead-methods removal PR |

## A2 — error format consistency

**Severity:** major (inconsistent formats break integrator error handling)

### A2.1 — HTTP envelope consistency

| Handler file | Error pattern | Has helper? | request_id present? | Notes |
|---|---|---|---|---|
| account.go | `respondError(w, shared.ErrUnauthorized(...))` + `respondGRPCError(w, err)` | Yes — `respondError` / `respondGRPCError` | No | Fully consistent |
| sms.go | `respondError(w, shared.ErrInvalidInput(...))` + `respondGRPCError(w, err)`, `w.WriteHeader(http.StatusNoContent)` for 204 | Yes | No | Fully consistent; `CancelSMS` returns 204 with no body (correct) |
| lookup.go | `respondError(w, shared.ErrInvalidInput(...))` + `respondGRPCError(w, err)` | Yes | No | Fully consistent |
| templates.go | `respondError(w, shared.ErrUnauthorized(...))` + `respondGRPCError(w, err)`, `w.WriteHeader(http.StatusNoContent)` for 204 | Yes | No | Fully consistent |
| webhooks.go | `respondError(w, shared.ErrUnauthorized(...))` + `respondGRPCError(w, err)`, `w.WriteHeader(http.StatusNoContent)` for 204 | Yes | No | Fully consistent |
| cascade.go | Auth errors → `http.Error(w, "unauthorized", http.StatusUnauthorized)` (plain text); downstream gRPC errors → `respondGRPCError(w, err)` | **Partially** — auth uses raw `http.Error`, not helper | No | **DRIFT: auth errors are plain-text, not JSON envelope** |
| common.go (helpers) | Defines `respondError = response.Error`, `respondGRPCError = response.GRPCError`, `respondJSON = response.JSON` | N/A — IS the helper | No | Delegates to `internal/api/http/response` package |

**Actual envelope shape** (from `internal/api/http/response/response.go:41–57`):

```
{"error": {"code": "INVALID_INPUT", "message": "...", "details": "..."}}
```

**Canonical envelope target (per umbrella spec §2):** `{code, message, details?, request_id}`.

**Current reality:** partially followed. There are two structural gaps:

1. **Nesting mismatch.** The actual envelope wraps the error fields under an `"error"` key: `{"error": {"code", "message", "details"}}`. The umbrella spec §2 prescribes a flat top-level object: `{code, message, details?, request_id}`. The nesting is consistent across all 5 well-behaved handlers — but it diverges from spec.

2. **`request_id` absent.** `response.Error()` never includes a `request_id` field. The `request_id` concept exists in the codebase (lookup handlers generate UUIDs for internal routing, but these are request-IDs passed to downstream services, not returned to callers). No middleware attaches a request-ID to the context or to error responses.

3. **`cascade.go` auth divergence.** All 4 auth-check paths in `cascade.go` use `http.Error(w, "unauthorized", http.StatusUnauthorized)` — returning plain text `unauthorized\n` with `Content-Type: text/plain`. The other 5 handler files use `respondError(w, shared.ErrUnauthorized(...))` which returns JSON. An integrator using the cascade API gets a different error format on auth failure than on any other endpoint.

### A2.2 — gRPC error mapping

`internal/gateway/client/grpc/server.go` uses `status.Error(codes.X, "message")` consistently for all 7 methods. Code coverage:

- `codes.Unauthenticated` — used for missing clientID (5 call sites)
- `codes.PermissionDenied` — used for cross-client access attempts (5 call sites)
- All calls pass a human-readable Russian-language string as the status message

**Gaps:**

1. **No `status.WithDetails()` usage.** Details (e.g., structured error fields, field violations) are never populated. Callers receive only a code + plain string. gRPC conventions for richer errors (e.g., `google.rpc.BadRequest.FieldViolation`) are not used anywhere.

2. **No downstream error mapping.** When the proxy methods call `s.messagingClient.SendMessage(ctx, req)` and the downstream returns an error, it is returned as-is to the caller without re-mapping. If the messaging service returns an `INTERNAL` code with a backend-internal message, that leaks through verbatim to the external caller. This is a minor information-leakage risk and means the external gRPC error surface is partially defined by the downstream services, not by a gateway contract.

3. **gRPC interceptor** (`internal/gateway/client/grpc/interceptor.go`) — exists but only handles auth, not error transformation.

### A2.3 — SMPP ESME mapping

No centralized bizError→ESME_* table exists. All ESME codes are assigned inline, scattered across `handler.go`. Inventory of actual mappings:

| Business condition | ESME code returned | Correct? |
|---|---|---|
| Invalid command length / bad decode | `ESME_RINVCMDLEN` | Correct for length errors |
| Validation failure (bind/submit) | `ESME_RINVCMDLEN` | **Wrong** — validation failure is not a length error; should be `ESME_RINVSRCADR` / `ESME_RINVDSTADR` / `ESME_RINVMSGLEN` depending on the field |
| Wrong password on bind | `ESME_RINVPASWD` | Correct |
| Invalid bind state (session not bound) | `ESME_RINVBNDSTS` | Correct |
| Kafka publish failure | `ESME_RSYSERR` | Correct |
| Rate limit exceeded | `ESME_RTHROTTLED` | Correct |
| Query SM — message not found | `ESME_RQUERYFAIL` | Correct |
| Cancel SM — cancel failure | `ESME_RCANCELFAIL` | Correct |
| Replace SM — replace failure | `ESME_RREPLACEFAIL` | Correct |
| Unsupported command ID | `ESME_RINVCMDID` | Correct |
| Unbind while session error | `ESME_RSYSERR` | Correct |

**Summary:** ESME mapping is scattered (no single lookup table), but most codes are semantically correct. The one real error: both decode errors and validation errors of `bind`/`submit` return `ESME_RINVCMDLEN`. A validation failure (e.g., empty `source_addr`) is not a command-length error. This misrepresents the failure to the client.

There is a `gwMapMessageStatusToSMPP()` helper at line 612 for message-state mapping — this is the only centralized SMPP mapping and covers message states only, not error conditions.

### Cross-transport consistency

Example: business error "client not found / not authenticated":

- **HTTP** (account, sms, lookup, templates, webhooks): `{"error": {"code": "UNAUTHORIZED", "message": "Клиент не найден"}}`, HTTP 401
- **HTTP** (cascade): plain text `unauthorized\n`, HTTP 401
- **gRPC**: `status.Error(codes.Unauthenticated, "клиент не найден")` — gRPC Unauthenticated code, string message only
- **SMPP** (bind): `ESME_RINVPASWD` (wrong password). Note: SMPP has no "session not found" concept — session state is managed at the transport level.

**Cross-transport semantics verdict:** partially preserved. HTTP and gRPC both signal 401/Unauthenticated for the same auth failure, which is correct. The SMPP equivalent (`ESME_RINVPASWD`) is the correct SMPP semantic. The failure is the cascade.go outlier returning plain text instead of JSON, breaking HTTP consistency.

The deeper issue is the envelope shape mismatch: HTTP callers see `{"error": {"code": ...}}` while the umbrella spec prescribes `{"code": ...}` at the top level. This is a consistent deviation, not a scattered one — which means a single fix in `response.Error()` would bring all 5 well-behaved handlers in line.

### Decisions

- **A2.1 (HTTP):** Two separate fixes needed. (a) `cascade.go` auth divergence is a 4-line inline fix — replace `http.Error(w, "unauthorized", http.StatusUnauthorized)` with `respondError(w, shared.ErrUnauthorized(""))`. (b) Envelope nesting + missing `request_id` require a change to `response.Error()` and request-ID middleware — this is a larger cross-cutting change. Propose `docs/superpowers/plans/2026-04-21-fix-error-contract.md`.
- **A2.2 (gRPC):** Adding `status.WithDetails()` and downstream error re-mapping requires a policy decision: what structured error types to expose. Include in the same fix plan.
- **A2.3 (SMPP):** The `ESME_RINVCMDLEN` overloading for validation errors is a real bug. A centralized mapping table would make this auditable. Include in fix plan. The cascade.go cascade fix is small enough to be inline.

Combined fix-plan stub candidate: `docs/superpowers/plans/2026-04-21-fix-error-contract.md`. Not created in this task.

## A3 — idempotency of write operations

**Severity:** critical — HTTP middleware absent; retry of `POST /sms/send` produces a new message every time, leading to double-charge and duplicate delivery.

### A3.1 — HTTP Idempotency-Key middleware

**Middleware existence:** absent. Searched `internal/gateway/client/middleware/` (3 files: `auth.go`, `auth_test.go`, `tenant_logger.go`) and `internal/api/middleware/` (10 files: `auth`, `cors`, `interfaces`, `logging`, `quota`, `ratelimit`, `recovery`, `tenant_logger` + tests). No `idempotency` file or `Idempotency-Key` header string found in either directory.

**Storage backend:** none — no `idempotency_keys` Postgres table in migrations, no Redis TTL store for HTTP idempotency keys.

**Middleware wiring:** none — `internal/gateway/client/router/router.go` middleware chain is `recovery → logging → cors → auth → tenantLogger → rateLimit → quota`. No idempotency step anywhere.

**Per-endpoint status:**

| Endpoint | Protected by middleware? | Handler-level check? | Downstream dedup? |
|---|---|---|---|
| POST /api/v1/sms/send | No | No | No — messaging service calls `uuid.New()` for every request; `external_id` has a plain index, not UNIQUE |
| POST /api/v1/sms/batch | No | No | No — same path, batch loop calls service per message |
| POST /api/v1/webhooks | No | No | No — no UNIQUE constraint on webhook URL+client |
| POST /api/v1/lookup | No | No | Partial — lookup results are read-only side-effect (HLR query); write to `lookup_log` has no dedup guard |
| POST /api/v1/lookup/bulk | No | No | Same as single lookup |
| POST /api/v1/templates | No | No | Partial — `idx_templates_client_name` is UNIQUE on `(client_id, name)`, so duplicate template name for same client will fail with 409/500, but only on exact name collision — not idempotency |
| POST /api/v1/cascade/deliveries | No | No | Partial — tarification layer has `commit_idempotency_guard` (PK=message_id), but this protects billing double-charge downstream, not cascade delivery creation |

### A3.2 — gRPC idempotency

`internal/gateway/client/grpc/interceptor.go` handles only authentication (dummy hardcoded UUID in load-test mode). No `idempotency-key` metadata is read in the interceptor or in any of the 5 implemented RPC methods (`SendMessage`, `SendBatch`, `GetMessageStatus`, `GetMessageHistory`, `GetBalance`-dead, `GetStatistics`-dead).

`messagingv1.SendMessageRequest` and `SendBatchRequest` proto messages have no `idempotency_key` or `client_msg_id` field (`api/proto/messaging/messaging.proto` verified line by line). The only field that could serve as a correlation ID is `external_id` (field 5 in `SendMessageRequest`), but: (a) it is optional, (b) it has a plain index — not UNIQUE — on the `messages` table, and (c) the messaging service does not check `external_id` before inserting.

The tarification proto (`api/proto/tarification/tarification.pb.go`) does have `idempotency_key` on `TarifyMessageRequest` and `TarifyLookupRequest`, but this is an internal service-to-service contract, not the external gRPC surface exposed to clients.

**Summary:** no gRPC-level idempotency mechanism exists for any externally-exposed write RPC.

### A3.3 — SMPP dedup

`handleSubmitSM` (`internal/gateway/smpp/server/handler.go:276–398`) generates `msgID := uuid.New()` unconditionally for every `submit_sm` PDU. The SMPP PDU field `user_message_reference` (TLV 0x0204) is never read — the `protocol.SubmitSMPDU` struct does not include it and the decode path ignores it. There is no correlation between SMPP sequence number and a dedup store: `messageID` is constructed as `fmt.Sprintf("%s-%d", h.session.ID, pdu.SequenceNumber)` and is only used as the `message_id` in the `submit_sm_resp` body. The real internal `msgID` (uuid.New()) is what flows to Kafka.

No Redis key is checked before Kafka publish to detect duplicate sequence numbers within a session. Redis `SaveMessageMapping` stores a DLR routing mapping per `msgID`, but this is write-only and is never queried for dedup purposes.

### A3.4 — Downstream dedup snapshot

**Tarification layer (internal, not external-facing):** has genuine idempotency.

- `tarification_log` table: `UNIQUE INDEX idx_tarification_log_idempotency ON tarification_log(idempotency_key, created_at)` — partitioned-table composite unique (`migrations/000013`).
- `provider_tarification_log` table: same pattern (`migrations/000039`).
- `commit_idempotency_guard` table: non-partitioned, `PK = message_id`, used by `billing/application/charge_dual.go` via `ClaimTx` (`ON CONFLICT (message_id) DO NOTHING`) — prevents billing double-charge within a single transaction.
- `commit_retry_queue` table: `ON CONFLICT DO NOTHING` on `message_id` PK (noted in `migrations/000109`).

These guards protect the **billing + tarification** path from duplicate charges when the pipeline retries internally. They do NOT protect against a client submitting two separate HTTP/gRPC/SMPP requests with identical payloads — because each request gets a fresh `uuid.New()` before reaching the tarification layer.

**Messaging layer:** no dedup. `messages` table has `external_id` indexed but not UNIQUE. `message_service.go:SendMessage` does not query by `external_id` before inserting. Each call creates a new message row unconditionally.

**messagingv1 proto:** no `client_msg_id` field — confirmed by reading `api/proto/messaging/messaging.proto` in full.

### Cross-transport safety analysis

**Scenario:** client retries `POST /api/v1/sms/send` with the same payload due to network timeout (client never received the 200 response).

- Current behavior: **double send + double charge**. The gateway handler calls the messaging gRPC service, which calls `uuid.New()` for a new message ID, publishes to Kafka, and the pipeline charges the client via tarification. The first request (which succeeded at the server) and the retry both complete independently. The client gets two distinct `message_id` values in responses (if they ever get the second response). The recipient receives two SMS. The client account is debited twice. The `commit_idempotency_guard` does NOT help here because the two requests have different `message_id` UUIDs.

**Scenario:** SMPP client retries `submit_sm` with the same `user_message_reference` (same sequence number) due to network timeout.

- Current behavior: **double send + double charge**, same mechanism. `user_message_reference` TLV is ignored entirely. The handler generates `uuid.New()` on each `submit_sm` received, publishes to Kafka immediately. Even if the client retries with the same SMPP sequence number, the handler treats it as a new message.

**Boundary condition:** the only partial protection exists if the retry happens at the billing commit stage for the exact same `message_id` UUID — which can only happen in pipeline-internal retries (Kafka consumer reprocessing with the same Kafka message), not in client-driven HTTP/gRPC/SMPP retries.

### Decisions

- **A3 overall: critical.** Client-facing retries on all three transports (HTTP, gRPC, SMPP) produce duplicate messages and duplicate billing charges. The downstream billing guard (`commit_idempotency_guard`) mitigates pipeline-internal retries only.

- Proposed fix plan: `docs/superpowers/plans/2026-04-21-fix-idempotency-middleware.md` — НЕ создаём сейчас, флагаем.

  Minimum viable fix requires:
  1. HTTP middleware: read `Idempotency-Key` header, store `(client_id + key) → response` in Redis with TTL (e.g., 24h); replay cached response on duplicate.
  2. gRPC interceptor: read `idempotency-key` metadata, same Redis store.
  3. SMPP handler: read TLV 0x0204 (`user_message_reference`), store `(session_id + ref) → msgID` in Redis, return cached `submit_sm_resp` on duplicate.
  4. messaging proto: add `idempotency_key` field to `SendMessageRequest` / `SendBatchRequest` so the gateway can pass the client-supplied key downstream for auditing.
  5. `messages` table: add UNIQUE constraint on `(client_id, external_id)` WHERE `external_id IS NOT NULL` — or equivalent — so even if middleware is bypassed, duplicate inserts are blocked at DB level.

## Summary of critical / major findings

_TODO: Task 6_
