# Contract Changes: tarification.proto

**File**: `api/proto/tarification/tarification.proto`  
**Feature**: 011-operator-sender-billing

## New Messages and RPCs

```protobuf
// ==================== Sender Name Billing ====================

// SenderNameBillingRecordProto представляет запись ежемесячного начисления
message SenderNameBillingRecordProto {
  string id = 1;
  string sender_registration_id = 2;
  string client_id = 3;
  string operator_id = 4;
  string billing_month = 5;      // ISO date "2026-04-01"
  string amount = 6;             // сумма в RUB (строковое представление decimal)
  google.protobuf.Timestamp created_at = 7;
}

// CreateSenderBillingRecordRequest — создать billing record при регистрации платного имени
message CreateSenderBillingRecordRequest {
  string sender_registration_id = 1;
  string client_id = 2;
  string operator_id = 3;
  string amount = 4;             // тариф оператора на момент регистрации
  // billing_month не передаётся — сервис вычисляет сам (текущий месяц, 1-е число)
}

// CreateSenderBillingRecordResponse
message CreateSenderBillingRecordResponse {
  SenderNameBillingRecordProto record = 1;
  bool already_existed = 2;     // true если запись за этот месяц уже была
}

// ListSenderBillingRecordsRequest
message ListSenderBillingRecordsRequest {
  string sender_registration_id = 1;  // фильтр по регистрации (обязателен)
  int32 limit = 2;
  int32 offset = 3;
}

// ListSenderBillingRecordsResponse
message ListSenderBillingRecordsResponse {
  repeated SenderNameBillingRecordProto records = 1;
  int32 total = 2;
}
```

## New RPC Methods (добавить в service TarificationService)

```protobuf
service TarificationService {
  // ... existing methods ...

  // Создать billing record для платного имени отправителя
  rpc CreateSenderBillingRecord(CreateSenderBillingRecordRequest)
      returns (CreateSenderBillingRecordResponse);

  // Получить историю начислений по регистрации
  rpc ListSenderBillingRecords(ListSenderBillingRecordsRequest)
      returns (ListSenderBillingRecordsResponse);
}
```

## HTTP REST Endpoints (portal-gateway)

### GET /api/operators/:id/sender-tariff
Возвращает тариф оператора для отображения inline перед регистрацией платного имени.

**Auth**: JWT (клиент)  
**Response**:
```json
{
  "operator_id": "uuid",
  "monthly_tariff_amount": "1500.00",
  "currency": "RUB",
  "current_month_amount": "1500.00"
}
```

### GET /api/sender-registrations/:id/billing
История начислений по конкретной регистрации.

**Auth**: JWT (клиент)  
**Response**:
```json
{
  "records": [
    {
      "id": "uuid",
      "billing_month": "2026-04-01",
      "amount": "1500.00",
      "created_at": "2026-04-01T00:01:00Z"
    }
  ],
  "total": 1
}
```

## HTTP REST Endpoints (admin-gateway)

### PUT /api/admin/operators/:id
Расширить существующий endpoint — принять `monthly_tariff_amount` в теле запроса.

**Auth**: JWT (admin role)  
**Request body** (дополнение к существующему):
```json
{
  "monthly_tariff_amount": "1500.000000"
}
```

## Backward Compatibility

Новые RPC-методы добавляются в конец service — additive only. Существующие клиенты не затронуты.
