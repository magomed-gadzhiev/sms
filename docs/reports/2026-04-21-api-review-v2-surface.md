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

_TODO: Task 3_

## 3. SMPP (cmd/smpp-gateway)

### 3.1. PDU Commands

_TODO: Task 4_

### 3.2. SMPP TLVs

_TODO: Task 4_

### 3.3. smppv1 control-plane RPC

_TODO: Task 4_

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
