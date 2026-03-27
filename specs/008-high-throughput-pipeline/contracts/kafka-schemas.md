# Kafka Message Schemas: High-Throughput Pipeline

**Feature**: 008-high-throughput-pipeline
**Date**: 2026-03-26
**Version**: 1.0

## Overview

Inter-stage Kafka message schemas для pipeline-архитектуры. Все схемы используют JSON serialization (совместимость с существующими KafkaMessage/DLRMessage). Schema versioning через поле `schema_version`.

## Schema: RoutedMessage (Topic: sms.routed)

**Producer**: Router Stage
**Consumer**: Sender Stage
**Partition Key**: provider_id (hex UUID string)

```json
{
  "schema_version": 1,
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "source": "+79001234567",
  "destination": "+79009876543",
  "text": "Hello, world!",
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "provider_id": "770e8400-e29b-41d4-a716-446655440002",
  "fallback_provider_id": "880e8400-e29b-41d4-a716-446655440003",
  "route_id": "990e8400-e29b-41d4-a716-446655440004",
  "priority": 0,
  "retry_count": 0,
  "max_retries": 3,
  "routed_at": "2026-03-26T12:00:00.123Z",
  "created_at": "2026-03-26T11:59:59.999Z",
  "metadata": {
    "template_id": "welcome",
    "campaign_id": "spring2026"
  }
}
```

**Backward compatibility**: RoutedMessage — расширение существующего KafkaMessage с добавлением `fallback_provider_id`, `routed_at`, `schema_version`. Consumers MUST игнорировать неизвестные поля.

## Schema: SentMessage (Topic: sms.sent)

**Producer**: Sender Stage
**Consumer**: Status Writer Stage
**Partition Key**: provider_id (hex UUID string)

```json
{
  "schema_version": 1,
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "provider_id": "770e8400-e29b-41d4-a716-446655440002",
  "smpp_message_id": "12345678",
  "status": "sent",
  "error_code": null,
  "error_message": null,
  "sent_at": "2026-03-26T12:00:00.456Z",
  "segments_count": 1,
  "connection_id": "conn-provider1-0"
}
```

**Status values**: `sent` | `failed` | `retry`

## Schema: StatusUpdate (Topic: sms.status)

**Producer**: Status Writer Stage (aggregated from sms.sent + sms.dlr)
**Consumer**: Internal (status writer writes directly to DB, topic for audit/replay)
**Partition Key**: message_id (hex UUID string)

```json
{
  "schema_version": 1,
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "delivered",
  "smpp_message_id": "12345678",
  "provider_id": "770e8400-e29b-41d4-a716-446655440002",
  "error_code": null,
  "error_message": null,
  "dlr_stat": "DELIVRD",
  "submit_date": "2026-03-26T12:00:00.456Z",
  "done_date": "2026-03-26T12:00:01.789Z",
  "updated_at": "2026-03-26T12:00:02.000Z"
}
```

## Schema: Existing KafkaMessage (Topic: sms.outgoing — unchanged)

```json
{
  "id": "msg-001",
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "source": "+79001234567",
  "destination": "+79009876543",
  "text": "Hello, world!",
  "provider_id": null,
  "route_id": null,
  "client_id": "660e8400-e29b-41d4-a716-446655440001",
  "priority": 0,
  "retry_count": 0,
  "max_retries": 3,
  "created_at": "2026-03-26T11:59:59.999Z",
  "metadata": {}
}
```

## Schema: Existing DLRMessage (Topic: sms.dlr — unchanged)

```json
{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "smpp_message_id": "12345678",
  "provider_id": "770e8400-e29b-41d4-a716-446655440002",
  "receipted_message_id": "12345678",
  "submit_date": "2026-03-26T12:00:00.456Z",
  "done_date": "2026-03-26T12:00:01.789Z",
  "stat": "DELIVRD",
  "err": null,
  "text": "",
  "created_at": "2026-03-26T12:00:02.000Z"
}
```

## Versioning Policy

- Поле `schema_version` присутствует во всех новых схемах
- При добавлении новых полей: version НЕ увеличивается (backward compatible)
- При изменении семантики существующих полей: version увеличивается
- Consumers MUST обрабатывать все версии ≤ current
- Consumers MUST игнорировать неизвестные поля
