# Модерация имён отправителей и шаблонов операторов

**Дата:** 2026-04-15
**Статус:** Approved
**Область:** sender-service, portal-gateway, admin-gateway, aggregator-gateway, routing-service

---

## Обзор

Переработка системы регистрации имён отправителей: гранулярные заявки по операторам, двухпутевая модерация (агрегатор vs администратор), независимые шаблоны операторов, fallback на общее имя пока заявка на рассмотрении.

**Что не работало в предыдущем дизайне:**
- `sender_names.status` — монолитный статус на всё имя, без разграничения по операторам
- `sender_registrations` в тарификации смешивал billing и модерацию в одно
- Нет разделения: кто модерирует субаккаунты (агрегатор) vs прямых пользователей (admin)
- Нет шаблонов операторов как самостоятельных сущностей с lifecycle
- Нет fallback-логики при routing: если имя не одобрено — блокируем, а не замещаем

---

## Ключевые принципы

1. **Оператор — только измерение**, не участник процесса. Заявки создаются в разрезе оператора, но оператор не задействован в принятии решений на платформе.

2. **Кто модерирует:**
   - Субаккаунт (`parent_client_id IS NOT NULL`) → заявку видит **агрегатор** (родительский аккаунт)
   - Прямой пользователь (`parent_client_id IS NULL`) → заявку видит **администратор** платформы

3. **Замещение, а не блокировка:** пока заявка на рассмотрении — routing использует `system_default_sender`. Отправка не блокируется.

4. **Снимок одобренного состояния:** `approved_type` хранит последнее одобренное состояние. При повторной заявке старое состояние остаётся активным до нового решения.

---

## Два пути модерации

```
Субаккаунт (parent_client_id IS NOT NULL):
  создать имя → sender_names.status = pending
  агрегатор одобряет → status = approved

  создать заявку у оператора X → operator_registrations (status = submitted)
  агрегатор одобряет → approved_type заполняется → имя ИСПОЛЬЗУЕМО для оператора X

Прямой пользователь (parent_client_id IS NULL):
  создать имя → sender_names.status = pending
  администратор одобряет → status = approved

  создать заявку у оператора X → operator_registrations (status = submitted)
  администратор одобряет → approved_type заполняется → имя ИСПОЛЬЗУЕМО для оператора X
```

Аналогично для `operator_templates`: субаккаунты → агрегатор, прямые → admin.

---

## Модель данных

### Существующие таблицы (не изменяются)

- `sender_names` — имя отправителя, статус первичной модерации (pending/approved/rejected)
- `sender_name_status_history` — история переходов статусов имени
- `sender_registrations` (tarification-service) — billing-записи (paid/free), остаются как есть

### Новые таблицы

#### `operator_registrations`

```sql
CREATE TABLE operator_registrations (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id    UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id       UUID         NOT NULL REFERENCES operators(id),
    registration_type VARCHAR(16)  NOT NULL CHECK (registration_type IN ('free', 'paid')),

    -- Текущий статус на рассмотрении
    status            VARCHAR(32)  NOT NULL DEFAULT 'submitted'
                          CHECK (status IN ('submitted', 'approved', 'rejected', 'revision_requested')),

    -- Последнее одобренное состояние (null = одобрений ещё не было)
    approved_type     VARCHAR(16)  CHECK (approved_type IN ('free', 'paid')),
    approved_at       TIMESTAMPTZ,

    moderator_note    TEXT,         -- комментарий агрегатора или admin при решении
    submitted_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT operator_registrations_unique UNIQUE (sender_name_id, operator_id)
);

CREATE INDEX idx_opreg_sender_name_id ON operator_registrations (sender_name_id);
CREATE INDEX idx_opreg_status         ON operator_registrations (status);
CREATE INDEX idx_opreg_operator_id    ON operator_registrations (operator_id);
```

**Ключевые поля:**
- `registration_type` — тип текущей заявки (может меняться при resubmit)
- `approved_type` — последнее одобренное состояние; используется в routing
- `UNIQUE (sender_name_id, operator_id)` — одна активная заявка на пару имя-оператор

#### `operator_registration_history`

```sql
CREATE TABLE operator_registration_history (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_registration_id UUID        NOT NULL REFERENCES operator_registrations(id) ON DELETE CASCADE,
    old_status               VARCHAR(32),
    new_status               VARCHAR(32) NOT NULL,
    actor_id                 UUID        REFERENCES users(id),
    actor_type               VARCHAR(20) NOT NULL CHECK (actor_type IN ('client', 'aggregator', 'admin', 'system')),
    comment                  TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_opreg_hist_registration_id ON operator_registration_history (operator_registration_id);
```

#### `operator_templates`

```sql
CREATE TABLE operator_templates (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id    UUID         NOT NULL REFERENCES operators(id),
    name           VARCHAR(255) NOT NULL,
    body           TEXT         NOT NULL,  -- формат специфичен для оператора

    status         VARCHAR(32)  NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'revision_requested')),

    moderator_note TEXT,
    submitted_at   TIMESTAMPTZ,
    resolved_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_optpl_sender_name_id ON operator_templates (sender_name_id);
CREATE INDEX idx_optpl_operator_id    ON operator_templates (operator_id);
CREATE INDEX idx_optpl_status         ON operator_templates (status);
```

---

## Статусные машины

### `operator_registrations.status`

```
                    [агрегатор или admin]
submitted ─────────────────────────────► approved
                                              └─ approved_type = registration_type
                                                 approved_at = NOW()
          ─────────────────────────────► rejected

          ─────────────────────────────► revision_requested
                                              │
                                  [клиент: resubmit]
                                              │
                                         submitted
                              (approved_type не меняется — старое состояние активно)
```

### `operator_templates.status`

```
draft ──[клиент: submit]──► submitted ──[агрегатор или admin]──► approved
                                                              ──► rejected
                                                              ──► revision_requested
                                                                      │
                                                         [клиент: resubmit]
                                                                      │
                                                                 submitted
```

Шаблоны независимы от `operator_registrations`. Клиент может создавать и подавать шаблоны для любого оператора в любое время, если `sender_names.status = approved`.

---

## Routing-логика

```
CheckSenderAllowed(client_id, sender_name, operator_id):

1. SELECT parent_client_id, sn.status, or.approved_type
   FROM clients c
   JOIN sender_names sn ON sn.client_id = c.id AND sn.name = sender_name
   LEFT JOIN operator_registrations or
     ON or.sender_name_id = sn.id AND or.operator_id = operator_id
   WHERE c.id = client_id

2. Если sn.status != 'approved':
   → use system_default_sender  (имя не прошло первичную модерацию)

3. Если client.parent_client_id IS NOT NULL (субаккаунт):
   → sn.status = 'approved' → use sender_name  (агрегатор уже одобрил имя — достаточно)

4. Если прямой пользователь:
   → approved_type IS NOT NULL → use sender_name
   → approved_type IS NULL     → use system_default_sender
```

**Кэш (Redis):**
- Ключ: `sender:check:{client_id}:{sender_name}:{operator_id}`, TTL = 5 мин
- Инвалидация при: `sender_names.status` → approved/rejected, `operator_registrations.approved_type` обновлён

**Fail-open:** если sender-service недоступен — routing использует `system_default_sender`. Сообщение не блокируется.

**`system_default_sender`** — читается из `system_settings` по ключу `default_sender_name`. Fallback: `"SMS"`.

---

## API

### Portal REST API (все клиенты)

**Регистрации у операторов:**
```
POST   /portal/v1/sender-names/{id}/operator-registrations             -- создать заявки (bulk)
GET    /portal/v1/sender-names/{id}/operator-registrations             -- список по операторам
GET    /portal/v1/sender-names/{id}/operator-registrations/{rid}
POST   /portal/v1/sender-names/{id}/operator-registrations/{rid}/resubmit
```

**Шаблоны операторов:**
```
GET    /portal/v1/sender-names/{id}/operator-templates
POST   /portal/v1/sender-names/{id}/operator-templates
GET    /portal/v1/sender-names/{id}/operator-templates/{tid}
PUT    /portal/v1/sender-names/{id}/operator-templates/{tid}           -- только draft
DELETE /portal/v1/sender-names/{id}/operator-templates/{tid}           -- только draft
POST   /portal/v1/sender-names/{id}/operator-templates/{tid}/submit
POST   /portal/v1/sender-names/{id}/operator-templates/{tid}/resubmit
```

**Ограничение:** создание заявок и шаблонов — только при `sender_names.status = approved`.

### Aggregator REST API (агрегатор — субаккаунты)

Агрегатор видит только заявки своих субаккаунтов.

```
GET    /aggregator/v1/operator-registrations           -- очередь субаккаунтов (фильтры: status, operator_id, sub_account_id)
GET    /aggregator/v1/operator-registrations/{id}
POST   /aggregator/v1/operator-registrations/{id}/approve
POST   /aggregator/v1/operator-registrations/{id}/reject
POST   /aggregator/v1/operator-registrations/{id}/request-revision

GET    /aggregator/v1/operator-templates               -- очередь шаблонов субаккаунтов
GET    /aggregator/v1/operator-templates/{id}
POST   /aggregator/v1/operator-templates/{id}/approve
POST   /aggregator/v1/operator-templates/{id}/reject
POST   /aggregator/v1/operator-templates/{id}/request-revision
```

Существующий `/aggregator/v1/sender-names` — без изменений (первичная модерация имён субаккаунтов).

### Admin REST API (администратор — прямые пользователи)

Администратор видит только заявки прямых пользователей.

```
GET    /admin/v1/operator-registrations                -- очередь прямых (фильтры: status, operator_id, client_id)
GET    /admin/v1/operator-registrations/{id}
POST   /admin/v1/operator-registrations/{id}/approve
POST   /admin/v1/operator-registrations/{id}/reject
POST   /admin/v1/operator-registrations/{id}/request-revision

GET    /admin/v1/operator-templates
GET    /admin/v1/operator-templates/{id}
POST   /admin/v1/operator-templates/{id}/approve
POST   /admin/v1/operator-templates/{id}/reject
POST   /admin/v1/operator-templates/{id}/request-revision
```

Существующий `/admin/v1/sender-names` — без изменений.

### Billing: изменение момента списания

Текущий `BulkCreateOperatorRegistrations` создаёт billing record и списывает при submit. **Новая логика:**
- Submit → создаётся `operator_registrations` (status = submitted), billing НЕ создаётся
- Approve (агрегатор или admin) → для `paid`: создаётся `sender_registration` в тарификации + billing record + списание

---

## Разграничение: шаблоны vs шаблоны операторов

| | `templates` | `operator_templates` |
|---|---|---|
| Назначение | SMS-шаблоны для кампаний | Документы для регистрации у оператора |
| Привязка | `sender_name_id` (опционально) | `(sender_name_id, operator_id)` |
| Модерация | Нет | Да: aggregator или admin |
| Формат | `{{переменные}}` | Специфичен для оператора |

---

## Компоненты и файлы

### Новые миграции

| Файл | Содержимое |
|------|-----------|
| `migrations/000094_operator_registrations.up.sql` | `operator_registrations`, `operator_registration_history` |
| `migrations/000095_operator_templates.up.sql` | `operator_templates` |

### Backend

| Путь | Назначение |
|------|-----------|
| `internal/gateway/portal/handlers/operator_registrations.go` | Portal REST |
| `internal/gateway/portal/handlers/operator_templates.go` | Portal REST |
| `internal/gateway/aggregator/handlers/operator_registrations.go` | Aggregator модерация |
| `internal/gateway/aggregator/handlers/operator_templates.go` | Aggregator модерация |
| `internal/gateway/admin/handlers/operator_registrations.go` | Admin модерация |
| `internal/gateway/admin/handlers/operator_templates.go` | Admin модерация |
| `internal/services/sender/storage/operator_registrations.go` | PostgreSQL репозиторий |
| `internal/services/sender/storage/operator_templates.go` | PostgreSQL репозиторий |
| `internal/services/sender/application/operator_registration_uc.go` | Use cases |
| `internal/services/sender/application/operator_template_uc.go` | Use cases |

### Routing (изменить)

| Путь | Изменение |
|------|----------|
| `internal/services/routing/` CheckSenderAllowed | Двухпутевая логика, возвращать `fallback_sender` вместо `allowed=false` |

### Frontend (portal)

| Путь | Изменение |
|------|----------|
| `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx` | Секция шаблонов оператора |
| `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx` | pending/approved состояние раздельно |
| `portal-frontend/src/api/client.ts` | Методы operator-registrations, operator-templates |

---

## Обработка ошибок

| Ситуация | HTTP код | Детали |
|----------|----------|--------|
| Создание заявки при sender_name не approved | 422 | `sender name must be approved first` |
| Создание шаблона при sender_name не approved | 422 | `sender name must be approved first` |
| Resubmit если status не revision_requested | 400 | `resubmit only allowed from revision_requested` |
| Редактирование шаблона не в draft | 400 | `template must be in draft status` |
| Дубль (sender_name_id, operator_id) | 409 | `registration for this operator already exists` |
| Агрегатор пытается модерировать не своего субаккаунта | 403 | — |
| Admin пытается модерировать субаккаунта | 403 | `use aggregator panel for sub-account moderation` |
| sender-service недоступен в routing | — | fallback: system_default_sender |

---

## Тестирование

### Unit (domain)

- `approved_type` обновляется только при approve
- Resubmit из revision_requested → submitted, approved_type не меняется
- Попытка создать заявку при не-approved sender_name → ошибка

### Functional (HTTP)

- Субаккаунт: имя approved → создать заявку оператора → агрегатор approves → routing = реальное имя
- Субаккаунт: заявка submitted → routing = system_default_sender
- Прямой: имя approved → создать заявку оператора → admin approves → routing = реальное имя
- Агрегатор не может модерировать заявку прямого пользователя → 403
- Admin не может модерировать заявку субаккаунта → 403

### Integration (routing)

- Субаккаунт, имя approved, заявка approved → реальное имя
- Субаккаунт, имя approved, заявка submitted → system_default_sender
- Субаккаунт, имя pending → system_default_sender
- Прямой, approved_type IS NOT NULL → реальное имя
- Прямой, approved_type IS NULL → system_default_sender
- sender-service timeout → system_default_sender (fail-open)
