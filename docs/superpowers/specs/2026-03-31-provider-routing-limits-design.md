# Дизайн: провайдеры, маршрутизация, лимиты

**Дата:** 2026-03-31
**Статус:** Approved
**Подход:** Эволюционный (Подход 1)

---

## Контекст и мотивация

Текущая архитектура имеет ряд структурных проблем:

1. **Нет per-client TPS на провайдере** — `throughput_per_sec` глобален, один агрессивный клиент может забрать весь throughput
2. **Заглушка (SIMULATOR) — хак в коде** — определяется через `if SystemType == "SIMULATOR"` внутри Sender, не конфигурируема
3. **Две параллельные системы маршрутизации** — legacy `routes` и новая `client_routes` не интегрированы; pipeline использует только legacy
4. **Нет глобальных defaults** — лимиты хардкодом в миграции, администратор не может менять без редеплоя
5. **Реселлер не может управлять лимитами субаккаунтов** — в Portal API нет endpoints для TPS, маршрутов и провайдеров субаккаунтов

---

## Ключевые решения

| Вопрос | Решение |
|--------|---------|
| Per-client TPS vs глобальный | **Б**: per-client — верхняя граница, глобальный TPS — общий потолок. Oversubscription допустим |
| Legacy vs client routing | **А**: убрать legacy, единая система. Общие маршруты = дефолтные `client_routes` с `client_id = NULL` |
| Заглушка | **Б**: полноценный провайдер с конфигурируемым поведением, доступный клиентам |
| Реселлер → субаккаунты | **Б**: делегирование — реселлер выделяет провайдеры + TPS-бюджет, субаккаунт распределяет |
| Defaults | **В**: иерархия — global → tariff plan → per-client → reseller override для субаккаунтов |

---

## Секция 1: Изменения модели данных

### Новая таблица `system_defaults`

```sql
CREATE TABLE system_defaults (
    key        VARCHAR(100) PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    updated_by UUID
);

INSERT INTO system_defaults VALUES
  ('rate_limit_per_second',    '10'),
  ('rate_limit_per_minute',    '100'),
  ('rate_limit_per_hour',      '1000'),
  ('default_tps_per_provider', '5'),
  ('max_providers_per_client', '10'),
  ('max_sub_accounts',         '0');
```

### Новая таблица `stub_provider_config`

```sql
CREATE TABLE stub_provider_config (
    provider_id      UUID PRIMARY KEY REFERENCES providers(id),
    min_delay_ms     INT DEFAULT 100,
    max_delay_ms     INT DEFAULT 500,
    failure_rate_pct INT DEFAULT 0,       -- % случайных ошибок (0-100)
    dlr_delay_ms     INT DEFAULT 1000,
    dlr_success_rate INT DEFAULT 100,     -- % успешных DLR
    dlr_statuses     JSONB DEFAULT '["DELIVRD"]',
    created_at       TIMESTAMPTZ DEFAULT now(),
    updated_at       TIMESTAMPTZ DEFAULT now()
);
```

### Новая таблица `operator_prefixes`

```sql
CREATE TABLE operator_prefixes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL REFERENCES operators(id),
    prefix      VARCHAR(20) NOT NULL,
    priority    INT DEFAULT 0,   -- длиннее prefix = выше priority
    active      BOOLEAN DEFAULT true,
    UNIQUE(prefix)
);
```

Используется для offline-определения оператора по номеру (без HLR). Если оператор не найден — используется дефолтный оператор платформы (`DEFAULT_OPERATOR_ID`), для которого администратор обязан настроить дефолтный маршрут.

### Расширение `tariff_plans`

```sql
ALTER TABLE tariff_plans ADD COLUMN
    rate_limit_per_second    INT,          -- NULL = из system_defaults
    rate_limit_per_minute    INT,
    rate_limit_per_hour      INT,
    default_tps_per_provider INT,
    max_providers            INT,
    max_sub_accounts         INT;
```

### Расширение `client_providers`

```sql
ALTER TABLE client_providers ADD COLUMN
    tps_limit INT;  -- NULL = из tariff_plan или system_defaults
```

### Расширение `client_routes`

```sql
-- client_id = NULL означает платформенный дефолт
ALTER TABLE client_routes ALTER COLUMN client_id DROP NOT NULL;

ALTER TABLE client_routes ADD COLUMN
    shared BOOLEAN DEFAULT false;  -- реселлер может поделиться с субаккаунтами

CREATE INDEX idx_client_routes_default
    ON client_routes (operator_id)
    WHERE client_id IS NULL AND active = true;
```

### Расширение `clients`

```sql
ALTER TABLE clients ADD COLUMN
    allocated_tps_budget INT;  -- NULL = не ограничено. Для субаккаунтов — бюджет от реселлера
```

---

## Секция 2: LimitResolver

### Интерфейс

```go
type ResolvedLimits struct {
    RateLimitPerSecond    int
    RateLimitPerMinute    int
    RateLimitPerHour      int
    DefaultTPSPerProvider int
    MaxProviders          int
    MaxSubAccounts        int
}

type LimitResolver interface {
    Resolve(ctx context.Context, clientID uuid.UUID) (*ResolvedLimits, error)
    ResolveProviderTPS(ctx context.Context, clientID, providerID uuid.UUID) (int, error)
}
```

### Цепочка разрешения `ResolveProviderTPS`

```
1. client_providers.tps_limit для (clientID, providerID) → если не NULL → return
2. tariff_plans.default_tps_per_provider для клиента    → если не NULL → return
3. system_defaults['default_tps_per_provider']          → return (всегда есть)
```

Для субаккаунта дополнительно:

```
4. resolved_tps = результат шагов 1-3
5. если client.parent_client_id != NULL && client.allocated_tps_budget != NULL:
     sum_allocated = SUM(tps_limit) всех провайдеров субаккаунта (кроме текущего)
     max_allowed = allocated_tps_budget - sum_allocated
     return min(resolved_tps, max_allowed)
```

### Кеширование

```go
type CachedLimitResolver struct {
    inner LimitResolver
    cache *redis.Client
    ttl   time.Duration  // 30 секунд
}
// Ключи:
// limits:{clientID}              → ResolvedLimits JSON
// limits:{clientID}:{providerID}:tps → int
```

Инвалидация через Redis Pub/Sub при изменении `client_providers`, `tariff_plans`, `system_defaults`.

### Интеграция с существующим кодом

**Rate Limit Middleware:**
```go
// Было: client.RateLimitPerSecond
// Стало:
resolved, _ := limitResolver.Resolve(ctx, clientID)
limits := resolved.RateLimitPerSecond
```

---

## Секция 3: Унифицированная маршрутизация

### Уровни маршрутов (приоритет по убыванию)

```
1. Собственные маршруты клиента  (client_routes WHERE client_id = X)
2. Shared маршруты реселлера     (client_routes WHERE client_id = parent_id AND shared = true)
3. Дефолтные платформы           (client_routes WHERE client_id IS NULL)
```

Внутри уровня — стратегия: `priority` (по умолчанию) / `weighted` / `smart`.

### UnifiedRouter

```go
type UnifiedRouter struct {
    routeRepo    ClientRouteRepository
    strategyRepo ClientRoutingStrategyRepository
    clientRepo   ClientRepository
    logger       zerolog.Logger
}

func (r *UnifiedRouter) Route(ctx context.Context, clientID, operatorID uuid.UUID) (*RoutingDecision, error) {
    // 1. Собственные маршруты
    routes := r.routeRepo.ListByClientAndOperator(clientID, operatorID)

    // 2. Shared от реселлера
    if len(routes) == 0 {
        client, _ := r.clientRepo.GetByID(ctx, clientID)
        if client.ParentClientID != nil {
            routes = r.routeRepo.ListSharedByClientAndOperator(*client.ParentClientID, operatorID)
        }
    }

    // 3. Дефолтные платформы
    if len(routes) == 0 {
        routes = r.routeRepo.ListDefaultByOperator(operatorID)
    }

    if len(routes) == 0 {
        return nil, ErrNoRouteFound
    }

    strategy := r.resolveStrategy(clientID, operatorID)
    selected := strategy.Select(routes)

    return &RoutingDecision{
        ProviderID: selected.ProviderID,
        RouteID:    selected.ID,
    }, nil
}
```

Оборачивается в `CachedUnifiedRouter` — in-memory кеш per (clientID, operatorID) с TTL 30 сек.

### Миграция legacy → unified

```sql
-- Шаг 1: создать дефолтные client_routes из legacy routes
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active)
SELECT NULL,
       resolve_operator_from_pattern(r.pattern),
       r.provider_id,
       r.priority,
       1,
       r.active
FROM routes r;

-- Шаг 2: failover → второй маршрут с меньшим приоритетом
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active)
SELECT NULL,
       resolve_operator_from_pattern(r.pattern),
       r.failover_provider_id,
       r.priority - 1,
       1,
       r.active
FROM routes r
WHERE r.failover_provider_id IS NOT NULL;
```

После миграции и проверки: удалить таблицу `routes`, `CachedRouter`, legacy `Router`.

### Router Stage

```go
// Было:
provider, err := cachedRouter.RouteMessage(ctx, msg)

// Стало:
operatorID := operatorResolver.Resolve(msg.Destination)
decision, err := unifiedRouter.Route(ctx, *msg.ClientID, operatorID)
msg.ProviderID = &decision.ProviderID
msg.RouteID = decision.RouteID
```

---

## Секция 4: Stub-провайдер

### Принцип

Stub — обычный провайдер в таблице `providers` (`system_type = 'SIMULATOR'`), но с конфигурируемым поведением через `stub_provider_config`. Доступен клиентам как платформенный провайдер.

### StubSender

```go
type Sender interface {
    SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, conn AsyncConn) (string, error)
}

type StubSender struct {
    configRepo  StubProviderConfigRepository
    dlrProducer queue.Producer
    logger      zerolog.Logger
}

func (s *StubSender) SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, _ AsyncConn) (string, error) {
    cfg, _ := s.configRepo.GetByProviderID(provider.ID)

    // Имитация задержки
    delay := cfg.MinDelayMs + rand.Intn(cfg.MaxDelayMs-cfg.MinDelayMs)
    time.Sleep(time.Duration(delay) * time.Millisecond)

    // Имитация ошибки
    if rand.Intn(100) < cfg.FailureRatePct {
        return "", fmt.Errorf("stub: simulated failure")
    }

    smppMsgID := "stub-" + uuid.New().String()
    go s.scheduleDLR(msg, smppMsgID, cfg)
    return smppMsgID, nil
}

func (s *StubSender) scheduleDLR(msg *shared.Message, smppMsgID string, cfg *StubProviderConfig) {
    time.Sleep(time.Duration(cfg.DLRDelayMs) * time.Millisecond)

    stat := "DELIVRD"
    if rand.Intn(100) >= cfg.DLRSuccessRate {
        stat = cfg.randomDLRStatus()
    }

    s.dlrProducer.PublishDLR(context.Background(), &queue.DLRMessage{
        MessageID:     msg.ID,
        SMPPMessageID: smppMsgID,
        Stat:          stat,
    })
}
```

### SenderFactory

```go
type SenderFactory struct {
    smppSender *SMPPSender
    stubSender *StubSender
}

func (f *SenderFactory) For(provider *shared.Provider) Sender {
    if provider.SystemType == "SIMULATOR" {
        return f.stubSender
    }
    return f.smppSender
}
```

`SMPPSender` больше не содержит `if SIMULATOR` логику. Для stub-провайдеров SMPP-соединение в пуле не создаётся.

---

## Секция 5: Управление реселлера субаккаунтами

### Модель делегирования

```
Реселлер имеет: Provider A (200 TPS), Provider B (100 TPS)
Реселлер выделяет субаккаунту:
  clients.allocated_tps_budget = 50
  client_providers: Provider A (ownership='inherited', tps_limit=30)
  client_providers: Provider B (ownership='inherited', tps_limit=20)
Субаккаунт настраивает свои маршруты внутри выделенных ресурсов.
```

### Portal API — новые endpoints

```
POST   /portal/v1/sub-accounts/:id/providers
       { "provider_id": "...", "tps_limit": 30 }

GET    /portal/v1/sub-accounts/:id/providers

PUT    /portal/v1/sub-accounts/:id/providers/:pid
       { "tps_limit": 40 }

DELETE /portal/v1/sub-accounts/:id/providers/:pid

PUT    /portal/v1/sub-accounts/:id/budget
       { "tps_budget": 50 }

POST   /portal/v1/sub-accounts/:id/routes
       { "operator_id": "...", "provider_id": "...", "priority": 100 }

GET    /portal/v1/sub-accounts/:id/routes
```

### Валидации

- При выделении провайдера: реселлер сам имеет этого провайдера
- При выделении провайдера: сумма `tps_limit` субаккаунта ≤ `allocated_tps_budget`
- При обновлении бюджета: новый бюджет ≥ сумма уже назначенных `tps_limit`
- При создании маршрута: провайдер должен быть в `client_providers` субаккаунта

---

## Секция 6: Интеграция с pipeline

### Двухуровневый BackpressureManager

```go
type BackpressureManager struct {
    global map[uuid.UUID]*TokenBucket             // per provider
    client map[clientProviderKey]*TokenBucket     // per (client, provider)
    mu     sync.RWMutex
}

func (m *BackpressureManager) TryAcquire(clientID, providerID uuid.UUID) bool {
    clientKey := clientProviderKey{clientID, providerID}

    // Сначала per-client (верхняя граница)
    if !m.client[clientKey].TryConsume() {
        return false
    }

    // Затем глобальный (общий потолок)
    if !m.global[providerID].TryConsume() {
        m.client[clientKey].Refund()
        return false
    }

    return true
}
```

Инициализация при старте Sender Stage:
1. Глобальные бакеты — для каждого активного провайдера
2. Per-client бакеты — для каждой пары (client, provider) из `client_providers`

### Инвалидация бакетов

При изменении `tps_limit` через API:
```go
redis.Publish("limits:invalidate", clientProviderKey{clientID, providerID})
```

Горутина в Sender Stage слушает события и обновляет бакеты:
```go
go func() {
    sub := redis.Subscribe("limits:invalidate")
    for msg := range sub.Channel() {
        key := parseKey(msg.Payload)
        tps, _ := limitResolver.ResolveProviderTPS(ctx, key.ClientID, key.ProviderID)
        bpManager.UpdateClient(key.ClientID, key.ProviderID, tps)
    }
}()
```

### Sender Stage — изменённый порядок шагов

```
1. Billing check (frozen?)
2. Tarification
3. TryAcquire(clientID, providerID)  ← per-client + global backpressure
4. Получение async соединения (nil для stub)
5. SenderFactory.For(provider).SendMessageAsync(...)
6. Failover (если primary failed)
7. Refund (если final failure)
```

---

## Итоговая карта изменений

| Компонент | Изменение |
|---|---|
| `system_defaults` | Новая таблица |
| `stub_provider_config` | Новая таблица |
| `operator_prefixes` | Новая таблица |
| `tariff_plans` | +6 полей лимитов |
| `client_providers` | +`tps_limit` |
| `client_routes` | +`shared`, `client_id` nullable |
| `clients` | +`allocated_tps_budget` |
| `LimitResolver` | Новый компонент с Redis-кешем |
| `UnifiedRouter` | Новый, заменяет legacy `Router` и `CachedRouter` |
| `BackpressureManager` | Двухуровневый: global + per-client |
| `SenderFactory` | Новый, выбирает `SMPPSender` vs `StubSender` |
| `StubSender` | Выносится из `SMPPSender`, конфигурируем |
| Portal reseller API | Новые endpoints для управления субаккаунтами |
| Admin API | Новые endpoints для `system_defaults`, stub config |
| Router Stage | Переключается на `UnifiedRouter` |
| Sender Stage | Двухуровневый backpressure + `SenderFactory` |
| Миграция | Legacy `routes` → `client_routes` (скрипт + удаление) |

---

## Что не входит в этот дизайн

- Тарификация провайдеров (отдельная система, не затрагивается)
- Billing / пополнение баланса
- UI изменения (admin panel, portal frontend)
- HLR / smart routing стратегия (существующая логика не меняется)
