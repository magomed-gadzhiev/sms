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

## Фикстуры

### SQL (`test/load/fixtures/`)

| Файл | Описание |
|------|----------|
| `seed.sql` | Полный сидинг БД для load-тестов |
| `cleanup.sql` | Удаление только load-test данных |

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
