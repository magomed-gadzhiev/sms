# Contract Changes: routing.proto

**File**: `api/proto/routing/routing.proto`  
**Feature**: 011-operator-sender-billing

## Changes

### 1. Message `Operator` — добавить поле monthly_tariff_amount

```protobuf
message Operator {
  string id = 1;
  string country_id = 2;
  string name = 3;
  string code = 4;
  bool supports_paid_sender = 5;
  bool supports_free_sender = 6;
  bool active = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
  // NEW: ежемесячный тариф в RUB; пустая строка = не задан (free-only оператор)
  string monthly_tariff_amount = 10;
}
```

### 2. Message `CreateOperatorRequest` — добавить поле

```protobuf
message CreateOperatorRequest {
  string country_id = 1;
  string name = 2;
  string code = 3;
  bool supports_paid_sender = 4;
  bool supports_free_sender = 5;
  // NEW:
  string monthly_tariff_amount = 6; // "0" или "" = не задан
}
```

### 3. Message `UpdateOperatorRequest` — добавить поле

```protobuf
message UpdateOperatorRequest {
  string id = 1;
  string name = 2;
  string code = 3;
  bool supports_paid_sender = 4;
  bool supports_free_sender = 5;
  bool active = 6;
  // NEW:
  string monthly_tariff_amount = 7; // "0" или "" = сбросить
}
```

## Backward Compatibility

- Новые поля добавляются с наибольшим номером (10, 6, 7) — additive only, бинарная совместимость сохраняется
- Старые клиенты, не передающие `monthly_tariff_amount`, получат пустую строку — интерпретируется как NULL/0

## Regeneration

После изменения proto:
```bash
cd api/proto/routing
protoc --go_out=../routingv1 --go_opt=paths=source_relative \
       --go-grpc_out=../routingv1 --go-grpc_opt=paths=source_relative \
       routing.proto
```
