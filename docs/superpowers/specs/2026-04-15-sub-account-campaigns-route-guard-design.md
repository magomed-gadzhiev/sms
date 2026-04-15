# Sub-Account Campaigns Endpoint & Route Guard

**Дата:** 2026-04-15
**Статус:** Утверждён

## Проблемы

1. **HIGH — Агрегатор не видит кампании суб-аккаунта.** Нет endpoint `GET /portal/v1/sub-accounts/{id}/campaigns` в campaign service. Таб «Кампании» на странице суб-аккаунта — placeholder.
2. **MED — Маршрут `/sub-accounts` не защищён по роли.** Суб-аккаунт может перейти по прямому URL и увидеть пустую страницу списка суб-аккаунтов (навигационная ссылка уже скрыта через `is_reseller`).

## Решение

### 1. Backend: `GET /portal/v1/sub-accounts/{id}/campaigns`

**Файлы:**
- `internal/gateway/portal/handlers/sub_accounts.go` — новый метод `GetSubAccountCampaigns`
- `internal/gateway/portal/router/router.go` — регистрация маршрута, инжекция `campaignClient`

**Логика `GetSubAccountCampaigns`:**
1. `getParentClientID(r)` — проверка аутентификации вызывающего
2. `GetSubAccount(subAccountID, parentClientID)` — проверка принадлежности суб-аккаунта
3. `ListCampaigns(clientId = subAccountID, status, limit, offset)` — получение кампаний
4. Проксирование gRPC-ответа как JSON

**Пагинация:** `parsePagination(r)` (page/per_page query params).
**Фильтрация:** `?status=` query param.

**Изменения в структуре:**
- Добавить `campaignClient campaignv1.CampaignServiceClient` в `SubAccountHandlers`
- Обновить `NewSubAccountHandlers` (конструктор)
- Обновить вызов конструктора в router.go

**Маршрут:** после существующих sub-account маршрутов (после `/sub-accounts/{id}/webhooks`).

### 2. Frontend: таб «Кампании» в SubAccountDetailPage

**Файлы:**
- `portal-frontend/src/api/client.ts` — новый метод в `subAccountsApi`
- `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` — замена placeholder

**API-клиент:**
```ts
campaigns: (id: string, params?: { page?: number; status?: string }) =>
  apiFetch<{ campaigns: Campaign[]; total: number }>(`/sub-accounts/${id}/campaigns?...`)
```

**Компонент `CampaignsTab`:**
- State: `campaigns`, `loading`, `error`, `page`, `statusFilter`
- `useEffect` для вызова API при смене `page`/`statusFilter`
- `DataTable` с колонками: Название, Статус (Badge), Получатели, Доставлено, Создана
- Пагинация, фильтр по статусу
- **Только для чтения** — агрегатор просматривает кампании суб-аккаунта, но не управляет ими (нет кнопок запуска/паузы/удаления)

### 3. Frontend: Route guard `RequireReseller`

**Файлы:**
- `portal-frontend/src/components/RequireReseller.tsx` — новый компонент
- `portal-frontend/src/App.tsx` — оборачивание маршрутов

**Компонент `RequireReseller`:**
```tsx
export function RequireReseller({ children }) {
  const { isAuthenticated, user, loading } = useAuth();
  if (loading) return <div>Loading...</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (!user?.is_reseller) return <Navigate to="/dashboard" replace />;
  return <>{children}</>;
}
```

**Применение в App.tsx:**
```tsx
<Route path="/sub-accounts" element={<RequireReseller><SubAccountsListPage /></RequireReseller>} />
<Route path="/sub-accounts/:id" element={<RequireReseller><SubAccountDetailPage /></RequireReseller>} />
```

## Не входит в скоуп

- Управление кампаниями суб-аккаунта от имени агрегатора (CRUD)
- Новая роль `aggregator` (используем существующий флаг `is_reseller`)
- Бэкенд-middleware для блокировки non-reseller вызовов на `/sub-accounts/*` (бэкенд уже фильтрует по `parentClientID`)
