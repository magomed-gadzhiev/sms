# Batch API Contract: High-Throughput Pipeline

**Feature**: 008-high-throughput-pipeline
**Date**: 2026-03-26
**Status**: Existing (no changes required)

## Overview

Batch API уже полностью реализован. Этот документ фиксирует существующий контракт для reference.

## HTTP Endpoint

### POST /api/v1/sms/batch

**Authentication**: Bearer token (JWT) или API key в заголовке `X-API-Key`

**Request**:
```json
{
  "messages": [
    {
      "source": "+79001234567",
      "destination": "+79009876543",
      "text": "Message 1",
      "provider_id": null,
      "route_id": null,
      "priority": 0,
      "scheduled_at": null,
      "template_id": null,
      "template_params": null
    },
    {
      "source": "+79001234567",
      "destination": "+79001111111",
      "text": "Message 2"
    }
  ]
}
```

**Limits**: До 10 000 сообщений в одном запросе (FR-002)

**Response (200 OK)**:
```json
{
  "results": [
    {
      "message_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "accepted",
      "error": null
    },
    {
      "message_id": null,
      "status": "rejected",
      "error": "invalid destination number"
    }
  ],
  "success_count": 1,
  "failed_count": 1
}
```

**Error Responses**:
- `400 Bad Request`: невалидный JSON, пустой массив messages
- `401 Unauthorized`: невалидный/отсутствующий token
- `429 Too Many Requests`: rate limit exceeded
- `500 Internal Server Error`: внутренняя ошибка

## gRPC Service

### MessagingService.SendBatch

Определён в `api/proto/messaging/messaging.proto`:

```protobuf
rpc SendBatch(SendBatchRequest) returns (SendBatchResponse);

message SendBatchRequest {
  string client_id = 1;
  repeated SendMessageRequest messages = 2;
  google.protobuf.Timestamp scheduled_at = 3;
}

message SendBatchResponse {
  repeated SendMessageResponse results = 1;
  int32 success_count = 2;
  int32 failed_count = 3;
}
```

## Pipeline Optimization (008)

Единственное изменение в рамках 008: gateway handler публикует batch сообщений в Kafka одним вызовом AsyncProducer (batch flush) вместо поштучного SyncProducer.SendMessage(). Контракт API не меняется.

**Before**:
```
for msg in batch:
    SyncProducer.SendMessage(sms.outgoing, msg)  // blocking per msg
```

**After**:
```
AsyncProducer.Input() <- batch_messages  // non-blocking, batch flush
```
