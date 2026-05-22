# Integration Tests

Интеграционные тесты проверяют взаимодействие между компонентами с реальными зависимостями.

## Требования

Для запуска integration тестов необходимо:

1. **PostgreSQL** - запущенная база данных
   - По умолчанию: `localhost:5432`
   - База данных: `smpp_test`
   - Пользователь: `smpp_test`
   - Пароль: `smpp_test`

2. **Kafka** - запущенный Kafka брокер
   - По умолчанию: `localhost:9092`

## Переменные окружения

Можно настроить подключение через переменные окружения:

```bash
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=smpp_test
export TEST_DB_PASSWORD=smpp_test
export TEST_DB_NAME=smpp_test
```

## Запуск тестов

```bash
# Все integration тесты
go test -tags=integration ./test/integration/...

# Конкретный тест
go test -tags=integration -v ./test/integration/... -run TestMessageRepository_Create

# С verbose выводом
go test -tags=integration -v ./test/integration/...
```

## Использование Docker Compose

Можно использовать Docker Compose для запуска тестовых сервисов:

```bash
# Запуск тестовых сервисов
docker-compose -f deployments/docker-compose.test.yml up -d

# Запуск тестов
go test -tags=integration ./test/integration/...

# Остановка сервисов
docker-compose -f deployments/docker-compose.test.yml down
```

## Использование Testcontainers

Для автоматического управления контейнерами можно использовать testcontainers:

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestWithPostgreSQL(t *testing.T) {
    ctx := context.Background()
    
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections"),
    }
    
    postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer postgresC.Terminate(ctx)
    
    // Использование контейнера для тестов
}
```

3. **Directus** - запущенная админ-панель (опционально, для тестов Directus)
   - По умолчанию: `http://localhost:8055`
   - Используйте переменные окружения для настройки:
     - `DIRECTUS_URL` (по умолчанию: `http://localhost:8055`)
     - `DIRECTUS_ADMIN_EMAIL` (по умолчанию: `admin@example.com`)
     - `DIRECTUS_ADMIN_PASSWORD` (по умолчанию: `admin`)

## Переменные окружения

Можно настроить подключение через переменные окружения:

```bash
# База данных
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=smpp_test
export TEST_DB_PASSWORD=smpp_test
export TEST_DB_NAME=smpp_test

# Directus (для тестов админ-панели)
export DIRECTUS_URL=http://localhost:8055
export DIRECTUS_ADMIN_EMAIL=admin@example.com
export DIRECTUS_ADMIN_PASSWORD=admin
```

## Запуск тестов

```bash
# Все integration тесты
go test -tags=integration ./test/integration/...

# Конкретный тест
go test -tags=integration -v ./test/integration/... -run TestMessageRepository_Create

# Тесты Directus
go test -tags=integration -v ./test/integration/... -run TestDirectus

# С verbose выводом
go test -tags=integration -v ./test/integration/...
```

## Использование Docker Compose

Можно использовать Docker Compose для запуска тестовых сервисов:

```bash
# Запуск всех сервисов, включая Directus
docker-compose -f deployments/docker-compose.yml up -d

# Запуск только необходимых сервисов для тестов
docker-compose -f deployments/docker-compose.yml up -d postgres directus

# Запуск тестов
go test -tags=integration ./test/integration/...

# Остановка сервисов
docker-compose -f deployments/docker-compose.yml down
```

## Использование Testcontainers

Для автоматического управления контейнерами можно использовать testcontainers:

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestWithPostgreSQL(t *testing.T) {
    ctx := context.Background()
    
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections"),
    }
    
    postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer postgresC.Terminate(ctx)
    
    // Использование контейнера для тестов
}
```

## Структура тестов

- `storage_test.go` - тесты для репозиториев (PostgreSQL)
- `kafka_test.go` - тесты для Kafka producer/consumer
- `directus_test.go` - тесты для Directus админ-панели (CRUD операции)

## Тесты Directus

Тесты Directus проверяют работу админ-панели через REST API:

- **TestDirectusHealth** - проверка доступности Directus
- **TestDirectusClientCRUD** - полный цикл CRUD для коллекции `clients`
- **TestDirectusProviderCRUD** - полный цикл CRUD для коллекции `providers`
- **TestDirectusRouteCRUD** - полный цикл CRUD для коллекции `routes` (требует провайдера)
- **TestDirectusListItems** - получение списка элементов из коллекций

### Запуск тестов Directus

Перед запуском тестов убедитесь, что:
1. Directus запущен и доступен (по умолчанию `http://localhost:8055`)
2. Настроены правильные учетные данные администратора
3. Коллекции настроены в Directus (можно использовать `scripts/setup-directus.js`)

```bash
# Запуск всех тестов Directus
go test -tags=integration -v ./test/integration/... -run TestDirectus

# Запуск конкретного теста
go test -tags=integration -v ./test/integration/... -run TestDirectusClientCRUD
```

## Примечания

- Тесты автоматически очищают данные после выполнения (кроме тестов Directus - они удаляют созданные тестовые записи)
- Каждый тест должен быть независимым
- Используйте `t.Skip()` если сервисы недоступны
- Тесты Directus требуют, чтобы коллекции были настроены в Directus (выполните `scripts/setup-directus.js` перед тестами)
