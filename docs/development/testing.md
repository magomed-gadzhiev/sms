# Тестирование

## Обзор

Проект использует стандартную библиотеку `testing` Go для написания тестов. Цель - достичь coverage > 70%.

## Запуск тестов

### Все тесты

```bash
# В dev контейнере или локально
go test ./...

# Используя скрипт (Linux/Mac)
./scripts/test.sh

# Используя скрипт (Windows PowerShell)
.\scripts\test.ps1
```

### Unit тесты

```bash
# Все unit тесты
go test ./internal/...

# С coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out

# С race detector
go test -race ./internal/...
```

### Integration тесты

Integration тесты требуют запущенных сервисов (PostgreSQL, Kafka).

```bash
# Запуск integration тестов
go test -tags=integration ./test/integration/...

# Используя скрипт
./scripts/test.sh --integration
.\scripts\test.ps1 -Integration
```

#### Опциональные сервисы: контракт деградации

Часть integration-тестов работает с сервисами, которые есть не во всех
окружениях. Такие тесты обязаны деградировать в явный SKIP, а не падать:

- **Registration-тесты** (`test/integration/registration_test.go`) ходят по
  HTTP в portal-gateway (`TEST_PORTAL_URL`, по умолчанию
  `http://localhost:8082` — порт Portal Gateway HTTP из
  `deployments/docker-compose.yml`). Если портал не развёрнут (connection
  refused) или не экспонирует роут (404/405) — тест SKIP-ается. HTTP-покрытие
  регистрации живёт в e2e (`e2e/tests/auth/auth-public.spec.ts`: успех,
  дубликат email, невалидный ввод). Чтобы прогнать registration-тесты по-настоящему,
  поднимите портал и задайте `TEST_PORTAL_URL` (make-таргет `test-integration`
  уже проставляет `http://127.0.0.1:8082`).

Решение (2026-09-23): в CI integration-джоба поднимает только postgres —
registration-тесты там сознательно SKIP-аются; запуск портала в этой джобе
не планируется до отдельного решения.

### Load тесты

Load тесты требуют запущенного API Gateway.

```bash
# Запуск load тестов
go test -tags=load ./test/load/...

# Используя скрипт
./scripts/test.sh --load
.\scripts\test.ps1 -Load
```

### Тесты конкретного пакета

```bash
go test ./internal/api/http
go test -v ./internal/router
```

### Тесты с race detector

```bash
go test -race ./...
```

### Тесты с coverage

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Используя скрипт с coverage
.\scripts\test.ps1 -Coverage
```

### Verbose режим

```bash
go test -v ./...
```

## Типы тестов

### Unit тесты

Тестирование отдельных функций и методов изолированно.

**Расположение:** В том же пакете, что и тестируемый код, файл `*_test.go`

**Пример:**

```go
package http

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestSendSMSRequest_Validate(t *testing.T) {
    tests := []struct {
        name    string
        request SendSMSRequest
        wantErr bool
    }{
        {
            name: "valid request",
            request: SendSMSRequest{
                Source:      "12345",
                Destination: "79001234567",
                Text:        "Test message",
            },
            wantErr: false,
        },
        {
            name: "missing source",
            request: SendSMSRequest{
                Destination: "79001234567",
                Text:        "Test message",
            },
            wantErr: true,
        },
        {
            name: "missing destination",
            request: SendSMSRequest{
                Source: "12345",
                Text:   "Test message",
            },
            wantErr: true,
        },
        {
            name: "missing text",
            request: SendSMSRequest{
                Source:      "12345",
                Destination: "79001234567",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.request.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration тесты

Тестирование взаимодействия между компонентами с реальными зависимостями.

**Требования:**
- Запущенные сервисы (PostgreSQL, Kafka, Redis)
- Тестовые данные
- Теги сборки: `-tags=integration`

**Расположение:** `test/integration/`

**Пример:**

```go
package integration

import (
    "context"
    "testing"
    "github.com/stretchr/testify/require"
)

func TestMessageRepository_Create(t *testing.T) {
    // Настройка тестовой БД
    db := setupTestDB(t)
    defer db.Close()

    repo := storage.NewMessageRepository(db)

    msg := &shared.Message{
        Source:      "12345",
        Destination: "79001234567",
        Text:        "Test message",
    }

    err := repo.Create(context.Background(), msg)
    require.NoError(t, err)
    require.NotEmpty(t, msg.ID)
}
```

### Mock тесты

Использование моков для изоляции тестируемого кода.

**Пример с интерфейсом:**

```go
// Интерфейс
type MessageProducer interface {
    PublishOutgoing(ctx context.Context, msg *queue.KafkaMessage) error
}

// Мок
type MockProducer struct {
    PublishOutgoingFunc func(ctx context.Context, msg *queue.KafkaMessage) error
}

func (m *MockProducer) PublishOutgoing(ctx context.Context, msg *queue.KafkaMessage) error {
    if m.PublishOutgoingFunc != nil {
        return m.PublishOutgoingFunc(ctx, msg)
    }
    return nil
}

// Тест
func TestHandler_SendSMS(t *testing.T) {
    mockProducer := &MockProducer{
        PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
            assert.Equal(t, "12345", msg.Source)
            return nil
        },
    }

    handler := NewHandler(mockProducer)
    // ...
}
```

## Тестовые утилиты

### Testutil пакет

В проекте есть пакет `internal/testutil` с вспомогательными утилитами:

- **Mocks** (`mocks.go`) - моки для репозиториев и producer
- **Fixtures** (`fixtures.go`) - функции для создания тестовых данных
- **TestDB** (`testdb.go`) - утилиты для работы с тестовой БД

**Пример использования:**

```go
import "github.com/smpp-server/smpp-server/internal/testutil"

// Создание тестового сообщения
msg := testutil.NewTestMessage()

// Создание мока producer
mockProducer := &testutil.MockProducer{
    PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
        // ваша логика
        return nil
    },
}
```

### Testify

Рекомендуется использовать `testify` для assertions:

```bash
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require
```

**Использование:**

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
    result, err := someFunction()
    
    // require прерывает тест при ошибке
    require.NoError(t, err)
    
    // assert продолжает тест
    assert.Equal(t, "expected", result)
    assert.NotEmpty(t, result)
    assert.True(t, someCondition)
}
```

### Testcontainers

Для integration тестов с реальными сервисами можно использовать `testcontainers`:

```go
import (
    "testing"
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

## Тестирование HTTP handlers

```go
package http

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestHandler_SendSMS(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    handler := NewHandler(mockProducer, mockRepo)
    router := gin.New()
    router.POST("/api/v1/sms/send", handler.SendSMS)
    
    reqBody := SendSMSRequest{
        Source:      "12345",
        Destination: "79001234567",
        Text:        "Test message",
    }
    
    body, _ := json.Marshal(reqBody)
    req := httptest.NewRequest("POST", "/api/v1/sms/send", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-API-Key", "test-api-key")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    
    var resp SendSMSResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NotEmpty(t, resp.MessageID)
    assert.Equal(t, "queued", resp.Status)
}
```

## Тестирование gRPC handlers

```go
package grpc

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc"
)

func TestSMSService_SendSMS(t *testing.T) {
    handler := NewSMSService(mockProducer, mockRepo)
    
    req := &smsv1.SendSMSRequest{
        Source:      "12345",
        Destination: "79001234567",
        Text:        "Test message",
    }
    
    resp, err := handler.SendSMS(context.Background(), req)
    
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.MessageId)
    assert.Equal(t, "queued", resp.Status)
}
```

## Тестирование SMPP протокола

```go
package protocol

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestEncodeSubmitSM(t *testing.T) {
    encoder := NewEncoder()
    
    submit := &SubmitSMPDU{
        SourceAddr:      "1234567890",
        DestinationAddr: "0987654321",
        ShortMessage:    []byte("Test message"),
    }
    
    data, err := encoder.EncodeSubmitSM(submit)
    assert.NoError(t, err)
    assert.NotEmpty(t, data)
    
    // Декодирование обратно
    decoder := NewDecoder(data)
    decoded, err := decoder.DecodeSubmitSM(data)
    assert.NoError(t, err)
    assert.Equal(t, submit.SourceAddr, decoded.SourceAddr)
}
```

## Нагрузочное тестирование

### Go load тесты

Load тесты написаны на Go и находятся в `test/load/`.

**Требования:**
- Запущенный API Gateway
- Теги сборки: `-tags=load`

**Примеры тестов:**
- `TestAPIGateway_Load` - базовое нагрузочное тестирование
- `TestAPIGateway_Stress` - стресс-тестирование с увеличивающейся нагрузкой
- `TestAPIGateway_SendBatchSMS_Load` - нагрузка на batch endpoint
- `TestAPIGateway_GetStatus_Load` - нагрузка на получение статуса
- `TestAPIGateway_MixedLoad` - смешанная нагрузка на все эндпоинты
- `BenchmarkAPIGateway_SendSMS` - бенчмарк для отправки SMS

**Запуск:**
```bash
go test -tags=load ./test/load/...
```

### Использование k6

Пример скрипта для k6:

```javascript
import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 100 },
        { duration: '1m', target: 200 },
        { duration: '30s', target: 0 },
    ],
};

export default function () {
    const payload = JSON.stringify({
        source: '12345',
        destination: '79001234567',
        text: 'Load test message',
    });
    
    const params = {
        headers: {
            'Content-Type': 'application/json',
            'X-API-Key': 'test-api-key',
        },
    };
    
    const res = http.post('http://localhost:8080/api/v1/sms/send', payload, params);
    
    check(res, {
        'status is 200': (r) => r.status === 200,
        'response has message_id': (r) => JSON.parse(r.body).message_id !== undefined,
    });
}
```

Запуск:

```bash
k6 run load-test.js
```

## Best Practices

1. **Именование тестов**
   - Используйте понятные имена: `TestFunctionName_Scenario`
   - Используйте табличные тесты для множественных сценариев

2. **Изоляция тестов**
   - Каждый тест должен быть независимым
   - Используйте `t.Cleanup()` для очистки

3. **Тестовые данные**
   - Используйте фикстуры для сложных данных
   - Избегайте хардкода, используйте константы

4. **Покрытие кода**
   - Стремитесь к coverage > 70%
   - Тестируйте edge cases и error paths

5. **Производительность**
   - Используйте `-race` для обнаружения race conditions
   - Избегайте медленных тестов в unit тестах

6. **Документация**
   - Комментируйте сложные тесты
   - Используйте понятные имена переменных

## CI/CD интеграция

Тесты должны запускаться автоматически в CI/CD pipeline:

```yaml
# Пример GitHub Actions
- name: Run tests
  run: |
    go test -v -race -coverprofile=coverage.out ./...
    
- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    file: ./coverage.out
```

## Бенчмарки и профилирование

Для детальной информации о бенчмарках и профилировании см. [benchmarking.md](benchmarking.md).

## Дополнительная документация

- [Руководство для контрибьюторов](contributing.md)
- [Решение проблем](troubleshooting.md)
- [Бенчмарки и профилирование](benchmarking.md)