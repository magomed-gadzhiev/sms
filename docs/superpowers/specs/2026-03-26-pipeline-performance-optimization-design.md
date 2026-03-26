# Pipeline Performance Optimization — Design Spec

**Дата:** 2026-03-26
**Цель:** Максимальная пропускная способность SMS pipeline на текущем железе (8 CPU / 16 GB RAM)
**Подход:** "Full Send" — комплексная оптимизация от PostgreSQL тюнинга до изменения архитектуры ingestion

## Контекст

### Текущие bottleneck (нагрузочное тестирование)

| Сервис | CPU | Проблема |
|--------|-----|----------|
| PostgreSQL | 240% | Главный bottleneck |
| messaging-service | 66% | Синхронные DB write на каждое SMS |
| pipeline-router | 59% | DB read routes/providers без кеша |
| routing-service | 34% | DB read routes/providers без кеша |

### Корневые причины

- `shared_buffers = 128MB` при 16GB RAM — дефолт PostgreSQL
- 2 синхронных DB write на ingestion (INSERT pending + UPDATE queued) — на каждое SMS
- 1-2 DB read в router stage (routes: 13 строк, providers: 7 строк) — без кеша, 1.7M запросов за тест
- 241K insert в `message_stats` — дополнительная запись на каждое SMS
- 8 индексов на `messages` — каждый INSERT/UPDATE обновляет все

### Допущения

- Eventual consistency до 10 сек допустима для статуса сообщений
- DB write полностью убираем из hot path ingestion (API возвращает 202 без записи в БД)
- SMPP-провайдеры — симулятор без ограничений
- Kafka — source of truth, PostgreSQL — eventual persistence

---

## Секция 1: PostgreSQL тюнинг + PgBouncer

### PostgreSQL конфигурация

```ini
# Memory (16 GB RAM)
shared_buffers = 4GB
effective_cache_size = 12GB
work_mem = 64MB
maintenance_work_mem = 512MB
wal_buffers = 64MB

# WAL & Checkpoints
min_wal_size = 1GB
max_wal_size = 4GB
checkpoint_completion_target = 0.9

# Write Performance
synchronous_commit = off
wal_writer_delay = 200ms
commit_delay = 100

# Parallelism
max_worker_processes = 8
max_parallel_workers_per_gather = 4
max_parallel_workers = 8

# Connections (PgBouncer перед PostgreSQL)
max_connections = 100
```

**`synchronous_commit = off`** — ключевой параметр. Потеря до ~600ms транзакций при крэше. Допустимо: Kafka — source of truth, статусы можно переиграть.

### PgBouncer

```ini
[pgbouncer]
pool_mode = transaction
max_client_conn = 200
default_pool_size = 30
reserve_pool_size = 5
reserve_pool_timeout = 3
server_idle_timeout = 300
```

- Transaction pooling — освобождает connection после каждой транзакции
- 200 клиентских → 30 реальных к PostgreSQL
- Overhead ~0.1ms на hop

---

## Секция 2: Async Ingestion — убираем DB write из hot path

### Текущий flow (2 DB write на сообщение)

```
API Request → INSERT messages (pending) → UPDATE messages (queued) → Kafka publish → Response
```

### Новый flow (0 DB write на сообщение)

```
API Request → Validate → Kafka publish (sms.outgoing) → Response 202 Accepted
                                                              ↓ (async)
                                                    Persist Stage → COPY batch insert
```

### Изменения в messaging-service

- `SendMessage()` больше не вызывает `messageRepo.Create()` и `UpdateStatus()`
- Генерирует `message_id` (UUID), валидирует вход, считает сегменты
- Публикует в `sms.outgoing` — `RequiredAcks=WaitForAll` остаётся гарантией durability
- Возвращает `202 Accepted` с `message_id`

### Новый pipeline stage: Persist

- Отдельная consumer group на `sms.outgoing`
- Потребляет параллельно с Router stage
- Накапливает батч: **2000 сообщений или 100ms timeout**
- Пишет через **pgx `CopyFrom`** — бинарный протокол, одна операция на весь батч
- Статус сразу `queued` (пропускаем `pending`)

### COPY vs batch INSERT

| | INSERT ... VALUES (1000 строк) | COPY (1000 строк) |
|---|---|---|
| Latency | ~15-30ms | ~3-5ms |
| Парсинг SQL | Да, каждый раз | Нет, бинарный протокол |
| WAL | По строке | Bulk |
| Индексы | Построчно | Батчевое обновление |

### GET `/messages/{id}` в первые секунды

Возвращает 404 до persist. Допустимо при eventual consistency до 10 сек.

---

## Секция 3: In-memory кеш routes/providers в Router Stage

### Проблема

Каждое сообщение = 1-2 DB query на `routes` (13 строк) и `providers` (7 строк). 100K сообщений = 170K+ запросов к PostgreSQL за статическими данными.

### Решение: in-memory с periodic refresh

```go
type RouteCache struct {
    mu         sync.RWMutex
    routes     []Route
    providers  map[uuid.UUID]Provider
    refreshTTL time.Duration  // 30 сек
}
```

- При старте — загрузка всех routes и providers из БД
- Фоновая горутина обновляет каждые 30 секунд
- Чтение через `RLock` — наносекунды, zero allocation
- Fallback на прямой DB query если кеш пуст

### Почему не Redis

| | In-memory | Redis |
|---|---|---|
| Latency | ~10ns | ~0.5ms |
| Зависимости | Нет | Сетевой hop |
| Consistency | 30 сек stale | Тот же TTL |
| Сложность | ~50 строк | Сериализация + клиент |

Для 20 строк конфигурационных данных Redis — overhead без пользы.

---

## Секция 4: Оптимизация Status Stage

### Увеличение batch size по stages

```
Persist stage:  batch_size=2000, timeout=100ms  (тяжёлый DB write)
Status stage:   batch_size=2000, timeout=100ms  (тяжёлый DB write)
Router stage:   batch_size=1000, timeout=50ms   (Kafka→Kafka, быстрый)
Sender stage:   batch_size=500,  timeout=10ms   (SMPP window лимитирует)
```

Логика: чем тяжелее IO-операция, тем больше батч.

### Temp table + COPY для batch UPDATE

Вместо `UPDATE ... FROM (VALUES ...)` с тысячами inline параметров:

```sql
-- 1. Temp table (создаётся один раз при старте connection)
CREATE TEMP TABLE IF NOT EXISTS status_batch (
    id UUID, status TEXT, smpp_message_id TEXT,
    provider_id UUID, submitted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ, segment_count INT
) ON COMMIT DELETE ROWS;

-- 2. COPY данные в temp table (бинарный протокол)
COPY status_batch FROM STDIN (FORMAT binary);

-- 3. UPDATE из temp table
UPDATE messages SET
    status = s.status,
    smpp_message_id = COALESCE(s.smpp_message_id, messages.smpp_message_id),
    provider_id = COALESCE(s.provider_id, messages.provider_id),
    submitted_at = COALESCE(s.submitted_at, messages.submitted_at),
    updated_at = s.updated_at,
    segment_count = COALESCE(s.segment_count, messages.segment_count)
FROM status_batch s
WHERE messages.id = s.id AND messages.updated_at < s.updated_at;
```

При 2000 строк: COPY в temp + один UPDATE быстрее, чем `UPDATE ... FROM (VALUES ($1,...), ($2,...), ...)`.

### message_stats — вынести из hot path

Агрегировать в памяти, flush раз в 10-30 секунд вместо записи на каждое SMS.

---

## Секция 5: Kafka партиционирование + Pipeline concurrency

### Партиционирование топиков

```
sms.outgoing    32 partitions   key=message_id     равномерная нагрузка на persist + router
sms.routed      32 partitions   key=provider_id    sender группирует по провайдеру
sms.sent        16 partitions   key=message_id     равномерная нагрузка на status
sms.dlr         16 partitions   key=message_id
sms.status      16 partitions   key=message_id
```

`sms.routed` по `provider_id` — sender инстансы специализируются по провайдерам, лучше утилизация SMPP connections.

### Pipeline worker scaling

```
pipeline-router:   replicas=2, workers=4  → 8 параллельных consumer goroutines
pipeline-sender:   replicas=4, workers=4  → 16 параллельных
pipeline-status:   replicas=2, workers=2  → 4 параллельных DB writers
pipeline-persist:  replicas=2, workers=2  → 4 параллельных COPY writers (новый)
```

### SMPP Window Size

```
Текущий:  window_size=50
Новый:    window_size=500
```

При 4 sender replicas × 4 workers × connection per provider — потенциально 8000 in-flight PDU.

### Backpressure

`MaxWindow` в backpressure manager: 50 → 500. `BurstSize` пропорционально.

---

## Секция 6: Оптимизация индексов на hot path

### Стратегия: минимум индексов на hot partitions

На текущем месяце (active writes) — только критичные индексы:

- **PK** `(id, created_at)` — обязательный
- **message_id** — lookup по ID из API
- **status + created_at** — DLR expiry, scheduled messages

Убираем с hot partition (5 индексов):
- `destination`, `client_id`, `provider_id`, `external_id`, `next_retry_at`

### Реализация

```sql
-- Hot partition — минимум индексов
CREATE TABLE messages_2026_04 PARTITION OF messages
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE INDEX idx_msg_2026_04_message_id ON messages_2026_04 (message_id);
CREATE INDEX idx_msg_2026_04_status_created ON messages_2026_04 (status, created_at);

-- При ротации — добавить полные индексы на прошлый месяц (CONCURRENTLY)
CREATE INDEX CONCURRENTLY idx_msg_2026_03_destination ON messages_2026_03 (destination);
CREATE INDEX CONCURRENTLY idx_msg_2026_03_client_id ON messages_2026_03 (client_id);
CREATE INDEX CONCURRENTLY idx_msg_2026_03_provider_id ON messages_2026_03 (provider_id);
CREATE INDEX CONCURRENTLY idx_msg_2026_03_external_id ON messages_2026_03 (external_id);
CREATE INDEX CONCURRENTLY idx_msg_2026_03_next_retry ON messages_2026_03 (next_retry_at);
```

### Эффект

Каждый убранный индекс — минус ~10-15% overhead на write. 5 индексов = ~30-40% выигрыш на INSERT/UPDATE.

---

## Ожидаемый суммарный эффект

| Оптимизация | Эффект на throughput |
|---|---|
| PostgreSQL тюнинг (`shared_buffers`, `synchronous_commit=off`) | +50-80% |
| PgBouncer (connection pooling) | +10-20% |
| Async ingestion (0 DB write в hot path) | +200-300% |
| In-memory route cache | +20-30% (разгружает PG CPU) |
| COPY protocol (persist + status) | +30-50% vs batch INSERT |
| Kafka 32 partitions + scaling | +50-100% |
| SMPP window 500 | +100-200% (при simulator) |
| Минимум индексов на hot partition | +30-40% на writes |

**Совокупный ожидаемый эффект: 5-10x от текущего throughput.**

Точные цифры определяются нагрузочным тестом после реализации.

---

## Что НЕ входит в scope

- Шардинг PostgreSQL (не нужен для одного сервера)
- Миграция на другую БД (TimescaleDB, ClickHouse)
- Read replicas (нет read-heavy нагрузки после кеширования)
- Kafka Streams / KsqlDB (over-engineering для текущего масштаба)
- Instant invalidation кеша через Kafka topic (YAGNI, TTL 30 сек достаточно)
