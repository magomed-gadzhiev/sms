# Broadcast Campaigns & Contact Management — Design Spec

**Дата:** 2026-03-27
**Статус:** Draft
**Подход:** C — JSONB + метаданные атрибутов + campaign-service

## Обзор

Механизм рассылок (campaigns) с контактными базами, которые пользователи могут создавать и наполнять через CSV/Excel импорт или API. Контакты имеют свободную схему атрибутов (обязательный — номер телефона), которые используются как переменные в шаблонах сообщений. Полноценный campaign-менеджер с сегментацией, A/B тестированием, auto/manual retry и продвинутой аналитикой.

## 1. Модель данных

### Контактные базы (Contact Lists)

**`contact_lists`** — базы контактов пользователя:
- `id UUID PK`, `client_id UUID FK(clients)`, `name TEXT`, `description TEXT`
- `contacts_count INT DEFAULT 0` (денормализованный счётчик)
- `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`

**`contact_list_attributes`** — метаданные атрибутов базы:
- `id UUID PK`, `contact_list_id UUID FK(contact_lists)`
- `name TEXT` — slug (`city`, `first_name`)
- `display_name TEXT` — человекочитаемое ("Город", "Имя")
- `type TEXT CHECK (type IN ('string', 'number', 'date', 'boolean'))`
- `required BOOLEAN DEFAULT false`
- `position INT` — порядок отображения

Атрибут `phone` — системный, всегда существует, не хранится в этой таблице.

**`contacts`** — контакты:
- `id UUID PK`, `contact_list_id UUID FK(contact_lists)`
- `phone TEXT NOT NULL` — нормализованный E.164
- `attributes JSONB DEFAULT '{}'` — пользовательские атрибуты (`{"city": "Москва", "age": 25}`)
- `tags TEXT[] DEFAULT '{}'` — массив тегов
- `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`
- **UNIQUE** (`contact_list_id`, `phone`)
- **GIN-индекс** на `attributes` (`jsonb_path_ops`)
- **GIN-индекс** на `tags`

**`contact_imports`** — история импортов:
- `id UUID PK`, `contact_list_id UUID FK`, `client_id UUID FK`
- `file_name TEXT`, `file_size BIGINT`
- `status TEXT CHECK (status IN ('pending', 'processing', 'completed', 'failed'))`
- `total_rows INT`, `imported_count INT`, `updated_count INT`, `error_count INT`
- `errors JSONB` — массив ошибок с номерами строк
- `column_mapping JSONB` — маппинг колонок CSV → атрибуты
- `created_at TIMESTAMPTZ`, `completed_at TIMESTAMPTZ`

### Кампании (Campaigns)

**`campaigns`** — рассылки:
- `id UUID PK`, `client_id UUID FK`, `name TEXT`
- `status TEXT CHECK (status IN ('draft', 'scheduled', 'materializing', 'running', 'paused', 'completed', 'cancelled'))`
- `contact_list_id UUID FK(contact_lists)`
- `template_id UUID FK(templates)`
- `source TEXT` (sender ID)
- `segment_rules JSONB` — правила сегментации
- `segment_tags TEXT[]` — фильтр по тегам
- `send_rate INT` — сообщений/сек (0 = rate-limit клиента)
- `scheduled_at TIMESTAMPTZ`, `started_at TIMESTAMPTZ`, `completed_at TIMESTAMPTZ`
- `retry_config JSONB` — `{"enabled": bool, "delay_hours": int, "max_retries": int, "alternative_template_id": uuid|null}`
- `total_recipients INT`, `sent_count INT`, `delivered_count INT`, `failed_count INT`
- `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`

**`campaign_variants`** — A/B варианты:
- `id UUID PK`, `campaign_id UUID FK`
- `name TEXT` ("Вариант A", "Вариант B")
- `template_id UUID FK(templates)`, `percentage INT` (доля аудитории, %)
- `is_winner BOOLEAN DEFAULT false`, `is_control BOOLEAN DEFAULT false`
- `sent_count INT`, `delivered_count INT`, `failed_count INT`

**`campaign_ab_config`** — настройки A/B теста:
- `campaign_id UUID PK FK`
- `metric TEXT CHECK (metric IN ('delivery_rate'))` — delivery_rate = delivered/sent. Дополнительные метрики (click_rate и т.д.) добавляются при интеграции с link tracking
- `test_duration_hours INT`
- `auto_select_winner BOOLEAN DEFAULT false`
- `winner_variant_id UUID FK(campaign_variants)`, `winner_selected_at TIMESTAMPTZ`

**`campaign_recipients`** — материализованная аудитория (hash-партиционирование по `campaign_id`, 16 партиций):
- `id UUID PK`, `campaign_id UUID`, `contact_id UUID`, `phone TEXT`
- `variant_id UUID FK(campaign_variants)`
- `status TEXT CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled'))`
- `message_id UUID` — ссылка на отправленное сообщение
- `retry_count INT DEFAULT 0`, `last_retry_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`

**`campaign_retry_log`** — лог повторных отправок:
- `id UUID PK`, `campaign_id UUID FK`, `recipient_id UUID FK`
- `retry_number INT`, `provider_id UUID`, `status TEXT`, `error_code TEXT`
- `created_at TIMESTAMPTZ`

**`campaign_stats_snapshots`** — агрегированные метрики:
- `campaign_id UUID`, `variant_id UUID` (nullable), `snapshot_at TIMESTAMPTZ`
- `sent INT`, `delivered INT`, `failed INT`, `pending INT`
- `avg_delivery_time_ms INT`, `cost DECIMAL`
- PK: (`campaign_id`, `variant_id`, `snapshot_at`)

## 2. Новые микросервисы

### contact-service (порт 5012)

Управление контактными базами, контактами, импортом.

```protobuf
service ContactService {
  // Базы контактов
  rpc CreateContactList(CreateContactListRequest) returns (ContactList);
  rpc ListContactLists(ListContactListsRequest) returns (ContactListPage);
  rpc GetContactList(GetContactListRequest) returns (ContactList);
  rpc UpdateContactList(UpdateContactListRequest) returns (ContactList);
  rpc DeleteContactList(DeleteContactListRequest) returns (google.protobuf.Empty);

  // Атрибуты базы
  rpc SetListAttributes(SetListAttributesRequest) returns (AttributeList);
  rpc GetListAttributes(GetListAttributesRequest) returns (AttributeList);

  // Контакты
  rpc CreateContact(CreateContactRequest) returns (Contact);
  rpc UpdateContact(UpdateContactRequest) returns (Contact);
  rpc DeleteContact(DeleteContactRequest) returns (google.protobuf.Empty);
  rpc ListContacts(ListContactsRequest) returns (ContactPage);
  rpc BatchUpsertContacts(BatchUpsertContactsRequest) returns (BatchResult);

  // Теги
  rpc AddTags(AddTagsRequest) returns (google.protobuf.Empty);
  rpc RemoveTags(RemoveTagsRequest) returns (google.protobuf.Empty);
  rpc ListTags(ListTagsRequest) returns (TagList);

  // Импорт
  rpc StartImport(StartImportRequest) returns (ImportJob);
  rpc GetImportStatus(GetImportStatusRequest) returns (ImportJob);
  rpc ListImports(ListImportsRequest) returns (ImportJobPage);

  // Сегментация
  rpc PreviewSegment(PreviewSegmentRequest) returns (SegmentPreview);
  rpc StreamSegment(StreamSegmentRequest) returns (stream ContactBatch);
}
```

Внутренний воркер обрабатывает CSV/Excel импорт асинхронно (потоковое чтение, батчи по 1000, upsert по phone).

### campaign-service (порт 5013)

Управление кампаниями, A/B тестированием, retry, аналитикой.

```protobuf
service CampaignService {
  // CRUD
  rpc CreateCampaign(CreateCampaignRequest) returns (Campaign);
  rpc GetCampaign(GetCampaignRequest) returns (Campaign);
  rpc ListCampaigns(ListCampaignsRequest) returns (CampaignPage);
  rpc UpdateCampaign(UpdateCampaignRequest) returns (Campaign);
  rpc DeleteCampaign(DeleteCampaignRequest) returns (google.protobuf.Empty);

  // Управление
  rpc LaunchCampaign(LaunchCampaignRequest) returns (Campaign);
  rpc PauseCampaign(PauseCampaignRequest) returns (Campaign);
  rpc ResumeCampaign(ResumeCampaignRequest) returns (Campaign);
  rpc CancelCampaign(CancelCampaignRequest) returns (Campaign);

  // A/B
  rpc SetVariants(SetVariantsRequest) returns (VariantList);
  rpc SetABConfig(SetABConfigRequest) returns (ABConfig);
  rpc SelectWinner(SelectWinnerRequest) returns (Campaign);

  // Retry
  rpc SetRetryConfig(SetRetryConfigRequest) returns (RetryConfig);
  rpc RetryFailed(RetryFailedRequest) returns (Campaign);

  // Аналитика
  rpc GetCampaignStats(GetCampaignStatsRequest) returns (CampaignStats);
  rpc GetCampaignTimeline(GetCampaignTimelineRequest) returns (TimelineData);
  rpc GetVariantComparison(GetVariantComparisonRequest) returns (VariantComparisonData);
  rpc GetDeliveryHeatmap(GetDeliveryHeatmapRequest) returns (HeatmapData);
  rpc GetOptimalSendTime(GetOptimalSendTimeRequest) returns (SendTimeRecommendation);
  rpc ExportReport(ExportReportRequest) returns (ReportFile);
}
```

### Интеграция с существующими сервисами

- **campaign-service → contact-service** (gRPC): `StreamSegment` для материализации аудитории
- **campaign-service → template-service** (gRPC): получение шаблона для подстановки переменных
- **campaign-service → Kafka `sms.outgoing`**: порционная публикация (батчи по 1000)
- **campaign-service ← Kafka `sms.status`**: consumer group `campaign-status-consumer`, обновление `campaign_recipients`
- **campaign-service → Kafka `campaign.events`**: события кампании для аналитики и вебхуков

### Метаданные в KafkaMessage

В `KafkaMessage` добавляется поле `metadata map[string]string`:
```json
{
  "campaign_id": "uuid",
  "variant_id": "uuid",
  "recipient_id": "uuid"
}
```
Проходит через весь pipeline и возвращается в `sms.status`.

## 3. Сегментация

### Формат правил

`segment_rules` — древовидная структура AND/OR:

```json
{
  "operator": "AND",
  "conditions": [
    {"field": "city", "op": "eq", "value": "Москва"},
    {
      "operator": "OR",
      "conditions": [
        {"field": "age", "op": "gte", "value": 25},
        {"field": "vip", "op": "eq", "value": true}
      ]
    }
  ]
}
```

### Операторы по типам

| Тип | Операторы |
|-----|-----------|
| `string` | `eq`, `neq`, `contains`, `starts_with`, `ends_with`, `in`, `not_in`, `is_empty`, `is_not_empty` |
| `number` | `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `between`, `is_empty`, `is_not_empty` |
| `date` | `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `between`, `is_empty`, `is_not_empty` |
| `boolean` | `eq`, `neq`, `is_empty`, `is_not_empty` |

### Комбинация с тегами

`segment_tags` применяется как AND поверх `segment_rules`. PostgreSQL overlap: `tags && segment_tags`.

### Трансляция в SQL

contact-service транслирует правила в параметризованный SQL:

```sql
SELECT id, phone, attributes FROM contacts
WHERE contact_list_id = $1
  AND attributes->>'city' = $2
  AND ((attributes->>'age')::int >= $3 OR (attributes->>'vip')::boolean = $4)
  AND tags && $5
```

Макс. глубина вложенности: 3 уровня.

## 4. A/B тестирование

### Процесс

1. **Настройка (draft):** 2-5 вариантов, каждый со своим `template_id` и `percentage`. Сумма = 100%. Опционально `is_control` вариант (не получает сообщения до выбора победителя).
   Пример: Вариант A (15%), Вариант B (15%), Control (70%).

2. **Материализация (LaunchCampaign):** campaign-service стримит сегментированные контакты от contact-service. Каждому recipient назначается `variant_id` (weighted random). Запись в `campaign_recipients` батчами по 10 000 через COPY.

3. **Отправка тестовой части:** публикуются сообщения только для не-control вариантов. Шаблон из `campaign_variants.template_id`, переменные из `contact.attributes`.

4. **Ожидание:** campaign-service потребляет `sms.status`, обновляет статусы. По таймеру `test_duration_hours` — сравнение метрик.

5. **Выбор победителя:**
   - Автоматический (`auto_select_winner = true`): по `metric` (delivery_rate)
   - Ручной: пользователь через UI

6. **Отправка control-группе:** шаблон победившего варианта.

## 5. Auto-retry и ручной retry

### Автоматический retry

Фоновый тикер в campaign-service (каждые 60 сек) проверяет кампании с `retry_config.enabled = true`:
- `campaign_recipients` WHERE `status = 'failed'` AND `retry_count < max_retries`
- Если прошло `delay_hours` с `last_retry_at`
- Повторная публикация в `sms.outgoing`
- Если задан `alternative_template_id` — используется он
- Лог в `campaign_retry_log`

### Ручной retry

`RetryFailed` RPC: создаёт новую волну retry для всех `status = 'failed'`. Опционально — с другим шаблоном.

## 6. Импорт контактов

### Процесс

1. **Загрузка файла:** portal-gateway принимает multipart upload, сохраняет в `uploads/{client_id}/{import_id}/`. Создаётся `contact_imports` (status: `pending`).

2. **Маппинг колонок:** portal-gateway парсит первые 5 строк, возвращает preview. Пользователь маппит колонки на атрибуты (или создаёт новые). `column_mapping`:
   ```json
   {
     "columns": {
       "0": {"target": "phone", "type": "phone"},
       "1": {"target": "first_name", "type": "string"},
       "2": {"target": "city", "type": "string"},
       "3": {"target": "age", "type": "number"}
     },
     "has_header": true,
     "delimiter": ","
   }
   ```

3. **Асинхронная обработка** (воркер в contact-service):
   - Потоковое чтение файла
   - Батчи по 1000: нормализация телефона (E.164), валидация типов, upsert по (`contact_list_id`, `phone`)
   - Ошибки → `errors` JSONB (номер строки + причина)
   - По завершении: обновление `contacts_count`

4. **Автосоздание атрибутов:** новые атрибуты при маппинге автоматически добавляются в `contact_list_attributes`.

### Подстановка переменных в шаблоны

При отправке campaign-service для каждого recipient:
1. Берёт `template.body` — `"Привет, {{first_name}}! Акция в городе {{city}}"`
2. Берёт `contact.attributes` — `{"first_name": "Иван", "city": "Москва"}`
3. Заменяет `{{key}}` → значение
4. Отсутствующий атрибут → пустая строка

При создании кампании — валидация: все переменные шаблона должны быть атрибутами выбранной контактной базы.

## 7. HTTP API (client-gateway & portal-gateway)

### Контактные базы и контакты

```
POST   /v1/contact-lists                          Создать базу
GET    /v1/contact-lists                          Список баз
GET    /v1/contact-lists/{id}                     Получить базу
PUT    /v1/contact-lists/{id}                     Обновить базу
DELETE /v1/contact-lists/{id}                     Удалить базу

PUT    /v1/contact-lists/{id}/attributes          Задать/обновить атрибуты
GET    /v1/contact-lists/{id}/attributes          Получить атрибуты

POST   /v1/contact-lists/{id}/contacts            Создать контакт
GET    /v1/contact-lists/{id}/contacts            Список (пагинация, фильтры)
GET    /v1/contact-lists/{id}/contacts/{cid}      Получить контакт
PUT    /v1/contact-lists/{id}/contacts/{cid}      Обновить контакт
DELETE /v1/contact-lists/{id}/contacts/{cid}      Удалить контакт
POST   /v1/contact-lists/{id}/contacts/batch      Batch upsert (до 10 000)

POST   /v1/contact-lists/{id}/contacts/tags       Массовое добавление тегов
DELETE /v1/contact-lists/{id}/contacts/tags       Массовое удаление тегов
GET    /v1/contact-lists/{id}/tags                Уникальные теги

POST   /v1/contact-lists/{id}/imports/upload      Загрузка файла
POST   /v1/contact-lists/{id}/imports/{iid}/start Запуск с маппингом
GET    /v1/contact-lists/{id}/imports/{iid}       Статус импорта
GET    /v1/contact-lists/{id}/imports              История импортов
```

### Кампании

```
POST   /v1/campaigns                              Создать
GET    /v1/campaigns                              Список
GET    /v1/campaigns/{id}                         Получить
PUT    /v1/campaigns/{id}                         Обновить (draft)
DELETE /v1/campaigns/{id}                         Удалить (только draft)

POST   /v1/campaigns/{id}/launch                  Запустить
POST   /v1/campaigns/{id}/pause                   Приостановить
POST   /v1/campaigns/{id}/resume                  Возобновить
POST   /v1/campaigns/{id}/cancel                  Отменить

PUT    /v1/campaigns/{id}/variants                Задать A/B варианты
PUT    /v1/campaigns/{id}/ab-config               Настройки A/B
POST   /v1/campaigns/{id}/select-winner           Ручной выбор победителя

PUT    /v1/campaigns/{id}/retry-config            Настройки auto-retry
POST   /v1/campaigns/{id}/retry                   Ручной retry

GET    /v1/campaigns/{id}/stats                   Общая статистика
GET    /v1/campaigns/{id}/timeline                Графики по времени
GET    /v1/campaigns/{id}/variants/compare        Сравнение A/B
GET    /v1/campaigns/{id}/heatmap                 Heatmap доставки
GET    /v1/campaigns/{id}/optimal-time            Рекомендации
GET    /v1/campaigns/{id}/report                  Экспорт (CSV/PDF)
```

## 8. Аналитика кампаний

### GetCampaignStats
Агрегация из `campaign_stats_snapshots` (обновляется каждые 30 сек для running кампаний):
- total_recipients, sent, delivered, failed, pending, retry
- delivery_rate, total_cost, avg_delivery_time_ms
- Разбивка per_operator, per_variant

### GetCampaignTimeline
Параметры: `interval` (1min/5min/1hour), `metric` (sent/delivered/failed).
Временной ряд из `campaign_recipients` с GROUP BY по интервалам `updated_at`.

### GetVariantComparison
Таблица сравнения всех вариантов: audience_size, sent, delivered, failed, delivery_rate, avg_delivery_time_ms, cost. Плюс winner info.

### GetDeliveryHeatmap
Матрица день_недели (0-6) × час (0-23) → delivered_count, delivery_rate. Из `campaign_recipients WHERE status = 'delivered'`.

### GetOptimalSendTime
Анализ всех завершённых кампаний клиента за 30 дней. Группировка по часу → ранжирование по delivery_rate + avg_delivery_time. Топ-3 рекомендации.

### ExportReport
- CSV: выгрузка campaign_recipients (phone, status, variant, delivery_time, error_code)
- PDF: сводный отчёт через Go HTML template → PDF

### campaign_stats_snapshots
Обновляется тикером каждые 30 сек для running кампаний. Аналитические эндпоинты читают из снапшотов; детальные (heatmap, timeline) — из `campaign_recipients`.

## 9. Frontend

### Новые страницы

**Контактные базы (`/contact-lists`)**
- Список баз — таблица с названием, кол-вом контактов, датой, действиями
- Создание/редактирование — форма с настройкой атрибутов (тип, обязательность, порядок)
- Детальная страница (`/contact-lists/{id}`) — динамическая таблица контактов, inline-редактирование, теги, импорт

**Импорт (`/contact-lists/{id}/import`)**
- Шаг 1: Drag-and-drop загрузка CSV/XLSX
- Шаг 2: Preview 5 строк + маппинг колонок на атрибуты
- Шаг 3: Прогресс-бар (polling каждые 2 сек)
- Шаг 4: Результат — imported/updated/errors, скачать лог ошибок

**Кампании (`/campaigns`)**
- Список — таблица: название, статус, база, дата, прогресс-бар, действия
- Wizard создания (5 шагов):
  1. Основное — название, база, sender ID
  2. Аудитория — SegmentBuilder + теги + preview кол-ва
  3. Сообщение — шаблон, preview подстановки, A/B настройка
  4. Расписание и retry — сейчас/по расписанию, auto-retry конфиг
  5. Подтверждение — сводка, оценка стоимости

**Детальная страница кампании (`/campaigns/{id}`)**
- Заголовок: статус, кнопки управления
- Вкладки: Обзор (статистика), Timeline (графики), A/B результаты, Heatmap, Получатели, Рекомендации
- Экспорт отчёта

### Компонент SegmentBuilder
- Загружает атрибуты из `contact_list_attributes`
- Дерево AND/OR условий (макс. 3 уровня)
- Автоматический выбор операторов по типу атрибута
- Debounced PreviewSegment при изменении

### Навигация
Новые пункты в Sidebar после «Сообщения»:
- **Контакты** → `/contact-lists`
- **Рассылки** → `/campaigns`

## 10. Производительность и масштаб

### Материализация аудитории (1М+)
1. contact-service: server-side streaming gRPC (`StreamSegment`), порции по 5000 контактов
2. campaign-service: запись в `campaign_recipients` через COPY, батчи по 10 000
3. Время на 1М контактов с фильтрацией: ~30-60 сек

### Порционная отправка в Kafka
- Батчи по 1000 сообщений
- Rate control: `send_rate` на уровне кампании
- Подстановка переменных перед публикацией
- `message_id` записывается в `campaign_recipients`

### Потребление статусов
- Consumer group: `campaign-status-consumer`
- Фильтрация по `campaign_id` в metadata
- Batch update `campaign_recipients` по 500
- Инкремент счётчиков в snapshots

### Партиционирование campaign_recipients
Hash partitioning по `campaign_id`, 16 партиций.

### Индексы

```sql
CREATE INDEX idx_contacts_list ON contacts(contact_list_id);
CREATE INDEX idx_contacts_list_phone ON contacts(contact_list_id, phone);
CREATE INDEX idx_contacts_attributes_gin ON contacts USING gin(attributes jsonb_path_ops);
CREATE INDEX idx_contacts_tags_gin ON contacts USING gin(tags);

CREATE INDEX idx_cr_campaign_status ON campaign_recipients(campaign_id, status);
CREATE INDEX idx_cr_campaign_variant ON campaign_recipients(campaign_id, variant_id);
CREATE INDEX idx_cr_message_id ON campaign_recipients(message_id);

CREATE INDEX idx_css_campaign ON campaign_stats_snapshots(campaign_id, snapshot_at DESC);
```

### Graceful pause/cancel
- **Pause:** прекращение публикации новых батчей, `pending` остаются pending
- **Resume:** продолжение с `campaign_recipients WHERE status = 'pending'`
- **Cancel:** все `pending` → `cancelled`, in-flight сообщения доходят до конца
