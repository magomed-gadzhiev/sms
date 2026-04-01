# Data Model: Technical Debt Resolution

**Branch**: `017-tech-debt-refactor` | **Date**: 2026-04-01

*Примечание: данный рефакторинг не добавляет новых таблиц БД и не меняет схему данных. Все изменения касаются кода (интерфейсы, реализации, компоненты).*

---

## Новые / изменённые сущности кода

### 1. DLRStatus (новый тип)

**Файл**: `internal/shared/dlr/status.go` (NEW)

```go
type DLRStatus string

const (
    DLRStatusDelivered DLRStatus = "DELIVRD"
    DLRStatusExpired   DLRStatus = "EXPIRED"
    DLRStatusRejected  DLRStatus = "REJECTD"
    DLRStatusUndeliv   DLRStatus = "UNDELIV"
)
```

**Связи**: используется в `messaging/application/dlr_service.go`  
**Валидация**: `IsTerminal(s DLRStatus) bool` — возвращает true для финальных статусов

---

### 2. ProviderRepository (изменение интерфейса)

**Файл**: `internal/services/routing/domain/repository.go` (MODIFY)

Добавляемый метод:
```go
// GetHealthBatch загружает health данные для нескольких провайдеров одним запросом
GetHealthBatch(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*ProviderHealth, error)
```

**Реализация**: `internal/services/routing/infrastructure/postgres/provider_repo.go`  
SQL паттерн: `SELECT provider_id, active_connections, total_connections FROM provider_health WHERE provider_id = ANY($1)`  
**Используется в**: `LeastLoadedSelector.SelectProvider`

---

### 3. RoundRobinSelector (изменение структуры)

**Файл**: `internal/services/routing/application/selection_strategy.go` (MODIFY)

Добавляемый метод:
```go
// PurgeStaleRoutes удаляет из currentIndex маршруты, не входящие в activeRouteIDs
// Вызывать после каждого обновления конфигурации маршрутов
func (s *RoundRobinSelector) PurgeStaleRoutes(activeRouteIDs []uuid.UUID)
```

**State transitions**:
- `currentIndex[routeID]` создаётся при первом обращении → обновляется при каждом выборе → удаляется при вызове `PurgeStaleRoutes`

---

### 4. application.Connection (изменение интерфейса)

**Файл**: `internal/services/provider/application/connection.go` (MODIFY — после проверки использований)

Текущий интерфейс содержит метод `SendMessage` который реализован как заглушка с ошибкой. После аудита использований:
- Если `SendMessage` не вызывается через интерфейс — удалить из интерфейса
- `ConnectionAdapter` тогда не должен реализовывать этот метод

**Проверка перед изменением**: `grep -r "\.SendMessage(" internal/services/provider/`

---

### 5. gRPC Helper (новый utility)

**Файл**: `internal/services/messaging/grpc/helpers.go` (NEW)  
*Или добавить в существующий файл если таковой есть*

```go
// parseClientID разбирает и валидирует client_id из строки запроса
func parseClientID(raw string) (uuid.UUID, error) // возвращает gRPC status error
```

**Используется в**: все методы `messaging/grpc/server.go` и аналогично `client/grpc/server.go`

---

## Frontend: изменения компонентов

Нет новых типов данных. Изменения — только добавление хуков мемоизации.

| Компонент | Что добавляется |
|-----------|----------------|
| `CampaignsPage.tsx` | `useMemo` для `columns` (line 90) |
| `CampaignDetailPage.tsx` | `useMemo`/`useCallback` |
| `MessagesPage.tsx` | `useMemo` для `columns` |
| `ContactListDetailPage.tsx` | `useMemo`/`useCallback` |
| `AuditLogPage.tsx` | `useMemo` для `columns` |
| `APIKeysPage.tsx` | `useMemo`/`useCallback` |
| `AnalyticsPage.tsx` | `useMemo` для вычисляемых данных |
| `WebhooksPage.tsx` | `useMemo`/`useCallback` |
| `DomainsPage.tsx` | `useMemo`/`useCallback` |
| `OperatorSupportMatrix.tsx` | `useMemo` для матрицы |

---

## Ограничения

- Нет миграций БД
- Нет изменений proto-файлов
- Нет новых Kafka топиков
- Все изменения обратно совместимы внутри микросервиса
