# Дизайн: Модерация имён отправителей и шаблонов агрегатором

**Дата:** 2026-04-15  
**Статус:** Approved

---

## Контекст

В кабинете агрегатора (`is_reseller = true`) уже реализована модерация **operator registrations** субаккаунтов (`/portal/v1/reseller/operator-registrations`). Однако возможность модерировать **имена отправителей** и **шаблоны** субаккаунтов отсутствует — эти объекты при создании субаккаунтом уходят на модерацию только к суперадмину.

Задача: дать агрегатору роль **конечного модератора** для sender names и templates своих субаккаунтов. Суперадмин в цепочку не входит.

---

## Решения

| Вопрос | Решение |
|--------|---------|
| Что модерирует агрегатор? | Имена отправителей (sender names) + шаблоны (templates) |
| UI | Две раздельные страницы в разделе агрегатора |
| Действия для sender names | Approve / Reject (с обязательной причиной; покрывает и "запросить доработку") |
| Действия для templates | Approve / Reject / Request Revision |
| Финальность | Approve агрегатора = финальный статус `approved` (суперадмин не участвует) |
| Реализация | Проверка владения SQL в portal gateway + делегация в существующие admin gRPC методы |

---

## Архитектура

### Принцип

Portal gateway для агрегатора:
1. Проверяет что агрегатор является `is_reseller = true`.
2. SQL ownership check: объект принадлежит субаккаунту с `parent_client_id = aggregator_client_id`.
3. Вызывает существующие admin gRPC методы с `actor_id` = ID портального пользователя агрегатора.

Бизнес-логика (статусные переходы, запись истории) остаётся в gRPC-сервисах без изменений.

### Ownership check SQL

```sql
-- Sender names
SELECT sn.status FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE sn.id = $1 AND c.parent_client_id = $aggregator_client_id

-- Templates
SELECT t.status FROM templates t
JOIN clients c ON c.id = t.client_id
WHERE t.id = $1 AND c.parent_client_id = $aggregator_client_id
```

---

## Бэкенд

### Новые файлы

**`internal/gateway/portal/handlers/reseller_sender_names.go`**

Структура `ResellerSenderNameHandlers`:
- Зависимости: `sendernamev1.SenderNameServiceClient`, `*pgxpool.Pool`
- Методы:
  - `ListResellerSenderNames` — SQL выборка с фильтром по статусу
  - `ApproveResellerSenderName` — ownership check → `ApproveSenderName` gRPC
  - `RejectResellerSenderName` — ownership check → `RejectSenderName` gRPC (reason обязателен)

**`internal/gateway/portal/handlers/reseller_templates.go`**

Структура `ResellerTemplateHandlers`:
- Зависимости: `templatev1.TemplateServiceClient`, `*pgxpool.Pool`
- Методы:
  - `ListResellerTemplates` — SQL выборка с фильтром по статусу
  - `ApproveResellerTemplate` — ownership check → `ApproveTemplate` gRPC
  - `RejectResellerTemplate` — ownership check → `RejectTemplate` gRPC (reason обязателен)
  - `RequestRevisionResellerTemplate` — ownership check → `RequestRevision` gRPC (comment обязателен)

### Listing SQL

```sql
-- Sender names субаккаунтов агрегатора
SELECT sn.id, sn.client_id, c.email AS sub_account_email,
       sn.name, sn.status, sn.rejection_reason, sn.created_at
FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE c.parent_client_id = $aggregator_client_id
  [AND sn.status = $status]
ORDER BY sn.created_at DESC
LIMIT 50

-- Templates субаккаунтов агрегатора
SELECT t.id, t.client_id, c.email AS sub_account_email,
       t.name, t.body, t.status, t.rejection_reason, t.created_at
FROM templates t
JOIN clients c ON c.id = t.client_id
WHERE c.parent_client_id = $aggregator_client_id
  [AND t.status = $status]
ORDER BY t.created_at DESC
LIMIT 50
```

### Новые маршруты в `router.go`

Добавляются под существующий `/reseller` subrouter:

```
GET  /portal/v1/reseller/sender-names
POST /portal/v1/reseller/sender-names/{id}/approve
POST /portal/v1/reseller/sender-names/{id}/reject

GET  /portal/v1/reseller/templates
POST /portal/v1/reseller/templates/{id}/approve
POST /portal/v1/reseller/templates/{id}/reject
POST /portal/v1/reseller/templates/{id}/request-revision
```

### Request/Response форматы

**Reject sender name:**
```json
POST /portal/v1/reseller/sender-names/{id}/reject
{ "reason": "Имя содержит запрещённые символы" }
```

**Reject template:**
```json
POST /portal/v1/reseller/templates/{id}/reject
{ "reason": "Шаблон содержит рекламный контент" }
```

**Request revision template:**
```json
POST /portal/v1/reseller/templates/{id}/request-revision
{ "comment": "Уточните тип трафика" }
```

Все action endpoints возвращают обновлённый объект с новым статусом (200 OK).

### Проверка агрегатора

В обоих обработчиках используется тот же паттерн что в `reseller_moderation.go`:
```go
var isReseller bool
pool.QueryRow(ctx, `SELECT is_reseller FROM clients WHERE id = $1`, clientID).Scan(&isReseller)
if !isReseller { // 401 }
```

### gRPC Actor ID

В action-эндпоинтах `actor_id` = `middleware.GetUserID(r.Context())` — ID портального пользователя агрегатора. Это обеспечивает корректную запись в историю статусов (actor_type = `"aggregator"`).

---

## Фронтенд

### Новые страницы

**`portal-frontend/src/pages/sub-accounts/ResellerSenderNamesPage.tsx`**

- Таблица: субаккаунт (email), имя отправителя, статус (badge), дата создания
- Фильтр по статусу: `pending` / `approved` / `rejected`
- Действия в строке (только для `pending`):
  - **Одобрить** — confirm dialog → approve API
  - **Отклонить** — модальное окно с обязательным полем "Причина"

**`portal-frontend/src/pages/sub-accounts/ResellerTemplatesPage.tsx`**

- Таблица: субаккаунт (email), название шаблона, тело (truncated до 80 символов), статус, дата
- Фильтр по статусу: `pending` / `approved` / `rejected` / `revision_requested`
- Действия в строке (для `pending` и `revision_requested`):
  - **Одобрить** — confirm dialog → approve API
  - **Отклонить** — модал с обязательным "Причина"
  - **Запросить доработку** — модал с обязательным "Комментарий"

### API методы (`portal-frontend/src/api/client.ts`)

```ts
// Sender names
listResellerSenderNames(params?: { status?: string }): Promise<{sender_names: SenderName[], total: number}>
approveResellerSenderName(id: string): Promise<SenderName>
rejectResellerSenderName(id: string, reason: string): Promise<SenderName>

// Templates
listResellerTemplates(params?: { status?: string }): Promise<{templates: Template[], total: number}>
approveResellerTemplate(id: string): Promise<Template>
rejectResellerTemplate(id: string, reason: string): Promise<Template>
requestRevisionResellerTemplate(id: string, comment: string): Promise<Template>
```

### Навигация

Две новые записи в сайдбаре в разделе агрегатора (рядом с "Модерация регистраций"):
- **"Имена отправителей"** → `/reseller/sender-names`
- **"Шаблоны"** → `/reseller/templates`

Видны только при `is_reseller = true` (как весь раздел агрегатора).

### Маршруты React Router

```
/reseller/sender-names  → ResellerSenderNamesPage
/reseller/templates     → ResellerTemplatesPage
```

---

## Ограничения области работ

- Не изменяются proto-файлы и gRPC-сервисы.
- Не изменяется статусная машина sender names (нет нового статуса `revision_requested` для них).
- Суперадмин по-прежнему видит все объекты в своей панели, но агрегатор уже вынес финальное решение.
- Пагинация в listing: фиксированный LIMIT 50 на первом этапе (аналогично operator registrations).

---

## Файлы, затрагиваемые изменениями

| Файл | Тип изменения |
|------|--------------|
| `internal/gateway/portal/handlers/reseller_sender_names.go` | Новый |
| `internal/gateway/portal/handlers/reseller_templates.go` | Новый |
| `internal/gateway/portal/router/router.go` | Изменение (новые маршруты + параметры SetupRouter) |
| `internal/gateway/portal/clients.go` | Изменение (новые gRPC клиенты и инициализация) |
| `portal-frontend/src/api/client.ts` | Изменение (новые API методы) |
| `portal-frontend/src/pages/sub-accounts/ResellerSenderNamesPage.tsx` | Новый |
| `portal-frontend/src/pages/sub-accounts/ResellerTemplatesPage.tsx` | Новый |
| `portal-frontend/src/App.tsx` или router файл | Изменение (новые маршруты) |
| Сайдбар / навигация | Изменение (новые пункты меню) |
