# Data Model: Operator-Based SMS Tarification System

**Date**: 2026-03-20

## Routing-Service Entities

### Country

| Field      | Type         | Constraints                    |
| ---------- | ------------ | ------------------------------ |
| id         | UUID         | PK                             |
| name       | VARCHAR(100) | NOT NULL                       |
| iso_code   | VARCHAR(2)   | NOT NULL, UNIQUE (ISO 3166-1)  |
| phone_code | VARCHAR(5)   | NOT NULL (e.g. "+7")           |
| currency   | VARCHAR(3)   | NOT NULL (ISO 4217, e.g. "RUB")|
| created_at | TIMESTAMPTZ  | NOT NULL, DEFAULT now()        |
| updated_at | TIMESTAMPTZ  | NOT NULL, DEFAULT now()        |

**Relationships**: One-to-many → Operator

### Operator

| Field                 | Type         | Constraints                         |
| --------------------- | ------------ | ----------------------------------- |
| id                    | UUID         | PK                                  |
| country_id            | UUID         | NOT NULL, FK → Country              |
| name                  | VARCHAR(100) | NOT NULL                            |
| code                  | VARCHAR(50)  | NOT NULL, UNIQUE (e.g. "mts-ru")    |
| supports_paid_sender  | BOOLEAN      | NOT NULL, DEFAULT false             |
| supports_free_sender  | BOOLEAN      | NOT NULL, DEFAULT false             |
| active                | BOOLEAN      | NOT NULL, DEFAULT true              |
| created_at            | TIMESTAMPTZ  | NOT NULL, DEFAULT now()             |
| updated_at            | TIMESTAMPTZ  | NOT NULL, DEFAULT now()             |

**Relationships**: Many-to-one → Country, One-to-many → OperatorPrefix

### OperatorPrefix

| Field       | Type        | Constraints                       |
| ----------- | ----------- | --------------------------------- |
| id          | UUID        | PK                                |
| operator_id | UUID        | NOT NULL, FK → Operator (CASCADE) |
| prefix      | VARCHAR(15) | NOT NULL, UNIQUE (e.g. "+7900")   |
| priority    | INTEGER     | NOT NULL, DEFAULT 0               |
| created_at  | TIMESTAMPTZ | NOT NULL, DEFAULT now()           |

**Relationships**: Many-to-one → Operator
**Lookup Logic**: Longest matching prefix wins; at equal length, highest priority wins

## Tarification-Service Entities

### SenderRegistration

| Field       | Type         | Constraints                                                |
| ----------- | ------------ | ---------------------------------------------------------- |
| id          | UUID         | PK                                                         |
| client_id   | UUID         | NOT NULL                                                   |
| operator_id | UUID         | NOT NULL                                                   |
| sender_name | VARCHAR(11)  | NOT NULL                                                   |
| type        | VARCHAR(20)  | NOT NULL, CHECK IN ('paid', 'free')                        |
| status      | VARCHAR(20)  | NOT NULL, CHECK IN ('active', 'pending', 'expired')        |
| created_at  | TIMESTAMPTZ  | NOT NULL, DEFAULT now()                                    |
| updated_at  | TIMESTAMPTZ  | NOT NULL, DEFAULT now()                                    |

**Unique constraint**: (client_id, operator_id, sender_name)
**Validation**: operator must support the registration type (supports_paid_sender / supports_free_sender)

**State transitions**:
- pending → active (admin approval)
- active → expired (admin action or TTL)
- pending → expired (rejected)

### TariffPlan

| Field           | Type        | Constraints                                                                      |
| --------------- | ----------- | -------------------------------------------------------------------------------- |
| id              | UUID        | PK                                                                               |
| operator_id     | UUID        | NOT NULL                                                                         |
| sender_category | VARCHAR(30) | NOT NULL, CHECK IN ('shared', 'paid_registered', 'free_registered')              |
| strategy        | VARCHAR(30) | NOT NULL, CHECK IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold') |
| active          | BOOLEAN     | NOT NULL, DEFAULT true                                                           |
| created_at      | TIMESTAMPTZ | NOT NULL, DEFAULT now()                                                          |
| updated_at      | TIMESTAMPTZ | NOT NULL, DEFAULT now()                                                          |

**Unique constraint**: (operator_id, sender_category) WHERE active = true
**Deactivation rule**: Cannot deactivate while any TariffPeriod is active (end_date >= today)

### TariffPeriod

| Field          | Type        | Constraints                             |
| -------------- | ----------- | --------------------------------------- |
| id             | UUID        | PK                                      |
| tariff_plan_id | UUID        | NOT NULL, FK → TariffPlan (CASCADE)     |
| start_date     | DATE        | NOT NULL                                |
| end_date       | DATE        | NOT NULL, CHECK (end_date > start_date) |
| created_at     | TIMESTAMPTZ | NOT NULL, DEFAULT now()                 |

**Exclusion constraint**: No overlapping periods within same tariff_plan_id (daterange)
**Immutability**: Tiers cannot be modified while period is active (start_date <= today <= end_date)

### TariffTier

| Field             | Type          | Constraints                                |
| ----------------- | ------------- | ------------------------------------------ |
| id                | UUID          | PK                                         |
| tariff_period_id  | UUID          | NOT NULL, FK → TariffPeriod (CASCADE)      |
| from_count        | INTEGER       | NOT NULL, DEFAULT 0, CHECK (>= 0)         |
| price_per_segment | NUMERIC(20,6) | NOT NULL, CHECK (>= 0)                    |

**Unique constraint**: (tariff_period_id, from_count)
**Ordering**: Sorted by from_count ASC. First tier must start at 0.

### PricingPeriod

| Field            | Type        | Constraints                                   |
| ---------------- | ----------- | --------------------------------------------- |
| id               | UUID        | PK                                            |
| tariff_period_id | UUID        | NOT NULL, FK → TariffPeriod (CASCADE)         |
| start_date       | DATE        | NOT NULL                                      |
| end_date         | DATE        | NOT NULL, CHECK (end_date > start_date)       |
| created_at       | TIMESTAMPTZ | NOT NULL, DEFAULT now()                       |

**Exclusion constraint**: No overlapping within same tariff_period_id
**Containment constraint**: Must be fully within parent TariffPeriod's [start_date, end_date]

### PrepaidFee

| Field            | Type          | Constraints                                 |
| ---------------- | ------------- | ------------------------------------------- |
| id               | UUID          | PK                                          |
| tariff_plan_id   | UUID          | NOT NULL, FK → TariffPlan                   |
| tariff_period_id | UUID          | NOT NULL, FK → TariffPeriod                 |
| amount           | NUMERIC(20,6) | NOT NULL, CHECK (> 0)                       |
| currency         | VARCHAR(3)    | NOT NULL                                    |
| charged          | BOOLEAN       | NOT NULL, DEFAULT false                     |
| charged_at       | TIMESTAMPTZ   | NULL                                        |
| created_at       | TIMESTAMPTZ   | NOT NULL, DEFAULT now()                     |

**Only for strategy**: `prepaid_threshold`

### UsageCounter

| Field            | Type        | Constraints                               |
| ---------------- | ----------- | ----------------------------------------- |
| id               | UUID        | PK                                        |
| client_id        | UUID        | NOT NULL                                  |
| tariff_plan_id   | UUID        | NOT NULL, FK → TariffPlan                 |
| tariff_period_id | UUID        | NOT NULL, FK → TariffPeriod               |
| segment_count    | INTEGER     | NOT NULL, DEFAULT 0, CHECK (>= 0)        |
| updated_at       | TIMESTAMPTZ | NOT NULL, DEFAULT now()                   |

**Unique constraint**: (client_id, tariff_plan_id, tariff_period_id)
**Concurrency**: SELECT FOR UPDATE for reads + atomic increment for writes
**Retention**: Stored indefinitely (compact data)

### TarificationLog

| Field             | Type          | Constraints                            |
| ----------------- | ------------- | -------------------------------------- |
| id                | UUID          | PK                                     |
| client_id         | UUID          | NOT NULL                               |
| message_id        | UUID          | NOT NULL                               |
| operator_id       | UUID          | NOT NULL                               |
| sender_category   | VARCHAR(30)   | NOT NULL                               |
| strategy          | VARCHAR(30)   | NOT NULL                               |
| tariff_plan_id    | UUID          | NOT NULL                               |
| tariff_period_id  | UUID          | NOT NULL                               |
| segment_count     | INTEGER       | NOT NULL                               |
| price_per_segment | NUMERIC(20,6) | NOT NULL                               |
| total_amount      | NUMERIC(20,6) | NOT NULL                               |
| recalc_amount     | NUMERIC(20,6) | NULL (only if threshold crossed)       |
| idempotency_key   | VARCHAR(64)   | NOT NULL, UNIQUE                       |
| created_at        | TIMESTAMPTZ   | NOT NULL, DEFAULT now()                |

**Partitioning**: Monthly by created_at (same pattern as messages table)
**Retention**: 12 months active, then archived/purged
**Indexes**: (client_id, created_at), (message_id), (idempotency_key) UNIQUE
