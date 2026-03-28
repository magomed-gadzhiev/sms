# Marketing Features Suite — Design Spec

**Date:** 2026-03-28
**Status:** Approved
**Scope:** 6 features across 4 parallel workstreams

---

## Overview

Набор маркетинговых фич для SMS-платформы, превращающих её из базового шлюза в полноценный инструмент маркетинговых кампаний.

### Фичи

1. **Template Engine** — Liquid-шаблонизатор с условной логикой, фильтрами и валидацией длины SMS
2. **Click Tracking** — Link-shortener с редиректом, событиями кликов и кастомными доменами клиентов
3. **Frequency Capping** — Ограничение частоты контактов на уровне аккаунта и кампании
4. **Local Time Delivery** — Доставка по локальному времени получателя + тихие часы
5. **A/B Test-Then-Send** — Тест на выборке, автоматический выбор победителя, rollout остальным
6. **Smart Segments** — Кросс-списковые сохранённые сегменты с визуальным конструктором

### Workstreams

```
Параллельно:  [WS1: Template Engine] ────────────────────────►
              [WS2: Click Tracking]  ────────────────────────►
              [WS3: Freq Capping + Local Time] ──────────────►
                                                              ╲
Последовательно:                                               [WS4: A/B + Segments] ──►
```

- WS1-3 независимы, не пересекаются по файлам
- WS4 зависит от WS2 (click metrics для A/B) и WS3 (capping-статус в recipients)

---

## WS1: Template Engine (Liquid-шаблонизатор)

### Архитектура

Новый пакет `internal/shared/template/` — общая библиотека, используемая campaign-service и будущими сервисами.

**Компоненты:**
- **Renderer** — обёртка над `osteele/liquid` с кастомными фильтрами для SMS
- **Validator** — проверяет синтаксис шаблона, считает min/max длину результата, предупреждает о превышении сегментов
- **Sandbox** — ограничение: без доступа к FS/сети, таймаут рендеринга 50мс на контакт, лимит output 1600 символов (10 SMS-сегментов max)

### Синтаксис

```liquid
Привет, {{ name | default: "друг" }}!
{% if vip %}Ваша скидка: {{ discount }}%{% endif %}
{{ order_date | date: "%d.%m.%Y" }}
```

### Кастомные фильтры

| Фильтр | Описание |
|---------|----------|
| `default` | Фолбэк при пустом значении |
| `truncate` | Обрезать до N символов |
| `phone_format` | Форматирование номера телефона |
| `date` | Форматирование дат с локалью |

### Интеграция в campaign-service

- При материализации кампании: для каждого recipient рендерится текст из шаблона + атрибуты контакта
- Добавляется поле `rendered_text` в `campaign_recipients` (TEXT, nullable) — рендеренный текст хранится, чтобы не рендерить повторно при retry
- При единичной отправке через API: рендеринг в client-gateway перед публикацией в Kafka
- **Pipeline-worker:** если `rendered_text` уже заполнен (кампания) — пропустить шаг рендеринга; если пуст (одиночная отправка) — рендерить на месте

### Превью на фронте

- Endpoint `POST /templates/preview` — принимает шаблон + массив тестовых данных, возвращает массив рендеренных результатов с предупреждениями о длине
- В CampaignWizard: при выборе шаблона и списка контактов — автоматический превью на 5 случайных контактах из списка (включая контакты с пустыми полями)

### Валидация

- Синтаксическая проверка при сохранении шаблона
- Предупреждение при создании кампании, если max длина рендеринга превышает 1 SMS-сегмент
- Ошибка, если шаблон ссылается на атрибуты, которых нет в схеме выбранного contact_list

---

## WS2: Click Tracking (Link Service)

### Новый микросервис: link-service

Полностью новый сервис, не затрагивает существующие.

### Таблицы

```sql
-- Кастомные домены клиентов
client_domains
├── id (UUID)
├── client_id (FK)
├── domain (TEXT, UNIQUE) -- "go.clientbrand.com"
├── status (pending_dns|pending_ssl|active|failed)
├── dns_txt_record (TEXT) -- значение для TXT-верификации
├── dns_verified_at (TIMESTAMPTZ)
├── ssl_cert_path (TEXT)
├── ssl_expires_at (TIMESTAMPTZ)
├── created_at, updated_at

-- Короткие ссылки
short_links
├── id (UUID)
├── client_id (FK)
├── domain_id (FK → client_domains, nullable)
├── code (VARCHAR(10), UNIQUE) -- "abc123"
├── original_url (TEXT)
├── message_id (UUID, nullable) -- привязка к SMS
├── campaign_id (UUID, nullable)
├── recipient_id (UUID, nullable)
├── expires_at (TIMESTAMPTZ)
├── created_at

-- События кликов (партиционирование по месяцам)
click_events
├── id (UUID)
├── short_link_id (FK)
├── client_id (FK)
├── campaign_id (UUID, nullable)
├── phone (TEXT) -- номер получателя (из recipient)
├── clicked_at (TIMESTAMPTZ) -- partition key
├── ip_address (INET)
├── user_agent (TEXT)
├── referer (TEXT)
├── country_code (VARCHAR(2)) -- GeoIP
├── is_unique (BOOLEAN) -- первый клик от этого phone по этой ссылке
```

### Поток работы

```
1. СОЗДАНИЕ ССЫЛКИ (при отправке SMS)
   Campaign-service/pipeline-worker перед отправкой:
   ├── Сканирует текст SMS на наличие URL (regex)
   ├── Для каждого URL вызывает link-service.Shorten()
   ├── link-service генерирует код (base62, 6-8 символов)
   ├── Выбирает домен: client_domain если есть, иначе дефолтный
   └── Заменяет URL в тексте на короткую ссылку

2. РЕДИРЕКТ (при клике получателя)
   GET https://go.clientbrand.com/abc123
   ├── link-service ищет code в short_links
   ├── Записывает click_event (async, через Kafka topic "link.clicks")
   ├── Определяет is_unique (Redis SET: "clicked:{link_id}" → SADD phone)
   └── HTTP 302 → original_url

3. АГРЕГАЦИЯ (для A/B и аналитики)
   Kafka consumer в analytics-service:
   ├── Читает из "link.clicks"
   ├── Обновляет campaign_stats: click_count, unique_click_count
   ├── Обновляет variant stats: variant_click_count
   └── Доступно через GET /campaigns/:id/stats
```

### Кастомные домены — процесс привязки

```
1. Клиент добавляет домен "go.clientbrand.com" через портал
2. Система генерирует TXT-запись: _sms-verify.go.clientbrand.com → "verify=abc123"
3. Клиент добавляет TXT-запись в свой DNS
4. Cron-job (каждые 5 мин) проверяет DNS → status: pending_dns → pending_ssl
5. Система запрашивает Let's Encrypt сертификат (ACME HTTP-01 challenge)
6. CNAME: go.clientbrand.com → links.platform.com (наш сервер)
7. status: active, домен готов к использованию
8. Cron-job для обновления SSL за 30 дней до истечения
```

### HTTP-сервер редиректа

Отдельный lightweight HTTP-сервер (не через gorilla/mux gateway, а прямой):
- Слушает на порту 8085
- HAProxy проксирует запросы по домену на этот порт
- Минимальная логика: lookup code → redirect
- Latency target: < 10мс (Redis-кеш для hot links)

### gRPC API

```protobuf
service LinkService {
  rpc ShortenURL(ShortenRequest) returns (ShortenResponse);
  rpc ShortenBatch(ShortenBatchRequest) returns (ShortenBatchResponse);
  rpc GetLinkStats(LinkStatsRequest) returns (LinkStatsResponse);
  rpc GetClicksByRecipient(ClicksByRecipientRequest) returns (ClicksResponse);
}

service DomainService {
  rpc AddDomain(AddDomainRequest) returns (Domain);
  rpc VerifyDomain(VerifyDomainRequest) returns (Domain);
  rpc ListDomains(ListDomainsRequest) returns (ListDomainsResponse);
  rpc DeleteDomain(DeleteDomainRequest) returns (Empty);
}
```

### Kafka topics

```
link.clicks — сырые события кликов (producer: redirect-handler, consumer: analytics-service)
```

Analytics-service потребляет `link.clicks`, агрегирует и обновляет campaign_variants.click_count / unique_click_count через gRPC-вызов в campaign-service.

---

## WS3: Frequency Capping + Local Time Delivery

### 3A: Frequency Capping

#### Модель данных

```sql
-- Политики частоты на уровне аккаунта
frequency_caps
├── id (UUID)
├── client_id (FK, UNIQUE per cap_type)
├── cap_type (marketing|transactional|all)
├── max_messages (INT)
├── period_hours (INT)
├── enabled (BOOLEAN, default true)
├── created_at, updated_at

-- Переопределение на уровне кампании
campaign_frequency_overrides
├── campaign_id (FK, PK)
├── max_messages (INT, nullable) -- null = использовать аккаунтный
├── period_hours (INT, nullable)
├── bypass (BOOLEAN, default false) -- полностью отключить capping
```

#### Проверка — Redis sorted sets

```
Key:   freq:{client_id}:{phone}
Value: sorted set, score = unix timestamp отправки
TTL:   автоматический = max(period_hours) среди активных caps клиента

Проверка:
  ZREMRANGEBYSCORE freq:client1:79991234567 0 (now - period_hours*3600)
  ZCARD freq:client1:79991234567
  → если count >= max_messages → CAPPED

Запись после отправки:
  ZADD freq:client1:79991234567 {timestamp} {message_id}
```

#### Точка интеграции: pipeline-worker

Проверка встраивается в pipeline-worker перед отправкой:

```
Pipeline Worker: process message
├── 1. Определить cap_type (transactional если message.metadata["transactional"] == true)
├── 2. Получить frequency_cap клиента (Redis cache: "fcap:{client_id}", TTL 5 мин)
├── 3. Если campaign_id → проверить campaign_frequency_overrides
│   └── bypass == true → пропустить проверку
│   └── override values → использовать вместо аккаунтных
├── 4. Проверить Redis sorted set
│   └── count < max → ОТПРАВИТЬ, ZADD после отправки
│   └── count >= max → CAPPED
├── 5. Если CAPPED:
│   └── campaign_recipient.status = 'capped'
│   └── message.status = 'capped'
│   └── Инкремент campaign.capped_count
│   └── НЕ отправлять
```

#### Новый статус "capped"

- `campaign_recipients.status`: добавить `capped`
- `messages.status`: добавить `capped`
- Счётчик `capped_count` в `campaigns`

#### Транзакционные SMS

Сообщения с флагом `transactional` проверяются только по cap_type `transactional` или `all`. Рекомендация — не ставить лимит на transactional или ставить высокий (10/час).

#### Мониторинг

- Prometheus counter `sms_capped_total{client_id, cap_type}`
- Alert если capped > 20% от объёма кампании

### 3B: Local Time Delivery + Quiet Hours

#### Определение часового пояса

**Библиотека:** `github.com/nyaruka/phonenumbers` (Go-порт libphonenumber)

```
Цепочка определения:
1. phonenumbers.Parse(phone, "") → region code ("RU", "KZ", "US")
2. Для России: DEF-код → timezone (маппинг Россвязи)
3. Для остальных: region code → основной timezone
   Многозонные страны (US, CA, AU, BR): area code → timezone
4. Fallback: UTC
```

**Новый пакет:** `internal/shared/timezone/`
- `Resolver` — интерфейс: `GetTimezone(phone string) (*time.Location, error)`
- `PhoneTimezoneResolver` — реализация на phonenumbers + локальные маппинги
- Маппинги в embedded JSON (Go embed)
- Redis-кеш: `tz:{phone_prefix}` → timezone string (TTL 24h)

#### Модель данных

```sql
-- Новое поле в campaign_recipients
ALTER TABLE campaign_recipients ADD COLUMN deliver_at TIMESTAMPTZ;
-- NULL = немедленно, NOT NULL = отправить в указанное время

-- Quiet hours на уровне клиента
client_quiet_hours
├── client_id (FK, PK)
├── enabled (BOOLEAN, default true)
├── start_hour (INT, 0-23, default 22)
├── end_hour (INT, 0-23, default 8)
├── action (postpone|skip)
├── created_at, updated_at
```

#### Расчёт deliver_at при материализации

```
Для каждого recipient:
1. timezone = resolver.GetTimezone(phone)
2. desired_local = campaign.scheduled_at в local time получателя
3. deliver_at = time.Date(desired_local).In(UTC)

Пример:
  Кампания: scheduled_at = "2026-03-28 10:00 MSK"
  Владивосток (UTC+10): deliver_at = 00:00 UTC
  Москва (UTC+3): deliver_at = 07:00 UTC
```

#### Quiet Hours логика

```
После расчёта deliver_at:
1. Получить client_quiet_hours
2. Конвертировать deliver_at в local time
3. Проверить попадание в тихие часы:
   Overnight (start > end, напр. 22-8):
     quiet = local_hour >= start_hour OR local_hour < end_hour
   Daytime (start < end, напр. 13-15):
     quiet = local_hour >= start_hour AND local_hour < end_hour
4. Если quiet == true:
   action == "postpone" → deliver_at = следующий end_hour по local time
   action == "skip" → recipient.status = "skipped_quiet_hours"
```

#### Pipeline-worker — обработка deliver_at

```
IMMEDIATE (deliver_at IS NULL): отправить сразу
SCHEDULED (deliver_at IS NOT NULL):
  Worker каждые 30 сек:
    SELECT FROM campaign_recipients
    WHERE status = 'pending' AND deliver_at <= now()
    ORDER BY deliver_at LIMIT {batch_size}
```

#### Порядок pre-send checks в pipeline-worker

```
1. deliver_at check → если не наступило, skip
2. frequency capping → если capped, mark & skip
3. template rendering → если rendered_text пуст, подставить данные; иначе использовать готовый
4. link shortening → заменить URL на короткие ссылки
5. send to provider
```

#### UI

- Кампания: переключатель «Отправить по локальному времени»
- Настройки аккаунта: «Тихие часы» (start/end, действие)
- Статистика: гистограмма deliver_at по часовым поясам

---

## WS4: A/B Test-Then-Send + Smart Segments

### 4A: A/B Test-Then-Send

#### Расширение модели данных

```sql
ALTER TABLE campaign_ab_config ADD COLUMN strategy VARCHAR(20) DEFAULT 'full_split';
-- 'full_split' (все сразу) | 'test_then_send' (тест → rollout)

ALTER TABLE campaign_ab_config ADD COLUMN test_percentage INT DEFAULT 100;
ALTER TABLE campaign_ab_config ADD COLUMN test_phase VARCHAR(20) DEFAULT 'none';
-- none | testing | waiting_winner | rollout | completed

ALTER TABLE campaign_ab_config ADD COLUMN winning_metric VARCHAR(20) DEFAULT 'delivery_rate';
-- delivery_rate | click_rate | unique_click_rate

ALTER TABLE campaign_ab_config ADD COLUMN test_started_at TIMESTAMPTZ;
ALTER TABLE campaign_ab_config ADD COLUMN rollout_started_at TIMESTAMPTZ;

-- Click-метрики в вариантах
ALTER TABLE campaign_variants ADD COLUMN click_count INT DEFAULT 0;
ALTER TABLE campaign_variants ADD COLUMN unique_click_count INT DEFAULT 0;

ALTER TABLE campaign_stats_snapshots ADD COLUMN clicks INT DEFAULT 0;
ALTER TABLE campaign_stats_snapshots ADD COLUMN unique_clicks INT DEFAULT 0;
```

#### Lifecycle: Test-Then-Send

```
1. МАТЕРИАЛИЗАЦИЯ (launch)
   ├── test_percentage (например 10%) от базы → распределить по вариантам
   ├── Остальные 90% → status = 'pending_rollout'
   ├── test_phase = 'testing', test_started_at = now()

2. ТЕСТОВАЯ ФАЗА (testing)
   ├── Отправить только тестовую группу
   ├── Собирать delivery_rate, click_rate
   └── Ждать test_duration_hours

3. ВЫБОР ПОБЕДИТЕЛЯ
   Автоматический (auto_select_winner = true):
   ├── Cron каждые 5 мин: проверка кампаний с test_phase='testing'
   │   AND test_started_at + duration < now()
   ├── Сравнить варианты по winning_metric
   ├── При равенстве (<1% разницы) → вариант с большей выборкой
   └── test_phase = 'rollout'

   Ручной (auto_select_winner = false):
   ├── test_phase = 'waiting_winner'
   ├── Уведомление клиенту (webhook + UI)
   └── POST /campaigns/:id/select-winner

4. ROLLOUT
   ├── UPDATE recipients SET variant_id=winner, status='pending'
   │   WHERE status='pending_rollout'
   ├── Рендерить текст по шаблону победителя
   └── Отправлять с rate limiting
```

#### Статистическая значимость

```
Минимум для автовыбора:
  - Каждый вариант >= 100 отправленных
  - Разница > 2% по метрике

Недостаточно данных:
  auto → выбрать вариант A с предупреждением
  manual → уведомить клиента
```

#### Новые статусы

- `campaign_recipients.status`: добавить `pending_rollout`
- `campaign_ab_config.test_phase`: `none|testing|waiting_winner|rollout|completed`

#### API

```
PUT  /campaigns/:id/ab-config — { strategy, test_percentage, test_duration_hours, auto_select_winner, winning_metric }
GET  /campaigns/:id/ab-status — { test_phase, variants_comparison, time_remaining, winner }
POST /campaigns/:id/select-winner — { variant_id } (только при waiting_winner)
GET  /campaigns/:id/variants/compare — расширен: click_rate, statistical_significance
```

### 4B: Smart Segments

#### Модель данных

```sql
saved_segments
├── id (UUID)
├── client_id (FK)
├── name (TEXT)
├── description (TEXT)
├── contact_list_ids (UUID[])
├── rules (JSONB)
├── tag_rules (JSONB)
├── estimated_count (INT)
├── estimated_at (TIMESTAMPTZ)
├── created_at, updated_at
```

#### Формат правил (rules JSONB)

```json
{
  "operator": "AND",
  "conditions": [
    {
      "field": "attributes.last_purchase_date",
      "op": "older_than_days",
      "value": 90
    },
    {
      "field": "attributes.city",
      "op": "in",
      "value": ["Москва", "Санкт-Петербург"]
    },
    {
      "operator": "OR",
      "conditions": [
        { "field": "attributes.age", "op": "gte", "value": 25 },
        { "field": "attributes.spending", "op": "gte", "value": 10000 }
      ]
    }
  ]
}
```

**Операторы:**

| op | Описание | Типы |
|----|----------|------|
| `eq`, `neq` | Равно / не равно | все |
| `gt`, `gte`, `lt`, `lte` | Сравнение | number, date |
| `in`, `not_in` | Входит в список | string, number |
| `contains`, `not_contains` | Подстрока | string |
| `contains_any`, `contains_all` | Пересечение массивов | tags |
| `is_empty`, `is_not_empty` | Пустое / непустое | все |
| `older_than_days`, `newer_than_days` | Относительная дата | date |

#### Кросс-списковая логика

```
1. Пользователь выбирает contact_list_ids
2. Объединённая схема атрибутов:
   List A: {name, city, age}
   List B: {name, phone_type, spending}
   Union:  {name, city, age, phone_type, spending}
3. Контакты без атрибута получают NULL (не проходят фильтр по этому полю)
4. UI показывает покрытие: "доступно в 1 из 2 списков"
```

#### SQL-генерация

```sql
SELECT DISTINCT ON (c.phone) c.*
FROM contacts c
WHERE c.contact_list_id = ANY($1::uuid[])
  AND (c.attributes->>'last_purchase_date')::date < (CURRENT_DATE - INTERVAL '90 days')
  AND c.attributes->>'city' IN ('Москва', 'Санкт-Петербург')
  AND c.tags && ARRAY['vip', 'loyal']
ORDER BY c.phone, c.updated_at DESC
```

Правила транслируются в параметризованные запросы — защита от SQL injection.

#### Дедупликация

`DISTINCT ON (c.phone)` с `ORDER BY c.updated_at DESC` — берётся самая свежая запись.

#### Интеграция с кампаниями

```sql
ALTER TABLE campaigns ADD COLUMN segment_id UUID REFERENCES saved_segments(id);
-- segment_id приоритетнее segment_rules + segment_tags
```

#### API

```
POST   /segments              — создать
GET    /segments              — список
GET    /segments/:id          — детали
PUT    /segments/:id          — обновить
DELETE /segments/:id          — удалить
POST   /segments/:id/estimate — пересчитать count
POST   /segments/preview      — count без сохранения
GET    /segments/:id/schema   — объединённая схема
```

#### UI

- Новая страница «Сегменты» в sidebar
- SegmentBuilder: мультиселект списков, объединённая схема, live-count, вложенные AND/OR
- CampaignWizard: выбор между списком+фильтры и сохранённым сегментом

---

## UI: общие изменения портала

### Новые страницы

| Страница | Path | Описание |
|----------|------|----------|
| Сегменты | `/segments` | CRUD сегментов |
| Детали сегмента | `/segments/:id` | Правила + превью |
| Домены | `/settings/domains` | Управление кастомными доменами |

### Изменения в существующих страницах

| Страница | Изменения |
|----------|-----------|
| CampaignWizard | Шаг шаблона: Liquid-превью. Шаг аудитории: выбор сегмента. Шаг A/B: strategy selector, test_percentage. |
| CampaignDetail | A/B status panel, click metrics, capped count. |
| Настройки аккаунта | Frequency caps config, quiet hours config. |
| Sidebar | Новый пункт «Сегменты» между Contacts и Campaigns. |

### Новые компоненты

| Компонент | Описание |
|-----------|----------|
| `TemplatePreview` | Рендеринг шаблона на реальных контактах, предупреждения о длине |
| `ABStatusPanel` | Фаза теста, таймер, сравнение вариантов, кнопка выбора победителя |
| `FrequencyCapForm` | Настройка лимитов marketing/transactional |
| `QuietHoursForm` | Настройка тихих часов |
| `DomainManager` | Добавление домена, DNS-инструкция, статус верификации |

---

## Новые сервисы и зависимости

### link-service (новый микросервис)

- gRPC: порт 9102
- HTTP redirect: порт 8085
- Зависимости: PostgreSQL, Redis, Kafka
- Docker: `deployments/docker/link-service.Dockerfile`

### Новые Go-зависимости

| Пакет | Назначение |
|-------|-----------|
| `github.com/osteele/liquid` | Liquid template engine |
| `github.com/nyaruka/phonenumbers` | Phone number parsing, timezone |
| `golang.org/x/crypto/acme/autocert` | Let's Encrypt автоматизация |

### Новые Kafka topics

| Topic | Producer | Consumer |
|-------|----------|----------|
| `link.clicks` | link-service (redirect handler) | analytics-service |

### Новые миграции БД

| Миграция | Таблицы |
|----------|---------|
| `000045_frequency_caps.up.sql` | `frequency_caps`, `campaign_frequency_overrides` |
| `000046_link_service.up.sql` | `client_domains`, `short_links`, `click_events` (partitioned) |
| `000047_local_time_delivery.up.sql` | `client_quiet_hours`, ALTER `campaign_recipients` ADD `deliver_at` |
| `000048_ab_test_then_send.up.sql` | ALTER `campaign_ab_config`, ALTER `campaign_variants`, ALTER `campaign_stats_snapshots` |
| `000049_smart_segments.up.sql` | `saved_segments`, ALTER `campaigns` ADD `segment_id` |
| `000050_capped_status.up.sql` | ALTER status enums: add `capped`, `pending_rollout`, `skipped_quiet_hours`; ALTER `campaigns` ADD `capped_count` |
| `000051_rendered_text.up.sql` | ALTER `campaign_recipients` ADD `rendered_text` |
