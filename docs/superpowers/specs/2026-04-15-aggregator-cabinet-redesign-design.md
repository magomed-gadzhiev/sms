# Дизайн: Редизайн кабинета агрегатора

**Дата:** 2026-04-15
**Статус:** Approved

---

## Контекст

Агрегаторы (`is_reseller = true`) — ключевой источник прибыли платформы. Однако их кабинет сейчас не отличается от кабинета обычного клиента: суб-аккаунты спрятаны в группе «Настройки» как единственный пункт меню, нет дашборда сети, модерация реализована частично (только operator registrations на бэкенде), маршрутизация и тарифы для субаккаунтов не имеют UI, аналитика по сети отсутствует.

Задача: сделать работу агрегатора в системе первоклассным опытом — удобным, полнофункциональным и масштабируемым.

---

## Решения

| Вопрос | Решение |
|--------|---------|
| Структура навигации | Переключатель режима: «Свой аккаунт» / «Управление сетью» |
| Запоминание выбора | localStorage (persist между сессиями) |
| Разделы режима «Управление сетью» | Дашборд сети, Суб-аккаунты, Модерация, Маршрутизация, Тарифы, Аналитика |
| Модерация | Единая страница с 3 вкладками: Имена / Шаблоны / Регистрации |
| Действия модерации sender names | Approve / Reject (с причиной) |
| Действия модерации templates | Approve / Reject / Request Revision |
| Действия модерации operator regs | Approve / Reject / Request Revision (уже реализовано) |
| Финальность approve агрегатора | Финальный `approved` статус, суперадмин не участвует |
| Тарифная модель | Фиксированные цены за SMS по направлениям (не маржа) |
| Маршрутизация | Общая страница + вкладка в деталях субаккаунта |
| Дашборд | 7 блоков метрик: баланс сети, трафик, delivery rate, pending-заявки, топ-5, проблемные, выручка |
| Бэкенд sender names/templates | Ownership check в portal gateway + делегация admin gRPC |

---

## Фазы реализации

### Фаза 1 — Ядро (переключатель + структура)

**Цель:** Дать агрегатору отдельное рабочее пространство.

**Переключатель режима:**
- Компонент `ModeSwitcher` в верхней части сайдбара (или в header)
- Два режима: «Свой аккаунт» (иконка пользователя) / «Управление сетью» (иконка сетки/группы)
- Выбор сохраняется в `localStorage` ключ `reseller_mode` (`own` | `network`)
- При входе: если `is_reseller = true`, читается сохранённый режим; если нет — всегда `own`
- При переключении: URL меняется на корневой раздел выбранного режима (`/dashboard` или `/network/dashboard`)

**Сайдбар в режиме «Управление сетью»:**

```
Дашборд сети          /network/dashboard
Суб-аккаунты          /network/sub-accounts
                      /network/sub-accounts/:id
Модерация [badge: N]  /network/moderation
Маршрутизация         /network/routing
Тарифы                /network/tariffs
Аналитика сети        /network/analytics
─────────────────────
← Свой аккаунт        (переключатель обратно)
```

**Сайдбар в режиме «Свой аккаунт»:**
Текущий сайдбар без изменений, кроме:
- Удалить «Суб-аккаунты» из группы «Настройки»
- Добавить переключатель «Управление сетью →» внизу/вверху

**Маршрутизация React Router:**

```
/network/*  → NetworkLayout (отдельный layout с сетевым сайдбаром)
  /network/dashboard       → NetworkDashboardPage
  /network/sub-accounts    → SubAccountsListPage (перенос существующего)
  /network/sub-accounts/:id → SubAccountDetailPage (перенос существующего)
  /network/moderation      → ModerationPage (вкладки)
  /network/routing         → NetworkRoutingPage
  /network/tariffs         → NetworkTariffsPage
  /network/analytics       → NetworkAnalyticsPage
```

Все `/network/*` маршруты обёрнуты в `RequireReseller`.

**Файлы фазы 1:**

| Файл | Тип |
|------|-----|
| `portal-frontend/src/components/layout/NetworkLayout.tsx` | Новый |
| `portal-frontend/src/components/layout/ModeSwitcher.tsx` | Новый |
| `portal-frontend/src/components/layout/UserLayout.tsx` | Изменение (добавить переключатель, убрать суб-аккаунты из Settings) |
| `portal-frontend/src/App.tsx` | Изменение (новые /network/* маршруты) |
| `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx` | Изменение (адаптация URL) |
| `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Изменение (адаптация URL) |

---

### Фаза 2 — Модерация

**Цель:** Агрегатор модерирует sender names, templates и operator registrations субаккаунтов.

**Единая страница `ModerationPage` с 3 вкладками:**

1. **Имена отправителей** — таблица: субаккаунт, имя, статус, дата. Действия: Одобрить / Отклонить (модал с причиной). Badge с кол-вом pending.
2. **Шаблоны** — таблица: субаккаунт, название, тело (truncated), статус, дата. Действия: Одобрить / Отклонить / Запросить доработку. Badge с кол-вом pending.
3. **Регистрации у операторов** — переиспользует данные из существующего бэкенда `reseller_moderation.go`. Действия: Одобрить / Отклонить / Запросить доработку.

**Счётчик в сайдбаре:**
- Общий badge рядом с «Модерация» — сумма pending по всем трём типам
- Отдельный API эндпоинт `GET /portal/v1/reseller/moderation/counts` возвращает `{ sender_names: N, templates: N, registrations: N }`

**Бэкенд — новые файлы/изменения:**

| Файл | Тип | Описание |
|------|-----|----------|
| `internal/gateway/portal/handlers/reseller_sender_names.go` | Новый | List / Approve / Reject sender names субаккаунтов |
| `internal/gateway/portal/handlers/reseller_templates.go` | Новый | List / Approve / Reject / RequestRevision templates |
| `internal/gateway/portal/handlers/reseller_moderation.go` | Изменение | Добавить `GetModerationCounts` |
| `internal/gateway/portal/router/router.go` | Изменение | Новые маршруты под /reseller |

**Новые маршруты:**

```
GET  /portal/v1/reseller/moderation/counts

GET  /portal/v1/reseller/sender-names
POST /portal/v1/reseller/sender-names/{id}/approve
POST /portal/v1/reseller/sender-names/{id}/reject

GET  /portal/v1/reseller/templates
POST /portal/v1/reseller/templates/{id}/approve
POST /portal/v1/reseller/templates/{id}/reject
POST /portal/v1/reseller/templates/{id}/request-revision
```

Существующие маршруты operator registrations остаются:
```
GET  /portal/v1/reseller/operator-registrations
POST /portal/v1/reseller/operator-registrations/{id}/approve
POST /portal/v1/reseller/operator-registrations/{id}/reject
POST /portal/v1/reseller/operator-registrations/{id}/request-revision
```

**Бэкенд-паттерн (одинаковый для sender names и templates):**
1. `checkReseller()` — проверка `is_reseller = true`
2. SQL ownership check — `JOIN clients c ON ... WHERE c.parent_client_id = $aggregator_id`
3. Проверка статуса (действие доступно только из `pending` / `revision_requested`)
4. Вызов существующего admin gRPC метода с `actor_id` = portal user ID

**Фронтенд:**

| Файл | Тип |
|------|-----|
| `portal-frontend/src/pages/network/ModerationPage.tsx` | Новый |
| `portal-frontend/src/api/client.ts` | Изменение (новые API методы) |

---

### Фаза 3 — Дашборд сети

**Цель:** Агрегатор видит состояние своей сети на одном экране.

**7 блоков:**

1. **Общий баланс сети** — карточка: сумма балансов всех субаккаунтов + свой баланс. Показывает: свой / субаккаунтов / итого.

2. **Трафик** — карточка: общее кол-во SMS за сегодня / неделю / месяц (переключатель периода). Суммируется по всем субаккаунтам.

3. **Delivery rate сети** — карточка: средний % доставки по всем субаккаунтам за выбранный период.

4. **Pending-заявки** — карточка с разбивкой: имена (N) / шаблоны (N) / регистрации (N). Клик → переход на страницу модерации.

5. **Топ-5 субаккаунтов по трафику** — мини-таблица: субаккаунт, кол-во SMS, delivery rate. За текущий месяц.

6. **Проблемные субаккаунты** — список алертов: низкий баланс (< порога), исчерпаны лимиты (daily/monthly), delivery rate < 70%.

7. **Выручка сети** — карточка: заработано на разнице тарифов (sum по всем субаккаунтам: (цена для субаккаунта - цена агрегатора) * кол-во SMS). За текущий месяц. Доступно после реализации Фазы 4 (тарифы).

**Бэкенд:**

| Файл | Тип | Описание |
|------|-----|----------|
| `internal/gateway/portal/handlers/reseller_dashboard.go` | Новый | Агрегированные метрики сети |

**Новые маршруты:**

```
GET /portal/v1/reseller/dashboard
    ?period=today|7d|30d
    Возвращает: network_balance, traffic, delivery_rate,
               moderation_counts, top_sub_accounts,
               problem_sub_accounts, revenue
```

**Источники данных:**
- Баланс: `billingClient.GetBalance` для каждого субаккаунта (или новый batch gRPC метод)
- Трафик + delivery rate: `analyticsClient.GetStatistics` для каждого субаккаунта (или агрегированный SQL)
- Moderation counts: переиспользует `GetModerationCounts` из Фазы 2
- Проблемные: SQL запрос к `clients` + `billing_accounts` с условиями
- Выручка: зависит от тарифов (Фаза 4), placeholder до реализации

**Фронтенд:**

| Файл | Тип |
|------|-----|
| `portal-frontend/src/pages/network/NetworkDashboardPage.tsx` | Новый |

---

### Фаза 4 — Маршрутизация и тарифы

**Цель:** Агрегатор полностью управляет маршрутизацией и ценами для субаккаунтов.

#### 4A. Маршрутизация сети

**Общая страница `NetworkRoutingPage`:**
- Таблица всех маршрутов по всем субаккаунтам
- Фильтры: субаккаунт, провайдер, страна/оператор
- Действия: назначить провайдера, создать маршрут, отозвать
- Массовое назначение: выбрать несколько субаккаунтов → назначить провайдера/маршрут

**Вкладка «Маршрутизация» в деталях субаккаунта:**
- Список назначенных провайдеров с возможностью добавить/удалить
- Список маршрутов с возможностью создать/редактировать/удалить
- Переиспользует существующие бэкенд-эндпоинты:
  ```
  POST /portal/v1/sub-accounts/{id}/providers
  GET  /portal/v1/sub-accounts/{id}/providers
  DELETE /portal/v1/sub-accounts/{id}/providers/{pid}
  POST /portal/v1/sub-accounts/{id}/routes
  GET  /portal/v1/sub-accounts/{id}/routes
  ```

**Бэкенд — новые маршруты для общей страницы:**

```
GET  /portal/v1/reseller/routing/providers   — все назначения провайдеров по всем субаккаунтам
GET  /portal/v1/reseller/routing/routes      — все маршруты по всем субаккаунтам
POST /portal/v1/reseller/routing/bulk-assign — массовое назначение провайдера/маршрута
```

#### 4B. Тарифы субаккаунтов

**Модель:** Агрегатор задаёт фиксированные цены за SMS по направлениям (страна + оператор) для каждого субаккаунта.

**Страница `NetworkTariffsPage`:**
- Выбор субаккаунта (dropdown)
- Таблица тарифов: страна, оператор, цена за SMS (руб.), валюта
- Действия: установить цену, массовая установка (все направления одной ценой)
- Кнопка «Скопировать тарифы» из другого субаккаунта

**Бэкенд:**

| Файл | Тип | Описание |
|------|-----|----------|
| `internal/gateway/portal/handlers/reseller_tariffs.go` | Новый | CRUD тарифов субаккаунтов |

**Новые маршруты:**

```
GET  /portal/v1/reseller/tariffs?sub_account_id=...          — тарифная сетка субаккаунта
PUT  /portal/v1/reseller/tariffs                              — установить/обновить тарифы
POST /portal/v1/reseller/tariffs/copy                         — скопировать из субаккаунта A в B
```

**Таблица БД:** Использует существующую инфраструктуру `AggregatorTariffRepository` (уже есть в коде). Если таблицы нет — создать:

```sql
CREATE TABLE reseller_sub_account_tariffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_client_id UUID NOT NULL REFERENCES clients(id),
    sub_account_id UUID NOT NULL REFERENCES clients(id),
    country_id VARCHAR(3) NOT NULL,
    operator_id UUID REFERENCES operators(id),
    price_per_sms NUMERIC(10,6) NOT NULL,
    currency VARCHAR(3) DEFAULT 'RUB',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(sub_account_id, country_id, operator_id)
);
```

#### 4C. Аналитика сети

**Страница `NetworkAnalyticsPage`:**
- Сводные графики: трафик, delivery rate, выручка по времени
- Фильтры: период, субаккаунт (все / конкретный), направление
- Сравнение субаккаунтов: выбрать 2-5 и сравнить метрики

**Бэкенд:**

```
GET /portal/v1/reseller/analytics
    ?period=7d|30d|90d
    &sub_account_id=...   (опционально)
    &group_by=day|week|month
```

Агрегирует данные через `analyticsClient.GetStatistics` по каждому субаккаунту.

**Фронтенд фазы 4:**

| Файл | Тип |
|------|-----|
| `portal-frontend/src/pages/network/NetworkRoutingPage.tsx` | Новый |
| `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` | Новый |
| `portal-frontend/src/pages/network/NetworkAnalyticsPage.tsx` | Новый |
| `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx` | Изменение (новая вкладка «Маршрутизация») |

---

## Сводная таблица файлов по фазам

| Фаза | Новых файлов | Изменённых файлов |
|------|-------------|-------------------|
| 1 — Ядро | 2 (NetworkLayout, ModeSwitcher) | 3 (UserLayout, App, SubAccount pages) |
| 2 — Модерация | 3 (reseller_sender_names.go, reseller_templates.go, ModerationPage.tsx) | 3 (reseller_moderation.go, router.go, client.ts) |
| 3 — Дашборд | 2 (reseller_dashboard.go, NetworkDashboardPage.tsx) | 1 (router.go) |
| 4 — Маршрутизация/Тарифы/Аналитика | 6 (3 backend + 3 frontend) | 3 (router.go, client.ts, SubAccountDetailPage) |

---

## Ограничения

- Proto-файлы и gRPC-сервисы не изменяются в фазах 1-3. Фаза 4 может потребовать новый gRPC сервис для тарифов (или прямой SQL).
- Выручка на дашборде (блок 7) становится доступна только после реализации тарифов (Фаза 4). До этого — placeholder «Доступно после настройки тарифов».
- Batch-запросы к gRPC для дашборда: если производительность недостаточна (N+1 запросов по субаккаунтам), оптимизация через агрегированные SQL запросы.
- Обратная совместимость URL: `/sub-accounts/*` редиректит на `/network/sub-accounts/*`.
