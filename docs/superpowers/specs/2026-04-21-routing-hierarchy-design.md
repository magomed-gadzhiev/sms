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

Специфичность правила считается по заполнённости четырёх измерений:

- `operator_id`
- `country_code`
- `traffic_type`
- Диапазон номера: «заполнен», если хотя бы один из `number_from`, `number_to` не NULL. (Оба NULL = wildcard.)

Пустые (NULL) поля = wildcard. Чем больше измерений заполнено — тем специфичнее правило.

**Не включаем в модель:**
- `paid_name`, `regex` — на аудите проекта (2026-04-21) 0 использований на проде. YAGNI: если понадобятся — отдельная аддитивная миграция.
- `schedule_id` колонка — `route_schedules.id` имеет тип `BIGSERIAL`, FK `UUID → BIGSERIAL` несовместим; плюс обратная ссылка `route_schedules.route_id → client_routes.id` уже существует (циклический FK). Расписания остаются во внешней таблице, lookup по `route_id`.

**Диапазоны (`number_from`, `number_to`):** `BIGINT NULL`. Matcher сравнивает `msg.phone_number::bigint BETWEEN number_from AND number_to` (NULL-ы трактуются как `-∞`/`+∞`). Диапазон-филтр выбран вместо regex по причинам: явная семантика, индексируемые запросы (btree), UI-очевидность («от» / «до»), покрывает ~95% реальных кейсов port-numbers/MNP/carrier-sub-ranges. Экзотику «каждый третий номер» решаем отдельной аддитивной миграцией, если потребуется.

### 4. Семантика наследования

**Override = replace cell.** Если на уровне суб-аккаунта найдено подошедшее правило, провайдерская цепочка *этого правила* используется целиком. Цепочки агрегатора/платформы для того же сообщения **не** подмешиваются.

Логическое следствие: чтобы переопределить один провайдер, сохранив остальные от агрегатора, суб-аккаунт должен явно завести всю цепочку. Это осознанная жертва в пользу предсказуемости.

### 5. Правила внутри уровня

- **General правило** уровня = правило с пустыми ключевыми полями (`operator_id IS NULL AND country_code IS NULL AND traffic_type IS NULL`). Срабатывает, если никакое специфичное правило того же уровня не подошло.
- **Specific правила** — с хотя бы одним заполненным ключевым полем.
- На уровне `platform` general **обязателен**. Presence гарантируется seed'ом в миграции + защитой на DELETE в admin-хендлере (Task 18). Unique-индекс на `(route_type)` **не применяем**: cell идентифицируется ключевыми полями (все NULL для general), так что «один cell на уровень» — автоматическое свойство, а внутри cell допустимы несколько рядов с разными `provider_id` (failover-цепочка).
- На уровнях `client` / `subaccount` general опционален. Если general-cell на уровне отсутствует — наследуется с уровня выше.

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

        best_specificity = max(count_filled_dimensions(r) for r in candidates)
        top = [r for r in candidates if count_filled_dimensions(r) == best_specificity]
        best = min(top, key=lambda r: (r.priority, r.created_at))

        return cell_provider_chain(best)                   # все правила той же ячейки, отсортированные по priority

    raise NoRouteFound                                      # по контракту не должно случаться — platform singleton
```

**Ключевые свойства:**

- Уровень сильнее специфичности: general суб-аккаунта побеждает specific агрегатора.
- `cell` = `(owner_type, owner_id, route_type, operator_id, country_code, traffic_type, number_from, number_to)`. В одной ячейке могут быть несколько правил, различающихся `provider_id` и `priority` — это failover-цепочка.
- `count_filled_dimensions(r)` = количество заполненных из {operator_id, country_code, traffic_type, phone_range}. phone_range засчитывается как 1, если хотя бы одно из number_from/number_to не NULL.
- `fields_match(r, msg)` для phone_range: если оба NULL — wildcard; иначе `msg.phone_number::bigint` должен быть в интервале `[COALESCE(number_from, -∞), COALESCE(number_to, +∞)]`.
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
  ADD COLUMN number_from  BIGINT NULL,
  ADD COLUMN number_to    BIGINT NULL;
  -- paid_name, regex НЕ добавляем (YAGNI, 0 uses в проде)
  -- schedule_id НЕ добавляем (см. пункт 3: FK несовместим, circular)

-- operator_id уже nullable с миграции 000080.

-- Sanity-check: from ≤ to если оба заполнены
ALTER TABLE client_routes ADD CONSTRAINT chk_number_range CHECK (
  number_from IS NULL OR number_to IS NULL OR number_from <= number_to
);

-- После backfill (см. миграция данных):
ALTER TABLE client_routes ALTER COLUMN owner_type SET NOT NULL;

-- Invariant: owner_id IS NULL iff owner_type='platform'
ALTER TABLE client_routes ADD CONSTRAINT chk_owner_id CHECK (
  (owner_type = 'platform' AND owner_id IS NULL)
  OR
  (owner_type <> 'platform' AND owner_id IS NOT NULL)
);

-- Уникальность (cell, provider) — один provider в ячейке только один раз.
-- Cells уникальны по ключевым полям автоматически, отдельные singleton-индексы
-- на general противоречили бы failover-цепочке (несколько провайдеров в ячейке).
CREATE UNIQUE INDEX uq_cell_provider ON client_routes (
  owner_type,
  COALESCE(owner_id,     '00000000-0000-0000-0000-000000000000'::uuid),
  route_type,
  COALESCE(operator_id,  '00000000-0000-0000-0000-000000000000'::uuid),
  COALESCE(country_code, ''),
  COALESCE(traffic_type, ''),
  COALESCE(number_from,  -1),
  COALESCE(number_to,    -1),
  provider_id
);

-- Индексы под matcher и range-lookup
CREATE INDEX ix_routes_owner_routetype ON client_routes (owner_type, owner_id, route_type);
CREATE INDEX ix_routes_number_range ON client_routes (number_from, number_to)
  WHERE number_from IS NOT NULL OR number_to IS NOT NULL;

-- После миграции данных
DROP TABLE route_condition_groups CASCADE;
DROP TABLE route_conditions CASCADE;
```

`route_schedules` сохраняется как есть — связь 1:N через `route_schedules.route_id → client_routes.id`. Никаких обратных FK в `client_routes` не добавляем.

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
    OperatorID  *uuid.UUID          // NULL = wildcard (ключевое измерение)
    CountryCode *string             // NULL = wildcard (ключевое измерение)
    TrafficType *string             // NULL = wildcard (ключевое измерение)
    NumberFrom  *int64              // NULL = -∞ (ключевое измерение — диапазон)
    NumberTo    *int64              // NULL = +∞ (ключевое измерение — диапазон)
    ProviderID  uuid.UUID
    Priority    int
    CreatedAt   time.Time
}
```

Старые поля `ClientID`, `Groups []ConditionGroup` удаляются.

### `MatchContext`

```go
type MatchContext struct {
    RouteType    string
    SubaccountID *uuid.UUID    // nil для не-суб-аккаунтов
    ClientID     uuid.UUID     // всегда есть (для суб-аккаунта — его parent; для обычного клиента — он сам)
    OperatorID   *uuid.UUID    // разрешён resolver'ом номер→оператор (может быть nil если номер не опознан)
    CountryCode  string
    TrafficType  string
    PhoneNumberInt int64       // для проверки диапазона number_from/number_to
    Time         time.Time     // для schedule (lookup через route_schedules.route_id)
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

### Шаг 1. Backfill `owner_type` / `owner_id`

```sql
-- Маршруты без клиента → platform
UPDATE client_routes SET owner_type = 'platform', owner_id = NULL
  WHERE client_id IS NULL;

-- Маршруты с клиентом → client или subaccount по наличию parent
UPDATE client_routes cr SET
  owner_type = CASE WHEN c.parent_client_id IS NULL THEN 'client'::route_owner_type
                    ELSE 'subaccount'::route_owner_type END,
  owner_id   = cr.client_id
FROM clients c
WHERE c.id = cr.client_id;

-- Sanity: нет orphan-маршрутов
DO $$
DECLARE orphans INT;
BEGIN
  SELECT count(*) INTO orphans FROM client_routes
    WHERE client_id IS NOT NULL AND owner_type IS NULL;
  IF orphans > 0 THEN
    RAISE EXCEPTION 'backfill: % orphan client_routes (client_id не ссылается на clients)', orphans;
  END IF;
END$$;
```

### Шаг 2. Backfill ключевых колонок из `route_conditions`

Pre-flight аудит (2026-04-21) показал: 0 мульти-групповых правил, используемые condition_type только `operator` (27) и `traffic_type` (3). `country`, `paid_name`, `regex` — не используются.

Реальные имена колонок: `route_condition_groups.route_id` (не rule_id), `route_conditions.condition_type` / `condition_value` (не type/value).

```sql
UPDATE client_routes cr SET
  operator_id  = sub.operator_id,
  country_code = sub.country_code,
  traffic_type = sub.traffic_type
FROM (
  SELECT
    rcg.route_id,
    MAX(CASE WHEN rc.condition_type = 'operator'     THEN rc.condition_value::uuid END) AS operator_id,
    MAX(CASE WHEN rc.condition_type = 'country'      THEN rc.condition_value        END) AS country_code,
    MAX(CASE WHEN rc.condition_type = 'traffic_type' THEN rc.condition_value        END) AS traffic_type
  FROM route_condition_groups rcg
  JOIN route_conditions rc ON rc.group_id = rcg.id
  GROUP BY rcg.route_id
) sub
WHERE cr.id = sub.route_id;
```

`number_from` / `number_to` не backfill'ятся — в старой схеме ranges не было. NULL по умолчанию корректен.

### Шаг 3. Финализация constraints

```sql
ALTER TABLE client_routes ALTER COLUMN owner_type SET NOT NULL;
-- chk_owner_id, uq_cell_provider, индексы — добавляются здесь (SQL выше, в секции схемы).
```

### Шаг 4. Seed platform general — НЕ ВЫПОЛНЯЕМ

Audit показал: 2 маршрута уже имеют `client_id IS NULL` (platform-sms failover pair). После Шага 1 у них будет `owner_type='platform'` и general-ячейка (operator_id/country_code/traffic_type NULL). Условие «platform general присутствует» выполнено автоматически для sms.

Для hlr/max seed невозможен без указания default-провайдера (в схеме `providers` нет колонки-маркера `code`, и hlr/max в проде не используются). Если канал пойдёт в работу — отдельный init-скрипт.

### Шаг 5. Drop старых таблиц

Только после валидации, что ни один production-запрос к `route_condition_groups`/`route_conditions` не остался (grep в коде). Выполняется отдельной миграцией 109 после переработки matcher'а и репозиториев.

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
  string operator_id = 4;    // разрешён resolver'ом на стороне pipeline
  string country_code = 5;
  string traffic_type = 6;
  int64 phone_number = 7;    // для проверки number_from/number_to
  google.protobuf.Timestamp at = 8;
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
   - 3 заполненных измерения побеждают 2 (из {operator_id, country_code, traffic_type, phone_range})
   - При равной specificity выигрывает priority ASC
   - При равной specificity и priority — created_at ASC
3. **Диапазон номеров (phone_range как 4-е измерение)**
   - Правило с `number_from=79260000000, number_to=79269999999` матчит номер `79261234567` и не матчит `79301234567`
   - NULL `number_from` трактуется как `-∞`, NULL `number_to` как `+∞`
   - Правило с заполненным диапазоном специфичнее правила без него при одинаковых остальных ключах
4. **Failover-цепочка**
   - В ячейке несколько провайдеров с разным priority → цепочка в нужном порядке
   - Override ячейки суб-аккаунтом: цепочка агрегатора для той же ячейки не используется
5. **Route type**
   - SMS-правило не применяется к HLR-сообщению
6. **Platform general presence**
   - DELETE последнего platform general для route_type → запрещено (409 в handler)
   - Несколько provider-рядов в platform general (failover-chain) разрешены

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

Порог ≤50 мульти-групп соблюдён. Плоская модель применима без потери семантики.

Дополнительный audit distribution по condition_type:
- `operator` — 27 conditions
- `traffic_type` — 3 conditions
- `country`, `paid_name`, `regex` — 0 использований

Следствие: `paid_name`, `regex` исключены из модели (YAGNI). `country_code` сохранён как product-level поле (cross-border routing). `number_from/number_to` добавлены как 4-е измерение (диапазоны — реальный use-case, решено в брейнсторме 2026-04-21).

## Риски и компромиссы

- **Override целиком vs per-provider merge.** Выбран override — жертвуем удобством («дополни чужую цепочку»), выигрываем предсказуемость. Если боль реальная — в v2 добавляется флаг `inherit_fallback` на ячейке, без ломания модели.
- **Диапазон вместо regex.** Жертвуем гибкостью (exotic-паттерны типа «каждый третий номер») ради простоты и explicit-семантики. Если появится реальный кейс — аддитивной миграцией добавим regex-поле обратно.
- **YAGNI на paid_name/regex.** Убрали поля, которые 0 раз использовались в проде. Если понадобятся — аддитивная миграция.
- **Platform general presence.** Гарантируется не seed'ом (на проде platform-sms уже есть), а DELETE-protection в admin-handler. Для hlr/max нет general'а вообще — если канал пойдёт, NoRouteFound до создания правила.
