# Unified Hot Path: Currency via Operator Meta Lookup — Design

**Date:** 2026-04-19
**Related:** Phase 3 follow-up — closes should-fix #1 from [Phase 3 final review](../plans/2026-04-19-phase3-unified-pricing-hot-path.md#operator-runbook-post-implementation-2026-04-19).
**Status:** approved

## Problem

Unified hot path в `unified_path.go` hardcodes `currency := "RUB"` перед вызовом `saga.Charge`. Комментарий в коде утверждает, что non-RUB аккаунты "провалятся в legacy" — но это неверно: billing валидирует currency ПОСЛЕ charge-точки ([billing_service.go:477](../../../internal/services/billing/application/billing_service.go)), где fallback уже небезопасен. Для любого subaccount'а с account.Currency ≠ RUB unified путь вернёт fatal error вместо graceful downgrade.

Legacy путь резолвит currency через `tariff_plans` repo join: `operator_id → operators.country_id → countries.currency` ([tariff_plan_repository.go:51](../../../internal/services/tarification/infrastructure/repository/tariff_plan_repository.go)). Эта связь — единственный источник currency в системе, унификации с тариф-специфичной валютой нет. Unified должен наследовать ту же механику.

Текущее состояние БД (2026-04-19): 1 страна (RU/RUB), 47 клиентов все в RUB, 0 non-RUB аккаунтов. Фикс **упреждающий**, не срочный, но архитектурно необходимый до включения rollout > 1% в случае добавления второй страны.

## Decisions

### D1. Currency через расширенный operator lookup (Option A)

**Выбрано:** Переименовать `OperatorCodeLookup` → `OperatorMetaLookup`; вернуть `OperatorMeta{Code, Currency}` одним вызовом. SQL расширяется до `JOIN operators → countries`. Кеш — тот же map с новым value-типом.

**Отвергнуто:**
- **(B) `price_rules.currency` nullable column.** Требует миграцию + fallback-логику. Возможность per-rule override сейчас не нужна (YAGNI). Если понадобится — легко добавить поверх A.
- **(C) `resolved_rules.currency` денормализация.** Усложняет cache invalidation: при смене `countries.currency` нужно инвалидировать весь кеш для всех субаккаунтов этой страны. Сейчас invalidation привязан к `price_rules.id`; country-currency change — вне этой механики.

**Обоснование A:**
- **Та же семантика, что legacy.** Currency всегда = country's currency оператора. Нет нового поведения, нет дополнительной semantics для админа.
- **Минимум surface для drift.** Один SQL изменяется (добавление `JOIN`). Cache (`CachedOperatorLookup`) уже существует, просто расширяется value-тип.
- **Нет schema migration.** Откат — `git revert`.
- **Кеш имеет те же свойства, что для operator code:** currency страны, как и оператор code, — практически иммутабельная reference data. Инвалидация через рестарт сервиса — приемлема.

### D2. Пустая currency → fallback на legacy

Если `countries.currency` NULL (data integrity gap — оператор без страны) → `OperatorMeta.Currency = ""`. Hot path проверяет: пустой currency → `fallbackReason = "currency_resolve_error"`, метрика `unifiedFallbackTotal{reason="currency_resolve_error"}`. Legacy справится через свой `tariff_plans` JOIN, который имеет ту же проблему, но там уже есть вся логика обработки.

### D3. Idempotency short-circuit currency

При retry'е unified-tarified сообщения response должен включать правильный currency. Два варианта:
- **(i)** Лишний `Meta()` lookup в short-circuit. Cache всё равно на первом вызове. +1 fast call.
- **(ii)** Денормализовать currency в `tarification_log`. Schema migration.

**Выбрано (i)** — не добавлять migration ради echo-поля. Billing делает собственную validation на retry независимо; response-currency нужен только для клиента, не для billing semantics. Если будущий audit/reporting потребует retrospective currency без lookup'а — перейти к (ii) отдельным таском.

Обе точки short-circuit (`TarificationService.TarifyMessage` step 1 и `tarifyUnified` internal guard) вызывают `operatorLookup.Meta(ctx, existing.OperatorID)` и используют `meta.Currency`.

## Components

### Domain & types (`internal/services/tarification/application/operator_lookup.go`)

```go
type OperatorMeta struct {
    Code     string
    Currency string
}

type OperatorMetaLookup interface {
    Meta(ctx context.Context, operatorID uuid.UUID) (OperatorMeta, error)
}

type operatorMetaSource interface {
    GetMetaByID(ctx context.Context, id uuid.UUID) (OperatorMeta, error)
}

type CachedOperatorLookup struct {
    inner operatorMetaSource
    mu    sync.RWMutex
    cache map[uuid.UUID]OperatorMeta
}

func NewCachedOperatorLookup(inner operatorMetaSource) *CachedOperatorLookup
func (c *CachedOperatorLookup) Meta(ctx context.Context, id uuid.UUID) (OperatorMeta, error)
```

**Прежний метод `Code(ctx, id) (string, error)` удаляется.** Единственный call site — `unified_path.go`, переключается на `Meta()`. Interface переименован: `OperatorCodeLookup → OperatorMetaLookup`.

### Repository (`internal/services/tarification/infrastructure/repository/operator_repository.go`)

```go
const getMetaSQL = `
  SELECT o.code, COALESCE(c.currency, '')
  FROM operators o
  LEFT JOIN countries c ON c.id = o.country_id
  WHERE o.id = $1
`

func (r *OperatorRepository) GetMetaByID(ctx context.Context, id uuid.UUID) (application.OperatorMeta, error) {
    var m application.OperatorMeta
    err := r.db.QueryRowxContext(ctx, getMetaSQL, id).Scan(&m.Code, &m.Currency)
    if errors.Is(err, sql.ErrNoRows) {
        return application.OperatorMeta{}, domain.ErrOperatorNotFound
    }
    return m, err
}
```

`LEFT JOIN` + `COALESCE(c.currency, '')`: если у оператора `country_id IS NULL` или страна не найдена, возвращаем `Currency = ""` без ошибки. Вызывающий (`unified_path`) обрабатывает как fallback.

**Кросс-package reference:** repo возвращает тип из application-пакета. Это импорт-direction'ит infrastructure → application, что обычно ОК в этой кодовой базе (см. `pricing_period_repository.go` / `tariff_plan_repository.go` — тот же паттерн со Scan'ом в domain-типы). Если code-review флагнёт — альтернатива: держать `OperatorMeta` в `domain/`.

**Запасной вариант:** если `infrastructure → application` вызовет лишние зависимости, переместить `OperatorMeta` в `domain/` (одна структура без поведения). Решение финализируется в плане.

### `unified_path.go`

**Step 3 changes (meta lookup вместо Code):**
```go
meta, err := d.operatorLookup.Meta(ctx, req.OperatorID)
if err != nil {
    unifiedFallbackTotal.WithLabelValues("operator_lookup_error").Inc()
    log.Warn().Err(err).Str("operator_id", req.OperatorID.String()).
        Msg("unified: operator meta lookup failed — fallback")
    return nil, "operator_lookup_error", nil
}
if meta.Currency == "" {
    unifiedFallbackTotal.WithLabelValues("currency_resolve_error").Inc()
    log.Warn().Str("operator_id", req.OperatorID.String()).
        Msg("unified: operator has no currency — fallback")
    return nil, "currency_resolve_error", nil
}
operatorCode := meta.Code
currency := meta.Currency // заменяет hardcoded "RUB" в step 7
```

**Step 7 charge unchanged** — просто использует локальную `currency` вместо прежней `"RUB"` строки. Комментарий про hardcoded RUB удаляется.

**Idempotency short-circuit (top of tarifyUnified):**

Базируемся на nullable-plan ветке — `existing.TariffPlanID` и `existing.SourceRuleID` уже `*uuid.UUID`, и текущий короткий замыкание возвращает `SourceRuleID.String()` для unified replays с hardcoded `Currency: "RUB"`.

```go
// Replace the current hardcoded `Currency: "RUB"` with resolved currency.
// meta.Currency кешируется; один дополнительный fast lookup на replay.
replayCurrency := "RUB" // fallback если meta-lookup падает; replay-ответ не money-критичен
if meta, metaErr := d.operatorLookup.Meta(ctx, existing.OperatorID); metaErr == nil && meta.Currency != "" {
    replayCurrency = meta.Currency
}
tariffPlanID := ""
switch {
case existing.TariffPlanID != nil:
    tariffPlanID = existing.TariffPlanID.String()
case existing.SourceRuleID != nil:
    tariffPlanID = existing.SourceRuleID.String()
}
return &TarifyMessageResponse{
    Approved:     true,
    TotalAmount:  existing.TotalAmount,
    Currency:     replayCurrency,
    Strategy:     string(existing.Strategy),
    TariffPlanID: tariffPlanID,
}, "", nil
```

### TarificationService (`tarification_service.go`)

**Step 1 short-circuit** (currently hardcodes `"RUB"` for unified replay) — такая же замена: вызов `s.unifiedDeps.operatorLookup.Meta(ctx, existing.OperatorID)`, fallback на `"RUB"` если ошибка.

**SetUnifiedDependencies signature:** parameter type `OperatorCodeLookup` → `OperatorMetaLookup`. Все call sites внутри этого change.

### Wiring (`cmd/services/tarification-service/main.go`)

```go
operatorRepo := tarificationrepo.NewOperatorRepository(dbx)
operatorLookup := application.NewCachedOperatorLookup(operatorRepo)
// ↑ тип: *CachedOperatorLookup, имплементит OperatorMetaLookup
tarificationService.SetUnifiedDependencies(
    true, rollout, priceResolver, costCalc,
    priceRuleRepo, subUsageRepo, operatorLookup,
)
```

Конструктор и wiring не меняются по форме — только value-тип, возвращаемый `NewCachedOperatorLookup`, теперь кеширует `OperatorMeta` вместо `string`.

## Errors

| Scenario | Result |
|---|---|
| `operator_id` не найден | `domain.ErrOperatorNotFound` → `fallbackReason="operator_lookup_error"` + existing metric |
| Оператор найден, `country_id IS NULL` или country.currency пусто | `OperatorMeta{Code: "...", Currency: ""}` → `fallbackReason="currency_resolve_error"` + new metric label |
| DB ошибка на `GetMetaByID` | Propagated; в hot path → fallback (`operator_lookup_error`) |
| Account.Currency != meta.Currency | Billing вернёт error после charge — fatal (уже post-charge); surface как `err` в caller. Legacy behavior, не меняется |

## Testing

**Unit — `operator_lookup_test.go`** (rename existing):
- `TestCachedOperatorLookup_HitAfterFirstCall` — 10 calls, 1 DB hit, возвращает `OperatorMeta{Code:"mts-ru", Currency:"RUB"}`.
- `TestCachedOperatorLookup_MissForNewID`.
- `TestCachedOperatorLookup_ErrorNotCached`.
- `TestCachedOperatorLookup_ConcurrentNoRace` (race-detector).
- Стабилизируем `stubOpRepo` на возврат `OperatorMeta`.

**Unit — `unified_path_test.go`**:
- Stub `stubOperatorLookup.Meta()` возвращает `OperatorMeta{Code:"mts-ru", Currency:"RUB"}` в happy-path.
- `TestTarifyUnified_HappyPathApproves` — assert `resp.Currency == "RUB"`.
- `TestTarifyUnified_EmptyCurrency_Fallback` (new) — stub возвращает `Currency=""` → `fallbackReason="currency_resolve_error"`, `unifiedFallbackTotal{reason="currency_resolve_error"}` delta == 1.
- `TestTarifyUnified_IdempotencyShortCircuits` — extended: stub также должен вернуть meta при replay'е; assert `resp.Currency == meta.Currency`.

**Repo integration** — не пишем (SQL тривиальный `LEFT JOIN`; validated on sandbox).

**Sandbox e2e** — после deploy проверить логи tarification-service на startup: не fatal'ит. Т.к. `unified_enabled=false` в sandbox, полный hot-path без отдельного test-rig не проверить. Достаточно для коммита; включение — отдельная ops-задача.

## Scope / YAGNI

- ✅ Один SQL JOIN, type rename.
- ✅ Новая метрика-label, не struct-поле.
- ✅ Replay extra-lookup — компромисс между schema-change и корректностью.
- ❌ Per-rule currency override — отвергнуто (YAGNI).
- ❌ Currency denormalization в `resolved_rules`/`tarification_log` — отвергнуто (schema-migration без value today).
- ❌ Currency conversion / multi-account per client — вне scope'а tarification; это billing decision.

## Follow-ups

- Если audit/reporting потребует retrospective currency без `operator_lookup` — мигрировать к (ii): `tarification_log.currency VARCHAR(3)` column.
- Если понадобится per-operator currency override (промо, seasonal pricing) — добавить `price_rules.currency` nullable column (D1 option B).
- Ops runbook: при добавлении второй страны — проверить что её `currency` не NULL в `countries`, иначе unified hot path будет fallback'ить для всех её операторов.

## Plan outline

~5 задач:
1. Domain rename: `OperatorCodeLookup → OperatorMetaLookup`, add `OperatorMeta` type, rename `Code()` method to `Meta()`. Tests updated (stubs return `OperatorMeta`).
2. Repository: `GetMetaByID` with `LEFT JOIN countries`, returns `OperatorMeta`.
3. CachedOperatorLookup: `map[uuid.UUID]OperatorMeta`, `Meta()` method.
4. unified_path: use `meta.Currency`, drop hardcoded RUB; add `currency_resolve_error` fallback branch; fix short-circuit. Tests updated.
5. tarification_service: short-circuit uses `meta.Currency` with RUB fallback. SetUnifiedDependencies arg type rename.

Wiring в main.go не меняется (type inference, NewCachedOperatorLookup signature unchanged).

## Self-review

- **Placeholder scan:** no TBD/TODO/fill-in.
- **Consistency:** `OperatorMeta{Code, Currency}` используется одинаково во всех секциях. `OperatorMetaLookup.Meta(ctx, uuid) (OperatorMeta, error)` — одна сигнатура.
- **Scope:** single backend fix. No frontend, no migration. Focused.
- **Ambiguity:** "где живёт `OperatorMeta`" — явно сказано, `application/`; альтернатива (domain/) оговорена как запасной вариант, финализация — в плане.
