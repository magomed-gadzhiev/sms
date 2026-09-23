# Load Tests

Нагрузочные тесты для SMS Gateway Platform.

## Подготовка

### 1. Запуск инфраструктуры

```bash
cd deployments && docker compose up -d
```

### 2. Сидинг базы данных

Перед первым запуском выполните seed-скрипт для создания тестовых данных:

```bash
# Через docker
docker exec -i postgres psql -U smpp -d smpp_db < test/load/fixtures/seed.sql

# Или напрямую
psql -h localhost -U smpp -d smpp_db -f test/load/fixtures/seed.sql
```

Seed-скрипт создаёт:
- **7 SMSC-провайдеров** (MTS, Beeline, Megafon, Tele2, Tinkoff, International, Backup)
- **13 маршрутов** (по операторам + catch-all)
- **4 клиента** с разными rate-limit профилями (10K/1K/100 rps)
- **Партиции messages/audit_log** на 2025–2026 годы
- **Пользователей и API-ключи** для auth-сервиса
- **HLR-провайдеров** и **шаблоны SMS**

### 3. Очистка после тестов

```bash
psql -h localhost -U smpp -d smpp_db -f test/load/fixtures/cleanup.sql
```

## Демо-данные стенда

Наполнение локального стенда правдоподобными данными для тестирования и отладки
(дашборды, списки сообщений, кампании, контакты, биллинг, имена отправителя).

```bash
# Полный цикл: очистка -> базовый сид -> демо-данные -> flush Redis
make demo-seed  # делегирует в scripts/demo-seed.sh (пре/пост-условия)

# Удалить все демо-данные
make demo-clean
```

Что создаётся (поверх `demo_seed.sql`):

- **Имена отправителя**: Demo/OTP (approved, с регистрациями у 4 операторов), PROMO (pending), NEWS (rejected)
- **Списки контактов**: «OTP-база» (1000 контактов) и «Промо-осень» (500), с атрибутами, тегами и завершёнными импортами
- **Кампании**: completed / paused / scheduled / draft / cancelled, с получателями и снапшотами статистики
  (running-статус не используется: воркер кампаний подхватывает такие кампании и начинает реальную рассылку)
- **История сообщений**: ~75 000 сообщений за последние 30 дней (85% delivered),
  с DLR-квитками, привязкой к операторам/маршрутам/шаблонам
- **Тарификация**: легаси-тарифы операторов + `tarification_log` (цены в детализации портала)
- **Price rules**: platform catch-all, RU+MTS, RU+Beeline, tiered-правило реселлера
- **Транзакции**: ежедневные списания по данным тарификации + пополнения/возвраты,
  цепочка `balance_before/after` сходится к балансу счёта

Объём задаётся константой в `demo_data.sql` (секция «История сообщений», `generate_series`).
Таймстемпы относительны `now()` — повторный запуск только через `make demo-clean` + `make demo-seed`.

### Живая отправка на стенде

Отправка (одиночные сообщения и рассылки) работает через `SIMULATOR`-провайдер —
без реального SMPP: submit эмулируется, DLR публикуется в Kafka через 50 мс.
`demo_data.sql` (секция 0.5) настраивает всё автоматически:

- `Provider-Simulator` + `stub_provider_config` (100% DELIVRD);
- платформенный catch-all маршрут `Demo-Simulator-Fallback` в `client_routes`
  (клиенты по умолчанию в `legacy`-режиме → используют платформенный уровень);
- деактивация недостижимых демо-провайдеров (иначе sender стартует минутами —
  синхронно биндится ко всем active);
- префикс 7985 (МТС) и регистрации имён на легаси-операторах (MTS/BEELINE/…),
  на которых ссылается `operator_prefixes` — пайплайн резолвит оператора по ним;
- тарифы легаси-операторов для тарификации отправок.

**Требование:** должны работать pipeline-воркеры (в базовый `up -d` не входят):

```bash
cd deployments && docker compose -f docker-compose.yml -f docker-compose.local.yml \
  --env-file .env up -d pipeline-router pipeline-sender pipeline-status pipeline-persist
```

Проверка: отправьте сообщение из портала — статус дойдёт до `delivered` за секунды,
в `tarification_log` появится цена, в транзакциях — списание. Рассылка запускается
из портала (кнопка запуска у scheduled/draft-кампании) и проходит материалайзер →
Kafka → тот же конвейер. Демо-сообщения со статусом `scheduled` (750 шт.) дрейнят
реальной отправкой в течение суток после сида — живой трафик на дашбордах.

Известные ограничения живого пути (поведение кода, не стенда): `delivered_at`
не проставляется статус-стадией (только `status`/`updated_at`), `dlr_receipts`
live-путём не пишется; при burst-рассылках DLR-обновление может проиграть гонку
persist-стадии (задокументирована в коде пайплайна) — DLR-задержка 50 мс сводит
эффект к нулю.

### Учётные записи (портал `http://localhost:8083`)

Пароль для всех демо-пользователей: **`Admin123!`** (хэш в `demo_seed.sql` проверен;
старые комментарии «example local password» в фикстурах были неверными).

| Логин | Пароль | Роль |
|-------|--------|------|
| `demo-client` / `client@demo.local` | `Admin123!` | клиент Demo-Main |
| `demo-reseller` / `reseller@demo.local` | `Admin123!` | реселлер |
| `admin` (email — как у существующего пользователя, на этом стенде `admin@example.com`) | `Admin123!` | администратор |

⚠️ **`demo_seed.sql` перезаписывает пароль пользователя `admin`** (ON CONFLICT по username;
email при этом не меняется — остаётся тем, с которым admin был создан через `cmd/seed-admin`).
После `make demo-seed` вход `admin` работает с `Admin123!`, а не с паролем из
`deployments/.admin-credentials.local`. `make demo-clean` пользователя `admin` не удаляет.

## Фикстуры

### SQL (`test/load/fixtures/`)

| Файл | Описание |
|------|----------|
| `seed.sql` | Полный сидинг БД для load-тестов |
| `cleanup.sql` | Удаление только load-test данных |
| `demo_seed.sql` | База демо-стенда: клиенты, провайдеры, маршруты, пользователи, счета |
| `demo_data.sql` | Демо-данные поверх базы: история сообщений, кампании, контакты, биллинг (см. «Демо-данные стенда») |
| `demo_cleanup.sql` | Удаление всех демо-данных (base + demo_data) |

### Go (`test/load/fixtures.go`)

Генератор реалистичных данных для Go load-тестов:

- `RandomDestination()` — случайный номер из 30+ российских префиксов
- `RandomInternationalDestination()` — международный номер (BY, KZ)
- `RandomSource()` — source-адрес из пула (15 вариантов)
- `RandomText()` — текст SMS по шаблонам (OTP, транзакции, промо, уведомления)
- `NewRandomSendPayload()` — полный JSON-payload для `/api/v1/sms/send`
- `NewRandomBatchPayload(n)` — batch-payload с n сообщениями
- `LoadProfile` — предопределённые профили: `ProfileSmoke`, `ProfileNormal`, `ProfileHigh`, `ProfilePeak`

### JSON (`test/load/fixtures/`)

Для k6-тестов (загружаются через `SharedArray`):

| Файл | Описание |
|------|----------|
| `destinations.json` | 50+ номеров по 10 операторам + международные |
| `sources.json` | 15 source-адресов (alphanumeric, numeric, MSISDN) |
| `messages.json` | 20+ шаблонов по 5 категориям (OTP, транзакции, промо, уведомления, алерты) |

## Запуск Go-тестов

```bash
# Все load-тесты
go test -tags=load -v ./test/load/...

# Конкретный тест
go test -tags=load -v ./test/load/... -run TestAPIGateway_Load

# Стресс-тест
go test -tags=load -v ./test/load/... -run TestAPIGateway_Stress

# High performance (10K msg/s)
go test -tags=load -v -timeout 30m ./test/load/... -run TestHighPerformanceLoad

# Бенчмарки
go test -tags=load -bench=. ./test/load/...
```

### Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `LOAD_TEST_BASE_URL` | `http://localhost:8080` | URL API Gateway (через HAProxy) |
| `LOAD_TEST_API_KEY` | `test-api-key` | API-ключ для аутентификации |

```bash
# Пример: тестирование через HAProxy
LOAD_TEST_BASE_URL=http://localhost:8080 go test -tags=load -v ./test/load/...

# Пример: напрямую к gateway-инстансу
LOAD_TEST_BASE_URL=http://localhost:18080 go test -tags=load -v ./test/load/...
```

## Запуск k6-тестов

```bash
# Общий load-тест (mixed endpoints)
k6 run scripts/k6_load_test.js

# 10K msg/s тест
k6 run scripts/k6_10k_load_test.js

# С кастомными параметрами
k6 run -e BASE_URL=http://localhost:8080 -e API_KEY=lt-high-volume-key-001 scripts/k6_10k_load_test.js

# Через Docker
docker run --rm -i --network host \
  -v $(pwd):/app -w /app \
  grafana/k6 run scripts/k6_load_test.js
```

## Типы тестов

### Go

| Тест | Нагрузка | Длительность | Цель |
|------|----------|--------------|------|
| `TestAPIGateway_Load` | 10 × 100 = 1K запросов | ~30с | Базовая проверка |
| `TestAPIGateway_Stress` | 5→20→50→100 воркеров | ~90с | Поведение при росте нагрузки |
| `TestAPIGateway_SendBatchSMS_Load` | 5 × 20 batch | ~60с | Batch endpoint |
| `TestAPIGateway_GetStatus_Load` | 20 × 50 = 1K статусов | ~30с | Read-нагрузка |
| `TestAPIGateway_MixedLoad` | 10 × 30 × 3 эндпоинта | ~60с | Реалистичный микс |
| `TestHighPerformanceLoad` | 200 воркеров × 50 rps | ~3 мин | 10K msg/s target |
| `TestSustainedLoad` | 200 воркеров | 10 мин | Стабильность при 10K |

### k6

| Скрипт | VUs | Длительность | Цель |
|--------|-----|--------------|------|
| `k6_load_test.js` | 500→5000 | ~17 мин | Mixed endpoints, общая нагрузка |
| `k6_10k_load_test.js` | 100→2500 | ~23 мин | 10K msg/s target, SendSMS only |

## API-ключи для тестирования

| API Key | Профиль | Rate Limit |
|---------|---------|------------|
| `lt-high-volume-key-001` | High Volume | 10K/sec |
| `lt-med-volume-key-002` | Medium Volume | 1K/sec |
| `lt-low-volume-key-003` | Low Volume | 100/sec |
| `test-api-key` | Default (совместимость) | 10K/sec |

## Метрики

Тесты измеряют:
- **Throughput** (req/s) — целевой показатель для high-perf: >= 10K msg/s
- **Latency** (p50, p95, p99) — целевые: p95 < 500ms, p99 < 1000ms
- **Success rate** — целевой: >= 95% (stress), >= 99% (10K)
- **Error rate** — по категориям (timeout, 4xx, 5xx)
