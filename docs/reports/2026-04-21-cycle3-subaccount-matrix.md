# Cycle 3 — Subaccount e2e Coverage Matrix

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-cycle3-subaccount-design.md`
**Phase completed:** skeleton
**OK rows with test_file:** 0 (audit-only cycle)

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` / `major` / `minor`
- `action`: `plan:<path>` / `test-added:<path>` / `wontfix:<reason>` / `see-cycle<N>:<finding>`

## Matrix

| surface | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| POST /sms/send | HTTP | client | C-billing | OK | | client_id from Auth middleware flows correctly to tarification_log | | |
| POST /sms/send | HTTP | subaccount_child | C-billing | OK | | child client_id from Auth middleware flows correctly to tarification_log | | |
| POST /sms/send | HTTP | subaccount_parent | C-billing | BROKEN | | see-phase0:F4 (B3 dual-charge OFF — aggregator margin not recorded) | critical | plan:dual-charge-default-enable TBD |
| POST /sms/send | gRPC-external | client | C-billing | BROKEN | | see-cycle2:F3 (A7.3 stub interceptor — all calls get dummy tenant) | critical | plan:fix-grpc-auth-interceptor.md |
| POST /sms/send | gRPC-external | subaccount_child | C-billing | BROKEN | | see-cycle2:F3 (A7.3 stub interceptor — all calls get dummy tenant) | critical | plan:fix-grpc-auth-interceptor.md |
| POST /sms/send | gRPC-external | subaccount_parent | C-billing | BROKEN | | see-cycle2:F3 + see-phase0:F4 (stub interceptor + B3 dual-charge OFF) | critical | plan:fix-grpc-auth-interceptor.md + plan:dual-charge-default-enable TBD |
| POST /sms/batch | HTTP | client | C-billing | OK | | clientID from ctx set in every per-message SendMessageRequest (sms.go:218) | | |
| POST /sms/batch | HTTP | subaccount_child | C-billing | OK | | same as /sms/send HTTP — child client_id flows correctly | | |
| POST /sms/batch | HTTP | subaccount_parent | C-billing | BROKEN | | see-phase0:F4 (B3 dual-charge OFF) | critical | plan:dual-charge-default-enable TBD |
| POST /sms/batch | gRPC-external | client | C-billing | BROKEN | | see-cycle2:F3 (A7.3 stub interceptor) | critical | plan:fix-grpc-auth-interceptor.md |
| POST /sms/batch | gRPC-external | subaccount_child | C-billing | BROKEN | | see-cycle2:F3 (A7.3 stub interceptor) | critical | plan:fix-grpc-auth-interceptor.md |
| POST /sms/batch | gRPC-external | subaccount_parent | C-billing | BROKEN | | see-cycle2:F3 + see-phase0:F4 | critical | plan:fix-grpc-auth-interceptor.md + plan:dual-charge-default-enable TBD |
| POST /cascade/deliveries | HTTP | client | C-billing | UNKNOWN | | cascade billing path not traced in this task; see cascade billing_integration.go | major | investigate-cascade-billing |
| POST /cascade/deliveries | HTTP | subaccount_parent | C-billing | BROKEN | | see-phase0:F4 (B3 dual-charge OFF) | critical | plan:dual-charge-default-enable TBD |
| POST /lookup | HTTP | client | C-billing | UNKNOWN | | HLR lookup billing path not confirmed; assumed no direct tarification_log write | minor | investigate-hlr-billing |
| submit_sm | SMPP | client | C-billing | OK | | user_id == client_id by convention for plain clients; billing lands on correct entity | | |
| submit_sm | SMPP | subaccount_child | C-billing | BROKEN | | see-cycle1:B2 kludge — auth_adapter.go:100-104 sets clientID=userID; child's real client_id never used; tarification_log.client_id=userID not actual client_id | critical | plan:fix-smpp-auth-clientid-lookup.md (stub) |
| submit_sm | SMPP | subaccount_parent | C-billing | BROKEN | | see-cycle1:B2 kludge + see-phase0:F4 (B3 dual-charge OFF) | critical | plan:fix-smpp-auth-clientid-lookup.md + plan:dual-charge-default-enable TBD |

| GET /sms/status/{id} | HTTP | client | C-visibility | OK | | storage: WHERE id=$1 + in-memory client_id check in application layer | | |
| GET /sms/status/{id} | HTTP | subaccount_child | C-visibility | OK | | same in-memory check uses child's client_id from middleware | | |
| GET /sms/status/{id} | HTTP | subaccount_parent | C-visibility | OK | | parent sees only own messages; no child aggregation | | |
| GET /sms/history | HTTP | client | C-visibility | OK | | SQL WHERE client_id=$1 (storage/message_repository.go:279) | | |
| GET /sms/history | HTTP | subaccount_child | C-visibility | OK | | child client_id from middleware → SQL filter | | |
| GET /sms/history | HTTP | subaccount_parent | C-visibility | OK | | parent sees own messages only; children invisible | | |
| GET /sms/scheduled | HTTP | client | C-visibility | OK | | SQL WHERE client_id=$1 AND status='scheduled' | | |
| GET /sms/scheduled | HTTP | subaccount_child | C-visibility | OK | | child client_id from middleware → SQL filter | | |
| GET /sms/scheduled | HTTP | subaccount_parent | C-visibility | OK | | parent sees own scheduled only | | |
| GET /account/balance | HTTP | client | C-visibility | OK | | SQL WHERE client_id=$1 (account_repository.go:55) | | |
| GET /account/balance | HTTP | subaccount_child | C-visibility | OK | | child client_id → correct account lookup | | |
| GET /account/balance | HTTP | subaccount_parent | C-visibility | OK | | parent sees own balance only; no child aggregation | | |
| GET /account/stats | HTTP | client | C-visibility | OK | | SQL AND client_id=$N in GetStatistics (metric_repository.go:183) | | |
| GET /account/stats | HTTP | subaccount_child | C-visibility | OK | | child client_id → SQL filter | | |
| GET /account/stats | HTTP | subaccount_parent | C-visibility | OK | | parent sees own stats only | | |
| GET /webhooks | HTTP | client | C-visibility | OK | | SQL WHERE client_id=$1 (subscription_repository.go:83) | | |
| GET /webhooks | HTTP | subaccount_child | C-visibility | OK | | child client_id from middleware | | |
| GET /webhooks | HTTP | subaccount_parent | C-visibility | OK | | parent sees own webhooks only | | |
| GET /webhooks/{id} | HTTP | client | C-visibility | OK | | SQL WHERE id=$1 AND client_id=$2 (subscription_repository.go:68) | | |
| GET /webhooks/{id} | HTTP | subaccount_child | C-visibility | OK | | same SQL guard | | |
| GET /webhooks/{id} | HTTP | subaccount_parent | C-visibility | OK | | same SQL guard | | |
| GET /lookup/history | HTTP | client | C-visibility | OK | | SQL client_id=$1 mandatory (lookup_log_repo.go:55) | | |
| GET /lookup/history | HTTP | subaccount_child | C-visibility | OK | | child client_id → SQL filter | | |
| GET /lookup/history | HTTP | subaccount_parent | C-visibility | OK | | parent sees own lookups only | | |
| GET /templates | HTTP | client | C-visibility | OK | | SQL WHERE t.client_id=$N (template_repository.go:144) | | |
| GET /templates | HTTP | subaccount_child | C-visibility | OK | | child client_id from middleware | | |
| GET /templates | HTTP | subaccount_parent | C-visibility | OK | | parent sees own templates only | | |
| GET /templates/{id} | HTTP | client | C-visibility | OK | | SQL WHERE t.id=$1 AND t.client_id=$2 (template_repository.go:103) | | |
| GET /templates/{id} | HTTP | subaccount_child | C-visibility | OK | | same SQL guard | | |
| GET /templates/{id} | HTTP | subaccount_parent | C-visibility | OK | | same SQL guard | | |
| GET /templates/{id}/audit | HTTP | client | C-visibility | BROKEN | | F-C1: handler discards clientID (_); gRPC omits ClientId; SQL WHERE template_id=$1 only — any client reads any template's audit log | major | investigate-fix: add ClientId to GetTemplateAuditLogRequest |
| GET /templates/{id}/audit | HTTP | subaccount_child | C-visibility | BROKEN | | see F-C1 — same bug; child can read parent's template audit | major | same fix |
| GET /templates/{id}/audit | HTTP | subaccount_parent | C-visibility | BROKEN | | see F-C1 — same bug | major | same fix |
| GET /cascade/deliveries | HTTP | client | C-visibility | OK | | SQL mandatory client_id=$1 (delivery_repo.go:122) | | |
| GET /cascade/deliveries | HTTP | subaccount_child | C-visibility | OK | | child client_id → SQL filter | | |
| GET /cascade/deliveries | HTTP | subaccount_parent | C-visibility | OK | | parent sees own deliveries only | | |
| GET /cascade/deliveries/{id} | HTTP | client | C-visibility | OK | | SQL WHERE id=$1 AND client_id=$2 (delivery_repo.go:76) | | |
| GET /cascade/deliveries/{id} | HTTP | subaccount_child | C-visibility | OK | | same SQL guard | | |
| GET /cascade/deliveries/{id} | HTTP | subaccount_parent | C-visibility | OK | | same SQL guard | | |
| GET /cascade/stats | HTTP | client | C-visibility | OK | | SQL WHERE client_id=$1 AND created_at BETWEEN $2 AND $3 (delivery_repo.go:208) | | |
| GET /cascade/stats | HTTP | subaccount_child | C-visibility | OK | | child client_id → SQL filter | | |
| GET /cascade/stats | HTTP | subaccount_parent | C-visibility | OK | | parent sees own stats only | | |
| GET /sms/status/{id} | gRPC-external | client | C-visibility | BROKEN | | see-cycle2:F3 (A7.3 stub — dummy tenant) | critical | plan:fix-grpc-auth-interceptor.md |
| GET /sms/status/{id} | gRPC-external | subaccount_child | C-visibility | BROKEN | | see-cycle2:F3 | critical | plan:fix-grpc-auth-interceptor.md |
| GET /sms/history | gRPC-external | client | C-visibility | BROKEN | | see-cycle2:F3 | critical | plan:fix-grpc-auth-interceptor.md |
| GET /sms/history | gRPC-external | subaccount_child | C-visibility | BROKEN | | see-cycle2:F3 | critical | plan:fix-grpc-auth-interceptor.md |
| GET /sms/history | gRPC-external | subaccount_parent | C-visibility | BROKEN | | see-cycle2:F3 | critical | plan:fix-grpc-auth-interceptor.md |
| query_sm | SMPP | client | C-visibility | OK | | user_id == client_id by convention; read ops use kludged ID that matches real account | | |
| query_sm | SMPP | subaccount_child | C-visibility | BROKEN | | see-cycle1:B2 kludge — kludged client_id=user_id used; reads wrong tenant's data | critical | plan:fix-smpp-auth-clientid-lookup.md |
| query_sm | SMPP | subaccount_parent | C-visibility | BROKEN | | see-cycle1:B2 kludge | critical | plan:fix-smpp-auth-clientid-lookup.md |

| POST /api/v1/webhooks | HTTP | client | C-dlr-routing | OK | | clientID from Auth middleware → gRPC ClientId → INSERT webhook_subscriptions.client_id = authenticated UUID (webhooks.go:25, server.go:49, subscription_repository.go:52-54) | | |
| POST /api/v1/webhooks | HTTP | subaccount_child | C-dlr-routing | OK | | child client_id from middleware → own webhook_subscription; no parent leakage | | |
| POST /api/v1/webhooks | HTTP | subaccount_parent | C-dlr-routing | OK | | parent creates own subscriptions; no child scope | | |
| DLR webhook delivery (SMPP provider → webhook callback) | async | client | C-dlr-routing | BROKEN | | F-D1: DLR callback sets MessageID=uuid.Nil; HandleDLR always hits "message not found, skipping"; webhook delivery for DLR events silently dead (delivery_service.go:168-175, sender/stage.go:81-104) | critical | fix: resolve SMPPMessageID→internal UUID via GetBySMPPMessageID before publishing DLRMessage |
| DLR webhook delivery (SMPP provider → webhook callback) | async | subaccount_child | C-dlr-routing | BROKEN | | F-D1: same zero UUID bug; moot (no delivery for any client) | critical | same fix as above |
| DLR webhook delivery (SMPP provider → webhook callback) | async | subaccount_parent | C-dlr-routing | BROKEN | | F-D1: same zero UUID bug | critical | same fix as above |
| DLR status upsert (sms.dlr → messages table) | async | client | C-dlr-routing | BROKEN | | F-D2: status/stage.go:223 uses dlr.MessageID=uuid.Nil for upsert; no rows matched; DLR status never persisted | major | same root cause as F-D1 |
| DLR status upsert (sms.dlr → messages table) | async | subaccount_child | C-dlr-routing | BROKEN | | F-D2: same | major | same fix |
| DLR status upsert (sms.dlr → messages table) | async | subaccount_parent | C-dlr-routing | BROKEN | | F-D2: same | major | same fix |
| DLR SMPP dispatch (smppv1.DeliverDLR → client receiver bind) | SMPP | client | C-dlr-routing | OK | | Reference cell: Cycle 2 confirmed + tests in grpc_server_test.go | | |
| submit_sm / DLR callback (inbound deliver_sm on gateway port) | SMPP | client | C-dlr-routing | BROKEN | | Reference cell: Cycle 2 A6-dlr — esm_class check missing; DLR silently misrouted as MO | critical | plan:fix-smpp-esm-class-dlr-check (stub from Cycle 2) |
| messagingv1 gRPC-external DLR | gRPC-external | client | C-dlr-routing | n/a | | messagingv1 has no DLR-specific RPC for external clients | | |

## Machine-readable

См. `2026-04-21-cycle3-subaccount-matrix.csv`.
