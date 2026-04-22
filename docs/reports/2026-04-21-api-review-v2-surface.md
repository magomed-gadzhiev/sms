# API Review v2 — Surface Inventory

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Revision:** branch `review/api-v2-phase-0`, последний inventory-коммит `df64b30` (B1/B2/B3 re-check). База от master — коммит `189a305` (план Phase 0).

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

**Deployment signals:**
- docker-compose: `deployments/docker-compose.yml` has NO service referencing `cmd/api` or `api.Dockerfile`. All gateways deployed are `client-gateway` (×2), `admin-gateway` (×2), `portal-gateway`, plus `smpp-gateway`, `worker`, `pipeline-*`, and domain microservices. `cmd/api` is absent.
- deploy/k8s/helm: no such directories exist in the repo.
- scripts/server.sh: no reference to `cmd/api` or `api-gateway` binary.
- Dockerfile: `deployments/docker/api.Dockerfile` exists and builds `./cmd/api` into binary `api` — but this Dockerfile is **not referenced by docker-compose.yml** and thus never executed in any known deployment.

**CI signals:**
`.github/workflows/ci.yml` runs `go build ./...` (compiles all packages including `cmd/api`) and `go test -count=1 -short ./...`. There are **no service-specific build/push steps** — CI is a monorepo build, not a per-binary Docker publish. No image is built or pushed for `cmd/api` in CI; same applies to `cmd/client-gateway`.

**Git activity (last ~2.5 months, since 2026-02-01):**
- cmd/api: 2 commits
- cmd/client-gateway: 11 commits
- internal/api: 22 commits
- internal/gateway/client: 31 commits

**Code signals in cmd/api:**
`cmd/api/main.go` (224 lines) is a fully functional server — initialises DB, Redis, Kafka, registers HTTP and gRPC handlers, runs graceful shutdown. There are **no TODO/FIXME/deprecated comments** anywhere in the file. The binary calls itself `api-gateway` in logging. It uses `internal/api/` packages (the monolith API layer), not `internal/gateway/client/` (the microservice layer).

The 2 recent commits to `cmd/api` (0678d9b, 1c0c2e8) added gRPC tracing and tenant-aware logging middleware — active feature work, not removal or deprecation notices.

**Verdict:** legacy NOT confirmed with full certainty. `cmd/api` is absent from all deployment configs and `scripts/server.sh`, which is strong evidence it is not running in the current production stack. However it is receiving active feature commits (2 in last 2.5 months vs 11 for `cmd/client-gateway`), has no deprecation markers, and its Dockerfile exists and is buildable. It looks like a parallel/legacy binary that is no longer deployed but has not been formally retired or marked deprecated.

**Impact on review scope:**
The umbrella scope (Cycle 1 = `cmd/client-gateway` + `internal/gateway/client/`; Cycle 2 = `cmd/admin-gateway` + `internal/gateway/admin/`) stands because `cmd/api` is not deployed. However the existence of active commits to `cmd/api` and `internal/api/` (22 commits) without a deprecation notice is a latent risk: a future developer could deploy the old binary by mistake, or the `internal/api/` layer could silently diverge. Recommended follow-up (out of scope for Cycle 1/2): add a `//go:build ignore` or `// Deprecated:` comment to `cmd/api/main.go` and remove `deployments/docker/api.Dockerfile` to prevent accidental deployment.

### F2. B1 — Integration test infrastructure

**CI workflows with postgres/redis service containers:** No. The single workflow `.github/workflows/ci.yml` has two jobs (`go`, `frontend`). Neither job declares a `services:` block. No postgres or redis container is spun up in CI.

**CI jobs running -tags=integration:** No. The `go` job runs `go test -count=1 -short ./...` — `-short` flag, no `-tags=integration` anywhere in the workflow file.

**scripts/check.sh integration mode:** No. `scripts/check.sh` supports only `--with-tests` (which maps to `go test -count=1 -short ./...`). No `--integration` flag, no `-tags=integration` invocation anywhere in the script.

**Integration-tagged Go files (.go with build tag):** 0. Grep for `^//go:build integration$` and `^// \+build integration$` across `**/*.go` returned no matches.

**Names of those files (sample):** n/a — no integration-tagged files exist.

**Decision on B1:**
- CLOSED — if CI has postgres+redis AND runs -tags=integration.
- OPEN — otherwise. Cycle 3 blocked until closed or explicitly downscoped.

**Current verdict:** OPEN. CI has no service containers and no integration test invocation. Zero integration-tagged test files exist in the codebase. Cycle 3 (subaccount billing path through CommitCharge + ChargeMessageDual) cannot be validated end-to-end without either: (a) spinning up postgres+redis in CI and adding `-tags=integration` test suite, or (b) explicitly downscoping Cycle 3 to unit tests with mocked billing client only.

### F3. B2 — AuthAdapter user_id=client_id kludge

**auth_adapter.go status:** `internal/gateway/smpp/server/auth_adapter.go`, 128 lines reviewed in full.

**Kludge presence:** Still present — same kludge in two methods.

**Evidence:**
```
// auth_adapter.go:53-56 (ValidateToken)
if resp.User.Id != "" {
    // Попытка получить client_id из метаданных пользователя
    // В текущей реализации используем user_id как client_id
    clientID = resp.User.Id
}

// auth_adapter.go:100-103 (AuthenticateBySystemID)
if resp.User.Id != "" {
    // Используем user_id как client_id (можно будет получить из Client Service)
    clientID = resp.User.Id
}

// auth_adapter.go:117-127 (GetClientID)
// GetClientID возвращает ClientID из UserID (временная реализация)
// В будущем можно получить через Client Service
func (a *AuthAdapter) GetClientID(ctx context.Context, userID string) (*uuid.UUID, error) {
    // Временная реализация: пытаемся преобразовать user_id в UUID
    id, err := uuid.Parse(userID)
    if err != nil {
        return nil, nil  // ClientID опционален — silently drops lookup failure
    }
    return &id, nil
}
```

The conflation is present in both `ValidateToken` (line 56: `clientID = resp.User.Id`) and `AuthenticateBySystemID` (line 103: `clientID = resp.User.Id`). A third kludge exists in `GetClientID` (line 120–127): "временная реализация" that simply parses user_id as UUID rather than performing a real Client Service lookup. All three paths result in `UserInfo.ClientID == UserInfo.UserID`.

**Decision on B2:**
- CLOSED — proper client_id resolution via client repo or authv1 service, no conflation.
- OPEN — conflation still present or replaced by different kludge.

**Current verdict:** OPEN. Three conflation points remain: `ValidateToken`, `AuthenticateBySystemID`, and `GetClientID`. No Client Service lookup exists. The `authv1.Authenticate` response only returns a `User` struct with `.Id` (the auth user ID), and `clientID` is assigned directly from it.

**Impact on Cycle 3 C-visibility (SMPP subaccount):** SMPP sessions authenticated via `AuthenticateBySystemID` will have `ClientID == UserID` — any Cycle 3 test asserting that `CommitCharge.ClientID` resolves to the correct billing sub-account will pass only if the user UUID happens to equal the billing client UUID, which is not guaranteed and not tested.

### F4. B3 — Dual-charge wiring in TarifyMessage

**ChargeMessageDual callers:**
- `api/proto/billingv1/billing_grpc.pb.go:104` — generated gRPC client stub `(*billingServiceClient).ChargeMessageDual`
- `internal/services/tarification/application/commit_charge.go:161` — `s.saga.ChargeDualAtomic(ctx, dualReq)` which wraps `ChargeMessageDual` (subaccount branch of `CommitCharge`)
- `internal/gateway/portal/handlers/billing_test.go:48` — mock implementation
- `internal/gateway/client/grpc/server_test.go:103` — mock implementation
- `internal/services/cascade/application/billing_integration.go` — cascade billing integration (separate path)

**TarifyMessage body calls:** When `commitOnSubmitEnabled == false` (default): calls `s.saga.Charge()` only — no `ChargeMessageDual` anywhere in the legacy branch. When `commitOnSubmitEnabled == true`: calls `s.Calculate()` (read-only) and returns immediately — no charge at all; charge deferred to `CommitCharge`.

**CommitCharge body calls:** `CommitCharge` (`commit_charge.go:56`) calls:
- For direct clients (line 118): `s.saga.Charge()` — single charge via `billingv1.ChargeMessage`
- For subaccounts (line 161): `s.saga.ChargeDualAtomic(ctx, dualReq)` — which calls `billingv1.ChargeMessageDual` atomically

**Feature flag gating dual-charge:** Flag name: `tarification.commit_on_submit_enabled` (Go field: `CommitOnSubmitEnabled bool`). Default value: **false** (hardcoded in `internal/config/config.go:420`: `v.SetDefault("tarification.commit_on_submit_enabled", false)`). Configured via `config.yaml` or env var `TARIFICATION_COMMIT_ON_SUBMIT_ENABLED`. The flag gates the entire commit-on-submit flow: when false, `TarifyMessage` charges inline via legacy `saga.Charge` (no `ChargeMessageDual`); when true, `TarifyMessage` is read-only and `CommitCharge` handles dual-charge for subaccounts.

**Decision on B3:**
- CLOSED — TarifyMessage directly calls ChargeMessageDual (or via CommitCharge with flag ON by default and no escape hatch).
- PARTIAL — wired via flag, flag default ON, but escape hatch exists.
- OPEN — dual-charge not wired at all, or flag default OFF.

**Current verdict:** OPEN. `ChargeMessageDual` is only reached when `commit_on_submit_enabled=true`, and the default is **false**. In the default production configuration, subaccount charges go through `saga.Charge()` (single charge on sub-account) inside the legacy `TarifyMessage` branch — the aggregator is not charged atomically via `ChargeMessageDual`. The dual-charge path (`CommitCharge` → `ChargeDualAtomic`) is fully implemented and correct, but is not active by default.

**Impact on Cycle 3 C-billing for subaccount_parent:** Tests must assume `commit_on_submit_enabled=false` unless the test fixture explicitly calls `SetCommitOnSubmitEnabled(true)`; under the default, `ChargeMessageDual` is never invoked and the aggregator balance is not decremented — any assertion about aggregator deduction will fail unless the flag is enabled in the test setup.

## 6. Go/no-go для циклов 1/2/3

### Cycle 1 (Contract: A1, A1-grpc, A2, A3) — **GO**

**Обоснование:**
- R1 закрыт (F1): `cmd/api` не deployed (docker-compose не ссылается), скоуп ревью стоит.
- Latent risk: `internal/api/` получил 22 коммита за 2.5 месяца без деплоя. Formal retirement — рекомендуется отдельным планом, но Cycle 1 не блокирует.
- B1-B3 не влияют на контрактный цикл.

**Предупредительные находки для Cycle 1:**
- **A1-grpc:** `CancelMessage` и `ListScheduledMessages` есть в HTTP, gRPC возвращает `codes.Unimplemented`. Методы `GetBalance`/`GetStatistics` в `server.go` используют чужие proto-типы, нигде не зарегистрированы — мёртвый код.
- **A1/A1-grpc:** Нет `.proto` source files для `messagingv1` — только сгенерированные `.pb.go`. Контракт де-факто нельзя эволюционировать вручную.
- **A2:** Cascade-хендлеры возвращают proto-response напрямую через `respondJSON`, остальные — собранный `inline map`. Консистентность envelope'а отсутствует.
- **A3:** Idempotency-Key middleware не обнаружен по inventory. Проверить Phase 1 Cycle 1.

### Cycle 2 (SMPP + tenant-glue: A6, A7) — **GO с оговоркой**

**Обоснование:**
- R1 закрыт.
- B2 (AuthAdapter kludge) формально OPEN, но это сам предмет Cycle 2 (ось A7). Закрывается внутри цикла, не внешний блокер.

**Критические находки для Cycle 2:**
- **A6-auth (CRITICAL):** SMPP password полностью игнорируется в `AuthenticateBySystemID` — авторизация только по `system_id` как API-ключу. Кто знает валидный system_id — биндится с любым паролем. Severity=critical. Требует отдельного security-fix плана, не inline-фикса.
- **A6-dlr:** `deliver_sm` не различает MO и DLR (`esm_class & 0x04` не проверяется, `receipted_message_id` не читается). DLR, приходящие по SMPP-линку, тихо роутятся как MO.
- **A6-tlv:** **Zero Tag-констант** в `internal/smpp/protocol/`. TLV обрабатываются как `map[uint16][]byte`, из submit_sm никогда не читаются. Массовая дыра контракта.
- **A6-submit-ext:** `submit_multi_sm` и `data_sm` объявлены, но падают в `generic_nack`.
- **A6-ops:** `query_sm`/`cancel_sm`/`replace_sm` — null-guard на `messageRepo`: если БД не подключена, silent failure вместо startup-error.
- **A7:** Нет ни одного metadata-based client_id в топ-5 gRPC. Все через request field.
- **A7:** `cascadev1.ChannelAdminService` и `StrategyAdminService` содержат 12 RPC без client_id. Должны быть защищены gateway-уровнем — проверить.
- **A7:** `tarificationv1` (27 RPC) смешивает hot-path / client-facing / admin-global в одном сервисе. Риск неправильного gateway-gating.
- **A7:** `billingv1.ChargeMessageDual` использует `sub_account_id`/`aggregator_id` вместо `client_id` — корректно для dual-charge, но требует документации.

### Cycle 3 (Subaccount e2e: C) — **NO-GO**

**Блокеры (все OPEN по F2/F3/F4):**

1. **B1** (F2): CI без postgres/redis, `-tags=integration` не запускается, 0 integration-tagged .go файлов в проекте. C-билинг/visibility/DLR-routing требуют реальных БД-проверок.
2. **B2** (F3): 3 точки конфлации `ClientID == UserID` в `auth_adapter.go`. SMPP subaccount не получает реальный client_id. Любой тест C-visibility для SMPP субаккаунта будет ложноположительным.
3. **B3** (F4): Feature flag `tarification.commit_on_submit_enabled = false` по умолчанию. Dual-charge в проде не выполняется. Тесты C-billing/subaccount_parent должны явно включать флаг.

**Требуемые действия до старта Cycle 3:**
- Мини-план B1-fix: добавить postgres+redis service containers в CI, jobs для `-tags=integration`. Или явный downscope Cycle 3 до unit+fakes (потеря coverage).
- Дождаться закрытия B2 в рамках Cycle 2 A7 (фактически там же и правится).
- Решение по B3: либо включить флаг по дефолту (отдельный план), либо зафиксировать в Cycle 3 спеке test setup expectations.

**Status:** NO-GO до закрытия B1 + B2 (через Cycle 2) + явного решения по B3.
