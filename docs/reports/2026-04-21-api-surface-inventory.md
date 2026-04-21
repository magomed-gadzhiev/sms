# API Surface Inventory — 2026-04-21

**Дата снапшота:** 2026-04-21
**Источник:** `internal/gateway/client/router/router.go`, `internal/gateway/smpp/server/handler.go`, `internal/smpp/server/handler.go`, `internal/smpp/protocol/constants.go`
**Ревизия:** commit <ХЭШ> (заполнить в конце Task 7)

## HTTP Endpoints (client-gateway)

| Method | Path | Handler file:line | Service call | DB tables written | Response shape |
|---|---|---|---|---|---|
| GET | /health | `internal/monitoring/health.go` (healthChecker.Handler) | local | — | n/a (health) |
| GET | /health/live | `internal/monitoring/health.go` (healthChecker.LivenessHandler) | local | — | n/a (health) |
| GET | /health/ready | `internal/monitoring/health.go` (healthChecker.ReadinessHandler) | local | — | n/a (health) |
| POST | /api/v1/sms/send | `internal/gateway/client/handlers/sms.go:57` | `h.messagingClient.SendMessage` | via `messagingv1.SendMessage` | inline map: message_id, status, segment_count, created_at |
| POST | /api/v1/sms/batch | `internal/gateway/client/handlers/sms.go:173` | `h.messagingClient.SendBatch` | via `messagingv1.SendBatch` | inline map: results[], success_count, failed_count |
| GET | /api/v1/sms/status/{id} | `internal/gateway/client/handlers/sms.go:283` | `h.messagingClient.GetMessageStatus` | via `messagingv1.GetMessageStatus` | inline map: message_id, status, status_message, timestamps |
| GET | /api/v1/sms/history | `internal/gateway/client/handlers/sms.go:346` | `h.messagingClient.GetMessageHistory` | via `messagingv1.GetMessageHistory` | inline map: messages[], total, limit, offset |
| GET | /api/v1/sms/scheduled | `internal/gateway/client/handlers/sms.go:457` | `h.messagingClient.ListScheduledMessages` | via `messagingv1.ListScheduledMessages` | inline map: messages ([]messageItem), total, limit, offset |
| DELETE | /api/v1/sms/{id} | `internal/gateway/client/handlers/sms.go:526` | `h.messagingClient.CancelMessage` | via `messagingv1.CancelMessage` | 204 No Content |
| GET | /api/v1/account/balance | `internal/gateway/client/handlers/account.go:34` | `h.billingClient.GetBalance` | via `billingv1.GetBalance` | inline map: client_id, balance, currency, updated_at |
| GET | /api/v1/account/stats | `internal/gateway/client/handlers/account.go:66` | `h.analyticsClient.GetStatistics` | via `analyticsv1.GetStatistics` | inline map: client_id, from, to, group_by, groups[], totals |
| POST | /api/v1/webhooks | `internal/gateway/client/handlers/webhooks.go:24` | `h.webhookClient.CreateSubscription` | via `webhookv1.CreateSubscription` | inline map: id, client_id, url, event_types, active, secret |
| GET | /api/v1/webhooks | `internal/gateway/client/handlers/webhooks.go:73` | `h.webhookClient.ListSubscriptions` | via `webhookv1.ListSubscriptions` | inline map: subscriptions[] |
| GET | /api/v1/webhooks/{id} | `internal/gateway/client/handlers/webhooks.go:95` | `h.webhookClient.GetSubscription` | via `webhookv1.GetSubscription` | inline map (subscriptionToMap) |
| PUT | /api/v1/webhooks/{id} | `internal/gateway/client/handlers/webhooks.go:115` | `h.webhookClient.UpdateSubscription` | via `webhookv1.UpdateSubscription` | inline map (subscriptionToMap) |
| DELETE | /api/v1/webhooks/{id} | `internal/gateway/client/handlers/webhooks.go:152` | `h.webhookClient.DeleteSubscription` | via `webhookv1.DeleteSubscription` | 204 No Content |
| POST | /api/v1/lookup | `internal/gateway/client/handlers/lookup.go:41` | `h.routingClient.NumberLookup` | via `routingv1.NumberLookup` | inline map: msisdn, operator_mccmnc, number_status, country_code, … |
| POST | /api/v1/lookup/bulk | `internal/gateway/client/handlers/lookup.go:100` | `h.routingClient.BulkNumberLookup` | via `routingv1.BulkNumberLookup` | inline map: results[], total_count, success_count, failed_count |
| GET | /api/v1/lookup/history | `internal/gateway/client/handlers/lookup.go:173` | `h.routingClient.GetLookupHistory` | via `routingv1.GetLookupHistory` | inline map: items[], total_count, page, page_size |
| POST | /api/v1/templates | `internal/gateway/client/handlers/templates.go:25` | `h.templateClient.CreateTemplate` | via `templatev1.CreateTemplate` | inline map (templateToMap) |
| GET | /api/v1/templates | `internal/gateway/client/handlers/templates.go:63` | `h.templateClient.ListTemplates` | via `templatev1.ListTemplates` | inline map: templates[], total, limit, offset |
| GET | /api/v1/templates/{id} | `internal/gateway/client/handlers/templates.go:109` | `h.templateClient.GetTemplate` | via `templatev1.GetTemplate` | inline map (templateToMap) |
| PUT | /api/v1/templates/{id} | `internal/gateway/client/handlers/templates.go:129` | `h.templateClient.UpdateTemplate` | via `templatev1.UpdateTemplate` | inline map (templateToMap) |
| DELETE | /api/v1/templates/{id} | `internal/gateway/client/handlers/templates.go:163` | `h.templateClient.DeleteTemplate` | via `templatev1.DeleteTemplate` | 204 No Content |
| GET | /api/v1/templates/{id}/audit | `internal/gateway/client/handlers/templates.go:183` | `h.templateClient.GetTemplateAuditLog` | via `templatev1.GetTemplateAuditLog` | inline map: entries[], total |
| GET | /docs | n/a (docs) | n/a (docs) | — | n/a (docs) |
| GET | /docs/swagger | n/a (docs) | n/a (docs) | — | n/a (docs) |
| GET | /docs/grpc | n/a (docs) | n/a (docs) | — | n/a (docs) |
| GET | /docs/openapi.yaml | n/a (docs) | n/a (docs) | — | n/a (docs) |
| POST | /api/v1/cascade/deliveries | `internal/gateway/client/handlers/cascade.go:35` | `h.cascadeClient.CreateDelivery` | via `cascadev1.CreateDelivery` | `*cascadev1.CreateDeliveryResponse` (proto, as-is) |
| GET | /api/v1/cascade/deliveries | `internal/gateway/client/handlers/cascade.go:87` | `h.cascadeClient.ListDeliveries` | via `cascadev1.ListDeliveries` | `*cascadev1.ListDeliveriesResponse` (proto, as-is) |
| GET | /api/v1/cascade/deliveries/{id} | `internal/gateway/client/handlers/cascade.go:66` | `h.cascadeClient.GetDelivery` | via `cascadev1.GetDelivery` | `*cascadev1.GetDeliveryResponse` (proto, as-is) |
| GET | /api/v1/cascade/stats | `internal/gateway/client/handlers/cascade.go:120` | `h.cascadeClient.GetDeliveryStats` | via `cascadev1.GetDeliveryStats` | `*cascadev1.GetDeliveryStatsResponse` (proto, as-is) |

## SMPP Commands

| PDU / Event | Layer | Handler file:line | Service call | DB tables written | Notes |
|---|---|---|---|---|---|
| bind_receiver | protocol | `internal/smpp/server/handler.go:103` | `clientRepo.GetByAPIKey` (local DB) | — (read-only auth check) | Auth: system_id matched against clients table via API key |
| bind_receiver | gateway | `internal/gateway/smpp/server/handler.go:113` | `authAdapter.AuthenticateBySystemID` (Auth Service) | Redis: saves session binding key | Auth delegated to Auth Service via AuthAdapter; session binding persisted in Redis |
| bind_transmitter | protocol | `internal/smpp/server/handler.go:143` | `clientRepo.GetByAPIKey` (local DB) | — (read-only auth check) | Same auth path as bind_receiver at protocol layer |
| bind_transmitter | gateway | `internal/gateway/smpp/server/handler.go:161` | `authAdapter.AuthenticateBySystemID` (Auth Service) | Redis: saves session binding key | Same as bind_receiver at gateway layer |
| bind_transceiver | protocol | `internal/smpp/server/handler.go:183` | `clientRepo.GetByAPIKey` (local DB) | — (read-only auth check) | Same auth path as bind_receiver at protocol layer |
| bind_transceiver | gateway | `internal/gateway/smpp/server/handler.go:209` | `authAdapter.AuthenticateBySystemID` (Auth Service) | Redis: saves session binding key | Same as bind_receiver at gateway layer |
| unbind | protocol | `internal/smpp/server/handler.go:223` | — (local session state) | — | Calls `session.Unbind()` only |
| unbind | gateway | `internal/gateway/smpp/server/handler.go:257` | — (local session state) | Redis: deletes session binding key | Calls `session.Unbind()` + removes Redis session key |
| submit_sm | protocol | `internal/smpp/server/handler.go:237` | Kafka `producer.PublishOutgoing` (outgoing topic) | — (Kafka publish only) | UDH not parsed; rate-limit enforced; message ID generated client-side |
| submit_sm | gateway | `internal/gateway/smpp/server/handler.go:277` | Kafka `producer.PublishOutgoing` (outgoing topic) | Redis: saves message→session mapping for DLR routing | UDH parsed (ESM_CLASS bit 0x40); user_id added to Kafka metadata; Redis mapping saved for DLR delivery |
| deliver_sm | gateway | `internal/gateway/smpp/server/handler.go:547` | `optOutRepo.Add` (conditional) | `opt_out_list` (if stop-keyword matched) | Not present in protocol layer. No MO/DLR split — handler treats all deliver_sm as MO only; no esm_class/receipted_message_id check. Stop-keywords: STOP, СТОП, ОТПИСАТЬСЯ, UNSUBSCRIBE |
| enquire_link | protocol | `internal/smpp/server/handler.go:325` | — (local session state) | — | Updates session.EnquireLinkSent timestamp |
| enquire_link | gateway | `internal/gateway/smpp/server/handler.go:401` | — (local session state) | Redis: refreshes session binding TTL | Calls `redisStore.RefreshSessionTTL` on each keepalive |
| query_sm | protocol | `internal/smpp/server/handler.go:338` | `messageRepo.GetByMessageID` (local DB) | — (read-only) | Returns message state mapped to SMPP MSG_STATE_* constants |
| query_sm | gateway | `internal/gateway/smpp/server/handler.go:418` | `messageRepo.GetByMessageID` (local DB) | — (read-only) | Identical logic to protocol layer; uses `gwMapMessageStatusToSMPP` |
| cancel_sm | protocol | `internal/smpp/server/handler.go:385` | `messageRepo.UpdateStatusByMessageID` (local DB) | `messages` (status → cancelled) | Only updates status; no Kafka event emitted |
| cancel_sm | gateway | `internal/gateway/smpp/server/handler.go:465` | `messageRepo.UpdateStatusByMessageID` (local DB) | `messages` (status → cancelled) | Identical logic to protocol layer |
| replace_sm | protocol | `internal/smpp/server/handler.go:418` | `messageRepo.UpdateTextByMessageID` (local DB) | `messages` (text field) | Allowed only for pending/queued status; no Kafka event |
| replace_sm | gateway | `internal/gateway/smpp/server/handler.go:498` | `messageRepo.UpdateTextByMessageID` (local DB) | `messages` (text field) | Identical logic to protocol layer |

## SMPP Supported TLVs

| Tag (hex) | Symbolic name | Used in (submit/deliver) | Mapped to field |
|---|---|---|---|
| _TODO: заполнить в Task 4_ | | | |

## Auth entry points

| Transport | Method | Source of identity | File:line |
|---|---|---|---|
| _TODO: заполнить в Task 5_ | | | |
