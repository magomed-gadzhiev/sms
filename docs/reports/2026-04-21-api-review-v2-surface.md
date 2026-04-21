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

_TODO: Task 5_

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
