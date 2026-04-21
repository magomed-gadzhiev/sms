# Многоуровневая маршрутизация: дизайн

**Дата:** 2026-04-21
**Автор:** brainstorm (user + Claude)
**Статус:** draft, ждёт ревью пользователя
**Связанные документы:** tarification-модель (memory `project_aggregator_decisions`), ревью routing 2026-04-21

## Проблема

Система маршрутизации (`internal/services/routing/`) построена на плоской client-based модели: одна колонка `client_routes.client_id`, `NULL` = default. Tarification уже перешла на unified owner-based иерархию с тремя уровнями (`platform / aggregator / subaccount`) и specificity-ранжированием. Routing такой иерархии **не имеет**:

- Нет `owner_type`/`owner_id`.
- Fallback бинарный: `client_routes` ИЛИ `default_routes`. Нет цепочки subaccount→aggregator→platform.
- Нет ранжирования по специфичности; только `priority int` внутри одного уровня.
- Portal API разведён по ролям, но не умеет управлять правилами уровня агрегатора для суб-аккаунтов.
- Тестов многоуровневой иерархии нет.

Цель — привести routing к той же модели, что и tarification, с семантикой, адаптированной под специфику маршрутизации (правило = набор провайдеров с failover, а не одно значение).

## Ключевые решения

### 1. Уровни иерархии

Три уровня: `platform / client / subaccount`.

- `platform` — глобальные правила, singleton для general, применяются ко всем, если ничего более частного не нашлось.
- `client` — правила клиента: либо обычного («user»), либо реселлера с суб-аккаунтами. Различия нет — определяется по наличию детей в `clients.parent_client_id`.
- `subaccount` — правила суб-аккаунта реселлера.

«User» в исходной формулировке пользователя = client без суб-аккаунтов. Отдельного уровня не вводим.

### 2. Модель владения

```sql
CREATE TYPE route_owner_type AS ENUM ('platform','client','subaccount');
```

Колонки `owner_type NOT NULL`, `owner_id UUID NULL` (NULL только при `owner_type='platform'`).

### 3. Ключ специфичности

Специфичность правила считается по заполнённости трёх полей:

- `operator_id`
- `country_code`
- `traffic_type`

Пустое (NULL) поле = wildcard. Чем больше заполнено — тем специфичнее правило.

`paid_name`, `regex`, `schedule_id` — дополнительные **фильтры** внутри правила. Они участвуют в матчинге (должны совпасть при заполнении), но **не участвуют в ранжировании специфичности**. Это компромисс: они используются редко и обычно как модификатор к ключу, а не как основа.

### 4. Семантика наследования

**Override = replace cell.** Если на уровне суб-аккаунта найдено подошедшее правило, провайдерская цепочка *этого правила* используется целиком. Цепочки агрегатора/платформы для того же сообщения **не** подмешиваются.

Логическое следствие: чтобы переопределить один провайдер, сохранив остальные от агрегатора, суб-аккаунт должен явно завести всю цепочку. Это осознанная жертва в пользу предсказуемости.

### 5. Правила внутри уровня

- **General правило** уровня = правило с пустыми ключевыми полями (`operator_id IS NULL AND country_code IS NULL AND traffic_type IS NULL`). Срабатывает, если никакое специфичное правило того же уровня не подошло.
- **Specific правила** — с хотя бы одним заполненным ключевым полем.
- На уровне `platform` general **обязателен** и **singleton** на `route_type` (гарантирован миграцией + partial unique index).
- На уровнях `client` / `subaccount` general опционален (0..1 на `(owner, route_type)`). Если отсутствует — наследуется с уровня выше.

### 6. Тип маршрута (`route_type`)

`sms` / `hlr` / `max` — ортогональное измерение. Каждый `route_type` имеет собственную независимую иерархию. Правило обязано иметь заполненный `route_type`.

## Алгоритм matcher

```
resolve(message, route_type):
    for level in [subaccount, client, platform]:          # от частного к общему
        owner_id = message.owner_on_level(level)           # None для platform
        if level != platform and owner_id is None:
            continue                                        # уровень отсутствует для этого сообщения

        candidates = rules.where(
            owner_type = level,
            owner_id   = owner_id,
            route_type = route_type,
            fields_match(message)                           # заполненные поля совпадают с message; NULL = wildcard
        )
        if candidates is empty:
            continue

        best_specificity = max(count_filled_key_fields(r) for r in candidates)
        top = [r for r in candidates if count_filled_key_fields(r) == best_specificity]
        best = min(top, key=lambda r: (r.priority, r.created_at))

        return cell_provider_chain(best)                   # все правила той же ячейки, отсортированные по priority

    raise NoRouteFound                                      # по контракту не должно случаться — platform singleton
```

**Ключевые свойства:**

- Уровень сильнее специфичности: general суб-аккаунта побеждает specific агрегатора.
- `cell` = `(owner_type, owner_id, route_type, operator_id, country_code, traffic_type)`. В одной ячейке могут быть несколько правил, различающихся `provider_id` и `priority` — это failover-цепочка.
- Тай-брейкер при равной specificity: `priority ASC`, далее `created_at ASC`.
- Unmatched сообщение невозможно, поскольку platform general существует для каждого `route_type`.

## Схема БД

### Изменения таблицы `client_routes`

```sql
CREATE TYPE route_owner_type AS ENUM ('platform','client','subaccount');

ALTER TABLE client_routes
  ADD COLUMN owner_type   route_owner_type,
  ADD COLUMN owner_id     UUID NULL,
  ADD COLUMN country_code CHAR(2) NULL,
  ADD COLUMN traffic_type TEXT NULL,
  ADD COLUMN paid_name    TEXT NULL,
  ADD COLUMN regex        TEXT NULL,
  ADD COLUMN schedule_id  UUID NULL REFERENCES route_schedules(id);

-- operator_id уже есть; NULL = wildcard. Снять NOT NULL если стоит.

-- После backfill (см. миграция данных):
ALTER TABLE client_routes ALTER COLUMN owner_type SET NOT NULL;

-- Invariant: owner_id IS NULL iff owner_type='platform'
ALTER TABLE client_routes ADD CONSTRAINT chk_owner_id CHECK (
  (owner_type = 'platform' AND owner_id IS NULL)
  OR
  (owner_type <> 'platform' AND owner_id IS NOT NULL)
);

-- Platform general — singleton на route_type
CREATE UNIQUE INDEX uq_platform_general ON client_routes (route_type)
  WHERE owner_type='platform'
    AND operator_id IS NULL
    AND country_code IS NULL
    AND traffic_type IS NULL;

-- Client/subaccount general — 0..1 на (owner, route_type)
CREATE UNIQUE INDEX uq_owner_general ON client_routes (owner_type, owner_id, route_type)
  WHERE operator_id IS NULL
    AND country_code IS NULL
    AND traffic_type IS NULL
    AND owner_type <> 'platform';

-- Уникальность (cell, provider) — один provider может быть в ячейке только один раз
CREATE UNIQUE INDEX uq_cell_provider ON client_routes (
  owner_type,
  COALESCE(owner_id,     '00000000-0000-0000-0000-000000000000'),
  route_type,
  COALESCE(operator_id,  '00000000-0000-0000-0000-000000000000'),
  COALESCE(country_code, ''),
  COALESCE(traffic_type, ''),
  provider_id
);

-- Indexes под matcher
CREATE INDEX ix_routes_owner_routetype ON client_routes (owner_type, owner_id, route_type);

-- После миграции данных
DROP TABLE route_condition_groups CASCADE;
DROP TABLE route_conditions CASCADE;
```

`route_schedules` сохраняется как есть — ссылка через `schedule_id`.

### Доменная модель (`internal/services/routing/domain/client_route.go`)

```go
type OwnerType string

const (
    OwnerPlatform   OwnerType = "platform"
    OwnerClient     OwnerType = "client"
    OwnerSubaccount OwnerType = "subaccount"
)

type ClientRoute struct {
    ID          uuid.UUID
    OwnerType   OwnerType
    OwnerID     *uuid.UUID          // NULL только для platform
    RouteType   string              // sms | hlr | max
    OperatorID  *uuid.UUID          // NULL = wildcard (ключевое поле)
    CountryCode *string             // NULL = wildcard (ключевое поле)
    TrafficType *string             // NULL = wildcard (ключевое поле)
    PaidName    *string             // NULL = wildcard (фильтр)
    Regex       *string             // NULL = wildcard (фильтр)
    ScheduleID  *uuid.UUID
    ProviderID  uuid.UUID
    Priority    int
    CreatedAt   time.Time
}
```

Старые поля `ClientID`, `Groups []ConditionGroup` удаляются.

### `MatchContext`

```go
type MatchContext struct {
    RouteType   string
    SubaccountID *uuid.UUID        // nil для не-суб-аккаунтов
    ClientID    uuid.UUID           // всегда есть (для суб-аккаунта — его parent; для обычного клиента — он сам)
    OperatorID  uuid.UUID
    CountryCode string
    TrafficType string
    PaidName    string
    PhoneNumber string               // для regex-фильтра
    Time        time.Time            // для schedule
}
```

Pipeline формирует `owner_chain`: `[SubaccountID?, ClientID, nil]`. Matcher проходит по цепочке сверху вниз.

## Миграция данных

### Шаг 0. Pre-flight check (ОБЯЗАТЕЛЬНО до запуска)

```sql
-- Сколько правил используют мульти-группы (OR между группами)?
SELECT count(*) FROM (
  SELECT rule_id FROM route_condition_groups
  GROUP BY rule_id HAVING count(*) > 1
) t;

-- Если > 50 — STOP, дизайн пересматриваем (возможно, возвращаемся к condition_groups).
-- Если 0..50 — продолжаем, развернём их вручную.
```

### Шаг 1. Backfill новых колонок из `route_conditions`

Для каждой строки `client_routes`:

- Собрать все `route_conditions` через `route_condition_groups`.
- Если одна group с conditions — поля переезжают в колонки.
- Если несколько groups (OR) — развернуть в N строк `client_routes`, сохранив `priority`, `provider_id`, `created_at`.

### Шаг 2. Заполнить `owner_type` / `owner_id`

```sql
UPDATE client_routes cr SET owner_type = 'platform', owner_id = NULL
  WHERE cr.client_id IS NULL;

UPDATE client_routes cr SET
  owner_type = CASE WHEN c.parent_client_id IS NULL THEN 'client'::route_owner_type
                    ELSE 'subaccount'::route_owner_type END,
  owner_id   = cr.client_id
FROM clients c WHERE c.id = cr.client_id;
```

### Шаг 3. Seed platform general

```sql
INSERT INTO client_routes (id, owner_type, owner_id, route_type, provider_id, priority)
  SELECT gen_random_uuid(), 'platform', NULL, rt, <default_provider>, 100
  FROM (VALUES ('sms'),('hlr'),('max')) AS t(rt)
  ON CONFLICT DO NOTHING;
```

`<default_provider>` — параметр миграции, определяется оператором деплоя (или `NULL`-safe-placeholder, который админ заменит после).

### Шаг 4. Drop старых таблиц

Только после валидации, что ни один production-запрос к `route_condition_groups`/`route_conditions` не остался (grep в коде).

## Portal API

### Единая RPC для matcher

```protobuf
// routing.proto
service RoutingService {
  rpc ResolveRoutes(ResolveRoutesRequest) returns (ResolveRoutesResponse);
}

message ResolveRoutesRequest {
  string route_type = 1;
  string subaccount_id = 2;  // optional
  string client_id = 3;
  string operator_id = 4;
  string country_code = 5;
  string traffic_type = 6;
  string paid_name = 7;
  string phone_number = 8;
  google.protobuf.Timestamp at = 9;
}

message ResolveRoutesResponse {
  repeated ProviderEntry chain = 1;   // failover-цепочка
  string matched_rule_id = 2;         // для аудита/биллинга
  route_owner_type matched_owner = 3; // на каком уровне сработало
}
```

### CRUD handlers (per-role)

- **Admin** (`/admin/routing/rules`): CRUD правил с `owner_type='platform'`. Запрещено удалять singleton platform general (ошибка 409).
- **Reseller** (`/portal/reseller/routing/rules`): CRUD для `owner_type='client', owner_id=self` и `owner_type='subaccount', owner_id IN (subaccounts of self)`.
- **Subaccount** (`/portal/routing/rules`): CRUD для `owner_type='subaccount', owner_id=self`.
- **Regular client** (`/portal/routing/rules`): CRUD для `owner_type='client', owner_id=self`.

Единая проверка доступа в handler:

```go
func assertOwnable(user User, ruleOwnerType OwnerType, ruleOwnerID *uuid.UUID) error
```

## Тесты

Требуется покрытие (unit, в `matcher_test.go`):

1. **Уровневая иерархия**
   - subaccount specific побеждает aggregator specific
   - subaccount general побеждает aggregator specific (level > specificity)
   - При отсутствии subaccount-правил консультируется aggregator
   - При отсутствии правил на всех уровнях кроме platform — используется platform general
2. **Специфичность внутри уровня**
   - 3 заполненных поля побеждают 2
   - При равной specificity выигрывает priority ASC
   - При равной specificity и priority — created_at ASC
3. **Фильтры vs ключ**
   - `paid_name='bank'` не делает правило специфичнее, чем правило с `paid_name IS NULL`, если ключевые поля одинаковы (тай-брейкер по priority)
   - Правило с `paid_name='bank'` не матчит сообщение с `paid_name='shop'`
4. **Failover-цепочка**
   - В ячейке несколько провайдеров с разным priority → цепочка в нужном порядке
   - Override ячейки суб-аккаунтом: цепочка агрегатора для той же ячейки не используется
5. **Route type**
   - SMS-правило не применяется к HLR-сообщению
6. **Platform singleton**
   - INSERT второго platform general → unique violation
   - DELETE последнего platform general → запрещено (ошибка в handler)

## Кеширование

Сохраняем текущий подход. Инвалидация — при write-операциях по ключу `(owner_type, owner_id, route_type)`. Plus инвалидация на уровень выше при изменении суб-аккаунта (не нужна, т.к. override целиком) — **не требуется**.

## Что вне скоупа

- UI портала (формы, таблицы, фильтры) — отдельная задача после бэкенда.
- Аналитика/биллинг по `matched_rule_id` (логирование в `messages`) — отдельный гейт.
- Политики удаления правил с активными сообщениями в pipeline (soft-delete vs hard) — текущее поведение сохраняется.
- Версионирование правил (audit trail) — не меняем, если уже есть; не добавляем, если нет.

## Открытые предусловия

1. **Pre-flight на стенде**: запустить Шаг 0 миграции. Если мульти-групп много — дизайн пересматривается (в первую очередь секция «Специфичность» и «Миграция данных»).
2. **`MatchContext` в pipeline**: проверить, передаёт ли tarification/pipeline `SubaccountID` + `ClientID` раздельно. Если нет — расширить контракт.
3. **Тесты `matcher_test.go`**: существующие тесты (`TestMatch_ClientSpecificRoute_*`, `TestMatch_FallsBackToDefault*`) удаляются/переписываются — они покрывают устаревшую бинарную модель.

## Pre-flight результат (2026-04-21)

Аудит на dev-сервере:

| Метрика | Значение |
|---|---|
| `multi_group_rules` (OR между группами) | 0 |
| `total_routes` | 39 |
| `platform_routes` (client_id IS NULL) | 2 |
| `total_groups` / `total_conditions` | 31 / 30 |
| routes by type | sms=39 |

Порог ≤50 мульти-групп соблюдён. Плоская модель применима без потери семантики. В миграции учесть: одна группа пустая (31 vs 30 conditions), hlr/max seed platform-general создаст два «висящих» правила — допустимо (занулит NoRouteFound при будущем использовании этих каналов).

## Риски и компромиссы

- **Override целиком vs per-provider merge.** Выбран override — жертвуем удобством («дополни чужую цепочку»), выигрываем предсказуемость. Если боль реальная — в v2 добавляется флаг `inherit_fallback` на ячейке, без ломания модели.
- **Паид-нейм/регекс/шедул — не часть specificity.** Жертвуем точностью ранжирования редких кейсов ради простоты. Если в проде окажется, что такие правила конкурируют на уровне — тай-брейкер через `priority`.
- **Миграция мульти-групп.** Если их много — боль. Pre-flight check обязателен.
- **Platform singleton требует seed.** Первая миграция должна знать «дефолтного провайдера». Если его нет — миграция падает; оператор должен заранее указать его через env/параметр.
