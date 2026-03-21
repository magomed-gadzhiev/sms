# Data Model: Number Lookup (HLR/MNP) + Smart Routing

**Feature**: 004-hlr-smart-routing | **Date**: 2026-03-21

## Entities

### HLRProvider

Конфигурация внешнего HLR-провайдера для выполнения number lookup запросов.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Уникальный идентификатор |
| name | VARCHAR(100) | UNIQUE, NOT NULL | Название провайдера |
| adapter_type | VARCHAR(50) | NOT NULL | Тип адаптера (http_rest, diameter, ss7) |
| config | JSONB | NOT NULL | Конфигурация подключения (URL, credentials, timeouts) |
| priority | INTEGER | NOT NULL, DEFAULT 1 | Приоритет для failover (1 = highest) |
| supported_regions | TEXT[] | NOT NULL | Список ISO-кодов стран, поддерживаемых провайдером |
| cost_per_lookup | NUMERIC(20,6) | NOT NULL | Стоимость одного HLR-запроса для расчёта маржи |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'healthy' | healthy / degraded / unhealthy / disabled |
| success_rate | NUMERIC(5,2) | DEFAULT 100.00 | Текущий % успешных запросов |
| last_success_at | TIMESTAMPTZ | | Время последнего успешного запроса |
| last_failure_at | TIMESTAMPTZ | | Время последней ошибки |
| active | BOOLEAN | NOT NULL, DEFAULT true | Включён/выключен |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |

**Indexes**: `idx_hlr_providers_active` (active, priority), `idx_hlr_providers_status` (status)

---

### SmartRouteWeight

Настраиваемые веса для взвешенного выбора провайдера по оператору/региону.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Уникальный идентификатор |
| operator_code | VARCHAR(20) | | Код оператора (NULL = default для региона) |
| country_code | VARCHAR(3) | NOT NULL | ISO-код страны |
| cost_weight | NUMERIC(3,2) | NOT NULL, DEFAULT 0.60 | Вес стоимости (0.00-1.00) |
| quality_weight | NUMERIC(3,2) | NOT NULL, DEFAULT 0.40 | Вес качества доставки (0.00-1.00) |
| active | BOOLEAN | NOT NULL, DEFAULT true | |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | |

**Constraints**: `CHECK (cost_weight + quality_weight = 1.00)`, UNIQUE (operator_code, country_code)

**Indexes**: `idx_smart_route_weights_lookup` (country_code, operator_code, active)

---

### LookupLog (PARTITIONED by month)

Аудит-лог всех HLR-запросов. Retention: 90 дней.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Уникальный идентификатор |
| msisdn | VARCHAR(20) | NOT NULL | Запрошенный номер телефона (PII) |
| operator_mccmnc | VARCHAR(10) | | Определённый оператор (MCC+MNC) |
| operator_name | VARCHAR(100) | | Название оператора |
| number_status | VARCHAR(20) | NOT NULL | active / absent / invalid / unknown |
| country_code | VARCHAR(3) | | ISO-код страны |
| number_type | VARCHAR(10) | | mobile / fixed / voip |
| is_ported | BOOLEAN | | Номер перенесён (MNP) |
| hlr_provider_id | UUID | FK → hlr_providers.id | Провайдер, выполнивший запрос (NULL если из кеша) |
| source | VARCHAR(20) | NOT NULL | sms_routing / api_lookup |
| client_id | UUID | NOT NULL | Клиент, инициировавший запрос |
| cached | BOOLEAN | NOT NULL, DEFAULT false | Результат из кеша или свежий HLR-запрос |
| latency_ms | INTEGER | | Время выполнения запроса в мс |
| request_id | VARCHAR(100) | NOT NULL | ID запроса для tracing |
| message_id | UUID | | Связанное сообщение (если source=sms_routing) |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Partition key |

**Partitioning**: RANGE by created_at (monthly, аналогично messages и audit_log)

**Indexes**: `idx_lookup_log_client` (client_id, created_at), `idx_lookup_log_msisdn` (msisdn, created_at), `idx_lookup_log_request` (request_id)

---

### Redis Cache: HLR Lookup Result

**Key pattern**: `hlr:{msisdn}` (e.g., `hlr:79001234567`)

**Value** (JSON):
```json
{
  "msisdn": "79001234567",
  "operator_mccmnc": "25001",
  "operator_name": "MTS",
  "number_status": "active",
  "country_code": "RU",
  "number_type": "mobile",
  "is_ported": true,
  "original_operator_mccmnc": "25002",
  "hlr_provider_id": "uuid",
  "queried_at": "2026-03-21T10:00:00Z"
}
```

**TTL**: 24 часа (86400 секунд), настраиваемый через конфигурацию.

**Invalidation**: По ключу при получении delivery failure с reason `wrong_operator`. Команда `DEL hlr:{msisdn}`.

---

## Entity Relationships

```
HLRProvider (1) ←── (N) LookupLog          # Провайдер, выполнивший lookup
SmartRouteWeight (N) ──→ (1) Country        # Веса привязаны к стране (existing entity)
SmartRouteWeight (N) ──→ (1) Operator       # Опционально — к конкретному оператору (existing entity)
LookupLog (N) ──→ (1) Client                # Клиент, запросивший lookup (existing entity)
LookupLog (N) ──→ (1) Message               # Связанное SMS (existing entity, nullable)
```

## State Transitions

### NumberStatus (value object, не persisted как state machine)

```
HLR Query → active    (номер активен, абонент доступен)
HLR Query → absent    (номер существует, абонент временно недоступен)
HLR Query → invalid   (номер не существует / деактивирован)
HLR Query → unknown   (HLR не смог определить статус)
```

### HLRProvider.status

```
healthy → degraded    (success_rate drops below 95%)
healthy → unhealthy   (success_rate drops below 80% OR timeout)
degraded → healthy    (success_rate recovers above 95%)
degraded → unhealthy  (success_rate drops below 80%)
unhealthy → degraded  (success_rate recovers above 80%)
unhealthy → healthy   (success_rate recovers above 95%)
* → disabled          (admin manually disables)
disabled → healthy    (admin manually enables, resets metrics)
```
