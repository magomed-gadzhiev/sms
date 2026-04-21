# API Review v2 — Surface Inventory

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Revision (master HEAD):** <ХЭШ> — заполнить в Task 8

## 1. HTTP Endpoints (cmd/client-gateway)

| Method | Path | Handler file:line | Service call | Response shape |
|---|---|---|---|---|
| GET | /health | n/a (health) | n/a (health) | inline map: status, checks |
| GET | /health/live | n/a (health) | n/a (health) | inline map: status |
| GET | /health/ready | n/a (health) | n/a (health) | inline map: status |
| POST | /api/v1/sms/send | internal/gateway/client/handlers/sms.go:57 | messagingv1.SendMessage | inline map: message_id, status, segment_count, created_at, error, scheduled_at |
| POST | /api/v1/sms/batch | internal/gateway/client/handlers/sms.go:173 | messagingv1.SendBatch | inline map: results, success_count, failed_count |
| GET | /api/v1/sms/status/{id} | internal/gateway/client/handlers/sms.go:283 | messagingv1.GetMessageStatus | inline map: message_id, status, status_message, timestamps, smpp_message_id, error_code |
| GET | /api/v1/sms/history | internal/gateway/client/handlers/sms.go:346 | messagingv1.GetMessageHistory | inline map: messages, total, limit, offset |
| GET | /api/v1/sms/scheduled | internal/gateway/client/handlers/sms.go:457 | messagingv1.ListScheduledMessages | inline map: messages, total, limit, offset |
| DELETE | /api/v1/sms/{id} | internal/gateway/client/handlers/sms.go:526 | messagingv1.CancelMessage | 204 No Content |
| GET | /api/v1/account/balance | internal/gateway/client/handlers/account.go:34 | billingv1.GetBalance | inline map: client_id, balance, currency, updated_at |
| GET | /api/v1/account/stats | internal/gateway/client/handlers/account.go:66 | analyticsv1.GetStatistics | inline map: client_id, from, to, group_by, groups, totals |
| POST | /api/v1/webhooks | internal/gateway/client/handlers/webhooks.go:24 | webhookv1.CreateSubscription | inline map: id, client_id, url, event_types, active, secret, created_at |
| GET | /api/v1/webhooks | internal/gateway/client/handlers/webhooks.go:73 | webhookv1.ListSubscriptions | inline map: subscriptions |
| GET | /api/v1/webhooks/{id} | internal/gateway/client/handlers/webhooks.go:95 | webhookv1.GetSubscription | inline map: id, client_id, url, event_types, active, timestamps |
| PUT | /api/v1/webhooks/{id} | internal/gateway/client/handlers/webhooks.go:115 | webhookv1.UpdateSubscription | inline map: id, client_id, url, event_types, active, timestamps |
| DELETE | /api/v1/webhooks/{id} | internal/gateway/client/handlers/webhooks.go:152 | webhookv1.DeleteSubscription | 204 No Content |
| POST | /api/v1/lookup | internal/gateway/client/handlers/lookup.go:41 | routingv1.NumberLookup | inline map: msisdn, operator_mccmnc, operator_name, number_status, country_code, number_type, is_ported, cached |
| POST | /api/v1/lookup/bulk | internal/gateway/client/handlers/lookup.go:100 | routingv1.BulkNumberLookup | inline map: results, total_count, success_count, failed_count |
| GET | /api/v1/lookup/history | internal/gateway/client/handlers/lookup.go:173 | routingv1.GetLookupHistory | inline map: items, total_count, page, page_size |
| POST | /api/v1/templates | internal/gateway/client/handlers/templates.go:25 | templatev1.CreateTemplate | templateToMap (id, client_id, name, body, variables, status, rejection_reason, timestamps) |
| GET | /api/v1/templates | internal/gateway/client/handlers/templates.go:63 | templatev1.ListTemplates | inline map: templates, total, limit, offset |
| GET | /api/v1/templates/{id} | internal/gateway/client/handlers/templates.go:109 | templatev1.GetTemplate | templateToMap (id, client_id, name, body, variables, status, rejection_reason, timestamps) |
| PUT | /api/v1/templates/{id} | internal/gateway/client/handlers/templates.go:129 | templatev1.UpdateTemplate | templateToMap (id, client_id, name, body, variables, status, rejection_reason, timestamps) |
| DELETE | /api/v1/templates/{id} | internal/gateway/client/handlers/templates.go:163 | templatev1.DeleteTemplate | 204 No Content |
| GET | /api/v1/templates/{id}/audit | internal/gateway/client/handlers/templates.go:183 | templatev1.GetTemplateAuditLog | inline map: entries, total |
| GET | /docs | n/a (docs) | n/a (docs) | HTML (Redoc UI) |
| GET | /docs/swagger | n/a (docs) | n/a (docs) | HTML (Swagger UI) |
| GET | /docs/grpc | n/a (docs) | n/a (docs) | HTML (gRPC docs) |
| GET | /docs/openapi.yaml | n/a (docs) | n/a (docs) | YAML (OpenAPI spec) |
| POST | /api/v1/cascade/deliveries | internal/gateway/client/handlers/cascade.go:35 | cascadev1.CreateDelivery | cascadev1.CreateDeliveryResponse (proto) |
| GET | /api/v1/cascade/deliveries | internal/gateway/client/handlers/cascade.go:87 | cascadev1.ListDeliveries | cascadev1.ListDeliveriesResponse (proto) |
| GET | /api/v1/cascade/deliveries/{id} | internal/gateway/client/handlers/cascade.go:66 | cascadev1.GetDelivery | cascadev1.GetDeliveryResponse (proto) |
| GET | /api/v1/cascade/stats | internal/gateway/client/handlers/cascade.go:120 | cascadev1.GetDeliveryStats | cascadev1.GetDeliveryStatsResponse (proto) |

## 2. gRPC External (cmd/client-gateway CLIENT_GRPC_PORT)

One service registered: `messagingv1.RegisterMessagingServiceServer` (default port 9090, overridable via `CLIENT_GRPC_PORT`).
Auth: unary interceptor `clientgrpc.AuthInterceptor` applied to all RPCs.
Reflection: enabled only when `SMPP_SERVICE_ENV=development`.

| Service | RPC | Handler file:line | Downstream internal gRPC | Notes |
|---|---|---|---|---|
| MessagingService | SendMessage | internal/gateway/client/grpc/server.go:38 | messagingv1.SendMessage | Forces req.ClientId from auth context; rejects mismatched client_id |
| MessagingService | SendBatch | internal/gateway/client/grpc/server.go:62 | messagingv1.SendBatch | Forces req.ClientId + all per-message ClientId from auth context |
| MessagingService | GetMessageStatus | internal/gateway/client/grpc/server.go:98 | messagingv1.GetMessageStatus | Forces req.ClientId from auth context |
| MessagingService | GetMessageHistory | internal/gateway/client/grpc/server.go:122 | messagingv1.GetMessageHistory | Forces req.ClientId from auth context |
| MessagingService | ProcessDLR | internal/gateway/client/grpc/server.go:172 | messagingv1.ProcessDLR | No client_id scoping — proxies directly; comment notes Messaging Service will check rights |
| MessagingService | CancelMessage | not implemented | — | Falls through to UnimplementedMessagingServiceServer (codes.Unimplemented) |
| MessagingService | ListScheduledMessages | not implemented | — | Falls through to UnimplementedMessagingServiceServer (codes.Unimplemented) |

**Surprises / anomalies:**

1. `CancelMessage` and `ListScheduledMessages` are declared in the proto and have HTTP equivalents (Task 1 table rows), but are **not implemented** in the external gRPC server. Callers on the gRPC channel get `codes.Unimplemented`.
2. `server.go` contains two methods outside the `MessagingServiceServer` interface — `GetBalance` (line 146, calls `billingClient.GetBalance`) and `GetStatistics` (line 192, calls `analyticsClient.GetStatistics`). These are **unreachable via gRPC** because they are not part of the registered service descriptor. They are dead code on the external gRPC port.
3. No `.proto` source files exist in `api/proto/messagingv1/` — only generated `.pb.go` files. The proto source is `messaging/messaging.proto` per the generated file header but is absent from the repository.
4. No mTLS is configured — `grpc.NewServer` uses only a `UnaryInterceptor`; no `grpc.Creds` option is passed.

## 3. SMPP (cmd/smpp-gateway)

### 3.1. PDU Commands

Switch/case dispatch in `internal/gateway/smpp/server/handler.go:73` (`HandlePDU`).
Auth for all bind variants: `AuthAdapter.AuthenticateBySystemID` (via `h.authenticate`), which calls `authv1.Authenticate` with `system_id` as API key; the SMPP `password` field is **not verified** — only `system_id` is passed to Auth Service.

| PDU | Handler file:line | Service call | Notes |
|---|---|---|---|
| `bind_receiver` | handler.go:113 | `authv1.Authenticate` via `AuthAdapter.AuthenticateBySystemID` | Auth by system_id only; password ignored. Saves session binding to Redis. |
| `bind_transmitter` | handler.go:161 | `authv1.Authenticate` via `AuthAdapter.AuthenticateBySystemID` | Same auth gap. Saves session binding to Redis. |
| `bind_transceiver` | handler.go:209 | `authv1.Authenticate` via `AuthAdapter.AuthenticateBySystemID` | Same auth gap. Saves session binding to Redis. |
| `unbind` | handler.go:257 | none | Calls `session.Unbind()`, deletes Redis binding. |
| `submit_sm` | handler.go:277 | `queue.Producer.PublishOutgoing` (Kafka topic: outgoing) | Rate-limit check; UDH parsed for multipart; message mapping saved to Redis for DLR routing. No gRPC call — goes directly to Kafka. |
| `deliver_sm` | handler.go:547 | `storage.OptOutRepository.Add` (DB, conditional) | No MO/DLR branching: `esm_class` and `receipted_message_id` TLV are **not inspected**. Handler treats all deliver_sm as MO. Opt-out recorded if text matches STOP/СТОП/ОТПИСАТЬСЯ/UNSUBSCRIBE and session has ClientID. |
| `enquire_link` | handler.go:401 | none | Refreshes Redis session TTL; replies with `enquire_link_resp`. |
| `query_sm` | handler.go:418 | `storage.MessageRepository.GetByMessageID` (DB) | Returns SMPP message state mapped from internal status. Returns `ESME_RQUERYFAIL` if `messageRepo` is nil. |
| `cancel_sm` | handler.go:465 | `storage.MessageRepository.UpdateStatusByMessageID` (DB) | Sets status to `cancelled`. Returns `ESME_RCANCELFAIL` if `messageRepo` is nil. |
| `replace_sm` | handler.go:498 | `storage.MessageRepository.UpdateTextByMessageID` (DB) | Only allows replace for `pending`/`queued` messages. Returns `ESME_RREPLACEFAIL` if `messageRepo` is nil. |

**Declared in protocol constants but NOT handled (fall through to `generic_nack` with `ESME_RINVCMDID`):**

- `submit_multi_sm` (0x00000021) — declared in `internal/smpp/protocol/constants.go:24`
- `data_sm` (0x00000103) — declared in `internal/smpp/protocol/constants.go:31`

**Critical finding — DLR not distinguished from MO in `deliver_sm`:** The handler at line 547 does not check `esm_class & 0x04` (delivery-receipt bit) nor look for the `receipted_message_id` TLV. All inbound `deliver_sm` are treated as mobile-originated messages. Actual DLRs arriving on an SMPP link (as opposed to the internal gRPC path) would be processed as opt-out candidates rather than delivery receipts, silently dropped unless text matches a stop-keyword.

### 3.2. SMPP TLVs

**Zero `Tag*` / `TLV_*` constants declared.** Neither `internal/smpp/protocol/constants.go`, `pdu.go`, nor `pdu_helper.go` define any TLV tag constants. The TLV wire format is implemented generically: `map[uint16][]byte` fields exist on `SubmitSMPDU`, `DeliverSMPDU`, and `BindRespPDU`; encoder (`encoder.go:496`) and decoder (`decoder.go:571`) iterate the map writing/reading raw `uint16` tag values.

**Magic-literal scan in `internal/gateway/smpp/` and `internal/smsc/`:**

No magic TLV hex literals (0x001E `receipted_message_id`, 0x0427 `message_state`, 0x020C `network_error_code`) were found anywhere in `internal/gateway/smpp/`. The TLV map on `DeliverSMPDU` is populated by the decoder but **never read** by `handleDeliverSM` in the new gateway layer — confirming the DLR non-distinction finding above.

In `internal/gateway/smpp/server/handler.go:658` the `BindRespPDU.TLV` map is allocated (`make(map[uint16][]byte)`) but left empty — the `sc_interface_version` TLV (0x0210) is not populated in bind responses.

| Tag (hex) | Symbolic name | Declared in | Used in submit_sm | Used in deliver_sm | Mapped field | Notes |
|---|---|---|---|---|---|---|
| — | — | No Tag constants declared anywhere in `internal/smpp/protocol/` | — | — | — | TLVs handled generically as `map[uint16][]byte`; no named constants exist |

### 3.3. smppv1 control-plane RPC

Service: `smpp.v1.SMPPGateway` (defined in `api/proto/smppv1/smpp_grpc.pb.go`).
Registered in `cmd/smpp-gateway/main.go:162`: `smppv1.RegisterSMPPGatewayServer(grpcServer, dlrGRPCServer)`.
Implementation struct: `GRPCServer` in `internal/gateway/smpp/server/grpc_server.go`.
Listening port: `GRPC_INTERNAL_PORT` env var, default `9095`.
No mTLS, no auth interceptor — internal network trust only.

| RPC | Handler file:line | Purpose | Notes |
|---|---|---|---|
| `DeliverDLR` | grpc_server.go:45 | Delivers a DLR (Delivery Receipt) to a connected SMPP client by finding the session for the given `system_id` and writing a `deliver_sm` PDU with `esm_class=0x04` to its TCP connection. | Called by the dlr-delivery service. Returns `Delivered: false` (not an error) when session not found or cannot receive; caller must handle retry logic. Metrics: `SMPPDLRDeliveryFailed` / `SMPPDLRDelivered`. |

## 4. gRPC Internal (top-5 hot-path services)

All five services use **request-field transport** for tenant context (`client_id` string field in the protobuf message). No service reads `client_id` from gRPC metadata (`metadata.FromIncomingContext`). No service has a health/reflection RPC registered.

| Service | RPC | client_id transport | Caller(s) | Notes |
|---|---|---|---|---|
| **messagingv1** | SendMessage | req field `client_id` | internal/gateway/client/grpc/server.go, internal/gateway/client/handlers/sms.go | — |
| **messagingv1** | SendBatch | req field `client_id` | internal/gateway/client/grpc/server.go, internal/gateway/client/handlers/sms.go | — |
| **messagingv1** | GetMessageStatus | req field `client_id` | internal/gateway/client/grpc/server.go, internal/gateway/client/handlers/sms.go | — |
| **messagingv1** | GetMessageHistory | req field `client_id` | internal/gateway/client/grpc/server.go, internal/gateway/portal/handlers/messages.go | — |
| **messagingv1** | ProcessDLR | — | internal/gateway/client/grpc/server.go | **TENANT-MISSING** — no client_id in ProcessDLRRequest; DLR is provider-internal event identified by message_id only |
| **messagingv1** | CancelMessage | req field `client_id` | internal/gateway/client/handlers/sms.go | — |
| **messagingv1** | ListScheduledMessages | req field `client_id` | internal/gateway/client/handlers/sms.go | — |
| **billingv1** | GetBalance | req field `client_id` | internal/gateway/client/grpc/server.go, internal/gateway/client/handlers/account.go, internal/gateway/portal/handlers/billing.go | — |
| **billingv1** | ChargeMessage | req field `client_id` | internal/pipeline/sender/stage.go, internal/services/tarification/application/saga.go | — |
| **billingv1** | ChargeMessageDual | req fields `sub_account_id` + `aggregator_id` (no `client_id`) | internal/services/cascade/application/billing_integration.go, internal/pipeline/sender/stage.go | **TENANT-MISSING** — uses `sub_account_id`/`aggregator_id` pair instead of canonical `client_id`; semantics differ |
| **billingv1** | AddCredits | req field `client_id` | internal/gateway/admin/handlers/billing.go, internal/gateway/portal/handlers/billing.go | — |
| **billingv1** | DeductCredits | req field `client_id` | internal/gateway/admin/handlers/billing.go | — |
| **billingv1** | GetTransactionHistory | req field `client_id` | internal/gateway/portal/handlers/billing.go | — |
| **billingv1** | GetPricingRules | req field `client_id` | internal/gateway/portal/handlers/tariffs.go | — |
| **billingv1** | CreatePricingRule | req field `client_id` (optional — empty = global rule) | internal/gateway/admin/handlers/billing.go | `client_id` is optional; empty means global rule |
| **billingv1** | TransferBalance | req fields `from_client_id` + `to_client_id` | internal/gateway/portal/handlers/sub_accounts.go | Uses `from_client_id`/`to_client_id` pair — consistent naming but different from other RPCs |
| **billingv1** | FreezeAccount | req field `client_id` | internal/gateway/admin/handlers/billing.go | — |
| **billingv1** | UnfreezeAccount | req field `client_id` | internal/gateway/admin/handlers/billing.go | — |
| **billingv1** | SetCreditLimit | req field `client_id` | internal/gateway/admin/handlers/billing.go | — |
| **billingv1** | SetLowBalanceThreshold | req field `client_id` | internal/gateway/portal/handlers/alerts.go | — |
| **billingv1** | ListBalances | — | internal/gateway/admin/handlers/billing.go | **TENANT-MISSING** — ListBalancesRequest has no client_id; admin-only, returns all tenants with filter by `search`/`status`/`below_threshold` |
| **tarificationv1** | TarifyMessage | req field `client_id` | internal/pipeline/sender/stage.go, internal/services/tarification/grpc/server.go | hot-path; also quota gating for resellers |
| **tarificationv1** | CommitCharge | req field `client_id` | internal/services/tarification/application/commit_retry_worker.go | `client_id` is sub-account here per comment in struct |
| **tarificationv1** | CreateSenderRegistration | req field `client_id` | internal/gateway/admin/handlers/tarification.go | — |
| **tarificationv1** | ListSenderRegistrations | req field `client_id` | internal/gateway/admin/handlers/tarification.go, internal/gateway/portal/handlers/tariffs.go | — |
| **tarificationv1** | UpdateSenderRegistration | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — UpdateSenderRegistrationRequest has only `id`, `status`, `type`; no client_id; authorization is by registration id only |
| **tarificationv1** | CreateTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — request has `operator_id`, `sender_category`, `strategy`; no client_id; operator-scoped, not tenant-scoped |
| **tarificationv1** | GetTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — request has only `id` |
| **tarificationv1** | ListTariffPlans | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — filtered by `operator_id` only |
| **tarificationv1** | UpdateTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — request has only `id`, `active` |
| **tarificationv1** | CreateTariffPeriod | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — child of tariff_plan, no client_id |
| **tarificationv1** | CreateTariffTier | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — child of tariff_period, no client_id |
| **tarificationv1** | UpdateTariffTier | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — request has only `id`, `from_count`, `price_per_segment` |
| **tarificationv1** | CreatePricingPeriod | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — child of tariff_period, no client_id |
| **tarificationv1** | CreatePrepaidFee | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — keyed by tariff_plan_id/tariff_period_id only |
| **tarificationv1** | TarifyLookup | req field `client_id` | internal/gateway/admin/handlers/client_routing.go | — |
| **tarificationv1** | GetUsageCounter | req field `client_id` | internal/services/tarification/grpc/server.go | — |
| **tarificationv1** | ListUsageCounters | req field `client_id` | internal/services/tarification/grpc/server.go | — |
| **tarificationv1** | CreateProviderTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — provider/operator scoped; no client_id |
| **tarificationv1** | GetProviderTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — keyed by `id` only |
| **tarificationv1** | ListProviderTariffPlans | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — filtered by `provider_id` only |
| **tarificationv1** | UpdateProviderTariffPlan | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — keyed by `id` only |
| **tarificationv1** | CreateProviderTariffPeriod | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — child of provider tariff plan |
| **tarificationv1** | CreateProviderTariffTier | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — child of provider tariff period |
| **tarificationv1** | UpdateProviderTariffTier | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — keyed by `id` only |
| **tarificationv1** | GetMarginReport | req field `client_id` | internal/gateway/portal/handlers/tariffs.go | — |
| **tarificationv1** | CreateSenderBillingRecord | req field `client_id` | internal/gateway/admin/handlers/tarification.go | — |
| **tarificationv1** | ListSenderBillingRecords | — | internal/gateway/admin/handlers/tarification.go | **TENANT-MISSING** — filtered by `sender_registration_id` only; client_id not in request |
| **cascadev1 / CascadeService** | CreateDelivery | req field `client_id` | internal/gateway/client/handlers/cascade.go | — |
| **cascadev1 / CascadeService** | GetDelivery | req field `client_id` | internal/gateway/client/handlers/cascade.go | — |
| **cascadev1 / CascadeService** | ListDeliveries | req field `client_id` | internal/gateway/client/handlers/cascade.go | — |
| **cascadev1 / CascadeService** | GetDeliveryStats | req field `client_id` | internal/gateway/client/handlers/cascade.go | — |
| **cascadev1 / ChannelAdminService** | ListChannels | — | internal/gateway/portal/handlers/cascade_channels.go | **TENANT-MISSING** — ListChannelsRequest is empty; returns global channel list |
| **cascadev1 / ChannelAdminService** | GetChannel | — | internal/gateway/portal/handlers/cascade_channels.go | **TENANT-MISSING** — keyed by `channel_id` only |
| **cascadev1 / ChannelAdminService** | CreateChannel | — | internal/gateway/portal/handlers/cascade_channels.go | **TENANT-MISSING** — no client_id in CreateChannelRequest |
| **cascadev1 / ChannelAdminService** | UpdateChannel | — | internal/gateway/portal/handlers/cascade_channels.go | **TENANT-MISSING** — no client_id in UpdateChannelRequest |
| **cascadev1 / ChannelAdminService** | ToggleChannel | — | internal/gateway/portal/handlers/cascade_channels.go | **TENANT-MISSING** — no client_id in ToggleChannelRequest |
| **cascadev1 / StrategyAdminService** | ListStrategies | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in ListStrategiesRequest |
| **cascadev1 / StrategyAdminService** | GetStrategy | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — keyed by `id` only |
| **cascadev1 / StrategyAdminService** | CreateStrategy | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in CreateStrategyRequest |
| **cascadev1 / StrategyAdminService** | UpdateStrategy | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in UpdateStrategyRequest |
| **cascadev1 / StrategyAdminService** | DeleteStrategy | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in DeleteStrategyRequest |
| **cascadev1 / StrategyAdminService** | GetOperatorChannelSupport | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in GetOCSRequest |
| **cascadev1 / StrategyAdminService** | UpdateOperatorChannelSupport | — | internal/gateway/portal/handlers/cascade_strategies.go | **TENANT-MISSING** — no client_id in UpdateOCSRequest |
| **webhookv1** | CreateSubscription | req field `client_id` | internal/gateway/client/handlers/webhooks.go, internal/gateway/portal/handlers/webhooks.go | — |
| **webhookv1** | UpdateSubscription | req field `client_id` | internal/gateway/client/handlers/webhooks.go, internal/gateway/portal/handlers/webhooks.go | — |
| **webhookv1** | DeleteSubscription | req field `client_id` | internal/gateway/client/handlers/webhooks.go, internal/gateway/portal/handlers/webhooks.go | — |
| **webhookv1** | GetSubscription | req field `client_id` | internal/gateway/client/handlers/webhooks.go, internal/gateway/portal/handlers/webhooks.go | — |
| **webhookv1** | ListSubscriptions | req field `client_id` | internal/gateway/client/handlers/webhooks.go, internal/gateway/portal/handlers/webhooks.go | — |

**Notes on mixed semantics within billingv1:**
- `ChargeMessageDual` uses `sub_account_id` + `aggregator_id` instead of canonical `client_id`.
- `TransferBalance` uses `from_client_id` + `to_client_id` pair (consistent semantics, different field names).
- `CreatePricingRule` has an optional `client_id` (empty = global rule); only RPC in the entire surface where client_id is intentionally absent for a valid tenant-agnostic case.

**Pattern for tarificationv1 admin RPCs:** The operator-tariff management RPCs (CreateTariffPlan through UpdateProviderTariffTier) are all TENANT-MISSING by design — they manage operator-level pricing structures shared across tenants. The concern is whether the admin gateway properly gates access to these by internal auth rather than relying on the absence of client_id to "be safe".

## 5. Findings

### F1. R1 — cmd/api legacy status

_TODO: Task 6_

### F2. B1 — Integration test infrastructure

_TODO: Task 7_

### F3. B2 — AuthAdapter user_id=client_id kludge

_TODO: Task 7_

### F4. B3 — Dual-charge wiring in TarifyMessage

_TODO: Task 7_

## 6. Go/no-go для циклов 1/2/3

_TODO: Task 8_
