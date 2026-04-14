# Client Portal: Mission Control Redesign

> **Date:** 2026-04-11
> **Purpose:** Comprehensive redesign of the client self-service portal with Mission Control concept — real-time command center dashboard, action-oriented navigation, and wow-factor features for competitive advantage.
> **Approach:** Hybrid — Mission Control as new dark-theme dashboard + existing pages restructured by action-oriented navigation with wow-improvements.
> **Audience:** Mixed — developers, marketers, business/finance managers.
> **No Figma:** Design derived from admin panel patterns and logic.

---

## Table of Contents

1. [Information Architecture](#1-information-architecture)
2. [Tech Stack & Constraints](#2-tech-stack--constraints)
3. [Phase 1: Command Center](#phase-1-command-center)
4. [Phase 2: «Отправить»](#phase-2-отправить)
5. [Phase 3: «Отследить»](#phase-3-отследить)
6. [Phase 4: «Аналитика»](#phase-4-аналитика)
7. [Phase 5: «Контакты»](#phase-5-контакты)
8. [Phase 6: «Финансы»](#phase-6-финансы)
9. [Phase 7: «Интеграции»](#phase-7-интеграции)
10. [Phase 8: «Настройки»](#phase-8-настройки)
11. [Phase 9: Cross-Cutting Improvements](#phase-9-cross-cutting-improvements)
12. [New Backend Endpoints Summary](#new-backend-endpoints-summary)
13. [Implementation Order](#implementation-order)

---

## 1. Information Architecture

### Current Navigation (5 groups, entity-oriented)

```
Dashboard, Analytics
Messaging: Messages, Detalization, Campaigns, Schedules, Templates, Sender Names
Contacts: Lists, Segments, Opt-out
Finance: Billing, Tariffs
Integrations: API Keys, Webhooks, Providers, Routing, Lookup
Settings: Profile, Sub-accounts, Domains, Notifications, SMPP, Default Senders, Audit, Cascade History
```

### New Navigation (8 groups, action-oriented)

```
Командный центр          — Live Dashboard (dark theme)
Отправить                — Quick Send, Campaigns, Wizard, Schedules, Templates, Sender Names
Отследить                — Live Feed, Detalization, Cascade History, Troubleshooting
Аналитика                — Statistics, Comparisons, Cost Simulator
Контакты                 — Lists, Segments, Opt-out
Финансы                  — Billing, Tariffs, Documents, Calculator
Интеграции               — API Keys, Webhooks, Providers, Routing, Lookup, SMPP
Настройки                — Profile, Sub-accounts, Domains, Notifications, Default Senders, Audit
```

### Key Principles

- Navigation follows client workflow: prepare → send → track → analyze → pay
- Command Center is the only dark-theme page — contrast creates wow on entry
- 8 new pages: Command Center, Quick Send, Live Feed, Troubleshooting, Comparisons, Cost Simulator, Documents, Calculator
- All existing pages preserved, regrouped
- SMPP moves from Settings to Integrations (logical fit)

---

## 2. Tech Stack & Constraints

- **Frontend:** TypeScript 5.7, React 19, Vite 6.0, React Router 7.1, Tailwind CSS 4.2, Radix UI, Recharts 3.8.1
- **Backend:** Go 1.24.0, gorilla/mux (REST), gRPC (inter-service), pgx/v5, go-redis/v9
- **Database:** PostgreSQL 15+, Redis 7+
- **Existing components:** Reuse all from `portal-frontend/src/components/ui/` and `components/data/` — Button, Input, Select, Modal, Badge, Toast, DataTable, FilterBar, StatCard, PageHeader, etc.
- **No new frontend dependencies** unless explicitly required (e.g., WebSocket client is built-in)
- **Real-time:** WebSocket for live feeds, polling as fallback. SSE optional future enhancement.

---

## Phase 1: Command Center

**Route:** `/dashboard` (replaces current dashboard)
**Theme:** Dark (`bg-slate-900`, `text-slate-200`), CSS variables for theming
**New file:** `portal-frontend/src/pages/CommandCenter.tsx` (replaces `DashboardPage.tsx`)

### Layout — 3-Column Grid

#### Top Bar
- Logo + client company name
- Global status: `● Все системы в норме` / `⚠ 1 провайдер деградирует` (derived from health data)
- Balance (quick access, clickable → billing)
- Notification bell with unread badge count

#### Row 1 — 3 KPI Cards

| Card | Data | Visual |
|------|------|--------|
| Speed | Current msg/sec, trend vs yesterday (%) | Sparkline (1h mini-chart, no axes) |
| Delivery Rate | 24h %, trend vs last week | Progress bar (green >95%, yellow 85-95%, red <85%) |
| Balance | Amount, burn rate ₽/hr, forecast "lasts Xh" | Progress bar showing depletion |

Each card: dark card (`bg-slate-800`), colored accent for the metric, trend arrow with percentage.

#### Row 2 — Live Feed (2/3) + Health Map (1/3)

**Live Feed:**
- Last ~20 messages, updating via WebSocket
- Each row: `HH:MM:SS` | status badge (colored) | masked phone | operator → provider | text fragment (40 chars)
- Pause/resume button
- Click row → navigate to detalization detail
- Counter: `847 msg/sec | 12,430 за последние 5 мин`

**Health Map:**
- List of client's providers
- Per provider: name, connection type (SMPP/HTTP), connections (active/total), success rate (colored by thresholds), msg/sec
- Red border on degraded provider card

#### Row 3 — Smart Alerts (1/3) + Active Campaigns (2/3)

**Smart Alerts:**
- Stack of up to 5 recent alerts
- Color-coded left border: red (critical), yellow (warning), green (success), blue (info)
- Alert types:
  - Provider degrading (success rate below threshold)
  - Low balance (burn rate forecast)
  - Campaign completed (with delivery rate)
  - Template approved/rejected
  - Sender name status change
- Each alert: title, description, relative time
- "Все уведомления →" link

**Active Campaigns:**
- Running campaigns: name, start time, progress bar (sent/total), ETA, current delivery rate
- Transactional flows (OTP, notifications): separate row with daily stats
- "+ Новая кампания" link → campaign wizard

### Auto-Refresh Strategy

| Component | Method | Interval |
|-----------|--------|----------|
| KPI cards | Polling | 15s |
| Live feed | WebSocket (primary), polling fallback | Real-time / 5s |
| Health Map | Polling | 30s |
| Smart Alerts | Polling | 30s |
| Active Campaigns | Polling | 15s |

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/ws/messages` | WebSocket | Real-time message stream for client | **New** |
| `/portal/v1/dashboard/metrics` | GET | KPI with burn rate and forecast | **Extend existing** |
| `/portal/v1/providers/health` | GET | Client's provider health (filtered from admin health API) | **New** |
| `/portal/v1/alerts` | GET | Smart alerts for client | **New** |

### Acceptance Criteria

- [ ] Dark theme renders correctly, transitions smoothly to light pages
- [ ] KPI cards show real-time data with trends and sparklines
- [ ] Live feed streams messages via WebSocket with pause/resume
- [ ] Health Map shows provider status with color-coded thresholds
- [ ] Smart Alerts display categorized notifications
- [ ] Active Campaigns show progress with ETA
- [ ] All data auto-refreshes at specified intervals
- [ ] Mobile responsive: single-column stack

---

## Phase 2: «Отправить»

### 2.1 Quick Send (new page)

**Route:** `/quick-send`
**New file:** `portal-frontend/src/pages/QuickSendPage.tsx`

Single-screen form for sending 1-100 messages without campaign wizard.

| Field | Type | Description |
|-------|------|-------------|
| Recipients | Textarea + chips | Enter numbers via comma/newline. Each → chip with format validation. Max 100 |
| Sender Name | Select | From client's approved sender names. Default sender pre-selected |
| Message text | Textarea | Character counter with segment calculation (160/306/459...). Template variable support |
| Template | Optional select | "Использовать шаблон" — fills text from approved template |
| Preview | Side panel | SMS Preview in "phone frame" — updates in real-time on typing |
| Cost | Inline | Auto-calculation: N messages × tariff = ₽X. Updates on recipient change |
| Send | Button | Primary, with confirmation dialog if >10 recipients |

**After send:** toast "Отправлено: 5 сообщений" + link "Отследить →" navigates to live feed filtered by this batch.

**Backend:**
| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/quick-send` | POST | Send without campaign creation | **New** (or adapt existing messages API) |

### 2.2 Campaigns — Enhancements

Existing 6-step wizard preserved. Additions:

- **Step 6 (cost confirmation):** Add pie chart of costs by operator, delivery rate prediction based on historical data for this segment
- **After launch:** Redirect to Command Center (not campaign list) where campaign is visible in "Active Campaigns" with live progress
- **A/B test results:** Side-by-side visualization of variants — delivery rate, click rate (if links), cost per variant. Winner highlighted with badge

### 2.3 Templates — Enhancements

- **Variable highlighting:** `{{variable}}` placeholders highlighted in indigo in preview (like admin Phase 7)
- **SMS Preview:** Phone mockup to the right of editor, updates on typing
- **Status progress:** Visual indicator: Draft → Pending → Under Review → Approved. Current step highlighted, future steps grey

### 2.4 Sender Names — Enhancements

- **Operator registration matrix:** When creating sender name, show operator matrix: which are free, which are paid (with price), checkboxes for selection. Client sees costs BEFORE registering, not after
- **Registration cost forecast:** "Регистрация на 4 оператора: ₽12,000" before confirmation

### 2.5 Schedules — No Changes

Current functionality sufficient.

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/quick-send` | POST | Send without campaign | **New** |
| `/portal/v1/templates/{id}/preview` | GET | Preview with sample value substitution | **New** |
| `/portal/v1/sender-names/registration-costs` | GET | Registration cost matrix by operator | **New** |

### Acceptance Criteria

- [ ] Quick Send page sends 1-100 messages in single form
- [ ] Real-time cost calculation and SMS preview
- [ ] Campaign step 6 shows cost breakdown pie chart
- [ ] Campaign launch redirects to Command Center
- [ ] Template variables highlighted in preview
- [ ] Sender name creation shows operator cost matrix
- [ ] After quick-send, "Отследить" link works

---

## Phase 3: «Отследить»

### 3.1 Live Feed (new page)

**Route:** `/tracking/live`
**New file:** `portal-frontend/src/pages/tracking/LiveFeedPage.tsx`

Full-screen real-time message stream (expanded version of Command Center's live feed).

**Top panel:** Inline filters — status (multi-select), sender name, operator, provider. Pause/Resume button.

**Main table — streaming rows with smooth animation:**

| Column | Content |
|--------|---------|
| Time | HH:MM:SS.ms |
| Status | Colored badge: delivered (green), sent (blue), failed (red), pending (grey), expired (orange) |
| Recipient | Masked +7(9XX)***-XX-XX |
| Operator | MNO name |
| Provider | Which provider handled it |
| Sender | Sender name |
| Text | First 40 chars, hover → full text |
| Latency | Time from send to DLR (if available) |

**Interactions:**
- Row click → slide-in drawer with full info: full text, StatusTimeline, DLR data, cost, route
- Filters apply to stream in real-time (client-side filtering of WebSocket data)
- Corner counter: "847 msg/sec | 12,430 за последние 5 мин"
- "Экспорт видимого в CSV" button

**Backend:** Same WebSocket `GET /portal/v1/ws/messages` as Command Center.

### 3.2 Detalization — Enhancements

Existing page preserved. Additions:

- **StatusTimeline:** Visual vertical lifecycle (like admin Phase 1). Colored dots connected by line: grey (pending) → blue (queued) → light blue (sent) → green (delivered) / red (failed). Each dot with timestamp
- **SMS Preview:** Phone mockup in detail modal
- **Collapsible filters:** 3 primary inline (dates, status), "Все фильтры" button expands remaining
- **CSV export:** Button in PageHeader

### 3.3 Cascade History — Enhancements

- **Visual cascade chain:** Horizontal diagram: SMS → Viber → WhatsApp with channel icons, arrows between steps, status per step. Green check where delivered, red X where failed, grey — not reached
- **Chain cost:** Total cascade delivery cost with breakdown by channel

### 3.4 Troubleshooting Center (new page)

**Route:** `/tracking/troubleshoot`
**New file:** `portal-frontend/src/pages/tracking/TroubleshootPage.tsx`

**Interface:** Centered search input (like Google): "Введите номер телефона или Message ID". "Диагностика" button.

**Result for phone number:**
```
+7(903)123-45-67
├─ Operator: МТС (MCC: 250, MNC: 01)
├─ Route: МТС → iDigital (SMPP, priority 1)
├─ Provider: iDigital — healthy (98.2%)
├─ Last 5 messages to this number:
│   14:32 — DELIVERED (0.8s latency)
│   13:15 — DELIVERED (1.2s latency)
│   12:01 — FAILED — Absent subscriber
│   11:30 — DELIVERED (0.6s latency)
│   09:44 — DELIVERED (0.9s latency)
├─ Delivery rate to this number: 80% (4/5)
└─ Recommendation: Номер периодически недоступен (absent subscriber)
```

**Result for Message ID:**
```
msg_a1b2c3d4
├─ Status: FAILED
├─ Timeline: pending (14:32:05) → queued (14:32:05) → sent (14:32:06) → FAILED (14:32:08)
├─ Recipient: +7(903)***-45-67 (МТС)
├─ Route: МТС → iDigital
├─ DLR code: 0x45 — Absent subscriber
├─ Sender: MyBrand
├─ Cost: ₽2.40 (tariff "Стандарт", МТС)
├─ Segments: 1
└─ Recommendation: Абонент был не в сети. Рассмотрите retry через cascade.
```

**Automatic recommendations (frontend logic based on data patterns):**
- Frequent failures to number → "Номер может быть неактивен, рассмотрите HLR-проверку"
- DLR timeout → "Провайдер медленно отдаёт DLR, это не ошибка доставки"
- Absent subscriber → "Абонент вне сети, используйте cascade с отложенным retry"

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/ws/messages` | WebSocket | Same as Command Center | **New** (shared) |
| `/portal/v1/troubleshoot/phone/{number}` | GET | Diagnostics by phone: combines HLR, routing, message history | **New** |
| `/portal/v1/troubleshoot/message/{id}` | GET | Diagnostics by message ID: extended message detail with routing info | **New** |

### Acceptance Criteria

- [ ] Live Feed streams messages full-screen with filters and pause
- [ ] Row click opens drawer with StatusTimeline and full details
- [ ] Detalization has StatusTimeline, SMS Preview, collapsible filters, CSV export
- [ ] Cascade History shows visual chain diagram with costs
- [ ] Troubleshooting resolves phone numbers and message IDs
- [ ] Recommendations display based on failure patterns
- [ ] CSV export works on live feed visible data

---

## Phase 4: «Аналитика»

### 4.1 Statistics — Enhancements

Existing page preserved. Additions:

- **Fixed-height table** with sticky header and sticky footer (totals) — like admin Phase 5
- **Custom date range:** "с" / "по" date fields alongside period presets 7d/30d/90d
- **CSV export:** Button in PageHeader
- **Group by sender name:** 4th tab added to existing "По дням / По операторам / По странам"
- **Delivery rate sparkline:** Mini trend chart in each table row for selected period (thin line, no axes)

### 4.2 Comparisons (new page)

**Route:** `/analytics/compare`
**New file:** `portal-frontend/src/pages/analytics/ComparePage.tsx`

**Interface:**
- Two date pickers: "Период A" and "Период B"
- Presets: this vs last week, this vs last month, custom
- Optional filter: by sender name, by operator

**Result — split view table:**

| Metric | Period A | Period B | Delta |
|--------|----------|----------|-------|
| Sent | 124,500 | 98,200 | +26.7% ↑ |
| Delivered | 120,100 | 93,800 | +28.0% ↑ |
| Failed | 4,400 | 4,400 | 0% → |
| Delivery rate | 96.5% | 95.5% | +1.0% ↑ |
| Cost | ₽248,000 | ₽196,400 | +26.3% ↑ |
| Avg latency | 1.1s | 1.4s | -21.4% ↑ |

**Visualizations:**
- Overlay chart: two periods superimposed (different colored lines)
- Breakdown by operator: where growth, where decline
- Anomaly highlighting: if delivery rate dropped >5% for specific operator → red "Внимание" badge

### 4.3 Cost Simulator (new page)

**Route:** `/analytics/cost-simulator`
**New file:** `portal-frontend/src/pages/analytics/CostSimulatorPage.tsx`

**Scenario 1: Tariff switch**
- Left: current tariff with real spend for selected period
- Right: dropdown to select alternative tariff plan
- Result: "На тарифе 'Премиум' ваши расходы составили бы ₽185,000 вместо ₽248,000 — экономия 25%"
- Visual: two columns side by side, difference highlighted green (savings) or red (overspend)

**Scenario 2: Campaign planning**
- Input: message count, operator distribution (auto from history or manual), sender name
- Result: cost forecast, delivery rate prediction, estimated delivery time
- "На текущем тарифе: ₽45,200. На тарифе 'Премиум': ₽38,000"

**Scenario 3: ROI calculator**
- Input: average order value, SMS-to-purchase conversion rate (%)
- Result: "Рассылка на 50,000 стоит ₽100,000. При конверсии 2% и среднем чеке ₽3,000 — ожидаемый доход ₽3,000,000. ROI: 2,900%"

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/analytics/compare` | GET | Two-period comparison with deltas | **New** |
| `/portal/v1/tariffs/simulate` | GET | Recalculate historical spend on different tariff | **New** |
| `/portal/v1/cost-estimate` | POST | Cost forecast with operator breakdown and delivery time | **Extend existing** |

ROI calculator is pure frontend — no backend needed.

### Acceptance Criteria

- [ ] Statistics table has fixed height, sticky header/footer
- [ ] Custom date range works alongside period presets
- [ ] 4th "По sender name" tab works
- [ ] Sparklines render in table rows
- [ ] Comparisons page shows split-view with deltas and overlay chart
- [ ] Anomaly badges highlight significant changes
- [ ] Cost Simulator handles all 3 scenarios
- [ ] ROI calculator computes in real-time
- [ ] CSV export on statistics page

---

## Phase 5: «Контакты»

### 5.1 Contact Lists — Enhancements

- **List statistics:** In list card: total numbers, valid (format-checked), invalid, in opt-out. Mini progress bar: "4,850 / 5,000 валидных (97%)"
- **Import with preview:** Add preview step to wizard: "Найдено 5,000 номеров. 4,850 валидных. 23 дубля. 127 уже в opt-out. Импортировать 4,850?" Table with first 10 rows for verification
- **HLR list check:** "Проверить доступность" button on list page. Triggers bulk HLR lookup, shows progress. Result: "4,200 активных, 650 недоступных". Option to auto-exclude unavailable

### 5.2 Segments — Enhancements

- **Live counter:** `≈ 12,400 контактов` updates on condition change in segment builder
- **Opt-out intersection:** When creating segment: "Из 12,400 контактов 230 в opt-out. Фактический охват: 12,170"

### 5.3 Opt-out — Enhancements

- **Unsubscribe reasons:** If data available, show breakdown: which campaign triggered opt-out, when. Chart: unsubscribe trend by month
- **Export:** CSV download of opt-out list

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/contact-lists/{id}/hlr-check` | POST | Trigger bulk HLR check | **New** (uses existing HLR service) |
| `/portal/v1/contact-lists/{id}/hlr-status` | GET | Check status/result | **New** |
| `/portal/v1/segments/{id}/estimate` | GET | Add opt-out intersection count | **Extend existing** |

### Acceptance Criteria

- [ ] List cards show validation statistics
- [ ] Import wizard has preview step with counts
- [ ] HLR bulk check launches and shows progress/results
- [ ] Segment builder shows live counter with opt-out intersection
- [ ] Opt-out page has trend chart and CSV export

---

## Phase 6: «Финансы»

### 6.1 Billing — Enhancements

- **Burn rate widget:** Next to balance at top: "₽8,200/ч • хватит на 17ч при текущем темпе". Forecast based on 2-hour rolling average
- **Spend chart:** Line chart of daily spend for selected period. Overlay: previous period in grey for comparison
- **Operator breakdown:** Pie chart: "МТС 42%, Мегафон 28%, Билайн 18%, Теле2 12%". Click sector → filter transactions
- **Alert threshold inline:** "Уведомить когда баланс ниже ₽X" field directly on billing page (move from settings)
- **CSV export transactions:** Button in PageHeader

### 6.2 Tariffs — Enhancements

- **Visual plan comparison:** Cards side by side (SaaS pricing page pattern). Current plan with "Ваш тариф" badge. Each card: name, price per SMS by operator, included limits, features
- **Your savings:** On each alternative tariff card: "На этом тарифе вы бы сэкономили ₽12,400/мес" or "Этот тариф дороже на ₽3,200/мес" — calculated from client's real data. Link to Cost Simulator
- **Tariff change history:** When switched, from which to which

### 6.3 Documents (new page)

**Route:** `/finance/documents`
**New file:** `portal-frontend/src/pages/finance/DocumentsPage.tsx`

**Tab "Договоры":**
- Client's contracts list (from contracts API): number, legal entity, start/end date, status
- Badge "Истекает через 14 дней" (yellow) — like admin Phase 10
- Click → details: terms, legal entity, dates. Read-only, no editing
- "Запросить продление" button → creates notification for admin

**Tab "Акты и счета":**
- Monthly invoices list: period, amount, status (generated, sent, paid)
- PDF download
- Period filter

### 6.4 Calculator (new page)

**Route:** `/finance/calculator`
**New file:** `portal-frontend/src/pages/finance/CalculatorPage.tsx`

Quick cost estimation tool (different from Cost Simulator — this is "how much to send X messages", Cost Simulator is "what if I change tariff").

**Single-screen interface:**

| Field | Type | Description |
|-------|------|-------------|
| Message count | Number + slider | 1 — 1,000,000 |
| Operator distribution | Auto/Manual toggle | Auto: based on client's historical proportion. Manual: % sliders per operator |
| Message length | Number input | Characters → auto segment calculation |
| Channel | Select | SMS / Viber / WhatsApp / Cascade |

**Result (updates in real-time):**
```
100,000 messages × 1 segment
├─ МТС (42%):     42,000 × ₽2.40 = ₽100,800
├─ Мегафон (28%): 28,000 × ₽2.20 = ₽61,600
├─ Билайн (18%):  18,000 × ₽2.50 = ₽45,000
├─ Теле2 (12%):   12,000 × ₽2.10 = ₽25,200
├─────────────────────────────────────���─────
│  Total: ₽232,600
│  Your balance: ₽142,350 — short ₽90,250
│  Estimated delivery time: ~8 min at 847 msg/sec
└───────────────────────────────────────────
```

Red badge if balance insufficient + "Пополнить баланс" button.

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/contracts` | GET | Client's contracts list | **New** (filter by client_id from existing contracts API) |
| `/portal/v1/contracts/{id}` | GET | Contract details | **New** |
| `/portal/v1/contracts/{id}/renewal-request` | POST | Renewal request (creates admin notification) | **New** |
| `/portal/v1/invoices` | GET | Invoices/acts list | **New** |
| `/portal/v1/invoices/{id}/pdf` | GET | PDF download | **New** |
| `/portal/v1/cost-estimate` | POST | Extend with operator breakdown and delivery time forecast | **Extend existing** |

### Acceptance Criteria

- [ ] Billing shows burn rate with time forecast
- [ ] Spend chart with previous period overlay
- [ ] Operator breakdown pie chart with transaction filtering
- [ ] Balance threshold configurable inline
- [ ] Tariff comparison cards with savings calculation
- [ ] Documents page shows contracts (read-only) and invoices (PDF download)
- [ ] Renewal request creates admin notification
- [ ] Calculator computes in real-time with operator breakdown
- [ ] Insufficient balance warning with top-up link

---

## Phase 7: «Интеграции»

### 7.1 API Keys — Enhancements

- **Usage statistics per key:** On key detail page: request chart for 7d/30d, breakdown by endpoint (messages:send 85%, messages:read 12%, analytics 3%), last call with IP and timestamp
- **Rate limit indicator:** "Использовано 420 / 1000 req/min" with progress bar. Yellow at >80%
- **Quick-start snippets:** On key page: ready code snippets for curl, Python, Node.js, Go, PHP with API key pre-inserted. One-click copy. Strong wow for developers
- **Integration test:** "Проверить интеграцию" button sends test message via API, shows result inline

### 7.2 Webhooks — Enhancements

- **Delivery log:** On webhook detail page: table of last 50 deliveries. Columns: time, event type, HTTP status, latency, payload (collapsible). Green for 2xx, red for 4xx/5xx
- **Retry statistics:** "3 из 50 потребовали retry, 1 failed после 3 попыток"
- **Live test:** "Отправить тестовый event" button with event type selection (delivered, failed, expired). Result inline: HTTP status, response body, latency

### 7.3 Providers — Enhancements

- **Health badge in list:** Next to each provider: green/yellow/red circle with success rate %, matching Health Map in Command Center
- **Sparkline in list:** Mini success rate chart for 24h in table row
- **Provider detail page:** Instead of wizard only: separate page with metrics — success rate 7d (chart), average latency, messages by day, current connections, last 20 errors log

### 7.4 Routing — Enhancements

- **Visual route map:** In addition to table: flow diagram. Left — operators (МТС, Мегафон...), right — providers (iDigital, SMSC...), between them — route lines. Line thickness proportional to traffic. Color by success rate
- **Drag-and-drop priority:** Like admin Phase 3: drag to change route priority, toast "Маршрут перемещён"
- **Cascade chains:** If client has multichannel configured, show cascade visually: SMS → (if fail) → Viber → (if fail) → WhatsApp. With delivery % per step from historical data

### 7.5 Lookup — Enhancements

- **Result with recommendation:** After HLR check: beyond operator/country/status, add "Рекомендуемый маршрут: МТС → iDigital (98.2% success rate)"
- **Bulk result visualization:** Pie chart by status (active, absent, unknown), breakdown by operator
- **History:** Save lookups with re-run option. "Последняя проверка 2ч назад — обновить?"

### 7.6 SMPP (relocated from Settings)

Moves to Integrations — logically an integration protocol, not a profile setting. Functionality unchanged, only sidebar location changes.

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/api-keys/{id}/usage` | GET | Key usage statistics | **New** |
| `/portal/v1/webhooks/{id}/deliveries` | GET | Webhook delivery log | **New** |
| `/portal/v1/webhooks/{id}/test` | POST | Send test event | **Extend existing** |
| `/portal/v1/providers/{id}/metrics` | GET | Provider metrics (from admin health API) | **New** |
| `/portal/v1/routes/traffic-map` | GET | Data for visual route map | **New** |

### Acceptance Criteria

- [ ] API key detail shows usage stats, rate limit indicator, code snippets
- [ ] Webhook detail shows delivery log with retry stats
- [ ] Live webhook test works with selectable event types
- [ ] Provider list has health badges and sparklines
- [ ] Provider detail page shows 7d metrics and error log
- [ ] Route map renders visual flow diagram
- [ ] Drag-and-drop route reordering works with toast
- [ ] Cascade chains visualized with historical delivery %
- [ ] Lookup shows route recommendation
- [ ] SMPP accessible from Integrations section

---

## Phase 8: «Настройки»

### 8.1 Profile — Enhancements

- **Profile completion widget:** Progress bar: "Профиль заполнен на 70%". Unfilled fields highlighted: "Добавьте телефон", "Включите 2FA"
- **Active sessions:** List of current sessions: browser, IP, geolocation (city), last activity. "Завершить" button per session (except current). "Завершить все кроме текущей" button
- **Login history:** Last 20 logins: date, IP, browser, success/failure. Red highlight on failed attempts

### 8.2 Sub-Accounts — Enhancements

- **Sub-account dashboard:** Click sub-account → inline mini-dashboard: balance, spend today/month, message count, delivery rate. Drawer or inline expansion, no page navigation
- **Limits visualization:** Progress bars: "Дневной лимит: 8,420 / 10,000 (84%)". Yellow at >80%, red at >95%
- **Balance transfer:** Quick action: "Перевести ₽X на суб-аккаунт Y" with confirmation. Add visual to existing functionality

### 8.3 Notifications — Enhancements

- **Telegram channel:** Third toggle column: email / in-app / Telegram. Setup: enter Telegram bot token or connect via bot (@SMSPlatformBot → /connect {api_key})
- **Threshold values:** For alerts like "balance below X" and "delivery rate below Y%": configurable thresholds on notifications page, not just in billing
- **Preview notification:** "Тест" button per channel: sends test notification

### 8.4 Domains — No Changes

### 8.5 Default Senders — No Changes

### 8.6 Audit — Enhancements

- **Visual timeline:** Optional "timeline" view (table/timeline toggle). Events on vertical axis with action-type icons
- **Sub-account filter:** If sub-accounts exist, show whose actions
- **Export:** CSV download for selected period

### Backend Dependencies

| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/sessions` | GET | Active sessions list | **New** |
| `DELETE /portal/v1/sessions/{id}` | DELETE | Terminate session | **New** |
| `DELETE /portal/v1/sessions/bulk` | DELETE | Terminate all except current | **New** |
| `/portal/v1/login-history` | GET | Login history | **New** (or extend audit log with action=login filter) |
| `/portal/v1/notifications/telegram/connect` | POST | Telegram connection | **New** |
| `/portal/v1/notifications/test` | POST | Test notification | **New** |

### Acceptance Criteria

- [ ] Profile completion widget shows progress with action hints
- [ ] Active sessions listed with terminate capability
- [ ] Login history shows success/failure with IP
- [ ] Sub-account inline dashboard with limit progress bars
- [ ] Balance transfer works with confirmation
- [ ] Telegram notification channel configurable
- [ ] Alert thresholds configurable on notifications page
- [ ] Test notification sends to selected channel
- [ ] Audit log has timeline view and sub-account filter
- [ ] CSV export on audit page

---

## Phase 9: Cross-Cutting Improvements

### 9.1 Dark Theme for Command Center

- Command Center only page with dark theme (`bg-slate-900`, `text-slate-200`)
- Smooth theme transition (CSS transition 300ms on `background-color` and `color` on body) when navigating to/from Command Center
- Implementation: CSS class `theme-dark` on Command Center layout wrapper, all components inherit via CSS variables
- All other pages remain light theme

### 9.2 Global Notification Center

- Bell icon in header — on all pages, not just Command Center
- Unread count badge
- Click → dropdown panel: last 10 notifications with type (alert, info, success), time, brief text
- "Все уведомления →" → full page `/notifications` with history, type filters, read/unread marking
- Connected to Smart Alerts from Command Center — same data source

**Backend:**
| Endpoint | Method | Description | Status |
|----------|--------|-------------|--------|
| `/portal/v1/notifications` | GET | Notification list (existing `notifications` table from 019) | **Extend existing** |
| `/portal/v1/notifications/{id}/read` | PATCH | Mark as read | **New** |
| WebSocket notification channel | WS | Push notifications | **New** |

### 9.3 CSV Export (reusable utility)

Shared utility `portal-frontend/src/utils/csvExport.ts`:

```ts
export function exportToCsv(filename: string, headers: string[], rows: string[][]): void
```

BOM marker `\uFEFF` for correct Excel/Cyrillic display. Used on pages: Detalization, Statistics, Billing/Transactions, Audit, Opt-out, Lookup results, Comparisons.

### 9.4 Command Palette Enhancements

Existing `Ctrl+K` works. Additions:

- **Quick actions:** "Отправить SMS", "Создать кампанию", "Проверить номер", "Пополнить баланс"
- **Entity search:** Enter phone number → "Диагностика +7..." (navigate to Troubleshooting), enter message ID → "Сообщение msg_..." (navigate to detalization)
- **Navigation:** All portal pages searchable by keywords

### 9.5 Onboarding for New Clients

First login → tooltip tour (4 steps):
1. "Это ваш Командный центр — здесь вы видите всё в реальном времени"
2. "Отправьте первое SMS →" (leads to Quick Send)
3. "Настройте API-ключ для интеграции →"
4. "Включите 2FA для безопасности →"

Sidebar checklist: 4 steps, checkmarks on completion. Hidden after completion or "Скрыть" button.

### 9.6 Responsive Design

All new pages mobile-friendly:
- Command Center on mobile: single column, KPI cards stacked, simplified live feed
- Tables: horizontal scroll or card-view on small screens
- Navigation: collapsible sidebar (already works)

### 9.7 Accessibility

- All interactive elements keyboard-navigable
- ARIA labels on badges, icons, charts
- Color contrast: never rely on color alone — duplicate with icons (✓, ✗, ⚠, ⏳)
- Focus rings on all interactive elements
- Partially done in 006-ux-a11y-audit, verify on new components

---

## New Backend Endpoints Summary

### WebSocket

| Endpoint | Description |
|----------|-------------|
| `GET /portal/v1/ws/messages` | Real-time message stream for client |
| WebSocket notification channel | Push notifications |

### New REST Endpoints

| Endpoint | Method | Phase |
|----------|--------|-------|
| `/portal/v1/dashboard/metrics` | GET | Phase 1 (extend existing — add burn rate, forecast) |
| `/portal/v1/providers/health` | GET | Phase 1 |
| `/portal/v1/alerts` | GET | Phase 1 |
| `/portal/v1/quick-send` | POST | Phase 2 |
| `/portal/v1/templates/{id}/preview` | GET | Phase 2 |
| `/portal/v1/sender-names/registration-costs` | GET | Phase 2 |
| `/portal/v1/troubleshoot/phone/{number}` | GET | Phase 3 |
| `/portal/v1/troubleshoot/message/{id}` | GET | Phase 3 |
| `/portal/v1/analytics/compare` | GET | Phase 4 |
| `/portal/v1/tariffs/simulate` | GET | Phase 4 |
| `/portal/v1/cost-estimate` | POST | Phase 4+6 (extend existing) |
| `/portal/v1/contact-lists/{id}/hlr-check` | POST | Phase 5 |
| `/portal/v1/contact-lists/{id}/hlr-status` | GET | Phase 5 |
| `/portal/v1/segments/{id}/estimate` | GET | Phase 5 (extend — add opt-out intersection) |
| `/portal/v1/contracts` | GET | Phase 6 |
| `/portal/v1/contracts/{id}` | GET | Phase 6 |
| `/portal/v1/contracts/{id}/renewal-request` | POST | Phase 6 |
| `/portal/v1/invoices` | GET | Phase 6 |
| `/portal/v1/invoices/{id}/pdf` | GET | Phase 6 |
| `/portal/v1/api-keys/{id}/usage` | GET | Phase 7 |
| `/portal/v1/webhooks/{id}/deliveries` | GET | Phase 7 |
| `/portal/v1/webhooks/{id}/test` | POST | Phase 7 (extend existing) |
| `/portal/v1/providers/{id}/metrics` | GET | Phase 7 |
| `/portal/v1/routes/traffic-map` | GET | Phase 7 |
| `/portal/v1/sessions` | GET | Phase 8 |
| `/portal/v1/sessions/{id}` | DELETE | Phase 8 |
| `/portal/v1/sessions/bulk` | DELETE | Phase 8 |
| `/portal/v1/login-history` | GET | Phase 8 |
| `/portal/v1/notifications/telegram/connect` | POST | Phase 8 |
| `/portal/v1/notifications/test` | POST | Phase 8 |
| `/portal/v1/notifications` | GET | Phase 9 (extend existing) |
| `/portal/v1/notifications/{id}/read` | PATCH | Phase 9 |

**Total: 2 WebSocket + 28 REST endpoints (10 new, 5 extended, 13 new CRUD)**

---

## Implementation Order

Phases ordered by **wow-factor impact** and **dependency chain**.

### Sprint 1: Command Center + Core Infrastructure (Phases 1, 9.1-9.3)
- **Phase 1:** Command Center — the hero page, dark theme, live data
- **Phase 9.1:** Dark theme CSS variables and transition
- **Phase 9.2:** Global notification center (bell icon)
- **Phase 9.3:** CSV export utility (used by many later phases)

**Why first:** Command Center IS the wow. Everything else builds on top. Backend: WebSocket endpoint, metrics extension, health API, alerts API.

### Sprint 2: Send + Track (Phases 2, 3)
- **Phase 2:** Quick Send, campaign enhancements, template preview, sender name costs
- **Phase 3:** Live Feed, Troubleshooting, detalization enhancements, cascade visualization

**Why second:** Core user workflow — send and track. Reuses WebSocket from Sprint 1. Backend: quick-send, troubleshooting, template preview endpoints.

### Sprint 3: Analytics + Finance (Phases 4, 6)
- **Phase 4:** Comparisons, Cost Simulator, statistics enhancements
- **Phase 6:** Billing enhancements, Documents, Calculator

**Why third:** Analytical depth and financial transparency. Backend: compare, simulate, contracts, invoices endpoints.

### Sprint 4: Contacts + Integrations (Phases 5, 7)
- **Phase 5:** Contact list HLR check, segment improvements, opt-out enhancements
- **Phase 7:** API usage stats, webhook logs, provider metrics, route map, lookup improvements

**Why fourth:** Power-user features. Backend: HLR bulk, API usage, webhook deliveries, route traffic map endpoints.

### Sprint 5: Settings + Polish (Phases 8, 9.4-9.7)
- **Phase 8:** Profile sessions, sub-account dashboards, Telegram notifications, audit timeline
- **Phase 9.4-9.7:** Command palette, onboarding, responsive, accessibility

**Why last:** Settings rarely drive wow. Polish and accessibility round out the experience. Backend: sessions, login history, Telegram, notification test endpoints.

---

## Notes for Implementation

1. **New navigation structure** should be implemented first (Sprint 1) — regrouping sidebar items is low-risk and sets the stage for all subsequent work.

2. **WebSocket infrastructure** is the critical path for Sprint 1. Build `useWebSocket` hook early — it powers Command Center, Live Feed, and Notification Center.

3. **Reuse admin components** where possible — StatusTimeline, SmsPreview, csvExport patterns from admin Phase 1/11 apply directly.

4. **Dark theme** should use CSS custom properties (variables) so the same components render correctly in both themes. Do not duplicate components.

5. **Backend endpoints** can be stubbed on frontend (mock data) to unblock frontend development. Mark stubs clearly with `// TODO: wire to real API` comments.

6. **Routing is config-only** — the visual route map and drag-and-drop are for manual configuration by the client, not automatic routing decisions. No auto-selection logic.

7. **One phase = one or more commits.** Each commit should be atomic and deployable.

8. **No new frontend dependencies** without explicit approval.
