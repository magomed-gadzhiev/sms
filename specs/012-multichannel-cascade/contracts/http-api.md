# HTTP API Contracts: Multichannel Cascade Delivery

**Feature**: `012-multichannel-cascade`  
**Gateway prefix**: все маршруты монтируются в соответствующих gateway'ах

---

## Portal Gateway — Admin Endpoints

### Channels

```
GET  /admin/channels
     → 200 [{id, channel_type, name, description, active, created_at, updated_at}]

GET  /admin/channels/{id}
     → 200 {id, channel_type, name, description, active, created_at, updated_at}
     → 404 if not found

POST /admin/channels
     Body: {channel_type, name, description, config}
     → 201 {id, channel_type, name, description, active, created_at, updated_at}
     → 400 if channel_type already exists or invalid

PUT  /admin/channels/{id}
     Body: {name?, description?, config?}
     → 200 updated channel
     → 404 if not found

PUT  /admin/channels/{id}/toggle
     Body: {active: bool}
     → 200 updated channel
     → 404 if not found
```

### Delivery Strategies

```
GET  /admin/delivery-strategies
     Query: ?active_only=true
     → 200 [{id, name, description, mode, active, steps: [{channel_id, channel_type, channel_name, step_order, timeout_s, billable}]}]

GET  /admin/delivery-strategies/{id}
     → 200 strategy with steps
     → 404 if not found

POST /admin/delivery-strategies
     Body: {name, description, mode: "sequential"|"parallel", steps: [{channel_id, step_order, timeout_s, billable}]}
     → 201 created strategy with steps
     → 400 if name exists, invalid mode, or invalid step config

PUT  /admin/delivery-strategies/{id}
     Body: {name?, description?, mode?, steps?}  — steps replaces all existing steps
     → 200 updated strategy
     → 404 if not found
     → 409 if strategy has active deliveries and steps changed

DELETE /admin/delivery-strategies/{id}
     → 204
     → 404 if not found
     → 409 if strategy has active deliveries
```

### Operator Channel Support Matrix

```
GET  /admin/operator-channel-support
     Query: ?operator_id=uuid
     → 200 [{operator_id, operator_name, channel_type, supported, notes, updated_at}]

PUT  /admin/operator-channel-support
     Body: {operator_id, channel_type, supported: bool, notes?}
     → 200 updated entry
     → 404 if operator not found
```

---

## Portal Gateway — Client Endpoints

### Cascade Deliveries

```
POST /cascade/deliveries
     Body: {strategy_id, recipient, text, sender_name?, message_id?}
     → 201 {id, status: "pending", strategy_id, recipient, created_at}
     → 400 if invalid strategy or recipient
     → 402 if insufficient balance

GET  /cascade/deliveries
     Query: ?strategy_id=&status=&date_from=&date_to=&page=1&page_size=20
     → 200 {deliveries: [...], total, page}

GET  /cascade/deliveries/{id}
     → 200 {id, status, delivered_via, total_cost, currency, attempts: [{channel_type, step_order, status, cost, sent_at, result_at}]}
     → 404 if not found or belongs to different client

GET  /cascade/stats
     Query: ?strategy_id=&date_from=&date_to=
     → 200 {total_deliveries, delivered_count, failed_count, delivery_rate, channel_stats, avg_cost, total_cost, currency}
```

---

## Portal Gateway — Webhook Endpoint (Flash Call Provider)

```
POST /webhooks/cascade/flash-call/{attempt_id}
     Body (провайдер-специфично): {status, provider_ref, ...}
     → 200 OK
     → 404 if attempt_id not found
     → 409 if attempt already finalized
```

**Аутентификация**: HMAC-подпись в заголовке `X-Flash-Signature` (provider webhook secret из channel config).

---

## Общие правила

- Все endpoint'ы защищены существующей auth middleware (`portal-gateway`).
- Admin endpoints требуют role `admin`.
- Client endpoints ограничены данными `client_id` из JWT.
- Все ответы с ошибками: `{"error": "...", "code": "..."}`.
- `X-Request-ID` header propagated через все gRPC вызовы.
