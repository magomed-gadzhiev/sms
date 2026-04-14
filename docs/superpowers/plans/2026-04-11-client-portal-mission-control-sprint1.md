# Client Portal: Mission Control — Sprint 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Sprint 1 of the Mission Control redesign: new action-oriented navigation, dark-theme Command Center dashboard with live data, WebSocket message feed, provider health map, smart alerts, and full-page notification center.

**Architecture:** New navigation restructuring requires only changes to `UserLayout.tsx`. Dark theme is CSS-variables-only, scoped to Command Center. Backend adds 3 new endpoints (provider health, alerts, WebSocket message stream) plus extends the existing dashboard endpoint. WebSocket uses `gorilla/websocket` (new backend dependency). Frontend uses native `WebSocket` API wrapped in a `useWebSocket` hook with reconnect logic.

**Tech Stack:** TypeScript 5.7, React 19, Tailwind CSS 4.2, Recharts 3.8.1 (sparkline), Go 1.25, gorilla/websocket, pgx/v5, zerolog

---

> **Scope note:** This plan covers Sprint 1 only (Phases 1, 9.1, 9.2, 9.3 from the spec). Phase 9.3 (csvExport) is already implemented at `portal-frontend/src/utils/csvExport.ts` — no work needed. Sprints 2–5 should be separate plans.

---

## File Map

**New frontend files:**
- `portal-frontend/src/hooks/useWebSocket.ts` — WebSocket hook with reconnect, pause/resume, typed messages
- `portal-frontend/src/pages/CommandCenter.tsx` — Dark-theme dashboard (replaces DashboardPage as default)
- `portal-frontend/src/pages/notifications/NotificationsPage.tsx` — Full notifications history page

**Modified frontend files:**
- `portal-frontend/src/components/layout/UserLayout.tsx` — New 8-group action-oriented navigation
- `portal-frontend/src/App.tsx` — Add `/command-center`, `/notifications` routes; redirect `/dashboard` → `/command-center`
- `portal-frontend/src/index.css` — CSS custom properties for dark theme
- `portal-frontend/src/api/client.ts` — Add `dashboardApi.getMetrics()`, `providersApi.getHealth()`, `alertsApi.list()`, `messagesApi.getWebSocketUrl()`

**New backend files:**
- `internal/gateway/portal/handlers/ws_messages.go` — WebSocket handler that streams live messages to client
- `internal/gateway/portal/handlers/health.go` — `GET /portal/v1/providers/health` endpoint
- `internal/gateway/portal/handlers/alerts.go` — `GET /portal/v1/alerts` endpoint

**Modified backend files:**
- `internal/gateway/portal/handlers/dashboard.go` — Extend `GetDashboard` to add `msg_per_sec`, `burn_rate`, `forecast_hours`, `sparkline_1h`
- `internal/gateway/portal/router/router.go` — Register new routes + inject new handler dependencies
- `go.mod` + `go.sum` — Add `github.com/gorilla/websocket`

---

## Task 1: New Navigation Structure

**Files:**
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Replace NAV_GROUPS in UserLayout.tsx**

Open `portal-frontend/src/components/layout/UserLayout.tsx`. Replace `DASHBOARD_NAV` and `NAV_GROUPS` constants (lines 9–63) with:

```tsx
const NAV_ITEMS: NavItem[] = [
  { path: '/command-center', label: 'Командный центр' },
];

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Отправить',
    items: [
      { path: '/campaigns/new', label: 'Быстрая отправка' },
      { path: '/campaigns', label: 'Кампании' },
      { path: '/campaign-schedules', label: 'Расписания' },
      { path: '/templates', label: 'Шаблоны' },
      { path: '/sender-names', label: 'Имена отправителей' },
    ],
  },
  {
    label: 'Отследить',
    items: [
      { path: '/messages', label: 'Сообщения' },
      { path: '/detalization', label: 'Детализация' },
      { path: '/cascade/history', label: 'История каскадов' },
    ],
  },
  {
    label: 'Аналитика',
    items: [
      { path: '/analytics', label: 'Статистика' },
    ],
  },
  {
    label: 'Контакты',
    items: [
      { path: '/contact-lists', label: 'Контактные базы' },
      { path: '/segments', label: 'Сегменты' },
      { path: '/opt-out', label: 'Список отписок' },
    ],
  },
  {
    label: 'Финансы',
    items: [
      { path: '/billing', label: 'Биллинг' },
      { path: '/tariffs', label: 'Тарифы' },
    ],
  },
  {
    label: 'Интеграции',
    items: [
      { path: '/api-keys', label: 'API Ключи' },
      { path: '/webhooks', label: 'Вебхуки' },
      { path: '/providers', label: 'Провайдеры' },
      { path: '/routing', label: 'Маршрутизация' },
      { path: '/lookup', label: 'Lookup' },
      { path: '/settings/smpp', label: 'SMPP' },
    ],
  },
  {
    label: 'Настройки',
    items: [
      { path: '/profile', label: 'Профиль' },
      { path: '/sub-accounts', label: 'Суб-аккаунты' },
      { path: '/settings/domains', label: 'Домены' },
      { path: '/settings/notifications', label: 'Уведомления' },
      { path: '/settings/default-senders', label: 'Имена по умолчанию' },
      { path: '/audit-log', label: 'Журнал аудита' },
    ],
  },
];
```

Then update the `<Sidebar>` call to use `items={NAV_ITEMS}` instead of `items={DASHBOARD_NAV}`.

- [ ] **Step 2: Verify sidebar renders correctly**

Run `npm run dev` from `portal-frontend/`, open browser at `http://localhost:5173`, confirm:
- Single top item "Командный центр"
- 7 collapsible groups in new order
- SMPP moved from Настройки → Интеграции
- Old "Рассылки" group replaced by "Отправить" + "Отследить"

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal): restructure navigation to 8 action-oriented groups"
```

---

## Task 2: Dark Theme CSS Variables

**Files:**
- Modify: `portal-frontend/src/index.css`

- [ ] **Step 1: Add dark theme variables to index.css**

Open `portal-frontend/src/index.css`. Append at the end:

```css
/* ── Dark Theme (Command Center only) ─────────────────────────────── */
.theme-dark {
  --cc-bg:           #0f172a; /* slate-900 */
  --cc-surface:      #1e293b; /* slate-800 */
  --cc-surface-2:    #334155; /* slate-700 */
  --cc-border:       #475569; /* slate-600 */
  --cc-text:         #e2e8f0; /* slate-200 */
  --cc-text-muted:   #94a3b8; /* slate-400 */
  --cc-accent-green: #22c55e; /* green-500 */
  --cc-accent-red:   #ef4444; /* red-500 */
  --cc-accent-yellow:#eab308; /* yellow-500 */
  --cc-accent-blue:  #3b82f6; /* blue-500 */
  color-scheme: dark;
}

/* Smooth transition when navigating to/from Command Center */
body {
  transition: background-color 300ms ease, color 300ms ease;
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/index.css
git commit -m "feat(portal): add CSS variables for Command Center dark theme"
```

---

## Task 3: Frontend API Client — New Endpoints

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add TypeScript types and API functions**

Open `portal-frontend/src/api/client.ts`. At the end of the file, add:

```ts
// ── Command Center Types ─────────────────────────────────────────────

export interface DashboardMetrics {
  balance: string;
  currency: string;
  msg_per_sec: number;
  msg_per_sec_trend_pct: number;      // vs yesterday
  delivery_rate_24h: number;           // 0–100
  delivery_rate_trend_pct: number;     // vs last week
  burn_rate_per_hour: string;          // ₽/hr
  forecast_hours: number;              // balance lasts N hours
  sparkline_1h: number[];              // last 60 data points (1 per min)
  messages_today: number;
  active_campaigns: ActiveCampaign[];
}

export interface ActiveCampaign {
  id: string;
  name: string;
  started_at: string;
  total: number;
  sent: number;
  delivery_rate: number;
  eta_minutes: number;
}

export interface ProviderHealth {
  id: string;
  name: string;
  connection_type: string; // SMPP | HTTP
  connections_active: number;
  connections_total: number;
  success_rate: number;    // 0–100
  msg_per_sec: number;
  is_degraded: boolean;
}

export interface AlertItem {
  id: string;
  type: 'critical' | 'warning' | 'success' | 'info';
  title: string;
  description: string;
  created_at: string;
}

export interface LiveMessageEvent {
  message_id: string;
  timestamp: string;
  status: 'delivered' | 'sent' | 'failed' | 'pending' | 'expired';
  phone_masked: string;
  operator: string;
  provider: string;
  sender: string;
  text_fragment: string;
}

// ── New API functions ────────────────────────────────────────────────

export const commandCenterApi = {
  getMetrics: () =>
    apiFetch<DashboardMetrics>('/dashboard/metrics'),

  getProviderHealth: () =>
    apiFetch<{ providers: ProviderHealth[] }>('/providers/health'),

  getAlerts: () =>
    apiFetch<{ items: AlertItem[] }>('/alerts'),

  /** Returns the WebSocket URL (ws:// or wss://) for the live message stream. */
  getLiveFeedUrl: (): string => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/portal/v1/ws/messages`;
  },
};
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd portal-frontend && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(portal): add Command Center API types and client functions"
```

---

## Task 4: useWebSocket Hook

**Files:**
- Create: `portal-frontend/src/hooks/useWebSocket.ts`

- [ ] **Step 1: Create the hook**

```ts
// portal-frontend/src/hooks/useWebSocket.ts
import { useEffect, useRef, useState, useCallback } from 'react';

export type WsStatus = 'connecting' | 'open' | 'closed' | 'error';

interface UseWebSocketOptions<T> {
  url: string;
  enabled?: boolean;
  /** Max messages to keep in buffer */
  bufferSize?: number;
  onMessage?: (msg: T) => void;
}

interface UseWebSocketResult<T> {
  status: WsStatus;
  messages: T[];
  isPaused: boolean;
  pause: () => void;
  resume: () => void;
  clearBuffer: () => void;
}

const RECONNECT_BASE_MS = 2_000;
const RECONNECT_MAX_MS  = 30_000;

export function useWebSocket<T>({
  url,
  enabled = true,
  bufferSize = 100,
  onMessage,
}: UseWebSocketOptions<T>): UseWebSocketResult<T> {
  const [status, setStatus]   = useState<WsStatus>('closed');
  const [messages, setMessages] = useState<T[]>([]);
  const [isPaused, setIsPaused] = useState(false);

  const wsRef         = useRef<WebSocket | null>(null);
  const pausedRef     = useRef(false);
  const timerRef      = useRef<ReturnType<typeof setTimeout> | null>(null);
  const delayRef      = useRef(RECONNECT_BASE_MS);
  const closedByUs    = useRef(false);
  const onMessageRef  = useRef(onMessage);
  onMessageRef.current = onMessage;

  const clearTimer = () => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  const connect = useCallback(() => {
    if (closedByUs.current) return;
    setStatus('connecting');

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus('open');
      delayRef.current = RECONNECT_BASE_MS;
    };

    ws.onmessage = (e: MessageEvent) => {
      if (pausedRef.current) return;
      try {
        const msg = JSON.parse(e.data as string) as T;
        onMessageRef.current?.(msg);
        setMessages((prev) => {
          const next = [msg, ...prev];
          return next.length > bufferSize ? next.slice(0, bufferSize) : next;
        });
      } catch {
        // ignore malformed
      }
    };

    ws.onerror = () => setStatus('error');

    ws.onclose = () => {
      wsRef.current = null;
      if (closedByUs.current) return;
      setStatus('closed');
      timerRef.current = setTimeout(() => {
        delayRef.current = Math.min(delayRef.current * 2, RECONNECT_MAX_MS);
        connect();
      }, delayRef.current);
    };
  }, [url, bufferSize]);

  useEffect(() => {
    if (!enabled) return;
    closedByUs.current = false;
    connect();
    return () => {
      closedByUs.current = true;
      clearTimer();
      wsRef.current?.close();
    };
  }, [enabled, connect]);

  const pause  = useCallback(() => { pausedRef.current = true;  setIsPaused(true);  }, []);
  const resume = useCallback(() => { pausedRef.current = false; setIsPaused(false); }, []);
  const clearBuffer = useCallback(() => setMessages([]), []);

  return { status, messages, isPaused, pause, resume, clearBuffer };
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd portal-frontend && npx tsc --noEmit
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/hooks/useWebSocket.ts
git commit -m "feat(portal): add useWebSocket hook with reconnect and pause/resume"
```

---

## Task 5: Backend — Add gorilla/websocket Dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add gorilla/websocket**

```bash
cd c:/projects/sms && go get github.com/gorilla/websocket@v1.5.3
```

Expected output: `go: added github.com/gorilla/websocket v1.5.3`

- [ ] **Step 2: Verify build still passes**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add gorilla/websocket v1.5.3 for Command Center live feed"
```

---

## Task 6: Backend — Extend Dashboard Endpoint

**Files:**
- Modify: `internal/gateway/portal/handlers/dashboard.go`

The goal is to add `msg_per_sec`, `burn_rate_per_hour`, `forecast_hours`, `sparkline_1h` to the existing `/portal/v1/dashboard` response. We also expose this data at a new alias path `/portal/v1/dashboard/metrics` (same handler, registered twice in router).

- [ ] **Step 1: Add helper functions at the top of dashboard.go (before DashboardHandlers struct)**

```go
// burnRatePerHour computes hourly balance consumption from last-2h window.
// Returns "0" if not enough data.
func burnRatePerHour(totalSpent2h float64) string {
	rate := totalSpent2h / 2.0
	return fmt.Sprintf("%.2f", rate)
}

// forecastHours returns how many hours the balance lasts at the given hourly burn rate.
// Returns 0 if burn rate is zero or balance is negative.
func forecastHours(balanceStr string, burnRateStr string) int {
	balance, err1 := strconv.ParseFloat(balanceStr, 64)
	burnRate, err2 := strconv.ParseFloat(burnRateStr, 64)
	if err1 != nil || err2 != nil || burnRate <= 0 || balance <= 0 {
		return 0
	}
	return int(balance / burnRate)
}
```

Add the missing imports at the top of `dashboard.go`:
```go
import (
	"fmt"
	"strconv"
	// … existing imports …
)
```

- [ ] **Step 2: Add sparkline + msg_per_sec goroutine inside GetDashboard**

Inside `GetDashboard`, after the existing goroutines (around line 211), add a new goroutine for 1-hour sparkline data and another for msg/sec. Add the variable declarations alongside the existing ones at the top of the function:

```go
var (
	// … existing vars …
	sparkline1h     []int64
	msgPerSec       float64
	burnRatePctOf2h float64 // total spent in last 2h
	deliveryRate24h int32
	deliveryRateTrend int32
)
```

Add this goroutine:

```go
// 7. Sparkline: 1-hour message counts grouped by minute (60 buckets)
wg.Add(1)
go func() {
	defer wg.Done()
	if h.analyticsClient == nil {
		return
	}
	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)

	resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
		ClientId: clientID.String(),
		From:     timestamppb.New(oneHourAgo),
		To:       timestamppb.New(now),
		GroupBy:  "minute",
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения спарклайна дашборда")
		return
	}
	// Build a 60-bucket array (most-recent-last)
	buckets := make(map[string]int64)
	for _, g := range resp.Groups {
		if g.Stats != nil {
			buckets[g.Key] = g.Stats.TotalSent
		}
	}
	sparkline1h = make([]int64, 60)
	for i := 59; i >= 0; i-- {
		t := now.Add(-time.Duration(i+1) * time.Minute)
		key := t.Format("2006-01-02T15:04")
		sparkline1h[59-i] = buckets[key]
	}
	// msg/sec = total sent in last 60s / 60
	if len(sparkline1h) > 0 {
		msgPerSec = float64(sparkline1h[len(sparkline1h)-1]) / 60.0
	}
}()

// 8. 24h delivery rate + trend vs last week
wg.Add(1)
go func() {
	defer wg.Done()
	if h.analyticsClient == nil {
		return
	}
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	resp, err := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
		ClientId: clientID.String(),
		From:     timestamppb.New(yesterday),
		To:       timestamppb.New(now),
		GroupBy:  "day",
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения delivery rate 24h")
		return
	}
	if resp.Totals != nil {
		deliveryRate24h = resp.Totals.SuccessRate
	}
	// Trend: compare with same 24h period 7 days ago
	weekAgo := now.AddDate(0, 0, -7)
	weekAgoEnd := weekAgo.Add(24 * time.Hour)
	resp2, err2 := h.analyticsClient.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
		ClientId: clientID.String(),
		From:     timestamppb.New(weekAgo),
		To:       timestamppb.New(weekAgoEnd),
		GroupBy:  "day",
	})
	if err2 == nil && resp2.Totals != nil {
		deliveryRateTrend = deliveryRate24h - resp2.Totals.SuccessRate
	}
}()
```

- [ ] **Step 3: Extend the response map in GetDashboard**

After `wg.Wait()`, update the `response` map to include the new fields:

```go
burnRate := burnRatePerHour(burnRatePctOf2h)
forecast := forecastHours(balance, burnRate)

if sparkline1h == nil {
	sparkline1h = make([]int64, 60)
}

response := map[string]interface{}{
	// … existing fields unchanged …
	"balance":                  balance,
	"currency":                 currency,
	"messages_today":           messagesToday,
	"messages_delivered_today": messagesDeliveredToday,
	"delivery_rate_today":      deliveryRateToday,
	"active_api_keys":          activeAPIKeys,
	"active_webhooks":          activeWebhooks,
	"profile_completion":       completion,
	// New fields:
	"msg_per_sec":              msgPerSec,
	"msg_per_sec_trend_pct":    0, // TODO: wire to yesterday's avg
	"delivery_rate_24h":        deliveryRate24h,
	"delivery_rate_trend_pct":  deliveryRateTrend,
	"burn_rate_per_hour":       burnRate,
	"forecast_hours":           forecast,
	"sparkline_1h":             sparkline1h,
	"charts": map[string]interface{}{
		"timeline_7d":         chartTimeline,
		"status_distribution": statusDistribution,
		"delivery_rate_trend": trendDelta,
	},
}
```

- [ ] **Step 4: Build and run existing tests**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/dashboard.go
git commit -m "feat(portal/dashboard): add burn_rate, forecast_hours, sparkline_1h, delivery_rate_24h to metrics"
```

---

## Task 7: Backend — Provider Health Endpoint

**Files:**
- Create: `internal/gateway/portal/handlers/health.go`

- [ ] **Step 1: Create health.go**

```go
package handlers

import (
	"net/http"

	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/rs/zerolog/log"
)

// HealthHandlers содержит handlers для проверки здоровья системы
type HealthHandlers struct {
	providerClient cpv1.ClientProviderServiceClient
}

// NewHealthHandlers создаёт HealthHandlers
func NewHealthHandlers(providerClient cpv1.ClientProviderServiceClient) *HealthHandlers {
	return &HealthHandlers{providerClient: providerClient}
}

// GetProviderHealth обрабатывает GET /portal/v1/providers/health
// Возвращает список провайдеров клиента с их health-метриками.
func (h *HealthHandlers) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if h.providerClient == nil {
		// Return empty list when service unavailable
		respondJSON(w, http.StatusOK, map[string]interface{}{"providers": []interface{}{}})
		return
	}

	resp, err := h.providerClient.ListClientProviders(r.Context(), &cpv1.ListClientProvidersRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения провайдеров клиента")
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	type providerHealth struct {
		ID                 string  `json:"id"`
		Name               string  `json:"name"`
		ConnectionType     string  `json:"connection_type"`
		ConnectionsActive  int32   `json:"connections_active"`
		ConnectionsTotal   int32   `json:"connections_total"`
		SuccessRate        float32 `json:"success_rate"`
		MsgPerSec          float32 `json:"msg_per_sec"`
		IsDegraded         bool    `json:"is_degraded"`
	}

	providers := make([]providerHealth, 0, len(resp.Providers))
	for _, p := range resp.Providers {
		ph := providerHealth{
			ID:             p.ProviderId,
			Name:           p.ProviderName,
			ConnectionType: p.ConnectionType,
			SuccessRate:    p.SuccessRate,
			MsgPerSec:      p.MsgPerSec,
			IsDegraded:     p.SuccessRate < 90.0,
		}
		providers = append(providers, ph)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/gateway/portal/handlers/...
```

Expected: 0 errors. If `ListClientProviders` or `SuccessRate` fields don't exist on the proto, replace with available field names from the proto. Add a `// TODO: wire to real metrics` comment and use mock values (0.0) as fallback.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/health.go
git commit -m "feat(portal): add GET /providers/health endpoint for Command Center health map"
```

---

## Task 8: Backend — Smart Alerts Endpoint

**Files:**
- Create: `internal/gateway/portal/handlers/alerts.go`

Alerts are derived from: low balance (billing), provider health (from health endpoint data), and recent notifications. This handler assembles them from available sources without additional DB queries beyond what already exists.

- [ ] **Step 1: Create alerts.go**

```go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AlertsHandlers содержит handlers для умных оповещений Command Center
type AlertsHandlers struct {
	billingClient  billingv1.BillingServiceClient
	providerClient cpv1.ClientProviderServiceClient
	pool           *pgxpool.Pool
}

// NewAlertsHandlers создаёт AlertsHandlers
func NewAlertsHandlers(
	billingClient billingv1.BillingServiceClient,
	providerClient cpv1.ClientProviderServiceClient,
	pool *pgxpool.Pool,
) *AlertsHandlers {
	return &AlertsHandlers{
		billingClient:  billingClient,
		providerClient: providerClient,
		pool:           pool,
	}
}

type alertItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // critical | warning | success | info
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// GetAlerts обрабатывает GET /portal/v1/alerts
// Возвращает до 5 актуальных оповещений: проблемы провайдеров, низкий баланс,
// последние системные уведомления.
func (h *AlertsHandlers) GetAlerts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	ctx := r.Context()
	now := time.Now().UTC().Format(time.RFC3339)
	alerts := make([]alertItem, 0, 5)

	// 1. Check provider health for degraded providers
	if h.providerClient != nil {
		resp, err := h.providerClient.ListClientProviders(ctx, &cpv1.ListClientProvidersRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Msg("alerts: не удалось получить провайдеры")
		} else {
			for _, p := range resp.Providers {
				if p.SuccessRate < 90.0 {
					alerts = append(alerts, alertItem{
						ID:          "provider-" + p.ProviderId,
						Type:        "critical",
						Title:       "Провайдер деградирует",
						Description: fmt.Sprintf("%s: success rate %.1f%%", p.ProviderName, p.SuccessRate),
						CreatedAt:   now,
					})
				}
			}
		}
	}

	// 2. Check balance: if forecast < 24h → warning
	if h.billingClient != nil {
		resp, err := h.billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{
			ClientId: clientID.String(),
		})
		if err != nil {
			log.Warn().Err(err).Msg("alerts: не удалось получить баланс")
		} else {
			balance, _ := strconv.ParseFloat(resp.Balance, 64)
			if balance < 1000 {
				alerts = append(alerts, alertItem{
					ID:          "low-balance",
					Type:        "warning",
					Title:       "Низкий баланс",
					Description: fmt.Sprintf("Осталось %.2f %s", balance, resp.Currency),
					CreatedAt:   now,
				})
			}
		}
	}

	// 3. Recent unread notifications from DB (up to 3)
	if h.pool != nil {
		userID, ok := middleware.GetUserID(ctx)
		if ok {
			rows, err := h.pool.Query(ctx,
				`SELECT id, type, body, created_at FROM notifications
				 WHERE user_id = $1 AND is_read = false
				 ORDER BY created_at DESC LIMIT 3`,
				userID,
			)
			if err != nil {
				log.Warn().Err(err).Msg("alerts: не удалось получить уведомления")
			} else {
				defer rows.Close()
				for rows.Next() {
					var id, typ, body string
					var createdAt interface{}
					if err := rows.Scan(&id, &typ, &body, &createdAt); err != nil {
						continue
					}
					ts := now
					if t, ok2 := createdAt.(interface{ Format(string) string }); ok2 {
						ts = t.Format(time.RFC3339)
					}
					aType := "info"
					if typ == "warning" || typ == "low_balance" {
						aType = "warning"
					} else if typ == "critical" || typ == "provider_degraded" {
						aType = "critical"
					}
					alerts = append(alerts, alertItem{
						ID:          "notif-" + id,
						Type:        aType,
						Title:       typ,
						Description: body,
						CreatedAt:   ts,
					})
				}
			}
		}
	}

	// Cap at 5
	if len(alerts) > 5 {
		alerts = alerts[:5]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"items": alerts})
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/gateway/portal/handlers/...
```

Expected: 0 errors. If proto field names differ, adjust accordingly.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/alerts.go
git commit -m "feat(portal): add GET /alerts endpoint for Command Center smart alerts"
```

---

## Task 9: Backend — WebSocket Message Stream

**Files:**
- Create: `internal/gateway/portal/handlers/ws_messages.go`

The WebSocket endpoint streams live message events to the client. It subscribes to the existing SSE hub (which reads from Kafka `sms.status` topic) and wraps events as WebSocket messages. For new message creation events (not just status updates), we additionally poll the DB every 2s for messages created in the last minute.

- [ ] **Step 1: Create ws_messages.go**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/sse"
)

var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin: func(r *http.Request) bool {
		// Same-origin check: allow only requests from the portal frontend origin
		origin := r.Header.Get("Origin")
		host := r.Host
		return origin == "" || origin == "https://"+host || origin == "http://"+host
	},
}

// WsMessagesHandlers handles WebSocket connections for the live message feed
type WsMessagesHandlers struct {
	hub    *sse.Hub
	dbPool *pgxpool.Pool
}

// NewWsMessagesHandlers creates WsMessagesHandlers
func NewWsMessagesHandlers(hub *sse.Hub, dbPool *pgxpool.Pool) *WsMessagesHandlers {
	return &WsMessagesHandlers{hub: hub, dbPool: dbPool}
}

type wsMessageEvent struct {
	MessageID    string `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Status       string `json:"status"`
	PhoneMasked  string `json:"phone_masked"`
	Operator     string `json:"operator"`
	Provider     string `json:"provider"`
	Sender       string `json:"sender"`
	TextFragment string `json:"text_fragment"`
}

// StreamMessages handles GET /portal/v1/ws/messages (WebSocket upgrade)
func (h *WsMessagesHandlers) StreamMessages(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn().Err(err).Msg("ws_messages: upgrade failed")
		return
	}
	defer conn.Close()

	// Subscribe to SSE hub for status updates
	var statusCh chan sse.Event
	if h.hub != nil {
		statusCh = h.hub.Subscribe(clientID.String())
		defer h.hub.Unsubscribe(clientID.String(), statusCh)
	}

	// Poll DB for recent messages every 2s (for new message events)
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()

	// Ping ticker to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Track last poll time to avoid duplicates
	lastPoll := time.Now().Add(-5 * time.Second)

	send := func(evt wsMessageEvent) bool {
		data, _ := json.Marshal(evt)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Debug().Err(err).Msg("ws_messages: write failed, closing")
			return false
		}
		return true
	}

	// Read loop (to detect client disconnect)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return

		case evt, ok := <-statusCh:
			if !ok {
				return
			}
			wsEvt := wsMessageEvent{
				MessageID: evt.MessageID,
				Timestamp: evt.UpdatedAt.Format(time.RFC3339),
				Status:    evt.Status,
			}
			if !send(wsEvt) {
				return
			}

		case <-pollTicker.C:
			if h.dbPool == nil {
				continue
			}
			since := lastPoll
			lastPoll = time.Now()

			rows, err := h.dbPool.Query(r.Context(),
				`SELECT m.id, m.created_at, m.status, m.recipient,
				        COALESCE(o.name, ''), COALESCE(p.name, ''),
				        COALESCE(sn.name, ''), LEFT(m.body, 40)
				 FROM messages m
				 LEFT JOIN operators o ON o.mcc = m.mcc AND o.mnc = m.mnc
				 LEFT JOIN providers p ON p.id = m.provider_id
				 LEFT JOIN sender_names sn ON sn.id = m.sender_name_id
				 WHERE m.client_id = $1
				   AND m.created_at > $2
				 ORDER BY m.created_at DESC
				 LIMIT 20`,
				clientID, since,
			)
			if err != nil {
				log.Warn().Err(err).Msg("ws_messages: db poll failed")
				continue
			}
			func() {
				defer rows.Close()
				for rows.Next() {
					var (
						id, status, recipient, operator, provider, sender, text string
						createdAt time.Time
					)
					if err := rows.Scan(&id, &createdAt, &status, &recipient,
						&operator, &provider, &sender, &text); err != nil {
						continue
					}
					wsEvt := wsMessageEvent{
						MessageID:    id,
						Timestamp:    createdAt.Format(time.RFC3339),
						Status:       status,
						PhoneMasked:  maskPhone(recipient),
						Operator:     operator,
						Provider:     provider,
						Sender:       sender,
						TextFragment: text,
					}
					if !send(wsEvt) {
						return
					}
				}
			}()

		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// maskPhone masks a phone number: +7(9XX)***-XX-XX
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	runes := []rune(phone)
	// Mask digits 4–9 (0-indexed) with *
	for i := 4; i < len(runes) && i < 10; i++ {
		if runes[i] >= '0' && runes[i] <= '9' {
			runes[i] = '*'
		}
	}
	return string(runes)
}
```

- [ ] **Step 2: Add Unsubscribe to SSE Hub**

Open `internal/gateway/portal/sse/hub.go`. Check if `Unsubscribe` method exists. If not, add it after the `Subscribe` method:

```go
// Unsubscribe removes a subscriber channel for the given clientID.
func (h *Hub) Unsubscribe(clientID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subs[clientID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.subs, clientID)
		}
	}
	close(ch)
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/gateway/portal/...
```

Expected: 0 errors. If the `messages` table columns differ (e.g., `body` vs `text`, `mcc`/`mnc` not available), simplify the query — use `''` literals for operator/provider and log a TODO comment.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/ws_messages.go internal/gateway/portal/sse/hub.go
git commit -m "feat(portal): add WebSocket /ws/messages endpoint for Command Center live feed"
```

---

## Task 10: Backend — Register New Routes

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add new handler parameters to SetupRouter signature**

In `router.go`, find the `SetupRouter` function signature and add:

```go
func SetupRouter(
	// … all existing params …
	detalizationHandlers *handlers.DetalizationHandlers,  // already exists
	cascadeHandlers *handlers.CascadeHandlers,            // already exists if present
	notificationSettingsHandlers *handlers.NotificationSettingsHandlers,
	// NEW:
	healthHandlers *handlers.HealthHandlers,
	alertsHandlers *handlers.AlertsHandlers,
	wsMessagesHandlers *handlers.WsMessagesHandlers,
) *mux.Router {
```

- [ ] **Step 2: Register new routes in the protected section**

Find the existing dashboard route registration and add below it:

```go
// Dashboard metrics alias for Command Center
protected.HandleFunc("/dashboard/metrics", dashboardHandlers.GetDashboard).Methods("GET")

// Provider health for Command Center health map
protected.HandleFunc("/providers/health", healthHandlers.GetProviderHealth).Methods("GET")

// Smart alerts for Command Center
protected.HandleFunc("/alerts", alertsHandlers.GetAlerts).Methods("GET")
```

Find the WebSocket section (after all protected routes, before return) and add:

```go
// WebSocket: live message stream for Command Center and Live Feed
// Note: WebSocket upgrade bypasses CSRF — session auth only
wsProtected := portalV1.PathPrefix("").Subrouter()
wsProtected.Use(sessionAuthMiddleware)
wsProtected.HandleFunc("/ws/messages", wsMessagesHandlers.StreamMessages).Methods("GET")
```

- [ ] **Step 3: Wire up new handlers in the portal gateway startup**

Find where `SetupRouter` is called (look in `cmd/portal-gateway/main.go` or similar entry point). Add construction of new handlers:

```go
healthHandlers := handlers.NewHealthHandlers(clients.ProviderClient)
alertsHandlers := handlers.NewAlertsHandlers(clients.BillingClient, clients.ProviderClient, dbPool)
wsMessagesHandlers := handlers.NewWsMessagesHandlers(sseHub, dbPool)
```

Then pass them to `SetupRouter(...)`.

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: 0 errors. Fix any compilation errors from the new parameters.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): register /dashboard/metrics, /providers/health, /alerts, /ws/messages routes"
```

---

## Task 11: Command Center Page

**Files:**
- Create: `portal-frontend/src/pages/CommandCenter.tsx`

This is the hero page. It uses `theme-dark` class for dark styling, polls or uses WebSocket for live data.

- [ ] **Step 1: Create CommandCenter.tsx**

```tsx
// portal-frontend/src/pages/CommandCenter.tsx
import { useEffect, useState, useRef, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  LineChart, Line, ResponsiveContainer, Tooltip,
} from 'recharts';
import {
  commandCenterApi,
  type DashboardMetrics,
  type ProviderHealth,
  type AlertItem,
  type LiveMessageEvent,
} from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { usePolling } from '../hooks/usePolling';
import { useNotifications } from '../hooks/useNotifications';
import { useAuth } from '../contexts/AuthContext';

// ── KPI Card ──────────────────────────────────────────────────────────

function KpiCard({
  title,
  value,
  subtitle,
  trend,
  children,
}: {
  title: string;
  value: string | number;
  subtitle?: string;
  trend?: number; // positive = up, negative = down
  children?: React.ReactNode;
}) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <span style={{ color: 'var(--cc-text-muted)' }} className="text-xs font-medium uppercase tracking-wider">
          {title}
        </span>
        {trend !== undefined && trend !== 0 && (
          <span
            className="text-xs font-semibold flex items-center gap-0.5"
            style={{ color: trend > 0 ? 'var(--cc-accent-green)' : 'var(--cc-accent-red)' }}
          >
            {trend > 0 ? '↑' : '↓'} {Math.abs(trend)}%
          </span>
        )}
      </div>
      <div style={{ color: 'var(--cc-text)' }} className="text-3xl font-bold tabular-nums">
        {value}
      </div>
      {subtitle && (
        <div style={{ color: 'var(--cc-text-muted)' }} className="text-xs">{subtitle}</div>
      )}
      {children}
    </div>
  );
}

// ── Sparkline ─────────────────────────────────────────────────────────

function Sparkline({ data }: { data: number[] }) {
  const chartData = data.map((v, i) => ({ i, v }));
  return (
    <ResponsiveContainer width="100%" height={40}>
      <LineChart data={chartData} margin={{ top: 4, bottom: 4, left: 0, right: 0 }}>
        <Line
          type="monotone"
          dataKey="v"
          stroke="var(--cc-accent-blue)"
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
        <Tooltip
          contentStyle={{ background: 'var(--cc-surface-2)', border: 'none', borderRadius: 4, fontSize: 11 }}
          labelFormatter={() => ''}
          formatter={(v: number) => [v, 'msg']}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}

// ── Progress Bar ──────────────────────────────────────────────────────

function ProgressBar({ value, max, color }: { value: number; max: number; color: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div style={{ background: 'var(--cc-surface-2)' }} className="w-full h-1.5 rounded-full overflow-hidden">
      <div style={{ width: `${pct}%`, background: color }} className="h-full rounded-full transition-all duration-500" />
    </div>
  );
}

// ── Status Badge (dark-theme) ─────────────────────────────────────────

function StatusBadgeDark({ status }: { status: string }) {
  const map: Record<string, { bg: string; text: string }> = {
    delivered: { bg: '#14532d', text: 'var(--cc-accent-green)' },
    sent:      { bg: '#1e3a5f', text: 'var(--cc-accent-blue)' },
    failed:    { bg: '#450a0a', text: 'var(--cc-accent-red)' },
    pending:   { bg: '#334155', text: 'var(--cc-text-muted)' },
    expired:   { bg: '#431407', text: '#f97316' },
  };
  const s = map[status] ?? map.pending;
  return (
    <span
      className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase"
      style={{ background: s.bg, color: s.text }}
    >
      {status}
    </span>
  );
}

// ── Health Map ────────────────────────────────────────────────────────

function HealthMap({ providers }: { providers: ProviderHealth[] }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">
        Провайдеры
      </h3>
      <div className="flex flex-col gap-2">
        {providers.length === 0 && (
          <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет провайдеров</p>
        )}
        {providers.map((p) => (
          <div
            key={p.id}
            style={{
              background: 'var(--cc-surface-2)',
              border: `1px solid ${p.is_degraded ? 'var(--cc-accent-red)' : 'var(--cc-border)'}`,
            }}
            className="rounded-lg p-2.5"
          >
            <div className="flex items-center justify-between mb-1">
              <span style={{ color: 'var(--cc-text)' }} className="text-xs font-medium">{p.name}</span>
              <span
                className="text-xs font-bold tabular-nums"
                style={{ color: p.success_rate >= 95 ? 'var(--cc-accent-green)' : p.success_rate >= 85 ? 'var(--cc-accent-yellow)' : 'var(--cc-accent-red)' }}
              >
                {p.success_rate.toFixed(1)}%
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
                {p.connection_type} · {p.msg_per_sec.toFixed(0)} msg/s
              </span>
              {p.is_degraded && (
                <span style={{ color: 'var(--cc-accent-red)' }} className="text-[10px] font-semibold">
                  ⚠ деградация
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Smart Alerts ──────────────────────────────────────────────────────

const ALERT_COLORS: Record<string, string> = {
  critical: 'var(--cc-accent-red)',
  warning:  'var(--cc-accent-yellow)',
  success:  'var(--cc-accent-green)',
  info:     'var(--cc-accent-blue)',
};

function SmartAlerts({ alerts }: { alerts: AlertItem[] }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Оповещения</h3>
        <Link to="/notifications" style={{ color: 'var(--cc-accent-blue)' }} className="text-xs hover:underline">
          Все →
        </Link>
      </div>
      <div className="flex flex-col gap-2">
        {alerts.length === 0 && (
          <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">✓ Нет активных оповещений</p>
        )}
        {alerts.map((a) => (
          <div
            key={a.id}
            className="flex gap-2 rounded-lg p-2"
            style={{ background: 'var(--cc-surface-2)', borderLeft: `3px solid ${ALERT_COLORS[a.type] ?? 'var(--cc-accent-blue)'}` }}
          >
            <div className="flex-1 min-w-0">
              <div style={{ color: 'var(--cc-text)' }} className="text-xs font-medium truncate">{a.title}</div>
              <div style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] truncate">{a.description}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Active Campaigns ──────────────────────────────────────────────────

function ActiveCampaigns({ campaigns }: { campaigns: DashboardMetrics['active_campaigns'] }) {
  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Активные кампании</h3>
        <Link to="/campaigns/new" style={{ color: 'var(--cc-accent-blue)' }} className="text-xs hover:underline">
          + Новая →
        </Link>
      </div>
      {campaigns.length === 0 && (
        <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs">Нет активных кампаний</p>
      )}
      {campaigns.map((c) => (
        <div key={c.id}>
          <div className="flex items-center justify-between mb-1">
            <Link
              to={`/campaigns/${c.id}`}
              style={{ color: 'var(--cc-text)' }}
              className="text-xs font-medium hover:underline truncate max-w-[60%]"
            >
              {c.name}
            </Link>
            <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] tabular-nums">
              {c.sent.toLocaleString()} / {c.total.toLocaleString()}
            </span>
          </div>
          <ProgressBar
            value={c.sent}
            max={c.total}
            color={c.delivery_rate >= 95 ? 'var(--cc-accent-green)' : c.delivery_rate >= 85 ? 'var(--cc-accent-yellow)' : 'var(--cc-accent-red)'}
          />
          <div className="flex items-center justify-between mt-0.5">
            <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
              {c.delivery_rate.toFixed(1)}% доставлено
            </span>
            {c.eta_minutes > 0 && (
              <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px]">
                ~{c.eta_minutes} мин
              </span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Live Feed ─────────────────────────────────────────────────────────

function LiveFeed({
  messages,
  msgPerSec,
  isPaused,
  onPause,
  onResume,
  status,
}: {
  messages: LiveMessageEvent[];
  msgPerSec: number;
  isPaused: boolean;
  onPause: () => void;
  onResume: () => void;
  status: string;
}) {
  const [count5min, setCount5min] = useState(0);
  const countRef = useRef(0);

  useEffect(() => {
    countRef.current += messages.length;
    const interval = setInterval(() => {
      setCount5min(countRef.current);
      countRef.current = 0;
    }, 300_000);
    return () => clearInterval(interval);
  }, [messages.length]);

  return (
    <div
      style={{ background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
      className="rounded-xl p-4 flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 style={{ color: 'var(--cc-text)' }} className="text-sm font-semibold">Live Feed</h3>
          <span
            className="w-2 h-2 rounded-full"
            style={{ background: status === 'open' ? 'var(--cc-accent-green)' : 'var(--cc-accent-red)' }}
            title={status}
          />
        </div>
        <div className="flex items-center gap-3">
          <span style={{ color: 'var(--cc-text-muted)' }} className="text-[10px] tabular-nums">
            {msgPerSec.toFixed(0)} msg/s · {count5min.toLocaleString()} за 5 мин
          </span>
          <button
            onClick={isPaused ? onResume : onPause}
            style={{
              background: 'var(--cc-surface-2)',
              color: 'var(--cc-text)',
              border: '1px solid var(--cc-border)',
            }}
            className="text-[10px] px-2 py-0.5 rounded hover:opacity-80 transition-opacity"
          >
            {isPaused ? '▶ Resumе' : '⏸ Pause'}
          </button>
        </div>
      </div>

      <div className="overflow-hidden" style={{ maxHeight: 260 }}>
        <table className="w-full text-[11px]">
          <tbody>
            {messages.slice(0, 20).map((m) => (
              <tr
                key={m.message_id + m.timestamp}
                style={{ borderBottom: '1px solid var(--cc-border)' }}
                className="hover:opacity-80 cursor-pointer"
              >
                <td className="py-1 pr-2 tabular-nums" style={{ color: 'var(--cc-text-muted)', whiteSpace: 'nowrap' }}>
                  {new Date(m.timestamp).toLocaleTimeString('ru', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                </td>
                <td className="py-1 pr-2">
                  <StatusBadgeDark status={m.status} />
                </td>
                <td className="py-1 pr-2 tabular-nums" style={{ color: 'var(--cc-text)' }}>
                  {m.phone_masked || '—'}
                </td>
                <td className="py-1 pr-2" style={{ color: 'var(--cc-text-muted)' }}>
                  {m.operator ? `${m.operator} → ${m.provider}` : m.provider}
                </td>
                <td className="py-1 truncate max-w-[120px]" style={{ color: 'var(--cc-text-muted)' }}>
                  {m.text_fragment}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {messages.length === 0 && (
          <p style={{ color: 'var(--cc-text-muted)' }} className="text-xs text-center py-4">
            {status === 'open' ? 'Ожидание сообщений...' : 'Подключение...'}
          </p>
        )}
      </div>
    </div>
  );
}

// ── Command Center (main component) ──────────────────────────────────

export function CommandCenter() {
  const { isAuthenticated, user } = useAuth();
  const { unreadCount } = useNotifications(isAuthenticated);

  const [metrics, setMetrics]   = useState<DashboardMetrics | null>(null);
  const [providers, setProviders] = useState<ProviderHealth[]>([]);
  const [alerts, setAlerts]     = useState<AlertItem[]>([]);

  const fetchMetrics   = useCallback(() => commandCenterApi.getMetrics().then(setMetrics).catch(() => {}), []);
  const fetchProviders = useCallback(() => commandCenterApi.getProviderHealth().then(r => setProviders(r.providers)).catch(() => {}), []);
  const fetchAlerts    = useCallback(() => commandCenterApi.getAlerts().then(r => setAlerts(r.items)).catch(() => {}), []);

  usePolling(fetchMetrics,   15_000);
  usePolling(fetchProviders, 30_000);
  usePolling(fetchAlerts,    30_000);

  const wsUrl = commandCenterApi.getLiveFeedUrl();
  const {
    status: wsStatus,
    messages: liveMessages,
    isPaused,
    pause,
    resume,
  } = useWebSocket<LiveMessageEvent>({ url: wsUrl, enabled: isAuthenticated, bufferSize: 100 });

  const degradedCount = providers.filter(p => p.is_degraded).length;
  const systemStatus = degradedCount === 0
    ? '● Все системы в норме'
    : `⚠ ${degradedCount} провайдер${degradedCount === 1 ? '' : 'а'} деградирует`;

  const balanceForecast = metrics
    ? `₽${metrics.burn_rate_per_hour}/ч · хватит на ${metrics.forecast_hours}ч`
    : '—';

  return (
    <div
      className="theme-dark min-h-screen p-4 md:p-6"
      style={{ background: 'var(--cc-bg)' }}
    >
      {/* Top bar */}
      <div className="flex items-center justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 style={{ color: 'var(--cc-text)' }} className="text-xl font-bold">
            Командный центр
          </h1>
          <div className="flex items-center gap-2 mt-1">
            <span
              className="text-xs"
              style={{ color: degradedCount === 0 ? 'var(--cc-accent-green)' : 'var(--cc-accent-yellow)' }}
            >
              {systemStatus}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-4">
          {metrics && (
            <Link
              to="/billing"
              style={{ color: 'var(--cc-text)', background: 'var(--cc-surface)', border: '1px solid var(--cc-border)' }}
              className="text-sm px-3 py-1.5 rounded-lg hover:opacity-80 transition-opacity"
            >
              {metrics.balance} {metrics.currency}
            </Link>
          )}
          <Link
            to="/notifications"
            className="relative p-2 rounded-full hover:opacity-80 transition-opacity"
            style={{ color: 'var(--cc-text-muted)' }}
            aria-label={`Уведомления${unreadCount > 0 ? `, ${unreadCount} непрочитанных` : ''}`}
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
            </svg>
            {unreadCount > 0 && (
              <span
                className="absolute top-0.5 right-0.5 min-w-[16px] h-4 bg-red-500 text-white text-[9px] font-bold rounded-full flex items-center justify-center px-1"
                aria-hidden="true"
              >
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </Link>
        </div>
      </div>

      {/* Row 1: KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
        <KpiCard
          title="Скорость"
          value={metrics ? `${metrics.msg_per_sec.toFixed(0)} msg/s` : '—'}
          trend={metrics?.msg_per_sec_trend_pct}
          subtitle="за последний час"
        >
          {metrics && <Sparkline data={metrics.sparkline_1h} />}
        </KpiCard>

        <KpiCard
          title="Доставлено (24ч)"
          value={metrics ? `${metrics.delivery_rate_24h}%` : '—'}
          trend={metrics?.delivery_rate_trend_pct}
        >
          {metrics && (
            <ProgressBar
              value={metrics.delivery_rate_24h}
              max={100}
              color={
                metrics.delivery_rate_24h >= 95
                  ? 'var(--cc-accent-green)'
                  : metrics.delivery_rate_24h >= 85
                  ? 'var(--cc-accent-yellow)'
                  : 'var(--cc-accent-red)'
              }
            />
          )}
        </KpiCard>

        <KpiCard
          title="Баланс"
          value={metrics ? `${metrics.balance} ${metrics.currency}` : '—'}
          subtitle={balanceForecast}
        >
          {metrics && (
            <ProgressBar
              value={Math.min(metrics.forecast_hours, 72)}
              max={72}
              color={
                metrics.forecast_hours >= 48
                  ? 'var(--cc-accent-green)'
                  : metrics.forecast_hours >= 12
                  ? 'var(--cc-accent-yellow)'
                  : 'var(--cc-accent-red)'
              }
            />
          )}
        </KpiCard>
      </div>

      {/* Row 2: Live Feed (2/3) + Health Map (1/3) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-4">
        <div className="lg:col-span-2">
          <LiveFeed
            messages={liveMessages}
            msgPerSec={metrics?.msg_per_sec ?? 0}
            isPaused={isPaused}
            onPause={pause}
            onResume={resume}
            status={wsStatus}
          />
        </div>
        <HealthMap providers={providers} />
      </div>

      {/* Row 3: Smart Alerts (1/3) + Active Campaigns (2/3) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <SmartAlerts alerts={alerts} />
        <div className="lg:col-span-2">
          <ActiveCampaigns campaigns={metrics?.active_campaigns ?? []} />
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd portal-frontend && npx tsc --noEmit
```

Expected: 0 errors. Fix any type mismatches.

- [ ] **Step 3: Visual check in browser**

Navigate to `http://localhost:5173/command-center` (after App.tsx update in next task). Verify:
- Dark background (`#0f172a`)
- 3 KPI cards in row 1
- Live feed table with status badges
- Health map on right
- Alerts and campaigns in row 3

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/CommandCenter.tsx
git commit -m "feat(portal): add Command Center dark-theme dashboard with KPI, live feed, health map"
```

---

## Task 12: Notifications Full Page

**Files:**
- Create: `portal-frontend/src/pages/notifications/NotificationsPage.tsx`

- [ ] **Step 1: Create the page**

```tsx
// portal-frontend/src/pages/notifications/NotificationsPage.tsx
import { useAuth } from '../../contexts/AuthContext';
import { useNotifications } from '../../hooks/useNotifications';
import { PageHeader } from '../../components/layout/PageHeader';
import { Badge } from '../../components/ui/Badge';

const TYPE_LABELS: Record<string, string> = {
  info: 'Инфо',
  warning: 'Предупреждение',
  critical: 'Критическое',
  success: 'Успех',
  low_balance: 'Низкий баланс',
  provider_degraded: 'Проблемы провайдера',
  campaign_completed: 'Кампания завершена',
  template_approved: 'Шаблон одобрен',
  template_rejected: 'Шаблон отклонён',
};

const TYPE_VARIANTS: Record<string, 'default' | 'info' | 'success' | 'warning' | 'danger'> = {
  info: 'info',
  warning: 'warning',
  critical: 'danger',
  success: 'success',
  low_balance: 'warning',
  provider_degraded: 'danger',
  campaign_completed: 'success',
  template_approved: 'success',
  template_rejected: 'danger',
};

export function NotificationsPage() {
  const { isAuthenticated } = useAuth();
  const { items, unreadCount, markRead, markAllRead } = useNotifications(isAuthenticated);

  return (
    <div>
      <PageHeader
        title="Уведомления"
        subtitle={unreadCount > 0 ? `${unreadCount} непрочитанных` : 'Все прочитаны'}
        actions={
          unreadCount > 0 ? (
            <button
              onClick={markAllRead}
              className="text-sm text-primary hover:underline"
            >
              Прочитать все
            </button>
          ) : undefined
        }
      />

      <div className="max-w-2xl">
        {items.length === 0 && (
          <div className="text-center py-16 text-gray-500 text-sm">
            Нет уведомлений
          </div>
        )}
        <ul className="divide-y divide-gray-100">
          {items.map((item) => (
            <li
              key={item.id}
              className={`flex items-start gap-3 py-3 px-2 rounded-lg transition-colors ${
                !item.is_read ? 'bg-blue-50' : 'hover:bg-gray-50'
              }`}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-0.5">
                  <Badge variant={TYPE_VARIANTS[item.type] ?? 'default'}>
                    {TYPE_LABELS[item.type] ?? item.type}
                  </Badge>
                  <span className="text-xs text-gray-400 tabular-nums">
                    {new Date(item.created_at).toLocaleString('ru', {
                      day: '2-digit', month: '2-digit', year: 'numeric',
                      hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                </div>
                <p className="text-sm text-gray-700">{item.body}</p>
              </div>
              {!item.is_read && (
                <button
                  onClick={() => markRead(item.id)}
                  className="shrink-0 text-xs text-gray-400 hover:text-gray-700"
                  aria-label="Отметить как прочитанное"
                >
                  ✓
                </button>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify compiles**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/notifications/NotificationsPage.tsx
git commit -m "feat(portal): add full-page notifications history"
```

---

## Task 13: Wire Up New Routes in App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Add imports for new pages**

At the top of `App.tsx`, add:

```tsx
import { CommandCenter } from './pages/CommandCenter';
import { NotificationsPage } from './pages/notifications/NotificationsPage';
```

- [ ] **Step 2: Add new routes inside the `<Route element={<RequireAuth />}>` block**

After `<Route path="/cascade/history/:id" element={<CascadeDeliveryDetail />} />`, add:

```tsx
<Route path="/command-center" element={<CommandCenter />} />
<Route path="/notifications" element={<NotificationsPage />} />
```

- [ ] **Step 3: Redirect /dashboard to /command-center**

Find the line:
```tsx
<Route path="/dashboard" element={<DashboardPage />} />
```

Replace it with:
```tsx
<Route path="/dashboard" element={<Navigate to="/command-center" replace />} />
```

- [ ] **Step 4: Update the catch-all redirect**

Find the final `<Route path="*" element={<Navigate to="/dashboard" replace />} />` and change to:

```tsx
<Route path="*" element={<Navigate to="/command-center" replace />} />
```

- [ ] **Step 5: Verify and check in browser**

```bash
cd portal-frontend && npx tsc --noEmit
```

Navigate to `http://localhost:5173/dashboard` — should redirect to `/command-center`.
Navigate to `/notifications` — should show notifications page.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/App.tsx
git commit -m "feat(portal): add /command-center and /notifications routes; redirect /dashboard"
```

---

## Self-Review Against Spec

### Coverage check

| Spec requirement | Task | Status |
|---|---|---|
| New 8-group action-oriented navigation | Task 1 | ✅ |
| SMPP moved to Integrations | Task 1 | ✅ |
| Dark theme CSS variables + transition | Task 2 | ✅ |
| KPI cards: speed/sparkline, delivery rate, balance/burn rate | Tasks 3, 8, 11 | ✅ |
| Live feed via WebSocket, pause/resume | Tasks 4, 5, 9, 11 | ✅ |
| Health Map with color-coded thresholds | Task 11 | ✅ |
| Smart Alerts (5 max, color-coded) | Tasks 8, 11 | ✅ |
| Active Campaigns with progress + ETA | Tasks 3, 11 | ✅ |
| Auto-refresh intervals (15s/30s) | Task 11 (usePolling) | ✅ |
| Global notification bell on all pages | Already exists in UserLayout | ✅ (pre-existing) |
| Full notifications page `/notifications` | Tasks 12, 13 | ✅ |
| CSV export utility | Already exists in `utils/csvExport.ts` | ✅ (pre-existing) |
| Backend: /dashboard/metrics (extended) | Tasks 6, 10 | ✅ |
| Backend: /providers/health | Tasks 7, 10 | ✅ |
| Backend: /alerts | Tasks 8, 10 | ✅ |
| Backend: /ws/messages WebSocket | Tasks 9, 10 | ✅ |
| Mobile responsive: single column stack | Task 11 (grid-cols-1 on mobile) | ✅ |

### Gaps/Notes

1. **`active_campaigns` in DashboardMetrics**: The backend `dashboard.go` extension in Task 6 does not yet add `active_campaigns` to the response (would require campaign service gRPC call). The frontend correctly shows empty array and the field is typed. Add a gRPC call to `CampaignClient.ListCampaigns` with status filter `running` in Task 6 as a follow-up commit.

2. **`burn_rate_per_hour` from billing**: The `burnRatePctOf2h` variable in Task 6 is declared but not populated (requires a billing transactions query for last 2h). Mark with `// TODO: compute from last-2h transactions` and return "0.00" until wired.

3. **`usePolling` import in CommandCenter**: The hook is at `hooks/usePolling.ts` — verify it exists (confirmed by explore agent).

4. **SSE hub `Unsubscribe`**: Task 9 adds this method only if it doesn't exist. Check `sse/hub.go` line 60+ before applying — if already present, skip.

---

## Execution Options

Plan complete and saved to `docs/superpowers/plans/2026-04-11-client-portal-mission-control-sprint1.md`.

**For Sprints 2–5, request separate plans:**
- Sprint 2: `реализуй quick-send, live-feed, troubleshooting` (Phases 2–3)
- Sprint 3: `реализуй analytics-compare, cost-simulator, finance-docs` (Phases 4, 6)
- Sprint 4: `реализуй contacts-hlr, integrations-metrics` (Phases 5, 7)
- Sprint 5: `реализуй settings-sessions, onboarding, audit-timeline` (Phases 8, 9.4–9.7)
