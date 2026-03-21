# Quickstart: Unit & Functional Tests

**Feature**: 005-unit-functional-tests
**Date**: 2026-03-21

## Запуск юнит-тестов

```bash
# Все юнит-тесты (без внешних зависимостей)
go test ./internal/services/*/application/... ./internal/services/*/domain/... ./internal/services/*/grpc/... ./internal/gateway/*/handlers/... ./internal/gateway/*/middleware/...

# Конкретный сервис
go test ./internal/services/auth/application/...
go test ./internal/services/messaging/application/...

# С покрытием (application-слой, цель >= 70%)
go test -cover ./internal/services/*/application/...

# Verbose (видна Describe/Context/It структура)
go test -v ./internal/services/auth/application/...
```

## Запуск функциональных тестов

```bash
# Требуется тестовая PostgreSQL и Redis
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=sms_test
export TEST_DB_PASSWORD=test_password
export TEST_DB_NAME=sms_test
export TEST_REDIS_HOST=localhost
export TEST_REDIS_PORT=6379

# Все функциональные тесты
go test ./test/functional/...

# Конкретная цепочка
go test -run TestMessagingChain ./test/functional/...
go test -run TestBillingChain ./test/functional/...
go test -run TestAuthChain ./test/functional/...
go test -run TestHLRRoutingChain ./test/functional/...
```

## Запуск существующих тестов (рефакторенных)

```bash
# Router, API, middleware, SMPP
go test ./internal/router/... ./internal/api/... ./internal/smpp/... ./internal/shared/...

# Integration (требуются запущенные сервисы)
go test -tags integration ./test/integration/...

# Load
go test -tags load ./test/load/...
```

## Структура тестового файла (шаблон)

```go
package application_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

    "sms/internal/services/{service}/application"
    "sms/internal/services/{service}/mocks"
)

func TestServiceName(t *testing.T) {
    t.Run("MethodName", func(t *testing.T) {
        t.Run("with valid input", func(t *testing.T) {
            // Setup mocks
            mockRepo := new(mocks.MockRepository)
            mockRepo.On("Method", mock.Anything, mock.Anything).Return(expected, nil)

            svc := application.NewService(mockRepo)

            // Act
            result, err := svc.Method(context.Background(), input)

            // Assert
            require.NoError(t, err)
            assert.Equal(t, expected, result)
            mockRepo.AssertExpectations(t)
        })

        t.Run("with invalid input", func(t *testing.T) {
            // Setup
            mockRepo := new(mocks.MockRepository)
            svc := application.NewService(mockRepo)

            // Act
            _, err := svc.Method(context.Background(), invalidInput)

            // Assert
            require.Error(t, err)
            assert.ErrorIs(t, err, domain.ErrInvalidInput)
            mockRepo.AssertExpectations(t)
        })
    })
}
```

## Структура функционального теста (шаблон)

```go
package functional_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "sms/internal/testutil"
)

func TestChainName(t *testing.T) {
    db, cleanup := testutil.SetupTestDB(t, testutil.GetTestDSN())
    defer cleanup()

    t.Run("happy path", func(t *testing.T) {
        tx := testutil.BeginTestTx(t, db)
        // tx автоматически откатится через t.Cleanup()

        // Setup: создание тестовых данных через tx
        // Act: вызов application-сервиса с tx-backed repositories
        // Assert: проверка состояния через tx queries
    })
}
```
