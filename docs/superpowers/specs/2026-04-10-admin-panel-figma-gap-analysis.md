# Admin Panel: Figma Design Gap Analysis

> **Date:** 2026-04-10
> **Figma file:** `FeU5OG5accz9vnsNZRaYpW` (page "MVP", node `9731:23573`)
> **Purpose:** Self-contained reference for AI-assisted implementation of missing admin panel features.

## Table of Contents

1. [Overview & Priority Map](#1-overview--priority-map)
2. [Current Architecture Summary](#2-current-architecture-summary)
3. [Sidebar Navigation Changes](#3-sidebar-navigation-changes)
4. [NEW-1: Детализация (Message Detail Log)](#new-1-детализация-message-detail-log)
5. [NEW-2: Список подключений / Kannel Management](#new-2-список-подключений--kannel-management)
6. [NEW-3: Индивидуальные маршруты (Client Routes)](#new-3-индивидуальные-маршруты-client-routes)
7. [NEW-4: Настройки (System Settings)](#new-4-настройки-system-settings)
8. [NEW-5: Индивидуальные тарифы (Client-Specific Tariffs)](#new-5-индивидуальные-тарифы-client-specific-tariffs)
9. [NEW-6: Юридические лица (Legal Entities)](#new-6-юридические-лица-legal-entities)
10. [NEW-7: Договоры (Contracts)](#new-7-договоры-contracts)
11. [REWORK-1: Имена отправителей (Sender Names)](#rework-1-имена-отправителей-sender-names)
12. [REWORK-2: Шаблоны операторов (Operator Templates)](#rework-2-шаблоны-операторов-operator-templates)
13. [REWORK-3: Справочник MCC/MNC (Enhanced Countries/Operators)](#rework-3-справочник-mccmnc-enhanced-countriesoperators)
14. [REWORK-4: Маршрутизация (Routing Overhaul)](#rework-4-маршрутизация-routing-overhaul)
15. [REWORK-5: Статистика (Statistics Redesign)](#rework-5-статистика-statistics-redesign)
16. [Implementation Order Recommendation](#16-implementation-order-recommendation)

---

## 1. Overview & Priority Map

### Figma Sections (18 total)

| # | Section | Figma Node | Figma Status | Implementation | Gap |
|---|---------|------------|-------------|----------------|-----|
| 1 | Клиенты | `9731:31575` | Green (done) | `/admin/clients` — ClientsPage.tsx | Minimal |
| 2 | Имена отправителей | `9808:29236` | Green (done) | `/admin/sender-names` — SenderNamesAdminPage.tsx | **REWORK** |
| 3 | Шаблоны операторов | `12077:58755` | Green (done) | `/admin/templates` — TemplatesPage.tsx | **REWORK** |
| 4 | Справочник MCC/MNC | `10226:33465` | Orange (WIP) | `/admin/countries` — CountriesPage.tsx | **REWORK** |
| 5 | Маршрутизация (общая) | `10226:34013` | Green (done) | `/admin/routes` — RoutesPage.tsx | **REWORK** |
| 6 | Авторизация | `10026:8997` | Green (done) | `/login` | Minimal |
| 7 | Детализация | `11108:24254` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 8 | Статистика | `11237:40043` | Green (done) | `/admin/analytics` — AnalyticsPage.tsx | **REWORK** |
| 9 | Поставщики | `11839:68503` | Green (done) | `/admin/providers` — ProvidersPage.tsx | Minimal |
| 10 | Каналы | `12146:83183` | Green (done) | `/admin/channels` — ChannelsPage.tsx | Minimal |
| 11 | Список подключений (Kannel) | `12149:95586` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 12 | Маршруты (инд.) | `13199:93902` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 13 | Настройки | `13199:93903` | Orange (WIP) | **NOT IMPLEMENTED** | **NEW** |
| 14 | Тарифы (общие) | `13533:136681` | Green (done) | `/admin/tarification` — TarificationPage.tsx | Partial match |
| 15 | Тарифы (индивидуальные) | `13607:102013` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 16 | Список юр.лиц | `15915:123972` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 17 | Договоры | `17427:155489` | Green (done) | **NOT IMPLEMENTED** | **NEW** |
| 18 | Статусы сообщений (sub) | `11033:35066` | Green (done) | **NOT IMPLEMENTED** | Inside Детализация |

### Figma Design Color Legend
- **Green border** = Design finalized, ready for implementation
- **Orange border** = Design still in progress
- **Purple border** = Needs review and approval
- **Red border** = Business requirements not yet gathered

---

## 2. Current Architecture Summary

### Tech Stack
- **Frontend:** TypeScript 5.7, React 19, Vite 6.0, React Router 7.1, Tailwind CSS 4.2, Radix UI, Recharts 3.8.1
- **Backend:** Go 1.24.0, gorilla/mux (HTTP), gRPC, pgx/v5 (PostgreSQL), go-redis/v9, zerolog, sarama (Kafka)
- **Database:** PostgreSQL 15+ (monthly partitioned tables), Redis 7+
- **Architecture:** Microservices with gRPC. HTTP gateways (admin/portal/client) convert REST to gRPC.

### Microservices (cmd/)
| Service | Port | Responsibility |
|---------|------|---------------|
| admin-gateway | HTTP | REST API for admin panel |
| portal-gateway | HTTP | REST API for client portal |
| client-gateway | HTTP | REST API for external clients |
| auth-service | 9090 | Authentication, users, roles |
| client-service | 9091 | Client management |
| provider-service | 9092 | Provider management |
| routing-service | 9093 | Routing, countries, operators, HLR |
| analytics-service | 9094 | Statistics, reports, real-time metrics |
| billing-service | 9095 | Accounts, transactions, pricing |
| webhook-service | 9096 | Event webhooks |
| messaging-service | 9097 | Message sending, DLR processing |
| template-service | 9099 | Templates + Sender names |
| tarification-service | 9100 | Tariff plans, periods, tiers |
| campaign-service | 9101 | Campaigns, A/B testing |
| contact-service | 9102 | Contact lists |
| link-service | 9103 | URL shortening |
| cascade-service | 9104 | Multi-channel delivery |
| smpp-gateway | — | SMPP protocol gateway |
| pipeline-worker | — | Message pipeline processor |

### Frontend File Structure
```
portal-frontend/src/
  api/admin.ts                     — All admin API functions
  components/layout/AdminSidebar.tsx — Sidebar navigation
  pages/admin/
    dashboard/DashboardPage.tsx
    ClientsPage.tsx
    SenderNamesAdminPage.tsx
    templates/TemplatesPage.tsx
    billing/BillingPage.tsx         — Tabs: Balances, Transactions, CreditLimits, Pricing
    tarification/TarificationPage.tsx — Tabs: Plans, Periods, Tiers, SenderRegistrations
    ProvidersPage.tsx
    RoutesPage.tsx
    channels/ChannelsPage.tsx
    delivery-strategies/DeliveryStrategiesPage.tsx
    HLRPage.tsx
    CountriesPage.tsx
    WebhooksPage.tsx
    AnalyticsPage.tsx
    MonitoringPage.tsx
    AuditLogPage.tsx
    users/UsersPage.tsx
    users/RolesPage.tsx
```

### Database Tables (key ones)
```
clients, users, sessions, api_keys, roles, permissions, role_permissions
providers, routes, client_routes, client_providers, client_routing_strategies
countries, operators, operator_prefixes
sender_names, sender_name_status_history, sender_name_billing_records
templates, template_audit_log, operator_template_registrations
tariff_plans, tariff_periods, tariff_tiers (legacy)
tariff_periods_new, tariff_tiers_new (hierarchical M081)
provider_cost_periods, provider_cost_tiers
accounts, transactions, balance_transfers, pricing_rules
messages (partitioned), dlr_receipts, audit_log (partitioned)
tarification_log (partitioned), usage_counters
campaigns, campaign_variants, campaign_recipients (hash-partitioned)
contact_lists, contacts, contact_imports
delivery_channels, delivery_strategies, delivery_strategy_steps
deliveries (partitioned), delivery_attempts (partitioned)
hlr_providers, lookup_log (partitioned)
webhook_subscriptions, notifications, system_defaults
```

### Admin API Base: `/admin/v1`
All admin HTTP endpoints are in `internal/gateway/admin/router/router.go` and handled by `internal/gateway/admin/handlers/*.go`.

---

## 3. Sidebar Navigation Changes

### Current Sidebar (AdminSidebar.tsx)
```
Dashboard
--- Основное ---
  Клиенты (/admin/clients)
  Шаблоны (/admin/templates)
  Имена отправителей (/admin/sender-names)
  Пользователи (/admin/users)
--- Финансы ---
  Биллинг (/admin/billing)
  Тарификация (/admin/tarification)
--- Инфраструктура ---
  Провайдеры (/admin/providers)
  Маршруты (/admin/routes)
  Каналы (/admin/channels)
  Стратегии доставки (/admin/delivery-strategies)
  HLR (/admin/hlr)
--- Справочники ---
  Страны (/admin/countries)
  Вебхуки (/admin/webhooks)
--- Наблюдение ---
  Аналитика (/admin/analytics)
  Мониторинг (/admin/monitoring)
  Аудит (/admin/audit)
```

### Figma Sidebar (from metadata frame analysis)
```
Клиенты
Имена отправителей
Шаблоны операторов          ← renamed from "Шаблоны"
--- Финансы ---
  Тарификация
--- Маршрутизация ---        ← new group
  [Routing items]
--- Справочники ---
  MCC/MNC                   ← renamed from "Страны"
--- Отчёты ---               ← renamed from "Наблюдение"
  Детализация               ← NEW
  Статистика                ← replaces Аналитика
--- ---
Настройки                   ← NEW
```

### Changes Required
1. Rename "Шаблоны" → "Шаблоны операторов" (complete functional rework)
2. Rename "Страны" → "Справочник MCC/MNC" (enhanced functionality)
3. Add "Детализация" under Отчёты section
4. Rename "Аналитика" → "Статистика" (reworked functionality)
5. Add "Настройки" as bottom-level section
6. Add "Маршрутизация" group (consolidating routing-related items)
7. Items from Figma NOT in current sidebar but implemented: Dashboard, Users, Billing, Providers, Channels, Delivery Strategies, HLR, Webhooks, Monitoring, Audit — these remain as-is, Figma focuses on other flows
8. NEW routes needed: `/admin/detalization`, `/admin/settings`, `/admin/connections`, `/admin/individual-routes`, `/admin/individual-tariffs`, `/admin/legal-entities`, `/admin/contracts`

---

## NEW-1: Детализация (Message Detail Log)

### Figma Reference
- **Section:** `11108:24254` (Green — ready)
- **Sub-section:** `11033:35066` "Статусы сообщений"
- **Frames found:** Multiple views — Default view, various filter states, message detail with status timeline

### What the Design Shows
A detailed **message log viewer** (not the same as Audit Log). Shows individual SMS messages with full delivery lifecycle:
- Table with message list (sender, recipient, text preview, status, date, provider)
- Detailed message view with status history timeline
- Status progression: pending → queued → sent → delivered/failed
- Filter by date range, status, client, provider
- "Статусы сообщений" sub-reference: visual legend for all possible statuses

### What Exists Now
- `AuditLogPage.tsx` — logs user ACTIONS (login, create, update, delete), not messages
- `MonitoringPage.tsx` — real-time metrics, not individual messages
- Backend: `messages` table exists (partitioned monthly) with full lifecycle fields
- Backend: `dlr_receipts` table exists for delivery reports
- No admin endpoint for message search/detail exists

### What Needs to Be Built

**Backend:**
- New admin endpoint: `GET /admin/v1/messages` — search/filter messages
  - Params: `client_id?`, `from?`, `to?`, `status?`, `destination?`, `source?`, `provider_id?`, `limit`, `offset`
  - Returns: paginated list with key fields
- New admin endpoint: `GET /admin/v1/messages/{id}` — get full message detail
  - Returns: message + related DLR receipts + tarification log entry
- Handler: `internal/gateway/admin/handlers/messages.go`
- gRPC: May need new method on `MessagingService` or `AnalyticsService`: `SearchMessages`, `GetMessageDetail`

**Database:**
- No schema changes needed — `messages` and `dlr_receipts` tables already exist
- Consider index on `messages(client_id, created_at)` for admin queries if not present

**Frontend:**
- New file: `pages/admin/detalization/DetalizationPage.tsx`
- Components:
  - FilterBar: date range, status (multi-select), client_id, provider_id, destination (phone search)
  - DataTable: columns — created_at, source, destination, text (truncated), status (badge), provider, segments
  - MessageDetailModal or separate page: full message info + DLR timeline + tarification info
  - StatusTimeline component: visual pipeline of status transitions with timestamps
- API: new functions in `api/admin.ts`: `messagesApi.list()`, `messagesApi.get(id)`
- Route: `/admin/detalization`

### Key UX From Design
- Message statuses shown as colored badges (like current implementation pattern)
- Detail view shows chronological status history: pending (grey) → sent (blue) → delivered (green) / failed (red)
- Includes DLR receipt data (submit_date, done_date, stat, error code)

---

## NEW-2: Список подключений / Kannel Management

### Figma Reference
- **Section:** `12149:95586` (Green — ready)
- **Key frame:** "Список подключений" at `12149:88261` (1440x924)

### What the Design Shows
Management interface for **SMPP connections** at the Kannel/gateway level:
- List of active SMPP connections/sessions
- Connection details: provider name, system_id, host:port, bind type, status (bound/unbound/error), active sessions count, TPS
- Actions: start/stop connections, view logs, reconnect
- This is **different from Providers** — Providers define configuration, Connections show runtime state

### What Exists Now
- `ProvidersPage.tsx` — CRUD for provider configs (static data)
- Provider health endpoint exists: `GET /admin/v1/providers/{id}/health`
- Backend `smpp-gateway` service manages actual SMPP connections
- No admin API for connection-level management

### What Needs to Be Built

**Backend:**
- New admin endpoints:
  - `GET /admin/v1/connections` — list active SMPP connections
  - `GET /admin/v1/connections/{id}` — connection detail with real-time metrics
  - `POST /admin/v1/connections/{id}/reconnect` — force reconnect
  - `POST /admin/v1/connections/{id}/stop` — gracefully close connection
- These need to communicate with the `smpp-gateway` service (possibly via gRPC or Redis pub/sub)
- New gRPC service or extend `ProviderService` with connection management methods

**Database:**
- May not need new tables — connection state is runtime
- Optional: `smpp_connections` table for connection audit log

**Frontend:**
- New file: `pages/admin/connections/ConnectionsPage.tsx`
- Components:
  - DataTable: provider name, system_id, host:port, bind_type, status (badge: bound/unbound/error), active_sessions, current_tps, uptime
  - Row actions: Reconnect, Stop
  - Auto-refresh with polling (like MonitoringPage pattern)
  - Connection detail view: message throughput chart, error log, last N messages
- API: `connectionsApi.list()`, `connectionsApi.get(id)`, `connectionsApi.reconnect(id)`, `connectionsApi.stop(id)`
- Route: `/admin/connections`

---

## NEW-3: Индивидуальные маршруты (Client Routes)

### Figma Reference
- **Section:** `13199:93902` (Green — ready)
- **Key frame:** "Список инд.маршрутов" at `12763:77302`

### What the Design Shows
Client-specific routing rules, separate from platform-level routes:
- Filtered by client
- Each route: client → operator → provider mapping with priority and weight
- Different from general routes which are regex/pattern-based

### What Exists Now
- `RoutesPage.tsx` — platform-level routes (regex patterns → provider)
- Backend: `client_routes` table EXISTS with fields: client_id, operator_id, provider_id, priority, weight, active, shared
- Backend: API endpoints EXIST: `POST/GET/PUT/DELETE /admin/v1/clients/{id}/routes`
- **Frontend page for client routes does NOT exist** — the API is there, just no UI

### What Needs to Be Built

**Backend:**
- Already exists! Client route endpoints are under `/admin/v1/clients/{id}/routes`
- May want to add a flat list endpoint: `GET /admin/v1/client-routes?client_id=X` for easier admin access

**Frontend:**
- New file: `pages/admin/client-routes/ClientRoutesPage.tsx`
- Components:
  - Client selector (SearchableSelect)
  - DataTable: client_name, operator_name, provider_name, priority, weight, active (badge), shared (badge)
  - Create modal: client_id (select), operator_id (select), provider_id (select), priority (number), weight (number)
  - Edit modal: same fields
  - Delete confirmation
  - Drag-and-drop reordering (priority) — as shown in Figma design
- API: reuse `clientsApi` or create `clientRoutesApi` wrapper
- Route: `/admin/individual-routes`

---

## NEW-4: Настройки (System Settings)

### Figma Reference
- **Section:** `13199:93903` (Orange — in progress)
- **Key frames:** "Настройки--default" at `12468:92774`, "Настройки-- внесены изменения" at `12077:67805`
- Large section (26069x10617 px) — indicates extensive settings

### What the Design Shows
System-wide settings page with multiple sections:
- General platform settings
- Default values for various parameters
- Configuration that currently requires code changes or direct DB access
- Shows "unsaved changes" indicator pattern — sections highlight when modified

### What Exists Now
- `system_defaults` table with key-value pairs
- API: `GET /admin/v1/system/defaults`, `PUT /admin/v1/system/defaults/{key}`
- No dedicated frontend page — settings accessed only via direct API calls

### What Needs to Be Built

**Backend:**
- Mostly exists via `system_defaults` API
- May need to expand the set of configurable keys
- Consider grouping defaults by category for UI sections

**Frontend:**
- New file: `pages/admin/settings/SettingsPage.tsx`
- Components:
  - Section-based layout (cards or accordion)
  - Each section: group of related settings with form fields
  - "Unsaved changes" detection per section (Figma shows save button appearing only when changes made)
  - Section-level save buttons
  - Possible sections:
    - General (platform name, default timezone, etc.)
    - SMS defaults (default encoding, max retries, retry interval)
    - Rate limits (global defaults)
    - Notifications (alert thresholds)
    - Security (session timeout, password policy)
- API: extend `systemDefaultsApi` or create `settingsApi` with grouped reads/writes
- Route: `/admin/settings`

---

## NEW-5: Индивидуальные тарифы (Client-Specific Tariffs)

### Figma Reference
- **Section:** `13607:102013` (Green — ready)
- **Key frame:** "Default" at `12763:76944` (1440x1260)

### What the Design Shows
Per-client tariff configuration:
- Select client, then manage their specific tariff periods/tiers
- Same hierarchical model as general tariffs but scoped to a client
- Shows: client selector, then operator/sender_category/traffic_type scope + periods + tiers
- Визуально отдельный от общих тарифов

### What Exists Now
- Hierarchical tariff system (M081) already supports `client_id` dimension!
- `tariff_periods_new` has `client_id` field — when set, applies only to that client
- `scope_priority` gets +100 for client-specific periods
- Backend endpoints exist: `POST/GET/PUT/DELETE /admin/v1/tarification/periods` with `client_id` parameter
- **Frontend: PeriodsTab already supports client_id filter, but there's no dedicated page for "individual tariffs"**

### What Needs to Be Built

**Backend:**
- Already fully supported! No changes needed.

**Frontend:**
- New file: `pages/admin/tarification/IndividualTariffsPage.tsx`
- Components:
  - Client selector (SearchableSelect) — required first step
  - Once client selected: same UI as TarificationPage but with client_id pre-filled
  - Periods list (filtered by client_id)
  - Tiers per period
  - Create period modal with client_id auto-filled
  - Visual indicator: "Индивидуальный тариф для [Client Name]"
- Can reuse `PeriodsTab` and `TiersTab` components with `clientId` prop
- Route: `/admin/individual-tariffs`

---

## NEW-6: Юридические лица (Legal Entities)

### Figma Reference
- **Section:** `15915:123972` (Green — ready)
- **Key notes from metadata:**
  - "Добавление доп.юр.лица" — adding additional legal entities
  - "Выбрано юр.лицо" at `12357:77588` — selected entity view
  - "Ввод названия юр.лица" at `12357:76742`
  - Designer notes: "Добавить второе и последующие юр.лица можно только из списка all networks. Если юр.лицо удаляется, то оно возвращается обратно в all networks"

### What the Design Shows
Management of legal entities (companies) that can be linked to operators:
- List of legal entities with INN (tax ID), company name
- CRUD operations: create, edit, delete
- Association with operators — each operator can have one or more legal entities
- "All Networks" concept — unassigned entities available for any operator
- When entity removed from operator → returns to "all networks" pool

### What Exists Now
- **Nothing.** No `legal_entities` table, no API endpoints, no frontend.
- The `operators` table has `monthly_tariff_amount` but no legal entity reference.

### What Needs to Be Built

**Database:**
- New table: `legal_entities`
  ```sql
  CREATE TABLE legal_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inn VARCHAR(12) NOT NULL,
    name VARCHAR(255) NOT NULL,
    full_name TEXT,
    address TEXT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
  );
  ```
- New junction table: `operator_legal_entities`
  ```sql
  CREATE TABLE operator_legal_entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID REFERENCES operators(id),
    legal_entity_id UUID REFERENCES legal_entities(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(operator_id, legal_entity_id)
  );
  ```

**Backend:**
- New gRPC methods (extend RoutingService or new LegalEntityService):
  - `CreateLegalEntity`, `UpdateLegalEntity`, `DeleteLegalEntity`, `ListLegalEntities`, `GetLegalEntity`
  - `AssignLegalEntityToOperator`, `RemoveLegalEntityFromOperator`, `ListOperatorLegalEntities`
- New admin HTTP endpoints:
  - `POST/GET /admin/v1/legal-entities`
  - `GET/PUT/DELETE /admin/v1/legal-entities/{id}`
  - `POST /admin/v1/operators/{id}/legal-entities` — assign
  - `DELETE /admin/v1/operators/{id}/legal-entities/{le_id}` — remove
  - `GET /admin/v1/operators/{id}/legal-entities` — list for operator

**Frontend:**
- New file: `pages/admin/legal-entities/LegalEntitiesPage.tsx`
- Components:
  - DataTable: INN, name, linked operators count, active (badge)
  - Create/Edit modal: inn, name, full_name, address
  - Delete confirmation
  - Operator assignment (accessible from CountriesPage operator detail too)
- Route: `/admin/legal-entities`

---

## NEW-7: Договоры (Contracts)

### Figma Reference
- **Section:** `17427:155489` (Green — ready)
- **Key metadata:** Contract items show INN + company name, linked to legal entities
- Frame "Договоры" at `12763:76618` with Contract sub-frames

### What the Design Shows
Contract management per client:
- List of contracts with: number, date, client, legal entity, status
- Contract detail view
- Link contracts to clients and legal entities
- Status tracking (active, expired, terminated)

### What Exists Now
- **Nothing.** No contracts table, no API, no frontend.

### What Needs to Be Built

**Database:**
- New table: `contracts`
  ```sql
  CREATE TABLE contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_number VARCHAR(100) NOT NULL,
    client_id UUID REFERENCES clients(id),
    legal_entity_id UUID REFERENCES legal_entities(id),
    status VARCHAR(20) DEFAULT 'active', -- active, expired, terminated
    start_date DATE NOT NULL,
    end_date DATE,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
  );
  ```

**Backend:**
- New gRPC service or extend existing
- New admin HTTP endpoints:
  - `POST/GET /admin/v1/contracts`
  - `GET/PUT/DELETE /admin/v1/contracts/{id}`
  - `GET /admin/v1/clients/{id}/contracts` — list contracts for client

**Frontend:**
- New file: `pages/admin/contracts/ContractsPage.tsx`
- Components:
  - DataTable: contract_number, client_name, legal_entity (INN + name), status, start_date, end_date
  - Create/Edit modal: contract_number, client_id (SearchableSelect), legal_entity_id (SearchableSelect), start_date, end_date, description
  - Status badge
- Route: `/admin/contracts`

---

## REWORK-1: Имена отправителей (Sender Names)

### Figma Reference
- **Section:** `9808:29236` (Green — ready)
- **Sub-sections with labels:**
  - "Список" — List view
  - "Добавить имя" — Add name (modal with validation rules)
  - "Просмотр имени" — Detailed name view page
  - "Удаление имени внутри страницы" — Delete from detail page
  - "Добавление шаблона оператора со страницы имени" — Add operator template from name page

### What the Design Shows
Much richer than current implementation:

1. **List View:** Table with sender names, their status, client, type of service
2. **Add Name Modal:** Form with validation rules visible:
   - "Имя должно совпадать с названием организации, ИП, товарным знаком или доменом"
   - "Латиница, не более 11 символов. Можно использовать цифры и знаки . _ —"
   - "Пробелы запрещены"
3. **Detail Page (NEW):** Full page view with:
   - **"Общая информация"** section: name, client, status, created_at
   - **"Шаблоны операторов:"** section — shows linked operator templates
   - **"Регистрация имени у операторов"** section — per-operator registration status grid showing:
     - MTS, Beeline, t2, Megafon, Yota, Gazprom Mobile, Motiv, T-Mobile, SberMobile, Rostelecom, Win mobile, Volna mobile, Miranda, MTT, VTB-mobile, Alfa-mobile
     - Each with individual registration status
4. **Delete Flow:** Confirmation, redirect to list, toast notification
5. **Add Operator Template:** Can add operator template directly from the name detail page

### What Exists Now
- `SenderNamesAdminPage.tsx` — simple table with approve/reject/deactivate modals
- Backend: `sender_names` table with basic status workflow
- Backend: `operator_template_registrations` table exists (links templates to operator registrations)
- No detail page, no per-operator registration view

### What Needs to Be Changed

**Backend:**
- New endpoint: `GET /admin/v1/sender-names/{id}` — get sender name detail
- New endpoint: `GET /admin/v1/sender-names/{id}/operator-registrations` — get per-operator status
- May need new table or view: `sender_name_operator_status` to track registration with each operator
- Extend SenderName proto with operator registration data

**Frontend:**
- Modify `SenderNamesAdminPage.tsx`:
  - Add validation hints in create/edit modal
  - Add click-to-detail navigation
- New file: `pages/admin/sender-names/SenderNameDetailPage.tsx`
  - "Общая информация" card
  - "Шаблоны операторов" linked list
  - "Регистрация у операторов" grid — each operator as a row/card with status badge
  - Delete button with confirmation
  - "Добавить шаблон оператора" button
- Route: `/admin/sender-names/:id`

---

## REWORK-2: Шаблоны операторов (Operator Templates)

### Figma Reference
- **Section:** `12077:58755` (Green — ready)
- **Sub-sections:**
  - "Список" — Template list with filters
  - "Удаление шаблонов" — Delete workflow
  - "Редактирование шаблона" — Template editor
  - "Все фильтры" — Advanced filters panel

### What the Design Shows
**Completely different concept from current Templates page:**

1. **List View:** Operator templates (not client review workflow)
   - Columns: name, operator, sender_name, text preview, status, created_at
   - Rich filtering: by operator, sender name, status, date range
2. **Template Editor:**
   - "Текст шаблона с переменными" — template body with variable placeholders
   - "Вставить переменную:" — variable insertion button/dropdown
   - Variables in `{variable_name}` format
   - Textarea with preview
3. **Filters Panel:** Expandable panel with multiple filter criteria
4. **Delete:** Confirmation dialog, bulk delete support

### What Exists Now
- `TemplatesPage.tsx` — **client template review workflow** (pending → review → approve/reject)
  - This is about reviewing templates submitted by clients
- `operator_template_registrations` table — links templates to operators for registration
- Template proto has: id, client_id, name, body, variables[], status, sender_name_id, traffic_type

### What Needs to Be Changed

**This is a conceptual split:**
- Current "Templates" page = client template review (keep as is, maybe rename to "Модерация шаблонов")
- New "Шаблоны операторов" page = operator-level SMS template management

**Backend:**
- The existing template system may be extended or a new entity created
- Key difference: operator templates are templates registered WITH operators (for SMS approval)
- Consider new entity `operator_templates` or extend `operator_template_registrations`
- New endpoints:
  - `GET /admin/v1/operator-templates` — list operator templates
  - `POST /admin/v1/operator-templates` — create (with body, variables, operator_id, sender_name_id)
  - `PUT /admin/v1/operator-templates/{id}` — edit
  - `DELETE /admin/v1/operator-templates/{id}` — delete

**Frontend:**
- New file: `pages/admin/operator-templates/OperatorTemplatesPage.tsx`
- Components:
  - FilterBar: operator (select), sender_name (select), status, date range
  - DataTable: name, operator_name, sender_name, body (truncated), status, created_at
  - Create/Edit page or modal:
    - operator_id (select)
    - sender_name_id (select, filtered by sender names)
    - name (text)
    - body (textarea with variable highlighting)
    - Variable insertion toolbar
  - Delete confirmation
- Route: `/admin/operator-templates`
- Rename existing `/admin/templates` to `/admin/template-moderation` or keep as-is

---

## REWORK-3: Справочник MCC/MNC (Enhanced Countries/Operators)

### Figma Reference
- **Section:** `10226:33465` (Orange — partially done)
- **Sub-sections (from metadata):**
  - "Список" — Country/operator list
  - "Просмотр страны" — Country detail
  - "Просмотр оператора (Россия)" — Russian operator detail
  - "Просмотр оператора (другие страны)" — Foreign operator detail
  - "Добавление нового оператора" — Multi-step: "Шаг 1. Основная информация", "Шаг 2. Настройки"
  - "Добавление доп.юр.лица" — Legal entity management per operator
  - Фильтры — Filters panel

### What the Design Shows
Significantly enhanced compared to current CountriesPage:

1. **MCC/MNC Table:** Primary view shows MCC, MNC, operator name (not just country → operator hierarchy)
2. **Country Detail:** Expanded info, list of operators
3. **Operator Detail View:** Different for Russia vs other countries
   - **Russia:** shows юр.лица, settings for paid/free sender support, monthly tariff, prefixes
   - **Other countries:** simplified view without юр.лица
4. **Operator Creation Wizard:** 2-step process
   - Step 1: Basic info (name, MCC, MNC, country)
   - Step 2: Settings (paid/free sender support, tariff amount)
5. **Legal Entity Management:** Per-operator юр.лица with add/remove
6. **Filters:** Country and operator filters are interdependent
   - Designer note: "Фильтр страны и оператора взаимозависимые. По выбранной стране подгружаются операторы"
7. **Settings per operator:** 
   - Designer note: "Настройки (поддержка платных и бесплатных имен) уже будут предзаполнены, с бэка. Но их можно поменять руками в интерфейсе."

### What Exists Now
- `CountriesPage.tsx` — 3-column hierarchical view (Countries | Operators | Prefixes)
- Simple create modals for countries and operators
- Operator fields: name, mcc, mnc, supports_paid_sender, supports_free_sender, monthly_tariff_amount
- No юр.лица, no 2-step wizard, no separate detail pages

### What Needs to Be Changed

**Backend:**
- Add legal entity association (see NEW-6)
- Add `GET /admin/v1/operators/{id}` detail endpoint with юр.лица included
- Possibly extend operator with additional settings fields

**Frontend:**
- Major rework of `CountriesPage.tsx` → rename to `MccMncPage.tsx`
- New components:
  - MCC/MNC table as primary view (flat table with MCC, MNC, operator, country columns)
  - Country detail page: `/admin/countries/:id`
  - Operator detail page: `/admin/operators/:id`
    - General info section
    - Settings section (paid/free sender support)
    - Юр.лица section (list + add/remove)
    - Prefixes section
  - 2-step operator creation wizard (Step 1: info, Step 2: settings)
  - Interdependent country/operator filters
- Route: `/admin/mcc-mnc` (or keep `/admin/countries` and add sub-routes)

---

## REWORK-4: Маршрутизация (Routing Overhaul)

### Figma Reference
- **Section:** `10226:34013` (Green — ready)
- **Key text found in metadata:**
  - "Маршрутизация" page header
  - Table columns include "Канал" (Channel) and "Поставщик" (Provider)
  - "Список маршрутов" — route list
  - "Добавление маршрута" — create route
  - "Удаление маршрута" — delete route
  - "Маршрут перемещен" — route reordering/drag-and-drop
  - Designer note: "Если выбрано All Networks, то появляется доп.селект с выбором юр.лица. Обязателен для заполнения."

### What the Design Shows
Completely different routing model from current:

1. **Route Table:** Each route has:
   - Priority/order (drag-and-drop reorderable)
   - Operator or "All Networks"
   - Channel (SMS, Flash Call, etc.)
   - Provider
   - Юр.лицо (when "All Networks" selected)
2. **Drag & Drop:** Routes can be reordered by dragging (priority changes)
3. **Add Route Modal:** Operator selector with "All Networks" option, channel selector, provider selector, conditional юр.лицо selector
4. **Delete Route:** Confirmation dialog
5. **All Networks + Юр.лицо:** When route applies to all operators, must specify legal entity

### What Exists Now
- `RoutesPage.tsx` — regex-pattern-based routes with provider_ids array and load_balance_strategy
- Current model: `routes` table with `pattern` (regex), `provider_ids`, `load_balance_strategy`
- Backend also has `client_routes` (operator-based) but no frontend for it

### What Needs to Be Changed

**This is a fundamental model change:**
- Current: Pattern-based routing → provider selection
- Figma: Operator-based routing → channel → provider, with юр.лицо

**Backend:**
- May need to extend the `client_routes` model or create a new routing table
- Add `channel_id` field to routes
- Add `legal_entity_id` field for "All Networks" routes
- Add reorder/priority update endpoint
- Route priority update: `PUT /admin/v1/routes/reorder` (batch priority update)

**Frontend:**
- Major rework of `RoutesPage.tsx`
- New components:
  - DataTable with drag-and-drop reordering (use @dnd-kit or similar)
  - Columns: order/priority, operator (or "All Networks"), channel, provider, юр.лицо, actions
  - Create modal:
    - operator_id select (with "All Networks" option)
    - channel_id select
    - provider_id select
    - legal_entity_id select (conditional — shown only when "All Networks")
  - Delete confirmation
  - Reorder handler (updates priorities via API)

---

## REWORK-5: Статистика (Statistics Redesign)

### Figma Reference
- **Section:** `11237:40043` (Green — ready)
- **Key frame:** "Группировка: По дням" at `11237:40044`
- Designer note: "Фиксированная высота контейнера с таблицей. скролл внутри контейнера (по аналогии в ЛК клиента, раздел 'Статистика'). Прилипающая шапка и саммари снизу"

### What the Design Shows
Enhanced statistics with multiple grouping modes:

1. **Grouping Options:** По дням (by day), По операторам (by operator), По странам (by country)
2. **Fixed-height table** with internal scroll, sticky header, and summary row at bottom
3. **Sticky summary:** Totals row pinned to bottom of table
4. **Multiple views** — designer created separate frames per grouping mode
5. **Same data, different aggregation**

### What Exists Now
- `AnalyticsPage.tsx`:
  - Period selector: 7d/30d/90d
  - group_by: day/week/country
  - StatCards: total_sent, delivered, failed, delivery_rate, cost
  - LineChart: delivered vs failed over time
  - BarChart: by provider
- Functional but different from Figma design

### What Needs to Be Changed

**Backend:**
- May need new grouping: `group_by=operator` (currently only day/week/country)
- Add summary totals in response
- Consider separate endpoint for grouped stats: `GET /admin/v1/analytics/grouped-stats`

**Frontend:**
- Rework `AnalyticsPage.tsx` → rename to `StatisticsPage.tsx`
- New components:
  - Grouping toggle tabs (По дням | По операторам | По странам)
  - Fixed-height DataTable container with `overflow-y: auto` and `position: sticky` header
  - Summary row (sticky bottom): totals for sent, delivered, failed, cost
  - Columns change per grouping:
    - По дням: date, sent, delivered, failed, delivery_rate, cost
    - По операторам: operator_name, sent, delivered, failed, delivery_rate, cost
    - По странам: country_name, sent, delivered, failed, delivery_rate, cost
  - Keep existing chart components as supplementary
- Route: keep `/admin/analytics` or rename to `/admin/statistics`

---

## 16. Implementation Order Recommendation

### Phase 1: Low-hanging fruit (backend exists, need frontend only)
1. **NEW-3: Индивидуальные маршруты** — backend API exists, just need frontend page
2. **NEW-5: Индивидуальные тарифы** — backend fully supports it, frontend page needed
3. **NEW-4: Настройки** — system_defaults API exists, need settings UI

### Phase 2: Frontend rework (existing pages, enhanced UX)
4. **REWORK-5: Статистика** — enhance existing AnalyticsPage
5. **REWORK-1: Имена отправителей** — add detail page + operator registration view
6. **REWORK-3: Справочник MCC/MNC** — enhance CountriesPage

### Phase 3: New features requiring backend + frontend
7. **NEW-1: Детализация** — new message log viewer (data exists, need API + UI)
8. **REWORK-2: Шаблоны операторов** — new concept, needs new entity
9. **NEW-6: Юридические лица** — new entity, new tables + API + UI
10. **NEW-7: Договоры** — new entity, depends on legal entities

### Phase 4: Complex infrastructure changes
11. **REWORK-4: Маршрутизация** — fundamental model change, affects core routing
12. **NEW-2: Kannel Management** — requires SMPP gateway integration

### Sidebar navigation update should happen incrementally with each phase.

---

## Appendix: Figma Node ID Quick Reference

Use these to fetch screenshots or design context when Figma MCP rate limit resets:

| Section | Node ID | Size |
|---------|---------|------|
| Клиенты | `9731:31575` | 4348x6050 |
| Имена отправителей | `9808:29236` | 10019x6083 |
| Шаблоны операторов | `12077:58755` | 8548x6083 |
| Справочник MCC/MNC | `10226:33465` | 19966x7137 |
| Маршрутизация | `10226:34013` | 16096x7131 |
| Авторизация | `10026:8997` | 2398x4750 |
| Детализация | `11108:24254` | 7680x5593 |
| Статистика | `11237:40043` | 9421x5593 |
| Поставщики | `11839:68503` | 5951x5748 |
| Каналы | `12146:83183` | 7639x6531 |
| Список подключений (Kannel) | `12149:95586` | 7636x6484 |
| Маршруты | `13199:93902` | 13761x5992 |
| Настройки | `13199:93903` | 26069x10617 |
| Тарифы (общие) | `13533:136681` | 17437x11047 |
| Тарифы (индивидуальные) | `13607:102013` | 15206x9487 |
| Список юр.лиц | `15915:123972` | 12639x3871 |
| Договоры | `17427:155489` | 4741x4610 |

**Figma file key:** `FeU5OG5accz9vnsNZRaYpW`
**Figma page:** "MVP" (node `9731:23573`)
