# Aggregator Dual-Charge & Quota Regulator — Design

**Date:** 2026-04-20
**Status:** Draft (awaiting user review)
**Supersedes in part:** `docs/superpowers/plans/2026-04-14-aggregator-phase2-tarification.md` (та же цель, уточнённая архитектура)

## Контекст и мотивация

В SMS-проекте существует двухуровневая клиентская модель: **агрегатор** (`clients.is_reseller = true`, `parent_client_id IS NULL`) и его **субаккаунты** (`parent_client_id = aggregator.id`). Агрегатор настраивает субаккаунтам свои тарифы (`aggregator_tariffs`, миграция 000097), при этом сам имеет контракт с платформой со своими ценами (`price_rule` как у обычного direct-клиента).

Текущая реализация `TarifyMessage` делает каскадный lookup тарифа субаккаунта и **одно** списание — с субаккаунта. При этом агрегатор не платит платформе ничего, и маржа (разница между ценой субаккаунта и ценой агрегатора) нигде не фиксируется. Это искажает биллинг: либо субаккаунт платит платформе цену агрегатора (и агрегатор не получает наценку), либо субаккаунт платит цену агрегатора (и платформа не получает свои деньги). Экономическая модель не согласована с кодом.

Вторая проблема — `AggregatorQuota` как доменная модель существует, но не регулирует ничего: таблицы `aggregator_quotas` нет, в hot-path не проверяется.

Цель спека — завершить Phase 2: ввести атомарное dual-списание, регулятор пула квоты, корректный refund-contract. Сделать это без трогания `is_reseller` (решение 2026-04-20: дискриминатор остаётся булевым).

## Решения, принятые в брейншторме

| # | Вопрос | Решение |
|---|--------|---------|
| 1 | Атомарность dual-списания | Одна PG-транзакция, две записи в `transactions` |
| 2 | Источник цены агрегатора | Обычный `price_rule` агрегатора (аггрегатор как клиент) |
| 3 | Отрицательная маржа | Разрешена, логируется, метрика в Prometheus |
| 4 | Модель квоты | Пул сегментов на агрегатора (не бюджет, не только баланс) |
| 5 | Overage-поведение | Soft-overage всегда — списание по `overage_rate` с баланса агрегатора |
| 6 | Период сброса | Календарный месяц UTC |
| 7 | Multi-part SMS | Списание по сегментам (не по сообщениям) |
| 8 | Порядок в hot-path | Atomic `UPDATE ... RETURNING` на квоте, без отдельного `SELECT FOR UPDATE` |
| 9 | Refund | Списание при успешном SUBMIT к провайдеру (не при tarify); pre-submit failure = не списано; post-submit delivery fail = не возвращается |
| 10 | Initial seed квоты | Admin-RPC явный; нет row — `QUOTA_NOT_CONFIGURED` |

## Scope

**В scope:**
- Миграция `000098_aggregator_dual_charge` — две новые таблицы.
- Новый RPC `CommitCharge` в tarification-service; существующий `TarifyMessage` переводится в read-only (dry-run).
- Новый внутренний RPC `ChargeDual` в billing-service.
- Admin-RPC для CRUD квот.
- Read-only endpoint для агрегатора "моя квота".
- Prometheus-метрики, Grafana-дашборд, алерты.
- Cron-мониторинг агрегаторов без настроенной квоты на текущий месяц.
- Unit/integration/E2E тесты.

**Out of scope:**
- Изменения схемы клиентов (`is_reseller` → `account_type`) — решение закрыто.
- Bulk-операции админа (копирование квот на следующий месяц) — отдельный мини-спек.
- Sharding `aggregator_quotas` для случая >100 msg/sec per aggregator.
- Ретроспективный refund по delivery outcome.
- UI-редизайн `/network/quota`.
- Переименование reseller → aggregator в API/UI (тех-долг).

## Архитектура

### Изменения в модели данных (миграция 000098)

**Таблица `aggregator_margin_log`** — один row на одно commit-списание, помесячное партиционирование по `created_at` (аналогично существующим `messages` и `tarification_log`).

Поля:
- `id UUID PK`
- `message_id UUID NOT NULL` — **UNIQUE** (идемпотентность retry)
- `aggregator_id UUID NOT NULL REFERENCES clients(id)`
- `sub_account_id UUID NOT NULL REFERENCES clients(id)`
- `operator_id UUID NOT NULL REFERENCES operators(id)`
- `segments INTEGER NOT NULL`
- `subaccount_price NUMERIC(12,6) NOT NULL`
- `aggregator_price NUMERIC(12,6) NOT NULL`
- `margin NUMERIC(12,6) NOT NULL` (может быть < 0)
- `charge_mode VARCHAR(16) NOT NULL` — `'pool'` | `'overage'` | `'split'`
- `pool_segments INTEGER NOT NULL DEFAULT 0` — сколько из N сегментов прошло по pool-цене
- `overage_segments INTEGER NOT NULL DEFAULT 0` — сколько по overage-цене
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Индексы:
- `UNIQUE(message_id)` — идемпотентность
- `(aggregator_id, created_at DESC)` — для отчётов по агрегатору
- `(sub_account_id, created_at DESC)` — для отчётов по субаккаунту

**Таблица `aggregator_quotas`** — один row на (aggregator_id, period_month), без партиционирования (таблица маленькая: N_агрегаторов × 12 месяцев).

Поля:
- `aggregator_id UUID NOT NULL REFERENCES clients(id)`
- `period_month DATE NOT NULL` — первое число месяца UTC (`date_trunc('month', NOW() AT TIME ZONE 'UTC')::date`)
- `segment_limit BIGINT NOT NULL CHECK (segment_limit >= 0)`
- `segments_used BIGINT NOT NULL DEFAULT 0`
- `overage_segments BIGINT NOT NULL DEFAULT 0`
- `overage_rate NUMERIC(12,6) NOT NULL CHECK (overage_rate >= 0)`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `PRIMARY KEY (aggregator_id, period_month)`

Индексы: PK покрывает типичные запросы; `(period_month)` для джоба мониторинга.

**Что НЕ меняется:**
- `clients`, `accounts`, `transactions`, `price_rule`, `aggregator_tariffs` — остаются.
- `AggregatorTariff` (000097) уже описывает тариф субаккаунта у агрегатора — используем как есть.

### RPC-контракты

**`tarification.TarifyMessage`** — переводится в read-only.

Вход: как сейчас (`subaccount_id`, `operator_id`, `text`, `sender_category`, ...).
Выход:
- `message_id` (новый UUID v7 — sortable, чтобы партиции margin_log росли монотонно)
- `segments`
- `subaccount_price`, `aggregator_price` (оба NUMERIC as string)
- `charge_mode` enum: `POOL` | `OVERAGE` | `SPLIT`
- `sufficient_balance bool`
- `error` enum: `QUOTA_NOT_CONFIGURED` | `INSUFFICIENT_BALANCE_SUBACCOUNT` | `INSUFFICIENT_BALANCE_AGGREGATOR` | `NO_TARIFF_SUBACCOUNT` | `NO_TARIFF_AGGREGATOR`

Никаких UPDATE/INSERT. Только SELECT.

**`tarification.CommitCharge`** — новый RPC, вызывается SMPP-клиентом после `SUBMIT_SM_RESP = OK`.

Вход:
- `message_id` (тот же UUID, что вернул `TarifyMessage`)

Tarification-service **не доверяет** параметрам из tarify — расчёт повторяется непосредственно перед вызовом `billing.ChargeDual`. Это защита от drift, если админ поменял тариф между tarify и commit. Строго атомарной согласованности "расчёт+списание" не достичь при межсервисном вызове, но окно drift сокращается до миллисекунд между повторным SELECT и транзакцией billing.

Выход:
- `committed bool`
- `subaccount_tx_id`, `aggregator_tx_id`, `margin_log_id` (UUID строки)
- `error` enum: `ALREADY_COMMITTED` | `INSUFFICIENT_BALANCE_SUBACCOUNT` | `INSUFFICIENT_BALANCE_AGGREGATOR` | `QUOTA_NOT_CONFIGURED` | `NO_TARIFF_*`

**`billing.ChargeDual`** — новый внутренний RPC, вызывается только tarification-service в рамках `CommitCharge`. Не экспонируется наружу.

Вход:
- `message_id` (idempotency key)
- `subaccount_id`, `aggregator_id`
- `subaccount_price`, `aggregator_price`
- `segments`, `pool_segments`, `overage_segments`
- `operator_id`
- `charge_mode`

Выход:
- `committed bool`
- `subaccount_tx_id`, `aggregator_tx_id`, `margin_log_id`
- `error` enum: `ALREADY_COMMITTED` | `INSUFFICIENT_BALANCE_SUBACCOUNT` | `INSUFFICIENT_BALANCE_AGGREGATOR` | `QUOTA_NOT_CONFIGURED`

Атомарность — внутри `billing.ChargeDual` (одна PG-транзакция).

**`billing.Charge`** (single-charge для direct-клиентов) — остаётся, не трогаем.

### Hot-path: TarifyMessage (read-only)

1. Загрузить клиента. Если `parent_client_id IS NULL` — это direct-клиент, идём в legacy-путь single-charge (вне этого спека). Дальше — только субаккаунты.
2. `aggregator_id = client.parent_client_id`.
3. Сегментация: N сегментов.
4. Цена субаккаунта — каскадный lookup `aggregator_tariffs`:
   - `(aggregator_id, subaccount_id, operator_id, sender_category)` — специфичный
   - `(aggregator_id, NULL, operator_id, sender_category)` — дефолт для всех субаккаунтов агрегатора
   - Иначе — `NO_TARIFF_SUBACCOUNT`.
   `subaccount_price = tariff_price * N`.
5. Цена агрегатора — обычный `price_rule(aggregator_id, operator_id, sender_category)`. Если нет — `NO_TARIFF_AGGREGATOR`. Это **pool-цена** агрегатора.
6. Загрузить квоту: `SELECT segment_limit, segments_used, overage_rate FROM aggregator_quotas WHERE aggregator_id = $1 AND period_month = date_trunc('month', NOW() AT TIME ZONE 'UTC')::date`. Если нет — `QUOTA_NOT_CONFIGURED`.
7. Определить `charge_mode` и `aggregator_price`:
   - `segments_used + N ≤ segment_limit` → `POOL`, `aggregator_price = pool_rate * N`
   - `segments_used ≥ segment_limit` → `OVERAGE`, `aggregator_price = overage_rate * N`
   - Иначе (частично пересечение границы) → `SPLIT`: `pool_part = segment_limit - segments_used`, `overage_part = N - pool_part`, `aggregator_price = pool_rate * pool_part + overage_rate * overage_part`.
8. Проверить балансы (SELECT без локов): `subaccount.balance >= subaccount_price`, `aggregator.balance >= aggregator_price`.
9. Сгенерировать `message_id = uuid_v7()`.
10. Вернуть расчёт вызывающему.

БД: только SELECT. Время ответа — несколько ms.

### Hot-path: CommitCharge (atomic)

Вызывается SMPP-клиентом после `SUBMIT_SM_RESP = OK` с `message_id` из tarify-ответа.

1. Tarification-service повторно выполняет шаги 1–7 TarifyMessage (расчёт — несколько SELECT'ов на тарифы и квоту).
2. Tarification-service вызывает `billing.ChargeDual` с готовыми параметрами. **Всё дальнейшее — внутри одной PG-транзакции в billing-service.**
3. Начало транзакции `READ COMMITTED`.
4. **Первой операцией** — `INSERT INTO aggregator_margin_log (...) VALUES (...) ON CONFLICT (message_id) DO NOTHING RETURNING id`. Если вернуло 0 rows — retry уже закоммиченного сообщения → `ROLLBACK`, return `ALREADY_COMMITTED`. Быстрый выход, никакие балансы не тронуты.
5. `UPDATE aggregator_quotas SET segments_used = segments_used + $N, overage_segments = overage_segments + $N_overage, updated_at = NOW() WHERE aggregator_id = $1 AND period_month = $2 RETURNING segments_used`. Если 0 rows (админ удалил квоту между tarify и commit) — `ROLLBACK`, `QUOTA_NOT_CONFIGURED`.
6. `UPDATE accounts SET balance = balance - $subaccount_price WHERE client_id = $subaccount_id AND balance >= $subaccount_price RETURNING balance`. Если 0 rows — `ROLLBACK`, `INSUFFICIENT_BALANCE_SUBACCOUNT`.
7. `UPDATE accounts SET balance = balance - $aggregator_price WHERE client_id = $aggregator_id AND balance >= $aggregator_price RETURNING balance`. Если 0 rows — `ROLLBACK`, `INSUFFICIENT_BALANCE_AGGREGATOR`.
8. `INSERT INTO transactions` — две записи (sub -subaccount_price, aggregator -aggregator_price), обе со `reference = message_id`.
9. `COMMIT`.

Порядок важен: `INSERT margin_log` первым — если идемпотентность-guard сработал, мы выходим до UPDATE баланса.

Почему `UPDATE ... WHERE balance >= amount`, а не `SELECT FOR UPDATE + UPDATE`: Postgres сам берёт row-lock при UPDATE, проверка `balance >= amount` в `WHERE` атомарна. Экономим один round-trip и удерживаем лок меньше времени.

Контеншн — на row `aggregator_quotas` для данного агрегатора. Все сообщения одного агрегатора сериализуются на этой строке. Типичное время транзакции — 5–15 мс, предел — ~100 msg/sec per aggregator. Если конкретный агрегатор превысит — решается sharding'ом счётчика (N партиций по hash(message_id)), но это вне scope.

### Pre-submit failure: просто не вызываем CommitCharge

SMPP-клиент получает ошибку на SUBMIT (провайдер отклонил, таймаут, разрыв соединения до ответа) → `CommitCharge` не вызывается. Деньги и квота не тронуты. Сообщение помечается `failed_presubmit` в pipeline.

Retry-сценарии:
- **Retry SUBMIT** (до получения окончательного ответа) — политика SMPP-клиента, не влияет на биллинг.
- **Окончательный SUBMIT-fail** — `CommitCharge` не вызван, всё.
- **SUBMIT OK, но `CommitCharge` упал/не дошёл** — SMPP-клиент повторяет `CommitCharge` с тем же `message_id`. UNIQUE-guard на `aggregator_margin_log.message_id` делает второй вызов no-op (`ALREADY_COMMITTED`). Никогда не списываем дважды.

Delivery outcome (delivered/failed после SUBMIT OK) **не влияет** на биллинг. Списание при успешном SUBMIT — финальное.

### Admin-RPC для квот

Новые RPC в admin-proto (путь уточним при реализации — либо расширение существующего admin.proto, либо новый `api/proto/admin/aggregator_admin.proto`):

- `SetAggregatorQuota(aggregator_id, period_month, segment_limit, overage_rate)` — UPSERT.
- `GetAggregatorQuota(aggregator_id, period_month)` — возвращает row + derived поля (`segments_remaining`, `usage_ratio`).
- `ListAggregatorQuotas(period_month?, aggregator_id?)` — список с фильтрами.
- `DeleteAggregatorQuota(aggregator_id, period_month)` — удаление (редкий кейс).

HTTP-endpoints на портале — под префиксом `/admin/aggregators/*/quotas`, роль `admin`. Обычный агрегатор не видит admin-endpoints.

**Read-only для агрегатора** — `GET /network/quota/current` и `GET /network/quota/history?months=6`. Возвращает свою квоту без возможности редактирования. Использует `auth.ClientID` из контекста.

### UI

- Admin-страница `/admin/aggregators/{id}/quotas` — таблица квот, кнопка "Установить на следующий месяц". Минимум для MVP: форма ввода `segment_limit` и `overage_rate`, submit на `SetAggregatorQuota`. Bulk-операции — отдельно.
- Агрегатор-дашборд `/network/` — на текущей странице добавить блок "Квота месяца" (used / limit + overage_used). Расширяет существующий `NetworkDashboardPage.tsx`.

### Метрики и алерты

Prometheus:
- `aggregator_margin_total{aggregator_id, charge_mode}` counter — сумма маржи (может быть отрицательной).
- `aggregator_negative_margin_messages_total{aggregator_id}` counter.
- `aggregator_quota_segments_used{aggregator_id}` gauge.
- `aggregator_quota_segment_limit{aggregator_id}` gauge.
- `aggregator_overage_segments_total{aggregator_id}` counter.
- `aggregators_without_quota_current_month` gauge (cron-джоб).
- `commit_charge_duration_seconds` histogram.
- `commit_charge_retry_total{error}` counter.

Grafana-дашборд `aggregator-billing`: маржа по агрегаторам (timeseries), top-5 с отрицательной маржой (table), использование пула (gauges), overage trend.

Алерты:
- `AggregatorNegativeMarginSustained` — `rate(aggregator_margin_total) < 0` 30 мин → warning.
- `AggregatorsWithoutQuotaExist` — `aggregators_without_quota_current_month > 0` 5 мин → critical.
- `CommitChargeP99LatencyHigh` — `histogram_quantile(0.99, commit_charge_duration_seconds) > 0.5` 10 мин → warning (контеншн на квоте).

Cron-джоб (раз в час): `SELECT COUNT(*) FROM clients c WHERE c.is_reseller AND NOT EXISTS (SELECT 1 FROM aggregator_quotas q WHERE q.aggregator_id = c.id AND q.period_month = date_trunc('month', NOW() AT TIME ZONE 'UTC')::date)`. Выставляет gauge.

## Тестирование

**Unit:**
- `billing.ChargeDual` — happy path, insufficient subaccount balance, insufficient aggregator balance, quota not configured, drift-сценарий (квота удалена между tarify и commit), SPLIT-граница, concurrent double-call (идемпотентность).
- `tarification.CommitCharge` — расчёт-drift (тариф изменился между tarify и commit — commit берёт актуальный), ALREADY_COMMITTED.
- Сегментация — если существующих тестов нет, добавить; если есть — переиспользовать.

**Integration (реальный Postgres через testcontainers):**
- Параллельные `CommitCharge` на одного агрегатора (10–50 goroutines) — проверка atomicity квоты: `segments_used` в конце == сумма N всех успешных.
- E2E-флоу: create aggregator → create subaccount → set quota → set tariffs (aggregator_tariffs + price_rule) → tarify → commit → assert balances, transactions, margin_log.
- Идемпотентность: вызов `CommitCharge` с одним `message_id` дважды — второй раз `ALREADY_COMMITTED`, balance не изменился.

**E2E (Playwright) — `e2e/tests/reseller/dual-charge.spec.ts`:**
- Один happy-path: агрегатор настроен, субаккаунт шлёт SMS, проверяем что оба баланса уменьшились на корректные суммы и в `aggregator_margin_log` есть row.

## План выкатки

1. Применить миграцию `000098_aggregator_dual_charge.up.sql`.
2. Deploy tarification-service с новыми `TarifyMessage` (dry-run) и `CommitCharge`. Фиче-флаг `DUAL_CHARGE_ENABLED` по умолчанию **off** — старый single-charge продолжает работать для субаккаунтов.
3. Deploy billing-service с `ChargeDual`.
4. Deploy SMPP-клиента с новым flow (`if parent_client_id != NULL && DUAL_CHARGE_ENABLED → CommitCharge; else → legacy Charge at tarify`).
5. Включить флаг для одного тестового агрегатора. Верификация 1–2 дня: `aggregator_margin_log` содержит корректные записи, балансы сходятся, квота инкрементится.
6. Включить для всех агрегаторов.
7. (После стабилизации) удалить legacy-флаг и код single-charge для субаккаунтов. Direct-клиенты остаются на single-charge как были.

**Rollback:** выключение `DUAL_CHARGE_ENABLED` возвращает single-charge. Данные в `aggregator_margin_log` остаются как history. Down-миграцию не катим в проде (таблицы не мешают).

## Риски и компромиссы

- **Контеншн на `aggregator_quotas`** — сериализует все commit'ы одного агрегатора. Предел ~100 msg/sec per aggregator. Sharding — вне scope, при необходимости в отдельной задаче.
- **Drift между tarify и commit** — защищены повторным расчётом в транзакции, но клиент увидит цену из tarify-ответа и спишут актуальную. Для UX: разница обычно копеечная (тариф меняется редко), при несовпадении — commit-ответ содержит фактические цены, и клиент может их логировать.
- **Забытая квота** — hard-stop для агрегатора. Смягчается алертом `AggregatorsWithoutQuotaExist`. В будущем возможен bulk-инструмент для админа.
- **Отрицательная маржа** — по решению разрешена. Метрика + алерт даёт админу видимость. Агрегатор сам принимает бизнес-решение.
- **Окно между SUBMIT OK и CommitCharge** — секунды. В это время сообщение уже ушло оператору, финансы не зафиксированы. Для отчётности "за минуту" — минорный drift. Обрабатывается retry'ем CommitCharge (идемпотентный).

## Зависимости

- Миграция 000097 (`aggregator_tariffs`) — должна быть применена. Если нет — сначала катим её.
- Существующий джоб создания партиций (если есть для `tarification_log` / `messages`) — переиспользовать для `aggregator_margin_log`. Если нет — добавить.
- Proto-generation pipeline — потребуется регенерация stubs после изменений в `tarification.proto` и `billing.proto`.

## Открытые вопросы (на момент реализации)

- Точное расположение admin-proto (существующий файл vs новый) — решить при старте плана.
- Необходимость `DeleteAggregatorQuota` в MVP — возможно можно отложить.
- Формат period_month в RPC (`"2026-05-01"` string vs `int32 year / int32 month`) — уточнить при написании proto.

## Ссылки

- Решения от 2026-04-20: `memory/project_aggregator_decisions.md`.
- Предыдущий план Phase 2: `docs/superpowers/plans/2026-04-14-aggregator-phase2-tarification.md` (superseded в части дизайна этим спеком).
- Существующая модель: [aggregator_tariff.go](../../../internal/services/tarification/domain/aggregator_tariff.go), [aggregator_resolver.go](../../../internal/services/tarification/infrastructure/aggregator_resolver.go).
