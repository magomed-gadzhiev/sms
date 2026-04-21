# Cycle 2 — SMPP + tenant-glue Coverage Matrix

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Plan:** `docs/superpowers/plans/2026-04-21-api-review-v2-cycle2-smpp-tenant.md`
**Phase completed:** skeleton (all UNKNOWN)
**OK rows with test_file (ratchet baseline):** 0
**Total rows:** заполнить в финале

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` (биллинг/auth) / `major` (контракт) / `minor` (косметика)
- `action`: `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`

## Matrix

| surface | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| SMPP bind_transmitter | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transmitter | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transmitter | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_receiver | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | client | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | subaccount_child | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP bind_transceiver | SMPP | subaccount_parent | A6-auth | BROKEN | | password ignored, system_id only | critical | plan:docs/superpowers/plans/2026-04-21-fix-smpp-auth-password.md |
| SMPP submit_sm | SMPP | client | A6-tlv | BROKEN | | inbound TLV map discarded at gateway; SAR/user_message_reference/message_payload not read | major | plan:TBD (Cycle 2 finale) |
| SMPP submit_sm | SMPP | subaccount_child | A6-tlv | BROKEN | | inbound TLV map discarded at gateway; SAR/user_message_reference/message_payload not read | major | plan:TBD (Cycle 2 finale) |
| SMPP submit_sm | SMPP | subaccount_parent | A6-tlv | BROKEN | | inbound TLV map discarded at gateway; SAR/user_message_reference/message_payload not read | major | plan:TBD (Cycle 2 finale) |
| SMPP submit_multi_sm | SMPP | client | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no submit_multi_sm case in switch | minor | wontfix:deprecated PDU, recommend submit_sm |
| SMPP submit_multi_sm | SMPP | subaccount_child | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no submit_multi_sm case in switch | minor | wontfix:deprecated PDU, recommend submit_sm |
| SMPP submit_multi_sm | SMPP | subaccount_parent | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no submit_multi_sm case in switch | minor | wontfix:deprecated PDU, recommend submit_sm |
| SMPP data_sm | SMPP | client | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no data_sm case in switch | minor | wontfix:low usage, recommend submit_sm |
| SMPP data_sm | SMPP | subaccount_child | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no data_sm case in switch | minor | wontfix:low usage, recommend submit_sm |
| SMPP data_sm | SMPP | subaccount_parent | A6-submit-ext | MISSING | | handler returns generic_nack(ESME_RINVCMDID); no data_sm case in switch | minor | wontfix:low usage, recommend submit_sm |
| SMPP submit_sm | SMPP | client | A6-registered-delivery | BROKEN | | registered_delivery byte stashed in KafkaMessage.Metadata but never read by outbound worker; provider always uses hardcoded value | major | plan:TBD (Cycle 2 finale) |
| SMPP submit_sm | SMPP | subaccount_child | A6-registered-delivery | BROKEN | | registered_delivery byte stashed in KafkaMessage.Metadata but never read by outbound worker; provider always uses hardcoded value | major | plan:TBD (Cycle 2 finale) |
| SMPP submit_sm | SMPP | subaccount_parent | A6-registered-delivery | BROKEN | | registered_delivery byte stashed in KafkaMessage.Metadata but never read by outbound worker; provider always uses hardcoded value | major | plan:TBD (Cycle 2 finale) |
| SMPP deliver_sm (DLR via provider link) | SMPP | client | A6-dlr | BROKEN | | handleDeliverSM never checks esm_class & 0x04; no TLV read; DLR silently treated as MO and discarded | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-deliver-dlr-branching.md |
| SMPP deliver_sm (DLR via provider link) | SMPP | subaccount_child | A6-dlr | BROKEN | | handleDeliverSM never checks esm_class & 0x04; no TLV read; DLR silently treated as MO and discarded | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-deliver-dlr-branching.md |
| SMPP deliver_sm (DLR via provider link) | SMPP | subaccount_parent | A6-dlr | BROKEN | | handleDeliverSM never checks esm_class & 0x04; no TLV read; DLR silently treated as MO and discarded | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-deliver-dlr-branching.md |
| SMPP deliver_sm (outbound to client) | SMPP | client | A6-dlr | OK | internal/gateway/smpp/server/grpc_server_test.go | DLR forwarded via smppv1.DeliverDLR gRPC → sendDeliverSM with esm_class=0x04 | — | — |
| SMPP deliver_sm (outbound to client) | SMPP | subaccount_child | A6-dlr | OK | internal/gateway/smpp/server/grpc_server_test.go | DLR forwarded via smppv1.DeliverDLR gRPC → sendDeliverSM with esm_class=0x04 | — | — |
| SMPP deliver_sm (outbound to client) | SMPP | subaccount_parent | A6-dlr | OK | internal/gateway/smpp/server/grpc_server_test.go | DLR forwarded via smppv1.DeliverDLR gRPC → sendDeliverSM with esm_class=0x04 | — | — |
| SMPP submit_sm | SMPP | client | A6-datacoding | DRIFT | | body bytes cast as-is (string(ShortMessage)); no UCS-2/GSM7/ISO-8859-5 transcoding; DataCoding dropped by KafkaMessage.ToMessage(); provider always receives DataCoding=0 | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-datacoding.md (TBD) |
| SMPP submit_sm | SMPP | subaccount_child | A6-datacoding | DRIFT | | body bytes cast as-is (string(ShortMessage)); no UCS-2/GSM7/ISO-8859-5 transcoding; DataCoding dropped by KafkaMessage.ToMessage(); provider always receives DataCoding=0 | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-datacoding.md (TBD) |
| SMPP submit_sm | SMPP | subaccount_parent | A6-datacoding | DRIFT | | body bytes cast as-is (string(ShortMessage)); no UCS-2/GSM7/ISO-8859-5 transcoding; DataCoding dropped by KafkaMessage.ToMessage(); provider always receives DataCoding=0 | major | plan:docs/superpowers/plans/2026-04-21-fix-smpp-datacoding.md (TBD) |
| SMPP submit_sm (multipart UDH) | SMPP | client | A6-long-msg | PARTIAL | | UDH esm_class & 0x40 parsed (handler.go:313-323), ref/total/part extracted; BUT metadata in KafkaMessage.Metadata dropped by ToMessage(); downstream receives segments as independent messages; SAR TLV path broken (see A6-tlv) | major | plan:covered by A6-tlv fix plan (KafkaMessage model) |
| SMPP submit_sm (multipart UDH) | SMPP | subaccount_child | A6-long-msg | PARTIAL | | UDH esm_class & 0x40 parsed (handler.go:313-323), ref/total/part extracted; BUT metadata in KafkaMessage.Metadata dropped by ToMessage(); downstream receives segments as independent messages; SAR TLV path broken (see A6-tlv) | major | plan:covered by A6-tlv fix plan (KafkaMessage model) |
| SMPP submit_sm (multipart UDH) | SMPP | subaccount_parent | A6-long-msg | PARTIAL | | UDH esm_class & 0x40 parsed (handler.go:313-323), ref/total/part extracted; BUT metadata in KafkaMessage.Metadata dropped by ToMessage(); downstream receives segments as independent messages; SAR TLV path broken (see A6-tlv) | major | plan:covered by A6-tlv fix plan (KafkaMessage model) |

| admin RPCs (cascadev1 ChannelAdmin + StrategyAdmin + tarificationv1 + billingv1 ListBalances) | gRPC | client | A7-admin-exposure | OK | — | not registered in cmd/client-gateway or cmd/api; only reachable internally | — | wontfix:verified gated |
| admin RPCs (cascadev1 ChannelAdmin + StrategyAdmin + tarificationv1 + billingv1 ListBalances) | gRPC | subaccount_child | A7-admin-exposure | OK | — | not registered in cmd/client-gateway or cmd/api; only reachable internally | — | wontfix:verified gated |
| admin RPCs (cascadev1 ChannelAdmin + StrategyAdmin + tarificationv1 + billingv1 ListBalances) | gRPC | subaccount_parent | A7-admin-exposure | OK | — | not registered in cmd/client-gateway or cmd/api; only reachable internally | — | wontfix:verified gated |
| messagingv1.SendMessage | gRPC | client | A7-enforcement | BROKEN | — | gRPC interceptor hardcoded dummy ID; auth permanently disabled; all callers get same tenant 00000000-0000-0000-0000-000000000001 | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.SendMessage | gRPC | subaccount_child | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.SendMessage | gRPC | subaccount_parent | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.GetMessageHistory | gRPC | client | A7-enforcement | BROKEN | — | gRPC interceptor hardcoded dummy ID; auth permanently disabled | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.GetMessageHistory | gRPC | subaccount_child | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.GetMessageHistory | gRPC | subaccount_parent | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| billingv1.GetBalance | gRPC | client | A7-enforcement | BROKEN | — | gRPC interceptor hardcoded dummy ID; auth permanently disabled | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| billingv1.GetBalance | gRPC | subaccount_child | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| billingv1.GetBalance | gRPC | subaccount_parent | A7-enforcement | BROKEN | — | same hardcoded interceptor bypass | critical | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| webhookv1.ListSubscriptions | HTTP | client | A7-enforcement | OK | — | HTTP middleware correctly validates token; ctx.ClientId injected; handler reads ctx only | — | — |
| webhookv1.ListSubscriptions | HTTP | subaccount_child | A7-enforcement | OK | — | same | — | — |
| webhookv1.ListSubscriptions | HTTP | subaccount_parent | A7-enforcement | OK | — | same | — | — |
| templatev1.ListTemplates | HTTP | client | A7-enforcement | OK | — | HTTP middleware correctly validates token; ctx.ClientId injected; handler reads ctx only | — | — |
| templatev1.ListTemplates | HTTP | subaccount_child | A7-enforcement | OK | — | same | — | — |
| templatev1.ListTemplates | HTTP | subaccount_parent | A7-enforcement | OK | — | same | — | — |
| messagingv1.GetMessageStatus | gRPC | client | A7-enforcement | DRIFT | — | downstream allows nil clientID (optional check); ownership enforcement skipped when clientID omitted; exploitable if service called directly bypassing proxy | minor | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.GetMessageStatus | gRPC | subaccount_child | A7-enforcement | DRIFT | — | same nil-clientID gap | minor | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |
| messagingv1.GetMessageStatus | gRPC | subaccount_parent | A7-enforcement | DRIFT | — | same nil-clientID gap | minor | plan:docs/superpowers/plans/2026-04-21-fix-grpc-auth-interceptor.md |

## Machine-readable

См. `2026-04-21-cycle2-smpp-glue-matrix.csv`.
