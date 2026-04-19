# `tarification_log` nullable plan/period + source_rule_id — Design

**Date:** 2026-04-19
**Related:** `docs/superpowers/plans/2026-04-19-phase3-unified-pricing-hot-path.md` — Phase 3 must-fix #1 from final branch review.
**Status:** approved

## Problem

`tarification_log.tariff_plan_id` и `.tariff_period_id` сейчас `UUID NOT NULL` ([migration 000013:101-102](../../../migrations/000013_create_tarification_tables.up.sql)). Phase 3 unified hot path пишет строки в эту же таблицу, но у него нет концепции `tariff_plan`/`tariff_period` — вся информация о ценообразовании в `price_rules`. Текущий хак: передаём `uuid.Nil` ([unified_path.go:191](../../../internal/services/tarification/application/unified_path.go)). NOT NULL проходит (zero UUID валиден), но:

1. **UX-bug в детализации:** API-хендлеры ([portal/admin detalization](../../../internal/gateway/portal/handlers/detalization.go#L478), [admin](../../../internal/gateway/admin/handlers/detalization.go#L315)) возвращают `tariff_plan_id::text` — пользователь видит `"00000000-0000-0000-0000-000000000000"` в UI.
2. **Сломан audit trail:** при вопросе "почему этот клиент получил именно такую цену?" для unified-строк нет link'а на `price_rules.id`. Forensics при инцидентах невозможен.
3. **Блокер 100% rollout:** на 1% rollout — 1% "безплановых" строк (косметический bug). На 100% — все строки без plan-link, UI и audit сломаны повсеместно.

FK constraint на `tariff_plan_id → tariff_plans(id)` в таблице **отсутствует** (только `NOT NULL`). Поэтому проблема семантическая, а не целостности.

## Decisions

### D1. Фикс через nullable + новая колонка `source_rule_id`

**Выбрано:**
```sql
ALTER TABLE tarification_log
    ALTER COLUMN tariff_plan_id DROP NOT NULL,
    ALTER COLUMN tariff_period_id DROP NOT NULL,
    ADD COLUMN source_rule_id UUID NULL;
```

Legacy путь пишет: `plan_id, period_id, source_rule_id=NULL`.
Unified путь пишет: `plan_id=NULL, period_id=NULL, source_rule_id=<rr.SourceRuleID>`.

**Отвергнуто:**
- **Отдельная таблица `unified_tarification_log`** — overkill. Phase 4 (cleanup) удалит legacy-путь; все новые строки станут unified; иметь две таблицы на время transition бессмысленно. После Phase 4 схема одноколоночная (`source_rule_id` всегда заполнен, plan/period всегда NULL) — тогда можно `ALTER DROP COLUMN` в отдельной миграции.
- **Sentinel row в `tariff_plans`** (reserved UUID для unified) — маскирует истину, никакого audit value.

### D2. Нет FK `source_rule_id → price_rules.id`

`price_rules` может удаляться/архивироваться (см. `DELETE` в repo). Hard FK заблокирует удаление правил или каскадно обнулит historical log rows — оба варианта плохие. Без FK — log хранит snapshot, правило может уйти, идентификатор остаётся.

### D3. Сlient side: frontend/API behaviour

- API возвращает `tariff_plan_id: null` (вместо zero-строки) когда поле NULL.
- Добавляется новое поле в response: `source_rule_id: <uuid> | null`.
- Frontend UI обрабатывает: если `tariff_plan_id=null && source_rule_id!=null` — показывает "Unified pricing (rule: <short_id>)".
- Frontend-часть **не входит в scope этого спека** (отдельная frontend-таска). Backend меняет контракт — frontend должен быть обновлён до рассылки. В плане отметим как cross-team dependency.

### D4. Rollback стратегия

Down-migration:
```sql
-- WARNING: если в tarification_log есть NULL plan_id (unified-строки),
-- DROP NOT NULL восстановить нельзя без чистки.
DELETE FROM tarification_log WHERE tariff_plan_id IS NULL; -- uncomment if needed
ALTER TABLE tarification_log
    ALTER COLUMN tariff_plan_id SET NOT NULL,
    ALTER COLUMN tariff_period_id SET NOT NULL,
    DROP COLUMN source_rule_id;
```

Unified-строки теряются при откате. Для песочницы/staging приемлемо. В проде откат = инцидент-уровень решение, документируется.

## Components

### Migration `000105_tarification_log_nullable_plan.up.sql` + `.down.sql`

`.up.sql`:
```sql
ALTER TABLE tarification_log
    ALTER COLUMN tariff_plan_id DROP NOT NULL,
    ALTER COLUMN tariff_period_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS source_rule_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_tarification_log_source_rule
    ON tarification_log(source_rule_id)
    WHERE source_rule_id IS NOT NULL;
```

Партиционированная таблица: `ALTER TABLE` на parent автоматически применяется к partitions в PostgreSQL 12+ (проект использует 15+). Проверить в плане shell-step'ом `\d+ tarification_log` на staging.

### Domain (`internal/services/tarification/domain/tarification_log.go`)

```go
type TarificationLog struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	MessageID       uuid.UUID
	OperatorID      uuid.UUID
	SenderCategory  SenderCategory
	Strategy        TarificationStrategy
	TariffPlanID    *uuid.UUID // было uuid.UUID; теперь nullable — NULL для unified
	TariffPeriodID  *uuid.UUID // было uuid.UUID; теперь nullable — NULL для unified
	SourceRuleID    *uuid.UUID // NULL для legacy; заполнено для unified
	SegmentCount    int
	PricePerSegment string
	TotalAmount     string
	RecalcAmount    *string
	IdempotencyKey  string
	CreatedAt       time.Time
}

func NewTarificationLog(
	clientID, messageID, operatorID, tariffPlanID, tariffPeriodID uuid.UUID,
	// ... как раньше, но внутри оборачиваем plan/period в &uuid.UUID
) *TarificationLog

func NewUnifiedTarificationLog(
	clientID, messageID, operatorID, sourceRuleID uuid.UUID,
	senderCategory SenderCategory,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog
```

### Repository (`infrastructure/repository/tarification_log_repository.go`)

- Insert SQL: `VALUES (..., $N, $N+1, $N+2)` — все три поля поддерживают NULL.
- Scan methods — менять сигнатуру `Scan` на `*uuid.UUID` для трёх полей. Конкретные implementation-детали в plan'е.
- 3 SELECT (строки 51/80/etc) — добавить `source_rule_id` в column list.

### Application (`tarification_service.go` и `unified_path.go`)

1. `TarifyMessage` шаг 1 (idempotency short-circuit): `s.planRepo.GetByID(ctx, existing.TariffPlanID)` — проверять `existing.TariffPlanID == nil`, при nil skip plan lookup, вернуть пустой Currency или использовать default.
2. `unified_path.go:191` — заменить `domain.NewTarificationLog(..., uuid.Nil, uuid.Nil, category, domain.StrategyUnified, ...)` на `domain.NewUnifiedTarificationLog(..., rr.SourceRuleID, category, ...)`.
3. Response struct `TarifyMessageResponse.TariffPlanID string` — при nil plan_id в лог-entry, возвращаем `rr.SourceRuleID.String()` (как сейчас в unified) или пустую строку (в legacy-idempotency-shortcircuit для unified-записей).

### Handlers (`portal/handlers/detalization.go`, `admin/handlers/detalization.go`)

- SELECT column list: `tariff_plan_id::text` → оставить как есть; scan в `*string` (nullable).
- Добавить `source_rule_id::text` в SELECT.
- JSON response: если plan_id nil, ключ `tariff_plan_id: null`; если source_rule_id не nil, ключ `source_rule_id: <uuid>`.
- `margin_report_repo`: проверить — если plan_id там не используется, не трогать.

## Errors

- `tarification_log_repository.Create` с nil plan_id: INSERT со `sql.NullString` → NULL в БД. Driver поддерживает.
- `GetByIdempotencyKey` для unified-строки: возвращает TarificationLog с nil plan_id. Caller (`TarifyMessage` шаг 1) обрабатывает.

## Testing

**Миграция:**
- Apply up → verify `\d+ tarification_log` показывает nullable и `source_rule_id`.
- Insert legacy-строка (plan+period filled, source_rule_id=NULL) — success.
- Insert unified-строка (plan=NULL, period=NULL, source_rule_id=UUID) — success.
- Down с NULL строками → expected failure (documented).

**Domain:**
- `NewTarificationLog` — plan/period non-nil.
- `NewUnifiedTarificationLog` — plan/period nil, source_rule_id set, Strategy=StrategyUnified.

**Repository:**
- Create+Get roundtrip для обеих форм.

**Integration:**
- TarifyMessage через unified path → лог строка с nil plan, nil period, source_rule_id=rule_id.
- TarifyMessage через legacy path → лог строка с plan, period, nil source_rule_id.
- Idempotency retry для unified — TarifyMessage возвращает approved без plan lookup.

**Handler contract test:**
- detalization API возвращает `tariff_plan_id: null, source_rule_id: "<uuid>"` для unified записи.
- detalization API возвращает `tariff_plan_id: "<uuid>", source_rule_id: null` для legacy.

## Scope / YAGNI

- ✅ Nullable plan/period в одной таблице + source_rule_id.
- ✅ Обновление handler'ов backend.
- ❌ Frontend UI-изменение — отдельная задача (cross-team).
- ❌ FK на price_rules — сознательно не добавляем (см. D2).
- ❌ Отдельная таблица — отвергнуто (см. D1).
- ❌ Миграция `margin_report_repo` — не использует plan/period колонки (проверить в плане).

## Follow-ups

- **Phase 4 cleanup:** когда legacy удаляется, ВСЕ строки становятся unified → plan_id, period_id навсегда NULL. Вторая миграция может `DROP COLUMN` их и переименовать `source_rule_id`.
- **Аналитический backfill:** исторические legacy-строки не имеют source_rule_id. Если нужна унифицированная аналитика, необходим маппинг `(plan_id, period_id) → synthetic rule_id` для старых данных. Не делаем сейчас.
- **Frontend таска:** UI показывает "Unified pricing (rule: <short_id>)" для unified-записей.

## Plan outline

Приблизительно 7 задач:
1. Миграция up/down + smoke-check `\d+`.
2. Domain: change fields to pointers, add `NewUnifiedTarificationLog`, update legacy constructor.
3. Repository: update Insert/Scan/Select signatures, tests.
4. TarificationService idempotency guard for nil plan_id.
5. unified_path.go: use new constructor.
6. Handlers: detalization + admin, nullable scan + source_rule_id in response.
7. End-to-end test (legacy & unified path).

## Self-review

- **Placeholder scan:** нет TBD/TODO.
- **Consistency:** `TariffPlanID *uuid.UUID`, `TariffPeriodID *uuid.UUID`, `SourceRuleID *uuid.UUID` — одинаково описаны в Domain и Repository секциях.
- **Scope:** единый backend-фикс. Frontend вынесен explicit как cross-team dependency.
- **Ambiguity:** "как возвращать `rr.SourceRuleID` в response" — явно: всегда в `rr.SourceRuleID.String()` для success path unified, пустая строка для unified-shortcircuit legacy request.
