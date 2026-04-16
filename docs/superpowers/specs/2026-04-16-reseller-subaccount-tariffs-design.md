# Reseller Sub-Account Tariffs — Design Spec

**Date:** 2026-04-16
**Status:** Approved
**Approach:** A — Отдельная модель данных для субаккаунтов

## Summary

Переработка системы тарификации субаккаунтов реселлера. Текущая плоская модель (`aggregator_tariffs`: оператор × категория → цена) заменяется полноценной системой со стратегиями тарификации, периодами и объёмными уровнями (tiers) — аналогично платформенной тарификации. Добавляются шаблоны тарифов с живой связью и переопределения на уровне субаккаунта.

## Requirements

- 4 стратегии: `fixed`, `threshold`, `threshold_recalc`, `prepaid_threshold`
- Периоды (date ranges) с запретом пересечений
- Уровни (tiers) — объёмные пороги с ценой за сегмент
- Полный набор измерений: страна, оператор, категория отправителя, тип трафика
- Шаблоны тарифов (живая связь — изменения автоприменяются)
- Наследование: override субаккаунта → шаблон → legacy fallback
- Обратная совместимость с `aggregator_tariffs`
- Копирование тарифов между субаккаунтами сохраняется

---

## 1. Data Model

### 1.1 reseller_tariff_templates

Именованные шаблоны тарифов реселлера.

```sql
CREATE TABLE reseller_tariff_templates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID        NOT NULL REFERENCES clients(id),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    active      BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reseller_template_unique_name
    ON reseller_tariff_templates (reseller_id, name) WHERE active = true;
```

### 1.2 reseller_tariff_plans

Тарифные планы — принадлежат шаблону (template_id) или субаккаунту напрямую (sub_account_id) как переопределение.

```sql
CREATE TABLE reseller_tariff_plans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id     UUID        NOT NULL REFERENCES clients(id),
    template_id     UUID        REFERENCES reseller_tariff_templates(id),
    sub_account_id  UUID        REFERENCES clients(id),
    country_id      UUID        REFERENCES countries(id),
    operator_id     UUID        REFERENCES operators(id),
    sender_category VARCHAR(32) NOT NULL DEFAULT 'standard',
    traffic_type    VARCHAR(32) NOT NULL DEFAULT 'any',
    strategy        VARCHAR(30) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active          BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (template_id IS NOT NULL AND sub_account_id IS NULL) OR
        (template_id IS NULL AND sub_account_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_reseller_plan_template_dims
    ON reseller_tariff_plans (template_id, COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid), sender_category, traffic_type)
    WHERE active = true AND template_id IS NOT NULL;

CREATE UNIQUE INDEX idx_reseller_plan_sub_account_dims
    ON reseller_tariff_plans (sub_account_id, COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid), sender_category, traffic_type)
    WHERE active = true AND sub_account_id IS NOT NULL;

CREATE INDEX idx_reseller_plan_template ON reseller_tariff_plans (template_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_sub_account ON reseller_tariff_plans (sub_account_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_reseller ON reseller_tariff_plans (reseller_id);
```

### 1.3 reseller_tariff_periods

Периоды с запретом пересечений внутри плана.

```sql
CREATE TABLE reseller_tariff_periods (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_plan_id UUID NOT NULL REFERENCES reseller_tariff_plans(id) ON DELETE CASCADE,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_date > start_date)
);

ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_periods_no_overlap
    EXCLUDE USING gist (tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_reseller_period_plan ON reseller_tariff_periods (tariff_plan_id);
CREATE INDEX idx_reseller_period_dates ON reseller_tariff_periods (tariff_plan_id, start_date, end_date);
```

### 1.4 reseller_tariff_tiers

Объёмные уровни цен внутри периода.

```sql
CREATE TABLE reseller_tariff_tiers (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_period_id UUID        NOT NULL REFERENCES reseller_tariff_periods(id) ON DELETE CASCADE,
    from_count       INTEGER     NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(10,6) NOT NULL CHECK (price_per_segment >= 0)
);

CREATE UNIQUE INDEX idx_reseller_tier_unique ON reseller_tariff_tiers (tariff_period_id, from_count);
```

### 1.5 sub_account_template_assignments

Привязка субаккаунта к шаблону (один шаблон на субаккаунт).

```sql
CREATE TABLE sub_account_template_assignments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_account_id UUID        NOT NULL REFERENCES clients(id),
    template_id    UUID        NOT NULL REFERENCES reseller_tariff_templates(id),
    assigned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_sub_account_template_unique ON sub_account_template_assignments (sub_account_id);
```

### 1.6 Lookup Priority

При тарификации сообщения субаккаунта:

1. **Override** — `reseller_tariff_plans WHERE sub_account_id = X AND active = true` → matching period → tier
2. **Template** — `sub_account_template_assignments` → `reseller_tariff_plans WHERE template_id = Y AND active = true` → matching period → tier
3. **Legacy fallback** — `aggregator_tariffs WHERE sub_account_id = X OR sub_account_id IS NULL`

Matching по измерениям: country_id, operator_id, sender_category, traffic_type. Более специфичный план (больше заполненных измерений) имеет приоритет.

---

## 2. Backend API

### 2.1 Templates

| Method | Path | Description |
|--------|------|-------------|
| GET | `/portal/v1/reseller/tariff-templates` | Список шаблонов реселлера |
| POST | `/portal/v1/reseller/tariff-templates` | Создать шаблон |
| PUT | `/portal/v1/reseller/tariff-templates/{id}` | Обновить имя/описание |
| DELETE | `/portal/v1/reseller/tariff-templates/{id}` | Деактивировать (soft delete) |
| POST | `/portal/v1/reseller/tariff-templates/{id}/assign` | Привязать субаккаунт к шаблону |
| DELETE | `/portal/v1/reseller/tariff-templates/{id}/assign/{sub_account_id}` | Отвязать субаккаунт |

### 2.2 Plans

| Method | Path | Description |
|--------|------|-------------|
| GET | `/portal/v1/reseller/tariff-plans?template_id=...&sub_account_id=...` | Список планов (фильтр) |
| POST | `/portal/v1/reseller/tariff-plans` | Создать план |
| PUT | `/portal/v1/reseller/tariff-plans/{id}` | Обновить стратегию/измерения |
| DELETE | `/portal/v1/reseller/tariff-plans/{id}` | Деактивировать план |

### 2.3 Periods & Tiers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/portal/v1/reseller/tariff-plans/{plan_id}/periods` | Периоды плана |
| POST | `/portal/v1/reseller/tariff-plans/{plan_id}/periods` | Создать период |
| PUT | `/portal/v1/reseller/tariff-periods/{id}` | Обновить даты |
| DELETE | `/portal/v1/reseller/tariff-periods/{id}` | Удалить период (каскадно удалит тиеры) |
| GET | `/portal/v1/reseller/tariff-periods/{period_id}/tiers` | Тиеры периода |
| POST | `/portal/v1/reseller/tariff-periods/{period_id}/tiers` | Создать/обновить тиеры (bulk upsert) |

### 2.4 Overview & Copy

| Method | Path | Description |
|--------|------|-------------|
| GET | `/portal/v1/reseller/tariff-overview?sub_account_id=...` | Resolved effective prices matrix |
| POST | `/portal/v1/reseller/tariff-plans/copy` | Копировать планы между субаккаунтами/шаблонами |

### 2.5 Backend Structure

- New handler: `ResellerTariffPlanHandlers` in `internal/gateway/portal/handlers/reseller_tariff_plans.go`
- All endpoints verify `is_reseller` and sub-account ownership
- Direct SQL via pgx (project pattern, no ORM)

---

## 3. Frontend UX

### 3.1 Page Structure

`/network/tariffs` — три таба (Radix Tabs):

**Tab 1: «Обзор»** (default)
- Матрица оператор × категория, read-only
- Показывает эффективную цену для выбранного субаккаунта (resolved lookup)
- Бейджи источника: «Шаблон» / «Переопределение» / «Legacy» / «Не задано»
- Клик по ячейке → навигация к редактированию соответствующего плана

**Tab 2: «Шаблоны»**
- Список шаблонов (карточки/таблица)
- CRUD шаблона (имя, описание)
- Привязка субаккаунтов (multi-select modal)
- Drill-down: шаблон → планы → периоды → тиеры

**Tab 3: «Переопределения»**
- Выбор субаккаунта → список override-планов
- Создание/редактирование override-плана

### 3.2 Tariff Plan Editor (shared component)

Используется в шаблонах и переопределениях:

1. **Header**: dropdowns для измерений (страна, оператор, категория, тип трафика) + стратегия
2. **Periods section**: таблица периодов (start — end), «Добавить период»
3. **Tiers section** (nested in period): таблица «от X сегментов → цена Y»
   - `fixed`: один тиер (from_count=0), UI скрывает from_count
   - `threshold*`: несколько строк, кнопка добавления
4. **Validation**: пересечение дат, монотонность from_count, обязательность тиера

### 3.3 Bulk Price Assignment

Диалог «Единая цена» создаёт `fixed`-планы (один тиер, бессрочный период) для всех операторов выбранной категории.

### 3.4 Component Files

```
portal-frontend/src/pages/network/
  NetworkTariffsPage.tsx           ← refactor: tabs
  components/
    TariffOverviewTab.tsx          ← resolved prices matrix
    TariffTemplatesTab.tsx         ← template CRUD
    TariffOverridesTab.tsx         ← sub-account overrides
    TariffPlanEditor.tsx           ← plan + periods + tiers editor
    TariffPeriodForm.tsx           ← period form
    TariffTierTable.tsx            ← inline tier editing
    TemplateAssignModal.tsx        ← assign sub-accounts to template
```

---

## 4. Migration & Backward Compatibility

### 4.1 Data Migration Strategy

- `aggregator_tariffs` table **remains untouched** — serves as legacy fallback
- **No automatic migration** — resellers adopt the new system manually via UI
- Overview endpoint resolves both sources: new system takes priority when available

### 4.2 Transition Flow

1. Reseller sees current prices in «Обзор» tab (marked as «Legacy»)
2. Creates template, adds plans with periods and tiers
3. Assigns sub-accounts to template
4. Overview updates — cells switch from «Legacy» to «Шаблон»
5. Optionally adds overrides for specific sub-accounts

### 4.3 Old API

`/portal/v1/reseller/tariffs` endpoints **remain operational** for backward compatibility. New endpoints run in parallel.

---

## 5. Validation & Edge Cases

### 5.1 Database Constraints

- Period overlap: `EXCLUDE USING gist` per plan
- Tier uniqueness: unique index on `(tariff_period_id, from_count)`
- One template per sub-account: unique index on `sub_account_id`
- Plan ownership: CHECK — exactly one of `template_id` / `sub_account_id` is NOT NULL

### 5.2 API Validation

- `fixed` strategy: exactly one tier with `from_count = 0`
- `threshold*` strategies: at least one tier with `from_count = 0`, monotonically increasing `from_count`
- Period dates: `end_date > start_date`, format `YYYY-MM-DD`
- Ownership: all mutations verify `is_reseller` + sub-account/template belongs to current reseller
- Template deletion: soft delete (`active = false`), assigned sub-accounts fall back to legacy

### 5.3 Edge Cases

| Situation | Behavior |
|-----------|----------|
| Sub-account without template or override | Fallback to `aggregator_tariffs` |
| Template deactivated | Sub-accounts automatically fall back to legacy |
| No active period for current date | Fallback to next level (template → legacy) |
| Override + template for same dimension combo | Override wins |
| Copy from sub-account with override plans | Copies as override plans for target (with periods and tiers) |
| Bulk assignment over existing plans | Upserts matching plans by dimensions |

### 5.4 Out of Scope

- `usage_counters` for reseller-level threshold strategies — first phase uses platform-level volume data
- Audit log for tariff changes (can be added later)
- Notifications to sub-accounts on tariff changes
