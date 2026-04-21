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

_TODO: Task 4_

## A3 — idempotency of write operations

_TODO: Task 5_

## Summary of critical / major findings

_TODO: Task 6_
