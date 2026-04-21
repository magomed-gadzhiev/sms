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

### Reference cells (predetermined)

| Cell | Resolution |
|---|---|
| C-dlr-routing / SMPP outbound (smppv1.DeliverDLR → client receiver bind) | OK (Cycle 2 + tests: `grpc_server_test.go`) |
| C-dlr-routing / SMPP inbound (deliver_sm на gateway port) | BROKEN (Cycle 2 A6-dlr — esm_class check missing) |
| C-dlr-routing / gRPC-external | n/a (messagingv1 не имеет DLR-specific RPC для внешнего клиента) |

### Webhook creation ownership

**Chain:** POST /api/v1/webhooks → `internal/gateway/client/handlers/webhooks.go:CreateWebhook` → `webhookv1.CreateSubscription` gRPC → `webhook/grpc/server.go:CreateSubscription` → `webhook/application/webhook_service.go:CreateSubscription` → SQL `INSERT INTO webhook_subscriptions`.

**Findings:**

1. `webhooks.go:25`: `clientID, ok := middleware.GetClientID(r.Context())` — extracts authenticated client's UUID from context. No override possible from request body (body only contains `url` and `event_types`).
2. `webhooks.go:48-52`: gRPC call sets `ClientId: clientID.String()` — the authenticated client UUID.
3. `webhook/grpc/server.go:34-49`: `CreateSubscription` validates `req.ClientId` non-empty, parses as UUID, passes to `s.webhookService.CreateSubscription(ctx, clientID, ...)` — no ctx injection, no override.
4. `webhook/application/webhook_service.go:65`: `sub := &domain.Subscription{..., ClientID: clientID, ...}` — the authenticated client_id.
5. `webhook/infrastructure/repository/subscription_repository.go:52-54`: `INSERT INTO webhook_subscriptions (id, client_id, ...) VALUES ($1, $2, ...)` where `$2 = sub.ClientID` = authenticated client UUID.

**Verdict: OK.** `webhook_subscriptions.client_id` = authenticated client_id. No way to inject a different client_id via request body. Applies equally to `client`, `subaccount_child`, and `subaccount_parent` — each gets their own subscription scoped to their own client_id.

### DLR delivery webhook resolution

**Chain:** provider DLR (deliver_sm) → `smsc/pool_async.go:startReader` → `dlrCallback` → `queue.DLRMessage` on Kafka `sms.dlr` topic → `webhook/application/delivery_service.go:HandleDLR` → `msgRepo.GetEnrichment(dlr.MessageID)` → lookup `messages.client_id` → `dispatchToSubscriptions(enrichment.ClientID, ...)` → `getSubscriptions(clientID)` → `ListActiveByClientID(clientID)`.

**Findings (step by step):**

1. `smsc/pool_async.go:217-230`: When `deliver_sm` received, `parseDLRFromDeliverSM` extracts `SMPPMessageID`, `Stat`, `Source`, `Destination`. Result is `*DeliverSMData` — **no internal MessageID (UUID)**.
2. `pipeline/sender/stage.go:81-104` (DLR callback): creates `queue.DLRMessage{SMPPMessageID: data.SMPPMessageID, ProviderID: &data.ProviderID, Stat: data.Stat, ...}`. **`MessageID` field is never set** — zero value = `uuid.Nil` (`00000000-0000-0000-0000-000000000000`). Published to Kafka topic `sms.dlr`.
3. `webhook/application/delivery_service.go:168`: `ds.msgRepo.GetEnrichment(ctx, dlr.MessageID)` — queries `messages WHERE id = '00000000-0000-0000-0000-000000000000'`. This always returns `nil, nil` (no rows).
4. `delivery_service.go:172-175`: `if enrichment == nil || enrichment.ClientID == nil { ds.logger.Warn().Msg("message not found for DLR, skipping"); return nil }` — **webhook delivery silently dropped every time**.
5. Because of step 4, `dispatchToSubscriptions` is never reached. No webhook callback is ever sent for SMPP provider DLRs.

**Root cause:** The internal message UUID is never resolved from `SMPPMessageID` before publishing the DLR to Kafka. The missing step is a lookup of `messages.id WHERE smpp_message_id = data.SMPPMessageID` (which exists as `storage.MessageRepository.GetBySMPPMessageID` at `storage/message_repository.go:114-115`) but is not called in the DLR callback path.

Note: The `status/stage.go:213-232` consumer also reads from `sms.dlr` but uses `dlr.MessageID` directly for the upsert. Since `MessageID` is zero, the DB upsert writes a status record for UUID `00000000-...`, which either silently fails (no row with that ID) or corrupts a phantom row. This is a separate bug but same root cause.

**Verdict for subaccount_child:** BROKEN. Even for a plain `client` webhook delivery for provider DLRs is broken. The subaccount question is moot since no DLR ever reaches `dispatchToSubscriptions`.

**Verdict for parent:** n/a — parent does not receive child's DLRs. No parent fallback logic exists in webhook resolution (`getSubscriptions` is strictly `WHERE client_id = $1` with no parent lookup). But this is a design gap (no DLR at all).

**Cross-child leakage:** No. `getSubscriptions` at `delivery_service.go:301` calls `ListActiveByClientID(ctx, clientID)` — strictly scoped to the resolved `client_id`. No cross-child leakage in design. The bigger issue is that this code path is never reached.

### New findings (Task 4)

**F-D1 (critical): DLR webhook delivery dead — MessageID never resolved from SMPPMessageID.**

- `pipeline/sender/stage.go:81-104`: DLR callback creates `queue.DLRMessage` with `MessageID = uuid.Nil`. The mapping from `SMPPMessageID → internal message UUID` is missing.
- `GetBySMPPMessageID` exists at `storage/message_repository.go:114-115` but is not called.
- Result: `delivery_service.go:HandleDLR` always hits "message not found, skipping" (line 173-175). All webhook DLR callbacks silently dropped.
- Affects all clients (not just subaccounts). Any HTTP client that registered a webhook for `delivered`/`failed` events receives nothing from SMPP provider DLRs.
- Severity: **critical** (complete feature failure — webhooks for DLR events are advertised but non-functional for SMPP-sourced DLRs).

**F-D2 (major): status/stage.go DLR upsert also broken by zero MessageID.**

- `pipeline/status/stage.go:223`: `MessageID: dlr.MessageID` where `dlr.MessageID = uuid.Nil`.
- The `batchUpsert` call will attempt `UPDATE messages WHERE id = '00000000-...'` — no match, no status update written.
- DLR status updates (DELIVRD, UNDELIV, etc.) never persisted to `messages` table via this path.
- Note: The `services/dlr/consumer.go` path (for SMPP outbound DLR dispatch) reads from `sms.status` topic (not `sms.dlr`) and uses `update.MessageID` properly — that path is separate and may work if status upsert were working, but since it's also broken by F-D2, SMPP DLR delivery via `DeliverDLR` gRPC to SMPP receiver clients is also broken for SMPP provider DLRs.
- Severity: **major** (DLR status tracking completely non-functional for SMPP provider receipts).

### Fix plan stub

**F-D1 fix:** In `pipeline/sender/stage.go` DLR callback, after receiving `data.SMPPMessageID`, perform a synchronous lookup `storage.MessageRepository.GetBySMPPMessageID(ctx, data.SMPPMessageID)` to resolve the internal `MessageID`. Set `dlrMsg.MessageID = msg.ID`. Also set `dlrMsg.ClientID = msg.ClientID` to avoid the DB lookup in `HandleDLR`.

Trade-off: this adds a DB roundtrip in the DLR hot path. Alternative: publish an intermediate Kafka message and resolve asynchronously, but that adds latency. Simplest correct fix: synchronous lookup in callback, acceptable since DLR volume is << send volume.

**F-D2 fix:** Same root cause as F-D1. Resolves automatically once F-D1 is fixed.

## Summary of critical / major findings (new, not reference)

| ID | Axis | Severity | Description | File |
|---|---|---|---|---|
| F-C1 | C-visibility | major | `GET /templates/{id}/audit` — no client_id enforcement; any authenticated client reads any template's audit log | `handlers/templates.go:184`, `audit_repository.go:89` |
| F-D1 | C-dlr-routing | critical | Webhook DLR delivery dead — `DLRMessage.MessageID` never populated in DLR callback; always zero UUID; `HandleDLR` always skips | `pipeline/sender/stage.go:81-104`, `delivery_service.go:168-175` |
| F-D2 | C-dlr-routing | major | DLR status upsert broken — same zero MessageID causes DB update to match no rows; DLR statuses never written to `messages` table via SMPP provider receipt path | `pipeline/status/stage.go:223`, `delivery_service.go:168-175` |
