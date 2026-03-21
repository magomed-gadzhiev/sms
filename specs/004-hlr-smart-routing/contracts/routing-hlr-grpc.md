# gRPC Contract: Routing Service — HLR Extension

**Proto file**: `api/proto/routing/routing.proto` (расширение существующего)

## New RPC Methods

### NumberLookup

Выполняет HLR-lookup для одного номера. Возвращает информацию об операторе и статусе.

```
rpc NumberLookup(NumberLookupRequest) returns (NumberLookupResponse);
```

**Request**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| msisdn | string | yes | Номер телефона в международном формате (E.164) |
| client_id | string | yes | ID клиента (для billing и audit) |
| force_refresh | bool | no | Принудительный HLR-запрос (bypass cache) |
| request_id | string | yes | ID запроса для tracing |

**Response**:
| Field | Type | Description |
|-------|------|-------------|
| msisdn | string | Запрошенный номер |
| operator_mccmnc | string | MCC+MNC реального оператора |
| operator_name | string | Название оператора |
| number_status | NumberStatus | active / absent / invalid / unknown |
| country_code | string | ISO-код страны |
| number_type | NumberType | mobile / fixed / voip |
| is_ported | bool | Номер был перенесён (MNP) |
| original_operator_mccmnc | string | Оригинальный оператор до MNP (если is_ported=true) |
| cached | bool | Результат из кеша |
| queried_at | google.protobuf.Timestamp | Время выполнения HLR-запроса |

**Enums**:
```
enum NumberStatus {
  NUMBER_STATUS_UNSPECIFIED = 0;
  NUMBER_STATUS_ACTIVE = 1;
  NUMBER_STATUS_ABSENT = 2;
  NUMBER_STATUS_INVALID = 3;
  NUMBER_STATUS_UNKNOWN = 4;
}

enum NumberType {
  NUMBER_TYPE_UNSPECIFIED = 0;
  NUMBER_TYPE_MOBILE = 1;
  NUMBER_TYPE_FIXED = 2;
  NUMBER_TYPE_VOIP = 3;
}
```

---

### BulkNumberLookup

Пакетный lookup до 1000 номеров.

```
rpc BulkNumberLookup(BulkNumberLookupRequest) returns (BulkNumberLookupResponse);
```

**Request**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| msisdns | repeated string | yes | Номера (max 1000) |
| client_id | string | yes | ID клиента |
| force_refresh | bool | no | Bypass cache для всех номеров |
| request_id | string | yes | ID запроса |

**Response**:
| Field | Type | Description |
|-------|------|-------------|
| results | repeated NumberLookupResponse | Результаты для каждого номера |
| total_count | int32 | Общее количество номеров |
| success_count | int32 | Успешно обработанных |
| failed_count | int32 | Ошибки (невалидный формат и т.д.) |

---

### HLR Provider Management (admin)

```
rpc CreateHLRProvider(CreateHLRProviderRequest) returns (HLRProvider);
rpc UpdateHLRProvider(UpdateHLRProviderRequest) returns (HLRProvider);
rpc DeleteHLRProvider(DeleteHLRProviderRequest) returns (google.protobuf.Empty);
rpc GetHLRProvider(GetHLRProviderRequest) returns (HLRProvider);
rpc ListHLRProviders(ListHLRProvidersRequest) returns (ListHLRProvidersResponse);
```

---

### Smart Route Weight Management (admin)

```
rpc SetSmartRouteWeights(SetSmartRouteWeightsRequest) returns (SmartRouteWeight);
rpc GetSmartRouteWeights(GetSmartRouteWeightsRequest) returns (SmartRouteWeight);
rpc ListSmartRouteWeights(ListSmartRouteWeightsRequest) returns (ListSmartRouteWeightsResponse);
rpc DeleteSmartRouteWeights(DeleteSmartRouteWeightsRequest) returns (google.protobuf.Empty);
```

---

### HLR-Enriched Route Selection (internal)

Расширение существующего `RouteMessage`:

```
rpc RouteMessage(RouteMessageRequest) returns (RouteMessageResponse);
```

Добавляемые поля в **RouteMessageResponse**:
| Field | Type | Description |
|-------|------|-------------|
| hlr_result | NumberLookupResponse | Результат HLR-lookup (если выполнен) |
| hlr_used | bool | Использовался ли HLR для маршрутизации |
| routing_score | double | Score выбранного провайдера |

---

### Lookup Log Query (portal/admin)

```
rpc GetLookupHistory(GetLookupHistoryRequest) returns (GetLookupHistoryResponse);
```

**Request**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| client_id | string | yes | ID клиента |
| from_date | google.protobuf.Timestamp | no | Начало периода |
| to_date | google.protobuf.Timestamp | no | Конец периода |
| msisdn_filter | string | no | Фильтр по номеру |
| source_filter | string | no | sms_routing / api_lookup |
| page | int32 | no | Страница (default 1) |
| page_size | int32 | no | Размер страницы (default 50, max 100) |

**Response**:
| Field | Type | Description |
|-------|------|-------------|
| items | repeated LookupLogEntry | Записи лога |
| total_count | int64 | Общее количество записей |
| page | int32 | Текущая страница |
| page_size | int32 | Размер страницы |
