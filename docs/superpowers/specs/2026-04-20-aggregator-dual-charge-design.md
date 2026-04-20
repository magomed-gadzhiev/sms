# Aggregator Dual-Charge: Atomicity & Commit-on-Submit — Design

**Date:** 2026-04-20 (обновлено после проверки реального состояния кода)
**Status:** Draft (awaiting user review)
**Related prior work:**
- Спек `2026-04-15-aggregator-billing-quotas-design.md` — схема и CRUD квот (реализовано).
- Планы `2026-04-14-aggregator-phase*` — черновики Phase 1-4 (Phase 2 реализована частично, Phase 3/4 закрыты).
- Миграции `000096_aggregator_tariffs`, `000097_aggregator_quotas` — применены.

Этот спек **не создаёт агрегаторскую инфраструктуру с нуля** — она уже есть. Спек сужает фокус до четырёх оставшихся разрывов и фиксирует решения, принятые в брейншторме 2026-04-20.

## Контекст

За период 2026-04-14 … 2026-04-15 реализована основная инфраструктура агрегаторской модели:

- Таблицы `aggregator_tariffs`, `aggregator_margin_log`, `aggregator_quotas` (миграции 000096, 000097).
- Поля `billing_mode`, `spending_limit_monthly`, `spending_limit_daily` на `clients`; `attributed_sub_account_id` на `transactions` (миграция 000097).
- Repository-слой: `aggregator_margin_log_repository.go`, `aggregator_quota_repository.go`, `aggregator_tariff_repository.go`.
- Application-слой: `QuotaService` (ConsumeQuota, CRUD), интеграция в `tarification_service`.
- Admin API: `internal/gateway/admin/handlers/aggregator_quotas.go` (CRUD).
- Portal API для агрегатора: `internal/gateway/portal/handlers/aggregator_quotas.go` (read-only + billing_mode на субаккаунте).
- Frontend: TariffsPage, AnalyticsPage, SimulatorPage, админ-страницы агрегаторов.
- Phase 3 (routing) и Phase 4 (analytics) — завершены.

**Что именно не работает или сделано неправильно (и это фокус текущего спека):**

1. **Dual-charge не атомарен.** `saga.ChargeDual()` делает два отдельных RPC-вызова в billing-service (`ChargeMessage` для субаккаунта, затем для агрегатора) и компенсирует первый при падении второго. Между первым и вторым списанием существует окно несогласованности. При высоком темпе или временных ошибках возможны: двойной refund, пропущенный refund, зависшее списание субаккаунта без парного у агрегатора.
2. **`TarifyMessage` не read-only.** В текущей реализации он может вызывать `ConsumeQuota` (пишет в БД). По решению брейншторма (Q9=D1) списание должно происходить **после успешного SUBMIT к провайдеру**, не при tarify. Сейчас мы списываем за попытку, даже если SUBMIT не состоялся.
3. **`aggregator_margin_log` не различает pool/overage/split.** Для аналитики маржи этого недостаточно: агрегатор не видит, сколько из его трафика прошло по пакетной цене, сколько по overage. Данные об этом есть на уровне квоты (`segments_used`), но не по конкретному сообщению.
4. **Message ID использует UUID v4.** Для партиционирования `aggregator_margin_log` в будущем (когда объём потребует) нужна sortable форма — UUID v7.

## Решения, зафиксированные в брейншторме 2026-04-20

| # | Вопрос | Решение |
|---|--------|---------|
| 1 | Атомарность dual-списания | Одна PG-транзакция внутри billing-service (новый метод/RPC, не два отдельных вызова) |
| 2 | Источник цены агрегатора | Обычный `price_rule` агрегатора (аггрегатор как клиент) |
| 3 | Отрицательная маржа | Разрешена, логируется, метрика в Prometheus |
| 4 | Модель квоты | Пул сегментов на агрегатора (реализовано) |
| 5 | Overage-поведение | Soft-overage всегда — списание по `overage_rate` с баланса агрегатора (частично реализовано) |
| 6 | Период сброса | Календарный месяц UTC (в существующей схеме — через `period_start`/`period_end`, согласовано) |
| 7 | Multi-part SMS | Списание по сегментам (реализовано) |
| 8 | Порядок в hot-path | Atomic `UPDATE ... RETURNING` на квоте внутри одной транзакции |
| 9 | Refund и момент списания | Списание **после успешного SUBMIT к провайдеру**; pre-submit failure = не списано; post-submit delivery fail = не возвращается |
| 10 | Initial seed квоты | `auto_renew=true` (уже в схеме) + lazy-create при первой отправке в новом периоде: наследует `segment_limit`/`overage_rate` из предыдущего периода; `auto_renew=false` = hard-stop с `QUOTA_NOT_CONFIGURED`. Admin-RPC остаётся для setup/override. **Изменено относительно исходной версии Q10=A в пользу совместимости с применённой схемой.** |

## Scope

**В scope (четыре оставшихся разрыва):**

1. Атомизация dual-charge: один SQL-круг внутри billing-service, охватывающий обе записи в `transactions`, `UPDATE balance` на субаккаунт и агрегатор, `UPDATE aggregator_quotas` (инкремент), `INSERT aggregator_margin_log` как идемпотентный guard.
2. Разделение `TarifyMessage` (read-only расчёт) и `CommitCharge` (атомарное списание), вызов последнего из pipeline после успешного SUBMIT_SM_RESP. `ConsumeQuota` переносится из tarify в commit.
3. Расширение `aggregator_margin_log`: миграция 000106 добавляет `charge_mode VARCHAR(16)`, `pool_segments INT`, `overage_segments INT`. Существующие записи получают `charge_mode='pool'` (дефолт — для исторических данных это приемлемая аппроксимация).
4. Lazy-create квоты через `auto_renew`: если на текущий период нет row'а и у последнего row предыдущего периода `auto_renew=true` — в первой транзакции `CommitCharge` текущего периода создать новый row с параметрами предыдущего. Если `auto_renew=false` или предыдущего периода нет — `QUOTA_NOT_CONFIGURED`.

**Out of scope:**

- Переделка схемы `aggregator_quotas` с `period_start`/`period_end` на `period_month` (несовместимо с существующим кодом; диапазон более гибкий, код уже работает с ним).
- Уведомления агрегатора при 80%/100% пула (поля `notified_80pct`, `notified_100pct` уже в схеме, фича закладывалась, реализуется отдельно).
- Bulk-операции админа (копирование квот на всех агрегаторов).
- Sharding `aggregator_quotas` для высокого contention.
- Изменения UI (текущие страницы функциональны).
- Переименование reseller → aggregator в API/URL.
- Изменения `is_reseller` флага.

## Архитектура

### Миграция 000106 (ALTER aggregator_margin_log)

Добавляет три поля:
- `charge_mode VARCHAR(16) NOT NULL DEFAULT 'pool'` — `pool` | `overage` | `split`.
- `pool_segments INT NOT NULL DEFAULT 0` — сколько из `segment_count` прошло по pool-цене.
- `overage_segments INT NOT NULL DEFAULT 0` — сколько по overage-цене.

`CHECK (pool_segments + overage_segments = segment_count)` — инвариант, который кодекс обеспечивает при вставке.

Down-миграция: `ALTER TABLE aggregator_margin_log DROP COLUMN charge_mode, DROP COLUMN pool_segments, DROP COLUMN overage_segments;`.

### Атомарный `ChargeMessageDual` в billing-service

**Proto:** добавить в `api/proto/billing/billing.proto` новый RPC:

```
rpc ChargeMessageDual(ChargeMessageDualRequest) returns (ChargeMessageDualResponse);
```

Вход включает: `message_id` (UUID v7 как string, используется как `idempotency_key`), `subaccount_id`, `aggregator_id`, `subaccount_price`, `aggregator_price`, `segments`, `pool_segments`, `overage_segments`, `operator_id`, `charge_mode`.

Выход: `committed bool`, `subaccount_tx_id`, `aggregator_tx_id`, `margin_log_id`, `error enum` (`ALREADY_COMMITTED`, `INSUFFICIENT_BALANCE_SUBACCOUNT`, `INSUFFICIENT_BALANCE_AGGREGATOR`, `QUOTA_NOT_CONFIGURED`).

**Реализация в billing-service.** Один handler, одна PG-транзакция `READ COMMITTED`, шаги:

1. `INSERT INTO aggregator_margin_log (..., idempotency_key = message_id) ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`. Если 0 rows → `ROLLBACK`, `ALREADY_COMMITTED`. Быстрый выход до трогания балансов.
2. Убедиться, что row `aggregator_quotas` существует для текущего периода — либо найти через `GetActive(aggregator_id)`, либо lazy-create (см. ниже).
3. `UPDATE aggregator_quotas SET segments_used = segments_used + $1 WHERE id = $2 RETURNING segments_used, segment_limit`. Если 0 rows → `ROLLBACK`, `QUOTA_NOT_CONFIGURED`.
4. `UPDATE accounts SET balance = balance - $subaccount_price WHERE client_id = $subaccount_id AND balance >= $subaccount_price RETURNING balance`. Если 0 rows → `ROLLBACK`, `INSUFFICIENT_BALANCE_SUBACCOUNT`.
5. `UPDATE accounts SET balance = balance - $aggregator_price WHERE client_id = $aggregator_id AND balance >= $aggregator_price RETURNING balance`. Если 0 rows → `ROLLBACK`, `INSUFFICIENT_BALANCE_AGGREGATOR`.
6. `INSERT INTO transactions` — две записи (subaccount -subaccount_price, aggregator -aggregator_price), обе со `reference = message_id`, у aggregator-записи `attributed_sub_account_id = subaccount_id` (колонка уже существует в схеме).
7. `COMMIT`.

Порядок: margin_log первым — идемпотентность-guard до UPDATE балансов. UPDATE `aggregator_quotas` перед UPDATE balances — контеншн-точка сериализуется первой, балансы попадают в уже сериализованный поток.

**Старый `saga.ChargeDual()` удаляется.** Вызывающий код (tarification commit-path) переводится на `billing.ChargeMessageDual`.

### Lazy-create квоты через `auto_renew`

Внутри `ChargeMessageDual`, перед инкрементом квоты:

1. `SELECT id, period_end, segment_limit, overage_rate, auto_renew FROM aggregator_quotas WHERE aggregator_id = $1 AND period_start <= CURRENT_DATE AND period_end >= CURRENT_DATE LIMIT 1` — ищем активный row.
2. Если row есть → используем, идём дальше.
3. Если row нет → ищем последний row агрегатора: `SELECT ... ORDER BY period_end DESC LIMIT 1`. Если предыдущий row есть и `auto_renew=true`: `INSERT INTO aggregator_quotas (aggregator_id, period_start, period_end, segment_limit, overage_rate, auto_renew, currency) VALUES (...)` — период `[date_trunc('month', NOW() UTC), date_trunc('month', NOW() UTC) + interval '1 month' - interval '1 day']`, остальные поля из предыдущего row. Row становится новым активным.
4. Если предыдущего row нет или `auto_renew=false` → `QUOTA_NOT_CONFIGURED`.

Lazy-create встроен в ту же транзакцию, что и dual-charge. Race condition на INSERT решается `ON CONFLICT (aggregator_id, period_start) DO NOTHING` с последующим `SELECT` (другой concurrent call мог вставить первым).

### `TarifyMessage` → read-only

Изменения в `internal/services/tarification/application/tarification_service.go`:

- Удалить вызов `ConsumeQuota` из tarify-флоу (если он там есть).
- Tarify выполняет: сегментацию, каскадный lookup тарифа субаккаунта (`aggregator_tariff_repository`), lookup тарифа агрегатора (`price_rule`), SELECT квоты (без инкремента), расчёт `charge_mode`/`pool_segments`/`overage_segments`, SELECT балансов для валидации.
- В ответе `TarifyMessageResponse` добавляются поля: `message_id` (UUID v7, сгенерирован здесь), `subaccount_price`, `aggregator_price`, `charge_mode`, `pool_segments`, `overage_segments`.
- БД: только SELECT, никаких UPDATE/INSERT.

### `CommitCharge` — новый RPC в tarification-service

Proto: добавить в `api/proto/tarification/tarification.proto` новый RPC `CommitCharge`.

Вход: `message_id` (только).
Выход: `committed bool`, `error enum` (`ALREADY_COMMITTED`, `INSUFFICIENT_BALANCE_SUBACCOUNT`, `INSUFFICIENT_BALANCE_AGGREGATOR`, `QUOTA_NOT_CONFIGURED`, `NO_TARIFF`).

Реализация:
1. Повторный расчёт (переиспользование кода из TarifyMessage — вынести в отдельный метод `calculate()` в `tarification_service`). Причина повторного расчёта — защита от drift тарифа/квоты между tarify и commit (обычно миллисекунды, но при долгих SUBMIT к провайдеру — секунды).
2. Вызов `billing.ChargeMessageDual` с параметрами из расчёта.
3. Возврат результата вызывающему.

### Pipeline integration (SMPP sender)

В `internal/pipeline/sender/stage.go` (или его аналоге — точное место уточнить при плане):
- На начале обработки сообщения вызывается `TarifyMessage` — получаем `message_id`, валидируем балансы. Если валидация не прошла — сообщение помечается `failed`, CommitCharge не вызывается.
- Отправляем SUBMIT_SM к провайдеру.
- Если `SUBMIT_SM_RESP = OK` (или эквивалент для не-SMPP каналов) → `CommitCharge(message_id)`. При `ALREADY_COMMITTED` продолжаем как обычно (retry-безопасно). При любой другой ошибке — логируем, сообщение помечается `charge_failed` (уже ушло провайдеру, но у нас финансы не зафиксированы — это окно решается повтором `CommitCharge`).
- Если SUBMIT упал (pre-submit failure) → `CommitCharge` не вызывается. Деньги на месте.

Retry commit: существующий механизм retry в pipeline (если есть) или новый — отдельный lightweight worker, который ищет сообщения со статусом `charge_failed` и повторяет `CommitCharge` по их `message_id`.

### UUID v7 для message_id

В `saga.go` (или в `tarification_service.calculate()`) замена `uuid.New()` на UUID v7 generator. Использовать существующий пакет, если есть (проверить `go.mod` на `github.com/google/uuid` версию с v7 поддержкой, или добавить `github.com/gofrs/uuid/v5`).

### Метрики

Расширить существующие метрики (не писать с нуля) следующими:
- `commit_charge_duration_seconds` — histogram, время от начала `CommitCharge` до commit в БД.
- `commit_charge_result_total{result}` — counter: `committed`, `already_committed`, `insufficient_balance_subaccount`, `insufficient_balance_aggregator`, `quota_not_configured`, `no_tariff`.
- `aggregator_margin_by_mode_total{aggregator_id, charge_mode}` — counter, суммарная маржа по режимам.

Если в проекте уже есть метрика `aggregator_margin_total` — добавить лейбл `charge_mode` или оставить неразмеченной и создать новую. Уточнить при реализации.

## Тестирование

Фокус — на новой логике, не на существующей.

**Unit (billing-service):**
- `ChargeMessageDual` happy path с разными `charge_mode` (pool, overage, split).
- `ALREADY_COMMITTED` при повторе с тем же `message_id` (баланс не меняется).
- `INSUFFICIENT_BALANCE_SUBACCOUNT` — сабакаунт пустой, ROLLBACK полный.
- `INSUFFICIENT_BALANCE_AGGREGATOR` — агрегатор пустой, субаккаунт не списан.
- Lazy-create: `auto_renew=true` → новый row создан; `auto_renew=false` → `QUOTA_NOT_CONFIGURED`.
- Конкурентные вызовы `ChargeMessageDual` с разными `message_id` на одного агрегатора — квота инкрементится корректно (нет lost updates).
- Конкурентный двойной вызов с одинаковым `message_id` — только один commit (второй `ALREADY_COMMITTED`).

**Integration (реальный Postgres):**
- Полный flow `TarifyMessage` → (моковый SUBMIT) → `CommitCharge` → проверка балансов, transactions, margin_log, quota.
- Pre-submit failure flow: `TarifyMessage` ok, SUBMIT отменён → balance не изменён, margin_log пуст.
- Retry flow: `CommitCharge` вызван дважды с одним `message_id` → результат идентичен одному вызову.

**E2E (Playwright) `e2e/tests/reseller/dual-charge.spec.ts`** (новый тест):
- Создать агрегатора, субаккаунт, настроить тарифы и квоту (через admin API).
- Отправить SMS от субаккаунта.
- Проверить: баланс субаккаунта уменьшился на `subaccount_price`, баланс агрегатора на `aggregator_price`, в `aggregator_margin_log` есть row с корректным `charge_mode`.

## План выкатки

1. **Миграция 000106** применяется — добавляет `charge_mode`, `pool_segments`, `overage_segments` в `aggregator_margin_log`. Старый код, записывающий в эту таблицу без новых полей, падает на NOT NULL → **миграция должна применяться синхронно с деплоем нового saga/billing-кода**, поэтому дефолт `'pool'` критичен.
2. **Deploy billing-service с `ChargeMessageDual`**. Старый `ChargeMessage` остаётся — используется direct-клиентами и (временно) старым saga.
3. **Deploy tarification-service с read-only `TarifyMessage` и новым `CommitCharge`**. Фиче-флаг `COMMIT_ON_SUBMIT_ENABLED` (default off): при off — старая логика (списание при tarify через `saga.ChargeDual`), при on — новая (tarify read-only + CommitCharge).
4. **Deploy pipeline/sender с разветвлением**: если флаг on и у сообщения есть `parent_client_id` у клиента — новый flow; иначе старый.
5. Включить флаг для одного тестового агрегатора. Верификация: маржа и балансы сходятся, `charge_mode` в лог записывается корректно.
6. Включить флаг для всех агрегаторов.
7. После стабилизации — удалить старый `saga.ChargeDual` и legacy-путь.

**Rollback:** выключение флага возвращает старую (неатомарную, но работающую) логику. Миграцию 000106 не откатываем — новые поля при старой логике заполняются дефолтами.

## Риски и компромиссы

- **Drift между tarify и commit.** Решение — повторный расчёт в commit. Разница обычно копеечная. Клиент в tarify-ответе получает оценку, в commit — актуальное списание. Если критично для UI — возвращать обе цены в commit-ответе.
- **Контеншн на `aggregator_quotas` row.** Сериализует все commit'ы одного агрегатора. ~100 msg/sec per aggregator. Решается sharding'ом row по hash(message_id) — вне scope.
- **Окно между SUBMIT OK и CommitCharge.** Секунды. Существующий retry-механизм или новый lightweight worker покрывает. Метрика `commit_charge_retry_total` отслеживает аномалии.
- **Отрицательная маржа.** По решению разрешена. Метрика + алерт — отдельная задача, не блокирует основной feature.
- **Lazy-create гонка.** Два concurrent первых commit'а в новом периоде → оба пытаются INSERT. `ON CONFLICT (aggregator_id, period_start) DO NOTHING` + последующий SELECT решает.

## Открытые вопросы

- Где именно в pipeline (точный файл и функция) вставляется вызов `CommitCharge` после SUBMIT_SM_RESP — уточнить при написании плана.
- Есть ли уже retry-механизм для failed-commit'ов или создавать новый worker — уточнить при написании плана.
- Версия `github.com/google/uuid` в go.mod — проверить UUID v7 support, или добавить альтернативный пакет.

## Ссылки

- Решения от 2026-04-20: `memory/project_aggregator_decisions.md`.
- Предыдущий спек (реализован): `docs/superpowers/specs/2026-04-15-aggregator-billing-quotas-design.md`.
- Планы Phase 1-4: `docs/superpowers/plans/2026-04-14-aggregator-phase*.md`.
- Существующие репозитории: [aggregator_margin_log_repository.go](../../../internal/services/tarification/infrastructure/repository/aggregator_margin_log_repository.go), [aggregator_quota_repository.go](../../../internal/services/tarification/infrastructure/repository/aggregator_quota_repository.go).
- Существующий `saga.ChargeDual`: [saga.go](../../../internal/services/tarification/application/saga.go) (подлежит удалению).
