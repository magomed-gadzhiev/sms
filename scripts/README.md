# Скрипты управления SMPP Server

## Windows (PowerShell)

### Запуск сервисов

```powershell
# Обычный запуск
.\scripts\start.ps1

# Запуск с пересборкой
.\scripts\start.ps1 -Build

# Запуск с применением миграций
.\scripts\start.ps1 -Migrate

# Полная очистка и запуск
.\scripts\start.ps1 -Clean -Migrate -Build
```

### Остановка сервисов

```powershell
# Обычная остановка
.\scripts\stop.ps1

# Остановка с удалением данных
.\scripts\stop.ps1 -Clean
```

### Проверка статуса

```powershell
.\scripts\status.ps1
```

### Генерация proto файлов

```powershell
.\scripts\generate-proto.ps1
```

### Нагрузочное тестирование

```powershell
# 1. Сначала настройте лимиты для нагрузочного тестирования
.\scripts\setup-load-test-limits.ps1

# 2. Запустите нагрузочный тест
.\scripts\load-test.ps1

# Или с кастомными параметрами
.\scripts\load-test.ps1 -VUs 100 -Duration 60

# С указанием URL и API ключа
.\scripts\load-test.ps1 -BaseUrl "http://localhost:8080" -ApiKey "your-api-key"

# Быстрый тест с фиксированным количеством пользователей и временем
.\scripts\load-test.ps1 -VUs 100 -Duration 60

# Максимальная нагрузка (использует настройки из скрипта: до 5000 VU)
.\scripts\load-test.ps1
```

## Linux/Mac (Bash)

### Запуск сервисов

```bash
# Обычный запуск
./scripts/start.sh

# Запуск с пересборкой
./scripts/start.sh --build

# Запуск с применением миграций
./scripts/start.sh --migrate

# Полная очистка и запуск
./scripts/start.sh --clean --migrate --build
```

### Остановка сервисов

```bash
# Обычная остановка
./scripts/stop.sh

# Остановка с удалением данных
./scripts/stop.sh --clean
```

### Генерация proto файлов

```bash
./scripts/generate-proto.sh
```

### Нагрузочное тестирование

```bash
# 1. Сначала настройте лимиты для нагрузочного тестирования
./scripts/setup-load-test-limits.sh

# 2. Запустите нагрузочный тест
./scripts/load-test.sh

# Или с кастомными параметрами
./scripts/load-test.sh --vus 100 --duration 60

# С указанием URL и API ключа
./scripts/load-test.sh --base-url "http://localhost:8080" --api-key "your-api-key"

# Быстрый тест с фиксированным количеством пользователей и временем
./scripts/load-test.sh --vus 100 --duration 60

# Максимальная нагрузка (использует настройки из скрипта: до 5000 VU)
./scripts/load-test.sh
```

## Описание скриптов

### start.ps1 / start.sh

Запускает все сервисы SMPP Server:

**Параметры:**
- `-Build` / `--build` - пересобрать Docker образы перед запуском
- `-Migrate` / `--migrate` - применить миграции БД
- `-Clean` / `--clean` - удалить существующие контейнеры и volumes перед запуском

**Процесс запуска:**
1. Проверка наличия Docker и docker-compose
2. Запуск инфраструктурных сервисов (PostgreSQL, Redis, Kafka, Zookeeper)
3. Ожидание готовности PostgreSQL
4. Применение миграций БД (если указан флаг)
5. Запуск приложений (API Gateway, SMPP Server, Workers)
6. Запуск мониторинга (Prometheus, Grafana, HAProxy)
7. Вывод статуса и доступных endpoints

### stop.ps1 / stop.sh

Останавливает все сервисы:

**Параметры:**
- `-Clean` / `--clean` - удалить volumes с данными (БД, Kafka, Redis)

### status.ps1

Проверяет статус всех сервисов:
- Статус контейнеров
- Health check endpoints
- Использование ресурсов

### generate-proto.ps1 / generate-proto.sh

Генерирует Go код из proto файлов:
- Устанавливает необходимые плагины (если не установлены)
- Генерирует gRPC и протобуф код
- Сохраняет в `api/proto/smsv1/`

### setup-directus.ps1 / setup-directus.sh

Настраивает коллекции в Directus после первого запуска:
- Скрывает чувствительные поля (password_hash, secret, api_key, key_hash)
- Настраивает relationships между коллекциями
- Настраивает интерфейсы для полей (JSON, UUID, timestamps)

**Параметры:**
- `-Url` / `--url` - URL Directus (по умолчанию: http://localhost:8055)
- `-Email` / `--email` - Email администратора Directus (по умолчанию: admin@example.com)
- `-Password` / `--password` - Пароль администратора Directus (по умолчанию: admin)

**Использование:**

```powershell
# Windows
.\scripts\setup-directus.ps1

# С кастомными параметрами
.\scripts\setup-directus.ps1 -Url "http://localhost:8055" -Email "admin@example.com" -Password "admin"

# Linux/Mac
./scripts/setup-directus.sh

# С кастомными параметрами
./scripts/setup-directus.sh --url "http://localhost:8055" --email "admin@example.com" --password "admin"
```

**Требования:**
- Node.js должен быть установлен
- Directus должен быть запущен и доступен
- Администратор Directus должен быть создан

**Примечание:** Этот скрипт необходимо запустить после первого запуска Directus для правильной настройки коллекций.

### setup-load-test-limits.ps1 / setup-load-test-limits.sh

Настраивает лимиты rate limiting для нагрузочного тестирования:

**Параметры:**
- `-ApiKey` / `--api-key` - API ключ клиента (по умолчанию: test-api-key)
- `-PerSecond` / `--per-second` - Лимит запросов в секунду (по умолчанию: 10000)
- `-PerMinute` / `--per-minute` - Лимит запросов в минуту (по умолчанию: 600000)
- `-PerHour` / `--per-hour` - Лимит запросов в час (по умолчанию: 36000000)

**Процесс:**
1. Обновление лимитов в базе данных PostgreSQL
2. Очистка счетчиков rate limiting в Redis

**Важно:** Запускайте этот скрипт перед нагрузочным тестированием, чтобы избежать ошибок 429 (Too Many Requests).

### load-test.ps1 / load-test.sh

Запускает нагрузочное тестирование с помощью k6:

**Параметры:**
- `-BaseUrl` / `--base-url` - URL API Gateway (по умолчанию: http://localhost:8080)
- `-ApiKey` / `--api-key` - API ключ для аутентификации (по умолчанию: test-api-key)
- `-Duration` / `--duration` - Длительность теста в секундах (0 = использовать настройки из скрипта)
- `-VUs` / `--vus` - Количество виртуальных пользователей (0 = использовать настройки из скрипта)

**Процесс:**
1. Проверка доступности Docker и сервисов
2. Автоматическое определение Docker сети (если доступна)
3. Запуск k6 в Docker контейнере
4. Вывод результатов тестирования

**Настройка теста:**
Параметры нагрузки настраиваются в файле `scripts/k6_load_test.js`:
- `stages` - этапы нагрузки (количество пользователей и длительность)
- `thresholds` - пороги производительности
- `sleep()` - задержка между запросами

**Ручной запуск через Docker:**

```powershell
# Windows
docker run --rm --network deployments_smpp-network `
  -v "${PWD}/scripts:/scripts" `
  grafana/k6 run /scripts/k6_load_test.js `
  -e BASE_URL=http://haproxy:8080 `
  -e API_KEY=test-api-key

# Linux/Mac
docker run --rm --network deployments_smpp-network \
  -v "$(pwd)/scripts:/scripts" \
  grafana/k6 run /scripts/k6_load_test.js \
  -e BASE_URL=http://haproxy:8080 \
  -e API_KEY=test-api-key
```

**Если k6 установлен локально:**

```bash
# Windows (PowerShell)
k6 run scripts/k6_load_test.js -e BASE_URL=http://localhost:8080 -e API_KEY=test-api-key

# Linux/Mac
k6 run scripts/k6_load_test.js -e BASE_URL=http://localhost:8080 -e API_KEY=test-api-key
```

## Доступные endpoints после запуска

| Сервис | URL | Описание |
|--------|-----|----------|
| HTTP API | http://localhost:8080 | REST API для отправки SMS |
| gRPC API | http://localhost:9090 | gRPC API для отправки SMS |
| SMPP Server | localhost:2775 | SMPP протокол для клиентов |
| Grafana | http://localhost:3000 | Дашборды мониторинга (admin/admin) |
| Prometheus | http://localhost:9091 | Метрики сервисов |
| HAProxy Stats | http://localhost:8404/stats | Статистика балансировщика |
| Directus UI | http://localhost:8055 | Админ-панель Directus |

## Примеры использования

### Первый запуск

```powershell
# Windows
.\scripts\start.ps1 -Build -Migrate

# Linux/Mac
./scripts/start.sh --build --migrate
```

### Обычный запуск (после первого)

```powershell
# Windows
.\scripts\start.ps1

# Linux/Mac
./scripts/start.sh
```

### Перезапуск с пересборкой

```powershell
# Windows
.\scripts\stop.ps1
.\scripts\start.ps1 -Build

# Linux/Mac
./scripts/stop.sh
./scripts/start.sh --build
```

### Полная очистка и перезапуск

```powershell
# Windows
.\scripts\start.ps1 -Clean -Migrate -Build

# Linux/Mac
./scripts/start.sh --clean --migrate --build
```

## Просмотр логов

```bash
# Все сервисы
docker-compose -f deployments/docker-compose.yml logs -f

# Конкретный сервис
docker-compose -f deployments/docker-compose.yml logs -f api-gateway-1
docker-compose -f deployments/docker-compose.yml logs -f smpp-server
docker-compose -f deployments/docker-compose.yml logs -f worker-1
```

## Troubleshooting

### Ошибка "port already allocated"

Проверьте, не запущены ли другие сервисы на занятых портах:
- 8080 (HTTP API)
- 9090 (gRPC API)
- 2775 (SMPP)
- 3000 (Grafana)
- 9091 (Prometheus)
- 5432 (PostgreSQL)
- 6379 (Redis)
- 9092 (Kafka)

### Ошибка миграций

```powershell
# Проверить статус миграций
docker run --rm \
  -v ./migrations:/migrations \
  --network deployments_smpp-network \
  migrate/migrate \
  -path /migrations \
  -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" \
  version

# Откатить последнюю миграцию
docker run --rm \
  -v ./migrations:/migrations \
  --network deployments_smpp-network \
  migrate/migrate \
  -path /migrations \
  -database "postgres://smpp:${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set}@postgres:5432/smpp_db?sslmode=disable" \
  down 1
```

### Сервисы не запускаются

1. Проверьте логи: `docker-compose -f deployments/docker-compose.yml logs [service-name]`
2. Проверьте статус: `.\scripts\status.ps1` или `docker-compose -f deployments/docker-compose.yml ps`
3. Пересоберите образы: `.\scripts\start.ps1 -Build` или `./scripts/start.sh --build`
4. Полная очистка: `.\scripts\start.ps1 -Clean -Migrate -Build`
