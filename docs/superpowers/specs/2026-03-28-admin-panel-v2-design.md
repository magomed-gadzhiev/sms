# Admin Panel v2 — Design Spec

**Дата:** 2026-03-28
**Заменяет:** 2026-03-27-admin-panel-design.md
**Подход:** Раздел в существующем portal-frontend с модульной архитектурой

## Контекст

### Текущее состояние
- **Admin Gateway** (Go) — 72+ REST-эндпоинтов (`/admin/v1/*`), gRPC-интеграция с 10+ микросервисами
- **Portal Frontend** (React 19 + TS + Vite + Tailwind 4 + Radix UI) — 11 admin-страниц уже реализованы (Clients, Providers, Routes, Billing, Monitoring, Analytics, Templates, Webhooks, HLR, Countries, Audit)
- **Auth** — роли admin/client/operator/superadmin, JWT + сессии Redis, TOTP 2FA
- **БД** — permissions/role_permissions таблицы существуют, но гранулярные permissions не используются в UI

### Что нужно сделать
1. Новый layout: compact sidebar + breadcrumbs
2. Live dashboard с автообновлением и алертами
3. Расширенный биллинг: заморозка, кредитный лимит, овердрафт, алерты низкого баланса
4. Новый модуль тарификации (API есть, UI нет)
5. Workflow шаблонов с этапами и ревьюером
6. Управление пользователями и гранулярные permissions
7. Интеграция permissions во все существующие модули

---

## 1. Архитектура и layout

### Compact Sidebar
- Ширина: 56px в свёрнутом состоянии (только иконки), 220px при hover
- Группы разделов:
  - **—** Dashboard (вне группы, первый пункт)
  - **Основное:** Клиенты, Шаблоны, Пользователи
  - **Финансы:** Биллинг, Тарификация
  - **Инфраструктура:** Провайдеры, Маршруты, HLR
  - **Справочники:** Операторы, Вебхуки
  - **Наблюдение:** Аналитика, Мониторинг, Аудит
- Пункты без permission у текущего пользователя — скрываются
- Подсветка активного пункта, tooltip с названием при свёрнутом состоянии

### Breadcrumbs
- Используем существующий prop `breadcrumbs` в компоненте `PageHeader`
- Формат: `Админ / Биллинг / Балансы`
- Clickable — каждый уровень ведёт на соответствующий маршрут

### Файловая структура
```
src/
  components/
    layout/
      AdminLayout.tsx          ← НОВЫЙ: wrapper для /admin/*
      AdminSidebar.tsx         ← НОВЫЙ: compact sidebar
    auth/
      RequirePermission.tsx    ← НОВЫЙ: permission guard
    data/
      PermissionMatrix.tsx     ← НОВЫЙ: таблица чекбоксов resource×action
      TimelineEvent.tsx        ← НОВЫЙ: элемент таймлайна
  hooks/
    usePolling.ts              ← НОВЫЙ: auto-refresh hook
    usePermission.ts           ← НОВЫЙ: проверка прав
  pages/admin/
    dashboard/
      DashboardPage.tsx        ← НОВЫЙ
      components/
        MetricsGrid.tsx
        TrafficChart.tsx
        AlertsFeed.tsx
    billing/
      BillingPage.tsx          ← РАСШИРИТЬ: добавить табы
      BalancesTab.tsx           ← НОВЫЙ
      TransactionsTab.tsx       ← существующий, вынести в отдельный файл
      CreditLimitsTab.tsx       ← НОВЫЙ
      PricingTab.tsx            ← существующий, вынести в отдельный файл
    tarification/               ← НОВЫЙ модуль
      TarificationPage.tsx
      PlansTab.tsx
      PeriodsTab.tsx
      TiersTab.tsx
      SenderRegistrationsTab.tsx
    templates/
      TemplatesPage.tsx         ← РАСШИРИТЬ: workflow
      TemplateReviewModal.tsx   ← НОВЫЙ: расширенная модалка ревью
    users/                      ← НОВЫЙ модуль
      UsersPage.tsx
      RolesPage.tsx
```

### Роутинг
```
/admin                          → redirect to /admin/dashboard
/admin/dashboard                → DashboardPage
/admin/clients                  → ClientsPage (существующий)
/admin/billing                  → BillingPage (табы: балансы, транзакции, лимиты, цены)
/admin/tarification             → TarificationPage (табы: планы, периоды, тиры, отправители)
/admin/providers                → ProvidersPage (существующий)
/admin/routes                   → RoutesPage (существующий)
/admin/templates                → TemplatesPage (расширенный)
/admin/analytics                → AnalyticsPage (существующий)
/admin/monitoring               → MonitoringPage (существующий)
/admin/audit                    → AuditLogPage (существующий)
/admin/users                    → UsersPage
/admin/users/roles              → RolesPage
/admin/webhooks                 → WebhooksPage (существующий)
/admin/hlr                      → HLRPage (существующий)
/admin/countries                → CountriesPage (существующий)
```

Каждый маршрут обёрнут в `<RequirePermission resource="..." action="read">`.

---

## 2. Live Dashboard

### Метрики (polling каждые 30 сек)

**Ряд 1 — Трафик:**
| Карточка | Источник | Формат |
|----------|----------|--------|
| Сообщений/сек | analyticsAdminApi.getRealtimeMetrics() | число + trend vs вчера |
| Доставлено сегодня | то же | число + delivery rate % |
| Ошибки | то же | число + trend, красный если выше нормы |
| В очереди | то же | число + avg latency |

**Ряд 2 — Бизнес:**
| Карточка | Источник | Формат |
|----------|----------|--------|
| Активных клиентов | clientsApi.list() | число из total |
| Выручка сегодня | billingApi.getTransactions({type: 'charge', today}) | сумма ₽ + trend |
| Провайдеры | providersApi.listHealth() | X/Y healthy + worst status |
| Шаблонов на ревью | templatesApi.list({status: 'pending'}) | число + сколько ждут >24ч |

**График трафика за час:**
- Recharts BarChart, группировка по 5 минут
- Два ряда: delivered (фиолетовый) + failed (красный)
- Источник: analyticsAdminApi.getStats({ period: '1h', group_by: '5min' })

**Лента алертов:**
- Вычисляются на клиенте из полученных данных
- Типы: провайдер degraded (success_rate < 95%), низкий баланс (< threshold), рост очереди (> 1000), новый клиент
- Сортировка по времени, цветовая кодировка: красный (critical), жёлтый (warning), синий (info)
- Максимум 10 последних

### Хук usePolling
```tsx
function usePolling(fn: () => Promise<void>, intervalMs: number): {
  pause: () => void;
  resume: () => void;
  isPaused: boolean;
  lastUpdated: Date | null;
}
```
- Вызывает fn немедленно при mount, затем каждые intervalMs
- Автоматический cleanup при unmount
- Переиспользуется в DashboardPage и MonitoringPage

---

## 3. Расширенный биллинг

### Новые табы

**Таб «Балансы»:**
- DataTable: клиент, баланс, статус (активен/заморожен), кредитный лимит, порог алерта
- Быстрые действия в строке: «Начислить», «Списать», «Заморозить/Разморозить»
- Модалка начисления/списания: тип (credit/debit/adjustment/refund), сумма, причина (обязательное поле). Создаёт транзакцию с admin_id
- Фильтры: поиск по клиенту, статус (все/активные/замороженные), баланс ниже порога

**Таб «Кредитные лимиты»:**
- DataTable: клиент, текущий баланс, кредитный лимит, доступный остаток (баланс + лимит), использование овердрафта
- Редактирование лимита inline или через модалку
- Клиент может отправлять SMS пока баланс > -credit_limit

**Существующие табы** (Транзакции, Правила цен) — без изменений, выносятся в отдельные файлы.

### Заморозка аккаунта
- Кнопка «Заморозить» в таблице балансов и на странице клиента
- При заморозке: frozen=true, frozen_at=now, frozen_by=admin_id
- Worker проверяет frozen перед списанием — если заморожен, сообщение отклоняется с ошибкой
- Разморозка — обратное действие
- Оба действия логируются в audit

### Алерты низкого баланса
- Поле low_balance_threshold на клиенте (настраивается в табе «Балансы»)
- Dashboard при каждом polling-цикле запрашивает список балансов и сравнивает с threshold клиентом — если баланс < threshold и threshold > 0, генерируется алерт
- Порог 0 = алерт отключён
- Алерты вычисляются на фронтенде, без серверного компонента нотификаций

### Изменения в БД
```sql
-- Миграция: add_billing_extensions
ALTER TABLE accounts ADD COLUMN frozen BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN credit_limit DECIMAL(15,2) NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN low_balance_threshold DECIMAL(15,2) NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN frozen_at TIMESTAMPTZ;
ALTER TABLE accounts ADD COLUMN frozen_by UUID REFERENCES users(id);
```

### Изменения в бэкенде
- **Billing service** (gRPC): `FreezeAccount`, `UnfreezeAccount`, `SetCreditLimit`, `SetLowBalanceThreshold`
- **Admin gateway** (REST):
  - `POST /admin/v1/billing/clients/{id}/freeze`
  - `POST /admin/v1/billing/clients/{id}/unfreeze`
  - `PUT /admin/v1/billing/clients/{id}/credit-limit`
  - `PUT /admin/v1/billing/clients/{id}/low-balance-threshold`
- **Worker**: проверка frozen и credit_limit перед charge

---

## 4. Тарификация

### Табы

**Таб «Тарифные планы»:**
- DataTable: название, оператор, стратегия (flat/tiered/volume), статус, количество клиентов
- CRUD через модалку: название, оператор (select из справочника), стратегия, описание
- Клик на план — раскрывает периоды и тиры этого плана

**Таб «Периоды»:**
- Фильтр по тарифному плану
- DataTable: план, дата начала, дата окончания, базовая цена, статус (active/expired/scheduled)
- CRUD: выбор плана, даты, базовая цена

**Таб «Тиры»:**
- Фильтр по тарифному плану и периоду
- DataTable: план, период, от (сегментов), до (сегментов), цена за сегмент
- Визуализация: Recharts StepChart «цена от объёма» — ступенчатый график для выбранного плана/периода
- CRUD: выбор плана+периода, диапазон, цена

**Таб «Регистрация отправителей»:**
- DataTable: sender ID, оператор, клиент, тип (paid/free), стоимость, статус, срок действия
- CRUD через модалку

### Бэкенд
API уже реализован в tarification service и admin gateway. Новых эндпоинтов не требуется:
- `GET/POST /admin/v1/tarification/plans`
- `GET/POST /admin/v1/tarification/periods`
- `GET/POST /admin/v1/tarification/tiers`
- `GET/POST /admin/v1/tarification/sender-registrations`

---

## 5. Workflow шаблонов

### Статусы
```
draft → pending → review → approved
                        ↘ revision_requested → pending
                        ↘ rejected
```

### Расширенный UI

**Список шаблонов:**
- Новые колонки: Ревьюер, Дата ревью
- Новый фильтр: статус revision_requested
- Бейджи: draft (серый), pending (жёлтый), review (синий), revision_requested (оранжевый), approved (зелёный), rejected (красный)

**Модалка ревью (TemplateReviewModal):**
- Просмотр тела шаблона с подсветкой переменных (`{name}`, `{code}` выделены цветом)
- Действия:
  - «Взять на ревью» — назначает текущего админа ревьюером, статус → review
  - «Одобрить» — статус → approved
  - «Запросить доработку» — текстовое поле с комментарием (обязательно), статус → revision_requested
  - «Отклонить» — причина обязательна, статус → rejected
- Таймлайн истории: компонент TimelineEvent, данные из audit log по resource_type=template

### Изменения в БД
```sql
-- Миграция: add_template_workflow
ALTER TABLE templates ADD COLUMN reviewer_id UUID REFERENCES users(id);
ALTER TABLE templates ADD COLUMN review_comment TEXT;
ALTER TABLE templates ADD COLUMN reviewed_at TIMESTAMPTZ;
```
Значения status: draft, pending, review, revision_requested, approved, rejected.

### Изменения в бэкенде
- **Template service** (gRPC): `AssignReviewer`, `RequestRevision`
- **Admin gateway** (REST):
  - `POST /admin/v1/templates/{id}/assign` — назначить ревьюера
  - `POST /admin/v1/templates/{id}/request-revision` — запросить доработку с комментарием
- **Portal gateway**: клиент видит review_comment и может пересоздать шаблон

---

## 6. Управление пользователями и ролями

### Страница пользователей (`/admin/users`)

**DataTable:**
| Колонка | Тип |
|---------|-----|
| username | text |
| email | text |
| Роль | Badge с названием |
| Статус | active/inactive Badge |
| 2FA | enabled/disabled |
| Последний вход | timestamp |
| Создан | timestamp |

**Действия:**
- Создать: username, email, пароль, роль (select), active
- Редактировать: роль, email, статус
- Деактивировать: soft delete, инвалидация сессий + API-ключей
- Сбросить 2FA: для потерявших доступ к TOTP
- Сбросить пароль: генерирует временный пароль

### Страница ролей (`/admin/users/roles`)

**Список ролей:**
- DataTable: название, описание, количество пользователей, встроенная (да/нет)
- Встроенные роли (admin, client, operator) нельзя удалить, но можно менять permissions

**Permission Matrix (PermissionMatrix компонент):**
- Строки = ресурсы: messages, clients, providers, routes, analytics, billing, tarification, templates, users, webhooks, countries, hlr, audit
- Колонки = действия: read, write, delete
- Чекбоксы на пересечении
- При создании/редактировании роли — интерактивная матрица
- «Выбрать все» по строке и по колонке

### Изменения в бэкенде
- **Auth service** (gRPC): `CreateUser`, `UpdateUser`, `DeactivateUser`, `ResetUser2FA`, `ResetUserPassword`, `CreateRole`, `UpdateRole`, `DeleteRole`, `ListPermissions`, `GetUserPermissions`
- **Admin gateway** (REST):
  - `GET/POST /admin/v1/users`
  - `GET/PUT/DELETE /admin/v1/users/{id}`
  - `POST /admin/v1/users/{id}/deactivate`
  - `POST /admin/v1/users/{id}/reset-2fa`
  - `POST /admin/v1/users/{id}/reset-password`
  - `GET/POST /admin/v1/roles`
  - `GET/PUT/DELETE /admin/v1/roles/{id}`
  - `GET /admin/v1/permissions`

---

## 7. Permission система (фронтенд)

### AuthContext расширение
```tsx
interface AuthState {
  user: ProfileData | null;
  role: UserRole;
  isAuthenticated: boolean;
  loading: boolean;
  isAdmin: boolean;
  permissions: Permission[];          // НОВОЕ
  hasPermission: (resource: string, action: string) => boolean;  // НОВОЕ
  login: (...) => Promise<LoginResult>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

interface Permission {
  resource: string;  // 'billing', 'clients', ...
  action: string;    // 'read', 'write', 'delete'
}
```

Permissions загружаются при логине. Auth service возвращает их вместе с профилем.

### RequirePermission компонент
```tsx
interface RequirePermissionProps {
  resource: string;
  action: string;
  children: ReactNode;
  fallback?: ReactNode;  // по умолчанию null (скрыть)
}
```
- На маршрутах: скрывает весь маршрут, показывает fallback или redirect
- На кнопках/действиях: скрывает элемент без permission
- Superadmin — всегда имеет все permissions

### usePermission хук
```tsx
function usePermission(resource: string, action: string): boolean;
```
Для императивной проверки в обработчиках.

---

## 8. Доработка существующих модулей

### Все модули
- Breadcrumbs в PageHeader
- RequirePermission обёртка на маршруте
- Переход на AdminLayout вместо UserLayout

### Клиенты
- Новые кнопки: «Заморозить баланс», «Назначить тариф»
- Ссылки на баланс и тарифный план клиента

### Мониторинг
- Рефакторинг: вынос auto-refresh логики в usePolling (переиспользуется в дашборде)

### Аудит
- Новый фильтр: resource_type
- Clickable ссылки на ресурс (клик на resource_id → переход на страницу ресурса)

### Остальные модули (Providers, Routes, Analytics, Webhooks, HLR, Countries)
- Только layout-изменения (sidebar + breadcrumbs + permission checks)

---

## 9. Новые shared-компоненты

| Компонент | Путь | Назначение |
|-----------|------|-----------|
| AdminLayout | components/layout/AdminLayout.tsx | Wrapper: compact sidebar + content area |
| AdminSidebar | components/layout/AdminSidebar.tsx | 56px→220px sidebar с группами и иконками |
| RequirePermission | components/auth/RequirePermission.tsx | Permission guard для маршрутов и элементов |
| PermissionMatrix | components/data/PermissionMatrix.tsx | Таблица чекбоксов resource×action |
| TimelineEvent | components/data/TimelineEvent.tsx | Элемент таймлайна (иконка, текст, дата, автор) |

| Хук | Путь | Назначение |
|-----|------|-----------|
| usePolling | hooks/usePolling.ts | Auto-refresh с паузой/resume |
| usePermission | hooks/usePermission.ts | Проверка прав из AuthContext |

---

## 10. Суммарные изменения в бэкенде

### Новые миграции
1. `add_billing_extensions` — frozen, credit_limit, low_balance_threshold в accounts
2. `add_template_workflow` — reviewer_id, review_comment, reviewed_at в templates

### Billing service — новые gRPC методы
- FreezeAccount, UnfreezeAccount, SetCreditLimit, SetLowBalanceThreshold

### Template service — новые gRPC методы
- AssignReviewer, RequestRevision

### Auth service — новые gRPC методы
- CreateUser, UpdateUser, DeactivateUser, ResetUser2FA, ResetUserPassword
- CreateRole, UpdateRole, DeleteRole, ListPermissions, GetUserPermissions

### Admin gateway — новые REST эндпоинты
- `/admin/v1/billing/clients/{id}/freeze|unfreeze`
- `/admin/v1/billing/clients/{id}/credit-limit`
- `/admin/v1/billing/clients/{id}/low-balance-threshold`
- `/admin/v1/templates/{id}/assign|request-revision`
- `/admin/v1/users/*` (CRUD + deactivate + reset-2fa + reset-password)
- `/admin/v1/roles/*` (CRUD)
- `/admin/v1/permissions` (list)

### Worker
- Проверка frozen и credit_limit перед charge операцией
