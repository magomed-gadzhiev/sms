# SMPP DLR Delivery — Production-Ready для первого агрегатора

**Дата:** 2026-04-15
**Статус:** Draft
**Scope:** DLR delivery клиентам, fix readPDU, SMSC receipt format

## Контекст

SMPP gateway (`smpp-gateway`) уже принимает соединения агрегаторов, обрабатывает bind/submit_sm/query_sm/cancel_sm/replace_sm и публикует сообщения в Kafka pipeline. Однако **обратная доставка DLR (delivery receipts) клиентам не реализована** — это главный blocker для первого клиента-агрегатора.

### Профиль первого клиента
- Средний агрегатор: до 100 msg/sec, 1-2 SMPP-сессии
- Подключение через VPN/выделенный канал (TLS не требуется)
- Ожидает стандартный SMSC receipt format в deliver_sm

### Что НЕ входит в scope
- TLS (клиент на VPN)
- submit_multi, data_sm
- Sliding window (async pipelining)
- Горизонтальное масштабирование smpp-gateway (один инстанс достаточен)

## Архитектура

```
Kafka [sms.status]
    │
    ▼
dlr-delivery-service (новый)
    │  1. consume StatusUpdate
    │  2. Redis lookup: message_id → session binding
    │  3. format SMSC receipt
    │  4. gRPC → smpp-gateway
    │
    ▼
smpp-gateway (internal gRPC :9095)
    │  DeliverDLR(system_id, receipt)
    │  → lookup session in-memory
    │  → send deliver_sm PDU
    │
    ▼
TCP conn → клиент-агрегатор
```

### Компоненты

1. **smpp-gateway** — при bind записывает session binding в Redis, при submit_sm записывает message маппинг в Redis. Новый internal gRPC endpoint `DeliverDLR` принимает DLR от delivery service и отправляет deliver_sm клиенту через существующую TCP-сессию.

2. **dlr-delivery-service** (новый микросервис) — Kafka consumer на `sms.status`, lookup в Redis, формирование SMSC receipt, gRPC dispatch к smpp-gateway.

3. **Redis** — два маппинга для связки message → session → gateway.

## Redis Schema

### Message маппинг (записывается smpp-gateway при submit_sm)
```
Key:    smpp:msg:{message_id}
Value:  JSON {system_id, source_addr, dest_addr, submit_date}
TTL:    24h
```

### Session маппинг (записывается smpp-gateway при bind)
```
Key:    smpp:session:{system_id}
Value:  JSON {gateway_addr: "smpp-gateway:9095"}
TTL:    300s (обновляется каждым enquire_link)
```

При unbind/disconnect — ключ удаляется явно.

При рестарте smpp-gateway: все ключи `smpp:session:*` станут stale (сессии TCP потеряны). Gateway при старте не чистит чужие ключи — TTL 300s обеспечивает автоматическую очистку. DLR delivery service получит gRPC ошибку при попытке dispatch, залогирует и пропустит — допустимая потеря для MVP.

## DLR Delivery Service — детали

### Kafka consumer
- Topic: `sms.status`
- Consumer group: `dlr-delivery`
- Обрабатывает финальные статусы: `delivered`, `failed`, `expired`, `rejected`
- Игнорирует промежуточные: `sent`, `queued`, `routed`

### Логика обработки

```
1. Получить StatusUpdate из Kafka
2. Redis GET smpp:msg:{message_id}
   → miss → log warning, skip (сообщение не от SMPP клиента)
   → hit  → {system_id, source_addr, dest_addr, submit_date}
3. Redis GET smpp:session:{system_id}
   → miss → клиент отключён, log, skip (DLR потерян — допустимо для MVP)
   → hit  → {gateway_addr}
4. Сформировать SMSC receipt string
5. gRPC → smpp-gateway.DeliverDLR(system_id, source_addr, dest_addr, receipt)
6. ACK Kafka offset
```

### Обработка ошибок
- gRPC unavailable → retry 3 раза с backoff (1s, 2s, 4s), затем skip + log error
- Redis unavailable → не ACK'ать Kafka offset, consumer retry на следующем poll

### Структура файлов
```
cmd/dlr-delivery/main.go                — точка входа
internal/services/dlr/consumer.go       — Kafka consumer loop
internal/services/dlr/formatter.go      — формирование SMSC receipt
internal/services/dlr/dispatcher.go     — gRPC вызов к smpp-gateway
```

## Изменения в smpp-gateway

### 1. Redis маппинг при submit_sm

В `internal/gateway/smpp/server/handler.go:handleSubmitSM` — после успешной публикации в Kafka записать:
```
smpp:msg:{msgID} → {system_id, source_addr, dest_addr, submit_date}  TTL 24h
```

Новая зависимость: `redis/go-redis/v9` клиент передаётся в Server и далее в Handler.

### 2. Redis маппинг при bind/unbind

При успешном bind:
```
smpp:session:{system_id} → {gateway_addr: "smpp-gateway:9095"}  TTL 300s
```
TTL обновляется при каждом enquire_link.

При unbind/disconnect — `DEL smpp:session:{system_id}`.

### 3. Internal gRPC endpoint

Новый proto `api/proto/smppv1/smpp.proto`:

```protobuf
syntax = "proto3";
package smppv1;
option go_package = "github.com/smpp-server/smpp-server/api/proto/smppv1";

service SMPPGateway {
  rpc DeliverDLR(DeliverDLRRequest) returns (DeliverDLRResponse);
}

message DeliverDLRRequest {
  string system_id = 1;
  string source_addr = 2;
  string destination_addr = 3;
  string receipt_text = 4;
}

message DeliverDLRResponse {
  bool delivered = 1;
  string error = 2;
}
```

Gateway слушает internal gRPC на `:9095`. При получении `DeliverDLR`:
1. Найти сессию по `system_id` в in-memory sessions map
2. Проверить что сессия bound (receiver или transceiver)
3. Сформировать deliver_sm PDU: `esm_class=0x04`, receipt в short_message
4. Записать в TCP conn клиента

### 4. Fix readPDU — io.ReadFull

В `internal/gateway/smpp/server/server.go:readPDU` заменить `conn.Read` на `io.ReadFull` для чтения header (16 bytes) и body. Предотвращает потерю данных при TCP fragmentation.

## SMSC Receipt Format

### deliver_sm PDU для DLR
- `esm_class` = `0x04` (delivery receipt)
- `source_addr` = оригинальный destination (получатель SMS)
- `destination_addr` = оригинальный source (отправитель SMS)
- `short_message` = SMSC receipt string

### Receipt template
```
id:{message_id} sub:001 dlvrd:{dlvrd} submit date:{YYMMDDHHmm} done date:{YYMMDDHHmm} stat:{stat} err:{err} text:{first20}
```

### Маппинг статусов

| Pipeline status | SMSC stat  | dlvrd |
|-----------------|------------|-------|
| `delivered`     | `DELIVRD`  | `001` |
| `failed`        | `UNDELIV`  | `000` |
| `expired`       | `EXPIRED`  | `000` |
| `rejected`      | `REJECTD`  | `000` |

### Error code
- `delivered` → `000`
- Остальные → error_code из StatusUpdate (если есть), иначе `000`

### Поле `id`
Тот же message_id что возвращён в submit_sm_resp — ключ корреляции для агрегатора.

## Docker и деплой

### Новый Dockerfile
`deployments/docker/dlr-delivery.Dockerfile` — multi-stage build, аналогичен существующим Go-сервисам.

### docker-compose.yml
Добавить сервис `dlr-delivery`:
- Зависит от: kafka, redis, smpp-gateway
- Переменные: KAFKA_BROKERS, REDIS_ADDR, SMPP_GATEWAY_GRPC_ADDR

Добавить expose порта `:9095` для smpp-gateway (internal gRPC, только docker network).

### Конфигурация

**dlr-delivery:**
- `kafka.brokers`, `kafka.consumer_group: dlr-delivery`
- `redis.addr`
- `smpp_gateway.grpc_addr: smpp-gateway:9095`
- `retry.max_attempts: 3`, `retry.backoff: 1s`

**smpp-gateway (дополнения):**
- `redis.addr`
- `grpc.internal_port: 9095`
- `redis.session_ttl: 300s`
- `redis.message_ttl: 24h`

## Мониторинг

### Prometheus-метрики dlr-delivery
- `dlr_events_consumed_total` (labels: status)
- `dlr_events_dispatched_total` (labels: status, result)
- `dlr_dispatch_duration_seconds` (histogram)
- `dlr_redis_lookup_miss_total` (labels: key_type — msg|session)

### Prometheus-метрики smpp-gateway (дополнения)
- `smpp_dlr_delivered_total` (labels: system_id)
- `smpp_dlr_delivery_failed_total` (labels: system_id, reason)

### Prometheus scrape config
Добавить job `dlr-delivery` в `deployments/configs/prometheus.yml`.
