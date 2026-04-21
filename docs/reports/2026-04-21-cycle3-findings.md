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

_TODO: Task 3_

## C-dlr-routing (across HTTP + gRPC + SMPP)

_TODO: Task 4_

## Summary of critical / major findings (new, not reference)

_TODO: Task 5_
