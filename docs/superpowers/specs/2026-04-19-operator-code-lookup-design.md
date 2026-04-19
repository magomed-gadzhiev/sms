# Operator ID → Code Lookup (Phase 3 Prerequisite) — Design

**Date:** 2026-04-19
**Related plan:** `docs/superpowers/plans/2026-04-19-phase3-unified-pricing-hot-path.md`
**Status:** approved

## Problem

`TarifyMessageRequest.OperatorID` — UUID. `price_rules.operator` — `VARCHAR(50) REFERENCES operators(code)` (строковый код, e.g. `"mts-ru"`). Без маппинга unified hot path будет передавать UUID-строку в `ResolveInput.Operator`, SQL `operator = $4` не совпадёт ни с одной записью → `ErrNoApplicableRule` → 100% fallback на legacy при любом rollout_percentage > 0. Ложный сигнал "unified работает" — метрика `price_not_found_total` будет спамить алерт, но реальной проблемы ценообразования нет, сломан только маппинг. Это блокер Phase 3.

Параллельная проблема: `TarifyMessageRequest` не содержит `trafficType`. В унифицированной модели `price_rules.traffic_type` — nullable (NULL = wildcard). Нужно явное решение, какое значение передавать в `ResolveInput`.

## Decisions

### D1. Operator lookup — вариант A (in-process lookup в tarification-service)

**Выбрано:** собственный тонкий repo `OperatorRepository.GetCodeByID(ctx, uuid) (string, error)` в infrastructure tarification-сервиса, плюс in-memory cache в application-слое.

**Отвергнуто:**
- (B) расширение gRPC-контракта `TarifyMessageRequest` полем `operator_code` — breaking change, 3+ callsite (sender pipeline, cascade integration, worker). Не оправдано для Phase 3 (cutover чтения, не изменение контракта).
- (C) денормализация `operator_code` по всему пайплайну — оверкилл, требует изменений в routing.

**Обоснование A:**
- Операторы — справочник (~десятки записей), код иммутабелен.
- Без TTL: инвалидация через рестарт сервиса приемлема, т.к. добавление оператора — редкая админ-операция и сопровождается редеплоем.
- Нулевой overhead в стационаре (после прогрева).
- Не нарушает изоляцию сервисов: таблица `operators` — shared-reference-data, FK из `price_rules.operator` уже делает её неявной зависимостью tarification.

### D2. TrafficType — вариант A (пустая строка)

**Выбрано:** `ResolveInput.TrafficType = ""` в обоих resolve-вызовах (subaccount и margin).

**Поведение:** `price_rules` seed-ом создаётся с `traffic_type = NULL`. SQL `(traffic_type IS NULL OR traffic_type = '')` матчит wildcard-правила. Специфичные правила "только transactional" не работают — их сейчас нет.

**Отвергнуто:**
- Хардкод `"transactional"` — ложный сигнал корректности, когда появится promo-трафик тихо тарифицируем его не по адресу.
- Добавление поля в `TarifyMessageRequest` — вне scope'а Phase 3.

**Follow-up (не в этом спеке):** когда разделим transactional/promo пайплайны (Phase 5 или отдельный таск), добавить `traffic_type` в request и в proto.

## Components

### `infrastructure/repository/operator_repository.go`

```go
type OperatorRepository struct { db *sqlx.DB }

func NewOperatorRepository(db *sqlx.DB) *OperatorRepository

// GetCodeByID возвращает operators.code для данного UUID.
// Если оператор не найден — возвращает domain.ErrOperatorNotFound.
func (r *OperatorRepository) GetCodeByID(ctx context.Context, id uuid.UUID) (string, error)
```

SQL: `SELECT code FROM operators WHERE id = $1`. Одна таблица, один запрос.

### `application/operator_lookup.go`

```go
type OperatorCodeLookup interface {
    Code(ctx context.Context, operatorID uuid.UUID) (string, error)
}

type CachedOperatorLookup struct {
    inner operatorCodeSource // interface: GetCodeByID
    mu    sync.RWMutex
    cache map[uuid.UUID]string
}

func NewCachedOperatorLookup(inner operatorCodeSource) *CachedOperatorLookup
func (c *CachedOperatorLookup) Code(ctx context.Context, id uuid.UUID) (string, error)
```

**Поведение:**
- Первый вызов для ID — `inner.GetCodeByID`, запись в map.
- Повторный — из map под RLock.
- Ошибка репо — пробрасывается вызывающему, кеш не трогается (не отравляется).

**Concurrent coldmiss:** Без singleflight. При одновременном coldmiss один и тот же UUID выполнит N параллельных SQL — приемлемо: операторов десятки, прогрев за секунды. Усложнение singleflight'ом не оправдано.

### Hot path integration

В `unified_path.go` (Task 8 плана Phase 3) перед построением `ResolveInput`:

```go
operatorCode, err := d.operatorLookup.Code(ctx, req.OperatorID)
if err != nil {
    unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
    log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
        Msg("unified: operator code lookup failed — fallback")
    return nil, "operator_lookup_error", nil
}

in := domain.ResolveInput{
    SubaccountID:   req.ClientID,
    Country:        "",          // не задействовано в Phase 3
    Operator:       operatorCode,
    SenderCategory: string(category),
    TrafficType:    "",          // wildcard, см. D2
    Now:            time.Now().UTC(),
}
```

### Wiring (`cmd/services/tarification-service/main.go`)

```go
operatorRepo := tarificationrepo.NewOperatorRepository(dbx)
operatorLookup := application.NewCachedOperatorLookup(operatorRepo)
tarificationService.SetUnifiedDependencies(
    true, rollout, priceResolver, costCalc,
    priceRuleRepo, subUsageRepo,
    operatorLookup, // ← новый аргумент
)
```

## Errors

- `domain.ErrOperatorNotFound` (новая, в `domain/errors.go`) — сигнализирует unknown UUID. Вызывающий (`tarifyUnified`) трактует как non-fatal: fallback на legacy, метрика.
- Любая другая ошибка БД из `GetCodeByID` — тоже fallback на legacy (legacy умеет работать с UUID-based lookup через `operator_id` в `tariff_plans`).

## Testing

**Unit (`operator_lookup_test.go`):**
1. Cache hit после первого call — ровно 1 вызов `inner.GetCodeByID`.
2. Miss для нового UUID — второй DB call.
3. Ошибка `inner` пробрасывается, следующий вызов не попадает в кеш (повторный retry ещё раз идёт в DB).
4. Concurrent calls для одного UUID не портят map (race detector должен пройти).

**Repo (опционально, если время):** unit с `sqlmock` или integration — не критично, SQL тривиальный.

**Hot-path tests (Task 8 плана):** добавить `stubOperatorLookup` в `unified_path_test.go`:
- `TestTarifyUnified_OperatorLookupError_Fallback`: lookup возвращает ошибку → `fallbackReason="operator_lookup_error"`, `resp=nil`, `err=nil`.
- Hard-wire в happy-path tests: lookup возвращает `"mts-ru"`, и `fakeRuleRepo.byLookup.Operator == "mts-ru"`.

## Scope / YAGNI

- ✅ In-process cache без TTL.
- ✅ Fallback reason `operator_lookup_error` в существующем `unifiedFallbackTotal` (новая метка, CounterVec уже есть).
- ❌ Нет: gRPC-альтернативы, TTL, singleflight, инвалидации по pubsub, метрики cache hit rate оператор-lookup'а (десятки ключей, бесполезно).

## Plan impact

Добавляется **Task 0** перед Task 1 основного плана Phase 3:
- Создать repo + lookup + тесты.
- Зафиксировать commit.

Изменения в существующих тасках:
- **Task 8** (`unified_path.go`): `unifiedDeps.operatorLookup OperatorCodeLookup`, вызов перед `ResolveInput`. Список test scenarios расширяется одним (operator_lookup_error).
- **Task 9** (`tarification_service.go`): `SetUnifiedDependencies` принимает `operatorLookup` доп. аргументом.
- **Task 10** (`main.go`): конструирует `operatorRepo` + `operatorLookup`, передаёт в setter.

Метрика `tarification_unified_fallback_total{reason}` в Task 6 уже CounterVec — новая метка не требует изменений схемы метрик.

## Self-review

- **Placeholder scan:** нет TBD/TODO.
- **Consistency:** `OperatorCodeLookup` интерфейс одинаково описан в компоненте и в integration-секции. `fallback_total` reason совпадает во всех местах (`operator_lookup_error`).
- **Scope:** один файл-компонент + одно изменение request-contract'а (его нет). Focused.
- **Ambiguity:** `ErrOperatorNotFound` — новая; спек явно говорит создать в `domain/errors.go`.
