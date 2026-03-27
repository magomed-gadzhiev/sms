# Advanced Routing & Provider Pricing Design

**Date:** 2026-03-27
**Status:** Approved

## Overview

Расширенная система маршрутизации SMS-платформы, которая:
- Эволюционирует pattern-based routing в operator→provider маппинг per client
- Учитывает пропускную способность провайдеров (TPS + квоты)
- Добавляет систему цен провайдеров (себестоимость) с полной тарификацией (4 стратегии)
- Даёт клиентам возможность настраивать стратегию маршрутизации (priority / weighted) per operator
- Поддерживает иерархию клиент→субклиент с шарингом провайдеров и контролем видимости

## Architecture: Three Independent Layers

### Layer 1: Routing (без цен)

Принятие решений о маршрутизации на основе стратегии, приоритетов/весов, доступности и capacity. Цены провайдеров не участвуют в routing pipeline.

### Layer 2: Cost Accounting (аналитика)

Учёт себестоимости отправленных сообщений. Полная тарификация провайдеров (план→период→тир, 4 стратегии). Используется для отчётов маржи, не для маршрутизации. Задел для будущего cost-based routing.

### Layer 3: Visibility (permissions)

Контроль видимости себестоимости и имён провайдеров для субклиентов. По умолчанию субклиенты не видят себестоимость. Родитель явно включает при необходимости.

## Data Model

### Provider Ownership

```sql
CREATE TABLE client_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    ownership VARCHAR(20) NOT NULL CHECK (ownership IN ('platform', 'private', 'inherited')),
    source_client_id UUID REFERENCES clients(id),  -- от кого унаследован (для inherited)
    shared_priority INT NOT NULL DEFAULT 0,         -- приоритет при конкуренции за shared capacity
    expose_cost BOOLEAN NOT NULL DEFAULT false,     -- показывать себестоимость (для inherited)
    expose_provider_name BOOLEAN NOT NULL DEFAULT true, -- показывать имя провайдера (для inherited)
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, provider_id)
);
```

- `platform` — назначен админом (shared провайдер)
- `private` — клиент сам подключил
- `inherited` — расшарен родителем, `source_client_id` указывает на родителя

### Provider Capacity

Расширение существующей таблицы `providers`:

```sql
ALTER TABLE providers RENAME COLUMN throughput_per_second TO tps_limit;
ALTER TABLE providers ADD COLUMN daily_quota INT;
ALTER TABLE providers ADD COLUMN monthly_quota INT;
```

Capacity — свойство провайдера целиком (не per operator). TPS — жёсткий лимит, квоты — суточная и месячная.

### Routing Strategy

```sql
CREATE TABLE client_routing_strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID REFERENCES operators(id),  -- NULL = дефолт аккаунта
    strategy VARCHAR(20) NOT NULL CHECK (strategy IN ('priority', 'weighted', 'smart')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id)
);

-- Дефолт аккаунта (operator_id IS NULL) — отдельный partial unique:
CREATE UNIQUE INDEX uq_client_routing_strategy_default
    ON client_routing_strategies (client_id)
    WHERE operator_id IS NULL;
```

Логика разрешения: (client, operator) → (client, NULL) → default `priority`.

### Client Routes

```sql
CREATE TABLE client_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    priority INT NOT NULL DEFAULT 0,    -- для strategy=priority (выше = приоритетнее)
    weight INT NOT NULL DEFAULT 1,      -- для strategy=weighted
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(client_id, operator_id, provider_id)
);
```

Маршрут = operator→provider для конкретного клиента. Один клиент может иметь несколько провайдеров для одного оператора — каждый с приоритетом и весом.

**Application-level constraint:** при создании client_route проверяем, что provider_id есть в client_providers для данного client_id. FK на уровне БД не подходит (нет composite FK на client_providers).

### Provider Tariff Plans (Cost Accounting)

Полная копия клиентской тарификации:

```sql
CREATE TABLE provider_tariff_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    operator_id UUID NOT NULL REFERENCES operators(id),
    strategy VARCHAR(50) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_id, operator_id) WHERE (active = true)  -- partial unique
);

CREATE TABLE provider_tariff_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id) ON DELETE CASCADE,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    EXCLUDE USING gist (
        provider_tariff_plan_id WITH =,
        daterange(start_date, end_date, '[]') WITH &&
    )
);

CREATE TABLE provider_tariff_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id) ON DELETE CASCADE,
    from_count INT NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    UNIQUE(provider_tariff_period_id, from_count)
);

CREATE TABLE provider_usage_counters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_tariff_plan_id UUID NOT NULL REFERENCES provider_tariff_plans(id),
    provider_tariff_period_id UUID NOT NULL REFERENCES provider_tariff_periods(id),
    segment_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_tariff_plan_id, provider_tariff_period_id)
);
```

### Provider Tarification Log

```sql
CREATE TABLE provider_tarification_log (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    provider_id UUID NOT NULL,
    operator_id UUID NOT NULL,
    client_id UUID NOT NULL,
    message_id UUID NOT NULL,
    segment_count INT NOT NULL,
    price_per_segment NUMERIC(20,6) NOT NULL,
    total_cost NUMERIC(20,6) NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    provider_tariff_plan_id UUID NOT NULL,
    provider_tariff_period_id UUID NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (id, created_at),
    UNIQUE(idempotency_key, created_at)
) PARTITION BY RANGE (created_at);
```

Партиционирование помесячное (12 партиций на 2026), аналогично `tarification_log`.

### Client Extensions

```sql
ALTER TABLE clients ADD COLUMN routing_mode VARCHAR(20) NOT NULL DEFAULT 'legacy'
    CHECK (routing_mode IN ('legacy', 'new', 'hybrid'));
ALTER TABLE clients ADD COLUMN cost_visibility_enabled BOOLEAN NOT NULL DEFAULT false;
```

- `routing_mode`: управление переходом (`legacy` → `hybrid` → `new`)
- `cost_visibility_enabled`: видимость себестоимости для данного клиента

## Routing Engine

### Updated Pipeline

```
1. Receive message (client_id, phone, text)
2. Resolve operator by prefix → operator_id, country_code
3. Check client.routing_mode:
   ├── 'legacy' → go to step 7
   ├── 'new'    → go to step 4, no fallback
   └── 'hybrid' → go to step 4, fallback to step 7
4. Find client_routing_strategy (client_id, operator_id)
   │  fallback → (client_id, NULL)
   │  fallback → default 'priority'
5. Find client_routes (client_id, operator_id) WHERE active=true
   │  Empty? → if hybrid, go to step 7; else → error
6. Filter & select provider:
   │  a. Filter: provider.active, health check, capacity check
   │  b. Select by strategy:
   │     ├── priority: sort by priority DESC, first available
   │     └── weighted: weighted random selection
   │  → provider selected
   │  → go to step 8
7. Legacy fallback: pattern-based routes + old selection strategies
8. Tarify client (existing tarification)
9. Enqueue → Kafka → Send via SMPP
10. On success: async log provider cost (Cost Accounting layer)
```

### Priority Strategy with Auto-Failover

```
Candidates: [Provider A (prio=10), Provider B (prio=5), Provider C (prio=1)]

1. Try Provider A (prio=10)
   ├── active? yes
   ├── healthy? yes (success_rate > threshold)
   ├── capacity? TPS: 45/100, daily: 8000/10000 — OK
   └── SELECTED

If Provider A unavailable:
   ├── active=false OR unhealthy OR capacity exhausted
   └── Skip → try Provider B (prio=5) → same checks → ...
```

### Weighted Strategy

```
After availability filtering:
  Provider A (weight=3), Provider C (weight=1)
  Provider B filtered out (unhealthy)

Probabilities: A = 3/4 = 75%, C = 1/4 = 25%
Algorithm: weighted random selection
```

### Capacity Tracking (Redis)

```
provider:{id}:tps_current    -- sliding window 1s (INCR + EXPIRE)
provider:{id}:daily_count    -- daily counter (reset 00:00 UTC)
provider:{id}:monthly_count  -- monthly counter (reset 1st of month)
```

Проверка — атомарная операция перед отправкой. Превышение лимита → провайдер исключается из кандидатов.

### Shared Provider Priority

Когда платформенный провайдер используется несколькими клиентами и TPS близок к лимиту:

- Клиенты сортируются по `client_providers.shared_priority` DESC
- Клиент с более высоким приоритетом получает ёмкость первым
- Клиент с низким приоритетом получает failover на другой провайдер

Реализация: Redis-based rate limiter с weighted fairness по shared_priority.

## Cost Accounting & Margin Analytics

### Cost Calculation Flow

Выполняется **после** успешной отправки (или DLR), асинхронно:

```
1. Find active provider_tariff_plan (provider_id, operator_id)
2. Find provider_tariff_period covering current date
3. Find tier by provider_usage_counter.segment_count
4. Calculate price_per_segment (same 4 strategies as client tarification)
5. Increment provider_usage_counter
6. Write to provider_tarification_log
```

### Margin Report

```
┌─────────────┬──────────┬──────────┬───────────┬─────────┬────────┐
│ Operator    │ Provider │ Segments │ Revenue   │ Cost    │ Margin │
├─────────────┼──────────┼──────────┼───────────┼─────────┼────────┤
│ МТС RU      │ ConnPlus │ 45,200   │ 113,000 ₽ │ 81,360 ₽│ 31,640 ₽│
│ Билайн RU   │ ConnPlus │ 23,100   │  50,820 ₽ │ 39,270 ₽│ 11,550 ₽│
├─────────────┼──────────┼──────────┼───────────┼─────────┼────────┤
│ TOTAL       │          │ 68,300   │ 163,820 ₽ │120,630 ₽│ 43,190 ₽│
└─────────────┴──────────┴──────────┴───────────┴─────────┴────────┘
```

```sql
Revenue = SUM(tarification_log.total_amount)        WHERE client_id = ?
Cost    = SUM(provider_tarification_log.total_cost) WHERE client_id = ?
Margin  = Revenue - Cost
```

Группировка: по оператору, провайдеру, периоду, субклиенту.

### Visibility Rules

| Роль | Revenue | Cost | Margin | Provider name |
|------|---------|------|--------|---------------|
| Агрегатор (cost_visibility=true) | yes | yes | yes | yes |
| Субклиент (default) | yes | no | no | no |
| Субклиент (expose_cost=true) | yes | yes | yes | depends on expose_provider_name |

## gRPC API

### Routing Service Extensions

```protobuf
// Client Providers
rpc AssignProviderToClient(AssignProviderRequest) returns (ClientProviderProto);
rpc RevokeProviderFromClient(RevokeProviderRequest) returns (google.protobuf.Empty);
rpc ListClientProviders(ListClientProvidersRequest) returns (ListClientProvidersResponse);
rpc UpdateClientProvider(UpdateClientProviderRequest) returns (ClientProviderProto);

// Provider Sharing (parent → child)
rpc ShareProviderWithChild(ShareProviderRequest) returns (ClientProviderProto);
rpc RevokeSharedProvider(RevokeSharedProviderRequest) returns (google.protobuf.Empty);
rpc ListSharedProviders(ListSharedProvidersRequest) returns (ListSharedProvidersResponse);

// Client Routes
rpc CreateClientRoute(CreateClientRouteRequest) returns (ClientRouteProto);
rpc UpdateClientRoute(UpdateClientRouteRequest) returns (ClientRouteProto);
rpc DeleteClientRoute(DeleteClientRouteRequest) returns (google.protobuf.Empty);
rpc ListClientRoutes(ListClientRoutesRequest) returns (ListClientRoutesResponse);

// Routing Strategy
rpc SetRoutingStrategy(SetRoutingStrategyRequest) returns (ClientRoutingStrategyProto);
rpc GetRoutingStrategy(GetRoutingStrategyRequest) returns (ClientRoutingStrategyProto);
rpc DeleteRoutingStrategy(DeleteRoutingStrategyRequest) returns (google.protobuf.Empty);
```

### Tarification Service Extensions

```protobuf
// Provider Tariff Plans
rpc CreateProviderTariffPlan(CreateProviderTariffPlanRequest) returns (ProviderTariffPlanProto);
rpc GetProviderTariffPlan(GetProviderTariffPlanRequest) returns (ProviderTariffPlanProto);
rpc ListProviderTariffPlans(ListProviderTariffPlansRequest) returns (ListProviderTariffPlansResponse);
rpc UpdateProviderTariffPlan(UpdateProviderTariffPlanRequest) returns (ProviderTariffPlanProto);

rpc CreateProviderTariffPeriod(CreateProviderTariffPeriodRequest) returns (ProviderTariffPeriodProto);
rpc CreateProviderTariffTier(CreateProviderTariffTierRequest) returns (ProviderTariffTierProto);
rpc UpdateProviderTariffTier(UpdateProviderTariffTierRequest) returns (ProviderTariffTierProto);

// Margin Analytics
rpc GetMarginReport(MarginReportRequest) returns (MarginReportResponse);
```

### REST API Mapping

```
POST   /api/v1/clients/{id}/providers              → AssignProviderToClient
GET    /api/v1/clients/{id}/providers              → ListClientProviders
DELETE /api/v1/clients/{id}/providers/{pid}        → RevokeProviderFromClient

POST   /api/v1/clients/{id}/providers/share        → ShareProviderWithChild
DELETE /api/v1/clients/{id}/providers/share/{sid}  → RevokeSharedProvider

POST   /api/v1/clients/{id}/routes                 → CreateClientRoute
GET    /api/v1/clients/{id}/routes                 → ListClientRoutes
PUT    /api/v1/clients/{id}/routes/{rid}           → UpdateClientRoute
DELETE /api/v1/clients/{id}/routes/{rid}           → DeleteClientRoute

PUT    /api/v1/clients/{id}/routing-strategy       → SetRoutingStrategy
GET    /api/v1/clients/{id}/routing-strategy       → GetRoutingStrategy

GET    /api/v1/clients/{id}/analytics/margin       → GetMarginReport
```

### Authorization

| Operation | Who can |
|-----------|---------|
| Assign platform provider | admin only |
| Add private provider | owner client |
| Share provider to child | parent client |
| CRUD client_routes | owner client (own providers only) |
| Set routing strategy | owner client |
| CRUD provider_tariff_plans | admin only |
| View margin report (full) | client with cost_visibility=true |
| View margin report (revenue only) | any client for own traffic |

## Migration & Backward Compatibility

### Migration Strategy

Постепенная миграция без даунтайма. Поле `clients.routing_mode` контролирует переключение:

- `legacy` — только pattern-based routes (текущее поведение)
- `hybrid` — client_routes приоритетнее, fallback на legacy
- `new` — только client_routes

### Routing Priority

```
routing_mode = 'hybrid':
  1. Check client_routes → found? use new system
  2. Not found → fallback to legacy pattern-based routes

routing_mode = 'new':
  1. Check client_routes → found? use new system
  2. Not found → error (no route)

routing_mode = 'legacy':
  1. Skip client_routes entirely
  2. Use legacy pattern-based routes
```

### smart_route_weights Integration

Текущий `smart_route_weights` становится третьей стратегией `smart` в `client_routing_strategies`. Это объединяет все стратегии в единую модель.

### Database Migrations

```
000037_create_client_providers.up.sql
  - CREATE TABLE client_providers

000038_create_client_routing.up.sql
  - CREATE TABLE client_routing_strategies
  - CREATE TABLE client_routes

000039_create_provider_tarification.up.sql
  - CREATE TABLE provider_tariff_plans
  - CREATE TABLE provider_tariff_periods
  - CREATE TABLE provider_tariff_tiers
  - CREATE TABLE provider_usage_counters
  - CREATE TABLE provider_tarification_log (partitioned, 12 monthly partitions)

000040_extend_providers_capacity.up.sql
  - ALTER TABLE providers: RENAME throughput_per_second → tps_limit
  - ALTER TABLE providers: ADD daily_quota, monthly_quota

000041_extend_clients_routing.up.sql
  - ALTER TABLE clients: ADD routing_mode DEFAULT 'legacy'
  - ALTER TABLE clients: ADD cost_visibility_enabled DEFAULT false
```

### Data Migration

Автоматическая генерация client_routes из существующих pattern-based routes (для маршрутов, маппящихся на конкретного оператора). Legacy-маршруты с regex/сложными паттернами остаются в fallback.
