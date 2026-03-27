# QA Bugfix Sprint — SMS Portal

**Дата:** 2026-03-27
**Ветка:** 009-saas-tenant-isolation
**Источник:** QA-отчёт от 2026-03-27 (qa-ui-test@example.com)
**Подход:** 4 параллельных независимых блока по слоям

---

## Scope

Исправление 11 багов (2 critical, 5 major, 4 minor), 4 UX-проблем из QA-отчёта.

---

## Блок 1: Backend security & validation

### BUG-01 — ClientAuthMiddleware (critical)

**Файл:** `internal/gateway/client/middleware/auth.go`

Текущее состояние: middleware принимает `authClient authv1.AuthServiceClient` но игнорирует его — при `LOAD_TEST_MODE != true` возвращает 401 для всех запросов.

**Фикс:**
1. Извлечь заголовок `X-API-Key`
2. Если пустой — вернуть 401 `MISSING_API_KEY`
3. Вызвать `authClient.Authenticate(ctx, &authv1.AuthenticateRequest{ApiKey: key})`
4. При успехе — записать `user_id`, `client_id`, `role` в context через существующие contextKey константы
5. При gRPC-ошибке — вернуть 401 `INVALID_API_KEY`
6. Режим `LOAD_TEST_MODE=true` сохранить как есть (dummy IDs)

### BUG-02 — CSRF токен = Session ID (critical/security)

**Файл:** `internal/gateway/portal/handlers/auth.go`, функция `setSessionCookies`

Текущее состояние: `csrf_token` cookie выставляется со значением `sessionID`.

**Фикс:**
1. Генерировать отдельный случайный токен: `crypto/rand` → 32 байта → hex-строка
2. `csrf_token` cookie = новый случайный токен (не связан с session ID)
3. Middleware `portal-gateway`, проверяющий CSRF (если существует), должен сверять `X-CSRF-Token` header с cookie-значением

### BUG-04 — Email validation на backend (major)

**Файл:** `internal/gateway/portal/handlers/auth.go`, функция `Register`

**Фикс:** После проверки `req.Email == ""` добавить валидацию формата через `net/mail.ParseAddress`. При невалидном email — 400 `INVALID_INPUT` "Неверный формат email".

### BUG-05 — Long strings → INTERNAL_ERROR (major)

**Файлы:**
- `internal/gateway/portal/handlers/api_keys.go` — `CreateAPIKey`
- `internal/gateway/portal/handlers/auth.go` — `Register`

**Фикс:** Добавить проверки до gRPC-вызовов:
- `req.Name` (API key name): `len > 255` → 400 "Имя не может превышать 255 символов"
- `req.CompanyName`: `len > 500` → 400 "Название компании не может превышать 500 символов"

### BUG-07 — Webhook URL validation message (minor)

**Файл:** `internal/gateway/portal/handlers/webhooks.go` (или delivery_client.go — уточнить при реализации)

**Фикс:** Разделить валидацию URL на два шага:
1. `url.Parse` → невалидный URL → "неверный формат URL"
2. Проверка схемы `https` → "URL должен использовать HTTPS"

---

## Блок 2: Frontend auth & UX

### BUG-03 — Logout не очищает UI (major)

**Файл:** `portal-frontend/src/contexts/AuthContext.tsx`, функция `logout`

Текущее состояние:
```typescript
try {
  await authApi.logout();
} catch (e) {
  if (!(e instanceof ApiError && e.status === 401)) throw e; // блокирует setUser(null)
}
setUser(null);
```

**Фикс:** Перенести `setUser(null)` в `finally`:
```typescript
try {
  await authApi.logout();
} catch (e) {
  if (!(e instanceof ApiError && e.status === 401)) throw e;
} finally {
  setUser(null);
}
```

### BUG-08 — Balance пустой на Dashboard (minor)

**Файл:** `portal-frontend/src/pages/dashboard/DashboardPage.tsx`

**Фикс:** Явная проверка `balance !== null && balance !== undefined` вместо falsy-проверки `balance`. Отображать `0.00` при нулевом балансе.

### BUG-10 — Missing aria-description (minor)

**Файл:** `portal-frontend/src/components/ui/Modal.tsx`

**Фикс:** Добавить `description` prop в Modal. Если передан — рендерить `<p id="modal-desc" className="sr-only">` и добавить `aria-describedby="modal-desc"` на корневой элемент диалога.

### U4 — Profile не обновляется в sidebar (UX)

**Файл:** `portal-frontend/src/pages/profile/ProfilePage.tsx`

**Фикс:** После успешного сохранения профиля вызывать `refreshUser()` из `useAuth()`. Метод уже реализован в AuthContext.tsx:55.

---

## Блок 3: Mobile responsive

### BUG-06 — Мобильная вёрстка (major)

Breakpoints: Tailwind стандартные (`sm: 640px`, `md: 768px`).

**`components/layout/Sidebar.tsx`:**
- Desktop (≥768px): постоянно виден, `w-[220px]`, без изменений
- Mobile (<768px): позиция `fixed`, z-index высокий, slide-in через `transform translate-x` transition
- Props: добавить `isOpen: boolean`, `onClose: () => void`

**`components/layout/UserLayout.tsx`:**
- Добавить state `isMobileMenuOpen: boolean`
- Hamburger-кнопка (≡) в header, `md:hidden`
- Backdrop `div` при `isMobileMenuOpen`, клик вызывает `setIsMobileMenuOpen(false)`
- Передавать `isOpen` и `onClose` в Sidebar

**`components/data/DataTable.tsx`:**
- Обернуть `<table>` в `<div className="overflow-x-auto">`
- Добавить `responsive?: boolean` prop для колонок с `hidden sm:table-cell` (необязательные колонки)

**Страницы с сетками статистики (DashboardPage, AnalyticsPage):**
- `grid-cols-1 sm:grid-cols-2 lg:grid-cols-4` вместо фиксированной сетки

---

## Блок 4: Services & infra

### BUG-09 — ListPlans не реализован (minor)

**Шаги:**
1. Проверить `api/proto/client/client.proto` на наличие `rpc ListPlans`
2. Если отсутствует — добавить метод и перегенерировать pb.go (или добавить вручную по образцу существующих)
3. Реализовать `ListPlans` в `internal/services/client/grpc/server.go` — запрос в таблицу `plans`
4. Проверить что auth-service корректно использует результат при регистрации

### BUG-11 — Deploy не пересобирает образы (minor)

**Файл:** `scripts/server.sh`

**Фикс:** В команде `deploy` заменить `docker compose up -d` на `docker compose up -d --build`. Для `deploy <service>` — аналогично.

### U1 — Нет кнопки Send Message (UX)

**Файл:** `portal-frontend/src/pages/...` (MessagesPage или аналог)

**Фикс:** Добавить кнопку "Отправить SMS" → модал с полями: номер получателя, текст, sender ID. POST на существующий client-gateway endpoint.

### U2 — Providers empty state (UX)

**Файл:** `portal-frontend/src/pages/providers/` (ProvidersPage или список)

**Фикс:** Empty state с текстом "Провайдеры не настроены" и кнопкой "Добавить провайдера" → редирект на wizard `/providers/new`.

### U3 — Dashboard empty state (UX)

**Файл:** `portal-frontend/src/pages/dashboard/DashboardPage.tsx`

**Фикс:** Если все ключевые метрики = 0 и нет сообщений — показывать guidance блок: "Создайте API ключ → Отправьте тестовое SMS → Отслеживайте статистику".

---

## Файлы по блокам (summary)

| Блок | Файлы |
|------|-------|
| 1 — Backend | `internal/gateway/client/middleware/auth.go`, `internal/gateway/portal/handlers/auth.go`, `internal/gateway/portal/handlers/api_keys.go`, `internal/gateway/portal/handlers/webhooks.go` |
| 2 — Frontend | `portal-frontend/src/contexts/AuthContext.tsx`, `portal-frontend/src/pages/dashboard/DashboardPage.tsx`, `portal-frontend/src/components/ui/Modal.tsx`, `portal-frontend/src/pages/profile/ProfilePage.tsx` |
| 3 — Mobile | `portal-frontend/src/components/layout/Sidebar.tsx`, `portal-frontend/src/components/layout/UserLayout.tsx`, `portal-frontend/src/components/data/DataTable.tsx`, страницы с grid |
| 4 — Services | `api/proto/client/client.proto`, `internal/services/client/grpc/server.go`, `scripts/server.sh`, frontend pages (Messages, Providers, Dashboard) |

## Независимость блоков

- Блоки 1 и 2 не пересекаются по файлам
- Блок 3 затрагивает только frontend layout-компоненты, не пересекается с Блоком 2 (разные файлы)
- Блок 4 затрагивает proto/services и infra-скрипты — независим от остальных
- Блоки 1-4 можно выполнять параллельно в отдельных git worktrees
