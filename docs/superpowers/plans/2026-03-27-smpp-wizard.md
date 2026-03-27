# SMPP Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Клиент самостоятельно подключает SMPP-провайдера через 6-шаговый portal wizard с live-тестом соединения и per-client изоляцией.

**Architecture:** Новые gRPC-методы в provider-service (CreateClientProvider, ListClientProviders, TestClientProviderConnection) + HTTP-хендлер в portal-gateway + React wizard на отдельной странице `/providers/new?step=N`. Данные изолируются по `client_id` из session-контекста.

**Tech Stack:** Go 1.24, gorilla/mux, jackc/pgx/v5, google.golang.org/grpc, React 19, TypeScript 5.7, Tailwind CSS 4.2, Radix UI.

---

## File Map

| Действие | Файл | Назначение |
|----------|------|-----------|
| Create | `migrations/000033_add_provider_client_id.up.sql` | Добавить client_id в providers |
| Create | `migrations/000033_add_provider_client_id.down.sql` | Откат миграции |
| Modify | `internal/shared/models.go` | Добавить ClientID в Provider struct |
| Modify | `internal/storage/provider_repository.go` | Методы с фильтрацией по client_id |
| Modify | `internal/services/provider/domain/provider.go` | Добавить ClientID и RoutingRule |
| Modify | `internal/services/provider/domain/repository.go` | Новые методы в интерфейсе |
| Modify | `internal/services/provider/infrastructure/repository/provider_repository.go` | Реализация новых методов |
| Create | `internal/services/provider/application/client_provider_service.go` | CRUD use cases для client-провайдеров |
| Create | `internal/services/provider/application/test_connection.go` | SMPP bind-тест с таймаутом |
| Modify | `internal/services/provider/grpc/server.go` | Новые gRPC-хендлеры |
| Modify | `api/proto/provider/provider.proto` | Новые rpc + messages |
| Modify | `internal/gateway/portal/clients.go` | Добавить ProviderClient |
| Modify | `cmd/portal-gateway/main.go` | Передать PROVIDER_SERVICE_ADDR |
| Create | `internal/gateway/portal/handlers/providers.go` | HTTP-хендлер /portal/v1/providers |
| Modify | `internal/gateway/portal/router/router.go` | Зарегистрировать маршруты |
| Modify | `portal-frontend/src/api/client.ts` | Добавить providersApi |
| Create | `portal-frontend/src/pages/providers/components/WizardProgress.tsx` | Прогресс-бар шагов |
| Create | `portal-frontend/src/pages/providers/components/WizardNav.tsx` | Кнопки Назад/Далее |
| Create | `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx` | Шаг 1: название, описание |
| Create | `portal-frontend/src/pages/providers/steps/Step2Connection.tsx` | Шаг 2: host, port, credentials |
| Create | `portal-frontend/src/pages/providers/steps/Step3Params.tsx` | Шаг 3: параметры соединения |
| Create | `portal-frontend/src/pages/providers/steps/Step4Test.tsx` | Шаг 4: live SMPP тест |
| Create | `portal-frontend/src/pages/providers/steps/Step5Routing.tsx` | Шаг 5: routing rules |
| Create | `portal-frontend/src/pages/providers/steps/Step6Summary.tsx` | Шаг 6: обзор + активация |
| Create | `portal-frontend/src/pages/providers/ProviderWizardPage.tsx` | Контейнер wizard |
| Create | `portal-frontend/src/pages/providers/ProvidersPage.tsx` | Список провайдеров клиента |
| Modify | `portal-frontend/src/App.tsx` | Добавить роуты /providers |

---

## Task 1: DB Migration — добавить client_id в providers

**Files:**
- Create: `migrations/000033_add_provider_client_id.up.sql`
- Create: `migrations/000033_add_provider_client_id.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000033_add_provider_client_id.up.sql
ALTER TABLE providers
  ADD COLUMN IF NOT EXISTS client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS tps_limit INTEGER NOT NULL DEFAULT 100,
  ADD COLUMN IF NOT EXISTS routing_rules JSONB NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_providers_client_id ON providers(client_id);
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000033_add_provider_client_id.down.sql
DROP INDEX IF EXISTS idx_providers_client_id;
ALTER TABLE providers
  DROP COLUMN IF EXISTS routing_rules,
  DROP COLUMN IF EXISTS tps_limit,
  DROP COLUMN IF EXISTS tags,
  DROP COLUMN IF EXISTS description,
  DROP COLUMN IF EXISTS client_id;
```

- [ ] **Step 3: Применить миграцию локально**

```bash
# Если используется golang-migrate:
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/sms?sslmode=disable" up 1
# Или через скрипт деплоя:
bash scripts/server.sh migrate
```

Ожидаемый результат: миграция применена без ошибок.

- [ ] **Step 4: Commit**

```bash
git add migrations/000033_add_provider_client_id.up.sql migrations/000033_add_provider_client_id.down.sql
git commit -m "feat(db): add client_id and routing_rules to providers table"
```

---

## Task 2: Shared model — обновить Provider struct

**Files:**
- Modify: `internal/shared/models.go`

- [ ] **Step 1: Добавить поля в Provider struct**

Найди `type Provider struct` в `internal/shared/models.go` и добавь поля после `UpdatedAt`:

```go
type Provider struct {
    ID                uuid.UUID  `json:"id" db:"id"`
    Name              string     `json:"name" db:"name"`
    Host              string     `json:"host" db:"host"`
    Port              int        `json:"port" db:"port"`
    SystemID          string     `json:"system_id" db:"system_id"`
    Password          string     `json:"password" db:"password"`
    SystemType        string     `json:"system_type" db:"system_type"`
    BindType          string     `json:"bind_type" db:"bind_type"`
    BindTON           int        `json:"bind_ton" db:"bind_ton"`
    BindNPI           int        `json:"bind_npi" db:"bind_npi"`
    AddrTON           int        `json:"addr_ton" db:"addr_ton"`
    AddrNPI           int        `json:"addr_npi" db:"addr_npi"`
    AddressRange      string     `json:"address_range" db:"address_range"`
    MaxConnections    int        `json:"max_connections" db:"max_connections"`
    Active            bool       `json:"active" db:"active"`
    Priority          int        `json:"priority" db:"priority"`
    ThroughputPerSec  int        `json:"throughput_per_second" db:"throughput_per_second"`
    CreatedAt         time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
    // Новые поля
    ClientID     *uuid.UUID      `json:"client_id,omitempty" db:"client_id"`
    Description  string          `json:"description" db:"description"`
    Tags         pq.StringArray  `json:"tags" db:"tags"`
    TPSLimit     int             `json:"tps_limit" db:"tps_limit"`
    RoutingRules json.RawMessage `json:"routing_rules" db:"routing_rules"`
}
```

В блок импортов `internal/shared/models.go` добавь если не хватает:
```go
import (
    "encoding/json"
    "github.com/lib/pq"
    // ... остальные импорты
)
```

- [ ] **Step 2: Проверить компиляцию**

```bash
cd /home/magomed/projects/sms && go build ./internal/shared/...
```

Ожидаемый результат: `go build` завершается без ошибок.

- [ ] **Step 3: Commit**

```bash
git add internal/shared/models.go
git commit -m "feat(shared): add client_id, routing_rules fields to Provider model"
```

---

## Task 3: Domain model — добавить ClientID и RoutingRule

**Files:**
- Modify: `internal/services/provider/domain/provider.go`

- [ ] **Step 1: Добавить поля и тип RoutingRule**

Открой `internal/services/provider/domain/provider.go`. Добавь тип `RoutingRule` и поля в `Provider`:

```go
// RoutingRule описывает правило маршрутизации через провайдера
type RoutingRule struct {
    Pattern  string `json:"pattern"`  // E.164 prefix ("+7") или regex
    Priority int    `json:"priority"` // Меньше = выше приоритет
}

type Provider struct {
    ID               uuid.UUID
    Name             string
    Host             string
    Port             int
    SystemID         string
    Password         string
    SystemType       string
    BindType         BindType
    BindTON          int
    BindNPI          int
    AddrTON          int
    AddrNPI          int
    AddressRange     string
    MaxConnections   int
    WindowSize       int
    Active           bool
    Priority         int
    ThroughputPerSec int
    CreatedAt        time.Time
    UpdatedAt        time.Time
    // Новые поля
    ClientID     *uuid.UUID
    Description  string
    Tags         []string
    TPSLimit     int
    RoutingRules []RoutingRule
}
```

- [ ] **Step 2: Добавить ошибки в domain/errors.go**

Открой `internal/services/provider/domain/errors.go` и добавь:

```go
var (
    ErrProviderLimitExceeded = errors.New("лимит провайдеров для тарифного плана исчерпан")
    ErrProviderNotOwnedByClient = errors.New("провайдер не принадлежит клиенту")
)
```

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./internal/services/provider/domain/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/provider/domain/
git commit -m "feat(provider): add ClientID, RoutingRule domain types and limit errors"
```

---

## Task 4: Repository — новые методы для client_id

**Files:**
- Modify: `internal/services/provider/domain/repository.go`
- Modify: `internal/storage/provider_repository.go`
- Modify: `internal/services/provider/infrastructure/repository/provider_repository.go`

- [ ] **Step 1: Обновить интерфейс репозитория**

В `internal/services/provider/domain/repository.go` добавь методы:

```go
type ProviderRepository interface {
    Create(ctx context.Context, provider *Provider) error
    Update(ctx context.Context, provider *Provider) error
    GetByID(ctx context.Context, id uuid.UUID) (*Provider, error)
    GetByName(ctx context.Context, name string) (*Provider, error)
    List(ctx context.Context, activeOnly bool, limit, offset int) ([]*Provider, int, error)
    Delete(ctx context.Context, id uuid.UUID) error
    // Новые методы для client-провайдеров:
    ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*Provider, error)
    CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error)
    GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*Provider, error)
}
```

- [ ] **Step 2: Добавить методы в storage-репозиторий**

В `internal/storage/provider_repository.go` добавь в конец файла:

```go
// ListByClientID возвращает провайдеров, принадлежащих клиенту
func (r *ProviderRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Provider, error) {
    var providers []*shared.Provider
    query := `SELECT * FROM providers WHERE client_id = $1 ORDER BY created_at DESC`
    err := r.db.SelectContext(ctx, &providers, query, clientID)
    if err != nil {
        return nil, err
    }
    return providers, nil
}

// CountByClientID считает провайдеров клиента
func (r *ProviderRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
    var count int
    query := `SELECT COUNT(*) FROM providers WHERE client_id = $1`
    err := r.db.GetContext(ctx, &count, query, clientID)
    return count, err
}

// GetByIDAndClientID получает провайдера по ID с проверкой принадлежности клиенту
func (r *ProviderRepository) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*shared.Provider, error) {
    var provider shared.Provider
    query := `SELECT * FROM providers WHERE id = $1 AND client_id = $2`
    err := r.db.GetContext(ctx, &provider, query, id, clientID)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &provider, nil
}
```

- [ ] **Step 3: Добавить методы в adapter**

В `internal/services/provider/infrastructure/repository/provider_repository.go` добавь:

```go
func (a *ProviderRepositoryAdapter) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
    providers, err := a.repo.ListByClientID(ctx, clientID)
    if err != nil {
        return nil, err
    }
    result := make([]*domain.Provider, len(providers))
    for i, p := range providers {
        result[i] = sharedToDomain(p)
    }
    return result, nil
}

func (a *ProviderRepositoryAdapter) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
    return a.repo.CountByClientID(ctx, clientID)
}

func (a *ProviderRepositoryAdapter) GetByIDAndClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
    p, err := a.repo.GetByIDAndClientID(ctx, id, clientID)
    if err != nil {
        if err == storage.ErrNotFound {
            return nil, domain.ErrProviderNotFound
        }
        return nil, err
    }
    return sharedToDomain(p), nil
}
```

Также обнови `sharedToDomain` и `domainToShared` в том же файле, добавив маппинг новых полей:

```go
// В sharedToDomain добавить:
ClientID:     p.ClientID,
Description:  p.Description,
Tags:         []string(p.Tags),
TPSLimit:     p.TPSLimit,
// RoutingRules: распарсить p.RoutingRules из JSON
// Добавь:
if len(p.RoutingRules) > 0 {
    _ = json.Unmarshal(p.RoutingRules, &d.RoutingRules)
}

// В domainToShared добавить:
ClientID:     p.ClientID,
Description:  p.Description,
Tags:         pq.StringArray(p.Tags),
TPSLimit:     p.TPSLimit,
// RoutingRules: сериализовать в JSON
rulesJSON, _ := json.Marshal(p.RoutingRules)
RoutingRules: json.RawMessage(rulesJSON),
```

- [ ] **Step 4: Проверить компиляцию**

```bash
go build ./internal/services/provider/... ./internal/storage/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/provider/domain/repository.go \
        internal/storage/provider_repository.go \
        internal/services/provider/infrastructure/repository/provider_repository.go
git commit -m "feat(provider): add client-scoped repository methods"
```

---

## Task 5: Application — TestConnection use case

**Files:**
- Create: `internal/services/provider/application/test_connection.go`
- Create: `internal/services/provider/application/test_connection_test.go`

- [ ] **Step 1: Написать тест**

```go
// internal/services/provider/application/test_connection_test.go
package application_test

import (
    "context"
    "net"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/services/provider/application"
)

func TestTestConnection_Timeout(t *testing.T) {
    svc := application.NewTestConnectionService()
    ctx := context.Background()

    // Адрес, который не отвечает — гарантированный таймаут
    result := svc.Test(ctx, application.TestConnectionConfig{
        Host:     "192.0.2.1", // TEST-NET, пакеты уходят в никуда
        Port:     2775,
        SystemID: "test",
        Password: "test",
        BindType: "transceiver",
        Timeout:  500 * time.Millisecond,
    })

    assert.False(t, result.Success)
    assert.Contains(t, result.Error, "timeout")
    assert.NotEmpty(t, result.Log)
}

func TestTestConnection_ConnectionRefused(t *testing.T) {
    // Запускаем сервер и сразу закрываем, чтобы получить "connection refused"
    l, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    port := l.Addr().(*net.TCPAddr).Port
    l.Close()

    svc := application.NewTestConnectionService()
    result := svc.Test(context.Background(), application.TestConnectionConfig{
        Host:     "127.0.0.1",
        Port:     port,
        SystemID: "test",
        Password: "test",
        BindType: "transceiver",
        Timeout:  2 * time.Second,
    })

    assert.False(t, result.Success)
    assert.Contains(t, result.Error, "refused")
}
```

- [ ] **Step 2: Запустить тест — убедиться что не компилируется**

```bash
go test ./internal/services/provider/application/... -run TestTestConnection -v
```

Ожидаемый результат: ошибка компиляции `cannot find package application`.

- [ ] **Step 3: Реализовать TestConnectionService**

```go
// internal/services/provider/application/test_connection.go
package application

import (
    "context"
    "encoding/binary"
    "fmt"
    "net"
    "time"
)

// TestConnectionConfig — параметры для теста SMPP-соединения
type TestConnectionConfig struct {
    Host     string
    Port     int
    SystemID string
    Password string
    BindType string        // "transceiver", "transmitter", "receiver"
    Timeout  time.Duration // 0 = default 10s
}

// TestConnectionResult — результат теста
type TestConnectionResult struct {
    Success   bool
    LatencyMs int64
    Log       []string
    Error     string
}

// TestConnectionService выполняет SMPP bind-тест
type TestConnectionService struct{}

func NewTestConnectionService() *TestConnectionService {
    return &TestConnectionService{}
}

func (s *TestConnectionService) Test(ctx context.Context, cfg TestConnectionConfig) TestConnectionResult {
    timeout := cfg.Timeout
    if timeout == 0 {
        timeout = 10 * time.Second
    }

    var logs []string
    start := time.Now()

    addLog := func(msg string) {
        logs = append(logs, msg)
    }

    // Шаг 1: TCP connect
    addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
    addLog(fmt.Sprintf("→ TCP connect %s ...", addr))

    dialCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    var d net.Dialer
    conn, err := d.DialContext(dialCtx, "tcp", addr)
    if err != nil {
        addLog(fmt.Sprintf("✗ TCP connect failed: %v", err))
        return TestConnectionResult{
            Success: false,
            Log:     logs,
            Error:   err.Error(),
        }
    }
    defer conn.Close()
    addLog("✓ TCP connect OK")

    // Шаг 2: SMPP bind
    bindCmd := bindCommandID(cfg.BindType)
    pdu := buildBindPDU(bindCmd, cfg.SystemID, cfg.Password)
    addLog(fmt.Sprintf("→ SMPP bind_%s (system_id=%s) ...", cfg.BindType, cfg.SystemID))

    conn.SetDeadline(time.Now().Add(timeout))
    if _, err := conn.Write(pdu); err != nil {
        addLog(fmt.Sprintf("✗ send bind failed: %v", err))
        return TestConnectionResult{Success: false, Log: logs, Error: err.Error()}
    }

    // Шаг 3: читаем ответ (минимум 16 байт — SMPP header)
    header := make([]byte, 16)
    if _, err := conn.Read(header); err != nil {
        addLog(fmt.Sprintf("✗ read bind_resp failed: %v", err))
        return TestConnectionResult{Success: false, Log: logs, Error: err.Error()}
    }

    cmdStatus := binary.BigEndian.Uint32(header[4:8])
    if cmdStatus != 0 {
        errMsg := fmt.Sprintf("SMPP bind_resp command_status=0x%08X", cmdStatus)
        if cmdStatus == 0x0000000D {
            errMsg += " (ESME_RBINDFAIL: неверный system_id или пароль)"
        }
        addLog(fmt.Sprintf("✗ %s", errMsg))
        return TestConnectionResult{Success: false, Log: logs, Error: errMsg}
    }
    addLog(fmt.Sprintf("✓ bind_%s_resp (command_status=0x00000000) OK", cfg.BindType))

    // Шаг 4: unbind
    addLog("→ unbind ...")
    unbind := buildUnbindPDU()
    conn.Write(unbind) // best-effort
    addLog("✓ unbind OK")

    latency := time.Since(start).Milliseconds()
    return TestConnectionResult{
        Success:   true,
        LatencyMs: latency,
        Log:       logs,
    }
}

// bindCommandID возвращает SMPP command_id для bind
func bindCommandID(bindType string) uint32 {
    switch bindType {
    case "transmitter":
        return 0x00000002
    case "receiver":
        return 0x00000001
    default: // transceiver
        return 0x00000009
    }
}

// buildBindPDU строит минимальный SMPP bind PDU
func buildBindPDU(commandID uint32, systemID, password string) []byte {
    // SMPP PDU: header (16 bytes) + body
    // Body: system_id\0 + password\0 + system_type\0 + interface_version + addr_ton + addr_npi + address_range\0
    body := []byte{}
    body = append(body, []byte(systemID)...)
    body = append(body, 0) // null terminator
    body = append(body, []byte(password)...)
    body = append(body, 0)
    body = append(body, 0)    // system_type (empty)
    body = append(body, 0x34) // interface_version: SMPP 3.4
    body = append(body, 0)    // addr_ton
    body = append(body, 0)    // addr_npi
    body = append(body, 0)    // address_range (empty)

    length := uint32(16 + len(body))
    pdu := make([]byte, 16)
    binary.BigEndian.PutUint32(pdu[0:4], length)
    binary.BigEndian.PutUint32(pdu[4:8], commandID)
    binary.BigEndian.PutUint32(pdu[8:12], 0)          // command_status = 0
    binary.BigEndian.PutUint32(pdu[12:16], 1)          // sequence_number = 1
    pdu = append(pdu, body...)
    return pdu
}

// buildUnbindPDU строит SMPP unbind PDU
func buildUnbindPDU() []byte {
    pdu := make([]byte, 16)
    binary.BigEndian.PutUint32(pdu[0:4], 16)           // length
    binary.BigEndian.PutUint32(pdu[4:8], 0x00000006)   // command_id = unbind
    binary.BigEndian.PutUint32(pdu[8:12], 0)
    binary.BigEndian.PutUint32(pdu[12:16], 2)
    return pdu
}
```

- [ ] **Step 4: Запустить тесты**

```bash
go test ./internal/services/provider/application/... -run TestTestConnection -v
```

Ожидаемый результат:
```
--- PASS: TestTestConnection_Timeout
--- PASS: TestTestConnection_ConnectionRefused
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/provider/application/test_connection.go \
        internal/services/provider/application/test_connection_test.go
git commit -m "feat(provider): add SMPP test connection service with bind/unbind"
```

---

## Task 6: Application — ClientProviderService (CRUD)

**Files:**
- Create: `internal/services/provider/application/client_provider_service.go`
- Create: `internal/services/provider/application/client_provider_service_test.go`

- [ ] **Step 1: Написать тесты**

```go
// internal/services/provider/application/client_provider_service_test.go
package application_test

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/services/provider/application"
    "github.com/smpp-server/smpp-server/internal/services/provider/domain"
    "github.com/smpp-server/smpp-server/internal/services/provider/mocks"
)

func TestClientProviderService_Create_LimitExceeded(t *testing.T) {
    repo := new(mocks.ProviderRepository)
    clientID := uuid.New()
    repo.On("CountByClientID", mock.Anything, clientID).Return(1, nil)

    svc := application.NewClientProviderService(repo, 1) // limit = 1
    _, err := svc.Create(context.Background(), clientID, &domain.Provider{
        Name: "test", Host: "localhost", Port: 2775,
        SystemID: "id", Password: "pw", BindType: domain.BindTypeTransceiver,
        MaxConnections: 1, WindowSize: 10,
    })

    assert.ErrorIs(t, err, domain.ErrProviderLimitExceeded)
}

func TestClientProviderService_Create_Success(t *testing.T) {
    repo := new(mocks.ProviderRepository)
    clientID := uuid.New()
    repo.On("CountByClientID", mock.Anything, clientID).Return(0, nil)
    repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Provider")).Return(nil)

    svc := application.NewClientProviderService(repo, 5)
    p, err := svc.Create(context.Background(), clientID, &domain.Provider{
        Name: "MyProvider", Host: "smpp.example.com", Port: 2775,
        SystemID: "sms_01", Password: "secret", BindType: domain.BindTypeTransceiver,
        MaxConnections: 1, WindowSize: 10,
    })

    require.NoError(t, err)
    assert.Equal(t, clientID, *p.ClientID)
    assert.NotEqual(t, uuid.Nil, p.ID)
    assert.True(t, p.Active)
    repo.AssertExpectations(t)
}

func TestClientProviderService_Delete_NotOwned(t *testing.T) {
    repo := new(mocks.ProviderRepository)
    providerID := uuid.New()
    clientID := uuid.New()
    repo.On("GetByIDAndClientID", mock.Anything, providerID, clientID).Return(nil, domain.ErrProviderNotFound)

    svc := application.NewClientProviderService(repo, 5)
    err := svc.Delete(context.Background(), providerID, clientID)

    assert.ErrorIs(t, err, domain.ErrProviderNotFound)
}
```

- [ ] **Step 2: Запустить тест — убедиться что не компилируется**

```bash
go test ./internal/services/provider/application/... -run TestClientProviderService -v 2>&1 | head -20
```

- [ ] **Step 3: Реализовать ClientProviderService**

```go
// internal/services/provider/application/client_provider_service.go
package application

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ClientProviderService управляет SMPP-провайдерами, принадлежащими клиентам
type ClientProviderService struct {
    repo          domain.ProviderRepository
    maxPerClient  int
}

func NewClientProviderService(repo domain.ProviderRepository, maxPerClient int) *ClientProviderService {
    return &ClientProviderService{repo: repo, maxPerClient: maxPerClient}
}

func (s *ClientProviderService) Create(ctx context.Context, clientID uuid.UUID, p *domain.Provider) (*domain.Provider, error) {
    // Проверка лимита провайдеров для клиента
    count, err := s.repo.CountByClientID(ctx, clientID)
    if err != nil {
        return nil, fmt.Errorf("проверка лимита провайдеров: %w", err)
    }
    if count >= s.maxPerClient {
        return nil, domain.ErrProviderLimitExceeded
    }

    if err := p.Validate(); err != nil {
        return nil, fmt.Errorf("валидация провайдера: %w", err)
    }

    now := time.Now()
    p.ID = uuid.New()
    p.ClientID = &clientID
    p.Active = true
    p.CreatedAt = now
    p.UpdatedAt = now

    if err := s.repo.Create(ctx, p); err != nil {
        return nil, fmt.Errorf("создание провайдера: %w", err)
    }
    return p, nil
}

func (s *ClientProviderService) List(ctx context.Context, clientID uuid.UUID) ([]*domain.Provider, error) {
    return s.repo.ListByClientID(ctx, clientID)
}

func (s *ClientProviderService) Get(ctx context.Context, id, clientID uuid.UUID) (*domain.Provider, error) {
    return s.repo.GetByIDAndClientID(ctx, id, clientID)
}

func (s *ClientProviderService) Update(ctx context.Context, id, clientID uuid.UUID, updates *domain.Provider) (*domain.Provider, error) {
    existing, err := s.repo.GetByIDAndClientID(ctx, id, clientID)
    if err != nil {
        return nil, err
    }

    if updates.Name != "" {
        existing.Name = updates.Name
    }
    if updates.Description != "" {
        existing.Description = updates.Description
    }
    if len(updates.Tags) > 0 {
        existing.Tags = updates.Tags
    }
    if updates.Host != "" {
        existing.Host = updates.Host
    }
    if updates.Port > 0 {
        existing.Port = updates.Port
    }
    if updates.SystemID != "" {
        existing.SystemID = updates.SystemID
    }
    if updates.Password != "" {
        existing.Password = updates.Password
    }
    if updates.BindType != "" {
        existing.BindType = updates.BindType
    }
    if updates.WindowSize > 0 {
        existing.WindowSize = updates.WindowSize
    }
    if updates.MaxConnections > 0 {
        existing.MaxConnections = updates.MaxConnections
    }
    if updates.TPSLimit > 0 {
        existing.TPSLimit = updates.TPSLimit
    }
    if len(updates.RoutingRules) > 0 {
        existing.RoutingRules = updates.RoutingRules
    }

    existing.UpdatedAt = time.Now()
    if err := s.repo.Update(ctx, existing); err != nil {
        return nil, fmt.Errorf("обновление провайдера: %w", err)
    }
    return existing, nil
}

func (s *ClientProviderService) Delete(ctx context.Context, id, clientID uuid.UUID) error {
    existing, err := s.repo.GetByIDAndClientID(ctx, id, clientID)
    if err != nil {
        return err
    }
    existing.Active = false
    existing.UpdatedAt = time.Now()
    return s.repo.Update(ctx, existing)
}
```

- [ ] **Step 4: Запустить тесты**

```bash
go test ./internal/services/provider/application/... -run TestClientProviderService -v
```

Ожидаемый результат: все 3 теста PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/provider/application/client_provider_service.go \
        internal/services/provider/application/client_provider_service_test.go
git commit -m "feat(provider): add ClientProviderService with plan limit enforcement"
```

---

## Task 7: Proto — новые RPC и messages

**Files:**
- Modify: `api/proto/provider/provider.proto`

- [ ] **Step 1: Добавить messages и rpc в provider.proto**

Открой `api/proto/provider/provider.proto` и добавь перед закрывающей скобкой `service ProviderService`:

```protobuf
// ---- Client-Provider RPC ----
rpc CreateClientProvider(CreateClientProviderRequest) returns (ClientProvider);
rpc ListClientProviders(ListClientProvidersRequest) returns (ListClientProvidersResponse);
rpc GetClientProvider(GetClientProviderRequest) returns (ClientProvider);
rpc UpdateClientProvider(UpdateClientProviderRequest) returns (ClientProvider);
rpc DeleteClientProvider(DeleteClientProviderRequest) returns (google.protobuf.Empty);
rpc TestClientProviderConnection(TestConnectionRequest) returns (TestConnectionResponse);
```

Добавь messages в конец файла (перед последней `}`):

```protobuf
message RoutingRule {
  string pattern  = 1;
  int32  priority = 2;
}

message ClientProvider {
  string id            = 1;
  string client_id     = 2;
  string name          = 3;
  string description   = 4;
  repeated string tags = 5;
  string host          = 6;
  int32  port          = 7;
  string system_id     = 8;
  int32  bind_type     = 9;
  int32  window_size   = 10;
  int32  max_connections = 11;
  int32  tps_limit     = 12;
  bool   active        = 13;
  repeated RoutingRule routing_rules = 14;
  string created_at    = 15;
  string updated_at    = 16;
}

message CreateClientProviderRequest {
  string client_id     = 1;
  string name          = 2;
  string description   = 3;
  repeated string tags = 4;
  string host          = 5;
  int32  port          = 6;
  string system_id     = 7;
  string password      = 8;
  int32  bind_type     = 9;  // 0=transceiver, 1=transmitter, 2=receiver
  int32  window_size   = 10;
  int32  max_connections = 11;
  int32  tps_limit     = 12;
  repeated RoutingRule routing_rules = 13;
}

message ListClientProvidersRequest  { string client_id = 1; }
message ListClientProvidersResponse { repeated ClientProvider providers = 1; }

message GetClientProviderRequest    { string id = 1; string client_id = 2; }

message UpdateClientProviderRequest {
  string id            = 1;
  string client_id     = 2;
  string name          = 3;
  string description   = 4;
  repeated string tags = 5;
  string host          = 6;
  int32  port          = 7;
  string system_id     = 8;
  string password      = 9;
  int32  bind_type     = 10;
  int32  window_size   = 11;
  int32  max_connections = 12;
  int32  tps_limit     = 13;
  repeated RoutingRule routing_rules = 14;
}

message DeleteClientProviderRequest { string id = 1; string client_id = 2; }

message TestConnectionRequest {
  string host      = 1;
  int32  port      = 2;
  string system_id = 3;
  string password  = 4;
  int32  bind_type = 5;
}

message TestConnectionResponse {
  bool            success    = 1;
  int64           latency_ms = 2;
  repeated string log        = 3;
  string          error      = 4;
}
```

- [ ] **Step 2: Сгенерировать Go-код из proto**

```bash
cd /home/magomed/projects/sms && bash scripts/generate-proto.sh
```

Ожидаемый результат: файлы `api/proto/provider/provider.pb.go` и `provider_grpc.pb.go` обновлены.

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./api/proto/provider/...
```

- [ ] **Step 4: Commit**

```bash
git add api/proto/provider/
git commit -m "feat(proto): add client-provider RPC methods and TestConnection messages"
```

---

## Task 8: gRPC Server — реализовать новые методы

**Files:**
- Modify: `internal/services/provider/grpc/server.go`

- [ ] **Step 1: Добавить ClientProviderService в Server**

Открой `internal/services/provider/grpc/server.go`. Добавь поле в структуру и обнови конструктор:

```go
type Server struct {
    providerv1.UnimplementedProviderServiceServer
    providerService       *application.ProviderService
    clientProviderService *application.ClientProviderService
    connectionPoolService application.ConnectionPoolService
    senderService         *application.SenderService
    testConnectionService *application.TestConnectionService
}

func NewServer(
    providerService *application.ProviderService,
    clientProviderService *application.ClientProviderService,
    connectionPoolService application.ConnectionPoolService,
    senderService *application.SenderService,
) *Server {
    return &Server{
        providerService:       providerService,
        clientProviderService: clientProviderService,
        connectionPoolService: connectionPoolService,
        senderService:         senderService,
        testConnectionService: application.NewTestConnectionService(),
    }
}
```

- [ ] **Step 2: Добавить хендлеры в конец server.go**

```go
func (s *Server) CreateClientProvider(ctx context.Context, req *providerv1.CreateClientProviderRequest) (*providerv1.ClientProvider, error) {
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }

    bindType := protoBindTypeToDomain(req.BindType)
    rules := protoRulesToDomain(req.RoutingRules)

    p, err := s.clientProviderService.Create(ctx, clientID, &domain.Provider{
        Name: req.Name, Description: req.Description, Tags: req.Tags,
        Host: req.Host, Port: int(req.Port),
        SystemID: req.SystemId, Password: req.Password,
        BindType: bindType, WindowSize: int(req.WindowSize),
        MaxConnections: int(req.MaxConnections), TPSLimit: int(req.TpsLimit),
        RoutingRules: rules,
    })
    if err != nil {
        if errors.Is(err, domain.ErrProviderLimitExceeded) {
            return nil, status.Error(codes.ResourceExhausted, err.Error())
        }
        return nil, status.Error(codes.Internal, err.Error())
    }
    return domainToClientProviderProto(p), nil
}

func (s *Server) ListClientProviders(ctx context.Context, req *providerv1.ListClientProvidersRequest) (*providerv1.ListClientProvidersResponse, error) {
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }
    providers, err := s.clientProviderService.List(ctx, clientID)
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }
    result := make([]*providerv1.ClientProvider, len(providers))
    for i, p := range providers {
        result[i] = domainToClientProviderProto(p)
    }
    return &providerv1.ListClientProvidersResponse{Providers: result}, nil
}

func (s *Server) GetClientProvider(ctx context.Context, req *providerv1.GetClientProviderRequest) (*providerv1.ClientProvider, error) {
    id, err := uuid.Parse(req.Id)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный id")
    }
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }
    p, err := s.clientProviderService.Get(ctx, id, clientID)
    if err != nil {
        if errors.Is(err, domain.ErrProviderNotFound) {
            return nil, status.Error(codes.NotFound, "провайдер не найден")
        }
        return nil, status.Error(codes.Internal, err.Error())
    }
    return domainToClientProviderProto(p), nil
}

func (s *Server) UpdateClientProvider(ctx context.Context, req *providerv1.UpdateClientProviderRequest) (*providerv1.ClientProvider, error) {
    id, err := uuid.Parse(req.Id)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный id")
    }
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }
    updates := &domain.Provider{
        Name: req.Name, Description: req.Description, Tags: req.Tags,
        Host: req.Host, Port: int(req.Port),
        SystemID: req.SystemId, Password: req.Password,
        BindType: protoBindTypeToDomain(req.BindType),
        WindowSize: int(req.WindowSize), MaxConnections: int(req.MaxConnections),
        TPSLimit: int(req.TpsLimit), RoutingRules: protoRulesToDomain(req.RoutingRules),
    }
    p, err := s.clientProviderService.Update(ctx, id, clientID, updates)
    if err != nil {
        if errors.Is(err, domain.ErrProviderNotFound) {
            return nil, status.Error(codes.NotFound, "провайдер не найден")
        }
        return nil, status.Error(codes.Internal, err.Error())
    }
    return domainToClientProviderProto(p), nil
}

func (s *Server) DeleteClientProvider(ctx context.Context, req *providerv1.DeleteClientProviderRequest) (*emptypb.Empty, error) {
    id, err := uuid.Parse(req.Id)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный id")
    }
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "неверный client_id")
    }
    if err := s.clientProviderService.Delete(ctx, id, clientID); err != nil {
        if errors.Is(err, domain.ErrProviderNotFound) {
            return nil, status.Error(codes.NotFound, "провайдер не найден")
        }
        return nil, status.Error(codes.Internal, err.Error())
    }
    return &emptypb.Empty{}, nil
}

func (s *Server) TestClientProviderConnection(ctx context.Context, req *providerv1.TestConnectionRequest) (*providerv1.TestConnectionResponse, error) {
    bindType := "transceiver"
    switch req.BindType {
    case 1:
        bindType = "transmitter"
    case 2:
        bindType = "receiver"
    }
    result := s.testConnectionService.Test(ctx, application.TestConnectionConfig{
        Host: req.Host, Port: int(req.Port),
        SystemID: req.SystemId, Password: req.Password,
        BindType: bindType,
    })
    return &providerv1.TestConnectionResponse{
        Success: result.Success, LatencyMs: result.LatencyMs,
        Log: result.Log, Error: result.Error,
    }, nil
}

// ---- helpers ----

func protoBindTypeToDomain(t int32) domain.BindType {
    switch t {
    case 1:
        return domain.BindTypeTransmitter
    case 2:
        return domain.BindTypeReceiver
    default:
        return domain.BindTypeTransceiver
    }
}

func protoRulesToDomain(rules []*providerv1.RoutingRule) []domain.RoutingRule {
    result := make([]domain.RoutingRule, len(rules))
    for i, r := range rules {
        result[i] = domain.RoutingRule{Pattern: r.Pattern, Priority: int(r.Priority)}
    }
    return result
}

func domainToClientProviderProto(p *domain.Provider) *providerv1.ClientProvider {
    cp := &providerv1.ClientProvider{
        Id: p.ID.String(), Name: p.Name, Description: p.Description,
        Tags: p.Tags, Host: p.Host, Port: int32(p.Port),
        SystemId: p.SystemID, WindowSize: int32(p.WindowSize),
        MaxConnections: int32(p.MaxConnections), TpsLimit: int32(p.TPSLimit),
        Active: p.Active,
        CreatedAt: p.CreatedAt.Format(time.RFC3339),
        UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
    }
    if p.ClientID != nil {
        cp.ClientId = p.ClientID.String()
    }
    cp.RoutingRules = make([]*providerv1.RoutingRule, len(p.RoutingRules))
    for i, r := range p.RoutingRules {
        cp.RoutingRules[i] = &providerv1.RoutingRule{Pattern: r.Pattern, Priority: int32(r.Priority)}
    }
    switch p.BindType {
    case domain.BindTypeTransmitter:
        cp.BindType = 1
    case domain.BindTypeReceiver:
        cp.BindType = 2
    default:
        cp.BindType = 0
    }
    return cp
}
```

Добавь в импорты server.go: `"google.golang.org/protobuf/types/known/emptypb"`.

- [ ] **Step 3: Обновить инициализацию Server в cmd/services/provider-service/main.go**

Найди вызов `grpc.NewServer(...)` и обнови сигнатуру — добавь `clientProviderService`:

```go
clientProviderService := application.NewClientProviderService(
    providerRepo,
    getProviderLimit(cfg), // функция, читающая лимит из конфига, default=5
)
grpcServer := providergrpc.NewServer(providerService, clientProviderService, poolService, senderService)
```

Добавь хелпер:
```go
func getProviderLimit(cfg *Config) int {
    if cfg.MaxProvidersPerClient > 0 {
        return cfg.MaxProvidersPerClient
    }
    return 5
}
```

- [ ] **Step 4: Компиляция**

```bash
go build ./internal/services/provider/... ./cmd/services/provider-service/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/provider/grpc/server.go \
        cmd/services/provider-service/
git commit -m "feat(provider-grpc): implement client-provider and test-connection handlers"
```

---

## Task 9: Portal Gateway — добавить ProviderClient

**Files:**
- Modify: `internal/gateway/portal/clients.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Добавить ProviderClient в ServiceClients**

В `internal/gateway/portal/clients.go`:

```go
import (
    // ... existing imports
    providerv1 "github.com/smpp-server/smpp-server/api/proto/provider"
)

type ServiceClients struct {
    AuthClient      authv1.AuthServiceClient
    ClientClient    clientv1.ClientServiceClient
    BillingClient   billingv1.BillingServiceClient
    MessagingClient messagingv1.MessagingServiceClient
    AnalyticsClient analyticsv1.AnalyticsServiceClient
    WebhookClient   webhookv1.WebhookServiceClient
    AuditClient     auditv1.AuditServiceClient
    RoutingClient   routingv1.RoutingServiceClient
    ProviderClient  providerv1.ProviderServiceClient  // новое поле
    conns           []*grpc.ClientConn
}

type ServiceAddresses struct {
    Auth      string
    Client    string
    Billing   string
    Messaging string
    Analytics string
    Webhook   string
    Audit     string
    Routing   string
    Provider  string  // новое поле
}
```

В функции `NewServiceClients`, после блока с `RoutingClient`, добавь:

```go
if addresses.Provider != "" {
    conn, err := grpc.Dial(addresses.Provider, opts...)
    if err != nil {
        return nil, fmt.Errorf("не удалось подключиться к Provider Service: %w", err)
    }
    clients.ProviderClient = providerv1.NewProviderServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}
```

- [ ] **Step 2: Добавить PROVIDER_SERVICE_ADDR в main.go**

В `cmd/portal-gateway/main.go` в блоке `ServiceAddresses{}` добавь:

```go
serviceAddresses := portal.ServiceAddresses{
    // ... existing fields
    Provider: getEnvOrDefault("PROVIDER_SERVICE_ADDR", "localhost:9096"),
}
```

- [ ] **Step 3: Компиляция**

```bash
go build ./internal/gateway/portal/... ./cmd/portal-gateway/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/clients.go cmd/portal-gateway/main.go
git commit -m "feat(portal-gateway): add ProviderServiceClient to gateway"
```

---

## Task 10: Portal Gateway — HTTP хендлер /providers

**Files:**
- Create: `internal/gateway/portal/handlers/providers.go`
- Create: `internal/gateway/portal/handlers/providers_test.go`

- [ ] **Step 1: Написать тест**

```go
// internal/gateway/portal/handlers/providers_test.go
package handlers_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/handlers"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// mockProviderClient реализует минимальный провайдер для теста
// (используем httptest + моковый grpc клиент)
func TestProviderHandlers_TestConnection_MissingFields(t *testing.T) {
    h := handlers.NewProviderHandlers(nil) // nil grpc client — до вызова не доходим

    body := `{"host": "", "port": 0}`
    req := httptest.NewRequest(http.MethodPost, "/portal/v1/providers/test", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    // Добавить client_id в context (как это делает middleware)
    ctx := middleware.WithClientID(req.Context(), "00000000-0000-0000-0000-000000000001")
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()

    h.TestConnection(w, req)

    require.Equal(t, http.StatusBadRequest, w.Code)
    var resp map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    require.NoError(t, err)
    assert.Contains(t, resp, "error")
}
```

- [ ] **Step 2: Запустить тест — убедиться что не компилируется**

```bash
go test ./internal/gateway/portal/handlers/... -run TestProviderHandlers -v 2>&1 | head -20
```

- [ ] **Step 3: Реализовать хендлер**

```go
// internal/gateway/portal/handlers/providers.go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/google/uuid"
    "github.com/gorilla/mux"
    providerv1 "github.com/smpp-server/smpp-server/api/proto/provider"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
    "github.com/smpp-server/smpp-server/internal/gateway/portal/shared"
)

type ProviderHandlers struct {
    providerClient providerv1.ProviderServiceClient
}

func NewProviderHandlers(providerClient providerv1.ProviderServiceClient) *ProviderHandlers {
    return &ProviderHandlers{providerClient: providerClient}
}

func (h *ProviderHandlers) ListProviders(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("клиент не найден"))
        return
    }
    resp, err := h.providerClient.ListClientProviders(r.Context(), &providerv1.ListClientProvidersRequest{
        ClientId: clientID.String(),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    respondJSON(w, http.StatusOK, resp.Providers)
}

func (h *ProviderHandlers) CreateProvider(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("клиент не найден"))
        return
    }
    var req providerv1.CreateClientProviderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
        return
    }
    if req.Name == "" || req.Host == "" || req.Port == 0 || req.SystemId == "" || req.Password == "" {
        respondError(w, shared.ErrInvalidInput("поля name, host, port, system_id, password обязательны"))
        return
    }
    req.ClientId = clientID.String()
    provider, err := h.providerClient.CreateClientProvider(r.Context(), &req)
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    respondJSON(w, http.StatusCreated, provider)
}

func (h *ProviderHandlers) GetProvider(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("клиент не найден"))
        return
    }
    id := mux.Vars(r)["id"]
    if _, err := uuid.Parse(id); err != nil {
        respondError(w, shared.ErrInvalidInput("неверный id"))
        return
    }
    provider, err := h.providerClient.GetClientProvider(r.Context(), &providerv1.GetClientProviderRequest{
        Id: id, ClientId: clientID.String(),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    respondJSON(w, http.StatusOK, provider)
}

func (h *ProviderHandlers) UpdateProvider(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("клиент не найден"))
        return
    }
    id := mux.Vars(r)["id"]
    var req providerv1.UpdateClientProviderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
        return
    }
    req.Id = id
    req.ClientId = clientID.String()
    provider, err := h.providerClient.UpdateClientProvider(r.Context(), &req)
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    respondJSON(w, http.StatusOK, provider)
}

func (h *ProviderHandlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
    clientID, ok := middleware.GetClientID(r.Context())
    if !ok {
        respondError(w, shared.ErrUnauthorized("клиент не найден"))
        return
    }
    id := mux.Vars(r)["id"]
    if _, err := h.providerClient.DeleteClientProvider(r.Context(), &providerv1.DeleteClientProviderRequest{
        Id: id, ClientId: clientID.String(),
    }); err != nil {
        respondGRPCError(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *ProviderHandlers) TestConnection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Host     string `json:"host"`
        Port     int    `json:"port"`
        SystemID string `json:"system_id"`
        Password string `json:"password"`
        BindType int    `json:"bind_type"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, shared.ErrInvalidInput("неверный формат запроса"))
        return
    }
    if req.Host == "" || req.Port == 0 || req.SystemID == "" || req.Password == "" {
        respondError(w, shared.ErrInvalidInput("поля host, port, system_id, password обязательны"))
        return
    }
    resp, err := h.providerClient.TestClientProviderConnection(r.Context(), &providerv1.TestConnectionRequest{
        Host: req.Host, Port: int32(req.Port),
        SystemId: req.SystemID, Password: req.Password,
        BindType: int32(req.BindType),
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":    resp.Success,
        "latency_ms": resp.LatencyMs,
        "log":        resp.Log,
        "error":      resp.Error,
    })
}
```

- [ ] **Step 4: Запустить тест**

```bash
go test ./internal/gateway/portal/handlers/... -run TestProviderHandlers -v
```

Ожидаемый результат: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/providers.go \
        internal/gateway/portal/handlers/providers_test.go
git commit -m "feat(portal-gateway): add provider HTTP handlers with client isolation"
```

---

## Task 11: Portal Gateway — зарегистрировать роуты

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Добавить ProviderHandlers в SetupRouter**

В сигнатуре `SetupRouter` добавь параметр:

```go
func SetupRouter(
    // ... existing params
    plansHandlers *handlers.PlansHandlers,
    providerHandlers *handlers.ProviderHandlers,  // новое
) *mux.Router {
```

В блоке защищённых маршрутов добавь (после webhooks):

```go
// Providers endpoints
providers := protected.PathPrefix("/providers").Subrouter()
providers.HandleFunc("", providerHandlers.ListProviders).Methods("GET")
providers.HandleFunc("", providerHandlers.CreateProvider).Methods("POST")
providers.HandleFunc("/test", providerHandlers.TestConnection).Methods("POST")
providers.HandleFunc("/{id}", providerHandlers.GetProvider).Methods("GET")
providers.HandleFunc("/{id}", providerHandlers.UpdateProvider).Methods("PUT")
providers.HandleFunc("/{id}", providerHandlers.DeleteProvider).Methods("DELETE")
```

- [ ] **Step 2: Обновить вызов SetupRouter в main.go portal-gateway**

Найди вызов `router.SetupRouter(...)` в `cmd/portal-gateway/main.go` и добавь:

```go
providerHandlers := handlers.NewProviderHandlers(serviceClients.ProviderClient)

r := router.SetupRouter(
    // ... existing args
    plansHandlers,
    providerHandlers,  // новое
)
```

- [ ] **Step 3: Компиляция**

```bash
go build ./cmd/portal-gateway/...
```

Ожидаемый результат: без ошибок.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(portal-gateway): register /providers routes"
```

---

## Task 12: Frontend — providersApi в client.ts

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Добавить типы и providersApi**

Открой `portal-frontend/src/api/client.ts`. Добавь типы перед экспортами:

```typescript
export interface RoutingRule {
  pattern: string;
  priority: number;
}

export interface ClientProvider {
  id: string;
  client_id: string;
  name: string;
  description: string;
  tags: string[];
  host: string;
  port: number;
  system_id: string;
  bind_type: number; // 0=transceiver, 1=transmitter, 2=receiver
  window_size: number;
  max_connections: number;
  tps_limit: number;
  active: boolean;
  routing_rules: RoutingRule[];
  created_at: string;
  updated_at: string;
}

export interface TestConnectionResult {
  success: boolean;
  latency_ms: number;
  log: string[];
  error?: string;
}

export interface CreateProviderPayload {
  name: string;
  description?: string;
  tags?: string[];
  host: string;
  port: number;
  system_id: string;
  password: string;
  bind_type: number;
  window_size: number;
  max_connections: number;
  tps_limit: number;
  routing_rules?: RoutingRule[];
}
```

Добавь `providersApi` в конец файла:

```typescript
export const providersApi = {
  async list(): Promise<ClientProvider[]> {
    const res = await fetch(`${BASE_URL}/providers`, { credentials: 'include' });
    if (!res.ok) throw await res.json();
    return res.json();
  },

  async create(data: CreateProviderPayload): Promise<ClientProvider> {
    const res = await fetch(`${BASE_URL}/providers`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(data),
    });
    if (!res.ok) throw await res.json();
    return res.json();
  },

  async get(id: string): Promise<ClientProvider> {
    const res = await fetch(`${BASE_URL}/providers/${id}`, { credentials: 'include' });
    if (!res.ok) throw await res.json();
    return res.json();
  },

  async update(id: string, data: Partial<CreateProviderPayload>): Promise<ClientProvider> {
    const res = await fetch(`${BASE_URL}/providers/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(data),
    });
    if (!res.ok) throw await res.json();
    return res.json();
  },

  async delete(id: string): Promise<void> {
    const res = await fetch(`${BASE_URL}/providers/${id}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    if (!res.ok) throw await res.json();
  },

  async test(config: {
    host: string; port: number; system_id: string;
    password: string; bind_type: number;
  }): Promise<TestConnectionResult> {
    const res = await fetch(`${BASE_URL}/providers/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(config),
    });
    if (!res.ok) throw await res.json();
    return res.json();
  },
};
```

Где `BASE_URL` — это константа `/portal/v1`, уже используемая в файле.

- [ ] **Step 2: Проверить сборку frontend**

```bash
cd portal-frontend && npx tsc --noEmit
```

Ожидаемый результат: без ошибок.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(frontend): add providersApi with types for SMPP wizard"
```

---

## Task 13: Frontend — WizardProgress и WizardNav

**Files:**
- Create: `portal-frontend/src/pages/providers/components/WizardProgress.tsx`
- Create: `portal-frontend/src/pages/providers/components/WizardNav.tsx`

- [ ] **Step 1: Создать WizardProgress**

```tsx
// portal-frontend/src/pages/providers/components/WizardProgress.tsx
import React from 'react';

const STEPS = [
  { label: 'Основное' },
  { label: 'Подключение' },
  { label: 'Параметры' },
  { label: 'Тест' },
  { label: 'Маршруты' },
  { label: 'Итог' },
];

interface WizardProgressProps {
  currentStep: number; // 1-based
}

export function WizardProgress({ currentStep }: WizardProgressProps) {
  return (
    <div className="flex items-center gap-0 w-full">
      {STEPS.map((step, idx) => {
        const stepNum = idx + 1;
        const isDone = stepNum < currentStep;
        const isActive = stepNum === currentStep;
        return (
          <React.Fragment key={stepNum}>
            <div className="flex flex-col items-center gap-1 shrink-0">
              <div
                className={[
                  'w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold',
                  isDone ? 'bg-green-600 text-white' :
                  isActive ? 'bg-blue-500 text-white' :
                  'bg-slate-700 text-slate-400',
                ].join(' ')}
              >
                {isDone ? '✓' : stepNum}
              </div>
              <span className={[
                'text-xs whitespace-nowrap',
                isActive ? 'text-blue-300' : isDone ? 'text-green-400' : 'text-slate-500',
              ].join(' ')}>
                {step.label}
              </span>
            </div>
            {idx < STEPS.length - 1 && (
              <div className={[
                'flex-1 h-0.5 mb-5',
                isDone ? 'bg-green-600' : isActive ? 'bg-blue-500' : 'bg-slate-700',
              ].join(' ')} />
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Создать WizardNav**

```tsx
// portal-frontend/src/pages/providers/components/WizardNav.tsx
interface WizardNavProps {
  currentStep: number;
  totalSteps: number;
  onBack: () => void;
  onNext: () => void;
  nextLabel?: string;
  nextDisabled?: boolean;
  loading?: boolean;
}

export function WizardNav({
  currentStep, totalSteps, onBack, onNext,
  nextLabel, nextDisabled, loading,
}: WizardNavProps) {
  const isLast = currentStep === totalSteps;
  return (
    <div className="flex items-center gap-3 mt-6">
      {currentStep > 1 && (
        <button
          onClick={onBack}
          disabled={loading}
          className="px-4 py-2 rounded bg-slate-800 border border-slate-600 text-slate-300 hover:bg-slate-700 disabled:opacity-50"
        >
          ← Назад
        </button>
      )}
      <button
        onClick={onNext}
        disabled={nextDisabled || loading}
        className="px-5 py-2 rounded bg-blue-600 text-white font-medium hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed"
      >
        {loading ? 'Загрузка...' : nextLabel ?? (isLast ? 'Активировать' : 'Далее →')}
      </button>
    </div>
  );
}
```

- [ ] **Step 3: Проверить сборку**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/providers/components/
git commit -m "feat(wizard): add WizardProgress and WizardNav components"
```

---

## Task 14: Frontend — Step1, Step2, Step3

**Files:**
- Create: `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx`
- Create: `portal-frontend/src/pages/providers/steps/Step2Connection.tsx`
- Create: `portal-frontend/src/pages/providers/steps/Step3Params.tsx`

- [ ] **Step 1: Создать Step1BasicInfo**

```tsx
// portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx
import { WizardNav } from '../components/WizardNav';

interface Step1Data { name: string; description: string; tags: string[] }
interface Props { data: Step1Data; onChange: (d: Step1Data) => void; onNext: () => void }

export function Step1BasicInfo({ data, onChange, onNext }: Props) {
  const valid = data.name.trim().length > 0;
  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 1 — Основная информация</h3>
      <p className="text-slate-400 text-sm mb-5">Укажи имя и описание провайдера</p>
      <div className="space-y-4 max-w-lg">
        <div>
          <label className="block text-sm text-slate-400 mb-1">Название *</label>
          <input
            value={data.name}
            onChange={e => onChange({ ...data, name: e.target.value })}
            placeholder="Мой SMPP провайдер"
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none"
          />
        </div>
        <div>
          <label className="block text-sm text-slate-400 mb-1">Описание</label>
          <textarea
            value={data.description}
            onChange={e => onChange({ ...data, description: e.target.value })}
            rows={3}
            placeholder="Например: основной провайдер для РФ-трафика"
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none resize-none"
          />
        </div>
        <div>
          <label className="block text-sm text-slate-400 mb-1">Теги (через запятую)</label>
          <input
            value={data.tags.join(', ')}
            onChange={e => onChange({ ...data, tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })}
            placeholder="россия, мобильные"
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none"
          />
        </div>
      </div>
      <WizardNav currentStep={1} totalSteps={6} onBack={() => {}} onNext={onNext} nextDisabled={!valid} />
    </div>
  );
}
```

- [ ] **Step 2: Создать Step2Connection**

```tsx
// portal-frontend/src/pages/providers/steps/Step2Connection.tsx
import { WizardNav } from '../components/WizardNav';

interface Step2Data { host: string; port: number; systemId: string; password: string; bindType: number }
interface Props { data: Step2Data; onChange: (d: Step2Data) => void; onBack: () => void; onNext: () => void }

const BIND_TYPES = [
  { value: 0, label: 'Transceiver (TX+RX)' },
  { value: 1, label: 'Transmitter (только отправка)' },
  { value: 2, label: 'Receiver (только приём)' },
];

export function Step2Connection({ data, onChange, onBack, onNext }: Props) {
  const valid = data.host && data.port > 0 && data.systemId && data.password;
  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 2 — Параметры подключения</h3>
      <p className="text-slate-400 text-sm mb-5">SMPP-реквизиты провайдера</p>
      <div className="space-y-4 max-w-lg">
        <div className="flex gap-3">
          <div className="flex-1">
            <label className="block text-sm text-slate-400 mb-1">Host *</label>
            <input value={data.host} onChange={e => onChange({ ...data, host: e.target.value })}
              placeholder="smpp.example.com"
              className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
          </div>
          <div className="w-28">
            <label className="block text-sm text-slate-400 mb-1">Port *</label>
            <input type="number" value={data.port || ''} onChange={e => onChange({ ...data, port: Number(e.target.value) })}
              placeholder="2775"
              className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
          </div>
        </div>
        <div>
          <label className="block text-sm text-slate-400 mb-1">System ID *</label>
          <input value={data.systemId} onChange={e => onChange({ ...data, systemId: e.target.value })}
            placeholder="sms_client_01"
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
        </div>
        <div>
          <label className="block text-sm text-slate-400 mb-1">Password *</label>
          <input type="password" value={data.password} onChange={e => onChange({ ...data, password: e.target.value })}
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
        </div>
        <div>
          <label className="block text-sm text-slate-400 mb-1">Bind Type</label>
          <select value={data.bindType} onChange={e => onChange({ ...data, bindType: Number(e.target.value) })}
            className="w-full px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none">
            {BIND_TYPES.map(bt => <option key={bt.value} value={bt.value}>{bt.label}</option>)}
          </select>
        </div>
      </div>
      <WizardNav currentStep={2} totalSteps={6} onBack={onBack} onNext={onNext} nextDisabled={!valid} />
    </div>
  );
}
```

- [ ] **Step 3: Создать Step3Params**

```tsx
// portal-frontend/src/pages/providers/steps/Step3Params.tsx
import { WizardNav } from '../components/WizardNav';

interface Step3Data { windowSize: number; maxConnections: number; tpsLimit: number }
interface Props { data: Step3Data; onChange: (d: Step3Data) => void; onBack: () => void; onNext: () => void }

function NumberInput({ label, hint, value, min, max, onChange }: {
  label: string; hint: string; value: number; min: number; max: number;
  onChange: (v: number) => void;
}) {
  return (
    <div>
      <label className="block text-sm text-slate-400 mb-1">{label}</label>
      <input type="number" value={value} min={min} max={max}
        onChange={e => onChange(Math.min(max, Math.max(min, Number(e.target.value))))}
        className="w-32 px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
      <span className="ml-2 text-xs text-slate-500">{hint}</span>
    </div>
  );
}

export function Step3Params({ data, onChange, onBack, onNext }: Props) {
  const valid = data.windowSize >= 1 && data.maxConnections >= 1 && data.tpsLimit >= 1;
  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 3 — Параметры соединения</h3>
      <p className="text-slate-400 text-sm mb-5">Настройки производительности</p>
      <div className="space-y-5 max-w-lg">
        <NumberInput label="Window Size" hint="1–1000 PDU (default: 10)"
          value={data.windowSize} min={1} max={1000} onChange={v => onChange({ ...data, windowSize: v })} />
        <NumberInput label="Max Connections" hint="1–10 параллельных SMPP-сессий"
          value={data.maxConnections} min={1} max={10} onChange={v => onChange({ ...data, maxConnections: v })} />
        <NumberInput label="TPS Limit" hint="1–1000 сообщений/сек"
          value={data.tpsLimit} min={1} max={1000} onChange={v => onChange({ ...data, tpsLimit: v })} />
      </div>
      <WizardNav currentStep={3} totalSteps={6} onBack={onBack} onNext={onNext} nextDisabled={!valid} />
    </div>
  );
}
```

- [ ] **Step 4: Проверить сборку**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx \
        portal-frontend/src/pages/providers/steps/Step2Connection.tsx \
        portal-frontend/src/pages/providers/steps/Step3Params.tsx
git commit -m "feat(wizard): add Step1 (basic info), Step2 (connection), Step3 (params)"
```

---

## Task 15: Frontend — Step4 (Test Connection)

**Files:**
- Create: `portal-frontend/src/pages/providers/steps/Step4Test.tsx`

- [ ] **Step 1: Создать Step4Test**

```tsx
// portal-frontend/src/pages/providers/steps/Step4Test.tsx
import { useState } from 'react';
import { providersApi, TestConnectionResult } from '../../../api/client';
import { WizardNav } from '../components/WizardNav';

interface Step2Data { host: string; port: number; systemId: string; password: string; bindType: number }
interface Props { connection: Step2Data; onBack: () => void; onNext: () => void }

export function Step4Test({ connection, onBack, onNext }: Props) {
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<TestConnectionResult | null>(null);

  const runTest = async () => {
    setTesting(true);
    setResult(null);
    try {
      const r = await providersApi.test({
        host: connection.host,
        port: connection.port,
        system_id: connection.systemId,
        password: connection.password,
        bind_type: connection.bindType,
      });
      setResult(r);
    } catch (err: any) {
      setResult({ success: false, latency_ms: 0, log: [], error: err?.error?.message ?? 'Ошибка запроса' });
    } finally {
      setTesting(false);
    }
  };

  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 4 — Тест соединения</h3>
      <p className="text-slate-400 text-sm mb-5">Проверим SMPP bind до провайдера</p>

      {/* Connection summary */}
      <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 mb-4 flex gap-6 flex-wrap">
        <div><span className="text-slate-500 text-xs">Host</span><br /><span className="text-slate-200 text-sm font-mono">{connection.host}:{connection.port}</span></div>
        <div><span className="text-slate-500 text-xs">System ID</span><br /><span className="text-slate-200 text-sm font-mono">{connection.systemId}</span></div>
        <div><span className="text-slate-500 text-xs">Bind</span><br /><span className="text-slate-200 text-sm">{['TRX', 'TX', 'RX'][connection.bindType]}</span></div>
      </div>

      {/* Test result */}
      {result && (
        <div className={`border rounded-lg p-4 mb-4 ${result.success ? 'bg-green-950 border-green-700' : 'bg-red-950 border-red-700'}`}>
          <div className="flex items-center gap-2 mb-2">
            <span className={result.success ? 'text-green-400 text-lg' : 'text-red-400 text-lg'}>
              {result.success ? '✓' : '✗'}
            </span>
            <span className={`font-semibold ${result.success ? 'text-green-300' : 'text-red-300'}`}>
              {result.success ? 'Соединение установлено' : 'Ошибка соединения'}
            </span>
            {result.success && (
              <span className="ml-auto text-green-600 text-sm">Латентность: {result.latency_ms}ms</span>
            )}
          </div>
          {result.log.length > 0 && (
            <div className="font-mono text-xs leading-relaxed">
              {result.log.map((line, i) => (
                <div key={i} className={result.success ? 'text-green-300' : 'text-red-300'}>{line}</div>
              ))}
            </div>
          )}
          {result.error && !result.success && (
            <p className="text-red-400 text-sm mt-1">{result.error}</p>
          )}
        </div>
      )}

      {/* Warning if test failed but user wants to proceed */}
      {result && !result.success && (
        <div className="bg-yellow-950 border border-yellow-700 rounded p-3 mb-4 text-yellow-300 text-sm">
          Тест не пройден — провайдер будет создан со статусом <strong>inactive</strong>. Трафик не будет отправляться до исправления настроек.
        </div>
      )}

      <div className="flex items-center gap-3">
        <button
          onClick={runTest}
          disabled={testing}
          className="px-4 py-2 rounded bg-slate-700 border border-slate-500 text-slate-200 hover:bg-slate-600 disabled:opacity-50"
        >
          {testing ? 'Проверка...' : result ? 'Тест снова' : 'Проверить соединение'}
        </button>
      </div>

      <WizardNav
        currentStep={4} totalSteps={6}
        onBack={onBack}
        onNext={onNext}
        nextDisabled={!result}
      />
    </div>
  );
}
```

- [ ] **Step 2: Проверить сборку**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/providers/steps/Step4Test.tsx
git commit -m "feat(wizard): add Step4 live SMPP connection test with log output"
```

---

## Task 16: Frontend — Step5 (Routing) и Step6 (Summary)

**Files:**
- Create: `portal-frontend/src/pages/providers/steps/Step5Routing.tsx`
- Create: `portal-frontend/src/pages/providers/steps/Step6Summary.tsx`

- [ ] **Step 1: Создать Step5Routing**

```tsx
// portal-frontend/src/pages/providers/steps/Step5Routing.tsx
import { useState } from 'react';
import { RoutingRule } from '../../../api/client';
import { WizardNav } from '../components/WizardNav';

interface Props { rules: RoutingRule[]; onChange: (r: RoutingRule[]) => void; onBack: () => void; onNext: () => void }

export function Step5Routing({ rules, onChange, onBack, onNext }: Props) {
  const [pattern, setPattern] = useState('');
  const [priority, setPriority] = useState(10);

  const addRule = () => {
    if (!pattern.trim()) return;
    onChange([...rules, { pattern: pattern.trim(), priority }]);
    setPattern('');
    setPriority(10);
  };

  const removeRule = (idx: number) => {
    onChange(rules.filter((_, i) => i !== idx));
  };

  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 5 — Маршруты</h3>
      <p className="text-slate-400 text-sm mb-2">Укажи, какие номера направлять через этого провайдера</p>
      <p className="text-slate-500 text-xs mb-5">Pattern: E.164 префикс (+7, +44) или regex (^\\+7[0-9]{{'{'}10{'}'}})$). Меньший priority = выше приоритет.</p>

      {/* Add rule */}
      <div className="flex gap-2 mb-4 max-w-lg">
        <input value={pattern} onChange={e => setPattern(e.target.value)}
          placeholder="+7"
          className="flex-1 px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 focus:border-blue-500 outline-none" />
        <input type="number" value={priority} onChange={e => setPriority(Number(e.target.value))}
          className="w-20 px-3 py-2 rounded bg-slate-800 border border-slate-600 text-slate-100 outline-none"
          placeholder="10" />
        <button onClick={addRule}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-500">
          + Добавить
        </button>
      </div>

      {/* Rules list */}
      {rules.length > 0 ? (
        <div className="space-y-2 mb-4 max-w-lg">
          {rules.map((r, i) => (
            <div key={i} className="flex items-center justify-between bg-slate-800 rounded px-3 py-2">
              <span className="font-mono text-slate-200 text-sm">{r.pattern}</span>
              <span className="text-slate-400 text-sm mx-3">priority: {r.priority}</span>
              <button onClick={() => removeRule(i)} className="text-red-400 hover:text-red-300 text-sm">✕</button>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-slate-500 text-sm mb-4">
          Нет правил — провайдер будет создан со статусом <strong className="text-yellow-400">inactive</strong>
        </div>
      )}

      <WizardNav currentStep={5} totalSteps={6} onBack={onBack} onNext={onNext} />
    </div>
  );
}
```

- [ ] **Step 2: Создать Step6Summary**

```tsx
// portal-frontend/src/pages/providers/steps/Step6Summary.tsx
import { WizardNav } from '../components/WizardNav';
import { CreateProviderPayload } from '../../../api/client';

const BIND_LABELS = ['Transceiver (TRX)', 'Transmitter (TX)', 'Receiver (RX)'];

interface Props {
  data: CreateProviderPayload;
  loading: boolean;
  onBack: () => void;
  onActivate: () => void;
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start gap-4 py-2 border-b border-slate-800">
      <span className="text-slate-500 text-sm w-40 shrink-0">{label}</span>
      <span className="text-slate-200 text-sm">{value}</span>
    </div>
  );
}

export function Step6Summary({ data, loading, onBack, onActivate }: Props) {
  return (
    <div>
      <h3 className="text-lg font-semibold text-slate-100 mb-1">Шаг 6 — Итог</h3>
      <p className="text-slate-400 text-sm mb-5">Проверь настройки перед активацией</p>
      <div className="max-w-lg bg-slate-900 rounded-lg px-4 py-2 mb-6">
        <Row label="Название" value={data.name} />
        {data.description && <Row label="Описание" value={data.description} />}
        {data.tags && data.tags.length > 0 && <Row label="Теги" value={data.tags.join(', ')} />}
        <Row label="Host" value={`${data.host}:${data.port}`} />
        <Row label="System ID" value={data.system_id} />
        <Row label="Bind Type" value={BIND_LABELS[data.bind_type] ?? 'Transceiver'} />
        <Row label="Window Size" value={data.window_size} />
        <Row label="Max Connections" value={data.max_connections} />
        <Row label="TPS Limit" value={`${data.tps_limit} msg/s`} />
        <Row label="Routing Rules" value={
          data.routing_rules && data.routing_rules.length > 0
            ? data.routing_rules.map(r => `${r.pattern} (p${r.priority})`).join(', ')
            : <span className="text-yellow-400">Нет — провайдер будет inactive</span>
        } />
      </div>
      <WizardNav
        currentStep={6} totalSteps={6}
        onBack={onBack}
        onNext={onActivate}
        nextLabel="Активировать"
        loading={loading}
      />
    </div>
  );
}
```

- [ ] **Step 3: Проверить сборку**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/providers/steps/Step5Routing.tsx \
        portal-frontend/src/pages/providers/steps/Step6Summary.tsx
git commit -m "feat(wizard): add Step5 (routing rules) and Step6 (summary + activate)"
```

---

## Task 17: Frontend — ProviderWizardPage (контейнер)

**Files:**
- Create: `portal-frontend/src/pages/providers/ProviderWizardPage.tsx`

- [ ] **Step 1: Создать ProviderWizardPage**

```tsx
// portal-frontend/src/pages/providers/ProviderWizardPage.tsx
import { useReducer, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { providersApi, CreateProviderPayload } from '../../api/client';
import { WizardProgress } from './components/WizardProgress';
import { Step1BasicInfo } from './steps/Step1BasicInfo';
import { Step2Connection } from './steps/Step2Connection';
import { Step3Params } from './steps/Step3Params';
import { Step4Test } from './steps/Step4Test';
import { Step5Routing } from './steps/Step5Routing';
import { Step6Summary } from './steps/Step6Summary';

interface WizardState {
  step1: { name: string; description: string; tags: string[] };
  step2: { host: string; port: number; systemId: string; password: string; bindType: number };
  step3: { windowSize: number; maxConnections: number; tpsLimit: number };
  step5: { pattern: string; priority: number }[];
  loading: boolean;
  error: string | null;
}

type Action =
  | { type: 'SET_STEP1'; payload: WizardState['step1'] }
  | { type: 'SET_STEP2'; payload: WizardState['step2'] }
  | { type: 'SET_STEP3'; payload: WizardState['step3'] }
  | { type: 'SET_STEP5'; payload: WizardState['step5'] }
  | { type: 'SET_LOADING'; payload: boolean }
  | { type: 'SET_ERROR'; payload: string | null };

const initialState: WizardState = {
  step1: { name: '', description: '', tags: [] },
  step2: { host: '', port: 2775, systemId: '', password: '', bindType: 0 },
  step3: { windowSize: 10, maxConnections: 1, tpsLimit: 100 },
  step5: [],
  loading: false,
  error: null,
};

function reducer(state: WizardState, action: Action): WizardState {
  switch (action.type) {
    case 'SET_STEP1': return { ...state, step1: action.payload };
    case 'SET_STEP2': return { ...state, step2: action.payload };
    case 'SET_STEP3': return { ...state, step3: action.payload };
    case 'SET_STEP5': return { ...state, step5: action.payload };
    case 'SET_LOADING': return { ...state, loading: action.payload };
    case 'SET_ERROR': return { ...state, error: action.payload };
    default: return state;
  }
}

export function ProviderWizardPage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [state, dispatch] = useReducer(reducer, initialState);

  const currentStep = Math.max(1, Math.min(6, Number(searchParams.get('step') ?? 1)));

  const setStep = (n: number) => setSearchParams({ step: String(n) });
  const next = () => setStep(currentStep + 1);
  const back = () => setStep(currentStep - 1);

  // Guard against unload
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = ''; };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, []);

  const buildPayload = (): CreateProviderPayload => ({
    name: state.step1.name,
    description: state.step1.description,
    tags: state.step1.tags,
    host: state.step2.host,
    port: state.step2.port,
    system_id: state.step2.systemId,
    password: state.step2.password,
    bind_type: state.step2.bindType,
    window_size: state.step3.windowSize,
    max_connections: state.step3.maxConnections,
    tps_limit: state.step3.tpsLimit,
    routing_rules: state.step5,
  });

  const handleActivate = async () => {
    dispatch({ type: 'SET_LOADING', payload: true });
    dispatch({ type: 'SET_ERROR', payload: null });
    try {
      await providersApi.create(buildPayload());
      navigate('/providers');
    } catch (err: any) {
      dispatch({ type: 'SET_ERROR', payload: err?.error?.message ?? 'Ошибка создания провайдера' });
      dispatch({ type: 'SET_LOADING', payload: false });
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Header */}
      <div className="bg-slate-900 border-b border-slate-800 px-6 py-4">
        <div className="flex items-center gap-2 text-sm mb-4">
          <button onClick={() => {
            if (window.confirm('Прогресс будет потерян. Выйти из wizard?')) navigate('/providers');
          }} className="text-slate-400 hover:text-slate-200">← Мои провайдеры</button>
          <span className="text-slate-600">/</span>
          <span className="text-slate-200 font-medium">Подключить SMPP провайдера</span>
        </div>
        <WizardProgress currentStep={currentStep} />
      </div>

      {/* Content */}
      <div className="flex">
        <div className="flex-1 p-8 max-w-2xl">
          {state.error && (
            <div className="bg-red-950 border border-red-700 rounded p-3 mb-4 text-red-300 text-sm">{state.error}</div>
          )}
          {currentStep === 1 && (
            <Step1BasicInfo data={state.step1} onChange={p => dispatch({ type: 'SET_STEP1', payload: p })} onNext={next} />
          )}
          {currentStep === 2 && (
            <Step2Connection data={state.step2} onChange={p => dispatch({ type: 'SET_STEP2', payload: p })} onBack={back} onNext={next} />
          )}
          {currentStep === 3 && (
            <Step3Params data={state.step3} onChange={p => dispatch({ type: 'SET_STEP3', payload: p })} onBack={back} onNext={next} />
          )}
          {currentStep === 4 && (
            <Step4Test connection={state.step2} onBack={back} onNext={next} />
          )}
          {currentStep === 5 && (
            <Step5Routing rules={state.step5} onChange={r => dispatch({ type: 'SET_STEP5', payload: r })} onBack={back} onNext={next} />
          )}
          {currentStep === 6 && (
            <Step6Summary data={buildPayload()} loading={state.loading} onBack={back} onActivate={handleActivate} />
          )}
        </div>

        {/* Sidebar tips */}
        <div className="w-56 bg-slate-900 border-l border-slate-800 p-5 shrink-0 hidden lg:block">
          <div className="text-xs text-slate-500 uppercase tracking-wider mb-2">Подсказка</div>
          {currentStep === 2 && <p className="text-sm text-slate-400">Стандартный SMPP-порт: 2775 (незашифрованный) или 2776 (TLS).</p>}
          {currentStep === 3 && <p className="text-sm text-slate-400">Window Size — количество PDU без подтверждения. Начни с 10, увеличивай при высокой нагрузке.</p>}
          {currentStep === 4 && <p className="text-sm text-slate-400">Тест делает реальный SMPP bind и сразу unbind. Трафик не отправляется. Если тест не прошёл — проверь IP-whitelist на стороне провайдера.</p>}
          {currentStep === 5 && <p className="text-sm text-slate-400">Без routing rules провайдер создаётся, но не получает трафик. Добавь правила сейчас или позже в настройках.</p>}
          <div className="mt-4 pt-4 border-t border-slate-800">
            <div className="text-xs text-slate-500 uppercase tracking-wider mb-2">Прогресс</div>
            <div className="text-sm text-slate-400">{currentStep} из 6</div>
            <div className="h-1 bg-slate-800 rounded mt-1">
              <div className="h-1 bg-blue-500 rounded" style={{ width: `${(currentStep / 6) * 100}%` }} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Проверить сборку**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/providers/ProviderWizardPage.tsx
git commit -m "feat(wizard): add ProviderWizardPage container with useReducer state"
```

---

## Task 18: Frontend — ProvidersPage и App.tsx routing

**Files:**
- Create: `portal-frontend/src/pages/providers/ProvidersPage.tsx`
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Создать ProvidersPage**

```tsx
// portal-frontend/src/pages/providers/ProvidersPage.tsx
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { providersApi, ClientProvider } from '../../api/client';

const BIND_LABELS = ['TRX', 'TX', 'RX'];

export function ProvidersPage() {
  const navigate = useNavigate();
  const [providers, setProviders] = useState<ClientProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    providersApi.list()
      .then(setProviders)
      .catch(err => setError(err?.error?.message ?? 'Ошибка загрузки'))
      .finally(() => setLoading(false));
  }, []);

  const handleDelete = async (id: string) => {
    if (!window.confirm('Удалить провайдера? Трафик перестанет маршрутизироваться через него.')) return;
    try {
      await providersApi.delete(id);
      setProviders(prev => prev.filter(p => p.id !== id));
    } catch (err: any) {
      alert(err?.error?.message ?? 'Ошибка удаления');
    }
  };

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-100">SMPP Провайдеры</h1>
          <p className="text-slate-400 text-sm mt-1">Управление подключёнными SMPP-провайдерами</p>
        </div>
        <button
          onClick={() => navigate('/providers/new?step=1')}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-500 font-medium"
        >
          + Подключить провайдера
        </button>
      </div>

      {loading && <div className="text-slate-400">Загрузка...</div>}
      {error && <div className="text-red-400">{error}</div>}

      {!loading && !error && providers.length === 0 && (
        <div className="text-center py-16 text-slate-500">
          <div className="text-4xl mb-3">📡</div>
          <p className="text-lg">Нет подключённых провайдеров</p>
          <p className="text-sm mt-1">Нажми "Подключить провайдера" чтобы начать</p>
        </div>
      )}

      {providers.length > 0 && (
        <div className="grid gap-4">
          {providers.map(p => (
            <div key={p.id} className="bg-slate-900 border border-slate-800 rounded-lg p-4 flex items-center gap-4">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-slate-100">{p.name}</span>
                  <span className={`text-xs px-2 py-0.5 rounded ${p.active ? 'bg-green-900 text-green-300' : 'bg-slate-700 text-slate-400'}`}>
                    {p.active ? 'Активен' : 'Неактивен'}
                  </span>
                </div>
                <div className="flex gap-4 mt-1 text-xs text-slate-500">
                  <span>{p.host}:{p.port}</span>
                  <span>{p.system_id}</span>
                  <span>{BIND_LABELS[p.bind_type] ?? 'TRX'}</span>
                  {p.routing_rules.length > 0 && <span>{p.routing_rules.length} маршрутов</span>}
                </div>
              </div>
              <button
                onClick={() => handleDelete(p.id)}
                className="text-red-400 hover:text-red-300 text-sm px-3 py-1"
              >
                Удалить
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Добавить роуты в App.tsx**

Открой `portal-frontend/src/App.tsx`. Добавь импорты:

```tsx
import { ProvidersPage } from './pages/providers/ProvidersPage';
import { ProviderWizardPage } from './pages/providers/ProviderWizardPage';
```

В блоке `protected routes` добавь (после `/audit-log`):

```tsx
<Route path="/providers" element={<ProvidersPage />} />
<Route path="/providers/new" element={<ProviderWizardPage />} />
```

- [ ] **Step 3: Финальная проверка сборки**

```bash
cd portal-frontend && npx tsc --noEmit && npm run build
```

Ожидаемый результат: сборка без ошибок и предупреждений.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/providers/ProvidersPage.tsx \
        portal-frontend/src/App.tsx
git commit -m "feat(frontend): add ProvidersPage and register /providers routes in App"
```

---

## Финальная проверка

- [ ] **Backend компиляция**

```bash
go build ./...
```

Ожидаемый результат: без ошибок.

- [ ] **Backend тесты**

```bash
go test ./internal/services/provider/... ./internal/gateway/portal/handlers/... -v
```

- [ ] **Frontend сборка**

```bash
cd portal-frontend && npm run build
```

- [ ] **Итоговый commit**

```bash
git add -A
git commit -m "feat(smpp-wizard): complete 6-step provider wizard with client isolation"
```
