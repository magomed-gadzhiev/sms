# QA Testing Fixes — Design Spec

**Date:** 2026-04-15
**Status:** Approved

## Overview

4 проблемы, выявленные при тестировании портала. Все — баги/пробелы в существующей функциональности, а не новые фичи.

---

## 1. [HIGH] `/admin/v1/audit` — отсутствующий маршрут

### Проблема

Фронтенд admin-панели (`portal-frontend/src/pages/admin/AuditLogPage.tsx`) вызывает `GET /admin/v1/audit`, но маршрут не зарегистрирован в admin gateway. Вся инфраструктура готова: БД (`audit_log`), gRPC-сервис (`auditv1.AuditService.QueryAuditLog`), портальный handler (`internal/gateway/portal/handlers/audit.go`).

### Решение

**Новый файл:** `internal/gateway/admin/handlers/audit.go`
- Структура `AuditHandlers` с полем `auditClient auditv1.AuditServiceClient`
- Метод `ListAuditLog(w, r)`:
  - Парсит query-параметры: `page`, `page_size`, `action`, `user_id`, `date_from`, `date_to`, `client_id`
  - Вызывает gRPC `QueryAuditLog`
  - Отличие от портального handler: нет фильтрации по `tenant_id` (admin видит все записи), добавлен опциональный фильтр `client_id`

**Регистрация маршрута:** `internal/gateway/admin/router/router.go`
- `adminV1.HandleFunc("/audit", auditHandlers.ListAuditLog).Methods("GET")`
- Передача `AuditHandlers` в конструктор роутера

**Инициализация:** в admin gateway setup — создание `AuditHandlers` с audit gRPC-клиентом.

**Формат ответа** (совпадает с портальным):
```json
{
  "entries": [...],
  "total": 150,
  "page": 1,
  "page_size": 20
}
```

### Затрагиваемые файлы

- `internal/gateway/admin/handlers/audit.go` — **новый**
- `internal/gateway/admin/router/router.go` — регистрация маршрута
- Admin gateway setup (clients/main) — инициализация handler

---

## 2. [HIGH] Analytics API: `groups` vs `rows`

### Проблема

Backend (`internal/gateway/admin/handlers/analytics.go:228`) возвращает `{"groups": [...], "totals": {...}}`. Frontend (`portal-frontend/src/api/admin.ts:248`, `portal-frontend/src/pages/admin/AnalyticsPage.tsx:99`) ожидает `{"rows": [...]}`. Результат — пустая таблица аналитики.

### Решение

Привести frontend в соответствие с backend/proto контрактом:

- `portal-frontend/src/api/admin.ts` (строки 247-256): переименовать `GroupedStatsResponse.rows` → `groups`, тип `GroupedStatRow` → `GroupedStatGroup`
- `portal-frontend/src/pages/admin/AnalyticsPage.tsx` (строка 99): `grouped?.rows` → `grouped?.groups`
- Проверить и обновить все остальные использования `.rows` из этого интерфейса

Backend и proto не изменяются.

### Затрагиваемые файлы

- `portal-frontend/src/api/admin.ts` — интерфейс
- `portal-frontend/src/pages/admin/AnalyticsPage.tsx` — использование

---

## 3. [HIGH] Admin UI: нет `client_id` при создании пользователя

### Проблема

Поле `client_id` существует в БД (миграция 000035), в `UpdateUserRequest` proto, но отсутствует в `CreateUserRequest` proto, Go handler, API-клиенте и форме создания. Админу приходится создавать пользователя, а затем отдельно назначать клиента через assign-client.

### Решение

**Proto:** `api/proto/auth/auth.proto`
- Добавить `string client_id = 6;` в `CreateUserRequest`
- Перегенерировать Go/gRPC код

**Backend:** `internal/gateway/admin/handlers/users.go` (строки 87-127)
- Добавить `ClientID string json:"client_id"` в struct запроса
- Передать `ClientId` в gRPC `CreateUserRequest`

**Auth-сервис:** handler `CreateUser`
- При наличии `client_id` — сохранять в `users.client_id` при INSERT

**Frontend API:** `portal-frontend/src/api/admin.ts` (строки 630-631)
- Добавить `client_id?: string` в тип параметров `create`

**Frontend форма:** `portal-frontend/src/pages/admin/users/UsersPage.tsx`
- Добавить `client_id: ''` в `createForm` state
- Добавить выпадающий список клиентов с поиском (данные из `GET /admin/v1/clients`)
- Поле опциональное

### Затрагиваемые файлы

- `api/proto/auth/auth.proto` — proto-определение
- `api/proto/authv1/` — перегенерированный код
- `internal/gateway/admin/handlers/users.go` — HTTP handler
- Auth-сервис gRPC handler — сохранение client_id
- `portal-frontend/src/api/admin.ts` — API-клиент
- `portal-frontend/src/pages/admin/users/UsersPage.tsx` — форма

---

## 4. [MED] Валидация баланса суб-аккаунта — UX-улучшения

### Проблема

При создании суб-аккаунта handler вслепую вызывает `TransferBalance`. Если у реселлера недостаточно средств, суб-аккаунт создаётся с нулевым балансом, а пользователь получает невнятный toast. Фронтенд не показывает доступный баланс.

### Решение (только frontend)

**Форма создания:** `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`
- При открытии модалки — загружать баланс реселлера через `GET /portal/v1/billing/balance`
- Показывать под полем "Начальный баланс": "Доступный баланс: X RUB"
- Если введённая сумма > доступного баланса — жёлтое предупреждение: "Сумма превышает доступный баланс. Суб-аккаунт будет создан, но перевод средств не выполнится"
- Кнопка "Создать" не блокируется — это предупреждение, не ошибка

**Toast-сообщение:** (строки 140-144)
- При `balance_transfer_error` использовать `toast.warning()` вместо `toast.success()`
- Текст: "Суб-аккаунт создан. Перевод начального баланса не выполнен — недостаточно средств"

Backend не изменяется — billing-сервис уже обрабатывает `ErrInsufficientBalance`.

### Затрагиваемые файлы

- `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` — форма и toast

---

## Вне скоупа

- Рефакторинг audit handler для переиспользования между portal/admin
- Backend pre-check баланса перед созданием суб-аккаунта
- Изменение assign-client endpoint (остаётся для отдельного использования)
