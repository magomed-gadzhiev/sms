# Раздел `/network/statistics` — корректность данных и UX overhaul

**Дата:** 2026-05-08
**Скоуп:** Раздел «Сеть → Статистика» (вкладки `Статистика`, `Аналитика`, `Мониторинг`) в портале агрегатора. Backend (тарификация, агрегатор `network_stats_hourly`, провайдер-биллинг, monitoring collector), API-контракт, frontend (формат метрик, графики trends, saved views, видимый UX, мобильный responsive).

**Single source of truth для writing-plans.**

---

## 1. Контекст и проблема

Аудит раздела `/network/statistics` (выполнен 2026-05-08) обнаружил три независимых класса дефектов на одном экране:

1. **Финансы недостоверны — два независимых источника проблемы.**
   - **(а) Реальный bug агрегатора.** SQL-агрегатор `network_stats_hourly` в [aggregation_worker.go:62-63](internal/services/network_analytics/application/aggregation_worker.go#L62-L63) хардкодит `0 AS revenue, 0 AS cost` — даже корректно записанные `tarification_log.total_amount` не доходят до KPI. Это устраняется в Slice 1.
   - **(б) Provider tarification — незавершённая фича.** `provider_tarification_log` полностью пуст за всю историю — сервис `TarifyProviderCost` написан (миграция 000039 от 2026-03-27), но не имеет caller'ов в pipeline. Это устраняется в Slice 2.
   - **(в) ОПРОВЕРГНУТАЯ гипотеза.** Изначальный аудит видел в UI «172 из 192 delivered не тарифицированы» и предположил регресс тарификатора. Дальнейшее расследование (subagent static analysis + SQL evidence) доказало: **регресса нет**. 250 из 271 «нетарифицированных» delivered-сообщений — это **тестовые данные**, посеянные напрямую SQL'ем во время manual testing 2026-05-04 12:00–12:50 (ни Go-код, ни триггеры не пишут `messages.send_method='api/campaign'`; default = NULL). Реальный трафик через pipeline тарифицируется на 100%. Spec обновлён 2026-05-08 после fact-check'а, эта пометка оставлена для исторической прозрачности.

2. **API ↔ UI рассинхрон.** API возвращает `0.16974` для KPI «Ошибки» — UI рендерит «0,17» без знака процента. KPI без `value` (Выручка/Себестоимость/Прибыль) рендерятся как «0 ₽» зелёным («ok»), что прямо лжёт пользователю — данных нет, UI говорит «всё хорошо». Endpoint `DELETE /reseller/views/{id}` возвращает `500 INTERNAL_ERROR` при попытке удалить шаблонный вид (должен быть `403 VIEW_IS_TEMPLATE`). `period_preset=banana` не валидируется (тихий fallback), `channel=SMS` не находит ничего из-за case-sensitivity (`sms` в БД).

3. **Monitoring — моки.** Live-метрики провайдеров читаются из Redis ключей `network:{partner}:provider:{id}`. **Read-side есть, write-side отсутствует.** Текущие значения посеяны вручную, статичны, не отражают реальный трафик (показано 91 msg/s суммарно, в БД за 7 дней ~270 сообщений = 0.0004 msg/s).

**Цель:** закрыть все три класса в одной зонтичной спеке через **vertical slices** — каждый slice от БД до UI, мержится самостоятельно, даёт видимое улучшение.

---

## 2. Зафиксированные решения брейншторма

| # | Развилка | Выбор |
|---|---|---|
| 1 | ~~Тарификатор: фикс или переписать~~ | ~~Регресс, найти и откатить~~ → **ОПРОВЕРГНУТО** fact-check'ом. Регресса нет. Решение удалено из плана. |
| 2 | Provider tarification: scope | **A-full:** подключить `TarifyProviderCost` в pipeline + полный админ UI для CRUD provider cost periods/tiers |
| 3 | Backfill `network_stats_hourly` | **B:** TRUNCATE + backfill за апрель–сегодня. До апреля — оставляем как есть (платформа была без полной тарификации) |
| 4 | Voice-канал | **A:** добавить `voice` как полноценный канал в селектор (будущая фича Voice OTP / IVR) |
| 5 | Monitoring | **A:** full live-collector в Redis из pipeline + отдельный воркер для расчётов + snapshot saver в PostgreSQL |
| 6 | Saved views | **A:** templates immutable; кнопка `📋` clone вместо `×` для template'ов |
| 7 | Подача плана | **Vertical slices** (vs горизонтальные слои или risk-first) |
| 8 | Метрика «format» | Backend в KPI-объект API добавляет поле `format: "count" \| "percent" \| "currency"`; фронт рендерит по нему |
| 9 | Pipeline-врезка для provider cost | После `status` stage когда `delivered/expired/failed` (uplink-биллинг, как у клиента) |
| 10 | Cost backfill | **Forward-only:** считаем cost только с момента активации тарифа провайдера. Историю не пересчитываем |
| 11 | Импорт CSV для provider tariffs | **Backlog** — не в этой спеке |
| 12 | `UpsertHourlyStats` семантика | Меняем `add → replace`, единая семантика, без дубль-API |
| 13 | Backfill утилита | Отдельный CLI `cmd/network-stats-backfill/main.go` (vs subcommand seed-admin) |
| 14 | Pending в Redis collector | Один SQL-запрос с `GROUP BY provider_id`, фан-аут по Redis (vs per-provider QPS) |
| 15 | Active conns | Новый gRPC-метод `GetActiveConns(provider_id) → int` в `cmd/smpp-server` |
| 16 | Где запускать collector | Отдельный сервис `cmd/monitoring-collector/main.go` (vs внутри network-analytics-service) |
| 17 | Mobile responsive | В Slice 4 (без выделения 4b) |
| 18 | Раскраска чисел в ячейках | Оставляем с tooltip-объяснением порогов (vs убрать совсем) |

---

## 3. Архитектура — общая модель данных

### 3.1 Изменения в `network_stats_hourly`

Колонки `revenue` (numeric) и `cost` (numeric) уже есть в таблице. Меняется **источник данных** — не хардкод 0, а JOIN'ы:

- `revenue = SUM(tarification_log.total_amount)` где `tarification_log.message_id = messages.id`
- `cost = SUM(provider_tarification_log.total_amount)` где `provider_tarification_log.message_id = messages.id`
- `profit = revenue - cost`
- `margin = (revenue - cost) / revenue` если `revenue > 0`, иначе `NULL`

Семантика `UpsertHourlyStats` меняется с накопительной (`v = stats.v + EXCLUDED.v`) на replace (`v = EXCLUDED.v`). Это позволяет ре-агрегации не удваивать значения и обеспечивает eventual consistency для запоздалой тарификации.

### 3.2 Provider tarification — данные

Используются **существующие** таблицы (миграция `000039_create_provider_tarification` от 2026-03-27):
- `provider_tariff_plans` — план биллинга от провайдера (provider_id × operator_id × strategy)
- `provider_tariff_periods` — действие плана во времени
- `provider_tariff_tiers` — пороги и цены
- `provider_tarification_log` — лог биллинга (запись на каждое успешно отправленное сообщение)
- `provider_usage_counters` — счётчики для threshold-стратегий

Новых таблиц для provider tarification не нужно — миграция уже накатана в марте.

### 3.3 Saved views — модель прав

Таблица `saved_views(id, partner_id, user_id, name, mode, filters, group_by, sort_by, sort_dir, columns, is_template, is_default)` уже есть. Меняется **handler-логика**:

- `is_template = true` AND `user_id IS NULL` → системный пресет, все агрегаторы видят, но не могут удалить.
- DELETE на template → `403 VIEW_IS_TEMPLATE` с подсказкой «Системный пресет нельзя удалить, можно клонировать».
- Новый endpoint `POST /reseller/views/{id}/clone` создаёт копию template'а с `is_template=false, user_id=current.id, name="<original> (копия)"`.

### 3.4 Monitoring — данные

**Redis ключи:**
- `network:{partner_id}:provider:{provider_id}` — HSET с полями `provider_name, health_status, throughput, queue_depth, pending, error_count, timeout_count, latency_p50, latency_p95, active_conns, last_seen`. TTL 60 секунд после последнего обновления.
- `latency:{partner_id}:{provider_id}` — sorted set с временным окном 60s для расчёта p50/p95.
- Inline-счётчики `HINCRBY` в pipeline (sender, status stage'и).

**PostgreSQL:** существующая `network_monitoring_snapshot` (миграция уже есть, см. SaveSnapshot в `monitoring_repo.go`).

---

## 4. Slices — детализация

### Slice 0 — Diagnostic Hotfix (0.3 дня)

**Цель:** UI перестаёт лгать без починки источников. После fact-check'а 2026-05-08 объём slice'а сокращён с 1 дня до 0.3 дня — D2 (регресс тарификатора) удалён, потому что регресса нет.

**Backend:**
- **F1:** в `views_repository.DeleteView` добавить domain-ошибки `ErrViewNotFound`, `ErrViewIsTemplate`, `ErrViewForbidden`. Handler в `network_statistics.go` маппит их в 404/403/403; всё остальное → 500.
- **F2 формат:** в KPI builder (`internal/services/network_analytics/application/service.go`) добавить поле `format` в KPI-DTO. Значения: `count, percent, currency`. Для существующих KPI выставить корректные значения.

**Frontend:**
- **F3:** в KPI-renderer'е (компонент в `portal-frontend/src/pages/network/`) изменить логику: `value === undefined || value === null` → рендерить `—` со статусом `unknown`. `value === 0` — это валидное значение, рендерить как `0`.
- **F2:** в KPI-renderer'е добавить ветки по `format` — `formatPercent(value * 100)` для `percent`, `formatCurrency(value, "RUB")` для `currency`, `formatCount(value)` для `count`.

**Acceptance:**
- KPI «Ошибки» в UI = `17,0%`, не `0,17`.
- KPI «Выручка/Себестоимость/Прибыль» = `—`, не `0 ₽` зелёным.
- DELETE template view → 403 с понятным сообщением в UI.

**Что было удалено из Slice 0** (после fact-check'а 2026-05-08): задачи D2 регресс bisect, точечный фикс и reseed-CLI. Регресса в тарификаторе нет — это была ошибка интерпретации аудита (тестовый seed принят за реальный трафик). См. раздел 1 пункт 1(в).

---

### Slice 1 — Выручка (3 дня)

**Цель:** KPI «Выручка» показывает реальные числа за апрель–сегодня.

**Backend:**
- **D1 SQL агрегатор** ([aggregation_worker.go:43-71](internal/services/network_analytics/application/aggregation_worker.go#L43-L71)): расширить `rawAggQuery`:
  - `LEFT JOIN tarification_log t ON t.message_id = m.id`
  - `COALESCE(SUM(t.total_amount), 0) AS revenue`
  - `0 AS cost` оставляем — это Slice 2.
- **`UpsertHourlyStats` смена семантики:** `INSERT ... ON CONFLICT DO UPDATE SET v = EXCLUDED.v` (replace). Регресс-тест: двойной прогон того же часа не удваивает значения.
- **Backfill утилита:** новый `cmd/network-stats-backfill/main.go` с флагами `--from`, `--to`, `--truncate`, `--partner`. Использует существующий `aggregation_worker.BackfillWindow(from, to)`. Прогресс в stdout (час за часом, ETA).
- **Скрипт-обёртка:** `scripts/run-backfill-april.sh` для текущего slice'а с дефолтами `--from 2026-04-01 --to now --truncate`.

**Frontend:**
- В backend KPI builder для «Выручка» при наличии данных — `format: "currency", currency: "RUB"`.
- Фронт-форматтер `formatCurrency(value, currency)` (если уже нет — добавить).

**Acceptance:**
- На sandbox после деплоя: `network_stats_hourly.revenue` для часов после деплоя сходится с `tarification_log.total_amount` за тот же период по тому же партнёру.
- После прогона backfill: суммарная revenue в `network_stats_hourly` за апрель = `SUM(tarification_log.total_amount) WHERE created_at >= '2026-04-01'` (с допуском на partner-фильтрацию).
- UI на `/network/statistics?period_preset=month`: «Выручка» показывает ненулевую сумму.
- Двойной прогон агрегатора на тот же час не удваивает revenue.

**Жертва:** до 1 апреля выручка остаётся `0 ₽`. Документировано как «исторические данные без полной тарификации».

---

### Slice 2 — Себестоимость и Прибыль (5–7 дней)

**Цель:** KPI «Себестоимость», «Прибыль», «Маржа» работают на тарифицированных провайдерах. Админ может настраивать provider cost.

**Backend — pipeline-врезка (D3.1):**
- **`internal/pipeline/status/stage.go`:** после успешной клиентской тарификации (`SenderService.TarifyClientCost`) для статусов `delivered/expired/failed` вызывается `ProviderTarificationService.TarifyProviderCost` с `provider_id` из `messages.provider_id`, `idempotency_key = 'provider:'+message.id`.
- **Обработка ошибок:** `tariff_plan` не найден — graceful skip (уже в сервисе). Любая другая ошибка — `log.Error` + метрика `provider_tarification_errors_total{reason}` для алертинга, pipeline не валится.
- **Wiring:** в `cmd/services/tarification-service/main.go` — `ProviderTarificationService` уже инстанциируется (строки 108+). Нужно прокинуть в `status` stage или вызвать через RPC.

**Backend — SQL агрегатор (D3.2):**
- В `rawAggQuery` добавить `LEFT JOIN provider_tarification_log pt ON pt.message_id = m.id` + `COALESCE(SUM(pt.total_amount), 0) AS cost`.
- В `stats_repository.GetStatsRows`: убрать хардкод 0, считать `profit = revenue - cost`. Margin: `CASE WHEN revenue > 0 THEN (revenue - cost) / revenue ELSE NULL END`.

**Backend — admin API:**
- Новые handler'ы в `internal/gateway/admin/handlers/provider_cost.go`:
  - `GET /admin/network/provider-cost/plans` (list)
  - `POST /admin/network/provider-cost/plans` (create)
  - `PATCH /admin/network/provider-cost/plans/{id}` (update)
  - `DELETE /admin/network/provider-cost/plans/{id}` (soft delete)
  - аналогично `/periods` и `/tiers`.
- Application-сервис `internal/services/tarification/application/provider_cost_admin_service.go` — валидации (периоды не пересекаются, tiers по возрастанию `from_count`).
- Repositories `provider_cost_*_repository.go` — частично существуют, дополнить методами CRUD.
- RBAC: middleware на admin-gateway. Агрегатор → 403.

**Frontend — admin страницы (D3.4):**
- Новая страница `portal-frontend/src/pages/admin/tarification/provider-cost/`:
  - `ProviderCostPage.tsx` — корневой роутер вкладок.
  - `ProviderCostPlansTab.tsx` — таблица планов: `name, provider, operator, strategy, is_active`. CRUD-формы.
  - `ProviderCostPeriodsTab.tsx` — для выбранного плана: периоды действия (`valid_from`, `valid_to`).
  - `ProviderCostTiersTab.tsx` — для выбранного периода: tiers (`from_count`, `price_per_segment`).
- API-клиент `portal-frontend/src/api/providerCost.ts`.
- Роутинг: `/admin/network/provider-cost` в `App.tsx` + ссылка в `AdminSidebar.tsx`.

**Frontend — KPI:**
- В backend для KPI «Себестоимость», «Прибыль» — `format: "currency"`. Для «Маржа» — `format: "percent"`. Если `value === null` → UI рендерит `—` (логика из Slice 0 уже работает).
- Tooltip на «Себестоимость»: «Себестоимость считается только для трафика после активации тарифа провайдера».

**Acceptance:**
- На sandbox создан тестовый тариф для одного провайдера (например, Provider-Beeline-RU) с `price_per_segment=1.0`. После 5 минут трафика `provider_tarification_log` имеет соответствующие записи.
- KPI «Себестоимость» > 0 для агрегатора с трафиком через этот провайдер.
- KPI «Прибыль» = «Выручка − Себестоимость», знак корректный.
- KPI «Маржа» — `(rev-cost)/rev` для тарифицированных провайдеров; `—` для остальных.
- Admin UI: CRUD работает на всех трёх уровнях (plan/period/tier). Удалить план — каскадно периоды и tiers (через FK ON DELETE CASCADE или application).
- RBAC: агрегатор не имеет доступа к `/admin/network/provider-cost/*` (403).

**Жертва:** до момента ввода первого тарифа cost остаётся `—` для всех. Forward-only, исторические периоды не billятся.

---

### Slice 3 — Monitoring Live (3–5 дней)

**Цель:** заменить моки в Redis настоящим collector'ом. Live-метрики провайдеров обновляются каждые 5 секунд; раз в минуту дублируются в `network_monitoring_snapshot`.

**Backend — inline-инструментация в pipeline:**
- `internal/pipeline/sender/stage.go`: при успешном `submit_sm` — `HINCRBY network:{partner}:provider:{provider} sent_total 1`, `HSET ... last_seen {unix}`.
- `internal/pipeline/status/stage.go`: при `failed` — `HINCRBY ... error_total 1`. При `expired` — `HINCRBY ... timeout_total 1`. Записать latency_us в `ZADD latency:{partner}:{provider} {now_ms} {latency_us}`, периодически `ZREMRANGEBYSCORE` старше 60s.
- Все Redis-вызовы — fire-and-forget через горутину с logged error. Метрика `monitoring_redis_errors_total` для алертинга.
- partner_id определяется по `client.parent_client_id` collapsing'у (тот же rule, что в `aggregation_worker`).

**Backend — collector worker:**
- Новый сервис `cmd/monitoring-collector/main.go`. Тикает каждые 5 секунд:
  1. `SCAN` по `network:*:provider:*`.
  2. Считает throughput по разнице счётчиков `(sent_now - sent_prev) / 5s`.
  3. `ZRANGE` по latency-набору, считает p50/p95 в Go.
  4. Один батч-SQL: `SELECT provider_id, count(*) FROM messages WHERE status='pending' AND created_at > now() - interval '24 hours' GROUP BY 1`. Кэш на 30 секунд.
  5. gRPC к `cmd/smpp-server` за списком active_conns: новый метод `GetActiveConns(provider_ids []uuid)` → `map[uuid]int`. Reuse существующего gRPC-клиента.
  6. Health: `throughput=0 AND last_seen > 5min` → `degraded`; `error_rate > 5%` → `warning`; иначе `ok`.
  7. `HSET` финальный набор полей в Redis. TTL 60s.

**Backend — snapshot worker:**
- Новый воркер `internal/services/network_analytics/application/snapshot_worker.go`. Регистрируется в `cmd/services/network-analytics-service/main.go`. Тикает каждые 60 секунд: `SCAN` Redis → batch `INSERT INTO network_monitoring_snapshot (...)` через существующий `MonitoringRepo.SaveSnapshot`.

**Backend — smpp-server gRPC:**
- В `cmd/smpp-server` экспортировать `GetActiveConns(provider_ids []uuid) → map[uuid]int` через gRPC. Подсчёт ведётся в существующем session-tracker'е.
- Proto: новое RPC в `api/proto/smpp_server/`.

**Cleanup:**
- `scripts/cleanup-monitoring-mocks.sh`: `redis-cli` `DEL network:*:provider:*`. Запускается один раз перед деплоем Slice 3.

**Frontend:**
- Подпись «Обновляется каждые 5 секунд» вместо «Обновлено 0 сек назад».
- При пустых полях провайдера (или `last_seen > 5min`) — серый текст с tooltip «Нет данных за последние 5 минут».

**Acceptance:**
- После cleanup моков и деплоя: `network:*:provider:*` ключи в Redis обновляются каждые 5 секунд, `last_seen` отстаёт от `now()` ≤ 5 секунд.
- Числа в UI Monitoring сходятся с БД: throughput за 5 минут ≈ `count(messages WHERE provider_id=X AND created_at > now()-5m) / 300`.
- После часа работы — `network_monitoring_snapshot` ≥ 50 строк/провайдер.
- При остановке collector'а ключи истекают по TTL за 60 секунд, UI показывает «нет данных».
- RBAC: данные scoped по `partner_id`.

**Жертва:** lag 5 секунд между событием и UI. Live-feel страдает, но это не sub-second-pulse, что приемлемо для оператора.

---

### Slice 4 — Контракт API + UX чистка (5–7 дней)

**Цель:** закрыть все остальные находки аудита.

**Backend — контракт:**
- **F4 валидация:** в `internal/gateway/portal/handlers/network_statistics.go` whitelist для `period_preset` (`today, yesterday, 7d, 30d, month, prev_month, year, custom`), `sort_by` (по списку колонок), `sort_dir` (`asc/desc`). Невалидное → 400 с понятным сообщением.
- **F5 case-insensitive channel:** нормализация `strings.ToLower(filter.Channel)` в handler'е перед SQL.
- **Voice в whitelist каналов:** добавить в domain-валидаторе (`internal/shared/channels.go` или эквивалент).
- **F7+F9:** в `/reseller/analytics-summary` KPI «Ошибки» поменять с целого числа на долю с `format=percent`. В `/reseller/monitoring` починить `dlr_rate` (сейчас хардкод 0.0% в monitoring_repo.go:116) — считать в collector'е.
- **`POST /reseller/views/{id}/clone`** — новый endpoint. Создаёт копию template'а с `is_template=false, user_id=current.id, name="<original> (копия)"`.

**Frontend — графики trends (F6):**
- Новый компонент `portal-frontend/src/components/network/TrendChart.tsx` на Recharts (уже в стеке).
- В `NetworkAnalyticsPage` под KPI-блоком и над таблицей — серия графиков по trend-metrics из API (`/reseller/analytics-summary` уже возвращает `trends.points`).
- Если `points.length < 2` — заглушка «Недостаточно данных».

**Frontend — статус-колонка (U3):**
- Безымянная колонка таблицы получает header «Статус».
- Tooltip с порогами: «🟢 норма (DLR ≥ 95%, нет таймаутов) / 🟡 внимание (DLR 80–95% или таймауты) / 🔴 критично (DLR < 80%)».
- `aria-label` на span: `Статус: критично`.

**Frontend — селектор операторов (U4):**
- Backend: SQL для списка `SELECT DISTINCT name FROM operators ORDER BY name`. Сейчас без DISTINCT — отсюда дубликаты.
- Frontend: заменить `<select>` на combobox с поиском (Radix UI).
- Алиасы (Билайн = Beeline) — backlog.

**Frontend — Saved views (U5):**
- У template'а скрываем `×`, показываем `📋` (clone).
- Клик по `📋` → POST `/reseller/views/{id}/clone` → перегружаем список → активируем клон.
- У user-views — `×` работает.

**Frontend — переезд блока «Виды» (U6):**
- Перенос `<SavedViewsBar>` из под-таблицы в шапку. Реализация — dropdown «Вид: Владелец ▾» рядом с табами.

**Frontend — chip-бар фильтров (U7):**
- Новый компонент `<ActiveFiltersBar>` под кнопкой «Применить» / над таблицей. Каждый chip с `×` для снятия фильтра.

**Frontend — экспорт (U8):**
- Кнопка → dropdown с опциями CSV / XLSX.
- Подпись «Экспорт текущей выборки (271 строка)» рядом.
- Async-flow через `StartExport` / `GetExportStatus` — частично есть, дополнить UI.

**Frontend — раскраска ячеек (U10):**
- Оставляем раскраску. Добавляем tooltip на каждую цветную ячейку: «Значение выше среднего за период (avg=N)» / «ниже среднего (avg=N)». Tooltip ссылается на расчёт в `service.go`.

**Frontend — мобильный (U11):**
- KPI-блок: горизонтальный скролл с overflow-snap.
- Таблица на ≤ 768px: card-view (одна карточка на строку).
- Bubble чата поддержки: убрать с мобильного или anchor к bottom без перекрытия.

**Frontend — autocomplete логина (U12):**
- Backend: `GET /reseller/sub-accounts?q=...` для typeahead.
- Frontend: combobox с поиском.

**Frontend — KPI Title Case (U16):**
- Убрать `text-transform: uppercase` в CSS KPI-карточки. Поправить spacing/font-weight.

**Acceptance:** см. список из 13 пунктов в брейншторме секции 5 (приведён ниже как приложение).

---

## 5. Backlog (явно вне скоупа)

- Импорт CSV для provider tariffs (вынесено из Slice 2).
- Production deploy hardening — blue-green swap для backfill (вынесено из Slice 1).
- Алертинг и push-нотификации на `health=warning` (вынесено из Slice 3).
- Алиасы операторов (Билайн = Beeline) для search в селекторе (вынесено из Slice 4 U4).
- Editable templates через админ-роль (вынесено из Saved views).
- `partner_id` в Prometheus metrics (Prom bridge не используется — выбран full Redis-collector).
- Money type precision: `domain.HourlyStatsRow.Revenue/Cost` and `domain.KPI.Value` are `float64`. The DB columns are `numeric(12,4)` (network_stats_hourly) / `numeric(20,6)` (tarification_log). For current transaction sizes (~thousands of RUB) this is safe; for very large windows or aggregator-of-aggregators scenarios, switch to `pgtype.Numeric` or `shopspring/decimal` to preserve exact decimal arithmetic. (Identified during Task 4 code review 2026-05-08.)
- KPI builder divergence: `service.go:buildStatKPIs` (used by `GetStatistics`/`GetAnalyticsSummary`) and `stats_repository.go:computeKPIs` (used by `GetDrillDown.Summary`) produce different KPI sets — `buildStatKPIs` returns 7 KPIs including Выручка/Себестоимость/Прибыль, `computeKPIs` returns 5 (Всего, Доставлено, Доставляемость, Ошибки, Прибыль) and lacks the money-input split. After Slice 1 added Выручка to `buildStatKPIs`, the DrillDown Summary still won't show it. Slice 2 will widen the gap further (cost/margin) unless these two builders are unified or `computeKPIs` is extended to mirror `buildStatKPIs`'s set. (Identified during final review 2026-05-09.)

---

## 6. Сводка и Acceptance Criteria

| Slice | Что | Размер | Merge gate |
|-------|-----|--------|------------|
| **0** | Diagnostic hotfix (F1+F2+F3) | 0.3 дня | UI больше не лжёт (rendering fix only) |
| **1** | Выручка | 3 дня | KPI «Выручка» = реальные числа |
| **2** | Себестоимость+Прибыль | 5–7 дней | Margin работает на тарифицированных провайдерах |
| **3** | Monitoring full live | 3–5 дней | Числа Monitoring = реальный трафик |
| **4** | Контракт + UX | 5–7 дней | Аудит закрыт |
| **Итого** | | **16.3–22.3 дней** | (был 17–23 до удаления D2) |

### Acceptance Slice 4 (полный список)

1. `period_preset=banana` → 400. `sort_by=zzz` → 400. `channel=SMS` ≡ `channel=sms`.
2. Канал `voice` доступен в селекторе.
3. На «Аналитика» под KPI отображаются графики trends.
4. KPI «Ошибки» во всех вкладках имеет одинаковый `format=percent`.
5. Selector операторов: дедупликация, поиск.
6. Saved views: `×` у user-views, `📋` clone у templates.
7. Блок «Виды» в шапке (dropdown).
8. Под «Применить» — chip-бар активных фильтров с `×`.
9. «Экспорт» → выбор формата + подпись скоупа.
10. Раскраска ячеек с tooltip-объяснением порогов.
11. Mobile 390px: KPI horizontal scroll, таблица card-view, без перекрытия чат-bubble.
12. «Логин» — combobox с поиском sub-accounts.
13. KPI-карточки в Title Case.

---

## 7. Риски и неопределённости

1. ~~D2 руутcause~~ — **закрыт**. Регресса нет (см. раздел 1 пункт 1(в)).
2. **`UpsertHourlyStats` смена семантики `add → replace`** может задеть других callers'ов агрегатора, которых я не нашёл grep'ом. Проверить ещё раз перед фиксом; если есть — добавить миграционный план.
3. **gRPC `GetActiveConns` в smpp-server** требует, чтобы там был session-tracker. Если его нет — Slice 3 раздувается на этот компонент. Уточнить при имплементации (первый шаг Slice 3 — чтение `cmd/smpp-server/`).
4. **Cardinality Redis при 100+ партнёрах** — на полном проде ключей будет много. На sandbox/пилоте незаметно. Алертинг на `redis_keyspace_size` отдельной задачей.
5. **Backfill (~30 минут на sandbox)** — пока идёт, `/network/statistics` показывает пустые числа. На прод будет нужен blue-green pattern (фиксировано в backlog).
6. **Forward-only cost** означает, что апрель-май навсегда без cost. Если бизнес захочет ретроактивный biллинг — отдельный спек, отдельное решение.

---

## 8. Open questions

Все вопросы брейншторма закрыты. Если в процессе имплементации обнаружатся новые развилки — фиксируем здесь и эскалируем.
