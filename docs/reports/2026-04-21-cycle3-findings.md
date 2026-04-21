# Cycle 3 — Subaccount e2e Findings

**Spec:** `docs/superpowers/specs/2026-04-21-cycle3-subaccount-design.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle3-subaccount.md`
**Branch:** `review/api-v2-cycle3-subaccount`

## C-billing (across HTTP + gRPC + SMPP)

### Reference cells (predetermined from Cycle 1/2)

| Cell | Resolution |
|---|---|
| C-billing / subaccount_parent / any transport | BROKEN — B3 dual-charge flag OFF by default. Charge goes single (only on subaccount), aggregator margin not recorded. See Phase 0 F4. |
| C-billing / gRPC-external / any role | BROKEN — A7.3 stub interceptor. All gRPC calls execute as dummy tenant. See Cycle 2 F3. |

### HTTP C-billing — fresh investigation

**Chain:** POST /sms/send → handler → messagingv1.SendMessage → messaging service → billing charge → tarification_log.

**ClientID propagation:**
1. HTTP middleware (`internal/gateway/client/middleware/auth.go:161-167`): sets `ClientIDKey` = `resp.User.ClientId` if non-empty, else `UserID`. For all roles the real `client_id` from Auth Service is stored in context.
2. Handler (`internal/gateway/client/handlers/sms.go:65, 116`): `middleware.GetClientID(r.Context())` extracts `ClientIDKey` → set as `protoReq.ClientId`. No override.
3. `messaging/grpc/server.go:49, 92`: `parseClientID(req.ClientId)` trusts `req.ClientId` directly (no ctx injection, no lookup). Passes `clientID` to `messageService.SendMessage`.
4. Billing call (`internal/pipeline/sender/stage.go:347-348`): `tarifyReq.ClientId = routedMsg.ClientID.String()` where `routedMsg.ClientID` comes from `kafkaMsg.ClientID` = the `client_id` set by messaging service at message creation.
5. `tarification_log` insert (`internal/services/tarification/application/tarification_service.go:496-497`): `domain.NewTarificationLog(req.ClientID, req.MessageID, ...)` → `tarification_log.client_id = req.ClientID` = the correct sender client_id.

**Verdict: OK.** The HTTP path correctly propagates `client_id` from Auth middleware all the way to `tarification_log.client_id`. No leakage observed. Evidence: `auth.go:163-166`, `sms.go:116`, `messaging/grpc/server.go:49`, `sender/stage.go:348`, `tarification_service.go:496-497`.

Note: `subaccount_child` billing still works correctly for the HTTP axis — child's own client_id is charged. The dual-charge gap (B3) only applies to `subaccount_parent` rows (reference cell above).

### SMPP C-billing — fresh investigation

**Chain:** submit_sm → session (B2 kludge) → KafkaMessage → Kafka consumer → billing.

**ClientID propagation despite B2 kludge (step by step):**
1. `internal/gateway/smpp/server/auth_adapter.go:100-104`: `AuthenticateBySystemID` sets `clientID = resp.User.Id` (= UserID). Comment explicitly says "Используем user_id как client_id". The real `client_id` from Auth Service (`resp.User.ClientId`) is never read in this path — the field does not appear in `AuthenticateBySystemID` at all.
2. `handler.go:185-192` (handleBindTransmitter / Transceiver): `clientID = &id` where `id = parsed from userInfo.ClientID` = UserID. Passed to `session.Bind(..., clientID, userInfo.UserID, ...)`.
3. `session.go:107`: `s.ClientID = clientID` — session.ClientID stores the kludged UserID.
4. `handler.go:354` (handleSubmitSM): `kafkaMsg.ClientID = h.session.ClientID` — kludged UserID written into Kafka message.
5. `pipeline/router/stage.go:181`: `ClientID: *kafkaMsg.ClientID` — kludged value forwarded to routedMsg.
6. `pipeline/sender/stage.go:348`: `ClientId: routedMsg.ClientID.String()` — kludged value passed to `TarifyMessage`.
7. `tarification_service.go:496-497`: `domain.NewTarificationLog(req.ClientID, ...)` → `tarification_log.client_id` = kludged UserID.

**Verdict for `client` role: OK (with caveat).** For plain `client` accounts the convention in this codebase is that UserID == ClientID (Auth Service returns User.Id as the lookup key, and clients are registered under their own ID). So billing lands on the correct entity by coincidence.

**Verdict for `subaccount_child`: BROKEN — direct consequence of B2 kludge.** For a subaccount_child user (user_id = xyz, real client_id = abc), the SMPP session stores xyz as ClientID. The tarification_log records `client_id = xyz` instead of `abc`. Billing charges the user entity, not the actual subaccount. This is the same gap already identified as B2 in Cycle 1/2.

**Evidence:** `auth_adapter.go:100-104` (kludge origin), `handler.go:185-192, 354`, `session.go:107`, `sender/stage.go:348`, `tarification_service.go:496-497`.

## C-visibility (across HTTP + gRPC + SMPP)

### Reference cells (predetermined)

| Cell | Resolution |
|---|---|
| C-visibility / gRPC-external / any role | BROKEN — A7.3 stub (see Cycle 2 F3). All gRPC callers see dummy tenant data; cross-tenant isolation absent. |
| C-visibility / SMPP / subaccount_child | BROKEN — B2 kludge would propagate kludged client_id to any read ops (query_sm). |

### HTTP C-visibility — per-service SQL filter audit

| Endpoint group | Downstream service | Repo method | SQL filter on client_id? | File:line | Verdict |
|---|---|---|---|---|---|
| GET sms/status/{id} | messaging | `GetByID` → in-memory check | SQL: `WHERE id = $1` only; application layer checks `msg.ClientID != *clientID` in-memory | `storage/message_repository.go:64`; `messaging/application/message_service.go:224` | OK (in-memory guard — weaker than SQL but functional) |
| GET sms/history | messaging | `GetByClientID` | SQL: `WHERE client_id = $1` | `storage/message_repository.go:279` | OK |
| GET sms/scheduled | messaging | `ListScheduled` | SQL: `WHERE client_id = $1 AND status = 'scheduled'` | `storage/message_repository.go:495,518` | OK |
| GET account/balance | billing | `GetByClientID` | SQL: `WHERE client_id = $1` | `billing/infrastructure/repository/account_repository.go:55` | OK |
| GET account/stats | analytics | `GetStatistics` | SQL: `AND client_id = $N` (conditional on filter.ClientID != nil, always set from req) | `analytics/infrastructure/repository/metric_repository.go:183` | OK |
| GET webhooks (list) | webhook | `ListByClientID` | SQL: `WHERE client_id = $1` | `webhook/infrastructure/repository/subscription_repository.go:83` | OK |
| GET webhooks/{id} | webhook | `GetByID(id, clientID)` | SQL: `WHERE id = $1 AND client_id = $2` | `webhook/infrastructure/repository/subscription_repository.go:68` | OK |
| GET lookup/history | routing | `GetHistory` (lookup_log_repo) | SQL: mandatory `client_id = $1` first condition | `routing/infrastructure/repository/lookup_log_repo.go:55` | OK |
| GET templates (list) | template | `ListByClientID` | SQL: `WHERE t.client_id = $N` (added when clientID != uuid.Nil; always non-nil from middleware) | `template/infrastructure/repository/template_repository.go:144` | OK |
| GET templates/{id} | template | `GetByID(id, clientID)` | SQL: `WHERE t.id = $1 AND t.client_id = $2` | `template/infrastructure/repository/template_repository.go:103` | OK |
| GET templates/{id}/audit | template | `ListByTemplateID` | SQL: `WHERE template_id = $1` only — no client_id; handler discards clientID (`_` at line 184) and gRPC call omits ClientId | `handlers/templates.go:184`; `template/grpc/server.go:246`; `template/infrastructure/repository/audit_repository.go:89` | **BROKEN** — new finding F-C1 |
| GET cascade/deliveries | cascade | `List` (delivery_repo) | SQL: mandatory `client_id = $1` first condition | `cascade/infrastructure/postgres/delivery_repo.go:122` | OK |
| GET cascade/deliveries/{id} | cascade | `GetByClientID(id, clientID)` | SQL: `WHERE id = $1 AND client_id = $2` | `cascade/infrastructure/postgres/delivery_repo.go:76` | OK |
| GET cascade/stats | cascade | `Stats` (delivery_repo) | SQL: `WHERE client_id = $1 AND created_at BETWEEN $2 AND $3` | `cascade/infrastructure/postgres/delivery_repo.go:208` | OK |

**Summary:** 13/14 endpoints correctly filter by `client_id`; 1 has a visibility bug.

**New findings:**

**F-C1 (major): `GET /api/v1/templates/{id}/audit` — no client_id enforcement.**
- `handlers/templates.go:184`: handler extracts clientID but discards it with `_`.
- gRPC call `GetTemplateAuditLog` (line 206) sends only `TemplateId`, no `ClientId`.
- `template/grpc/server.go:246`: `GetTemplateAuditLog` receives no `client_id`, calls `templateService.GetAuditLog(ctx, templateID, ...)` with no ownership check.
- `audit_repository.go:89`: SQL is `WHERE template_id = $1` — any authenticated client who knows a template ID (guessable UUID) can read its full audit history.
- Severity: **major** — information leak (audit log content: old/new body, actor IDs, change reasons). Not critical since it requires a valid session and a known template UUID (not trivially discoverable), but it's a genuine cross-tenant read.

**sms/status in-memory guard note (not a new finding, but documented):**
`GetMessageStatus` fetches by `id` only in SQL, then checks ownership in Go. If `clientID` is nil in the gRPC call, the check is skipped entirely (`if clientID != nil && ...`). The HTTP handler always passes a non-nil `clientID`, so for the HTTP axis this is OK. For a hypothetical direct gRPC caller with `clientID=""`, the check is bypassed — but this is the pre-existing gRPC-external BROKEN cell (A7.3 stub), not an HTTP bug.

### Decisions

- HTTP C-visibility = OK for `client` and `subaccount_child` for 13/14 endpoints. F-C1 (audit log) is an exception.
- F-C1 fix path: add `ClientId` field to `GetTemplateAuditLogRequest` proto, propagate from handler, enforce in grpc server via ownership check (fetch template first, verify client_id).
- subaccount_parent visibility: all SQL queries filter strictly by `client_id = <own_id>`. A `subaccount_parent` (reseller) does NOT see its children's data via any of these endpoints. Whether this is intentional is a product decision — the `is_reseller` flag exists but no "see children" query logic is implemented anywhere in the read path. Hypothesis: current behavior is likely a gap (aggregator should be able to see children's messages for support purposes), but since the spec is silent on this, marking as **UNKNOWN / design gap** rather than BROKEN.

## C-dlr-routing (across HTTP + gRPC + SMPP)

_TODO: Task 4_

## Summary of critical / major findings (new, not reference)

_TODO: Task 5_
