# Data Model: High-Throughput Pipeline Architecture

**Feature**: 008-high-throughput-pipeline
**Date**: 2026-03-26

## Overview

Pipeline-архитектура не требует новых таблиц базы данных. Все существующие таблицы (messages, providers, routes, dlr_receipts) остаются без изменений. Основные изменения — в Kafka-топиках (межстадийные сообщения) и in-memory структурах pipeline-worker.

## Kafka Message Schemas (Inter-Stage)

### Entity: RoutedMessage (sms.outgoing → sms.routed)

Результат работы router stage. Добавляет routing metadata к исходному KafkaMessage.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| message_id | uuid | yes | Уникальный идентификатор сообщения |
| source | string | yes | Адрес отправителя |
| destination | string | yes | Адрес получателя |
| text | string | yes | Текст SMS |
| client_id | uuid | no | ID клиента |
| provider_id | uuid | yes | ID выбранного оператора (primary) |
| fallback_provider_id | uuid | no | ID резервного оператора |
| route_id | uuid | no | ID использованного маршрута |
| priority | int | yes | Приоритет (0=normal, 1=high) |
| retry_count | int | yes | Текущий счётчик повторов |
| max_retries | int | yes | Максимальное количество повторов |
| routed_at | timestamp | yes | Время маршрутизации |
| created_at | timestamp | yes | Время создания сообщения |
| metadata | map[string]any | no | Произвольные метаданные |

**Partition key**: provider_id (hex string)
**Validation**: provider_id must not be empty; destination must be valid E.164

### Entity: SentMessage (sms.routed → sms.sent)

Результат работы sender stage. Фиксирует результат SMPP-отправки.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| message_id | uuid | yes | Уникальный идентификатор сообщения |
| provider_id | uuid | yes | ID оператора, через которого отправлено |
| smpp_message_id | string | yes | Message ID от SMPP-сервера |
| status | string | yes | Статус: "sent", "failed", "retry" |
| error_code | int | no | SMPP error code (если failed) |
| error_message | string | no | Описание ошибки |
| sent_at | timestamp | yes | Время отправки |
| segments_count | int | yes | Количество сегментов SMS |
| connection_id | string | yes | ID SMPP-соединения |

**Partition key**: provider_id (hex string)
**Validation**: status must be one of: sent, failed, retry

### Entity: StatusUpdate (sms.sent + sms.dlr → sms.status)

Объединённый статус для записи в БД. Status writer обрабатывает оба типа событий.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| message_id | uuid | yes | Уникальный идентификатор сообщения |
| status | string | yes | Новый статус сообщения |
| smpp_message_id | string | no | SMPP message ID |
| provider_id | uuid | no | ID оператора |
| error_code | int | no | Код ошибки |
| error_message | string | no | Описание ошибки |
| dlr_stat | string | no | DLR status (DELIVRD, UNDELIV, EXPIRED) |
| submit_date | timestamp | no | DLR submit date |
| done_date | timestamp | no | DLR done date |
| updated_at | timestamp | yes | Время события |

**Partition key**: message_id (hex string) — идемпотентный upsert по message_id
**Validation**: message_id must not be empty; status must be valid enum value

## In-Memory Structures

### BackpressureState (per-operator)

Состояние backpressure для каждого оператора в sender stage.

| Field | Type | Description |
|-------|------|-------------|
| provider_id | uuid | ID оператора |
| tokens_per_second | int | Максимальный throughput (из providers.throughput_per_second) |
| available_tokens | atomic int64 | Текущее количество доступных tokens |
| burst_size | int | Max burst = tokens_per_second × 2 |
| is_throttled | atomic bool | Флаг активного backpressure |
| last_refill | timestamp | Последнее пополнение tokens |
| pending_count | atomic int64 | Количество ожидающих ответа PDU (window) |
| max_window | int | Максимальный window size (default: 50) |

### AsyncConnection (SMPP connection with windowing)

Расширение существующего Connection для async sending.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Уникальный ID соединения |
| provider_id | uuid | ID оператора |
| conn | net.Conn | TCP-соединение |
| bound | bool | Статус bind |
| sequence_num | atomic uint32 | Счётчик sequence number |
| pending_responses | sync.Map[uint32, chan *Response] | Ожидающие ответы по sequence num |
| window_semaphore | chan struct{} | Semaphore для sliding window |
| writer_ch | chan *PDU | Канал для writer горутины |
| created_at | timestamp | Время создания |
| last_used | atomic timestamp | Последнее использование |
| metrics | ConnectionMetrics | Per-connection метрики |

### BatchAccumulator

Аккумулятор сообщений для batch-обработки в каждой стадии.

| Field | Type | Description |
|-------|------|-------------|
| messages | []Message | Буфер сообщений |
| max_size | int | Максимальный размер batch (default: 500) |
| max_wait | duration | Максимальное время ожидания (default: 10ms) |
| timer | *time.Timer | Таймер flush |
| flush_ch | chan []Message | Канал для отправки batch |

## State Transitions

### Message Status (существующий, без изменений)

```
pending → queued → sent → delivered
                      ↘ failed
                      ↘ expired
           ↘ rejected
```

### Pipeline Stage Transitions (новый)

```
[sms.outgoing]
    ↓ Router Stage
[sms.routed] (message + routing metadata)
    ↓ Sender Stage
[sms.sent] (message + send result)
    ↓ Status Writer Stage
[DB: messages table] (status update)

Failover path:
[sms.routed] → Sender Stage (primary fails)
    ↓ retry with fallback_provider_id
[sms.sent] (status=failed, no fallback)
    ↓ re-route via sms.failed → Router Stage
[sms.routed] (new provider)
```

## Kafka Topic Configuration

| Topic | Partitions | Retention | Compression | Replication | Key |
|-------|-----------|-----------|-------------|-------------|-----|
| sms.outgoing | 16 | 24h | snappy | 1 (dev) / 3 (prod) | message_id |
| sms.routed | 16 | 24h | snappy | 1 (dev) / 3 (prod) | provider_id |
| sms.sent | 16 | 24h | snappy | 1 (dev) / 3 (prod) | provider_id |
| sms.status | 8 | 24h | snappy | 1 (dev) / 3 (prod) | message_id |
| sms.dlr | 8 | 24h | snappy | 1 (dev) / 3 (prod) | message_id |
| sms.failed | 4 | 72h | snappy | 1 (dev) / 3 (prod) | message_id |

## Database Changes

**Нет новых таблиц или миграций**. Существующие таблицы messages, providers, routes, dlr_receipts полностью покрывают потребности pipeline. Оптимизации:

- **messages table**: существующие индексы по status, provider_id, created_at достаточны
- **Batch upsert**: status writer использует `INSERT ... ON CONFLICT (id) DO UPDATE` для идемпотентности
- **Connection pooling**: увеличить `MaxOpenConns` до 200 для status writer instances (высокий write throughput)
