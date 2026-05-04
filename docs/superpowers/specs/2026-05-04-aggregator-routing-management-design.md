# Управление маршрутизацией в кабинете агрегатора — Design

**Дата:** 2026-05-04
**Скоуп:** Полноценный раздел «Сеть» в портале агрегатора: каталог провайдеров, шаблоны provider-set / route-set, назначения суб-аккаунтам, override-механика. Заменяет существующую страницу `/network/routing` (bulk-assign + read-only вкладки маршрутов).

**Single source of truth для writing-plans.**

---

## 1. Контекст и проблема

Текущая страница `/network/routing` ([NetworkRoutingPage.tsx](portal-frontend/src/pages/network/NetworkRoutingPage.tsx)) закрывает только один сценарий — массовое назначение провайдера на чекбоксы суб-аккаунтов. Не закрывает: индивидуальное управление, отзыв назначения, редактирование приоритета, управление маршрутами (вкладка «Маршруты» косметически мёртвая после миграции на `route_condition_groups` — `country_id` всегда NULL).

Кроме того, селект провайдеров для bulk-операции собран из уже-назначенных провайдеров ([NetworkRoutingPage.tsx:62-72](portal-frontend/src/pages/network/NetworkRoutingPage.tsx#L62-L72)); основной use-case «выдать новый провайдер всем» не работает, фронт падает на инпут UUID.

Цель — заменить эту страницу полноценным разделом, в котором агрегатор управляет провайдерами и маршрутами как для всех суб-аккаунтов сразу (через шаблоны), так и индивидуально (через override).

---

## 2. Зафиксированные решения брейншторма

| # | Развилка | Выбор |
|---|----------|-------|
| 1 | Inheritance-модель | **Templates + assign:** агрегатор делает шаблоны, явно назначает суб-аккаунтам; изменения шаблона распространяются на подписанных |
| 2 | Гранулярность шаблонов | **Раздельно provider-set и route-set;** валидация: route-set не может ссылаться на провайдер вне provider-set'а назначенного суб-аккаунта |
| 3 | Каталог провайдеров | **Платформенные + private:** агрегатор может выбирать из глобальных или добавлять своих с собственными SMPP-кредами; `inherited` (двухуровневая иерархия) — read-only пометка, без управления |
| 4 | Структура раздела | **Гибрид:** «Сеть» в сайдбаре (каталог, provider-sets, route-sets, назначения) + быстрый доступ из карточки суб-аккаунта (текущие настройки + override-форма) |
| 5 | Редактор маршрута | **Список + Drawer:** таблица с drag для приоритета, клик → drawer справа с полным редактором; превью «куда пойдёт сообщение», inline-статус, дубликат маршрута |
| 6 | Страница назначений | **Таблица + bulk-panel:** чекбоксы, sticky-bar при выделении, поиск/фильтр, inline-select; колонки Override и Валидация; smart-default для новых суб-аккаунтов |

---

## 3. Модель данных

### 3.1 Новые таблицы

**`reseller_provider_sets`** — шаблон каталога провайдеров.

| Колонка | Тип | Примечание |
|---|---|---|
| `id` | UUID PK | |
| `reseller_id` | UUID FK → `clients(id)` | владелец-агрегатор |
| `name` | VARCHAR(100) | |
| `is_default` | BOOLEAN | назначается автоматически новому суб-аккаунту |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

Индексы: UNIQUE(`reseller_id`, `name`); partial UNIQUE WHERE `is_default = true` по `reseller_id` (один default на агрегатора).

**`reseller_provider_set_items`** — содержимое provider-set.

| Колонка | Тип | Примечание |
|---|---|---|
| `id` | UUID PK | |
| `set_id` | UUID FK → `reseller_provider_sets(id)` ON DELETE CASCADE | |
| `provider_id` | UUID FK → `providers(id)` ON DELETE RESTRICT | |
| `priority` | INT NOT NULL DEFAULT 0 | переезжает в `client_providers.shared_priority` при назначении |
| `expose_cost` | BOOLEAN NOT NULL DEFAULT false | |
| `expose_provider_name` | BOOLEAN NOT NULL DEFAULT true | |

Индексы: UNIQUE(`set_id`, `provider_id`); INDEX(`provider_id`).

**`reseller_route_sets`** — шаблон маршрутов. Поля идентичны `reseller_provider_sets`.

**`reseller_route_set_items`** — правила маршрутизации в шаблоне (зеркало `client_routes` без `client_id`).

| Колонка | Тип | Примечание |
|---|---|---|
| `id` | UUID PK | |
| `set_id` | UUID FK → `reseller_route_sets(id)` ON DELETE CASCADE | |
| `name` | VARCHAR(255) | |
| `comment` | TEXT | |
| `provider_id` | UUID FK → `providers(id)` ON DELETE RESTRICT | |
| `priority` | INT NOT NULL DEFAULT 0 | порядок применения |
| `share` | INT NOT NULL DEFAULT 100 | |
| `route_type` | VARCHAR(10) NOT NULL DEFAULT 'sms' | |
| `status` | VARCHAR(20) NOT NULL DEFAULT 'active' | |

**`route_set_condition_groups`**, **`route_set_conditions`**, **`route_set_schedules`** — точные копии существующих `route_condition_groups`, `route_conditions`, `route_schedules` (см. [миграцию 000077](migrations/000077_routing_overhaul.up.sql)), с FK на `reseller_route_set_items.id` вместо `client_routes.id`.

**`subaccount_routing_assignment`** — привязка суб-аккаунта к шаблонам.

| Колонка | Тип | Примечание |
|---|---|---|
| `client_id` | UUID PK FK → `clients(id)` ON DELETE CASCADE | |
| `provider_set_id` | UUID NULL FK → `reseller_provider_sets(id)` ON DELETE SET NULL | |
| `route_set_id` | UUID NULL FK → `reseller_route_sets(id)` ON DELETE SET NULL | |
| `assigned_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |

Pre-validation в коде: `set.reseller_id = clients[client_id].parent_client_id` (Postgres CHECK с подзапросом не поддерживается).

### 3.2 Изменения существующих таблиц

**`client_routes`:** добавляется колонка `source` ENUM `('template','override')`. Семантика:
- `source='template'` — материализовано из `reseller_route_set_items`. Стирается при смене route-set'а
- `source='override'` — индивидуальное правило этого суб-аккаунта. **Не стирается** при смене шаблона

Существующие записи мигрируются в `source='template'` (или `'override'` если `client_id IS NOT NULL` и нет соответствующего route-set назначения — в момент миграции у нас вообще нет route-set'ов, всё мигрирует в `'override'`, агрегатор решит позже).

**`client_providers.ownership`:** семантика уточняется без изменения схемы:
- `'inherited'` — материализовано из `reseller_provider_set_items`
- `'private'` — override (индивидуальный провайдер этого суб-аккаунта)
- `'platform'` — глобальный (для самостоятельных клиентов вне reseller-иерархии)

### 3.3 Применение шаблона: материализация

**Решение:** материализация в существующие `client_providers` / `client_routes` (синхронно, под транзакцией). Pipeline и роутер читают из тех же таблиц что и сейчас, hot-path не меняется.

При назначении/изменении/смене шаблона:
1. BEGIN
2. Pre-validation (см. §6)
3. UPSERT `subaccount_routing_assignment`
4. DELETE `client_providers WHERE client_id=X AND ownership='inherited'`
5. INSERT новые из `reseller_provider_set_items` с `ownership='inherited'`, `shared_priority=item.priority`
6. DELETE `client_routes WHERE client_id=X AND source='template'` (CASCADE на condition_groups, conditions, schedules)
7. INSERT новые из `reseller_route_set_items` с `source='template'`, copy condition_groups / conditions / schedules
8. COMMIT
9. Audit-log (вне транзакции)

`client_providers WHERE ownership='private'` и `client_routes WHERE source='override'` — НЕ затрагиваются.

**Жертвы материализации:**
- При изменении шаблона нужно перезаписать у всех подписанных. До 50 суб-аккаунтов — синхронно одной транзакцией; больше — per-client цикл с per-client транзакциями. Фоновый job — отложено, добавляем при появлении пейн-поинта
- Дублирование схемы условий маршрутов (`route_set_condition_groups` повторяет `route_condition_groups`). Альтернатива — UNION-таблица с дискриминатором — хуже для типобезопасности. Принимаем дублирование

### 3.4 Двухуровневая иерархия (`inherited`)

ENUM `ownership='inherited'` существует для двухуровневой иерархии (главный реселлер → дочерний реселлер → клиент). В первой итерации:
- Если у суб-аккаунта уже есть `client_providers` с `ownership='inherited'` И `source_client_id != current resellerID` — отображаем как read-only, без управления
- Не строим UI для двухуровневых reseller-цепочек. Отдельная история

---

## 4. API

Все endpoints под `/portal/v1/reseller/network/`. Под middleware `ResellerOnlyMiddleware` (см. [router.go:463](internal/gateway/portal/router/router.go#L463)). Существующий `/portal/v1/reseller/routing/*` (bulk-assign, list-providers, list-routes) удаляется в той же миграции.

### 4.1 Каталог провайдеров

```
GET    /reseller/network/providers
       → [{id, name, ownership, smpp_host, smpp_port, system_id, system_type, active}]
       Содержит: глобальные platform-провайдеры + private-провайдеры этого reseller'а.

POST   /reseller/network/providers
       body: {name, smpp_host, smpp_port, system_id, password, system_type, ...}
       → создаёт private-провайдера (ownership='private', source_client_id=resellerID).

PUT    /reseller/network/providers/{id}
       body: те же поля
       → редактирует private. Platform — 403.

DELETE /reseller/network/providers/{id}
       → 409 если используется в provider-set этого reseller'а или в client_providers (override).
```

### 4.2 Provider-sets

```
GET    /reseller/network/provider-sets
       → [{id, name, is_default, item_count, assigned_count}]

POST   /reseller/network/provider-sets         body: {name, is_default}
PUT    /reseller/network/provider-sets/{id}    body: {name, is_default}
DELETE /reseller/network/provider-sets/{id}    → 409 если назначен суб-аккаунту

GET    /reseller/network/provider-sets/{id}/items
       → [{id, provider_id, provider_name, priority, expose_cost, expose_provider_name}]

PUT    /reseller/network/provider-sets/{id}/items
       body: [{provider_id, priority, expose_cost, expose_provider_name}, ...]
       → атомарный bulk-replace. Pre-validate: удаляемые провайдеры не используются
         в route-set'ах подписанных суб-аккаунтов или в их override-маршрутах.
       → после успеха: re-materialize client_providers для всех подписанных.
```

Решение: PUT-replace всего списка вместо POST/DELETE/PUT по одному. Drag-and-drop приоритетов всё равно отправляет полный список; меньше round-trip'ов; проще атомарность.

### 4.3 Route-sets

```
GET    /reseller/network/route-sets
       → [{id, name, is_default, item_count, assigned_count}]

POST   /reseller/network/route-sets         body: {name, is_default}
PUT    /reseller/network/route-sets/{id}    body: {name, is_default}
DELETE /reseller/network/route-sets/{id}    → 409 если назначен

GET    /reseller/network/route-sets/{id}/items
       → [{id, name, comment, provider_id, priority, share, route_type, status,
           condition_groups: [{logic_op, conditions: [{type, value}]}],
           schedules: [{date_from, date_to, time_from, time_to, weekdays, timezone}]}]

POST   /reseller/network/route-sets/{id}/items
       body: {name, comment, provider_id, priority, share, route_type,
              condition_groups, schedules}
       → создаёт правило. Pre-validate: provider_id должен быть в provider-set
         каждого подписанного суб-аккаунта.

PUT    /reseller/network/route-sets/{id}/items/{item_id}    → редактирует целиком, та же валидация
DELETE /reseller/network/route-sets/{id}/items/{item_id}

POST   /reseller/network/route-sets/{id}/items/{item_id}/duplicate
       → копирует с новым именем и инкрементом priority

PUT    /reseller/network/route-sets/{id}/items/reorder
       body: [{item_id, priority}, ...]
       → bulk-update приоритетов для drag-and-drop.

POST   /reseller/network/route-sets/{id}/preview
       body: {phone, sender_id, traffic_type}
       → симуляция: возвращает [{matched_item_id, item_name, provider_id, provider_name, priority}]
         в порядке приоритета. Учитывает условия и расписание (текущее время по умолчанию).
         Без отправки.
```

### 4.4 Назначения суб-аккаунтам

```
GET    /reseller/network/assignments
       → [{client_id, sub_account_name, provider_set_id, provider_set_name,
           route_set_id, route_set_name, has_overrides, validation_status, validation_error}]
       validation_status: 'ok' | 'conflict' | 'unassigned'

PUT    /reseller/network/assignments/{client_id}
       body: {provider_set_id, route_set_id}  (любое = null = снять)
       → транзакция: UPSERT assignment + pre-validate + re-materialize.
       → 409 при конфликте.

POST   /reseller/network/assignments/bulk
       body: {client_ids: [...], provider_set_id, route_set_id}
       → per-client цикл, не атомарно.
       → response: {results: [{client_id, status: 'ok' | 'conflict' | 'error', error?: ...}]}

POST   /reseller/network/assignments/bulk/dry-run
       body: {client_ids: [...], provider_set_id, route_set_id}
       → возвращает прогноз: у скольких будет конфликт без записи.
       → используется фронтом для подсветки в UI до bulk-операции.
```

### 4.5 Override-endpoints (контекстный из карточки суб-аккаунта)

```
GET    /sub-accounts/{id}/network/overview
       → {provider_set: {...}, route_set: {...},
          provider_overrides: [{provider_id, name, priority, ownership}],
          route_overrides: [{...}]}

POST   /sub-accounts/{id}/network/provider-overrides
       body: {provider_id, priority, expose_cost, expose_provider_name}
       → INSERT client_providers с ownership='private'.
       → 409 если этот provider_id уже есть как ownership='inherited' (доступен через шаблон).
         Сообщение: «Этот провайдер уже доступен через шаблон. Чтобы изменить параметры —
         измени provider-set или сделай новый шаблон для этого суб-аккаунта.»

DELETE /sub-accounts/{id}/network/provider-overrides/{provider_id}

POST   /sub-accounts/{id}/network/route-overrides
       body: {name, provider_id, priority, condition_groups, schedules, ...}
       → INSERT client_routes с source='override'.

PUT    /sub-accounts/{id}/network/route-overrides/{route_id}
DELETE /sub-accounts/{id}/network/route-overrides/{route_id}
```

### 4.6 Что НЕ предусмотрено

- **Версионирование шаблонов / откат** — YAGNI. Аудит-логирование изменений идёт в существующий `audit_log`
- **WebSocket / live-updates таблицы назначений** — на 50 суб-аккаунтов polling/refresh достаточно
- **Дифф-предпросмотр** «что изменится при применении нового шаблона» — отдельная большая фича, отложено
- **Экспорт/импорт JSON** шаблонов — отложено

### 4.7 Жертвы

- **Атомарная транзакция для bulk на 100+ — долгий запрос.** Лимит: до 50 одной транзакцией; больше — per-client цикл. Жертвую атомарностью bulk'а ради простоты
- **Pre-validation provider-set ↔ route-set делается в коде, не в SQL CHECK.** Postgres CHECK не поддерживает кросс-таблицы. Цена: можно вставить «битое» назначение через прямой SQL. Доверяем единственной точке записи
- **Override priority унаследованного провайдера не поддерживается.** Если хочется изменить priority конкретного унаследованного провайдера у одного суб-аккаунта — нужно сделать отдельный provider-set (с другим priority) и назначить его. Альтернатива (поле `client_providers.override_priority` или upgrade `inherited→private`) ломает простую материализацию. Цена: больше шаблонов; защищает от запутывания «откуда пришёл этот priority»

---

## 5. UI и компоненты

### 5.1 Изменения навигации

Текущий пункт «Маршрутизация» в группе «Сеть» сайдбара заменяется четырьмя подпунктами:

```
СЕТЬ
  Дашборд сети
  Суб-аккаунты
  ──────────────
  Провайдеры          (NEW — каталог reseller'a)
  Provider-sets       (NEW)
  Route-sets          (NEW)
  Назначения          (REPLACES «Маршрутизация»)
  ──────────────
  Тарифы
  Статистика
```

Текущая `NetworkRoutingPage.tsx` удаляется. Route `/network/routing` редиректит на `/network/assignments`.

### 5.2 Новые страницы

**`/network/providers` — Каталог провайдеров агрегатора**
- Таблица: Имя | Ownership (badge `Платформенный` / `Свой`) | SMPP host | Active | Действия
- «+ Добавить своего провайдера» → drawer с формой SMPP-кред
- Platform read-only; private — через drawer
- DELETE private — modal-confirm с проверкой usage

Компонент: `pages/network/ProvidersCatalogPage.tsx` + `ProviderForm.tsx` (новый или выделенный).

**`/network/provider-sets` — Список и редактор Provider-set'ов** (master-detail)
- Левая колонка: список set'ов (имя, badge `default`, count items, count assigned), «+ Создать»
- Правая колонка: содержимое выбранного set'а
  - Заголовок: имя (inline rename), чекбокс «По умолчанию для новых суб-аккаунтов»
  - Таблица: Имя | Приоритет (drag) | Expose cost ☑ | Expose name ☑ | Удалить
  - «+ Добавить провайдера» → modal с searchable select
  - PUT items (bulk-replace) — autosave при изменении строки или sticky «Сохранить»

Компонент: `pages/network/ProviderSetsPage.tsx`.

**`/network/route-sets` — Список и редактор Route-set'ов** (master-detail)
- Левая колонка: список set'ов, как у provider-sets
- Правая колонка: редактор выбранного route-set'а
  - Заголовок: имя, чекбокс default
  - **Превью-блок** (collapsible): поля phone/sender/traffic_type → POST `/preview` → подсветка матча
  - Таблица правил с drag-handle:
    - № | Имя | Условия (краткая сводка) | Провайдер | Доля | Активен (toggle inline) | Дубль | Удалить
  - Клик по строке → выезжает Drawer с полным редактором правила
  - «+ Правило» → пустой Drawer

Компонент: `pages/network/RouteSetsPage.tsx` + `RouteRuleDrawer.tsx` + `RouteConditionsEditor.tsx` + `RouteSchedulesEditor.tsx`.

**`/network/assignments` — Таблица назначений**
- Заголовок + dropdown «Создать шаблон» (provider-set / route-set)
- **Bulk-panel** (sticky, появляется при выделении): «Выбрано N | Назначить provider-set ▼ | Назначить route-set ▼ | Сбросить»
- Поиск по имени + фильтр по шаблону
- Таблица: ☐ | Суб-аккаунт (link → карточка) | Provider-set (inline-select) | Route-set (inline-select) | Override-индикатор | Валидация (✓/⚠ tooltip) | Действия
- Inline-select на изменение → PUT `/assignments/{client_id}` с spinner + toast результата

Компонент: `pages/network/AssignmentsPage.tsx`.

### 5.3 Изменения в карточке суб-аккаунта

Добавляется секция/вкладка «Сеть» (зависит от текущей структуры карточки):
- Текущий provider-set / route-set с кнопкой «Изменить» (modal)
- «Кастомные провайдеры» (overrides ownership='private') — таблица + добавить
- «Кастомные маршруты» (source='override') — таблица + добавить, drawer-редактор как у route-set
- Плашка над override-секциями: «Эти настройки переопределяют шаблон. Изменения шаблона не затронут эти записи»

Компонент: `pages/sub-accounts/SubAccountNetworkTab.tsx` (новый), реюзает `RouteRuleDrawer` и `ProviderForm`.

### 5.4 Общие компоненты

- `Drawer.tsx` — существующий
- `RouteRuleDrawer.tsx` — общий для route-sets и override-маршрутов в карточке
- `RouteConditionsEditor.tsx` — груп условий с logic_op IF/AND/AND_NOT/OR/OR_NOT, строки `[type ▼] [value]`
- `RouteSchedulesEditor.tsx` — даты, время, weekdays bitmask, timezone
- `ProviderSelect.tsx` — searchable select по доступным провайдерам
- `SubAccountSelect.tsx` — для bulk-операций
- `ValidationBadge.tsx` — ✓/⚠/— с tooltip

### 5.5 Жертвы UI

- **Master-detail** ломается на узких экранах (≤1280px). Принимаю — портал desktop-first
- **Inline-select autosave** в assignments → confirm только при destructive change
- **Drawer для route rule** не вместит 10+ групп условий. Typical 1-3, на больше — drawer scrollable
- **Нет flow-builder'а** (вариант D из вопроса 5 был отклонён). Приоритет = порядок применения, поток показывается через preview
- **Override-механика в карточке** = два места правки концептуально одного. Решается явной плашкой о том, что override не пересоздаётся при смене шаблона

---

## 6. Валидация и обработка ошибок

### 6.1 Главный инвариант

**Не существует записи `client_routes` (любого `source`), которая ссылается на `provider_id`, отсутствующий в `client_providers` того же `client_id`.**

Все валидации существуют, чтобы этот инвариант невозможно было нарушить через портал.

### 6.2 Точки валидации

**Точка 1: PUT items provider-set**
Удаляемые провайдеры не должны использоваться в route-set'ах подписанных суб-аккаунтов или в их override-маршрутах. Иначе **409** с `details: [{client_id, client_name, route_name}]`.

UX: modal с тремя действиями — Отменить (default) / Удалить конфликтующие маршруты (cascade-cleanup) / Открыть конфликтный маршрут.

**Точка 2: POST/PUT route-set item**
Для каждого подписанного суб-аккаунта проверить, что `provider_id` правила есть в его provider-set. Иначе **409** с `details: [{client_id, client_name, current_provider_set, missing}]`.

UX: в селекте провайдеров правила фронт подсвечивает доступность. До сохранения — pre-check.

**Точка 3: PUT assignment**
Все `provider_id` в правилах нового route-set должны быть в новом provider-set. Иначе **409** с предложением сменить пару.

UX: bulk-panel делает dry-run-валидацию через `POST /assignments/bulk/dry-run` и подсвечивает, у скольких будет конфликт. Bulk применяется per-client; конфликтные пропускаются с per-client error в response.

**Точка 4: POST override-маршрута в карточке суб-аккаунта**
`provider_id` должен быть в provider-set суб-аккаунта (через `inherited`) или в его private-провайдерах (override level). Иначе **409**.

UX: селект провайдеров override-формы фильтруется до доступных.

**Точка 5: DELETE private-провайдера**
- Используется в provider-set этого reseller'а — **409**
- Используется в `client_providers` с `ownership='private'` — **409**
- Только если нигде не используется — DELETE

### 6.3 Формат 409-ответов

Существующий `shared.ErrConflict` (см. `internal/shared/errors.go`):

```json
{
  "error": {
    "code": "ROUTING_VALIDATION_CONFLICT",
    "message": "...",
    "details": {
      "kind": "provider_used_in_routes" | "route_uses_unavailable_provider" | ...,
      "items": [...]
    }
  }
}
```

Единый `code` — фронт открывает соответствующий modal с предложениями cleanup'а.

### 6.4 Транзакционность

**Применение шаблона = транзакция per client_id** (см. §3.3 шаги 1-9). При сбое внутри — ROLLBACK, 500 с request_id.

**Bulk-операция = независимые транзакции:**
- Per client цикл, каждый — отдельная транзакция
- Один сбой не откатывает остальных
- Response: `{results: [{client_id, status, error?}]}`
- Toast: «Назначено N / M. Конфликты: K (показать)»

### 6.5 Race-conditions

**Не реализуем** optimistic concurrency в первой итерации. Соло-разработчик, агрегатор обычно один человек, две вкладки — экзотика. Если в проде проявится — добавим `If-Unmodified-Since` + 412.

### 6.6 Защита от ownership

Каждый endpoint **дополнительно** к `ResellerOnlyMiddleware`:
- `/provider-sets/{id}` и `/route-sets/{id}`: `set.reseller_id = current_client_id` → иначе **404** (не 403, чтобы не утекало существование чужих ID)
- `/assignments/{client_id}`: `clients[client_id].parent_client_id = current_client_id` → **404**
- `/providers/{id}` (DELETE/PUT): `provider.ownership = 'private' AND provider.source_client_id = current_client_id` → **403** для platform или **404** для чужого private

Дублирование с middleware допустимо: при работе с ID в URL middleware-only недостаточно.

### 6.7 Аудит-лог

Все изменения логируются в существующий `audit_log` (см. `internal/services/audit/`):
- `routing.provider_set.created` / `.updated` / `.deleted` / `.items.replaced` (с diff old/new)
- `routing.route_set.*` аналогично
- `routing.assignment.changed` (client_id, old set_ids, new set_ids)
- `routing.bulk_assign` (count, success_count, conflict_count)

После COMMIT, не внутри транзакции.

### 6.8 Жертвы валидации

- Без optimistic concurrency — описано в §6.5
- Cleanup orphan-маршрутов как явный отдельный POST, не cascade-delete автоматически. Цена: лишний клик. Защищает от молчаливой потери конфигурации
- Bulk не атомарен — описано в §6.4
- Audit-log без UI в первой итерации — данные пишутся, страница «История изменений» позже

---

## 7. Тестирование

### 7.1 Уровни

**Unit (`internal/gateway/portal/handlers/*_test.go`)**
- Pre-validation в `PutProviderSetItems`: 409 с конкретным списком конфликтов
- Pre-validation в `CreateRouteSetItem`: 409 при provider_id вне provider-set
- Pre-validation в `PutAssignment`: 409 при паре set'ов с конфликтом
- Ownership-проверки: чужой `set_id` → 404; private чужого reseller'а → 404
- Иерархия: `set.reseller_id = parent_client_id` суб-аккаунта

Подход: testify, мок-репозиторий через interface. Каждый случай 409 = отдельный тест-кейс.

**Integration (`tests/integration/*` или `*_integration_test.go`)**
- Транзакционная материализация: после assignment в `client_providers` появились `ownership='inherited'`, в `client_routes` — `source='template'`, condition_groups скопированы
- DELETE на provider-set с активными assignments → 409
- Refresh при изменении шаблона: items обновились у всех подписанных
- Override-сохранение: материализация не трогает `ownership='private'` и `source='override'`
- Cascade на DELETE route-set: items, condition_groups, conditions, schedules удалились
- Race на bulk: 1 из 50 конфликтует, остальные применились, response per-client
- Inheritance не путается: агрегатор B не может затронуть суб-аккаунт A

**E2E (`e2e/tests/reseller/network-routing.spec.ts`)**
Расширяем существующий файл, который сейчас покрывает только bulk-assign. Используем `e2e/helpers/api.ts` и `e2e/pages/`.

Сценарии:
- Полный flow: создать provider-set → добавить 2 провайдера → создать route-set → правило → назначить обоим суб-аккаунтам → проверить в карточке
- Override: добавить private-провайдера в карточке → видеть индикатор Override в `/network/assignments`
- Конфликт-flow: удаление провайдера → modal → «Открыть конфликтный маршрут»
- Bulk с частичным успехом: 4 суб-аккаунта (1 конфликт) → toast 3/4 → детали
- Preview: ввести phone/sender → подсветка ожидаемого правила

**Smoke на сервере**
После деплоя — ручной прогон через `aggregator@test.local` (см. memory `project_sandbox_test_credentials`). Сценарии те же, что E2E. Для invalid-номеров — см. memory `feedback_invalid_numbers_only`.

### 7.2 Тестовая инфраструктура

Новые фикстуры в `internal/testutil/`:
- `SeedReseller(pool)` → reseller_id + 2 суб-аккаунта
- `SeedProviderSet(pool, resellerID, items)` → set_id
- `SeedRouteSet(pool, resellerID, items)` → set_id
- `AssignSets(pool, clientID, providerSetID, routeSetID)` → создаёт assignment + материализует

E2E page-objects:
- `e2e/pages/ProvidersCatalogPage.ts`
- `e2e/pages/ProviderSetsPage.ts`
- `e2e/pages/RouteSetsPage.ts`
- `e2e/pages/AssignmentsPage.ts`
- Расширение `NetworkRoutingPage.ts` для редиректа

### 7.3 Метрики

- Минимальное покрытие новых handler'ов: 80% (`go cover`)
- Все новые `*_test.go` идут в `./scripts/check.sh --with-tests` и в CI
- ESLint baseline (69 warnings) не должен расти. Новый фронт-код — без `any`, корректные deps в `useEffect`
- Page-объекты строго в `e2e/pages/`

### 7.4 Что не делаем

**Property-based testing** (gopter / quick) для конфликтов provider-set ↔ route-set — overkill. 5-7 explicit-кейсов покрывают типовые конфликты.

### 7.5 Жертвы тестирования

- Material-state проверка только на integration-уровне (моки бесполезны для SQL)
- E2E — только 5 ключевых сценариев. API покрывается integration'ом
- Пайплайн / роутер не тестируем заново. Убеждаемся через integration, что таблицы после материализации содержат правильные строки; поведение пайплайна на этих строках уже покрыто отдельными тестами

---

## 8. Миграция и порядок выкатывания

### 8.1 БД-миграции

1. `NNNNNN_create_reseller_provider_sets.up.sql` — таблицы `reseller_provider_sets`, `reseller_provider_set_items`
2. `NNNNNN_create_reseller_route_sets.up.sql` — `reseller_route_sets`, `reseller_route_set_items`, `route_set_condition_groups`, `route_set_conditions`, `route_set_schedules`
3. `NNNNNN_create_subaccount_routing_assignment.up.sql`
4. `NNNNNN_add_client_routes_source.up.sql` — колонка `source`, миграция существующих в `'override'`

Все миграции с idempotent-DDL (`IF NOT EXISTS`), down-миграции для отката.

### 8.2 Порядок выкатывания фронта/бэка

1. Миграции БД
2. Новые backend-endpoints (`/reseller/network/*`) + тесты, **старые `/reseller/routing/*` остаются** для совместимости
3. Новые frontend-страницы + сайдбар, **новый редирект `/network/routing` → `/network/assignments`**
4. Удаление старого `NetworkRoutingPage.tsx` + удаление старых backend-endpoints

После шага 4 — ручной smoke на сервере.

### 8.3 Откат

Все миграции имеют `.down.sql`. При критическом баге:
- Down-миграция в обратном порядке (4→3→2→1)
- Старые endpoint'ы остаются (см. §8.2 шаг 2-3 — они удаляются только на шаге 4); до шага 4 откат фронта = возврат к старой странице

---

## 9. Не-функциональные требования

- **Latency:** все CRUD на шаблоны ≤200ms p95 (нет hot-path)
- **Material-bulk:** до 50 суб-аккаунтов под одну транзакцию ≤2s; больше — индикатор прогресса в UI, per-client транзакции
- **БД-нагрузка:** при изменении шаблона — pre-validation одним запросом с JOIN'ами, не N+1
- **Безопасность:** все endpoint'ы под session-auth + CSRF + ResellerOnlyMiddleware + per-handler ownership-check (см. §6.6); никаких URL-leak'ов чужих ID

---

## 10. Открытые вопросы (на этапе плана)

- Есть ли уже существующий `Drawer.tsx` в `portal-frontend/src/components/ui/`, или его надо реализовать. Если есть — переиспользуем; если нет — добавляем как часть этого скоупа
- Существующий компонент `ProviderForm` (для глобального каталога админ-панели) — переиспользуем 1-в-1 или форкаем под reseller-флавор
- Конкретное место «Сеть»-вкладки в карточке суб-аккаунта (зависит от текущей структуры — вкладки/одностраничная карточка)

Эти вопросы решает writing-plans — не блокируют дизайн.
