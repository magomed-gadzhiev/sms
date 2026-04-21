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

## Machine-readable

См. `2026-04-21-cycle3-subaccount-matrix.csv`.
