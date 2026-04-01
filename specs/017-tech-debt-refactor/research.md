# Research: Technical Debt Resolution

**Branch**: `017-tech-debt-refactor` | **Date**: 2026-04-01

## Категория 1: Дублирование авторизации gRPC

### Находки

**Файлы с дублированием:**
- `internal/services/messaging/grpc/server.go` — `uuid.Parse(req.ClientId)` повторяется в `SendMessage` (line 52), `SendBatch`, `GetMessageHistory`, `GetMessageStatus` и других методах
- `internal/services/client/grpc/server.go` — аналогичный паттерн в `CreateClient`, `UpdateClient`, `DeleteClient`, `GetClient`

**Паттерн дублирования (пример из SendMessage, line 47-55):**
```go
if req.ClientId == "" {
    return nil, status.Error(codes.InvalidArgument, "client_id is required")
}
clientID, err := uuid.Parse(req.ClientId)
if err != nil {
    return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
}
```

**Уточнение**: client_id передаётся в поле запроса (`req.ClientId`), НЕ в gRPC metadata/context. Поэтому решение — не interceptor на уровне metadata, а helper-функция `parseClientID(req.ClientId) (uuid.UUID, error)`.

### Решение

- **Decision**: Вспомогательная функция `parseClientID` в пакете `grpc/`, вызываемая из каждого метода
- **Rationale**: Interceptor не подходит, так как client_id в теле запроса (разные proto), а не в metadata. Helper устраняет дублирование без изменения архитектуры.
- **Alternatives considered**: gRPC unary interceptor — отклонён, т.к. `req.ClientId` — поле разных proto-типов, нет единого интерфейса

---

## Категория 2: N+1 запросы в LeastLoadedSelector

### Находки

**Файл**: `internal/services/routing/application/selection_strategy.go`, lines 108-117

```go
for _, provider := range activeProviders {
    health, err := s.providerRepo.GetHealth(ctx, provider.ID)  // ← N запросов!
```

**Интерфейс репозитория** (`domain/repository.go`, line 24):
```go
GetHealth(ctx context.Context, id uuid.UUID) (*ProviderHealth, error)
```
Метода `GetHealthBatch` нет — нужно добавить.

### Решение

- **Decision**: Добавить `GetHealthBatch(ctx, ids []uuid.UUID) (map[uuid.UUID]*ProviderHealth, error)` в `ProviderRepository` + PostgreSQL реализацию с `WHERE id = ANY($1)`
- **Rationale**: Один SQL запрос с ANY($1) вместо N. Стандартный паттерн для pgx/v5.
- **Alternatives considered**: Кеш в Redis — избыточно для данной задачи; health данные уже в памяти провайдера

---

## Категория 3: Утечка памяти в RoundRobinSelector

### Находки

**Файл**: `internal/services/routing/application/selection_strategy.go`, lines 18-26

```go
type RoundRobinSelector struct {
    currentIndex map[uuid.UUID]int  // растёт бесконечно
}
```

Когда маршрут удаляется или меняется его ID, старая запись в `currentIndex` остаётся. Для долгоживущего процесса с частым созданием маршрутов это утечка.

### Решение

- **Decision**: Добавить метод `PurgeStaleRoutes(activeRouteIDs []uuid.UUID)` — удаляет из `currentIndex` записи, не входящие в `activeRouteIDs`. Вызывать после загрузки маршрутов в `RoutingService.SelectProvider`.
- **Rationale**: Минимальное изменение, не требует горутин или TTL. Selector вызывается с актуальным списком маршрутов — их ID и есть baseline.
- **Alternatives considered**: sync.Map с TTL — усложняет код без дополнительной пользы; подойдёт при >10k маршрутов (не наш случай)

---

## Категория 4: Магические строки DLR-статусов

### Находки

**Файл**: `internal/services/messaging/application/dlr_service.go`, lines 98-103

```go
switch stat {
case "DELIVRD":
case "EXPIRED":
case "REJECTD", "UNDELIV":
```

**Существующие константы** в `internal/smpp/protocol/constants.go` (lines 189-200): есть `MSG_STATE_DELIVERED` и аналоги для SMPP message states, но они относятся к SMPP `message_state` полю (int), а не к DLR stat строке.

### Решение

- **Decision**: Создать `internal/shared/dlr/status.go` с типизированными константами `DLRStatus` (`DELIVRD`, `EXPIRED`, `REJECTD`, `UNDELIV`) и использовать их в `dlr_service.go`
- **Rationale**: Центральный пакет `shared/dlr/` — правильное место, т.к. DLR может обрабатываться в разных сервисах (messaging, cascade)
- **Alternatives considered**: Добавить в `smpp/protocol/constants.go` — неправильно, т.к. DLR stat — не SMPP протокольная константа

---

## Категория 5: SMPPConnectionAdapter заглушка

### Находки

**Файл**: `internal/services/provider/infrastructure/smpp/pool_adapter.go`, lines 186-190

```go
func (a *ConnectionAdapter) SendMessage(...) (string, error) {
    return "", fmt.Errorf("use SenderService.SendMessage instead of direct connection call")
}
```

**Контекст**: `ConnectionAdapter` реализует интерфейс `application.Connection`. `SendMessage` — часть этого интерфейса. Реальная отправка происходит через `SenderService`, у которого есть доступ к `smsc.Sender` с провайдер-контекстом.

### Решение

- **Decision**: Удалить метод `SendMessage` из интерфейса `application.Connection` если он нигде не вызывается через этот интерфейс; или переименовать интерфейс чтобы исключить `SendMessage`
- **Rationale**: Заглушка с паник-подобной ошибкой нарушает принцип наименьшего удивления. Если метод не должен вызываться — его не должно быть в интерфейсе.
- **Action required**: Перед удалением — проверить все места использования `application.Connection` interface

---

## Категория 6: Тихие ошибки

### Находки

**1. DLR event publish** — `internal/services/messaging/application/dlr_service.go`, line 128:
```go
if err := s.eventPublisher.PublishMessageStatusChanged(ctx, msg, oldStatus); err != nil {
    log.Warn().Err(err).Msg("ошибка публикации события message.status.changed")
}
```
→ Уровень Warn неверен: потеря Kafka события — бизнес-критичная ошибка. Исправить на Error.

**2. Scheduler Warn** — `internal/services/messaging/application/scheduler.go`, line 113:
```go
s.logger.Warn().Err(err).Msg("failed to update status to queued (message already in Kafka)")
```
→ **Обоснован**: сообщение уже в Kafka и будет обработано. Статус "pending" вместо "queued" — временное несоответствие, pipeline-worker исправит при обработке. Уровень Warn оставить.

**3. io.Copy в webhook** — `internal/services/webhook/infrastructure/http/delivery_client.go`, line 72:
```go
io.Copy(io.Discard, resp.Body)  // err игнорируется
```
→ Некритично (чтение ответа для дренажа соединения), но стоит логировать.

### Решение

- **Decision**: Изменить уровень в `dlr_service.go:128` с Warn на Error; добавить метрику `dlr_event_publish_errors_total`
- **Scheduler Warn**: оставить как есть — это архитектурно обоснованный компромисс
- **io.Copy**: добавить `if _, err := io.Copy(io.Discard, resp.Body); err != nil { logger.Debug()... }`

---

## Категория 7: Фронтенд — useMemo/useCallback

### Находки

**CampaignsPage.tsx**, line 90:
```tsx
const columns: Column<Campaign>[] = [  // без useMemo — пересоздаётся при каждом рендере
  { key: 'name', render: (c) => <span>{c.name}</span> },
  ...
```

**Затронутые файлы** (по git status):
- `CampaignsPage.tsx`, `CampaignDetailPage.tsx`, `MessagesPage.tsx`, `ContactListDetailPage.tsx`, `AuditLogPage.tsx`, `APIKeysPage.tsx`, `AnalyticsPage.tsx`, `WebhooksPage.tsx`, `DomainsPage.tsx`, `OperatorSupportMatrix.tsx`

**Паттерн проблемы**: `columns` и `rowActions` определяются как `const` внутри функционального компонента без `useMemo` — DataTable получает новый массив при каждом рендере и может делать лишние ре-рендеры.

### Решение

- **Decision**: Обернуть `columns` в `useMemo(() => [...], [зависимости])` и обработчики действий в `useCallback` во всех затронутых компонентах
- **Rationale**: Стандартная практика React; особенно важно для DataTable который может принимать `columns` как prop и рендерить заголовки отдельно
- **Alternatives considered**: React.memo на DataTable — дополнительно, но не замена useMemo на caller side
