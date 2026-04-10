# Многомерная аналитика для клиентского портала

**Дата:** 2026-04-10  
**Статус:** Approved  
**Область:** portal-frontend, internal/gateway/portal, internal/services/analytics, api/proto/analytics

---

## Контекст

Текущая страница аналитики портала предоставляет клиенту только два измерения: хронологию (день/неделя) и разбивку по странам. Клиент не может получить информацию в разрезе операторов, отправителей, кампаний или детальных статусов, а также не может провалиться до уровня конкретных сообщений. Цель — дать клиенту максимум информации о его рассылках в любой комбинации измерений.

---

## Дизайн

### 1. Панель фильтров

Два ряда фильтров:

**Ряд 1 — временной диапазон** (без изменений):
- Быстрые кнопки: 7d / 30d / 90d
- Произвольный диапазон: date_from + date_to (макс. 366 дней)
- Чекбокс "Сравнить с предыдущим периодом"

**Ряд 2 — измерения** (новые мультиселекты, все опциональны):
- **Страна** — из данных клиента
- **Оператор** — список операторов (каскадная фильтрация при выборе страны)
- **Отправитель** — sender names клиента
- **Кампания** — список рассылок клиента
- **Статус** — multiselect: delivered / failed / expired / rejected / queued

Логика фильтрации: AND между разными фильтрами, OR внутри каждого мультиселекта. Кнопка "Сбросить фильтры" справа.

**Группировка** (отдельный селект — определяет ключ строки в таблице и ось X графика):
- `day` | `week` | `country` | `operator` | `sender` | `campaign` | `status`

---

### 2. Сводные карточки и графики

**Карточки** (пересчитываются по активным фильтрам):
- Отправлено
- Доставлено
- Ошибки
- Доставляемость (%)
- Стоимость (RUB)
- Среднее время доставки (мс → форматируется в с или мин)

**Основной график:**
- Если группировка = `day` / `week` → LineChart, ось X = время
- Если группировка = `operator` / `sender` / `campaign` / `country` / `status` → BarChart горизонтальный, ось X = названия сущностей
- Два ряда: Отправлено + Доставлено
- При compare=true добавляются пунктирные ряды предыдущего периода

**Donut chart статусов** (рядом с основным графиком):
- Показывает соотношение: delivered / failed / expired / rejected
- Кликабелен: клик на сектор добавляет статус в фильтр

---

### 3. Таблица агрегатов

Колонки: `[Ключ группировки]` | Отправлено | Доставлено | Ошибки | Истекло | Доставляемость | Среднее время | Стоимость

- Все колонки сортируемые
- Название первой колонки меняется в зависимости от группировки: "Дата" / "Оператор" / "Отправитель" / "Кампания" / "Страна" / "Статус"

**Drill-down (боковая панель / drawer):**
- Открывается кликом на строку таблицы
- Таблица агрегатов остаётся видна (drawer поверх, не модал)
- Содержимое drawer:
  - Заголовок: `{ключ} · {период}` (например "МТС · 08.04.2026")
  - Мини-сводка: 6 метрик для этой строки
  - Таблица сообщений с пагинацией (50 на страницу):
    - ID сообщения
    - Телефон (маскирован: `+7***1234`, маскировка на бэкенде)
    - Отправитель
    - Статус
    - Время отправки
    - Время доставки
    - Код ошибки (если есть)
  - Ссылка "Открыть сообщение" → переход на карточку сообщения
- Данные загружаются при открытии drawer (lazy), не заранее

---

### 4. Бэкенд: изменения в proto

Файл: `api/proto/analytics/analytics.proto`

**Расширение `GetStatisticsRequest`:**
```protobuf
repeated string operator_ids  = 6;   // фильтр по операторам
repeated string sender_names  = 7;   // фильтр по отправителям
repeated string campaign_ids  = 8;   // фильтр по кампаниям
repeated string statuses      = 9;   // фильтр по статусам
// group_by расширяется: day, week, hour, country, operator, sender, campaign, status
```

**Новый метод `GetMessagesList`:**
```protobuf
rpc GetMessagesList(GetMessagesListRequest) returns (GetMessagesListResponse);

message GetMessagesListRequest {
  string client_id      = 1;
  google.protobuf.Timestamp from = 2;
  google.protobuf.Timestamp to   = 3;
  repeated string operator_ids  = 4;
  repeated string sender_names  = 5;
  repeated string campaign_ids  = 6;
  repeated string statuses      = 7;
  string dimension_key  = 8;   // значение строки drill-down (напр. "mts" или "2026-04-08")
  string dimension_type = 9;   // тип ключа: operator/sender/campaign/country/status/date
  int32  page           = 10;
  int32  page_size      = 11;  // макс 50
}

message GetMessagesListResponse {
  repeated MessageRecord messages = 1;
  int32 total = 2;
}

message MessageRecord {
  string message_id         = 1;
  string phone_masked       = 2;   // маскировка на бэкенде
  string sender_name        = 3;
  string status             = 4;
  google.protobuf.Timestamp sent_at      = 5;
  google.protobuf.Timestamp delivered_at = 6;
  string error_code         = 7;
}
```

---

### 5. Бэкенд: изменения в портальном HTTP API

Файл: `internal/gateway/portal/handlers/analytics.go`

**`GET /analytics` — новые query-параметры:**
- `operator_ids` — comma-separated
- `sender_names` — comma-separated
- `campaign_ids` — comma-separated
- `statuses` — comma-separated (allowed: delivered, failed, expired, rejected, queued)
- `group_by` расширяется до: day, week, country, operator, sender, campaign, status

Валидация: каждый параметр парсится и проверяется whitelist-ом перед передачей в gRPC.

**Новые справочные endpoints для мультиселектов:**
- `GET /analytics/filter-options` — возвращает списки `operators`, `sender_names`, `campaigns` для текущего клиента; вызывается при монтировании страницы

**Новый endpoint `GET /analytics/messages`:**
- Те же параметры + `dimension_key`, `dimension_type`, `page`, `page_size`
- Передаёт в `analyticsClient.GetMessagesList`
- `page_size` ограничивается до 50 на бэкенде

---

### 6. Бэкенд: изменения в сервисе аналитики

Файл: `internal/services/analytics/application/analytics_service.go` и репозиторий

- Расширить `StatisticsFilters` новыми полями: `OperatorIDs`, `SenderNames`, `CampaignIDs`, `Statuses`
- Расширить SQL в `MetricRepository.GetStatistics` для поддержки новых GROUP BY и WHERE условий
- Реализовать `GetMessagesList` в репозитории: JOIN с таблицей messages, маскировка телефона (`regexp_replace` или substring в SQL), пагинация через LIMIT/OFFSET

---

### 7. Фронтенд: изменения

Файл: `portal-frontend/src/pages/analytics/AnalyticsPage.tsx`

- Добавить состояния для новых фильтров: `operatorIds`, `senderNames`, `campaignIds`, `statuses`
- Добавить загрузку справочников для мультиселектов (операторы, отправители, кампании) — три отдельных небольших API-вызова при монтировании
- Переключить график между LineChart и BarChart в зависимости от группировки
- Добавить Donut chart (Recharts `PieChart`) для статусов
- Реализовать Drawer-компонент (Radix UI `Dialog` или кастомный) для drill-down
- Lazy-загрузка сообщений в drawer через новый `/analytics/messages`

---

## Что НЕ входит в scope

- Экспорт в CSV/Excel (отдельная задача)
- Real-time обновление без перезагрузки страницы
- Алерты и пороговые уведомления
- Сравнение между двумя произвольными периодами (только "текущий vs предыдущий")

---

## Зависимости

- Таблица `messages` должна содержать `operator_id`, `sender_name`, `campaign_id` — нужно проверить наличие этих полей и при необходимости добавить в метрики
- Таблица `operators` должна быть доступна из аналитического сервиса для получения имён операторов
