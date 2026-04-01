# Research: Portal UX Improvements for Managers

**Feature**: 019-portal-ux-improvements  
**Phase**: 0 — Research  
**Date**: 2026-04-01

---

## 1. Recharts — Библиотека графиков

**Decision**: Использовать Recharts v3.8.1 (уже установлен в `portal-frontend/package.json`)

**Rationale**: Не нужна установка новых зависимостей. Recharts поддерживает все необходимые типы графиков:
- `LineChart` — временной ряд отправок (дашборд + аналитика)
- `PieChart` (режим donut с `innerRadius`) — распределение статусов
- `BarChart` с `layout="vertical"` — топ стран
- `Tooltip` с кастомным `content` — детальные подсказки

**Alternatives considered**: Chart.js (слишком тяжёлый), Victory (нет в зависимостях), D3 (слишком низкоуровневый для React)

---

## 2. Command Palette — Component Pattern

**Decision**: Собственный компонент на базе Radix UI `Dialog` (уже установлен: `@radix-ui/react-dialog: ^1.1.15`)

**Rationale**: Radix UI Dialog уже используется в проекте. Паттерн: `Dialog` с полем поиска, debounce 300 мс для API-запросов, клавиатурная навигация через `onKeyDown` + `aria-selected`. Статический реестр навигационных пунктов формируется из `NAV_GROUPS` в `UserLayout.tsx`.

**Implementation**: `portal-frontend/src/components/ui/CommandPalette.tsx` — новый компонент. Хук `useCommandPalette` для логики + состояния. Поиск по записям БД: новый бэкенд-эндпоинт `GET /portal/v1/search?q={query}`.

**Alternatives considered**: CMDK (новая зависимость), Headless UI (нет в проекте)

---

## 3. Notifications — Delivery Mechanism

**Decision**: REST polling каждые 30–60 сек (подтверждено в spec.md Clarifications)

**Rationale**: WebSocket/SSE не используются согласно FR-029. Новая таблица `notifications` в PostgreSQL (portal-gateway's own PostgreSQL pool). Notifications создаются portal-gateway при смене статусов шаблонов/имён отправителей (через существующие handlers), и через фоновый планировщик для кампаний.

**Backend pattern**: Фоновая горутина в portal-gateway по паттерну Start/Stop (как `scheduler.go`) периодически проверяет завершённые кампании и создаёт уведомления. Для шаблонов/имён отправителей — уведомления создаются синхронно в handlers при изменении статуса.

**Alternatives considered**: Kafka consumer в portal-gateway (сложнее, требует координации с campaign-service), SSE (исключено по FR-029)

---

## 4. Async CSV Export — Architecture

**Decision**: Синхронный экспорт через gRPC до 10K записей (уже реализован для messages). Асинхронный экспорт для >10K: job state хранится в Redis (TTL 1 час), файл генерируется в `/tmp`, ссылка на скачивание возвращается через polling.

**Rationale**: Избегает новой таблицы в БД. Redis уже используется в portal-gateway. Паттерн: POST /export/start → {job_id}, GET /export/{job_id}/status → {status, download_url}, GET /export/{job_id}/download → file stream.

**Alternatives considered**: PostgreSQL `export_jobs` таблица (избыточно для временных задач), S3 (нет в стеке)

---

## 5. DataTable Bulk Selection — Pattern

**Decision**: Расширить существующий `DataTable` через новый опциональный prop `onBulkAction`. Когда передан — добавляется колонка с чекбоксами и панель массовых действий над/под таблицей.

**Rationale**: Сохраняет обратную совместимость (prop необязательный). Существующие 15+ страниц с `DataTable` не затронуты. Состояние выбора локальное (`Set<string>` по key field).

**Implementation**: `DataTable` получает новый опциональный prop `bulkActions?: BulkAction[]`. Компонент `BulkActionBar` отображается над таблицей при `selectedIds.size > 0`.

---

## 6. Period Comparison in Analytics

**Decision**: Добавить query param `compare=true` к существующему `GET /portal/v1/analytics`. Бэкенд делает второй gRPC-запрос к analytics-service для предыдущего периода и возвращает `previous_timeline` в ответе.

**Rationale**: Не нужен новый эндпоинт. Минимальные изменения в существующем `AnalyticsHandlers.GetAnalytics()`.

**Cost forecast algorithm** (подтверждён в spec.md): скользящее среднее расходов за последние 7 дней × оставшиеся дни периода. Вычисляется на фронтенде из данных timeline.

---

## 7. Inline Form Validation

**Decision**: React-based inline validation без новых зависимостей. Хук `useFormValidation` с правилами (required, minLength, maxLength, pattern). Валидация `onChange` после первого `onBlur` (стандартный UX-паттерн). Счётчик символов через `CharacterCounter` компонент.

**Rationale**: В проекте нет react-hook-form или formik. Добавлять зависимость ради inline валидации — избыточно. Простой кастомный хук покрывает все сценарии из spec.

---

## 8. Resolved: Notifications — Creation Events

**Decision**: Создавать уведомления в следующих ситуациях:
1. **Завершение кампании** — фоновый планировщик в portal-gateway проверяет кампании со статусом `completed`/`failed` за последние 5 минут через gRPC к campaign-service
2. **Смена статуса шаблона** — в `templates.go` handler при ответе от template-service с новым статусом
3. **Смена статуса имени отправителя** — в `sender_names.go` handler при изменении статуса
4. **Низкий баланс** — в `billing.go` handler при пополнении баланса, если он всё ещё ниже порога

**Database**: Прямые INSERT через portal-gateway's pgxpool. Нет нового microservice.

---

## 9. Migration Numbering

**Decision**: Следующий номер миграции — `000072` (последняя существующая: `000071_max_messenger_channel`).

- `000072_notifications.up.sql` — таблица `notifications`

