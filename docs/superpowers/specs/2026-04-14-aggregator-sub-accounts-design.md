# Aggregator Sub-Accounts System Design

**Date:** 2026-04-14
**Status:** Approved
**Version:** 1.0

## 1. Overview

### 1.1. Goal

Implement an aggregator model in the SMS platform — partners who bring their own clients (sub-accounts), earn on tariff markup, and self-manage their client pool through the platform portal.

### 1.2. Business Objectives

- Attract aggregators as a sales channel — partners bring clients and earn on them
- Scale client base without proportional growth in platform operational costs
- Transparent earning model for aggregators with analytics and forecasting tools
- Platform retains full control over tariffs, providers, and service quality

### 1.3. Key Decisions

| Parameter | Decision |
|---|---|
| Hierarchy depth | 2 levels: platform → aggregator → sub-account |
| Earning model | Hybrid: markup + max markup cap from platform |
| Financial model | Aggregator as intermediary (balance pool) |
| White-label | None (phase 1) |
| Routing | Full delegation from available provider pool |
| Tarification | Single deduction from aggregator, virtual balance for sub-account |
| Self-service | Full (sub-accounts, balance, tariffs, routes, sender names, API, SMPP, webhooks, analytics, message logs) |
| Platform control | Individual tariffs, max markup, sub-account limit, traffic limit, rate-limiting, cascade freeze, audit log, provider restrictions |
| Analytics | Full business dashboard with forecast and margin simulator |
| Balance top-up | Manual via platform administration |

## 2. Data Model

### 2.1. Account Type Migration

Replace `is_reseller` flag with explicit `account_type` in `clients` table:

```sql
ALTER TABLE clients ADD COLUMN account_type VARCHAR(20) NOT NULL DEFAULT 'direct';
-- Values: 'direct', 'aggregator', 'sub_account'

-- Data migration
UPDATE clients SET account_type = 'aggregator' WHERE is_reseller = true;
UPDATE clients SET account_type = 'sub_account' WHERE parent_client_id IS NOT NULL;

-- Remove old flag
ALTER TABLE clients DROP COLUMN is_reseller;
-- parent_client_id remains (sub → aggregator link)
```

Constraints:
- `account_type = 'aggregator'` → `parent_client_id IS NULL`
- `account_type = 'sub_account'` → `parent_client_id IS NOT NULL`
- `account_type = 'direct'` → `parent_client_id IS NULL`
- Sub-account cannot have sub-accounts (application-level CHECK)

### 2.2. Aggregator Profile

New table `aggregator_profiles` (1:1 with `clients` where `account_type = 'aggregator'`):

```sql
CREATE TABLE aggregator_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL UNIQUE REFERENCES clients(id),
    max_markup_percent NUMERIC(5,2),              -- max markup, e.g. 200.00 = +200%
    max_sub_accounts INTEGER DEFAULT 100,
    pool_monthly_limit INTEGER,                    -- total SMS/month for entire pool
    pool_rate_limit_per_second INTEGER,            -- total msg/sec for pool
    allowed_provider_ids UUID[],                   -- which providers are available
    auto_freeze_sub_accounts BOOLEAN DEFAULT true, -- cascade freeze
    notes TEXT,                                    -- admin notes
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_aggregator_profiles_client_id ON aggregator_profiles(client_id);
```

### 2.3. Aggregator Tariffs

Aggregator sets tariffs for sub-accounts via separate table:

```sql
CREATE TABLE aggregator_tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID REFERENCES clients(id),       -- NULL = default tariff for all new sub-accounts
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

Tariff lookup order for sub-account SMS:
1. `aggregator_tariffs` with specific `sub_account_id` + `operator_id` + `sender_category`
2. `aggregator_tariffs` with `sub_account_id = NULL` (aggregator's default tariff)
3. Aggregator's own purchase tariff (margin = 0)

Validation on create/update:
- `price_per_segment` >= aggregator's purchase tariff for same operator
- `price_per_segment` <= purchase tariff × (1 + `max_markup_percent` / 100) if max_markup_percent is set

### 2.4. Virtual Balance for Sub-Accounts

Extend existing `accounts` table:

```sql
ALTER TABLE accounts ADD COLUMN is_virtual BOOLEAN DEFAULT false;
ALTER TABLE accounts ADD COLUMN allocated_by UUID REFERENCES clients(id);
```

- `account_type = 'sub_account'` → `is_virtual = true`, `allocated_by = parent_client_id`
- `account_type = 'aggregator'` or `'direct'` → `is_virtual = false`

### 2.5. Aggregator Margin Log

```sql
CREATE TABLE aggregator_margin_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID NOT NULL REFERENCES clients(id),
    message_id UUID NOT NULL,
    operator_id UUID NOT NULL REFERENCES operators(id),
    segment_count INTEGER NOT NULL,
    sub_account_price NUMERIC(20,6) NOT NULL,     -- price per segment for sub-account
    aggregator_price NUMERIC(20,6) NOT NULL,      -- purchase price per segment for aggregator
    sub_account_total NUMERIC(20,6) NOT NULL,     -- sub_account_price × segment_count
    aggregator_total NUMERIC(20,6) NOT NULL,      -- aggregator_price × segment_count
    margin NUMERIC(20,6) NOT NULL,                -- sub_account_total - aggregator_total
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    idempotency_key VARCHAR(255) NOT NULL UNIQUE
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_agg_margin_log_agg ON aggregator_margin_log(aggregator_id, created_at);
CREATE INDEX idx_agg_margin_log_sub ON aggregator_margin_log(sub_account_id, created_at);
CREATE INDEX idx_agg_margin_log_operator ON aggregator_margin_log(operator_id);
```

### 2.6. Aggregator Audit Log

```sql
CREATE TABLE aggregator_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    action VARCHAR(50) NOT NULL,        -- create_sub_account, update_tariff, transfer_balance, ...
    target_type VARCHAR(30) NOT NULL,   -- sub_account, tariff, route, balance, ...
    target_id UUID,
    details JSONB,                      -- full action context
    ip_address INET,
    created_at TIMESTAMP NOT NULL DEFAULT now()
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_agg_audit_log_agg ON aggregator_audit_log(aggregator_id, created_at);
CREATE INDEX idx_agg_audit_log_action ON aggregator_audit_log(action);
```

### 2.7. Default Routes

```sql
CREATE TABLE aggregator_default_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id UUID NOT NULL REFERENCES clients(id),
    operator_id UUID REFERENCES operators(id),     -- NULL = for all operators
    provider_id UUID NOT NULL REFERENCES providers(id),
    strategy VARCHAR(20) DEFAULT 'priority',        -- priority, weighted
    priority INTEGER DEFAULT 0,
    weight INTEGER DEFAULT 100,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (aggregator_id, operator_id, provider_id)
);

CREATE INDEX idx_agg_default_routes_agg ON aggregator_default_routes(aggregator_id);
```

## 3. Tarification Flow

### 3.1. Sub-Account SMS Tarification

When a sub-account sends SMS:

```
Sub-account sends SMS
        │
        ▼
1. Resolve operator by recipient phone number
        │
        ▼
2. Lookup sub-account tariff (aggregator_tariffs)
   → specific sub_account_id + operator_id + sender_category
   → fallback: sub_account_id = NULL (aggregator's default tariff)
   → fallback: aggregator's purchase tariff (margin = 0)
        │
        ▼
3. Lookup aggregator's purchase tariff (existing tarification system)
   → tariff_plans + tariff_periods + tariff_tiers for aggregator_id
        │
        ▼
4. Check limits:
   → sub-account virtual balance >= sub_account_price × segments?
   → aggregator real balance >= aggregator_price × segments?
   → pool has not exceeded pool_monthly_limit?
   → pool has not exceeded pool_rate_limit_per_second?
        │
        ▼
5. Atomic transaction:
   a) Deduct sub_account_price × segments from sub-account virtual balance
   b) Deduct aggregator_price × segments from aggregator real balance
   c) Write to tarification_log (for sub-account)
   d) Write to aggregator_margin_log (margin)
   e) Update usage_counters (for sub-account AND aggregator pool)
        │
        ▼
6. Send SMS via sub-account route
```

### 3.2. Pool Limit Checks

Pool limits are checked **cumulatively** — aggregator + all sub-accounts:

- **pool_monthly_limit**: sum of all SMS for current month across all sub-accounts + aggregator's own traffic. Cached counter in Redis with increment on each send.
- **pool_rate_limit_per_second**: shared rate-limiter in Redis (sliding window). Every SMS from any sub-account increments the pool counter.

### 3.3. Cascade Balance Check

SMS from sub-account requires sufficient balance at **both levels**:

```
Sub-account virtual balance:  500 RUB  (sub-account tariff: 3.50 RUB)
Aggregator real balance:      100 RUB  (purchase tariff: 2.00 RUB)

Sub-account can send:
  → by own balance: 500 / 3.50 = 142 SMS
  → by aggregator balance: 100 / 2.00 = 50 SMS
  → actual: min(142, 50) = 50 SMS

After 50 SMS: sub-account gets "insufficient funds" error,
even though virtual balance still has 325 RUB
```

Sub-account sees "insufficient funds" without revealing the reason (aggregator balance exhausted). Aggregator sees in dashboard that their real balance is running low.

### 3.4. Aggregator's Own SMS

Aggregator can also send SMS directly (not through sub-accounts):
- Standard tarification at aggregator's purchase tariff
- Deduction from real balance
- No entry in `aggregator_margin_log`
- Traffic counts toward `pool_monthly_limit`

### 3.5. Direct Client Tarification

For `account_type = 'direct'` — nothing changes, existing tarification logic works as-is.

## 4. Routing

### 4.1. Available Providers

Platform controls which providers are available to aggregator via `aggregator_profiles.allowed_provider_ids` — this is the source of truth. When admin updates `allowed_provider_ids`, corresponding `client_providers` records are created/removed automatically for the aggregator.

Existing `client_providers` table is used as-is:
- For aggregator: `client_providers` records are auto-synced from `allowed_provider_ids`, `ownership = 'platform'`
- For sub-account: `client_providers` with `ownership = 'inherited'`, `source_client_id = aggregator_id` — created by aggregator when assigning routes

### 4.2. Route Management

```
Platform
  │
  ├── Assigns providers to aggregator (allowed_provider_ids)
  │   └── client_providers: ownership = 'platform'
  │
  ▼
Aggregator
  │
  ├── Creates routes from available providers
  │   └── client_routing_strategies + client_routes (for sub_account_id)
  │
  ├── Can create default routes (for all new sub-accounts)
  │   └── aggregator_default_routes
  │
  ▼
Sub-account
      └── Receives routes assigned by aggregator
          └── client_routes + client_routing_strategies
```

### 4.3. Default Routes

When creating a sub-account, default routes from `aggregator_default_routes` are copied to `client_routes` and `client_routing_strategies`.

### 4.4. Validation

On route create/update for sub-account:
1. `provider_id` must be in `aggregator_profiles.allowed_provider_ids`
2. Provider must have `active = true`
3. Aggregator cannot assign a provider they don't have in `client_providers`

On removing a provider from aggregator's pool (by platform):
- All sub-account routes using this provider are deactivated (`active = false`)
- Aggregator receives notification

## 5. Sub-Account Management

### 5.1. Sub-Account Lifecycle

```
Aggregator creates sub-account
        │
        ▼
  [active = true]  ←── auto-activation (no platform moderation)
        │
        ├── Aggregator allocates balance
        ├── Aggregator sets tariffs
        ├── Aggregator assigns routes (or defaults are copied)
        ├── Aggregator generates API key / configures SMPP
        │
        ▼
  Sub-account operates
        │
        ├── Aggregator deactivates  ──→  [active = false]
        │                                      │
        │                               Can be reactivated
        │
        ├── Aggregator frozen by platform  ──→  cascade freeze
        │                                       of all sub-accounts
        │
        └── Deletion  ──→  soft delete (active = false, deleted_at)
                           Balance returned to aggregator
```

### 5.2. Sub-Account Creation

When aggregator creates a sub-account:

1. Check `count(sub_accounts) < aggregator_profiles.max_sub_accounts`
2. Create record in `clients`:
   - `account_type = 'sub_account'`
   - `parent_client_id = aggregator_id`
   - `plan_id` — inherited from aggregator's plan
   - Generate `api_key` and `secret`
3. Create record in `accounts`:
   - `is_virtual = true`
   - `allocated_by = aggregator_id`
   - `balance = 0`
4. Copy `aggregator_default_routes` → `client_routes`
5. Apply default tariffs (`aggregator_tariffs` with `sub_account_id = NULL`)
6. Write to `aggregator_audit_log`

### 5.3. Balance Distribution

Transfer aggregator → sub-account:

```
Request: transfer(aggregator_id, sub_account_id, amount)
        │
        ▼
  Checks:
  ├── sub_account.parent_client_id == aggregator_id?
  ├── aggregator.balance >= amount?
  ├── aggregator not frozen?
        │
        ▼
  Atomic transaction:
  ├── aggregator.balance -= amount
  ├── sub_account.balance += amount
  ├── Write to transactions (type 'transfer_out' for aggregator)
  ├── Write to transactions (type 'transfer_in' for sub-account)
  ├── Write to balance_transfers
  └── Write to aggregator_audit_log
```

Reverse transfer (sub-account → aggregator) is also supported — aggregator can withdraw balance.

### 5.4. Cascade Freeze

When platform freezes aggregator (`accounts.frozen = true`):

1. Freeze aggregator account
2. If `aggregator_profiles.auto_freeze_sub_accounts = true`:
   - Freeze all `accounts` where `allocated_by = aggregator_id`
   - All active SMPP sessions for sub-accounts are unbound
   - Sub-account API requests return 403
3. Write to `aggregator_audit_log` (action = 'cascade_freeze')

On unfreeze:
- Sub-accounts are **not** automatically unfrozen
- Aggregator can unfreeze individually or use **"unfreeze all"** button to unfreeze all sub-accounts at once
- This protects against cases where freeze was caused by a problematic sub-account

### 5.5. Sender Names

Aggregator manages sender names for sub-accounts via existing `sender_registrations`:

- Aggregator creates sender name registration for sub-account
- `sender_registrations.client_id = sub_account_id`
- Billing for sender name (`sender_name_billing_records`) is charged to **aggregator's real balance**, not sub-account
- Aggregator can pass through cost via tariffs or separate agreement

### 5.6. SMPP Connections

Aggregator configures SMPP bindings for sub-accounts:
- `system_id` and `password` are generated per sub-account
- Sub-account rate-limit is bounded by individual limit **and** `pool_rate_limit_per_second`
- On cascade freeze — sub-account SMPP sessions are unbound

## 6. Analytics and Business Dashboard

### 6.1. Dashboard Layout

```
┌─────────────────────────────────────────────────────────┐
│  AGGREGATOR DASHBOARD                                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  [Balance: 245,000 RUB] [Margin today: 12,400 RUB]     │
│  [Sub-accounts: 18/100] [SMS today: 34,200]             │
│                                                         │
│  ┌─── Margin over period ─────────────────────────┐    │
│  │  Line chart: margin by days                     │    │
│  │  Toggle: day / week / month / custom range      │    │
│  │  Comparison with previous period                │    │
│  └────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─── Top sub-accounts ──┐  ┌─── Top directions ────┐  │
│  │ 1. Client A +4,200    │  │ 1. MTS     +5,100     │  │
│  │ 2. Client B +3,100    │  │ 2. Beeline +3,800     │  │
│  │ 3. Client C +2,800    │  │ 3. Megafon +2,200     │  │
│  └────────────────────────┘  └───────────────────────┘  │
│                                                         │
│  ┌─── Forecast ───────────────────────────────────┐    │
│  │ At current pace this month: ~380,000 SMS       │    │
│  │ Expected margin: ~152,000 RUB                  │    │
│  │ Balance sufficient for: ~12 days               │    │
│  └────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### 6.2. Base Metrics (Real-time)

Source: `aggregator_margin_log` + `accounts` + Redis cache.

| Metric | Source |
|---|---|
| Current balance | `accounts.balance` |
| Margin for today / week / month | `SUM(margin)` from `aggregator_margin_log` |
| SMS for period (pool) | Redis counter + `aggregator_margin_log` |
| Average margin per SMS | `SUM(margin) / COUNT(*)` |
| Active sub-accounts | `COUNT(clients WHERE parent = agg AND active)` |

### 6.3. Detailed Breakdown

**By sub-accounts:**
- Table: sub-account, SMS for period, sub-account spend, aggregator spend, margin, avg margin/SMS
- Sortable by any column
- Filter by period

**By operators / countries:**
- Table: operator (country), SMS, margin, share of total margin
- Visualization: pie chart by operators

**By days:**
- Line chart: margin and SMS count by days
- Ability to compare two periods on same chart

### 6.4. Tariff Simulator

"What if" tool — aggregator models margin change with tariff adjustment:

```
Input:
  ├── Sub-account: [Client A] (or "all")
  ├── Operator: [MTS] (or "all")
  ├── Current tariff: 3.50 RUB  (auto-filled)
  ├── New tariff: [4.00 RUB]    (aggregator input)
  └── Calculation period: [last 30 days]

Calculation (based on real data for period):
  ├── SMS for period: 8,200
  ├── Margin at current tariff: 8,200 × (3.50 - 2.00) = 12,300 RUB
  ├── Margin at new tariff: 8,200 × (4.00 - 2.00) = 16,400 RUB
  └── Difference: +4,100 RUB/month (+33%)

Limits:
  ├── New tariff >= purchase price (2.00 RUB) — otherwise warning
  └── New tariff <= max markup — otherwise warning
```

Simulator **does not apply** tariffs — only shows forecast. "Apply tariff" button navigates to tariff editing page.

### 6.5. Forecasting

Based on data from last 7/14/30 days:

- **Margin forecast**: linear extrapolation of trend for current month
- **Balance forecast**: "at current spend rate, balance sufficient for N days"
- **Warnings**: if balance will run out before end of month — orange alert

Algorithm: simple linear regression on daily data. No complex ML — sufficient for phase 1.

### 6.6. Report Export

Aggregator can export:
- Margin details for period — CSV / XLSX
- Sub-account SMS list — CSV (via existing export mechanism)
- Summary report — XLSX with multiple sheets (summary, by sub-accounts, by operators)

Export via existing async export mechanism (Redis job queue).

## 7. API and Portal

### 7.1. New gRPC Methods (aggregator service)

```protobuf
service AggregatorService {
  // Aggregator profile (platform admin)
  rpc CreateAggregatorProfile(CreateAggregatorProfileRequest) returns (AggregatorProfile);
  rpc UpdateAggregatorProfile(UpdateAggregatorProfileRequest) returns (AggregatorProfile);
  rpc GetAggregatorProfile(GetAggregatorProfileRequest) returns (AggregatorProfile);
  rpc ListAggregatorProfiles(ListAggregatorProfilesRequest) returns (ListAggregatorProfilesResponse);

  // Sub-account management (aggregator)
  rpc CreateSubAccount(CreateSubAccountRequest) returns (SubAccountResponse);
  rpc UpdateSubAccount(UpdateSubAccountRequest) returns (SubAccountResponse);
  rpc DeactivateSubAccount(DeactivateSubAccountRequest) returns (SubAccountResponse);
  rpc ListSubAccounts(ListSubAccountsRequest) returns (ListSubAccountsResponse);
  rpc GetSubAccount(GetSubAccountRequest) returns (SubAccountResponse);

  // Balance (aggregator)
  rpc TransferBalance(TransferBalanceRequest) returns (TransferBalanceResponse);
  rpc WithdrawBalance(WithdrawBalanceRequest) returns (TransferBalanceResponse);
  rpc UnfreezeAllSubAccounts(UnfreezeAllSubAccountsRequest) returns (UnfreezeAllSubAccountsResponse);

  // Sub-account tariffs (aggregator)
  rpc SetSubAccountTariff(SetSubAccountTariffRequest) returns (AggregatorTariff);
  rpc SetDefaultTariff(SetDefaultTariffRequest) returns (AggregatorTariff);
  rpc ListSubAccountTariffs(ListSubAccountTariffsRequest) returns (ListSubAccountTariffsResponse);
  rpc DeleteSubAccountTariff(DeleteSubAccountTariffRequest) returns (google.protobuf.Empty);

  // Sub-account routes (aggregator)
  rpc CreateSubAccountRoute(CreateSubAccountRouteRequest) returns (SubAccountRoute);
  rpc UpdateSubAccountRoute(UpdateSubAccountRouteRequest) returns (SubAccountRoute);
  rpc DeleteSubAccountRoute(DeleteSubAccountRouteRequest) returns (google.protobuf.Empty);
  rpc ListSubAccountRoutes(ListSubAccountRoutesRequest) returns (ListSubAccountRoutesResponse);
  rpc SetDefaultRoutes(SetDefaultRoutesRequest) returns (SetDefaultRoutesResponse);
  rpc ListDefaultRoutes(ListDefaultRoutesRequest) returns (ListDefaultRoutesResponse);

  // Analytics (aggregator)
  rpc GetDashboardSummary(DashboardSummaryRequest) returns (DashboardSummaryResponse);
  rpc GetMarginBySubAccounts(MarginBySubAccountsRequest) returns (MarginBySubAccountsResponse);
  rpc GetMarginByOperators(MarginByOperatorsRequest) returns (MarginByOperatorsResponse);
  rpc GetMarginTimeSeries(MarginTimeSeriesRequest) returns (MarginTimeSeriesResponse);
  rpc SimulateTariffChange(SimulateTariffRequest) returns (SimulateTariffResponse);
  rpc GetBalanceForecast(BalanceForecastRequest) returns (BalanceForecastResponse);
  rpc ExportMarginReport(ExportMarginReportRequest) returns (ExportJobResponse);

  // Audit (platform admin)
  rpc ListAggregatorAuditLog(ListAuditLogRequest) returns (ListAuditLogResponse);
}
```

### 7.2. HTTP Endpoints (portal, gorilla/mux)

**Aggregator section** — accessible when `account_type = 'aggregator'`:

```
# Sub-accounts
GET    /api/v1/aggregator/sub-accounts
POST   /api/v1/aggregator/sub-accounts
GET    /api/v1/aggregator/sub-accounts/:id
PUT    /api/v1/aggregator/sub-accounts/:id
DELETE /api/v1/aggregator/sub-accounts/:id
POST   /api/v1/aggregator/sub-accounts/:id/deactivate
POST   /api/v1/aggregator/sub-accounts/unfreeze-all

# Balance
POST   /api/v1/aggregator/sub-accounts/:id/transfer
POST   /api/v1/aggregator/sub-accounts/:id/withdraw

# Tariffs
GET    /api/v1/aggregator/tariffs
POST   /api/v1/aggregator/tariffs
PUT    /api/v1/aggregator/tariffs/:id
DELETE /api/v1/aggregator/tariffs/:id
GET    /api/v1/aggregator/tariffs/defaults
POST   /api/v1/aggregator/tariffs/defaults

# Routes
GET    /api/v1/aggregator/routes
POST   /api/v1/aggregator/routes
PUT    /api/v1/aggregator/routes/:id
DELETE /api/v1/aggregator/routes/:id
GET    /api/v1/aggregator/routes/defaults
POST   /api/v1/aggregator/routes/defaults

# Sender names
GET    /api/v1/aggregator/sub-accounts/:id/senders
POST   /api/v1/aggregator/sub-accounts/:id/senders
DELETE /api/v1/aggregator/sub-accounts/:id/senders/:sender_id

# API keys
POST   /api/v1/aggregator/sub-accounts/:id/api-keys/rotate

# SMPP
GET    /api/v1/aggregator/sub-accounts/:id/smpp
PUT    /api/v1/aggregator/sub-accounts/:id/smpp

# Webhooks
GET    /api/v1/aggregator/sub-accounts/:id/webhooks
PUT    /api/v1/aggregator/sub-accounts/:id/webhooks

# Analytics
GET    /api/v1/aggregator/dashboard
GET    /api/v1/aggregator/analytics/margin/by-sub-accounts
GET    /api/v1/aggregator/analytics/margin/by-operators
GET    /api/v1/aggregator/analytics/margin/time-series
POST   /api/v1/aggregator/analytics/simulate-tariff
GET    /api/v1/aggregator/analytics/forecast
POST   /api/v1/aggregator/analytics/export

# Message logs
GET    /api/v1/aggregator/sub-accounts/:id/messages
```

**Admin section** — new endpoints for aggregator management:

```
# Aggregator profiles
GET    /api/v1/admin/aggregators
POST   /api/v1/admin/aggregators
GET    /api/v1/admin/aggregators/:id
PUT    /api/v1/admin/aggregators/:id
GET    /api/v1/admin/aggregators/:id/audit-log
GET    /api/v1/admin/aggregators/:id/sub-accounts
```

### 7.3. Authorization

Middleware checks `account_type` from JWT token:

| Endpoint | direct | aggregator | sub_account | admin |
|---|---|---|---|---|
| `/api/v1/aggregator/*` | 403 | OK | 403 | OK |
| `/api/v1/admin/aggregators/*` | 403 | 403 | 403 | OK |
| `/api/v1/messages` (own) | OK | OK | OK | OK |
| `/api/v1/billing` (own balance) | OK | OK | OK (virtual) | OK |

Additional: aggregator sees **only their own** sub-accounts. Check `sub_account.parent_client_id == aggregator_id` on every request.

### 7.4. Portal — New Pages (React)

```
portal-frontend/src/pages/
├── aggregator/                           -- new section
│   ├── AggregatorDashboardPage.tsx        -- summary, charts, forecast
│   ├── SubAccountsListPage.tsx            -- sub-accounts list
│   ├── SubAccountDetailPage.tsx           -- manage specific sub-account
│   │   ├── tabs/BalanceTab.tsx            -- transfer / withdraw balance
│   │   ├── tabs/TariffsTab.tsx            -- sub-account tariffs
│   │   ├── tabs/RoutesTab.tsx             -- sub-account routes
│   │   ├── tabs/SendersTab.tsx            -- sender names
│   │   ├── tabs/ConnectionsTab.tsx        -- API keys, SMPP, webhooks
│   │   └── tabs/MessagesTab.tsx           -- message logs
│   ├── TariffsPage.tsx                    -- all tariffs + defaults
│   ├── RoutesPage.tsx                     -- all routes + defaults
│   ├── AnalyticsPage.tsx                  -- detailed analytics
│   │   ├── tabs/BySubAccountsTab.tsx
│   │   ├── tabs/ByOperatorsTab.tsx
│   │   └── tabs/TimeSeriesTab.tsx
│   └── SimulatorPage.tsx                  -- tariff simulator
│
├── admin/
│   └── aggregators/                       -- admin management
│       ├── AggregatorsListPage.tsx
│       ├── AggregatorProfilePage.tsx
│       └── AggregatorAuditLogPage.tsx
```

Navigation: when aggregator logs in, sidebar shows "My Clients" section with sub-items: Dashboard, Sub-accounts, Tariffs, Routes, Analytics, Simulator.

## 8. Information Visibility Matrix

| Information | Platform | Aggregator | Sub-account |
|---|---|---|---|
| Platform global tariffs | Yes | No | No |
| Aggregator purchase tariff | Yes | Yes | No |
| Sub-account tariff | Yes | Yes | Yes |
| Aggregator margin | Yes | Yes | No |
| Aggregator real balance | Yes | Yes | No |
| Sub-account virtual balance | Yes | Yes | Yes |
| Providers / gateway names | Yes | Configurable | No |
| Sub-account message logs | Yes | Yes | Own only |
| Aggregator actions (audit) | Yes | No | No |

## 9. Implementation Phases

| Phase | Content | Outcome |
|---|---|---|
| 1. Core | Account types, aggregator profile, virtual balances, sub-account management | Aggregator can create sub-accounts and distribute balance |
| 2. Tarification | Sub-account tariffs, markup, constraints, dual deduction | Sub-accounts send SMS, aggregator earns margin |
| 3. Routing | Aggregator route creation, assignment to sub-accounts, defaults | Aggregator fully manages sub-account routing |
| 4. Analytics | Margin dashboard, breakdowns, forecasts, tariff simulator, export | Aggregator sees full business picture |
| 5. Control | Pool rate-limiting, cascade freeze, audit log, traffic limits | Platform has full control over aggregators |

## 10. Go Package Structure

```
internal/services/aggregator/
├── domain/
│   ├── models.go           -- AggregatorProfile, AggregatorTariff, MarginEntry
│   └── errors.go           -- domain errors
├── repository/
│   ├── profile.go          -- aggregator_profiles CRUD
│   ├── tariff.go           -- aggregator_tariffs CRUD
│   ├── margin.go           -- aggregator_margin_log queries
│   ├── audit.go            -- aggregator_audit_log writes/queries
│   └── default_routes.go   -- aggregator_default_routes CRUD
├── service/
│   ├── aggregator.go       -- profile management, sub-account lifecycle
│   ├── tarification.go     -- tariff lookup, validation, margin calculation
│   ├── analytics.go        -- dashboard, breakdowns, forecast, simulator
│   └── routing.go          -- route management, default routes
└── handler/
    ├── portal.go           -- HTTP handlers for /api/v1/aggregator/*
    └── admin.go            -- HTTP handlers for /api/v1/admin/aggregators/*
```

Reuses existing services: `billing` (balance operations), `tarification` (purchase tariff lookup), `routing` (client_routes, client_providers), `provider` (provider details).
