# Quickstart: Multichannel Cascade Delivery

**Feature**: `012-multichannel-cascade`

---

## Что реализуется

Механизм каскадной доставки сообщений через несколько каналов (SMS, flash call, reverse call,
мессенджеры) с event-driven оркестрацией через Kafka.

---

## Компоненты

| Компонент | Расположение | Назначение |
|-----------|-------------|-----------|
| `cascade-service` | `cmd/services/cascade-service/` | Оркестратор: state machine, channel adapters, scheduler |
| Portal handlers | `internal/gateway/portal/handlers/cascade.go` | Admin: каналы, стратегии, OCS матрица |
| Client handlers | `internal/gateway/client/handlers/cascade.go` | Client: отправка, история, статистика |
| Webhook handler | `internal/gateway/portal/handlers/cascade_webhook.go` | Flash call webhook endpoint |
| Frontend admin | `portal-frontend/src/pages/channels/` | Управление каналами |
| Frontend admin | `portal-frontend/src/pages/delivery-strategies/` | Управление стратегиями |
| Frontend client | `portal-frontend/src/pages/cascade-history/` | История доставок |

---

## Зависимости (существующие сервисы)

- **routing-service**: gRPC → HLR lookup для reachability check
- **tarification-service**: gRPC → тарификация попыток
- **billing-service**: gRPC → баланс, рефанды
- **Kafka**: топики `cascade.*` (новые), существующие `sms.*` для SMS-канала

---

## Запуск локально

```bash
# Применить миграции (000067–000070)
scripts/server.sh migrate

# Запустить cascade-service
cd cmd/services/cascade-service
go run main.go

# Или через Docker Compose (после добавления в deployments/docker-compose.yml)
docker-compose up cascade-service
```

---

## Пример создания каскада через API

```bash
# 1. Создать стратегию (admin)
curl -X POST /admin/delivery-strategies \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "flash-call-then-sms",
    "mode": "sequential",
    "steps": [
      {"channel_id": "<flash-call-uuid>", "step_order": 1, "timeout_s": 15, "billable": false},
      {"channel_id": "<sms-uuid>",        "step_order": 2, "timeout_s": 60, "billable": true}
    ]
  }'

# 2. Отправить через каскад (client)
curl -X POST /cascade/deliveries \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  -d '{
    "strategy_id": "<strategy-uuid>",
    "recipient": "+79001234567",
    "text": "Your code: 1234"
  }'

# 3. Проверить статус
curl /cascade/deliveries/<delivery-id> \
  -H "Authorization: Bearer $CLIENT_TOKEN"
```

---

## Kafka-топики (новые)

Добавить в `configs/config.example.yaml`:

```yaml
kafka:
  topic_cascade_start:          "cascade.start"
  topic_cascade_attempt_send:   "cascade.attempt.send"
  topic_cascade_attempt_result: "cascade.attempt.result"
  topic_cascade_billing:        "cascade.billing"
```

---

## Переменные окружения (cascade-service)

```
CASCADE_SERVICE_ADDR=:9110
ROUTING_SERVICE_ADDR=routing-service:9103
TARIFICATION_SERVICE_ADDR=tarification-service:9100
BILLING_SERVICE_ADDR=billing-service:9097
FLASH_CALL_PROVIDER_URL=https://provider.example.com/api
FLASH_CALL_WEBHOOK_SECRET=<secret>
```

---

## Тестирование

```bash
# Unit тесты
go test ./internal/services/cascade/...

# Functional тесты (с реальными зависимостями)
go test -tags=functional ./test/cascade/...

# Integration тесты
go test -tags=integration ./tests/cascade/...
```
