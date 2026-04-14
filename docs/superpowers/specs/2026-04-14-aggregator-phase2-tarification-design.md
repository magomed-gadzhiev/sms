# Aggregator Phase 2: Tarification — Design Spec

**Date:** 2026-04-14
**Status:** Approved
**Version:** 1.0
**Parent spec:** [aggregator-sub-accounts-design.md](2026-04-14-aggregator-sub-accounts-design.md)

## 1. Overview

### 1.1. Goal

Implement tarification for aggregator sub-accounts: aggregator sets tariffs for sub-accounts, platform performs dual deduction (sub-account virtual balance + aggregator real balance) on each SMS, and logs margin for analytics.

### 1.2. Scope

**In scope:**
- Database: `aggregator_tariffs` table, `aggregator_margin_log` table (partitioned)
- Backend: tariff CRUD (repository, service, gRPC, HTTP handlers)
- Backend: dual deduction in tarification service (`ChargeMessageDual` in billing)
- Backend: dual refund on failed delivery (`RefundMessageDual` in billing)
- Backend: cascade balance check (both sub-account and aggregator balances)
- Backend: markup validation on tariff create/update
- Backend: client account_type + parent_client_id caching in Redis
- Frontend: `TariffsPage` (full CRUD + defaults)
- Frontend: `TariffsTab` (read-only with link to TariffsPage)

**Out of scope (later phases):**
- Pool limits: pool_monthly_limit, pool_rate_limit_per_second (Phase 5)
- Cascade freeze (Phase 5)
- Analytics dashboard, tariff simulator, forecasts (Phase 4)
- Route management (Phase 3)

## 2. Data Model

### 2.1. aggregator_tariffs

Aggregator sets tariffs for sub-accounts. Separate from platform's `tariff_plans` system.

```sql
CREATE TABLE aggregator_tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID REFERENCES clients(id),       -- NULL = default tariff for all sub-accounts
    operator_id UUID NOT NULL REFERENCES operators(id),
    sender_category VARCHAR(30) NOT NULL,              -- shared, paid_registered, free_registered
    price_per_segment NUMERIC(20,6) NOT NULL,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (aggregator_id, sub_account_id, operator_id, sender_category)
);

CREATE INDEX idx_aggregator_tariffs_agg ON aggregator_tariffs(aggregator_id);
CREATE INDEX idx_aggregator_tariffs_sub ON aggregator_tariffs(sub_account_id);
```

### 2.2. aggregator_margin_log

Partitioned by month. Records margin per SMS for analytics.

```sql
CREATE TABLE aggregator_margin_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID NOT NULL REFERENCES clients(id),
    message_id UUID NOT NULL,
    operator_id UUID NOT NULL REFERENCES operators(id),
    segment_count INTEGER NOT NULL,
    sub_account_price NUMERIC(20,6) NOT NULL,     -- price per segment charged to sub-account
    aggregator_price NUMERIC(20,6) NOT NULL,      -- purchase price per segment charged to aggregator
    sub_account_total NUMERIC(20,6) NOT NULL,     -- sub_account_price * segment_count
    aggregator_total NUMERIC(20,6) NOT NULL,      -- aggregator_price * segment_count
    margin NUMERIC(20,6) NOT NULL,                -- sub_account_total - aggregator_total
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    idempotency_key VARCHAR(255) NOT NULL UNIQUE
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_agg_margin_log_agg ON aggregator_margin_log(aggregator_id, created_at);
CREATE INDEX idx_agg_margin_log_sub ON aggregator_margin_log(sub_account_id, created_at);
CREATE INDEX idx_agg_margin_log_operator ON aggregator_margin_log(operator_id);

-- Monthly partitions (create for current + next 12 months)
CREATE TABLE aggregator_margin_log_2026_04 PARTITION OF aggregator_margin_log
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
-- ... etc
```

## 3. Tariff Lookup

### 3.1. Cascade Lookup Order

When sub-account sends SMS, tariff is resolved in order:

1. **Personal tariff:** `aggregator_tariffs` WHERE `sub_account_id = :sub_id` AND `operator_id = :op_id` AND `sender_category = :cat` AND `active = true`
2. **Default tariff:** `aggregator_tariffs` WHERE `aggregator_id = :agg_id` AND `sub_account_id IS NULL` AND `operator_id = :op_id` AND `sender_category = :cat` AND `active = true`
3. **Fallback:** aggregator's purchase tariff from platform `tariff_plans` system (margin = 0)

### 3.2. Purchase Tariff

The platform's purchase tariff (from `tariff_plans → tariff_periods → tariff_tiers`) is universal — it does not depend on who sends. It is used for:
- Charging the aggregator's real balance on each sub-account SMS
- Calculating margin in `aggregator_margin_log`
- Validating aggregator tariff bounds (markup limits)

Purchase tariffs cannot be changed retroactively — new `tariff_period` starts from a future date.

## 4. Tarification Flow

### 4.1. Sub-Account SMS Flow

Extension of existing `TarifyMessage()` in `tarification_service.go`:

```
TarifyMessage(clientID, messageID, operatorID, senderName, segmentCount, idempotencyKey)
  │
  ├─ Resolve account_type from Redis cache (key: client:{clientID}:account_info)
  │   Cache miss → query clients table → cache with TTL 5min
  │
  ├─ account_type != 'sub_account'
  │   → existing tarification logic (no changes)
  │
  └─ account_type == 'sub_account'
      │
      1. Determine sender_category (existing logic)
      │
      2. Lookup aggregator tariff (cascade: personal → default → fallback)
      │   → sub_account_price = resolved price_per_segment
      │
      3. Lookup purchase tariff (existing tariff_plans system for aggregator)
      │   → aggregator_price = resolved price_per_segment
      │
      4. Calculate amounts:
      │   sub_amount = sub_account_price × segment_count
      │   agg_amount = aggregator_price × segment_count
      │
      5. saga.ChargeDual() → billing.ChargeMessageDual(
      │     sub_account_id, aggregator_id,
      │     sub_amount, agg_amount, message_id)
      │   → Single DB transaction:
      │     a) SELECT FOR UPDATE on both accounts
      │     b) Check sub-account virtual balance >= sub_amount
      │     c) Check aggregator real balance >= agg_amount
      │     d) Check neither account is frozen
      │     e) Deduct sub_amount from sub-account
      │     f) Deduct agg_amount from aggregator
      │     g) Write 2 transaction records
      │
      6. Write tarification_log (client_id = sub_account_id)
      │
      7. Write aggregator_margin_log:
      │   margin = sub_account_total - aggregator_total
      │   idempotency_key = message_id
      │
      8. Publish tarification event to Kafka
```

### 4.2. Cascade Balance Check

SMS requires sufficient balance at **both levels**:

- Sub-account virtual balance >= sub_account_price × segments
- Aggregator real balance >= aggregator_price × segments

If either is insufficient → reject with "insufficient funds". Sub-account does NOT see which balance is exhausted (privacy: aggregator's real balance is hidden from sub-account).

### 4.3. Refund on Failed Delivery

When pipeline exhausts all retries (`sender/stage.go:424-447`):

```
RefundMessageDual(sub_account_id, aggregator_id, sub_amount, agg_amount, message_id)
  → Single DB transaction:
    a) SELECT FOR UPDATE on both accounts
    b) Credit sub_amount to sub-account virtual balance
    c) Credit agg_amount to aggregator real balance
    d) Write 2 refund transaction records
```

### 4.4. Direct Client & Aggregator's Own SMS

- `account_type = 'direct'` → no changes, existing flow
- `account_type = 'aggregator'` sending own SMS → standard tarification (single deduction from real balance, no margin log)

## 5. Markup Validation

### 5.1. On Tariff Create/Update

When aggregator creates or updates a tariff in `aggregator_tariffs`:

```
Validate:
  1. price_per_segment >= purchase_tariff (current active period)
     → if violated: error "Tariff cannot be below purchase price"

  2. IF aggregator_profiles.max_markup_percent IS NOT NULL:
     price_per_segment <= purchase_tariff × (1 + max_markup_percent / 100)
     → if violated: error "Tariff exceeds maximum markup of {max_markup_percent}%"
```

### 5.2. On Purchase Tariff Change

When platform creates a new `tariff_period` with different prices:

- Check all active `aggregator_tariffs` against new prices
- If any tariff becomes invalid (below new purchase price):
  - **Do NOT auto-deactivate** (avoid breaking aggregator operations)
  - Send notification to aggregator about affected tariffs
  - Log warning in system logs
- Aggregator is responsible for updating their tariffs

## 6. Account Info Caching

### 6.1. Redis Cache

Key: `client:{clientID}:account_info`
Value: JSON `{ "account_type": "sub_account", "parent_client_id": "uuid" }`
TTL: 5 minutes

Cache invalidation: on `clients` table update (account_type or parent_client_id change) — delete key.

### 6.2. Lookup Flow

```go
func (s *TarificationService) getAccountInfo(ctx context.Context, clientID uuid.UUID) (*AccountInfo, error) {
    // 1. Try Redis
    info, err := s.cache.Get(ctx, fmt.Sprintf("client:%s:account_info", clientID))
    if err == nil {
        return info, nil
    }
    // 2. Query DB
    client, err := s.clientRepo.GetByID(ctx, clientID)
    if err != nil {
        return nil, err
    }
    // 3. Cache and return
    info = &AccountInfo{AccountType: client.AccountType, ParentClientID: client.ParentClientID}
    s.cache.Set(ctx, fmt.Sprintf("client:%s:account_info", clientID), info, 5*time.Minute)
    return info, nil
}
```

## 7. Billing Service — New Methods

### 7.1. ChargeMessageDual

```protobuf
rpc ChargeMessageDual(ChargeMessageDualRequest) returns (ChargeMessageDualResponse);

message ChargeMessageDualRequest {
    string sub_account_id = 1;
    string aggregator_id = 2;
    string sub_account_amount = 3;   // decimal as string
    string aggregator_amount = 4;    // decimal as string
    string message_id = 5;
    string currency = 6;
    string description = 7;
}

message ChargeMessageDualResponse {
    bool success = 1;
    string sub_account_transaction_id = 2;
    string aggregator_transaction_id = 3;
    string sub_account_new_balance = 4;
    string aggregator_new_balance = 5;
    string error = 6;
    string rejection_reason = 7;  // insufficient_sub_balance, insufficient_agg_balance, frozen
}
```

### 7.2. RefundMessageDual

```protobuf
rpc RefundMessageDual(RefundMessageDualRequest) returns (RefundMessageDualResponse);

message RefundMessageDualRequest {
    string sub_account_id = 1;
    string aggregator_id = 2;
    string sub_account_amount = 3;
    string aggregator_amount = 4;
    string message_id = 5;
    string currency = 6;
    string description = 7;
}

message RefundMessageDualResponse {
    bool success = 1;
    string sub_account_transaction_id = 2;
    string aggregator_transaction_id = 3;
    string error = 4;
}
```

## 8. gRPC — Aggregator Tariff Methods

Extension of existing `AggregatorService`:

```protobuf
// Sub-account tariffs (aggregator)
rpc SetSubAccountTariff(SetSubAccountTariffRequest) returns (AggregatorTariff);
rpc SetDefaultTariff(SetDefaultTariffRequest) returns (AggregatorTariff);
rpc ListSubAccountTariffs(ListSubAccountTariffsRequest) returns (ListSubAccountTariffsResponse);
rpc DeleteSubAccountTariff(DeleteSubAccountTariffRequest) returns (google.protobuf.Empty);

message SetSubAccountTariffRequest {
    string aggregator_id = 1;
    string sub_account_id = 2;       // specific sub-account
    string operator_id = 3;
    string sender_category = 4;
    string price_per_segment = 5;    // decimal as string
}

message SetDefaultTariffRequest {
    string aggregator_id = 1;
    string operator_id = 2;
    string sender_category = 3;
    string price_per_segment = 4;
}

message AggregatorTariff {
    string id = 1;
    string aggregator_id = 2;
    string sub_account_id = 3;       // empty if default
    string operator_id = 4;
    string sender_category = 5;
    string price_per_segment = 6;
    bool active = 7;
    string created_at = 8;
    string updated_at = 9;
    string operator_name = 10;       // joined for display
    string sub_account_name = 11;    // joined for display
}

message ListSubAccountTariffsRequest {
    string aggregator_id = 1;
    string sub_account_id = 2;       // optional filter
    string operator_id = 3;          // optional filter
    bool defaults_only = 4;          // only show default tariffs
    int32 page = 5;
    int32 page_size = 6;
}

message ListSubAccountTariffsResponse {
    repeated AggregatorTariff tariffs = 1;
    int32 total = 2;
}

message DeleteSubAccountTariffRequest {
    string aggregator_id = 1;
    string tariff_id = 2;
}
```

## 9. HTTP Endpoints

```
# Tariff management (aggregator portal)
GET    /api/v1/aggregator/tariffs                — list all tariffs (personal + defaults)
POST   /api/v1/aggregator/tariffs                — create personal tariff
PUT    /api/v1/aggregator/tariffs/:id            — update tariff
DELETE /api/v1/aggregator/tariffs/:id            — delete tariff
GET    /api/v1/aggregator/tariffs/defaults       — list default tariffs only
POST   /api/v1/aggregator/tariffs/defaults       — create/update default tariff

# Query params for GET /tariffs:
#   sub_account_id — filter by sub-account
#   operator_id    — filter by operator
#   defaults_only  — show only defaults (bool)
#   page, page_size
```

## 10. Frontend

### 10.1. TariffsPage (`portal-frontend/src/pages/aggregator/TariffsPage.tsx`)

Full tariff management page:

- **Header:** "Tariffs" title
- **Tabs:** "Personal tariffs" | "Default tariffs"
- **Personal tariffs tab:**
  - Filters: sub-account dropdown, operator dropdown
  - Table: sub-account name, operator, sender category, price, purchase price (read-only), markup %, actions (edit/delete)
  - "Add tariff" button → modal with sub-account, operator, category, price inputs
  - Markup validation shown inline (min/max bounds)
- **Default tariffs tab:**
  - Table: operator, sender category, price, purchase price, markup %
  - "Add default tariff" button → modal
  - Note: "Default tariffs apply to all sub-accounts without personal tariffs"

### 10.2. TariffsTab (`portal-frontend/src/pages/aggregator/tabs/TariffsTab.tsx`)

Read-only view on sub-account detail page:

- Table: operator, sender category, price, type (personal/default), markup %
- Shows effective tariffs for this sub-account (personal override + defaults)
- Link: "Manage tariffs →" navigates to TariffsPage with sub_account_id filter pre-applied

## 11. Pipeline Integration

### 11.1. Changes to sender/stage.go

Minimal changes to pipeline — tarification service handles all logic internally:

- `TarifyMessage()` gRPC request remains the same (clientID = sub_account_id)
- Tarification service resolves account_type internally and performs dual deduction
- Pipeline only needs change in **refund path**: detect if dual refund is needed

For refund, pipeline needs to know if the original charge was dual:
- Option: tarification response includes `is_dual_charge: true` + `aggregator_id` + `aggregator_amount`
- Pipeline stores this and calls `RefundMessageDual` instead of `AddCredits` on failure

### 11.2. TarifyMessage Response Extension

```protobuf
message TarifyMessageResponse {
    // existing fields...
    bool approved = 1;
    string total_amount = 2;
    // ...

    // new fields for dual charge
    bool is_dual_charge = 10;
    string aggregator_id = 11;
    string aggregator_amount = 12;
}
```

## 12. File Map

```
# Migrations
migrations/
  000096_create_aggregator_tariffs.up.sql
  000096_create_aggregator_tariffs.down.sql
  000097_create_aggregator_margin_log.up.sql
  000097_create_aggregator_margin_log.down.sql

# Domain
internal/services/aggregator/domain/
  tariff.go                    -- AggregatorTariff model
  margin.go                    -- MarginEntry model

# Repository
internal/services/aggregator/repository/
  tariff_repository.go         -- aggregator_tariffs CRUD
  margin_repository.go         -- aggregator_margin_log writes

# Service
internal/services/aggregator/service/
  tariff_service.go            -- tariff CRUD + validation logic

# Tarification service extension
internal/services/tarification/application/
  tarification_service.go      -- MODIFY: add sub-account branch in TarifyMessage()
  saga.go                      -- MODIFY: add ChargeDual() method

# Billing service extension
internal/services/billing/application/
  billing_service.go           -- MODIFY: add ChargeMessageDual(), RefundMessageDual()
internal/services/billing/grpc/
  server.go                    -- MODIFY: add gRPC handlers for new methods

# Proto
api/proto/billing/billing.proto       -- MODIFY: add ChargeMessageDual, RefundMessageDual
api/proto/tarification/tarification.proto  -- MODIFY: extend TarifyMessageResponse
api/proto/aggregator/aggregator.proto  -- MODIFY: add tariff RPCs

# Pipeline
internal/pipeline/sender/stage.go     -- MODIFY: dual refund path

# HTTP handlers
internal/gateway/portal/handlers/
  aggregator_tariffs.go        -- NEW: tariff HTTP handlers

# Frontend
portal-frontend/src/pages/aggregator/
  TariffsPage.tsx              -- NEW: full tariff management
  tabs/TariffsTab.tsx          -- MODIFY: read-only view with link
```
