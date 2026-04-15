# Модерация имён отправителей и шаблонов операторов

**Дата:** 2026-04-15
**Статус:** Approved
**Область:** sender-service, portal-gateway, admin-gateway, routing-service

---

## Обзор

Переработка системы регистрации имён отправителей: двухпутевая модель для субаккаунтов и прямых аккаунтов, независимая модерация шаблонов операторов, fallback на общее имя пока идёт проверка.

**Что не работало в предыдущем дизайне:**
- `sender_names.status` — монолитный статус на всё имя, без разграничения по операторам
- `sender_registrations` в тарификации смешивал billing и регистрацию в одно
- Нет субаккаунтного пути (агрегатор как конечный модератор)
- Нет шаблонов операторов как самостоятельных сущностей
- Нет fallback-логики при routing: если имя не одобрено — блокируем, а не замещаем

---

## Два пути регистрации

```
Субаккаунт (parent_client_id IS NOT NULL):
  создать имя → sender_names.status = pending
  агрегатор одобряет → status = approved → имя ИСПОЛЬЗУЕМО
  (операторы не задействованы — зона ответственности агрегатора)

Прямой аккаунт (parent_client_id IS NULL):
  создать имя → sender_names.status = pending
  агрегатор одобряет → status = approved
  клиент выбирает операторов → operator_registrations создаются (status = submitted)
  менеджер вручную связывается с операторами, фиксирует результат в системе
  оператор одобрен → approved_type заполняется → имя ИСПОЛЬЗУЕМО у этого оператора
```

Пока имя не одобрено у конкретного оператора — routing использует `system_default_sender` вместо фактического имени. Отправка НЕ блокируется.

---

## Модель данных

### Существующие таблицы (не изменяются)

- `sender_names` — имя отправителя, статус агрегаторской модерации
- `sender_name_status_history` — история переходов статусов
- `sender_registrations` (tarification) — billing-записи (paid/free)

### Новые таблицы

#### `operator_registrations`

```sql
CREATE TABLE operator_registrations (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id    UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id       UUID         NOT NULL REFERENCES operators(id),
    registration_type VARCHAR(16)  NOT NULL CHECK (registration_type IN ('free', 'paid')),

    -- Текущий статус заявки на модерации
    status            VARCHAR(32)  NOT NULL DEFAULT 'submitted'
                          CHECK (status IN ('submitted', 'approved', 'rejected', 'revision_requested')),

    -- Снимок последнего одобренного состояния (null = ещё не было одобрения)
    approved_type     VARCHAR(16)  CHECK (approved_type IN ('free', 'paid')),
    approved_at       TIMESTAMPTZ,

    admin_note        TEXT,
    submitted_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT operator_registrations_unique UNIQUE (sender_name_id, operator_id)
);

CREATE INDEX idx_opreg_sender_name_id ON operator_registrations (sender_name_id);
CREATE INDEX idx_opreg_status          ON operator_registrations (status);
CREATE INDEX idx_opreg_operator_id     ON operator_registrations (operator_id);
```

**Ключевые поля:**
- `registration_type` — что сейчас на рассмотрении (может отличаться от `approved_type`)
- `approved_type` — последнее одобренное состояние; именно это используется в routing
- `approved_at` — когда было одобрено (null если ещё не было)

#### `operator_registration_history`

```sql
CREATE TABLE operator_registration_history (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_registration_id UUID        NOT NULL REFERENCES operator_registrations(id) ON DELETE CASCADE,
    old_status               VARCHAR(32),
    new_status               VARCHAR(32) NOT NULL,
    actor_id                 UUID        REFERENCES users(id),
    actor_type               VARCHAR(20) NOT NULL CHECK (actor_type IN ('client', 'admin', 'system')),
    comment                  TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_opreg_history_registration_id ON operator_registration_history (operator_registration_id);
```

#### `operator_templates`

```sql
CREATE TABLE operator_templates (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id    UUID         NOT NULL REFERENCES operators(id),
    name           VARCHAR(255) NOT NULL,
    body           TEXT         NOT NULL,  -- формат специфичен для каждого оператора

    status         VARCHAR(32)  NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft', 'submitted', 'approved', 'rejected', 'revision_requested')),

    admin_note     TEXT,
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
submitted ──────────────────────────► approved
                                           (approved_type = registration_type, approved_at = now)
          ──────────────────────────► rejected

          ──────────────────────────► revision_requested
                                           │
                               [клиент: resubmit]
                                           │
                                      submitted
```

При повторном submit после `revision_requested`: создаётся запись в `operator_registration_history`, статус возвращается в `submitted`. `approved_type` не меняется — старое одобренное состояние остаётся активным.

При `approved`: `approved_type = registration_type`, `approved_at = NOW()`.

### `operator_templates.status`

```
draft ──[submit]──► submitted ──► approved
                             ──► rejected
                             ──► revision_requested ──[resubmit]──► submitted
```

Шаблоны полностью независимы от `operator_registrations`. Клиент может добавлять, изменять и отправлять шаблоны в любое время, если имя в статусе `approved`.

---

## Routing-логика

```
CheckSenderAllowed(client_id, sender_name, operator_id):

1. Получить клиента:
   SELECT parent_client_id FROM clients WHERE id = client_id

2. Если субаккаунт (parent_client_id IS NOT NULL):
   SELECT status FROM sender_names
   WHERE name = sender_name AND client_id = client_id
   → status = 'approved' → allowed = true, use sender_name
   → иначе → allowed = true, use system_default_sender  ← НЕ блокируем

3. Если прямой аккаунт:
   -- Сначала проверяем что само имя одобрено агрегатором
   SELECT sn.status, or.approved_type
   FROM sender_names sn
   LEFT JOIN operator_registrations or
     ON or.sender_name_id = sn.id AND or.operator_id = operator_id
   WHERE sn.name = sender_name AND sn.client_id = client_id

   → sn.status != 'approved' → use system_default_sender  ← имя не прошло агрегатора
   → sn.status = 'approved' AND approved_type IS NOT NULL → use sender_name
   → sn.status = 'approved' AND approved_type IS NULL → use system_default_sender  ← нет operator approval
```

**Кэш (Redis):**
- Ключ: `sender:check:{client_id}:{sender_name}:{operator_id}`, TTL = 5 мин
- Инвалидация при переходе `operator_registrations.status → approved` или `sender_names.status → approved/rejected`

**Fail-open для замены имени:** если sender-service недоступен, routing использует `system_default_sender`. Сообщение не блокируется.

**`system_default_sender`** — читается из таблицы `system_settings` по ключу `default_sender_name`. Если запись отсутствует — используется `"SMS"` как хардкод fallback.

---

## API

### Portal REST API (клиент — прямые аккаунты)

**Регистрации у операторов:**
```
POST   /portal/v1/sender-names/{id}/operator-registrations        -- bulk submit
GET    /portal/v1/sender-names/{id}/operator-registrations        -- текущие статусы по операторам
POST   /portal/v1/sender-names/{id}/operator-registrations/{rid}/resubmit
```

**Шаблоны операторов:**
```
GET    /portal/v1/sender-names/{id}/operator-templates
POST   /portal/v1/sender-names/{id}/operator-templates
PUT    /portal/v1/sender-names/{id}/operator-templates/{tid}       -- только draft
POST   /portal/v1/sender-names/{id}/operator-templates/{tid}/submit
POST   /portal/v1/sender-names/{id}/operator-templates/{tid}/resubmit
DELETE /portal/v1/sender-names/{id}/operator-templates/{tid}       -- только draft
```

**Ограничение:** создавать `operator_registrations` и `operator_templates` можно только если `sender_names.status = 'approved'`.

### Admin REST API

**Модерация регистраций:**
```
GET    /admin/v1/operator-registrations                  -- очередь (фильтры: status, operator_id, client_id)
GET    /admin/v1/operator-registrations/{id}
POST   /admin/v1/operator-registrations/{id}/approve
POST   /admin/v1/operator-registrations/{id}/reject
POST   /admin/v1/operator-registrations/{id}/request-revision
```

**Модерация шаблонов операторов:**
```
GET    /admin/v1/operator-templates                  -- очередь (фильтры: status, operator_id, client_id)
GET    /admin/v1/operator-templates/{id}
POST   /admin/v1/operator-templates/{id}/approve
POST   /admin/v1/operator-templates/{id}/reject
POST   /admin/v1/operator-templates/{id}/request-revision
```

Существующие `/admin/v1/sender-names` — без изменений (агрегаторская модерация имён).

### Изменения в BulkCreateOperatorRegistrations

Текущий `BulkCreateOperatorRegistrations` создаёт записи напрямую в тарификации и списывает деньги при создании. **Изменения:**
- При submit создаётся `operator_registrations` (status = submitted), billing НЕ создаётся
- При admin approve: создаётся `sender_registration` в тарификации (для paid) и billing record
- Списание с клиента происходит в момент одобрения, не в момент подачи заявки

---

## Разграничение: шаблоны vs шаблоны операторов

| | `templates` | `operator_templates` |
|---|---|---|
| Назначение | SMS-шаблоны для кампаний | Пакет документов для регистрации у оператора |
| Привязка | `sender_name_id` (опционально) | `(sender_name_id, operator_id)` |
| Модерация | Нет (только активный/неактивный) | Да: submitted → approved/rejected |
| Формат | `{{переменные}}` | Специфичен для каждого оператора |
| Независимость | Самостоятельные | Самостоятельные от registration |

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
| `internal/gateway/portal/handlers/operator_registrations.go` | Portal REST для регистраций |
| `internal/gateway/portal/handlers/operator_templates.go` | Portal REST для шаблонов |
| `internal/gateway/admin/handlers/operator_registrations.go` | Admin REST очередь + модерация |
| `internal/gateway/admin/handlers/operator_templates.go` | Admin REST очередь + модерация |
| `internal/services/sender/storage/operator_registrations.go` | PostgreSQL репозиторий |
| `internal/services/sender/storage/operator_templates.go` | PostgreSQL репозиторий |
| `internal/services/sender/application/operator_registration_uc.go` | Use cases: submit, approve, reject |
| `internal/services/sender/application/operator_template_uc.go` | Use cases: submit, approve, reject |

### Routing (изменить)

| Путь | Изменение |
|------|----------|
| `internal/services/routing/` (CheckSenderAllowed) | Добавить `approved_type` проверку, вернуть `fallback_sender` вместо `allowed=false` |

### Frontend (portal)

| Путь | Изменение |
|------|----------|
| `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx` | Добавить секцию шаблонов оператора |
| `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx` | Показывать pending/approved состояние отдельно |
| `portal-frontend/src/api/client.ts` | Методы для operator-templates |

---

## Обработка ошибок

| Ситуация | HTTP код | Детали |
|----------|----------|--------|
| Создание operator_registration если sender_name не approved | 422 | `sender name must be approved first` |
| Создание operator_template если sender_name не approved | 422 | `sender name must be approved first` |
| Повторный submit если status не revision_requested | 400 | `resubmit only allowed from revision_requested` |
| Редактирование шаблона не в draft | 400 | `template must be in draft status` |
| Дубль (sender_name_id, operator_id) в operator_registrations | 409 | `registration for this operator already exists` |
| sender-service недоступен в routing | — | fallback: system_default_sender, не fail |

---

## Тестирование

### Unit (domain)

- Переходы статусов operator_registration: все допустимые и недопустимые
- approved_type обновляется только при approve, не при submit/resubmit
- Попытка создать registration при не-approved sender_name → ошибка

### Functional (HTTP)

- Прямой аккаунт: создать имя → одобрить (admin) → создать operator_registration → одобрить (admin) → routing использует реальное имя
- Прямой аккаунт: создать operator_registration → проверить routing → видим system_default_sender
- Субаккаунт: создать имя → одобрить (admin) → routing использует реальное имя (без operator step)
- Operator template: draft → submit → revision_requested → resubmit → approved

### Integration (routing)

- Субаккаунт, имя approved → реальное имя в сообщении
- Субаккаунт, имя pending → system_default_sender
- Прямой аккаунт, approved_type IS NOT NULL → реальное имя
- Прямой аккаунт, approved_type IS NULL → system_default_sender
- sender-service timeout → system_default_sender (fail-open)
