# Hierarchical Tariff Periods Design

**Date:** 2026-04-09
**Status:** Approved

## Problem

Current tarification system has flat periods tied to `tariff_plans` (operator + sender_category). No ability to:
- Set global/country-level base prices
- Override prices for specific operators, traffic types, or clients
- Auto-close periods when creating new ones
- Create open-ended (no end date) periods

## Solution: Dimension-based periods with scope priority

Replace `tariff_plans` + `tariff_periods` with a single `tariff_periods` table that carries dimension columns and an auto-computed scope priority. Add analogous `provider_cost_periods` for provider cost tracking.

## Dimension Hierarchy

Priority (low to high):

| Level | Priority | Dimensions filled |
|-------|----------|-------------------|
| Global | 0 | all NULL |
| Country | 10 | country_id |
| Operator | 20 | country_id, operator_id |
| Sender category | 30 | country_id, operator_id, sender_category |
| Traffic type | 40 | country_id, operator_id, sender_category, traffic_type |
| + Client | +100 | any combination + client_id |

Client is a **parallel axis** — an override at any level. A period with `(country=KZ, client=Acme)` has priority 110 and overrides `(country=KZ)` at priority 10 for that client.

## Data Model

### tariff_periods (replaces tariff_plans + old tariff_periods)

```sql
CREATE TABLE tariff_periods (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- Dimensions (all nullable, NULL = "any")
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  sender_category TEXT,           -- shared / paid_registered / free_registered
  traffic_type    TEXT,           -- authorization / transactional / service / extensible
  client_id       UUID REFERENCES accounts(id),

  -- Scope
  scope_key       TEXT NOT NULL,  -- canonical key: sorted filled dimensions
  scope_priority  INT NOT NULL,   -- auto-computed from filled dimensions

  -- Strategy (moved from tariff_plans)
  strategy        TEXT NOT NULL,  -- fixed / threshold / threshold_recalc / prepaid_threshold

  -- Dates
  start_date      DATE NOT NULL,
  end_date        DATE,           -- NULL = open-ended

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_dates CHECK (end_date IS NULL OR end_date > start_date)
);

-- Overlap prevention within same scope
CREATE EXTENSION IF NOT EXISTS btree_gist;
ALTER TABLE tariff_periods ADD CONSTRAINT no_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, end_date, '[]') WITH &&
  );

-- Lookup index (hot path)
CREATE INDEX idx_tariff_periods_lookup ON tariff_periods (
  country_id, operator_id, sender_category, traffic_type, client_id,
  start_date, end_date
);

-- Scope-based queries
CREATE INDEX idx_tariff_periods_scope ON tariff_periods (scope_priority, scope_key);
```

### tariff_tiers (structure unchanged)

```sql
CREATE TABLE tariff_tiers (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tariff_period_id  UUID NOT NULL REFERENCES tariff_periods(id) ON DELETE CASCADE,
  from_count        INT NOT NULL CHECK (from_count >= 0),
  price_per_segment NUMERIC(20,6) NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tariff_period_id, from_count)
);
```

### provider_cost_periods (analogous, for cost analysis)

```sql
CREATE TABLE provider_cost_periods (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id     UUID NOT NULL REFERENCES providers(id),
  country_id      UUID REFERENCES countries(id),
  operator_id     UUID REFERENCES operators(id),
  traffic_type    TEXT,

  scope_key       TEXT NOT NULL,
  scope_priority  INT NOT NULL,
  strategy        TEXT NOT NULL,

  start_date      DATE NOT NULL,
  end_date        DATE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT valid_cost_dates CHECK (end_date IS NULL OR end_date > start_date)
);

ALTER TABLE provider_cost_periods ADD CONSTRAINT no_cost_overlap_in_scope
  EXCLUDE USING gist (
    scope_key WITH =,
    daterange(start_date, end_date, '[]') WITH &&
  );

CREATE TABLE provider_cost_tiers (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cost_period_id    UUID NOT NULL REFERENCES provider_cost_periods(id) ON DELETE CASCADE,
  from_count        INT NOT NULL CHECK (from_count >= 0),
  cost_per_segment  NUMERIC(20,6) NOT NULL,
  UNIQUE (cost_period_id, from_count)
);
```

No `sender_category` or `client_id` — provider costs don't depend on these.

Provider cost hierarchy (strict order):
- provider_id (always required, base level): priority 0
- country_id: priority 10
- operator_id (requires country_id): priority 20
- traffic_type (requires operator_id): priority 30

## scope_key and scope_priority Computation

`scope_key` is built from filled dimensions in canonical order:

```
country:{id}|operator:{id}|sender_category:{val}|traffic_type:{val}|client:{id}
```

Only filled dimensions are included. Examples:
- Global: `global`
- Country: `country:uuid-kz`
- Country + Operator: `country:uuid-kz|operator:uuid-beeline`
- Country + Client: `client:uuid-acme|country:uuid-kz`

`scope_priority` is computed based on the most specific filled dimension:
- all NULL: 0 (global)
- country_id: 10
- operator_id: 20
- sender_category: 30
- traffic_type: 40
- client_id: +100 (additive to any level)

**Strict hierarchy rule:** Lower dimensions require all higher ones to be filled:
- operator_id requires country_id
- sender_category requires operator_id (and thus country_id)
- traffic_type requires sender_category (and thus operator_id and country_id)
- client_id can be added at any level (parallel axis)

Attempting to create a period with e.g. `traffic_type` set but `operator_id` NULL is a validation error.

## Period Lifecycle

### Auto-close on new period creation

When creating a new period in scope_key `X` with `start_date = D`:

1. Find current period in same scope_key where `end_date IS NULL` or `end_date >= D`
2. If open-ended (`end_date IS NULL`): set `end_date = D - 1 day`
3. If has concrete `end_date >= D`: error (overlap with closed period)
4. New period is created (open-ended by default)

### Containment validation

When creating a period at level N, find the "parent" period — nearest ancestor in hierarchy:

**Finding parent:** Remove the most specific dimension from current set (order: traffic_type → sender_category → operator → country) and find an active period in the resulting scope covering the new period's dates.

Rules:
- If no parent found: error `"no active period at parent level for these dates"`
- If `new.start_date < parent.start_date`: error
- If `new.end_date > parent.end_date` (and parent is not open-ended): error
- Global level (scope_priority=0): no parent, no containment check
- Client override (+100): parent is the analogous scope without client_id

### Child protection on auto-close

When auto-closing a parent period (new period in same scope):
- Check all child periods (higher scope_priority, dimensions are superset of current)
- If any child has `end_date IS NULL` or `end_date > new parent end_date`: error `"close child periods first"`

### Constraints

- `start_date >= today` (no retroactive periods)
- Delete only if no child periods exist
- One period per scope per date range (exclusion constraint)

## Tariff Lookup Algorithm

### Finding applicable tariff

Given: country_id, operator_id, sender_category, traffic_type, client_id, date.

```sql
SELECT tp.id, tp.scope_priority
FROM tariff_periods tp
WHERE (tp.country_id = :country OR tp.country_id IS NULL)
  AND (tp.operator_id = :operator OR tp.operator_id IS NULL)
  AND (tp.sender_category = :sender_cat OR tp.sender_category IS NULL)
  AND (tp.traffic_type = :traffic_type OR tp.traffic_type IS NULL)
  AND (tp.client_id = :client OR tp.client_id IS NULL)
  AND tp.start_date <= :date
  AND (tp.end_date IS NULL OR tp.end_date >= :date)
ORDER BY tp.scope_priority DESC
LIMIT 1
```

### Tier inheritance

The matched period may have no tiers — inherit from the nearest ancestor that has tiers:

```sql
WITH matched_periods AS (
  SELECT tp.id, tp.scope_priority
  FROM tariff_periods tp
  WHERE (tp.country_id = :country OR tp.country_id IS NULL)
    AND (tp.operator_id = :operator OR tp.operator_id IS NULL)
    AND (tp.sender_category = :sender_cat OR tp.sender_category IS NULL)
    AND (tp.traffic_type = :traffic_type OR tp.traffic_type IS NULL)
    AND (tp.client_id = :client OR tp.client_id IS NULL)
    AND tp.start_date <= :date
    AND (tp.end_date IS NULL OR tp.end_date >= :date)
  ORDER BY scope_priority DESC
),
period_with_tiers AS (
  SELECT mp.id
  FROM matched_periods mp
  WHERE EXISTS (SELECT 1 FROM tariff_tiers tt WHERE tt.tariff_period_id = mp.id)
  ORDER BY mp.scope_priority DESC
  LIMIT 1
)
SELECT tt.* FROM tariff_tiers tt
WHERE tt.tariff_period_id = (SELECT id FROM period_with_tiers)
ORDER BY tt.from_count ASC
```

Strategy is taken from the highest-priority matched period (same as tariff lookup), not from the tier source. A client override can have its own strategy but inherit tiers.

### Caching

- Redis key: `tariff:{country}:{operator}:{sender_cat}:{traffic_type}:{client}:{date}`
- TTL: 1 hour
- Invalidation: on period create/update/delete, flush keys matching affected dimensions

## REST API

### Periods

```
GET    /admin/v1/tarification/periods?country_id=&operator_id=&sender_category=&traffic_type=&client_id=
POST   /admin/v1/tarification/periods
PUT    /admin/v1/tarification/periods/{id}
DELETE /admin/v1/tarification/periods/{id}
```

**POST body:**
```json
{
  "country_id": "uuid | null",
  "operator_id": "uuid | null",
  "sender_category": "paid_registered | null",
  "traffic_type": "transactional | null",
  "client_id": "uuid | null",
  "strategy": "threshold",
  "start_date": "2026-04-01",
  "end_date": "2026-06-30 | null"
}
```

Backend automatically:
- Computes scope_key and scope_priority
- Auto-closes previous open-ended period in same scope
- Validates containment and child periods

**PUT** — limited editing:
- Can change: `end_date` (with child containment check), `strategy`
- Cannot change: dimensions, `start_date` (create a new period instead)

**DELETE** — only if no child periods. Cascades to tiers.

### Tiers (unchanged)

```
GET    /admin/v1/tarification/tiers?period_id=
POST   /admin/v1/tarification/tiers
PUT    /admin/v1/tarification/tiers/{id}
DELETE /admin/v1/tarification/tiers/{id}
```

### Provider costs (analogous)

```
GET/POST/PUT/DELETE /admin/v1/tarification/provider-costs/periods
GET/POST/PUT/DELETE /admin/v1/tarification/provider-costs/tiers
```

## UI Changes (PeriodsTab)

1. **Filters** — by each dimension: country, operator, sender category, traffic type, client
2. **Table columns** — Scope (human-readable from filled dimensions), Strategy, Start date, End date, Status, Actions (Edit / Delete)
3. **Create modal** — dimension selectors with cascading (select country → operator list filters). Selecting operator auto-fills country. Client field shows "Individual" badge.
4. **Hierarchy visualization** — grouping or indent by scope_priority to show nesting
5. **Auto-close warning** — on create: "Previous period (01.01 — open-ended) will be closed on 31.03"
6. **Validation errors** — shown inline: containment violations, child blocking, overlap

## Error Messages

| Situation | Error |
|-----------|-------|
| Child period outside parent | `"период выходит за границы родительского периода (scope: Казахстан, 01.01–30.06)"` |
| No parent period for dates | `"нет активного периода на родительском уровне для даты 01.04"` |
| Children block auto-close | `"невозможно закрыть период: есть активные дочерние периоды (Beeline KZ, Tele2 KZ)"` |
| Overlap in same scope | `"период пересекается с существующим (01.01–30.06) в этом scope"` |
| Delete with children | `"невозможно удалить: есть дочерние периоды (3 шт.)"` |
| Edit end_date cuts children | `"новая дата окончания раньше, чем у дочерних периодов (Beeline KZ до 30.09)"` |

## Migration

From old `tariff_plans` + `tariff_periods` → new `tariff_periods` with dimensions:

1. For each old `tariff_plan`: take `operator_id`, `sender_category`, `strategy`
2. Look up `country_id` from operators table
3. Migrate each old `tariff_period` → new `tariff_period` with filled `country_id`, `operator_id`, `sender_category`, `strategy`
4. `tariff_tiers` — update foreign key to new period id
5. Down migration: reconstruct `tariff_plans` from unique (operator_id, sender_category, strategy) combinations
