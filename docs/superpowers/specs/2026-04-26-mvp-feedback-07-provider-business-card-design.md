# Spec №7: Провайдеры — бизнес-карточка с поддержкой нескольких SMPP-подключений

**Дата:** 2026-04-26
**Источник:** MVP-таблица, строка «Интеграции → Провайдеры»
**Статус:** Не сделано (структурно текущая модель не подходит).

## Контекст

Спека требует:
1. Кнопка «Добавление Провайдера»: Название / Комментарий / Договор / аккаунт-менеджер / Кнопка «Добавить SMPP-подключение».
2. Карточка провайдера: Название / Комментарий / Договор / аккаунт-менеджер (ФИО, телефон, почта) / Кнопка «Тарифы» (себестоимости) / Список sender names / SMPP-подключения этого провайдера.
3. Список провайдеров: Название / Кол-во активных подключений (n из m) / Senders / Аккаунт-менеджер (ФИО).

Аудит:
- Текущая модель: одна запись в `providers` = одно SMPP-подключение (host:port:system_id). Поля: `id, name UNIQUE, host, port, system_id, password, system_type, bind_type, max_connections, throughput_per_second, daily_quota, monthly_quota, active`.
- 1:N (один провайдер → N подключений) **не поддерживается**.
- Полей `comment / contract / account_manager_*` нет.
- `sender_names` привязаны к `client_id`, не к `provider_id`. Связи provider↔sender нет.
- Стоимости провайдера: `provider_tarification` (`migrations/000039`) — это уже есть, привязано к `provider_id`.
- Контракты: `contracts` (`migrations/000083`) — привязаны к `client_id` и `legal_entity_id`. Не подходят для провайдерских договоров без расширения.

## Решение

### Подход

**Гибрид: переименовать существующий `providers` в `smpp_connections`, добавить новую таблицу `provider_cards` (бизнес-карточка), `smpp_connections` ссылается на `provider_cards.id` через FK.**

Альтернатива «всё на одной таблице» (ALTER providers + плоские поля) ломает UNIQUE(name), которым пользуются seed-скрипты и роутинг. Гибрид сохраняет всю существующую логику routing/billing/tarification (они работают с физическим подключением) и добавляет бизнес-уровень сверху.

### Что меняем

**1. Миграция `000121_provider_cards`**

```sql
-- Шаг 1: новая таблица бизнес-карточек
CREATE TABLE provider_cards (
  id              SERIAL PRIMARY KEY,
  name            VARCHAR(255) UNIQUE NOT NULL,   -- логическое имя провайдера
  comment         TEXT,
  contract_text   TEXT,                           -- свободный текст договора (номер, дата, ссылка)
  account_manager_name  VARCHAR(255),
  account_manager_phone VARCHAR(50),
  account_manager_email VARCHAR(255),
  active          BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Шаг 2: переименовать providers → smpp_connections, добавить FK
ALTER TABLE providers RENAME TO smpp_connections;
ALTER TABLE smpp_connections ADD COLUMN provider_card_id INTEGER NULL REFERENCES provider_cards(id);

-- Шаг 3: миграция данных. Каждое существующее подключение
-- становится своим provider_card (1:1) — позже админ может объединить.
INSERT INTO provider_cards (name, comment, active, created_at)
SELECT name, NULL, active, created_at FROM smpp_connections;

UPDATE smpp_connections sc
SET provider_card_id = pc.id
FROM provider_cards pc
WHERE pc.name = sc.name;

ALTER TABLE smpp_connections ALTER COLUMN provider_card_id SET NOT NULL;
ALTER TABLE smpp_connections DROP CONSTRAINT IF EXISTS providers_name_key;
-- name на smpp_connections больше не уникально (несколько коннектов могут иметь одно имя)
ALTER TABLE smpp_connections RENAME COLUMN name TO connection_label;
-- "connection_label" = человекочитаемый ярлык подключения (напр. "MTS-Primary", "MTS-Backup")

-- Шаг 4: связь provider_cards ↔ sender_names (M:N)
CREATE TABLE provider_card_sender_names (
  provider_card_id INTEGER NOT NULL REFERENCES provider_cards(id) ON DELETE CASCADE,
  sender_name_id   INTEGER NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
  status           VARCHAR(50) NOT NULL DEFAULT 'pending',
  PRIMARY KEY (provider_card_id, sender_name_id)
);

CREATE INDEX idx_smpp_connections_card ON smpp_connections(provider_card_id);
```

`down`: обратные операции. Сделать аккуратно, чтобы routing-таблицы и `provider_tarification.provider_id` (теперь = `smpp_connections.id`) не сломались.

**Внимание к `provider_tarification` и `routing_*`:** все они ссылаются на `providers.id` через FK. После RENAME ссылки сохраняются, но семантически это теперь «тариф/маршрут на конкретное подключение», а не на бизнес-провайдера. Для MVP — это OK (тарифы и маршруты остаются на уровне коннекта; если бизнес попросит «один тариф на все коннекты провайдера X» — добавим `provider_tarification.provider_card_id` в отдельной задаче).

**2. Backend: новые операции**

`internal/services/routing/application/provider_card_service.go`:
- `CreateCard(name, comment, contract, manager_*)`
- `UpdateCard(id, fields)`
- `DeleteCard(id)` — запрещено, если есть привязанные `smpp_connections` (нужно сначала удалить коннекты).
- `ListCards()` — с агрегатами: `total_connections`, `active_connections`, `senders_count`.
- `GetCard(id)` — с вложенными списками подключений и sender names.

`internal/services/routing/application/smpp_connection_service.go` (переименование `provider_service`):
- `AddConnection(provider_card_id, label, host, port, ...)` — то же что было `CreateProvider`, плюс FK.
- `UpdateConnection(id, fields)` — то же что было `UpdateProvider` (если есть; иначе реализовать).
- `DeleteConnection(id)`.

API:
```
GET    /portal/v1/provider-cards
GET    /portal/v1/provider-cards/:id
POST   /portal/v1/provider-cards
PATCH  /portal/v1/provider-cards/:id
DELETE /portal/v1/provider-cards/:id

GET    /portal/v1/provider-cards/:id/connections
POST   /portal/v1/provider-cards/:id/connections
PATCH  /portal/v1/connections/:id
DELETE /portal/v1/connections/:id

GET    /portal/v1/provider-cards/:id/sender-names    # список через provider_card_sender_names
POST   /portal/v1/provider-cards/:id/sender-names    # привязать
DELETE /portal/v1/provider-cards/:id/sender-names/:sender_id
```

**3. Frontend**

Структура страницы `/providers` (она же — лейбл «Подключения» из spec №3 — но теперь это страница **бизнес-карточек**, а внутри карточки — список коннектов):

```
/providers                      # список провайдеров (бизнес-карточек)
  └── columns: Название | Подключения (n из m) | Senders | Аккаунт-менеджер | Действия

/providers/:cardId              # карточка провайдера
  ├── Header: Название / Комментарий / Договор / Аккаунт-менеджер (ФИО, тел, email)
  ├── Tab "Подключения"         # список SMPP, кнопка "+ Добавить подключение"
  ├── Tab "Sender names"        # список привязанных, кнопка "+ Привязать"
  └── Tab "Тарифы"              # переиспользует существующую страницу тарификации

/providers/:cardId/connections/new       # визард из spec №3 (5 шагов)
/providers/:cardId/connections/:id/edit  # тот же визард в режиме edit
```

**Список провайдеров (главная страница `/providers`):**
- Колонки: Название / **Подключения «n из m»** / **Senders (count)** / Аккаунт-менеджер (ФИО) / Действия (Редактировать, Удалить).
- «n из m» = `active_connections / total_connections`. Активность подключения определяется по полю `smpp_connections.active` И флагу bind-статуса в Redis (`internal/gateway/smpp/server/redis_store.go`). На бэке агрегатор: `n = COUNT(*) FROM smpp_connections WHERE provider_card_id=$1 AND active=true AND bound_at_redis=true`. Если проверка Redis дорогая — кешировать ~30 сек.

**Карточка провайдера:**
- Отдельная страница, не модалка (в карточке несколько вкладок и листов).
- Шапка с реквизитами + кнопка «Редактировать карточку».
- Tab «Подключения» — таблица как в spec №3, кнопка «+ Добавить подключение» открывает визард в контексте `cardId`.
- Tab «Sender names» — простой лист с возможностью привязки/отвязки.
- Tab «Тарифы» — embed существующей UI тарификации (фильтр по `provider_card_id` или `connection_id` на выбор).

### Что НЕ делаем

- **`account_managers` как отдельная сущность с reuse.** Текстовые поля на карточке достаточны для MVP. Если у нас будет 50+ провайдеров и менеджеры дублируются — добавим reuse.
- **`provider_contracts` как отдельная таблица с файлами.** Сейчас договор — текстовое поле (можно вписать номер, дату, URL на S3). Файловое хранилище — отдельный спек.
- **Перенос `provider_tarification` на уровень `provider_cards`.** Тарифы остаются на коннекты. Если бизнес попросит общий тариф — отдельная задача.
- **Удаление колонки `connection_label` UNIQUE.** Намеренно — допускаем «MTS-Primary» и «MTS-Backup» с одним именем у разных провайдеров.

### Открытые вопросы (закрываю сам)

- **Договор:** свободный текст. Если позже понадобится файл — добавляем колонку `contract_url` + UI upload отдельным спеком.
- **Аккаунт-менеджер:** три текстовых поля (ФИО / тел / email) на самой карточке, без reuse-сущности.
- **Sender names в карточке:** показываем только те, что **явно привязаны** через `provider_card_sender_names` (статус `approved` оператором этого провайдера). Глобальный список одобренных у этого провайдера sender'ов — отдельная фича модерации.

## Acceptance criteria

1. Миграция `000121` накатывается; данные мигрируют 1:1 (каждое старое подключение становится `provider_card` + `smpp_connection`).
2. Существующая логика routing/billing/tarification работает без регрессий (проверить smoke-тестом отправки сообщения).
3. Страница `/providers` показывает таблицу карточек с колонкой «n из m».
4. Карточка `/providers/:cardId` открывает 3 вкладки.
5. Из карточки можно добавить второе SMPP-подключение к существующему провайдеру; оба показываются в списке.
6. Привязка sender_name к карточке отображается во вкладке «Sender names».
7. Редактирование реквизитов карточки не задевает SMPP-подключения (бинды активны).

## Размер задачи

L (1.5–2 рабочих недели: миграция с проверкой целостности referenced FK + новый слой `provider_card_*` + переписывание UI `/providers` на двух уровнях).

**Риски:**
- Миграция RENAME providers → smpp_connections трогает все FK в БД. Нужен careful smoke на staging до прод.
- Roling deploy: API клиентов, использующих старый `/providers` endpoint, ломается. Если есть внешние потребители — нужен deprecation period с алиасами.

**Митигация:** проверить через `\d+ smpp_connections` все referencing tables; согласовать deploy с QA.
