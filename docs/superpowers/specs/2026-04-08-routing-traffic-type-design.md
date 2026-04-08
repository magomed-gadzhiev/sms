# Routing Overhaul: Traffic Type, Conditions, Schedules & UX

**Date:** 2026-04-08
**Status:** Approved

## Overview

Полная переработка системы маршрутизации SMS-платформы. Добавляем тип трафика в шаблоны, сложные условия маршрутов с логическими группами, расписания, и полностью заменяем UI маршрутизации.

## Decisions

| Вопрос | Решение |
|--------|---------|
| Scope | Полный UX из макетов (условия, расписание, regex, логические группы) |
| Traffic type | Явное поле `traffic_type` в шаблоне (authorization / transactional / service) |
| Default routes | Единая таблица с `client_id = NULL` для дефолтных маршрутов |
| Matching | Гибрид: нормализованная БД + in-memory matcher + Redis cache |
| Schedules | Отдельная таблица `route_schedules` (несколько расписаний на маршрут) |
| Conditions storage | Нормализованные таблицы (groups + conditions) |
| UI | Полная замена RoutingPage |

## 1. Schema

### 1.1 Templates — traffic_type

```sql
ALTER TABLE templates ADD COLUMN traffic_type VARCHAR(20) NOT NULL DEFAULT 'transactional';
-- Values: 'authorization', 'transactional', 'service'
```

### 1.2 Routes — расширение client_routes

```sql
ALTER TABLE client_routes
  ADD COLUMN name VARCHAR(255),
  ADD COLUMN comment TEXT,
  ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active',  -- active, draft
  ADD COLUMN share INTEGER NOT NULL DEFAULT 100,            -- доля %
  ADD COLUMN route_type VARCHAR(10) NOT NULL DEFAULT 'sms', -- sms, hlr, max
  ALTER COLUMN client_id DROP NOT NULL;                     -- NULL = default route
```

### 1.3 route_condition_groups

```sql
CREATE TABLE route_condition_groups (
  id          BIGSERIAL PRIMARY KEY,
  route_id    BIGINT NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  group_index SMALLINT NOT NULL,
  logic_op    VARCHAR(10) NOT NULL,  -- 'IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rcg_route ON route_condition_groups(route_id);
```

### 1.4 route_conditions

```sql
CREATE TABLE route_conditions (
  id              BIGSERIAL PRIMARY KEY,
  group_id        BIGINT NOT NULL REFERENCES route_condition_groups(id) ON DELETE CASCADE,
  condition_type  VARCHAR(30) NOT NULL,  -- 'operator', 'country', 'traffic_type', 'paid_name', 'regex'
  condition_value TEXT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rc_group ON route_conditions(group_id);
```

Multiple values for one condition_type within a group are OR-joined (e.g. operator=MTS OR operator=Beeline).

### 1.5 route_schedules

```sql
CREATE TABLE route_schedules (
  id         BIGSERIAL PRIMARY KEY,
  route_id   BIGINT NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  date_from  DATE,
  date_to    DATE,
  time_from  TIME,
  time_to    TIME,
  weekdays   SMALLINT NOT NULL DEFAULT 127,  -- bitmask: 1=Mon, 2=Tue, 4=Wed... 127=all
  timezone   VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rs_route ON route_schedules(route_id);
```

## 2. Backend — Domain Model

### 2.1 Types

```go
type TrafficType string
const (
    TrafficTypeAuthorization TrafficType = "authorization"
    TrafficTypeTransactional TrafficType = "transactional"
    TrafficTypeService       TrafficType = "service"
)

type RouteStatus string
const (
    RouteStatusActive RouteStatus = "active"
    RouteStatusDraft  RouteStatus = "draft"
)

type ConditionType string
const (
    ConditionOperator    ConditionType = "operator"
    ConditionCountry     ConditionType = "country"
    ConditionTrafficType ConditionType = "traffic_type"
    ConditionPaidName    ConditionType = "paid_name"
    ConditionRegex       ConditionType = "regex"
)

type LogicOp string
const (
    LogicIf     LogicOp = "IF"
    LogicAnd    LogicOp = "AND"
    LogicAndNot LogicOp = "AND_NOT"
    LogicOr     LogicOp = "OR"
    LogicOrNot  LogicOp = "OR_NOT"
)
```

### 2.2 Structures

```go
type Route struct {
    ID         int64
    ClientID   *int64          // nil = default
    Name       string
    Comment    string
    Status     RouteStatus
    RouteType  string          // sms, hlr, max
    OperatorID *int64          // legacy, kept for backward compat; matching uses conditions
    ProviderID int64
    Priority   int
    Share      int             // 0-100%
    Groups     []ConditionGroup
    Schedules  []Schedule
}

type ConditionGroup struct {
    ID         int64
    GroupIndex int
    LogicOp    LogicOp
    Conditions []Condition
}

type Condition struct {
    ID    int64
    Type  ConditionType
    Value string
}

type Schedule struct {
    ID       int64
    DateFrom *time.Time
    DateTo   *time.Time
    TimeFrom *string
    TimeTo   *string
    Weekdays int       // bitmask
    Timezone string
}
```

### 2.3 RouteMatcher (in-memory)

```
RouteMatcher
  ├── Load()               — loads all active routes + groups + conditions + schedules, compiles regex
  ├── Match(MatchContext)   — returns matching routes sorted by priority
  └── Invalidate()         — triggered by Kafka event on route change
```

```go
type MatchContext struct {
    RouteType   string
    ClientID    int64
    OperatorID  *int64
    CountryCode string
    TrafficType TrafficType
    PaidName    bool
    MessageBody string
    SenderName  string
}
```

**Matching algorithm:**
1. Filter by `route_type`
2. Find client-specific routes (`client_id = ctx.ClientID`), match conditions
3. If none found — fallback to default routes (`client_id IS NULL`)
4. For each route: evaluate condition groups in order (IF → AND/AND_NOT/OR/OR_NOT)
5. Check schedules (time, weekdays)
6. Sort by priority (lowest = highest), apply share weight

## 3. API Endpoints

### 3.1 CRUD

```
POST   /portal/v1/routes              — create route
GET    /portal/v1/routes              — list (filters: client_id, status, route_type, operator, country)
GET    /portal/v1/routes/:id          — get route with conditions and schedules
PUT    /portal/v1/routes/:id          — update (full replace of conditions/schedules)
DELETE /portal/v1/routes/:id          — delete
```

### 3.2 Create/Update payload

```json
{
  "client_id": null,
  "name": "МТС Transactional",
  "comment": "Основной маршрут для transactional SMS по России и МТС",
  "status": "active",
  "route_type": "sms",
  "operator_id": 3,
  "provider_id": 5,
  "priority": 11,
  "share": 100,
  "condition_groups": [
    {
      "logic_op": "IF",
      "conditions": [
        {"type": "operator", "value": "MTS"},
        {"type": "traffic_type", "value": "transactional"},
        {"type": "country", "value": "RU"},
        {"type": "paid_name", "value": "true"}
      ]
    },
    {
      "logic_op": "AND",
      "conditions": [
        {"type": "regex", "value": "/(код|code)/ui"}
      ]
    },
    {
      "logic_op": "AND_NOT",
      "conditions": [
        {"type": "regex", "value": "/(промокод|promocode)/ui"}
      ]
    }
  ],
  "schedules": [
    {
      "date_from": "2026-04-08",
      "date_to": "2026-12-31",
      "time_from": "08:00",
      "time_to": "22:00",
      "weekdays": 31,
      "timezone": "Europe/Moscow"
    }
  ]
}
```

### 3.3 List response

```json
{
  "routes": [
    {
      "id": 1,
      "client_id": null,
      "status": "active",
      "route_type": "sms",
      "priority": 10,
      "share": 100,
      "provider": {"id": 5, "name": "Provider-MTS-RU", "channel": "Standard SMS"},
      "condition_tags": ["SMS", "Россия", "МТС", "Transactional"],
      "schedule_summary": "Пн–Пт, 08:00–22:00",
      "comment": "Основной дефолтный маршрут по МТС"
    }
  ],
  "total": 6
}
```

### 3.4 References

```
GET /portal/v1/routes/references — operators, countries, providers, channels, traffic types
```

### 3.5 Conflict check

```
POST /portal/v1/routes/check-conflicts — accepts conditions, returns overlapping routes
```

### 3.6 Templates

Existing `PUT /portal/v1/templates/:id` extended with `traffic_type` field.

## 4. Frontend

### 4.1 File structure

```
portal-frontend/src/pages/routing/
  ├── RoutingPage.tsx              — full replacement, route list table
  ├── RouteModal.tsx               — create/edit modal
  ├── components/
  │   ├── RouteTable.tsx           — sortable route table
  │   ├── RouteFilters.tsx         — search + filter chips
  │   ├── ConditionEditor.tsx      — condition editor with groups
  │   ├── ConditionGroup.tsx       — single condition group
  │   ├── ConditionRow.tsx         — condition row (type + value + delete)
  │   ├── ScheduleEditor.tsx       — schedule editor (dates, time, weekdays)
  │   ├── RouteSummary.tsx         — preview sidebar in modal
  │   └── ConflictDialog.tsx       — conflict check results dialog
  └── types.ts                     — Route, ConditionGroup, Schedule types
```

### 4.2 RoutingPage

Replaces current page. Matches "Общие маршруты" mockup:
- Header: title + "Проверить конфликты" + "+ Добавить маршрут" buttons
- Toolbar: search input, filter chips (type, status, country)
- Table: Условия (tags), Провайдеры, Приоритет, Доля, Статус (with comment tooltip), Actions
- Click "+" or "✎" opens RouteModal

### 4.3 RouteModal

Matches "Добавление маршрута" mockup:
- **"Основное"** — type (sms/hlr/max), priority, share, comment, status chip
- **"Условия срабатывания"** — ConditionEditor with groups (IF/AND/AND NOT/OR/OR NOT)
- **"Канал доставки"** — provider select, channel select
- **"Расписание"** — toggle on/off, dates, time, weekdays, timezone
- **Sidebar "Предпросмотр"** — RouteSummary, updates reactively
- **Footer** — "Отмена", "Сохранить как черновик", "Сохранить маршрут"

### 4.4 Templates extension

In TemplatesPage: add "Тип трафика" select to create/edit form, show as tag in table.

## 5. Pipeline Integration

### 5.1 Match context assembly

```
API Request → Validate → Resolve Operator → Build MatchContext → RouteMatcher.Match() → Send via Provider
```

MatchContext built from:
- `route_type` — from request (default: sms)
- `client_id` — from auth
- `operator_id` — from operator resolve (by phone prefix)
- `country_code` — from operator or prefix
- `traffic_type` — from template linked to message
- `paid_name` — from sender_name
- `message_body` — message text for regex conditions

### 5.2 Fallback logic

1. Find routes with `client_id = ctx.ClientID` → match conditions → filter by schedule
2. If found → select by priority (lowest = highest)
3. If not found → find routes with `client_id IS NULL` (default) → match → filter
4. If nothing found → error "no route found", message to retry queue or reject

### 5.3 Cache invalidation

On route CRUD:
- Backend publishes Kafka event `route.updated`
- All service instances listen and call `RouteMatcher.Invalidate()`
- Matcher reloads all routes from DB
- Redis cache cleared (`routes:active:*` keys)

### 5.4 Backward compatibility

Existing `client_routes` migrated:
- Each gets one IF group with `operator = <operator_id>` condition
- `weight` → `share`
- `active = true` → `status = 'active'`
- Data migration in SQL migration file
