# Implementation Plan: Portal UX Improvements for Managers

**Branch**: `019-portal-ux-improvements` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/019-portal-ux-improvements/spec.md`

## Summary

Улучшение UX портала для менеджеров: добавление интерактивных графиков на дашборд и аналитику (Recharts), массовые действия в таблицах, свободная навигация по шагам мастера рассылки, экспорт данных в CSV, палитра команд (Ctrl+K), inline-валидация форм и центр уведомлений с REST polling. Все изменения — расширение существующего portal-gateway (Go) и portal-frontend (React).

---

## Technical Context

**Language/Version**: Go 1.24.0 (backend), TypeScript 5.7 + React 19 (frontend)  
**Primary Dependencies**: gorilla/mux, gRPC (analyticsv1, messagingv1, campaignv1), pgx/v5 (portal's own pool), redis/go-redis/v9 (export jobs), Recharts 3.8.1 (charts), Radix UI Dialog/DropdownMenu/Tabs (UI components), Tailwind CSS 4.2  
**Storage**: PostgreSQL 15+ (новая таблица `notifications`), Redis 7+ (export job state, TTL 1h)  
**Testing**: testify (Go), существующие функциональные тесты в `test/`  
**Target Platform**: Linux server (Docker), Desktop browser (Chrome/Firefox)  
**Project Type**: Web application — портальный фронтенд + API gateway  
**Performance Goals**: Dashboard charts < 500ms, search results < 200ms, polling каждые 30–60 сек  
**Constraints**: UTF-8 без BOM для CSV; уведомления polling (не WebSocket/SSE); RUB-only; session auth + CSRF  
**Scale/Scope**: ~100 активных менеджеров, экспорт до 100K записей (async), уведомления до 1K/месяц/пользователь

---

## Constitution Check

### I. Domain-Driven Design ✅
- Новые handlers (`notifications.go`, `search.go`, `export.go`) добавляются в `internal/gateway/portal/handlers/` — тот же bounded context portal-gateway.
- Таблица `notifications` принадлежит portal-gateway — нет cross-service DB sharing.
- Фоновый scheduler читает данные только через gRPC к существующим сервисам.

### II. Event-Driven Architecture ✅
- Уведомления о кампаниях создаются через polling к campaign-service (gRPC), не через прямую Kafka-подписку в этом сервисе.
- Нет нарушения: portal-gateway не обязан быть Kafka-консьюмером для данной фичи.

### III. Contract-First APIs ✅
- Все новые HTTP-эндпоинты задокументированы в `contracts/portal-api.md` до реализации.
- Изменения к существующим `GET /analytics` и `GET /dashboard` — аддитивные (новые опциональные поля), backward-compatible.

### IV. Observability ✅
- Новые handlers логируют через zerolog.
- Prometheus метрики: счётчик `portal_notifications_created_total`, `portal_export_jobs_total` добавляются в новые handlers.

### V. Data Safety ✅
- Уведомления содержат только публичные данные (статусы, счётчики) — без SMS-текстов, балансов, API-ключей.
- Export файлы хранятся в `/tmp` с TTL 1 час (Redis TTL + goroutine cleanup).
- `user_id` проверяется при GET /export/{job_id}/download.

### VI. Simplicity ✅
- Не создаётся новый сервис — всё в portal-gateway.
- DataTable расширяется через опциональный prop (нет breaking change).
- Inline validation — кастомный хук без новых зависимостей.

**Result**: Все ворота пройдены. Complexity Tracking не требуется.

---

## Project Structure

### Documentation (this feature)

```text
specs/019-portal-ux-improvements/
├── plan.md              # Этот файл
├── research.md          # Phase 0 output ✅
├── data-model.md        # Phase 1 output ✅
├── quickstart.md        # Phase 1 output ✅
├── contracts/
│   └── portal-api.md   # Phase 1 output ✅
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code Layout

```text
# Backend — расширение portal-gateway
internal/gateway/portal/
├── handlers/
│   ├── analytics.go           # MODIFY: добавить compare + cost
│   ├── dashboard.go           # MODIFY: добавить charts data
│   ├── notifications.go       # NEW: GET /notifications, POST /read, POST /read-all
│   ├── search.go              # NEW: GET /search?q=
│   └── export.go              # NEW: POST /export/start, GET /status, GET /download
├── notifications/
│   └── scheduler.go           # NEW: фоновый планировщик создания уведомлений
└── router/
    └── router.go              # MODIFY: зарегистрировать новые маршруты

cmd/portal-gateway/
└── main.go                    # MODIFY: инициализация notifications scheduler

migrations/
├── 000072_notifications.up.sql   # NEW
└── 000072_notifications.down.sql # NEW

# Frontend — portal-frontend
src/
├── api/
│   └── client.ts              # MODIFY: notificationsApi, searchApi, exportApi
├── components/
│   ├── campaigns/
│   │   └── StepIndicator.tsx  # NEW: визуальный прогресс-бар шагов
│   ├── data/
│   │   ├── DataTable.tsx      # MODIFY: добавить bulk selection props
│   │   └── BulkActionBar.tsx  # NEW: панель массовых действий
│   ├── layout/
│   │   └── UserLayout.tsx     # MODIFY: добавить NotificationBell + Ctrl+K
│   └── ui/
│       ├── CommandPalette.tsx # NEW: модальная палитра команд
│       ├── NotificationBell.tsx  # NEW: иконка + бейдж
│       ├── NotificationPanel.tsx # NEW: выпадающая панель уведомлений
│       └── CharacterCounter.tsx  # NEW: счётчик символов с цветом
├── hooks/
│   ├── useNotifications.ts    # NEW: polling + read actions
│   ├── useCommandPalette.ts   # NEW: search state + keyboard nav
│   └── useFormValidation.ts   # NEW: inline validation rules + state
└── pages/
    ├── analytics/
    │   └── AnalyticsPage.tsx  # MODIFY: charts + comparison + cost tab
    ├── campaigns/
    │   └── CampaignWizardPage.tsx  # MODIFY: free step navigation
    ├── dashboard/
    │   └── DashboardPage.tsx  # MODIFY: добавить Recharts charts
    ├── messages/
    │   └── MessagesPage.tsx   # MODIFY: export кнопка + bulk actions
    ├── sender-names/          # MODIFY: bulk + inline validation
    └── templates/             # MODIFY: bulk + inline validation + char counter
```

**Structure Decision**: Web application (Option 2). Backend — `internal/gateway/portal/` (существующий). Frontend — `portal-frontend/src/` (существующий). Нет новых сервисов, нет новых проектов.

---

## Implementation Details

### Backend Changes

#### 1. Migration: `000072_notifications`

```sql
-- up
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    type        VARCHAR(64) NOT NULL,
    body        TEXT NOT NULL,
    object_type VARCHAR(64),
    object_id   VARCHAR(255),
    is_read     BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user_unread ON notifications (user_id, is_read)
    WHERE is_read = false;
CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);
```

#### 2. `notifications.go` — 3 handler functions

- `GetNotifications(w, r)` — SELECT с пагинацией + COUNT(unread)
- `MarkNotificationRead(w, r)` — UPDATE WHERE id + user_id
- `MarkAllNotificationsRead(w, r)` — UPDATE WHERE user_id

#### 3. `search.go` — Global Search

Параллельные запросы к 3 таблицам через gRPC:
- templates (name ILIKE `%q%`, LIMIT 3)
- sender_names (name ILIKE `%q%`, LIMIT 3)
- contact_lists (name ILIKE `%q%`, LIMIT 3)

Минимальная длина запроса: 2 символа. Результат слит в единый массив.

#### 4. `export.go` — Async Export

- `StartExport`: генерирует UUID, сохраняет в Redis (status=pending), запускает goroutine
- Goroutine: стримирует данные через gRPC (paginated), пишет CSV в `/tmp`, обновляет Redis
- `GetExportStatus`: читает из Redis
- `DownloadExport`: проверяет user_id, стримирует файл, удаляет после скачивания

#### 5. `dashboard.go` расширение

Дополнительный gRPC-запрос к analytics-service за данными 7 дней для charts. Параллельно с основным запросом через `errgroup`.

#### 6. `analytics.go` расширение

При `compare=true`: второй gRPC-запрос за предыдущий период того же размера. При `include_cost=true`: из billing-service.

#### 7. `notifications/scheduler.go`

Горутина по паттерну Start/Stop (ticker каждые 5 минут):
1. Запрашивает завершённые кампании за последние 10 минут через campaign-service gRPC
2. Проверяет, нет ли уже уведомления для этой кампании (SELECT EXISTS)
3. Создаёт уведомление если нет

---

### Frontend Changes

#### 1. Dashboard Charts (Recharts)

```tsx
// LineChart для timeline_7d
<ResponsiveContainer width="100%" height={200}>
  <LineChart data={charts.timeline_7d}>
    <Line type="monotone" dataKey="sent" stroke="#3B82F6" />
    <Line type="monotone" dataKey="delivered" stroke="#10B981" />
    <Tooltip />
    <XAxis dataKey="date" />
  </LineChart>
</ResponsiveContainer>

// PieChart (donut) для status_distribution
// BarChart layout="vertical" для top_countries
```

Skeleton-заглушки через существующий `animate-pulse` Tailwind класс при загрузке.

#### 2. DataTable Bulk Selection

Новые опциональные props:
```tsx
interface DataTableProps<T> {
  // ... existing props
  bulkActions?: BulkAction<T>[];  // если передан — включает режим bulk
}
```

Состояние `selectedIds: Set<string>` внутри компонента.  
Чекбокс в шапке: indeterminate если выбраны не все.  
`BulkActionBar` рендерится над таблицей при `selectedIds.size > 0`.

#### 3. CommandPalette

Открывается по `Ctrl+K` / `Cmd+K` (listener в `UserLayout`).  
Статический реестр: все пункты из `NAV_GROUPS` + "Создать рассылку", "Добавить шаблон" и т.д.  
Динамический поиск: debounce 300ms → `GET /search?q=...`  
Keyboard nav: `ArrowUp/Down` меняют `activeIndex`, `Enter` активирует, `Escape` закрывает.

#### 4. NotificationBell + Polling

`useNotifications` hook:
- `useEffect` → `setInterval(30_000)` для polling
- Кеширует в state, обновляет при фокусе вкладки (`visibilitychange` event)

`NotificationBell`: иконка + бейдж `unread_count` (скрыт при 0).  
`NotificationPanel`: `@radix-ui/react-dropdown-menu` с историей уведомлений.

#### 5. CampaignWizard — Free Navigation

`StepIndicator` компонент с кликабельными шагами.  
Клик на шаг i: `if (i <= maxReachedStep) setStep(STEPS[i])`.  
Ошибки валидации: `validationErrors: Record<WizardStep, boolean>` — красная точка на индикаторе.

#### 6. Inline Validation

```tsx
const { fieldProps, isValid, errors } = useFormValidation({
  name: { required: true, minLength: 3, maxLength: 100 },
  body: { required: true, maxLength: 1600 },
});
```

`fieldProps(fieldName)` возвращает `{ onChange, onBlur, 'aria-invalid', 'aria-describedby' }`.  
Валидация запускается на `onBlur`, а затем на каждый `onChange` если поле уже было потронуто.

---

## Complexity Tracking

*(Нет нарушений конституции — секция пуста)*

