# Unified Pricing Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить три параллельные модели тарификации (`tariff_plans` / `aggregator_tariffs` / `reseller_tariff_plans`) на единую `price_rules` с денормализованным горячим кешем `resolved_rules`, сохранив обратную совместимость через фича-флаг и фазную миграцию.

**Architecture:** Source-of-truth `price_rules` + lazy-materialized `resolved_rules`. Owner-first fallback (subaccount → aggregator → platform), specificity bitmap по измерениям (traffic_type > sender_category > operator > country). Версионирование через `price_rules_version`. Инвалидация через outbox-воркер. Usage counter на субаккаунт.

**Tech Stack:** Go 1.24.0, PostgreSQL 15+ (pgx/v5, btree_gist extension), Redis 7+ (aggregator_id cache), spf13/viper (config), stretchr/testify (tests), golang-migrate.

**Спек:** [docs/superpowers/specs/2026-04-18-unified-pricing-model-design.md](../specs/2026-04-18-unified-pricing-model-design.md)

**Фазы:**
- **Phase 1 (этот план, детально):** expand schema + новый код под фича-флагом. Прод не затронут. Integration-тесты репозиториев исключены — testcontainers-инфраструктуры нет.
- **Phase 2 (operational, отдельная сессия):** backfill + dual-write + reconciliation + integration-тесты репозиториев (поднять testcontainers отдельно).
- **Phase 3 (operational, отдельная сессия):** cutover чтения.
- **Phase 4 (operational, отдельная сессия):** contract — снос legacy.

**Примечание по ветке:** Работа ведётся в worktree `.worktrees/unified-pricing-phase-1` на ветке `feat/unified-pricing-phase-1`.

**Примечание по тестам:** В Phase 1 только юнит-тесты с моками для application-слоя (`PriceResolver`, `CostCalculator`, `TiersCache`, `ResolvedRulesJanitor`, domain-валидация). Репозитории без тестов — валидация через smoke-миграции и CI-компиляцию. Integration — Phase 2.

**Go-module path:** `github.com/smpp-server/smpp-server` — использовать в импортах вместо placeholder `your-module-path`.

---

## Phase 1 — Expand schema + новый код под фича-флагом

### File Structure

**Миграции (создать):**
- `migrations/000104_unified_pricing_schema.up.sql` / `.down.sql`

**Новый код (создать):**
- `internal/services/tarification/domain/price_rule.go` — типы `PriceRule`, `PriceOwnerType`, `PriceModelType`, `TieredSpec`, `PrepaidThresholdSpec`, бизнес-валидация.
- `internal/services/tarification/domain/resolved_rule.go` — тип `ResolvedRule`.
- `internal/services/tarification/domain/subaccount_usage_counter.go` — тип `SubaccountUsageCounter`, `ComputePeriodKey`.
- `internal/services/tarification/domain/price_rule_repository.go` — интерфейсы `PriceRuleRepository`, `ResolvedRulesRepository`, `SubaccountUsageCounterRepository`, `PriceRulesVersionRepository`, `InvalidationOutboxRepository`.
- `internal/services/tarification/infrastructure/repository/price_rule_repository.go`
- `internal/services/tarification/infrastructure/repository/resolved_rules_repository.go`
- `internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository.go`
- `internal/services/tarification/infrastructure/repository/price_rules_version_repository.go`
- `internal/services/tarification/infrastructure/repository/invalidation_outbox_repository.go`
- `internal/services/tarification/application/price_resolver.go` — `PriceResolver.Resolve(ctx, input) → ResolvedRule`.
- `internal/services/tarification/application/cost_calculator.go` — `CostCalculator.Calculate(rule, usageBefore, segments) → cost`.
- `internal/services/tarification/application/resolved_rules_janitor.go` — воркер.
- `internal/services/tarification/application/tiers_cache.go` — in-process LRU для parsed tiers.

**Существующий код (модификация):**
- `internal/services/tarification/application/tarification_service.go` — добавить ветку под `UnifiedTarificationEnabled` фича-флаг (в Phase 1 только скелет, по факту не вызывается).
- `internal/platform/config/config.go` или соответствующий config-файл сервиса — добавить поле `UnifiedTarificationEnabled bool` с default=false. (Проверить где живёт конфиг tarification-service.)

**Тесты (создать):**
- `internal/services/tarification/domain/price_rule_test.go`
- `internal/services/tarification/domain/subaccount_usage_counter_test.go`
- `internal/services/tarification/application/price_resolver_test.go`
- `internal/services/tarification/application/cost_calculator_test.go`
- `internal/services/tarification/application/tiers_cache_test.go`
- `internal/services/tarification/application/resolved_rules_janitor_test.go`
- `internal/services/tarification/infrastructure/repository/price_rule_repository_integration_test.go` — требует тестовый PostgreSQL (уже используется в проекте через testcontainers? — проверить).

---

### Task 1: Миграция 000104 — расширение схемы

**Files:**
- Create: `migrations/000104_unified_pricing_schema.up.sql`
- Create: `migrations/000104_unified_pricing_schema.down.sql`

- [ ] **Step 1.1: Создать .up.sql**

```sql
-- migrations/000104_unified_pricing_schema.up.sql
BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE price_owner_type AS ENUM ('platform', 'aggregator', 'subaccount');
CREATE TYPE price_model_type AS ENUM ('fixed', 'tiered', 'prepaid_threshold');

CREATE TABLE price_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type       price_owner_type NOT NULL,
    owner_id         UUID NULL,
    country          VARCHAR(2)  NULL REFERENCES countries(iso_code),
    operator         VARCHAR(50) NULL REFERENCES operators(code),
    sender_category  VARCHAR(16) NULL,
    traffic_type     VARCHAR(16) NULL,
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NULL,
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       UUID NULL,

    CONSTRAINT owner_id_platform_rule CHECK (
        (owner_type = 'platform' AND owner_id IS NULL)
        OR (owner_type <> 'platform' AND owner_id IS NOT NULL)
    ),
    CONSTRAINT value_xor_tiers CHECK (
        (price_model = 'fixed' AND price_value IS NOT NULL AND tiers_json IS NULL)
        OR (price_model IN ('tiered','prepaid_threshold') AND tiers_json IS NOT NULL AND price_value IS NULL)
    ),
    CONSTRAINT sender_category_valid CHECK (
        sender_category IS NULL OR sender_category IN ('paid','free','none')
    ),
    CONSTRAINT traffic_type_valid CHECK (
        traffic_type IS NULL OR traffic_type IN ('transactional','marketing','service')
    ),
    CONSTRAINT valid_to_after_from CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE UNIQUE INDEX platform_catchall_singleton ON price_rules (owner_type)
WHERE owner_type = 'platform'
  AND country IS NULL AND operator IS NULL
  AND sender_category IS NULL AND traffic_type IS NULL
  AND valid_to IS NULL;

ALTER TABLE price_rules ADD CONSTRAINT no_overlapping_periods
EXCLUDE USING gist (
    owner_type WITH =,
    COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid) WITH =,
    COALESCE(country, '') WITH =,
    COALESCE(operator, '') WITH =,
    COALESCE(sender_category, '') WITH =,
    COALESCE(traffic_type, '') WITH =,
    tstzrange(valid_from, valid_to, '[)') WITH &&
);

CREATE INDEX idx_price_rules_lookup ON price_rules
    (owner_type, owner_id, country, operator, sender_category, traffic_type, valid_from DESC);

CREATE TABLE price_rules_version (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    version    BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO price_rules_version (id) VALUES (1);

CREATE TABLE resolved_rules (
    subaccount_id    UUID NOT NULL,
    country          VARCHAR(2) NOT NULL,
    operator         VARCHAR(50) NOT NULL,
    sender_category  VARCHAR(16) NOT NULL,
    traffic_type     VARCHAR(16) NOT NULL,
    effective_date   DATE NOT NULL,
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    source_rule_id   UUID NOT NULL REFERENCES price_rules(id),
    source_level     price_owner_type NOT NULL,
    aggregator_id    UUID NOT NULL,
    resolved_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    rules_version    BIGINT NOT NULL,

    PRIMARY KEY (subaccount_id, country, operator, sender_category, traffic_type, effective_date)
);

CREATE INDEX idx_resolved_rules_by_aggregator ON resolved_rules (aggregator_id);
CREATE INDEX idx_resolved_rules_by_source ON resolved_rules (source_rule_id);

CREATE TABLE subaccount_usage_counters (
    subaccount_id    UUID NOT NULL,
    period_key       TEXT NOT NULL,
    segments_used    BIGINT NOT NULL DEFAULT 0,
    amount_charged   NUMERIC(14,6) NOT NULL DEFAULT 0,
    last_updated     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (subaccount_id, period_key)
);

CREATE TABLE resolved_rules_invalidation_outbox (
    id             BIGSERIAL PRIMARY KEY,
    rule_id        UUID NOT NULL,
    owner_type     price_owner_type NOT NULL,
    owner_id       UUID NULL,
    affected_dims  JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ NULL
);

CREATE INDEX idx_outbox_unprocessed ON resolved_rules_invalidation_outbox (created_at)
WHERE processed_at IS NULL;

-- Триггер: инкремент price_rules_version + публикация в outbox
CREATE OR REPLACE FUNCTION price_rules_after_change() RETURNS TRIGGER AS $$
DECLARE
    target_rule_id UUID;
    target_owner_type price_owner_type;
    target_owner_id UUID;
    target_dims JSONB;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_rule_id := OLD.id;
        target_owner_type := OLD.owner_type;
        target_owner_id := OLD.owner_id;
        target_dims := jsonb_build_object(
            'country', OLD.country, 'operator', OLD.operator,
            'sender_category', OLD.sender_category, 'traffic_type', OLD.traffic_type
        );
    ELSE
        target_rule_id := NEW.id;
        target_owner_type := NEW.owner_type;
        target_owner_id := NEW.owner_id;
        target_dims := jsonb_build_object(
            'country', NEW.country, 'operator', NEW.operator,
            'sender_category', NEW.sender_category, 'traffic_type', NEW.traffic_type
        );
    END IF;

    UPDATE price_rules_version SET version = version + 1, updated_at = now() WHERE id = 1;

    INSERT INTO resolved_rules_invalidation_outbox (rule_id, owner_type, owner_id, affected_dims)
    VALUES (target_rule_id, target_owner_type, target_owner_id, target_dims);

    IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_price_rules_after_change
AFTER INSERT OR UPDATE OR DELETE ON price_rules
FOR EACH ROW EXECUTE FUNCTION price_rules_after_change();

COMMIT;
```

- [ ] **Step 1.2: Создать .down.sql**

```sql
-- migrations/000104_unified_pricing_schema.down.sql
BEGIN;

DROP TRIGGER IF EXISTS trg_price_rules_after_change ON price_rules;
DROP FUNCTION IF EXISTS price_rules_after_change();

DROP TABLE IF EXISTS resolved_rules_invalidation_outbox;
DROP TABLE IF EXISTS subaccount_usage_counters;
DROP TABLE IF EXISTS resolved_rules;
DROP TABLE IF EXISTS price_rules_version;
DROP TABLE IF EXISTS price_rules;

DROP TYPE IF EXISTS price_model_type;
DROP TYPE IF EXISTS price_owner_type;

COMMIT;
```

- [ ] **Step 1.3: Применить миграцию локально**

```bash
./scripts/server.sh migrate
```

Expected: миграция 000104 применяется без ошибок.

- [ ] **Step 1.4: Smoke-тест миграции**

```bash
./scripts/server.sh exec "psql -U postgres -d sms -c '\\d price_rules'"
./scripts/server.sh exec "psql -U postgres -d sms -c 'SELECT version FROM price_rules_version;'"
```

Expected: схема выведена, `version = 1`.

- [ ] **Step 1.5: Commit**

```bash
git add migrations/000104_unified_pricing_schema.up.sql migrations/000104_unified_pricing_schema.down.sql
git commit -m "feat(tarification): add unified price_rules schema

Phase 1 of unified pricing model migration. Introduces:
- price_rules (source of truth, 3 owner types)
- resolved_rules (hot-path denormalized cache)
- price_rules_version (global monotonic version)
- subaccount_usage_counters (unified per-subaccount usage)
- resolved_rules_invalidation_outbox
- Triggers for version bump and invalidation events
- Exclusion constraint for overlapping periods
- Catch-all singleton invariant on platform rules

See docs/superpowers/specs/2026-04-18-unified-pricing-model-design.md"
```

---

### Task 2: Domain типы — `PriceRule` и enums

**Files:**
- Create: `internal/services/tarification/domain/price_rule.go`
- Test: `internal/services/tarification/domain/price_rule_test.go`

- [ ] **Step 2.1: Написать тест**

```go
// internal/services/tarification/domain/price_rule_test.go
package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPriceRule_Validate_OK_Fixed(t *testing.T) {
	p := PriceRule{
		ID:         uuid.New(),
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: ptrDec("1.5"),
	}
	require.NoError(t, p.Validate())
}

func TestPriceRule_Validate_Fails_OwnerIDMismatch(t *testing.T) {
	ownerID := uuid.New()
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		OwnerID:    &ownerID,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: ptrDec("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrOwnerIDMismatch)
}

func TestPriceRule_Validate_Fails_AggregatorWithoutOwnerID(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerAggregator,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: ptrDec("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrOwnerIDMismatch)
}

func TestPriceRule_Validate_Fails_FixedWithoutValue(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_FixedWithTiers(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: ptrDec("1.0"),
		TiersJSON:  []byte(`{"tiers":[]}`),
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_TieredWithoutTiers(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelTiered,
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_ValidToBeforeFrom(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  now,
		ValidTo:    &earlier,
		PriceModel: ModelFixed,
		PriceValue: ptrDec("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidPeriod)
}

func TestSpecificityBitmap(t *testing.T) {
	tests := []struct {
		country, operator, category, traffic string
		expected                             int
	}{
		{"", "", "", "", 0},
		{"RU", "", "", "", 1},
		{"", "MTS", "", "", 2},
		{"RU", "MTS", "", "", 3},
		{"", "", "paid", "", 4},
		{"", "", "", "transactional", 8},
		{"RU", "MTS", "paid", "transactional", 15},
	}
	for _, tc := range tests {
		var c, o, cat, tr *string
		if tc.country != "" {
			c = &tc.country
		}
		if tc.operator != "" {
			o = &tc.operator
		}
		if tc.category != "" {
			cat = &tc.category
		}
		if tc.traffic != "" {
			tr = &tc.traffic
		}
		p := PriceRule{Country: c, Operator: o, SenderCategory: cat, TrafficType: tr}
		require.Equal(t, tc.expected, p.SpecificityBitmap())
	}
}

func ptrDec(s string) *string { return &s }
```

- [ ] **Step 2.2: Запустить тест — убедиться что fail**

```bash
go test ./internal/services/tarification/domain/ -run TestPriceRule -v
```

Expected: компиляция падает (типов нет).

- [ ] **Step 2.3: Написать реализацию**

```go
// internal/services/tarification/domain/price_rule.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type PriceOwnerType string

const (
	OwnerPlatform   PriceOwnerType = "platform"
	OwnerAggregator PriceOwnerType = "aggregator"
	OwnerSubaccount PriceOwnerType = "subaccount"
)

type PriceModelType string

const (
	ModelFixed             PriceModelType = "fixed"
	ModelTiered            PriceModelType = "tiered"
	ModelPrepaidThreshold  PriceModelType = "prepaid_threshold"
)

type SenderCategory string

const (
	SenderPaid SenderCategory = "paid"
	SenderFree SenderCategory = "free"
	SenderNone SenderCategory = "none"
)

type TrafficType string

const (
	TrafficTransactional TrafficType = "transactional"
	TrafficMarketing     TrafficType = "marketing"
	TrafficService       TrafficType = "service"
)

var (
	ErrOwnerIDMismatch   = errors.New("owner_id must be NULL iff owner_type=platform")
	ErrPriceSpecMismatch = errors.New("price_value/tiers_json does not match price_model")
	ErrInvalidPeriod     = errors.New("valid_to must be after valid_from")
)

// PriceRule — единое правило ценообразования.
// PriceValue и TiersJSON — строковые представления (NUMERIC/JSONB). Application-слой
// конвертирует при расчёте. Хранить как string/[]byte, чтобы не тащить decimal-зависимость
// в domain.
type PriceRule struct {
	ID             uuid.UUID
	OwnerType      PriceOwnerType
	OwnerID        *uuid.UUID
	Country        *string
	Operator       *string
	SenderCategory *string
	TrafficType    *string
	ValidFrom      time.Time
	ValidTo        *time.Time
	PriceModel     PriceModelType
	PriceValue     *string // NUMERIC as string
	TiersJSON      []byte  // raw JSONB
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedBy      *uuid.UUID
}

func (p *PriceRule) Validate() error {
	if p.OwnerType == OwnerPlatform && p.OwnerID != nil {
		return ErrOwnerIDMismatch
	}
	if p.OwnerType != OwnerPlatform && p.OwnerID == nil {
		return ErrOwnerIDMismatch
	}
	if p.PriceModel == ModelFixed {
		if p.PriceValue == nil || len(p.TiersJSON) > 0 {
			return ErrPriceSpecMismatch
		}
	} else {
		if len(p.TiersJSON) == 0 || p.PriceValue != nil {
			return ErrPriceSpecMismatch
		}
	}
	if p.ValidTo != nil && !p.ValidTo.After(p.ValidFrom) {
		return ErrInvalidPeriod
	}
	return nil
}

// SpecificityBitmap — битмап по измерениям для tie-breaker в lookup.
// traffic_type=8, sender_category=4, operator=2, country=1.
func (p *PriceRule) SpecificityBitmap() int {
	bm := 0
	if p.Country != nil && *p.Country != "" {
		bm |= 1
	}
	if p.Operator != nil && *p.Operator != "" {
		bm |= 2
	}
	if p.SenderCategory != nil && *p.SenderCategory != "" {
		bm |= 4
	}
	if p.TrafficType != nil && *p.TrafficType != "" {
		bm |= 8
	}
	return bm
}
```

Заметка: тест `ptrDec` возвращает `*string`, а в типе `PriceValue` — `*string`. В тестах дальше фикс сигнатуры — оставить как есть, это `*string`, не `*decimal`. При необходимости — поправить сигнатуру.

- [ ] **Step 2.4: Запустить тест**

```bash
go test ./internal/services/tarification/domain/ -run TestPriceRule -v
go test ./internal/services/tarification/domain/ -run TestSpecificityBitmap -v
```

Expected: PASS.

- [ ] **Step 2.5: Commit**

```bash
git add internal/services/tarification/domain/price_rule.go internal/services/tarification/domain/price_rule_test.go
git commit -m "feat(tarification): add PriceRule domain type with validation"
```

---

### Task 3: Domain типы — `ResolvedRule`, `SubaccountUsageCounter`, `period_key`

**Files:**
- Create: `internal/services/tarification/domain/resolved_rule.go`
- Create: `internal/services/tarification/domain/subaccount_usage_counter.go`
- Test: `internal/services/tarification/domain/subaccount_usage_counter_test.go`

- [ ] **Step 3.1: Написать тест `ComputePeriodKey`**

```go
// internal/services/tarification/domain/subaccount_usage_counter_test.go
package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputePeriodKey_Month(t *testing.T) {
	at := time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC)
	require.Equal(t, "2026-04", ComputePeriodKey(PeriodCalendarMonth, at))
}

func TestComputePeriodKey_Day(t *testing.T) {
	at := time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC)
	require.Equal(t, "2026-04-18", ComputePeriodKey(PeriodCalendarDay, at))
}

func TestComputePeriodKey_UTC(t *testing.T) {
	// Локальное время в MSK (UTC+3) 23:30 28 февраля
	// В UTC — 20:30 28 февраля → period_key = '2026-02'
	msk := time.FixedZone("MSK", 3*3600)
	at := time.Date(2026, 3, 1, 2, 30, 0, 0, msk) // 01 марта по MSK, 22:30 28 февраля UTC
	require.Equal(t, "2026-02", ComputePeriodKey(PeriodCalendarMonth, at))
}

func TestComputePeriodKey_Empty_DefaultsToMonth(t *testing.T) {
	at := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	require.Equal(t, "2026-04", ComputePeriodKey("", at))
}
```

- [ ] **Step 3.2: Запустить — убедиться что fail**

```bash
go test ./internal/services/tarification/domain/ -run TestComputePeriodKey -v
```

- [ ] **Step 3.3: Реализовать `resolved_rule.go`**

```go
// internal/services/tarification/domain/resolved_rule.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ResolvedRule — материализованный выбор правила для конкретного
// (subaccount, измерения, дата). Горячий путь читает отсюда.
type ResolvedRule struct {
	SubaccountID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	EffectiveDate  time.Time // DATE, UTC midnight

	PriceModel   PriceModelType
	PriceValue   *string
	TiersJSON    []byte
	SourceRuleID uuid.UUID
	SourceLevel  PriceOwnerType
	AggregatorID uuid.UUID

	ResolvedAt    time.Time
	RulesVersion  int64
}
```

- [ ] **Step 3.4: Реализовать `subaccount_usage_counter.go`**

```go
// internal/services/tarification/domain/subaccount_usage_counter.go
package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PeriodType string

const (
	PeriodCalendarMonth PeriodType = "calendar_month"
	PeriodCalendarDay   PeriodType = "calendar_day"
)

type SubaccountUsageCounter struct {
	SubaccountID   uuid.UUID
	PeriodKey      string
	SegmentsUsed   int64
	AmountCharged  string // NUMERIC as string
	LastUpdated    time.Time
}

// ComputePeriodKey — возвращает строковый ключ периода по UTC.
// Пустой период трактуется как calendar_month (для price_model='fixed' где периода нет в tiers_json).
func ComputePeriodKey(period PeriodType, at time.Time) string {
	utc := at.UTC()
	switch period {
	case PeriodCalendarDay:
		return utc.Format("2006-01-02")
	case PeriodCalendarMonth, "":
		return utc.Format("2006-01")
	default:
		return fmt.Sprintf("unknown-%s-%s", period, utc.Format("2006-01"))
	}
}
```

- [ ] **Step 3.5: Запустить тест**

```bash
go test ./internal/services/tarification/domain/ -run TestComputePeriodKey -v
```

Expected: PASS.

- [ ] **Step 3.6: Commit**

```bash
git add internal/services/tarification/domain/resolved_rule.go \
        internal/services/tarification/domain/subaccount_usage_counter.go \
        internal/services/tarification/domain/subaccount_usage_counter_test.go
git commit -m "feat(tarification): add ResolvedRule and SubaccountUsageCounter types"
```

---

### Task 4: Интерфейсы репозиториев

**Files:**
- Create: `internal/services/tarification/domain/price_rule_repository.go`

- [ ] **Step 4.1: Написать интерфейсы**

```go
// internal/services/tarification/domain/price_rule_repository.go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ResolveInput — запрос на поиск применимого правила.
type ResolveInput struct {
	SubaccountID   uuid.UUID
	AggregatorID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	Now            time.Time
}

// PriceRuleRepository — CRUD + lookup по price_rules.
type PriceRuleRepository interface {
	// FindApplicable — реализует алгоритм owner-first + specificity bitmap.
	// Возвращает (nil, ErrNoApplicableRule), если ни одного правила не найдено.
	// Catch-all на платформе гарантирует, что это случается только при повреждении инварианта.
	FindApplicable(ctx context.Context, in ResolveInput) (*PriceRule, error)

	Create(ctx context.Context, r *PriceRule) error
	Update(ctx context.Context, r *PriceRule) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*PriceRule, error)

	// HasPlatformCatchAll — инвариант-чек для startup.
	HasPlatformCatchAll(ctx context.Context) (bool, error)
}

type ResolvedRulesRepository interface {
	Get(ctx context.Context, subaccountID uuid.UUID, country, operator, senderCategory, trafficType string, effectiveDate time.Time) (*ResolvedRule, error)
	Upsert(ctx context.Context, r *ResolvedRule) error
	// DeleteAffected — принимает критерии инвалидации из outbox event.
	DeleteAffected(ctx context.Context, ownerType PriceOwnerType, ownerID *uuid.UUID, country, operator, senderCategory, trafficType *string) (int64, error)
}

type PriceRulesVersionRepository interface {
	GetVersion(ctx context.Context) (int64, error)
}

type SubaccountUsageCounterRepository interface {
	// Increment — атомарный UPSERT. Возвращает segmentsUsed ПОСЛЕ инкремента.
	Increment(ctx context.Context, subaccountID uuid.UUID, periodKey string, segments int64, amountCharged string) (int64, error)
	Get(ctx context.Context, subaccountID uuid.UUID, periodKey string) (*SubaccountUsageCounter, error)
}

type InvalidationEvent struct {
	ID        int64
	RuleID    uuid.UUID
	OwnerType PriceOwnerType
	OwnerID   *uuid.UUID
	Country   *string
	Operator  *string
	SenderCat *string
	Traffic   *string
	CreatedAt time.Time
}

type InvalidationOutboxRepository interface {
	ListUnprocessed(ctx context.Context, limit int) ([]InvalidationEvent, error)
	MarkProcessed(ctx context.Context, ids []int64) error
}
```

Интерфейс `ErrNoApplicableRule` — добавить в errors.go домена (если нет).

- [ ] **Step 4.2: Убедиться что компилируется**

```bash
go build ./internal/services/tarification/domain/...
```

Expected: пройдёт либо ругнётся на `ErrNoApplicableRule`. Если ругнётся — добавить в `errors.go`:

```go
// в internal/services/tarification/domain/errors.go (существующий файл)
var ErrNoApplicableRule = errors.New("no applicable price rule found")
```

- [ ] **Step 4.3: Commit**

```bash
git add internal/services/tarification/domain/price_rule_repository.go \
        internal/services/tarification/domain/errors.go
git commit -m "feat(tarification): define unified pricing repository interfaces"
```

---

### Task 5: `PriceRuleRepository` реализация (pgx)

**Files:**
- Create: `internal/services/tarification/infrastructure/repository/price_rule_repository.go`
- Test: `internal/services/tarification/infrastructure/repository/price_rule_repository_integration_test.go`

Интеграционный тест требует работающий PostgreSQL. Убедиться, что в проекте используется testcontainers или direct-connect для integration-тестов (проверить существующий `tariff_period_repository.go` для шаблона).

- [ ] **Step 5.1: Написать интеграционный тест для `FindApplicable` (owner-first, specificity)**

Тест создаёт набор правил и проверяет алгоритм. Файл `price_rule_repository_integration_test.go`:

```go
//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/your-module-path/internal/services/tarification/domain"
)

// helper: setupPool — переиспользовать существующий helper из проекта.
// Если нет — подключиться через DATABASE_URL env.

func TestPriceRuleRepo_FindApplicable_OwnerFirst(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()
	aggID := uuid.New()
	subID := uuid.New()
	now := time.Now()

	// Платформа: specific правило РФ+МТС = 2.00
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		Country:    ptr("RU"), Operator: ptr("MTS"),
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("2.0"),
	})

	// Платформа: catch-all = 5.00
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("5.0"),
	})

	// Субаккаунт: catch-all = 1.00
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerSubaccount, OwnerID: &subID,
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("1.0"),
	})

	// Запрос: РФ + МТС → должен победить субаккаунт-catchall (owner-first)
	rule, err := repo.FindApplicable(ctx, domain.ResolveInput{
		SubaccountID: subID, AggregatorID: aggID,
		Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: now,
	})
	require.NoError(t, err)
	require.Equal(t, domain.OwnerSubaccount, rule.OwnerType)
	require.Equal(t, "1.0", *rule.PriceValue)
}

func TestPriceRuleRepo_FindApplicable_SpecificityWithinOwner(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()
	subID := uuid.New()
	now := time.Now()

	// Субаккаунт: country-only = 1.0
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerSubaccount, OwnerID: &subID,
		Country:    ptr("RU"),
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("1.0"),
	})
	// Субаккаунт: traffic-type-only = 0.5
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerSubaccount, OwnerID: &subID,
		TrafficType: ptr("transactional"),
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("0.5"),
	})
	// catch-all платформа = 10
	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("10.0"),
	})

	// Запрос: RU + transactional → traffic=8 побеждает country=1
	rule, err := repo.FindApplicable(ctx, domain.ResolveInput{
		SubaccountID: subID, AggregatorID: uuid.New(),
		Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: now,
	})
	require.NoError(t, err)
	require.Equal(t, "0.5", *rule.PriceValue)
}

func TestPriceRuleRepo_FindApplicable_CatchAllOnlyOnPlatform(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()
	now := time.Now()

	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		ValidFrom:  now.Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("7.77"),
	})

	rule, err := repo.FindApplicable(ctx, domain.ResolveInput{
		SubaccountID: uuid.New(), AggregatorID: uuid.New(),
		Country: "KZ", Operator: "Kcell",
		SenderCategory: "paid", TrafficType: "marketing",
		Now: now,
	})
	require.NoError(t, err)
	require.Equal(t, domain.OwnerPlatform, rule.OwnerType)
	require.Equal(t, "7.77", *rule.PriceValue)
}

func TestPriceRuleRepo_FindApplicable_NoRule_ReturnsError(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()

	_, err := repo.FindApplicable(ctx, domain.ResolveInput{
		SubaccountID: uuid.New(), AggregatorID: uuid.New(),
		Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.ErrorIs(t, err, domain.ErrNoApplicableRule)
}

func TestPriceRuleRepo_HasPlatformCatchAll(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()

	has, err := repo.HasPlatformCatchAll(ctx)
	require.NoError(t, err)
	require.False(t, has)

	mustCreate(t, repo, &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		ValidFrom:  time.Now().Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("1.0"),
	})

	has, err = repo.HasPlatformCatchAll(ctx)
	require.NoError(t, err)
	require.True(t, has)
}

func TestPriceRuleRepo_OverlappingPeriods_Rejected(t *testing.T) {
	pool := setupPool(t)
	defer cleanPriceRules(t, pool)

	repo := NewPriceRuleRepository(pool)
	ctx := context.Background()
	now := time.Now()
	to := now.Add(time.Hour)

	mustCreate(t, repo, &domain.PriceRule{
		ID: uuid.New(), OwnerType: domain.OwnerPlatform,
		ValidFrom: now, ValidTo: &to,
		PriceModel: domain.ModelFixed, PriceValue: ptr("1.0"),
	})

	err := repo.Create(ctx, &domain.PriceRule{
		ID: uuid.New(), OwnerType: domain.OwnerPlatform,
		ValidFrom: now.Add(30 * time.Minute), ValidTo: &to,
		PriceModel: domain.ModelFixed, PriceValue: ptr("2.0"),
	})
	require.Error(t, err) // EXCLUDE constraint violation
}

// helpers
func ptr(s string) *string { return &s }

func mustCreate(t *testing.T, repo domain.PriceRuleRepository, r *domain.PriceRule) {
	t.Helper()
	require.NoError(t, repo.Create(context.Background(), r))
}

func cleanPriceRules(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, _ = pool.Exec(context.Background(), "DELETE FROM price_rules")
}

// setupPool — должен использовать DATABASE_URL env или testcontainers (см. проектный стандарт).
func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Реализация — скопировать из существующего integration-теста в этой же папке,
	// например из tariff_period_repository_integration_test.go если он есть,
	// или следовать паттерну других *_integration_test.go в проекте.
	t.Fatal("setupPool must be implemented using project's integration-test standard — see existing integration tests in this repo")
	return nil
}
```

- [ ] **Step 5.2: Реализовать репозиторий**

```go
// internal/services/tarification/infrastructure/repository/price_rule_repository.go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type PriceRuleRepository struct {
	pool *pgxpool.Pool
}

func NewPriceRuleRepository(pool *pgxpool.Pool) *PriceRuleRepository {
	return &PriceRuleRepository{pool: pool}
}

const findApplicableSQL = `
WITH ranked AS (
  SELECT
    id, owner_type, owner_id, country, operator, sender_category, traffic_type,
    valid_from, valid_to, price_model, price_value, tiers_json,
    created_at, updated_at, created_by,
    CASE owner_type
      WHEN 'subaccount' THEN 3
      WHEN 'aggregator' THEN 2
      WHEN 'platform'   THEN 1
    END AS owner_rank,
    (CASE WHEN traffic_type    IS NOT NULL THEN 8 ELSE 0 END) +
    (CASE WHEN sender_category IS NOT NULL THEN 4 ELSE 0 END) +
    (CASE WHEN operator        IS NOT NULL THEN 2 ELSE 0 END) +
    (CASE WHEN country         IS NOT NULL THEN 1 ELSE 0 END) AS specificity_rank
  FROM price_rules
  WHERE (
         (owner_type='subaccount' AND owner_id = $1)
      OR (owner_type='aggregator' AND owner_id = $2)
      OR (owner_type='platform')
  )
    AND (country         IS NULL OR country         = $3)
    AND (operator        IS NULL OR operator        = $4)
    AND (sender_category IS NULL OR sender_category = $5)
    AND (traffic_type    IS NULL OR traffic_type    = $6)
    AND valid_from <= $7
    AND (valid_to IS NULL OR valid_to > $7)
)
SELECT id, owner_type, owner_id, country, operator, sender_category, traffic_type,
       valid_from, valid_to, price_model, price_value::text, tiers_json,
       created_at, updated_at, created_by
FROM ranked
ORDER BY owner_rank DESC, specificity_rank DESC, valid_from DESC
LIMIT 1;
`

func (r *PriceRuleRepository) FindApplicable(ctx context.Context, in domain.ResolveInput) (*domain.PriceRule, error) {
	row := r.pool.QueryRow(ctx, findApplicableSQL,
		in.SubaccountID, in.AggregatorID,
		in.Country, in.Operator, in.SenderCategory, in.TrafficType,
		in.Now,
	)
	rule, err := scanPriceRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoApplicableRule
	}
	if err != nil {
		return nil, fmt.Errorf("find applicable: %w", err)
	}
	return rule, nil
}

const createSQL = `
INSERT INTO price_rules (
  id, owner_type, owner_id, country, operator, sender_category, traffic_type,
  valid_from, valid_to, price_model, price_value, tiers_json, created_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11::numeric, $12, $13
);
`

func (r *PriceRuleRepository) Create(ctx context.Context, p *domain.PriceRule) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, createSQL,
		p.ID, p.OwnerType, p.OwnerID,
		p.Country, p.Operator, p.SenderCategory, p.TrafficType,
		p.ValidFrom, p.ValidTo, p.PriceModel, p.PriceValue, p.TiersJSON, p.CreatedBy,
	)
	return err
}

const updateSQL = `
UPDATE price_rules SET
  country = $2, operator = $3, sender_category = $4, traffic_type = $5,
  valid_from = $6, valid_to = $7,
  price_model = $8, price_value = $9::numeric, tiers_json = $10,
  updated_at = now()
WHERE id = $1;
`

func (r *PriceRuleRepository) Update(ctx context.Context, p *domain.PriceRule) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, updateSQL,
		p.ID,
		p.Country, p.Operator, p.SenderCategory, p.TrafficType,
		p.ValidFrom, p.ValidTo, p.PriceModel, p.PriceValue, p.TiersJSON,
	)
	return err
}

func (r *PriceRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM price_rules WHERE id = $1`, id)
	return err
}

const getByIDSQL = `
SELECT id, owner_type, owner_id, country, operator, sender_category, traffic_type,
       valid_from, valid_to, price_model, price_value::text, tiers_json,
       created_at, updated_at, created_by
FROM price_rules WHERE id = $1;
`

func (r *PriceRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PriceRule, error) {
	row := r.pool.QueryRow(ctx, getByIDSQL, id)
	p, err := scanPriceRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoApplicableRule
	}
	return p, err
}

func (r *PriceRuleRepository) HasPlatformCatchAll(ctx context.Context) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM price_rules
WHERE owner_type='platform' AND country IS NULL AND operator IS NULL
  AND sender_category IS NULL AND traffic_type IS NULL
  AND valid_to IS NULL
  AND valid_from <= now()
`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPriceRule(row rowScanner) (*domain.PriceRule, error) {
	p := &domain.PriceRule{}
	var priceValue *string
	err := row.Scan(
		&p.ID, &p.OwnerType, &p.OwnerID,
		&p.Country, &p.Operator, &p.SenderCategory, &p.TrafficType,
		&p.ValidFrom, &p.ValidTo, &p.PriceModel, &priceValue, &p.TiersJSON,
		&p.CreatedAt, &p.UpdatedAt, &p.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	p.PriceValue = priceValue
	return p, nil
}
```

Важно: `your-module-path` — заменить на фактический Go-module из `go.mod`. Проверить `head -1 go.mod`.

- [ ] **Step 5.3: Перед запуском — определить setupPool helper**

Найти существующий pattern:

```bash
grep -r "func setupPool\|func newTestPool\|testcontainers" internal/services/*/infrastructure/repository/ --include="*.go" -l | head -5
```

Скопировать/адаптировать в файл теста.

- [ ] **Step 5.4: Запустить интеграционный тест**

```bash
go test -tags=integration ./internal/services/tarification/infrastructure/repository/ -run TestPriceRuleRepo -v
```

Expected: PASS.

- [ ] **Step 5.5: Commit**

```bash
git add internal/services/tarification/infrastructure/repository/price_rule_repository.go \
        internal/services/tarification/infrastructure/repository/price_rule_repository_integration_test.go
git commit -m "feat(tarification): implement PriceRuleRepository with owner-first lookup"
```

---

### Task 6: Остальные репозитории (`ResolvedRules`, `Version`, `UsageCounter`, `Outbox`)

**Files:**
- Create: `internal/services/tarification/infrastructure/repository/resolved_rules_repository.go`
- Create: `internal/services/tarification/infrastructure/repository/price_rules_version_repository.go`
- Create: `internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository.go`
- Create: `internal/services/tarification/infrastructure/repository/invalidation_outbox_repository.go`
- Test: `internal/services/tarification/infrastructure/repository/resolved_rules_repository_integration_test.go`
- Test: `internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository_integration_test.go`

- [ ] **Step 6.1: Реализовать `PriceRulesVersionRepository` (простой, без отдельного теста)**

```go
// internal/services/tarification/infrastructure/repository/price_rules_version_repository.go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PriceRulesVersionRepository struct {
	pool *pgxpool.Pool
}

func NewPriceRulesVersionRepository(pool *pgxpool.Pool) *PriceRulesVersionRepository {
	return &PriceRulesVersionRepository{pool: pool}
}

func (r *PriceRulesVersionRepository) GetVersion(ctx context.Context) (int64, error) {
	var v int64
	err := r.pool.QueryRow(ctx, `SELECT version FROM price_rules_version WHERE id=1`).Scan(&v)
	return v, err
}
```

- [ ] **Step 6.2: Реализовать `ResolvedRulesRepository`**

```go
// internal/services/tarification/infrastructure/repository/resolved_rules_repository.go
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type ResolvedRulesRepository struct {
	pool *pgxpool.Pool
}

func NewResolvedRulesRepository(pool *pgxpool.Pool) *ResolvedRulesRepository {
	return &ResolvedRulesRepository{pool: pool}
}

const getResolvedSQL = `
SELECT subaccount_id, country, operator, sender_category, traffic_type, effective_date,
       price_model, price_value::text, tiers_json,
       source_rule_id, source_level, aggregator_id, resolved_at, rules_version
FROM resolved_rules
WHERE subaccount_id=$1 AND country=$2 AND operator=$3 AND sender_category=$4
  AND traffic_type=$5 AND effective_date=$6;
`

func (r *ResolvedRulesRepository) Get(ctx context.Context, subID uuid.UUID, country, operator, cat, traffic string, date time.Time) (*domain.ResolvedRule, error) {
	var rr domain.ResolvedRule
	var priceValue *string
	err := r.pool.QueryRow(ctx, getResolvedSQL,
		subID, country, operator, cat, traffic, date,
	).Scan(
		&rr.SubaccountID, &rr.Country, &rr.Operator, &rr.SenderCategory, &rr.TrafficType, &rr.EffectiveDate,
		&rr.PriceModel, &priceValue, &rr.TiersJSON,
		&rr.SourceRuleID, &rr.SourceLevel, &rr.AggregatorID, &rr.ResolvedAt, &rr.RulesVersion,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rr.PriceValue = priceValue
	return &rr, nil
}

const upsertResolvedSQL = `
INSERT INTO resolved_rules (
  subaccount_id, country, operator, sender_category, traffic_type, effective_date,
  price_model, price_value, tiers_json,
  source_rule_id, source_level, aggregator_id, rules_version
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8::numeric, $9,
  $10, $11, $12, $13
)
ON CONFLICT (subaccount_id, country, operator, sender_category, traffic_type, effective_date)
DO UPDATE SET
  price_model   = EXCLUDED.price_model,
  price_value   = EXCLUDED.price_value,
  tiers_json    = EXCLUDED.tiers_json,
  source_rule_id= EXCLUDED.source_rule_id,
  source_level  = EXCLUDED.source_level,
  aggregator_id = EXCLUDED.aggregator_id,
  resolved_at   = now(),
  rules_version = EXCLUDED.rules_version;
`

func (r *ResolvedRulesRepository) Upsert(ctx context.Context, rr *domain.ResolvedRule) error {
	_, err := r.pool.Exec(ctx, upsertResolvedSQL,
		rr.SubaccountID, rr.Country, rr.Operator, rr.SenderCategory, rr.TrafficType, rr.EffectiveDate,
		rr.PriceModel, rr.PriceValue, rr.TiersJSON,
		rr.SourceRuleID, rr.SourceLevel, rr.AggregatorID, rr.RulesVersion,
	)
	return err
}

func (r *ResolvedRulesRepository) DeleteAffected(ctx context.Context, ownerType domain.PriceOwnerType, ownerID *uuid.UUID, country, operator, cat, traffic *string) (int64, error) {
	// Стратегия: сносим все строки, затронутые изменением правила.
	// - ownerType=subaccount: только этот subaccount_id
	// - ownerType=aggregator: все subaccount этого агрегатора (denormalized aggregator_id)
	// - ownerType=platform: все строки, удовлетворяющие dims (может быть вся таблица)
	// Dims — опциональные фильтры: NULL значит "любое".
	q := `DELETE FROM resolved_rules WHERE 1=1`
	args := []any{}
	argN := 0
	add := func(expr string, val any) {
		argN++
		q += " AND " + expr + fmt.Sprintf("$%d", argN)
		args = append(args, val)
	}
	switch ownerType {
	case domain.OwnerSubaccount:
		if ownerID == nil {
			return 0, errors.New("subaccount invalidation requires owner_id")
		}
		add("subaccount_id=", *ownerID)
	case domain.OwnerAggregator:
		if ownerID == nil {
			return 0, errors.New("aggregator invalidation requires owner_id")
		}
		add("aggregator_id=", *ownerID)
	case domain.OwnerPlatform:
		// no owner filter
	}
	if country != nil {
		add("country=", *country)
	}
	if operator != nil {
		add("operator=", *operator)
	}
	if cat != nil {
		add("sender_category=", *cat)
	}
	if traffic != nil {
		add("traffic_type=", *traffic)
	}
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
```

Добавить импорт `"fmt"` в файле.

- [ ] **Step 6.3: Написать integration-тест для `ResolvedRulesRepository`**

Минимальный набор: upsert → get → delete-affected → проверка.

```go
//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/your-module-path/internal/services/tarification/domain"
)

func TestResolvedRulesRepo_UpsertAndGet(t *testing.T) {
	pool := setupPool(t)
	defer func() { _, _ = pool.Exec(context.Background(), "TRUNCATE resolved_rules, price_rules CASCADE") }()

	priceRepo := NewPriceRuleRepository(pool)
	resolvedRepo := NewResolvedRulesRepository(pool)
	ctx := context.Background()

	// Нужно правило, на которое ссылается resolved.source_rule_id
	rule := &domain.PriceRule{
		ID:         uuid.New(),
		OwnerType:  domain.OwnerPlatform,
		ValidFrom:  time.Now().Add(-time.Hour),
		PriceModel: domain.ModelFixed,
		PriceValue: ptr("1.5"),
	}
	require.NoError(t, priceRepo.Create(ctx, rule))

	rr := &domain.ResolvedRule{
		SubaccountID:   uuid.New(),
		Country:        "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		EffectiveDate:  time.Now().UTC().Truncate(24 * time.Hour),
		PriceModel:     domain.ModelFixed,
		PriceValue:     ptr("1.5"),
		SourceRuleID:   rule.ID,
		SourceLevel:    domain.OwnerPlatform,
		AggregatorID:   uuid.New(),
		RulesVersion:   1,
	}
	require.NoError(t, resolvedRepo.Upsert(ctx, rr))

	got, err := resolvedRepo.Get(ctx, rr.SubaccountID, rr.Country, rr.Operator, rr.SenderCategory, rr.TrafficType, rr.EffectiveDate)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, rr.SourceRuleID, got.SourceRuleID)
	require.Equal(t, int64(1), got.RulesVersion)
}

func TestResolvedRulesRepo_DeleteAffected_ByAggregator(t *testing.T) {
	pool := setupPool(t)
	defer func() { _, _ = pool.Exec(context.Background(), "TRUNCATE resolved_rules, price_rules CASCADE") }()

	priceRepo := NewPriceRuleRepository(pool)
	resolvedRepo := NewResolvedRulesRepository(pool)
	ctx := context.Background()
	aggID := uuid.New()

	rule := &domain.PriceRule{
		ID: uuid.New(), OwnerType: domain.OwnerPlatform,
		ValidFrom: time.Now().Add(-time.Hour),
		PriceModel: domain.ModelFixed, PriceValue: ptr("1.0"),
	}
	require.NoError(t, priceRepo.Create(ctx, rule))

	for i := 0; i < 3; i++ {
		require.NoError(t, resolvedRepo.Upsert(ctx, &domain.ResolvedRule{
			SubaccountID: uuid.New(), Country: "RU", Operator: "MTS",
			SenderCategory: "paid", TrafficType: "transactional",
			EffectiveDate: time.Now().UTC().Truncate(24 * time.Hour),
			PriceModel:    domain.ModelFixed, PriceValue: ptr("1.0"),
			SourceRuleID:  rule.ID, SourceLevel: domain.OwnerPlatform,
			AggregatorID:  aggID, RulesVersion: 1,
		}))
	}

	deleted, err := resolvedRepo.DeleteAffected(ctx, domain.OwnerAggregator, &aggID, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
}
```

- [ ] **Step 6.4: Запустить тесты**

```bash
go test -tags=integration ./internal/services/tarification/infrastructure/repository/ -run TestResolvedRulesRepo -v
```

Expected: PASS.

- [ ] **Step 6.5: Реализовать `SubaccountUsageCounterRepository`**

```go
// internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository.go
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type SubaccountUsageCounterRepository struct {
	pool *pgxpool.Pool
}

func NewSubaccountUsageCounterRepository(pool *pgxpool.Pool) *SubaccountUsageCounterRepository {
	return &SubaccountUsageCounterRepository{pool: pool}
}

const incrementSQL = `
INSERT INTO subaccount_usage_counters (subaccount_id, period_key, segments_used, amount_charged, last_updated)
VALUES ($1, $2, $3, $4::numeric, now())
ON CONFLICT (subaccount_id, period_key)
DO UPDATE SET
  segments_used  = subaccount_usage_counters.segments_used  + EXCLUDED.segments_used,
  amount_charged = subaccount_usage_counters.amount_charged + EXCLUDED.amount_charged,
  last_updated   = now()
RETURNING segments_used;
`

func (r *SubaccountUsageCounterRepository) Increment(ctx context.Context, subID uuid.UUID, periodKey string, segments int64, amount string) (int64, error) {
	var newTotal int64
	err := r.pool.QueryRow(ctx, incrementSQL, subID, periodKey, segments, amount).Scan(&newTotal)
	return newTotal, err
}

const getCounterSQL = `
SELECT subaccount_id, period_key, segments_used, amount_charged::text, last_updated
FROM subaccount_usage_counters
WHERE subaccount_id=$1 AND period_key=$2;
`

func (r *SubaccountUsageCounterRepository) Get(ctx context.Context, subID uuid.UUID, periodKey string) (*domain.SubaccountUsageCounter, error) {
	var c domain.SubaccountUsageCounter
	err := r.pool.QueryRow(ctx, getCounterSQL, subID, periodKey).Scan(
		&c.SubaccountID, &c.PeriodKey, &c.SegmentsUsed, &c.AmountCharged, &c.LastUpdated,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}
```

- [ ] **Step 6.6: Тест для `Increment` (идемпотентный инкремент)**

```go
//go:build integration

func TestUsageCounter_Increment_Accumulates(t *testing.T) {
	pool := setupPool(t)
	defer func() { _, _ = pool.Exec(context.Background(), "TRUNCATE subaccount_usage_counters") }()

	repo := NewSubaccountUsageCounterRepository(pool)
	ctx := context.Background()
	subID := uuid.New()

	total, err := repo.Increment(ctx, subID, "2026-04", 100, "50.0")
	require.NoError(t, err)
	require.Equal(t, int64(100), total)

	total, err = repo.Increment(ctx, subID, "2026-04", 50, "25.0")
	require.NoError(t, err)
	require.Equal(t, int64(150), total)

	got, err := repo.Get(ctx, subID, "2026-04")
	require.NoError(t, err)
	require.Equal(t, int64(150), got.SegmentsUsed)
}
```

Запустить:

```bash
go test -tags=integration ./internal/services/tarification/infrastructure/repository/ -run TestUsageCounter -v
```

Expected: PASS.

- [ ] **Step 6.7: Реализовать `InvalidationOutboxRepository`**

```go
// internal/services/tarification/infrastructure/repository/invalidation_outbox_repository.go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type InvalidationOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewInvalidationOutboxRepository(pool *pgxpool.Pool) *InvalidationOutboxRepository {
	return &InvalidationOutboxRepository{pool: pool}
}

const listUnprocessedSQL = `
SELECT id, rule_id, owner_type, owner_id,
       affected_dims->>'country',
       affected_dims->>'operator',
       affected_dims->>'sender_category',
       affected_dims->>'traffic_type',
       created_at
FROM resolved_rules_invalidation_outbox
WHERE processed_at IS NULL
ORDER BY id
LIMIT $1;
`

func (r *InvalidationOutboxRepository) ListUnprocessed(ctx context.Context, limit int) ([]domain.InvalidationEvent, error) {
	rows, err := r.pool.Query(ctx, listUnprocessedSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.InvalidationEvent
	for rows.Next() {
		var ev domain.InvalidationEvent
		if err := rows.Scan(&ev.ID, &ev.RuleID, &ev.OwnerType, &ev.OwnerID,
			&ev.Country, &ev.Operator, &ev.SenderCat, &ev.Traffic, &ev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (r *InvalidationOutboxRepository) MarkProcessed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE resolved_rules_invalidation_outbox SET processed_at=now() WHERE id = ANY($1)`, ids)
	return err
}
```

- [ ] **Step 6.8: Запустить все repo-тесты**

```bash
go test -tags=integration ./internal/services/tarification/infrastructure/repository/ -v
```

Expected: все PASS.

- [ ] **Step 6.9: Commit**

```bash
git add internal/services/tarification/infrastructure/repository/resolved_rules_repository.go \
        internal/services/tarification/infrastructure/repository/price_rules_version_repository.go \
        internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository.go \
        internal/services/tarification/infrastructure/repository/invalidation_outbox_repository.go \
        internal/services/tarification/infrastructure/repository/resolved_rules_repository_integration_test.go \
        internal/services/tarification/infrastructure/repository/subaccount_usage_counter_repository_integration_test.go
git commit -m "feat(tarification): implement resolved rules, version, usage counter, outbox repos"
```

---

### Task 7: `TiersCache` — in-process LRU

**Files:**
- Create: `internal/services/tarification/application/tiers_cache.go`
- Test: `internal/services/tarification/application/tiers_cache_test.go`

Используем стандартный подход: `github.com/hashicorp/golang-lru/v2` (проверить: есть ли в go.mod; если нет — `go get`).

- [ ] **Step 7.1: Проверить наличие LRU-зависимости**

```bash
grep "hashicorp/golang-lru" go.mod
```

Если нет — добавить: `go get github.com/hashicorp/golang-lru/v2@latest`.

- [ ] **Step 7.2: Написать тест**

```go
// internal/services/tarification/application/tiers_cache_test.go
package application

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTiersCache_ParsesOnce(t *testing.T) {
	cache, err := NewTiersCache(128)
	require.NoError(t, err)

	ruleID := uuid.New()
	raw := []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`)
	v := int64(1)

	spec1, err := cache.GetOrParseTiered(ruleID, v, raw)
	require.NoError(t, err)
	require.Len(t, spec1.Tiers, 2)

	// Второй раз — тот же object (кеш-hit)
	spec2, err := cache.GetOrParseTiered(ruleID, v, raw)
	require.NoError(t, err)
	require.Same(t, spec1, spec2)
}

func TestTiersCache_InvalidatedByVersion(t *testing.T) {
	cache, err := NewTiersCache(128)
	require.NoError(t, err)

	ruleID := uuid.New()
	rawV1 := []byte(`{"period":"calendar_month","tiers":[{"up_to":null,"price":1.0}]}`)
	rawV2 := []byte(`{"period":"calendar_month","tiers":[{"up_to":null,"price":2.0}]}`)

	s1, _ := cache.GetOrParseTiered(ruleID, 1, rawV1)
	s2, _ := cache.GetOrParseTiered(ruleID, 2, rawV2)
	require.NotSame(t, s1, s2)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rawV2, &parsed))
}
```

- [ ] **Step 7.3: Реализация**

```go
// internal/services/tarification/application/tiers_cache.go
package application

import (
	"encoding/json"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/google/uuid"
)

type TieredSpec struct {
	Period string       `json:"period"`
	Tiers  []TieredTier `json:"tiers"`
}

type TieredTier struct {
	UpTo  *int64  `json:"up_to"` // null = infinity
	Price float64 `json:"price"`
}

type PrepaidThresholdSpec struct {
	Period            string  `json:"period"`
	PrepaidAmount     float64 `json:"prepaid_amount"`
	IncludedSegments  int64   `json:"included_segments"`
	OveragePrice      float64 `json:"overage_price"`
}

type cacheKey struct {
	RuleID  uuid.UUID
	Version int64
}

type TiersCache struct {
	tiered   *lru.Cache[cacheKey, *TieredSpec]
	prepaid  *lru.Cache[cacheKey, *PrepaidThresholdSpec]
}

func NewTiersCache(size int) (*TiersCache, error) {
	t, err := lru.New[cacheKey, *TieredSpec](size)
	if err != nil {
		return nil, err
	}
	p, err := lru.New[cacheKey, *PrepaidThresholdSpec](size)
	if err != nil {
		return nil, err
	}
	return &TiersCache{tiered: t, prepaid: p}, nil
}

func (c *TiersCache) GetOrParseTiered(ruleID uuid.UUID, version int64, raw []byte) (*TieredSpec, error) {
	k := cacheKey{ruleID, version}
	if v, ok := c.tiered.Get(k); ok {
		return v, nil
	}
	var spec TieredSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse tiered spec: %w", err)
	}
	c.tiered.Add(k, &spec)
	return &spec, nil
}

func (c *TiersCache) GetOrParsePrepaid(ruleID uuid.UUID, version int64, raw []byte) (*PrepaidThresholdSpec, error) {
	k := cacheKey{ruleID, version}
	if v, ok := c.prepaid.Get(k); ok {
		return v, nil
	}
	var spec PrepaidThresholdSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse prepaid spec: %w", err)
	}
	c.prepaid.Add(k, &spec)
	return &spec, nil
}
```

- [ ] **Step 7.4: Запустить тест**

```bash
go test ./internal/services/tarification/application/ -run TestTiersCache -v
```

Expected: PASS.

- [ ] **Step 7.5: Commit**

```bash
git add internal/services/tarification/application/tiers_cache.go \
        internal/services/tarification/application/tiers_cache_test.go \
        go.mod go.sum
git commit -m "feat(tarification): add in-process LRU cache for parsed tiers"
```

---

### Task 8: `CostCalculator`

**Files:**
- Create: `internal/services/tarification/application/cost_calculator.go`
- Test: `internal/services/tarification/application/cost_calculator_test.go`

- [ ] **Step 8.1: Написать тест**

```go
// internal/services/tarification/application/cost_calculator_test.go
package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/your-module-path/internal/services/tarification/domain"
)

func TestCostCalculator_Fixed(t *testing.T) {
	cache, _ := NewTiersCache(8)
	calc := NewCostCalculator(cache)

	price := "1.5"
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelFixed,
		PriceValue:   &price,
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 0, 3)
	require.NoError(t, err)
	require.InDelta(t, 4.5, cost, 1e-9)
}

func TestCostCalculator_Tiered_Progressive_AllInOneTier(t *testing.T) {
	cache, _ := NewTiersCache(8)
	calc := NewCostCalculator(cache)

	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 0, 100)
	require.NoError(t, err)
	require.InDelta(t, 100.0, cost, 1e-9)
}

func TestCostCalculator_Tiered_Progressive_CrossingBoundary(t *testing.T) {
	cache, _ := NewTiersCache(8)
	calc := NewCostCalculator(cache)

	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=9950, segments=100 → 50 по 1.0 + 50 по 0.5 = 75
	cost, err := calc.Calculate(rr, 9950, 100)
	require.NoError(t, err)
	require.InDelta(t, 75.0, cost, 1e-9)
}

func TestCostCalculator_Prepaid_WithinLimit(t *testing.T) {
	cache, _ := NewTiersCache(8)
	calc := NewCostCalculator(cache)

	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelPrepaidThreshold,
		TiersJSON:    []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 4000, 500)
	require.NoError(t, err)
	require.InDelta(t, 0.0, cost, 1e-9)
}

func TestCostCalculator_Prepaid_Overage(t *testing.T) {
	cache, _ := NewTiersCache(8)
	calc := NewCostCalculator(cache)

	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelPrepaidThreshold,
		TiersJSON:    []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=4900, segments=200 → 100 бесплатных, 100 по 0.5 = 50
	cost, err := calc.Calculate(rr, 4900, 200)
	require.NoError(t, err)
	require.InDelta(t, 50.0, cost, 1e-9)
}
```

- [ ] **Step 8.2: Реализация**

```go
// internal/services/tarification/application/cost_calculator.go
package application

import (
	"fmt"
	"strconv"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type CostCalculator struct {
	cache *TiersCache
}

func NewCostCalculator(cache *TiersCache) *CostCalculator {
	return &CostCalculator{cache: cache}
}

// Calculate возвращает стоимость (в валюте платформы) за данный batch сегментов.
// usageBefore — segments_used ДО текущего сообщения (для tiered/prepaid).
func (c *CostCalculator) Calculate(rr *domain.ResolvedRule, usageBefore, segments int64) (float64, error) {
	switch rr.PriceModel {
	case domain.ModelFixed:
		if rr.PriceValue == nil {
			return 0, fmt.Errorf("fixed rule %s has nil price_value", rr.SourceRuleID)
		}
		v, err := strconv.ParseFloat(*rr.PriceValue, 64)
		if err != nil {
			return 0, fmt.Errorf("parse price_value: %w", err)
		}
		return v * float64(segments), nil

	case domain.ModelTiered:
		spec, err := c.cache.GetOrParseTiered(rr.SourceRuleID, rr.RulesVersion, rr.TiersJSON)
		if err != nil {
			return 0, err
		}
		return walkTiersProgressive(spec.Tiers, usageBefore, segments), nil

	case domain.ModelPrepaidThreshold:
		spec, err := c.cache.GetOrParsePrepaid(rr.SourceRuleID, rr.RulesVersion, rr.TiersJSON)
		if err != nil {
			return 0, err
		}
		newUsage := usageBefore + segments
		if newUsage <= spec.IncludedSegments {
			return 0, nil
		}
		// overage — только сегменты СВЕРХ лимита
		overageSegments := newUsage - spec.IncludedSegments
		if usageBefore > spec.IncludedSegments {
			overageSegments = segments // всё сверх лимита
		}
		return spec.OveragePrice * float64(overageSegments), nil

	default:
		return 0, fmt.Errorf("unknown price_model: %s", rr.PriceModel)
	}
}

// walkTiersProgressive: считает прогрессивно. Каждый сегмент оплачивается
// по тиру, в котором он находится. Тиры упорядочены по up_to возрастанию;
// последний up_to=nil означает бесконечность.
func walkTiersProgressive(tiers []TieredTier, usageBefore, segments int64) float64 {
	total := 0.0
	remaining := segments
	cursor := usageBefore
	for _, tier := range tiers {
		if remaining == 0 {
			break
		}
		var capacity int64
		if tier.UpTo == nil {
			capacity = remaining // бесконечный тир
		} else {
			capacity = *tier.UpTo - cursor
			if capacity < 0 {
				capacity = 0
			}
		}
		take := remaining
		if take > capacity {
			take = capacity
		}
		if take > 0 {
			total += tier.Price * float64(take)
			remaining -= take
			cursor += take
		}
	}
	return total
}
```

- [ ] **Step 8.3: Запустить тесты**

```bash
go test ./internal/services/tarification/application/ -run TestCostCalculator -v
```

Expected: PASS.

- [ ] **Step 8.4: Commit**

```bash
git add internal/services/tarification/application/cost_calculator.go \
        internal/services/tarification/application/cost_calculator_test.go
git commit -m "feat(tarification): implement CostCalculator for fixed/tiered/prepaid models"
```

---

### Task 9: `PriceResolver`

**Files:**
- Create: `internal/services/tarification/application/price_resolver.go`
- Test: `internal/services/tarification/application/price_resolver_test.go` (unit с mock-repo)

- [ ] **Step 9.1: Написать unit-тест с моками**

```go
// internal/services/tarification/application/price_resolver_test.go
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type fakeRuleRepo struct {
	byLookup *domain.PriceRule
	err      error
}

func (f *fakeRuleRepo) FindApplicable(_ context.Context, _ domain.ResolveInput) (*domain.PriceRule, error) {
	return f.byLookup, f.err
}
func (f *fakeRuleRepo) Create(context.Context, *domain.PriceRule) error           { return nil }
func (f *fakeRuleRepo) Update(context.Context, *domain.PriceRule) error           { return nil }
func (f *fakeRuleRepo) Delete(context.Context, uuid.UUID) error                   { return nil }
func (f *fakeRuleRepo) GetByID(context.Context, uuid.UUID) (*domain.PriceRule, error) { return nil, nil }
func (f *fakeRuleRepo) HasPlatformCatchAll(context.Context) (bool, error)         { return true, nil }

type fakeResolvedRepo struct {
	cached   *domain.ResolvedRule
	upserted *domain.ResolvedRule
}

func (f *fakeResolvedRepo) Get(context.Context, uuid.UUID, string, string, string, string, time.Time) (*domain.ResolvedRule, error) {
	return f.cached, nil
}
func (f *fakeResolvedRepo) Upsert(_ context.Context, r *domain.ResolvedRule) error {
	f.upserted = r
	return nil
}
func (f *fakeResolvedRepo) DeleteAffected(context.Context, domain.PriceOwnerType, *uuid.UUID, *string, *string, *string, *string) (int64, error) {
	return 0, nil
}

type fakeVersionRepo struct{ v int64 }

func (f *fakeVersionRepo) GetVersion(context.Context) (int64, error) { return f.v, nil }

type fakeAggResolver struct {
	aggID uuid.UUID
}

func (f *fakeAggResolver) AggregatorFor(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return f.aggID, nil
}

func TestPriceResolver_Resolve_CacheHit_SameVersion(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()
	ruleID := uuid.New()

	cached := &domain.ResolvedRule{
		SubaccountID: subID, AggregatorID: aggID,
		Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		EffectiveDate: time.Now().UTC().Truncate(24 * time.Hour),
		PriceModel: domain.ModelFixed, PriceValue: ptrStr("1.5"),
		SourceRuleID: ruleID, SourceLevel: domain.OwnerPlatform,
		RulesVersion: 5,
	}
	resolver := NewPriceResolver(
		&fakeRuleRepo{},
		&fakeResolvedRepo{cached: cached},
		&fakeVersionRepo{v: 5},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, ruleID, rr.SourceRuleID)
}

func TestPriceResolver_Resolve_CacheMiss_FallsBackToLookup(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()
	ruleID := uuid.New()

	rule := &domain.PriceRule{
		ID: ruleID, OwnerType: domain.OwnerPlatform,
		PriceModel: domain.ModelFixed, PriceValue: ptrStr("2.0"),
		ValidFrom: time.Now().Add(-time.Hour),
	}
	resolvedRepo := &fakeResolvedRepo{cached: nil}
	resolver := NewPriceResolver(
		&fakeRuleRepo{byLookup: rule},
		resolvedRepo,
		&fakeVersionRepo{v: 10},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, ruleID, rr.SourceRuleID)
	require.NotNil(t, resolvedRepo.upserted)
	require.Equal(t, int64(10), resolvedRepo.upserted.RulesVersion)
	require.Equal(t, aggID, resolvedRepo.upserted.AggregatorID)
}

func TestPriceResolver_Resolve_StaleVersion_Refreshes(t *testing.T) {
	subID := uuid.New()
	aggID := uuid.New()

	cached := &domain.ResolvedRule{
		SubaccountID: subID, AggregatorID: aggID,
		RulesVersion: 3,
		// ... stale
	}
	newRule := &domain.PriceRule{
		ID: uuid.New(), OwnerType: domain.OwnerPlatform,
		PriceModel: domain.ModelFixed, PriceValue: ptrStr("3.0"),
		ValidFrom: time.Now().Add(-time.Hour),
	}
	resolvedRepo := &fakeResolvedRepo{cached: cached}
	resolver := NewPriceResolver(
		&fakeRuleRepo{byLookup: newRule},
		resolvedRepo,
		&fakeVersionRepo{v: 10},
		&fakeAggResolver{aggID: aggID},
	)

	rr, err := resolver.Resolve(context.Background(), domain.ResolveInput{
		SubaccountID: subID, Country: "RU", Operator: "MTS",
		SenderCategory: "paid", TrafficType: "transactional",
		Now: time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, newRule.ID, rr.SourceRuleID)
	require.NotNil(t, resolvedRepo.upserted)
}

func ptrStr(s string) *string { return &s }
```

- [ ] **Step 9.2: Реализация**

```go
// internal/services/tarification/application/price_resolver.go
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/your-module-path/internal/services/tarification/domain"
)

// AggregatorResolver — минимальный интерфейс для получения aggregator_id по subaccount_id.
// В проде реализуется через Redis-cache + fallback на clients-таблицу.
type AggregatorResolver interface {
	AggregatorFor(ctx context.Context, subaccountID uuid.UUID) (uuid.UUID, error)
}

type PriceResolver struct {
	ruleRepo     domain.PriceRuleRepository
	resolvedRepo domain.ResolvedRulesRepository
	versionRepo  domain.PriceRulesVersionRepository
	aggResolver  AggregatorResolver
}

func NewPriceResolver(
	ruleRepo domain.PriceRuleRepository,
	resolvedRepo domain.ResolvedRulesRepository,
	versionRepo domain.PriceRulesVersionRepository,
	aggResolver AggregatorResolver,
) *PriceResolver {
	return &PriceResolver{
		ruleRepo: ruleRepo, resolvedRepo: resolvedRepo,
		versionRepo: versionRepo, aggResolver: aggResolver,
	}
}

func (r *PriceResolver) Resolve(ctx context.Context, in domain.ResolveInput) (*domain.ResolvedRule, error) {
	aggID, err := r.aggResolver.AggregatorFor(ctx, in.SubaccountID)
	if err != nil {
		return nil, fmt.Errorf("resolve aggregator: %w", err)
	}
	in.AggregatorID = aggID

	effectiveDate := in.Now.UTC().Truncate(24 * time.Hour)

	// Стадия 1 — cache hit?
	cached, err := r.resolvedRepo.Get(ctx, in.SubaccountID,
		in.Country, in.Operator, in.SenderCategory, in.TrafficType, effectiveDate)
	if err != nil {
		return nil, fmt.Errorf("get resolved: %w", err)
	}
	currentVersion, err := r.versionRepo.GetVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	if cached != nil && cached.RulesVersion == currentVersion {
		return cached, nil
	}

	// Стадия 2 — lookup + materialize
	versionAtSelect := currentVersion
	rule, err := r.ruleRepo.FindApplicable(ctx, in)
	if err != nil {
		return nil, err
	}

	rr := &domain.ResolvedRule{
		SubaccountID:   in.SubaccountID,
		Country:        in.Country,
		Operator:       in.Operator,
		SenderCategory: in.SenderCategory,
		TrafficType:    in.TrafficType,
		EffectiveDate:  effectiveDate,
		PriceModel:     rule.PriceModel,
		PriceValue:     rule.PriceValue,
		TiersJSON:      rule.TiersJSON,
		SourceRuleID:   rule.ID,
		SourceLevel:    rule.OwnerType,
		AggregatorID:   aggID,
		RulesVersion:   versionAtSelect,
	}
	if err := r.resolvedRepo.Upsert(ctx, rr); err != nil {
		return nil, fmt.Errorf("upsert resolved: %w", err)
	}
	return rr, nil
}
```

- [ ] **Step 9.3: Запустить тесты**

```bash
go test ./internal/services/tarification/application/ -run TestPriceResolver -v
```

Expected: PASS.

- [ ] **Step 9.4: Commit**

```bash
git add internal/services/tarification/application/price_resolver.go \
        internal/services/tarification/application/price_resolver_test.go
git commit -m "feat(tarification): implement PriceResolver with cache hit/miss logic"
```

---

### Task 10: `ResolvedRulesJanitor` воркер

**Files:**
- Create: `internal/services/tarification/application/resolved_rules_janitor.go`
- Test: `internal/services/tarification/application/resolved_rules_janitor_test.go`

- [ ] **Step 10.1: Написать unit-тест**

```go
// internal/services/tarification/application/resolved_rules_janitor_test.go
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type fakeOutbox struct {
	events    []domain.InvalidationEvent
	processed []int64
}

func (f *fakeOutbox) ListUnprocessed(_ context.Context, limit int) ([]domain.InvalidationEvent, error) {
	if limit >= len(f.events) {
		out := f.events
		f.events = nil
		return out, nil
	}
	out := f.events[:limit]
	f.events = f.events[limit:]
	return out, nil
}

func (f *fakeOutbox) MarkProcessed(_ context.Context, ids []int64) error {
	f.processed = append(f.processed, ids...)
	return nil
}

type recordingResolvedRepo struct {
	deleteCalls []string
}

func (r *recordingResolvedRepo) Get(context.Context, uuid.UUID, string, string, string, string, time.Time) (*domain.ResolvedRule, error) {
	return nil, nil
}
func (r *recordingResolvedRepo) Upsert(context.Context, *domain.ResolvedRule) error { return nil }
func (r *recordingResolvedRepo) DeleteAffected(_ context.Context, ot domain.PriceOwnerType, oid *uuid.UUID, c, o, cat, tr *string) (int64, error) {
	key := string(ot)
	if oid != nil {
		key += "/" + oid.String()
	}
	r.deleteCalls = append(r.deleteCalls, key)
	return 1, nil
}

func TestJanitor_ProcessesAllEvents(t *testing.T) {
	subID := uuid.New()
	outbox := &fakeOutbox{events: []domain.InvalidationEvent{
		{ID: 1, OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RuleID: uuid.New()},
		{ID: 2, OwnerType: domain.OwnerPlatform, RuleID: uuid.New()},
	}}
	resolved := &recordingResolvedRepo{}
	j := NewResolvedRulesJanitor(outbox, resolved, 10)

	require.NoError(t, j.RunOnce(context.Background()))
	require.Equal(t, []int64{1, 2}, outbox.processed)
	require.Len(t, resolved.deleteCalls, 2)
}
```

- [ ] **Step 10.2: Реализация**

```go
// internal/services/tarification/application/resolved_rules_janitor.go
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/your-module-path/internal/services/tarification/domain"
)

type ResolvedRulesJanitor struct {
	outbox       domain.InvalidationOutboxRepository
	resolvedRepo domain.ResolvedRulesRepository
	batchSize    int
}

func NewResolvedRulesJanitor(
	outbox domain.InvalidationOutboxRepository,
	resolvedRepo domain.ResolvedRulesRepository,
	batchSize int,
) *ResolvedRulesJanitor {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &ResolvedRulesJanitor{outbox: outbox, resolvedRepo: resolvedRepo, batchSize: batchSize}
}

// RunOnce обрабатывает один батч событий. Вызывать циклически из runner.
func (j *ResolvedRulesJanitor) RunOnce(ctx context.Context) error {
	events, err := j.outbox.ListUnprocessed(ctx, j.batchSize)
	if err != nil {
		return fmt.Errorf("list unprocessed: %w", err)
	}
	if len(events) == 0 {
		return nil
	}
	var processed []int64
	for _, ev := range events {
		if _, err := j.resolvedRepo.DeleteAffected(ctx,
			ev.OwnerType, ev.OwnerID, ev.Country, ev.Operator, ev.SenderCat, ev.Traffic,
		); err != nil {
			// один фейл не должен остановить весь батч; логируем и пропускаем.
			// В этой фазе достаточно логировать через stderr; интеграция с проектным логгером — следующая итерация.
			fmt.Printf("[janitor] delete_affected failed for event %d: %v\n", ev.ID, err)
			continue
		}
		processed = append(processed, ev.ID)
	}
	if len(processed) > 0 {
		if err := j.outbox.MarkProcessed(ctx, processed); err != nil {
			return fmt.Errorf("mark processed: %w", err)
		}
	}
	return nil
}

// Run — loop с фиксированным интервалом. Остановка по ctx.Done.
func (j *ResolvedRulesJanitor) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = j.RunOnce(ctx)
		}
	}
}
```

- [ ] **Step 10.3: Запустить тест**

```bash
go test ./internal/services/tarification/application/ -run TestJanitor -v
```

Expected: PASS.

- [ ] **Step 10.4: Commit**

```bash
git add internal/services/tarification/application/resolved_rules_janitor.go \
        internal/services/tarification/application/resolved_rules_janitor_test.go
git commit -m "feat(tarification): add ResolvedRulesJanitor worker"
```

---

### Task 11: Startup invariant check + фича-флаг в конфиге

**Files:**
- Modify: конфиг-файл tarification-service (найти путь через `grep -r "viper.Unmarshal\|Config" cmd/tarification`)
- Modify: main tarification-service (`cmd/tarification-service/main.go` — проверить реальный путь)

- [ ] **Step 11.1: Найти main и config**

```bash
find cmd -type d -name "*tarification*"
find . -path "*cmd*tarification*" -name "main.go"
grep -r "UnifiedTarificationEnabled\|TarificationConfig" --include="*.go" -l | head -5
```

Запомнить реальные пути.

- [ ] **Step 11.2: Добавить флаг в структуру Config**

В файле конфига сервиса tarification (типично `internal/services/tarification/config.go` или аналогичный):

```go
type Config struct {
    // ... существующие поля
    UnifiedTarificationEnabled bool `mapstructure:"unified_tarification_enabled"`
    TiersCacheSize             int  `mapstructure:"tiers_cache_size"`
    JanitorIntervalSeconds     int  `mapstructure:"janitor_interval_seconds"`
}
```

Дефолты в viper-инициализации:

```go
viper.SetDefault("unified_tarification_enabled", false)
viper.SetDefault("tiers_cache_size", 1024)
viper.SetDefault("janitor_interval_seconds", 30)
```

- [ ] **Step 11.3: Startup invariant check + wire-up**

В main/инициализации сервиса:

```go
// после создания pgxpool.Pool
priceRuleRepo := repository.NewPriceRuleRepository(pool)

if cfg.UnifiedTarificationEnabled {
    has, err := priceRuleRepo.HasPlatformCatchAll(ctx)
    if err != nil {
        logger.Fatal().Err(err).Msg("check platform catch-all")
    }
    if !has {
        logger.Fatal().Msg("unified tarification enabled but platform catch-all price rule is missing — service cannot start")
    }

    // Init остальных компонентов
    resolvedRepo := repository.NewResolvedRulesRepository(pool)
    versionRepo := repository.NewPriceRulesVersionRepository(pool)
    usageRepo := repository.NewSubaccountUsageCounterRepository(pool)
    outboxRepo := repository.NewInvalidationOutboxRepository(pool)

    tiersCache, err := application.NewTiersCache(cfg.TiersCacheSize)
    if err != nil {
        logger.Fatal().Err(err).Msg("init tiers cache")
    }
    _ = application.NewCostCalculator(tiersCache)
    _ = application.NewPriceResolver(priceRuleRepo, resolvedRepo, versionRepo, aggResolver)

    janitor := application.NewResolvedRulesJanitor(outboxRepo, resolvedRepo, 100)
    go janitor.Run(ctx, time.Duration(cfg.JanitorIntervalSeconds)*time.Second)

    logger.Info().Msg("unified tarification mode ENABLED (read-only, hot path still uses legacy)")
}
```

Заметка: `aggResolver` — реализацию напишем в Task 12. Пока можно временно nil-подменить, так как в Phase 1 `UnifiedTarificationEnabled=false` по дефолту, и этот блок не исполняется.

- [ ] **Step 11.4: Проверить сборку**

```bash
./scripts/check.sh
```

Expected: Go build/vet — pass (на Windows с Device Guard будет `[SKIP]`, это норм; CI проверит).

- [ ] **Step 11.5: Commit**

```bash
git add <config-file> <main-file>
git commit -m "feat(tarification): add feature flag and wire unified pricing components"
```

---

### Task 12: `AggregatorResolver` — реализация через Redis + fallback на DB

**Files:**
- Create: `internal/services/tarification/infrastructure/aggregator_resolver.go`
- Test: `internal/services/tarification/infrastructure/aggregator_resolver_test.go` (integration — требует Redis + PG)

- [ ] **Step 12.1: Посмотреть, как в проекте получают aggregator_id по subaccount_id**

```bash
grep -rn "aggregator_id\|AggregatorID" internal/services/client --include="*.go" | head -20
grep -rn "sub_account\|subaccount" internal/services/client/domain --include="*.go" | head -10
```

Запомнить, в какой таблице/сервисе хранится связь (обычно `sub_accounts(id, parent_client_id)` или `clients(parent_client_id)`).

- [ ] **Step 12.2: Написать интеграционный тест**

```go
//go:build integration

package infrastructure

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAggregatorResolver_ResolvesFromDB(t *testing.T) {
	pool := setupPool(t)
	redisClient := setupRedis(t)
	ctx := context.Background()

	aggID := uuid.New()
	subID := uuid.New()

	// Установить pool: создать клиента-агрегатора и субаккаунт
	// (использовать существующую схему — имена таблиц/колонок взять по шагу 12.1)
	_, err := pool.Exec(ctx, `INSERT INTO clients (id, name, ...) VALUES ($1, 'agg', ...)`, aggID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sub_accounts (id, parent_client_id, ...) VALUES ($1, $2, ...)`, subID, aggID)
	require.NoError(t, err)

	resolver := NewAggregatorResolver(pool, redisClient, 300)
	got, err := resolver.AggregatorFor(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, aggID, got)

	// Второй вызов — из кеша
	got2, err := resolver.AggregatorFor(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, aggID, got2)
}
```

SQL в тесте — подогнать под реальную схему.

- [ ] **Step 12.3: Реализация**

```go
// internal/services/tarification/infrastructure/aggregator_resolver.go
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type AggregatorResolver struct {
	pool   *pgxpool.Pool
	redis  *redis.Client
	ttlSec int
}

func NewAggregatorResolver(pool *pgxpool.Pool, rdb *redis.Client, ttlSeconds int) *AggregatorResolver {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return &AggregatorResolver{pool: pool, redis: rdb, ttlSec: ttlSeconds}
}

const aggregatorLookupSQL = `
SELECT parent_client_id
FROM sub_accounts
WHERE id = $1;
`
// ПРИМЕЧАНИЕ: если реальная схема иная (например, clients.parent_client_id),
// скорректировать SQL по шагу 12.1.

func (r *AggregatorResolver) AggregatorFor(ctx context.Context, subID uuid.UUID) (uuid.UUID, error) {
	key := fmt.Sprintf("client:%s:aggregator_id", subID)
	if r.redis != nil {
		v, err := r.redis.Get(ctx, key).Result()
		if err == nil {
			id, perr := uuid.Parse(v)
			if perr == nil {
				return id, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			// продолжаем с DB
		}
	}
	var aggID uuid.UUID
	err := r.pool.QueryRow(ctx, aggregatorLookupSQL, subID).Scan(&aggID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("subaccount %s has no parent client", subID)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if r.redis != nil {
		_ = r.redis.Set(ctx, key, aggID.String(), time.Duration(r.ttlSec)*time.Second).Err()
	}
	return aggID, nil
}
```

- [ ] **Step 12.4: Запустить тест**

```bash
go test -tags=integration ./internal/services/tarification/infrastructure/ -run TestAggregatorResolver -v
```

Expected: PASS.

- [ ] **Step 12.5: Commit**

```bash
git add internal/services/tarification/infrastructure/aggregator_resolver.go \
        internal/services/tarification/infrastructure/aggregator_resolver_test.go
git commit -m "feat(tarification): add AggregatorResolver with Redis cache + DB fallback"
```

---

### Task 13: Прогон всех тестов + финальный commit Phase 1

- [ ] **Step 13.1: Полный прогон**

```bash
./scripts/check.sh --with-tests
```

Expected: pass (на Windows go-чеки skip, остальное проходит).

- [ ] **Step 13.2: Интеграционные тесты (если не включены в check.sh)**

```bash
go test -tags=integration ./internal/services/tarification/... -v
```

Expected: все PASS.

- [ ] **Step 13.3: Финальная проверка — миграция up/down**

```bash
./scripts/server.sh exec "cd /opt/sms && migrate -path migrations -database \$DATABASE_URL down 1"
./scripts/server.sh exec "cd /opt/sms && migrate -path migrations -database \$DATABASE_URL up"
```

Expected: миграция 000104 откатывается и накатывается чисто.

- [ ] **Step 13.4: Оставить PR-заметку для Phase 2**

Добавить в конец `docs/superpowers/plans/2026-04-18-unified-pricing-model.md` (этого файла) список follow-ups (уже описан ниже).

- [ ] **Step 13.5: Финальный обзорный коммит (если осталось что-то несобранное)**

```bash
git status
# Если всё закоммичено — ничего не делаем.
```

Phase 1 закрыта.

---

## Phase 2 — Backfill + dual-write (отдельная сессия)

**Цель:** Перенести данные из legacy-таблиц в `price_rules`, включить двойную запись, сверять результаты с `PriceResolver` в тени.

### Задачи высокого уровня

1. **Backfill-скрипт `scripts/backfill_price_rules.go`** — однократный импорт:
   - `tariff_plans` + `tariff_periods` + `tariff_tiers` → `price_rules` (owner_type=platform).
   - `tariff_periods_new` → `price_rules` (owner_type=platform, маппинг scope → измерения).
   - `aggregator_tariffs` с `sub_account_id IS NULL` → owner_type=aggregator.
   - `aggregator_tariffs` с `sub_account_id != NULL` → owner_type=subaccount.
   - `reseller_tariff_plans` с назначениями → соответствующий owner_type.
   - Обработка overlapping periods: сортировка по `created_at`, авто-закрытие предыдущего (`valid_to = new.valid_from`), лог случаев.
   - Pre-flight audit: NULL-invariants в legacy, сломанные FK, дубли.

2. **Dual-write в админ-API** (пишет в обе системы в одной DB-транзакции):
   - Найти handler-ы, которые правят `tariff_plans`/`aggregator_tariffs`/`reseller_tariff_plans`.
   - Для каждого — после legacy-записи вставить/обновить соответствующее правило в `price_rules`.
   - Фейл любой → rollback всей транзакции.

3. **Reconciliation-job** (раз в сутки):
   - Для каждого активного субаккаунта × (страна, оператор, категория, трафик) — сверяем цену legacy vs `PriceResolver`.
   - Расхождения → метрика в Prometheus + запись в `reconciliation_diff_log` (создать таблицу).
   - Алерт в Grafana при count > 0.

**Как запустить эту фазу:**

```
Читай docs/superpowers/specs/2026-04-18-unified-pricing-model-design.md раздел "Фаза 2".
Phase 1 завершена (миграция 000104 применена, price_rules и компаньон-таблицы существуют, PriceResolver+JanitorUnder feature flag ready).

Задачи:
1. Написать backfill-скрипт с pre-flight audit и обработкой overlapping periods.
2. Прогнать backfill на staging с копией продакшен-данных. Проверить инвариант catch-all.
3. Включить dual-write в admin-API (атомарно).
4. Добавить reconciliation-job + Grafana-панель.
5. Оставить работать на проде 1-2 недели, мониторить reconciliation diff.

Работа через /execute-with-review — обязательный review каждой задачи.
```

---

## Phase 3 — Cutover чтения (отдельная сессия)

**Цель:** Горячий путь читает из `price_rules`/`resolved_rules`. Legacy dual-write продолжает работать как страховка.

### Задачи

1. Реализовать новую ветку в `tarification_service.TarifyMessage` под `UnifiedTarificationEnabled=true`:
   - `PriceResolver.Resolve` → `CostCalculator.Calculate` → UPSERT `subaccount_usage_counters` → `saga.Charge`.
   - Аггрегатор-маржа: второй resolve с owner=aggregator (исключая субаккаунт override) + дельта для `aggregator_margin_log`.
   - Идемпотентность через существующий `tarification_log` по ключу.
2. Добавить метрики: `cache_hit_rate`, `resolve_latency_p99`, `price_not_found_total` (алерт если > 0), `margin_per_hour_rub`.
3. Включить `UnifiedTarificationEnabled=true` на staging, прогнать нагрузочный тест.
4. Поэтапное включение в проде: 1% трафика (через consistent hashing по subaccount_id) → 10% → 50% → 100%.
5. Наблюдать 1-2 недели.

**Как запустить:**
```
Читай спек + план. Phase 2 завершена (dual-write работает, reconciliation чистый).
Задача: реализовать новый hot path, включить поэтапно. Через /execute-with-review.
```

---

## Phase 4 — Contract (отдельная сессия)

**Цель:** Снос legacy.

1. Отключить dual-write в админ-API.
2. Переименовать legacy-таблицы: `tariff_plans` → `tariff_plans_deprecated_20260601` и т.д.
3. Удалить legacy-код: `tariff_plan_service`, `aggregator_tariffs` repo, `reseller_tariff_plans` repo.
4. Оставить на 2 недели без инцидентов.
5. Финальный DROP `*_deprecated_*`.

**Как запустить:**
```
Читай спек + план. Phase 3 работает на 100% трафика ≥2 недели, инцидентов нет.
Задача: снос legacy. Через /execute-with-review.
```

---

## Примечания для исполнителя Phase 1

1. **Go-module path** (`github.com/your-module-path`) в каждом файле — заменить на фактический из `go.mod` (первая строка `module ...`).
2. **`setupPool` helper** в integration-тестах — скопировать из существующих `*_integration_test.go` в `internal/services/tarification/infrastructure/repository/` (например, из `tariff_period_repository_integration_test.go`, если такой есть). Не изобретать свой.
3. **Windows Device Guard**: `go vet`/`go build` могут падать локально, это норма — `scripts/check.sh` их скипает с `[SKIP]`. CI на Linux проверит.
4. **Миграция 000104**: следующий доступный номер проверен (последняя — 000103). Если в master прилетит что-то раньше — пересогласовать номер.
5. **Фича-флаг** `UnifiedTarificationEnabled` в Phase 1 остаётся `false`. Код нового пути существует, но не вызывается. Прод не затронут.
6. **Review**: по CLAUDE.md использовать `/execute-with-review` (обязательный код-ревью каждой задачи субагентом-ревьюером).
