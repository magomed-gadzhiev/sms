# Scheduled Messages API — Complete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить `GET /api/v1/sms/scheduled` эндпоинт для просмотра запланированных сообщений и отобразить статус `scheduled` во фронтенде (messages history page).

**Architecture:** Сервис уже умеет создавать scheduled messages (90% готово). Нужно дополнить: repository query, service method, gRPC proto + handler, HTTP handler, OpenAPI spec entry, frontend status badge. Все компоненты следуют уже устоявшимся паттернам GetHistory.

**Tech Stack:** Go 1.24, jackc/pgx/v5, google.golang.org/grpc, gorilla/mux, TypeScript + React 19, protobuf.

---

## File Map

| Файл | Действие | Что делаем |
|---|---|---|
| `api/proto/messaging/messaging.proto` | Modify | Добавить `ListScheduledMessages` RPC + request/response |
| `api/proto/messagingv1/messaging.pb.go` | Regenerate | `protoc` из proto |
| `internal/services/messaging/infrastructure/repository/message_repository.go` | Modify | `ListScheduled` метод |
| `internal/services/messaging/application/message_service.go` | Modify | `ListScheduledMessages` метод |
| `internal/services/messaging/grpc/server.go` | Modify | gRPC handler `ListScheduledMessages` |
| `internal/gateway/client/handlers/sms.go` | Modify | HTTP handler `ListScheduled` |
| `internal/gateway/client/router/router.go` | Modify | Зарегистрировать `GET /api/v1/sms/scheduled` |
| `api/openapi/openapi.yaml` | Modify | Добавить endpoint + `scheduled` в status enum |
| `portal-frontend/src/pages/messages/MessagesPage.tsx` | Modify | Бейджик `scheduled` + колонка `Planned At` |

---

## Task 1: Добавить ListScheduled в репозиторий

**Files:**
- Modify: `internal/services/messaging/infrastructure/repository/message_repository.go`
- Test: `internal/services/messaging/infrastructure/repository/message_repository_test.go`

- [ ] **Step 1.1: Написать failing тест**

Добавить в конец `message_repository_test.go`:

```go
func TestMessageRepository_ListScheduled(t *testing.T) {
    ctx := context.Background()
    // Предположим, что setupTestDB возвращает репозиторий с тестовой БД
    // Смотри существующие тесты в том же файле для паттерна setup
    repo := setupTestRepo(t)

    clientID := uuid.New()
    now := time.Now().UTC()

    // Создать 3 scheduled + 1 sent сообщение
    for i := 0; i < 3; i++ {
        scheduledAt := now.Add(time.Duration(i+1) * time.Hour)
        msg := &domain.Message{
            ID:          uuid.New(),
            ClientID:    clientID,
            Source:      "TEST",
            Destination: fmt.Sprintf("+7999000000%d", i),
            Text:        fmt.Sprintf("message %d", i),
            Status:      "scheduled",
            ScheduledAt: &scheduledAt,
            CreatedAt:   now,
            UpdatedAt:   now,
        }
        require.NoError(t, repo.Create(ctx, msg))
    }
    // sent сообщение — не должно попасть в результат
    sentMsg := &domain.Message{
        ID:          uuid.New(),
        ClientID:    clientID,
        Source:      "TEST",
        Destination: "+79990000099",
        Text:        "sent message",
        Status:      "sent",
        CreatedAt:   now,
        UpdatedAt:   now,
    }
    require.NoError(t, repo.Create(ctx, sentMsg))

    messages, total, err := repo.ListScheduled(ctx, clientID, 10, 0)
    require.NoError(t, err)
    assert.Equal(t, 3, total)
    assert.Len(t, messages, 3)
    // Проверить сортировку по scheduled_at ASC
    assert.True(t, messages[0].ScheduledAt.Before(*messages[1].ScheduledAt))
}

func TestMessageRepository_ListScheduled_Pagination(t *testing.T) {
    ctx := context.Background()
    repo := setupTestRepo(t)
    clientID := uuid.New()
    now := time.Now().UTC()

    for i := 0; i < 5; i++ {
        scheduledAt := now.Add(time.Duration(i+1) * time.Hour)
        msg := &domain.Message{
            ID:          uuid.New(),
            ClientID:    clientID,
            Source:      "TEST",
            Destination: fmt.Sprintf("+7999000000%d", i),
            Text:        fmt.Sprintf("msg %d", i),
            Status:      "scheduled",
            ScheduledAt: &scheduledAt,
            CreatedAt:   now,
            UpdatedAt:   now,
        }
        require.NoError(t, repo.Create(ctx, msg))
    }

    messages, total, err := repo.ListScheduled(ctx, clientID, 2, 2)
    require.NoError(t, err)
    assert.Equal(t, 5, total)  // total всегда полный count
    assert.Len(t, messages, 2) // page size = 2
}
```

- [ ] **Step 1.2: Запустить тест — убедиться, что падает**

```bash
cd /c/projects/sms
go test ./internal/services/messaging/infrastructure/repository/... -run TestMessageRepository_ListScheduled -v
```

Ожидаем: `FAIL — undefined: repo.ListScheduled`

- [ ] **Step 1.3: Реализовать ListScheduled**

Добавить метод в `internal/services/messaging/infrastructure/repository/message_repository.go` после метода `GetScheduledReady`:

```go
// ListScheduled returns paginated list of messages with status='scheduled' for a client.
func (r *MessageRepository) ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Message, int, error) {
    const countQuery = `
        SELECT COUNT(*)
        FROM messages
        WHERE client_id = $1 AND status = 'scheduled'`

    const listQuery = `
        SELECT id, message_id, external_id, source, destination, text,
               status, status_message, client_id, provider_id, route_id,
               retry_count, max_retries, next_retry_at,
               smpp_message_id, submitted_at, delivered_at, failed_at,
               scheduled_at, created_at, updated_at
        FROM messages
        WHERE client_id = $1 AND status = 'scheduled'
        ORDER BY scheduled_at ASC
        LIMIT $2 OFFSET $3`

    var total int
    if err := r.db.QueryRow(ctx, countQuery, clientID).Scan(&total); err != nil {
        return nil, 0, fmt.Errorf("count scheduled: %w", err)
    }

    rows, err := r.db.Query(ctx, listQuery, clientID, limit, offset)
    if err != nil {
        return nil, 0, fmt.Errorf("list scheduled: %w", err)
    }
    defer rows.Close()

    var messages []*domain.Message
    for rows.Next() {
        var sharedMsg shared.Message
        if err := rows.Scan(
            &sharedMsg.ID, &sharedMsg.MessageID, &sharedMsg.ExternalID,
            &sharedMsg.Source, &sharedMsg.Destination, &sharedMsg.Text,
            &sharedMsg.Status, &sharedMsg.StatusMessage,
            &sharedMsg.ClientID, &sharedMsg.ProviderID, &sharedMsg.RouteID,
            &sharedMsg.RetryCount, &sharedMsg.MaxRetries, &sharedMsg.NextRetryAt,
            &sharedMsg.SMPPMessageID, &sharedMsg.SubmittedAt, &sharedMsg.DeliveredAt,
            &sharedMsg.FailedAt, &sharedMsg.ScheduledAt,
            &sharedMsg.CreatedAt, &sharedMsg.UpdatedAt,
        ); err != nil {
            return nil, 0, fmt.Errorf("scan scheduled message: %w", err)
        }
        messages = append(messages, domain.MessageFromShared(&sharedMsg))
    }
    if err := rows.Err(); err != nil {
        return nil, 0, fmt.Errorf("iterate scheduled messages: %w", err)
    }

    return messages, total, nil
}
```

> **Примечание:** Если в репозитории есть хелпер `scanMessage()` или аналогичный — используй его вместо ручного Scan. Посмотри существующий метод `GetHistory` в том же файле для точных имён полей shared.Message.

- [ ] **Step 1.4: Запустить тесты — убедиться, что проходят**

```bash
go test ./internal/services/messaging/infrastructure/repository/... -run TestMessageRepository_ListScheduled -v
```

Ожидаем: `PASS`

- [ ] **Step 1.5: Коммит**

```bash
git add internal/services/messaging/infrastructure/repository/message_repository.go
git add internal/services/messaging/infrastructure/repository/message_repository_test.go
git commit -m "feat(messaging): add ListScheduled repository method"
```

---

## Task 2: Добавить ListScheduledMessages в сервис

**Files:**
- Modify: `internal/services/messaging/application/message_service.go`
- Test: `internal/services/messaging/application/message_service_test.go`

- [ ] **Step 2.1: Написать failing тест**

Добавить в `message_service_test.go`:

```go
func TestMessageService_ListScheduledMessages(t *testing.T) {
    // Найди паттерн создания mockRepo в существующих тестах этого файла
    mockRepo := new(MockMessageRepository)
    mockPublisher := new(MockEventPublisher)
    svc := NewMessageService(mockRepo, mockPublisher)

    clientID := uuid.New()
    now := time.Now().UTC()
    scheduledAt := now.Add(2 * time.Hour)

    expected := []*domain.Message{
        {
            ID:          uuid.New(),
            ClientID:    clientID,
            Status:      "scheduled",
            ScheduledAt: &scheduledAt,
        },
    }

    mockRepo.On("ListScheduled", mock.Anything, clientID, 10, 0).Return(expected, 1, nil)

    messages, total, err := svc.ListScheduledMessages(context.Background(), clientID, 10, 0)

    require.NoError(t, err)
    assert.Equal(t, 1, total)
    assert.Equal(t, expected, messages)
    mockRepo.AssertExpectations(t)
}

func TestMessageService_ListScheduledMessages_RepoError(t *testing.T) {
    mockRepo := new(MockMessageRepository)
    mockPublisher := new(MockEventPublisher)
    svc := NewMessageService(mockRepo, mockPublisher)

    clientID := uuid.New()
    mockRepo.On("ListScheduled", mock.Anything, clientID, 10, 0).
        Return(nil, 0, errors.New("db error"))

    _, _, err := svc.ListScheduledMessages(context.Background(), clientID, 10, 0)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "db error")
}
```

- [ ] **Step 2.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/messaging/application/... -run TestMessageService_ListScheduled -v
```

Ожидаем: `FAIL — undefined: svc.ListScheduledMessages`

- [ ] **Step 2.3: Реализовать ListScheduledMessages**

Добавить метод в `internal/services/messaging/application/message_service.go` рядом с `CancelMessage`:

```go
// ListScheduledMessages returns paginated list of scheduled (not yet sent) messages for a client.
func (s *MessageService) ListScheduledMessages(
    ctx context.Context,
    clientID uuid.UUID,
    limit, offset int,
) ([]*domain.Message, int, error) {
    if limit <= 0 || limit > 1000 {
        limit = 100
    }
    if offset < 0 {
        offset = 0
    }
    return s.messageRepo.ListScheduled(ctx, clientID, limit, offset)
}
```

- [ ] **Step 2.4: Добавить ListScheduled в интерфейс MessageRepository**

Найди интерфейс `MessageRepository` в `internal/services/messaging/domain/` (или в том же файле application) и добавь метод:

```go
ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*Message, int, error)
```

- [ ] **Step 2.5: Запустить тесты**

```bash
go test ./internal/services/messaging/application/... -run TestMessageService_ListScheduled -v
```

Ожидаем: `PASS`

- [ ] **Step 2.6: Коммит**

```bash
git add internal/services/messaging/application/message_service.go
git add internal/services/messaging/application/message_service_test.go
git add internal/services/messaging/domain/
git commit -m "feat(messaging): add ListScheduledMessages service method"
```

---

## Task 3: Добавить proto RPC + регенерировать pb.go

**Files:**
- Modify: `api/proto/messaging/messaging.proto`
- Regenerate: `api/proto/messagingv1/messaging.pb.go`

- [ ] **Step 3.1: Добавить RPC и message types в proto**

В файле `api/proto/messaging/messaging.proto` добавить:

1. В сервис `MessagingService` (рядом с `GetMessageHistory`):

```protobuf
rpc ListScheduledMessages(ListScheduledMessagesRequest) returns (ListScheduledMessagesResponse);
```

2. После message `GetMessageHistoryResponse` добавить:

```protobuf
message ListScheduledMessagesRequest {
  string client_id = 1;
  int32 limit = 2;
  int32 offset = 3;
}

message ListScheduledMessagesResponse {
  repeated MessageInfo messages = 1;
  int32 total = 2;
  int32 limit = 3;
  int32 offset = 4;
}
```

- [ ] **Step 3.2: Регенерировать pb.go**

```bash
cd /c/projects/sms
# Используй тот же protoc-вызов, что и для других proto файлов
# Проверь Makefile или scripts/ для точной команды, обычно:
protoc --go_out=. --go-grpc_out=. api/proto/messaging/messaging.proto
# Или если есть Makefile target:
make proto
```

Проверь, что `api/proto/messagingv1/messaging.pb.go` обновился — должны появиться `ListScheduledMessagesRequest`, `ListScheduledMessagesResponse`, метод `ListScheduledMessages` в интерфейсе.

- [ ] **Step 3.3: Убедиться, что проект компилируется**

```bash
go build ./...
```

Ожидаем: компиляция без ошибок (кроме "not implemented" в grpc server, это исправим в Task 4).

- [ ] **Step 3.4: Коммит**

```bash
git add api/proto/messaging/messaging.proto api/proto/messagingv1/
git commit -m "feat(messaging): add ListScheduledMessages proto RPC"
```

---

## Task 4: Реализовать gRPC handler

**Files:**
- Modify: `internal/services/messaging/grpc/server.go`
- Test: `internal/services/messaging/grpc/server_test.go`

- [ ] **Step 4.1: Написать failing тест**

Добавить в `server_test.go` (по аналогии с тестом `GetMessageHistory`):

```go
func TestServer_ListScheduledMessages(t *testing.T) {
    mockSvc := new(MockMessageService)
    srv := NewServer(mockSvc)

    clientID := uuid.New()
    now := time.Now().UTC()
    scheduledAt := now.Add(2 * time.Hour)

    domainMessages := []*domain.Message{
        {
            ID:          uuid.New(),
            ClientID:    clientID,
            Source:      "TEST",
            Destination: "+79991234567",
            Text:        "hello",
            Status:      "scheduled",
            ScheduledAt: &scheduledAt,
            CreatedAt:   now,
            UpdatedAt:   now,
        },
    }

    mockSvc.On("ListScheduledMessages", mock.Anything, clientID, 10, 0).
        Return(domainMessages, 1, nil)

    req := &pb.ListScheduledMessagesRequest{
        ClientId: clientID.String(),
        Limit:    10,
        Offset:   0,
    }

    resp, err := srv.ListScheduledMessages(context.Background(), req)

    require.NoError(t, err)
    assert.Equal(t, int32(1), resp.Total)
    assert.Len(t, resp.Messages, 1)
    assert.Equal(t, "scheduled", resp.Messages[0].Status)
    mockSvc.AssertExpectations(t)
}

func TestServer_ListScheduledMessages_InvalidClientID(t *testing.T) {
    mockSvc := new(MockMessageService)
    srv := NewServer(mockSvc)

    req := &pb.ListScheduledMessagesRequest{
        ClientId: "not-a-uuid",
        Limit:    10,
    }

    _, err := srv.ListScheduledMessages(context.Background(), req)
    require.Error(t, err)
    assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
```

- [ ] **Step 4.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/services/messaging/grpc/... -run TestServer_ListScheduledMessages -v
```

Ожидаем: `FAIL`

- [ ] **Step 4.3: Реализовать gRPC handler**

Добавить в `internal/services/messaging/grpc/server.go` рядом с `GetMessageHistory`:

```go
func (s *Server) ListScheduledMessages(
    ctx context.Context,
    req *pb.ListScheduledMessagesRequest,
) (*pb.ListScheduledMessagesResponse, error) {
    clientID, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
    }

    limit := int(req.Limit)
    offset := int(req.Offset)

    messages, total, err := s.messageService.ListScheduledMessages(ctx, clientID, limit, offset)
    if err != nil {
        s.logger.Error().Err(err).Str("client_id", req.ClientId).Msg("list scheduled messages failed")
        return nil, status.Errorf(codes.Internal, "list scheduled messages: %v", err)
    }

    pbMessages := make([]*pb.MessageInfo, len(messages))
    for i, msg := range messages {
        pbMessages[i] = messageToProto(msg) // используй существующий хелпер, найди его в server.go
    }

    return &pb.ListScheduledMessagesResponse{
        Messages: pbMessages,
        Total:    int32(total),
        Limit:    req.Limit,
        Offset:   req.Offset,
    }, nil
}
```

> **Важно:** Найди существующую функцию `messageToProto` или `toMessageInfo` в `server.go` и используй её. Если она называется иначе — адаптируй вызов.

- [ ] **Step 4.4: Запустить тесты**

```bash
go test ./internal/services/messaging/grpc/... -run TestServer_ListScheduledMessages -v
```

Ожидаем: `PASS`

- [ ] **Step 4.5: Коммит**

```bash
git add internal/services/messaging/grpc/server.go
git add internal/services/messaging/grpc/server_test.go
git commit -m "feat(messaging): add ListScheduledMessages gRPC handler"
```

---

## Task 5: HTTP handler + роутер

**Files:**
- Modify: `internal/gateway/client/handlers/sms.go`
- Modify: `internal/gateway/client/router/router.go`
- Test: `internal/gateway/client/handlers/sms_test.go`

- [ ] **Step 5.1: Написать failing тест**

Добавить в `sms_test.go` (по аналогии с тестом `TestGetHistory`):

```go
func TestListScheduled(t *testing.T) {
    mockClient := new(MockMessagingClient)
    handler := NewSMSHandlers(mockClient)

    clientID := uuid.New()
    now := time.Now().UTC()
    scheduledAt := now.Add(2 * time.Hour)

    mockClient.On("ListScheduledMessages", mock.Anything, &pb.ListScheduledMessagesRequest{
        ClientId: clientID.String(),
        Limit:    10,
        Offset:   0,
    }).Return(&pb.ListScheduledMessagesResponse{
        Messages: []*pb.MessageInfo{
            {
                MessageId:   "msg-1",
                Status:      "scheduled",
                Destination: "+79991234567",
                ScheduledAt: timestamppb.New(scheduledAt),
            },
        },
        Total:  1,
        Limit:  10,
        Offset: 0,
    }, nil)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/scheduled?limit=10&offset=0", nil)
    req = req.WithContext(context.WithValue(req.Context(), clientIDKey, clientID.String()))
    w := httptest.NewRecorder()

    handler.ListScheduled(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, float64(1), resp["total"])
    messages := resp["messages"].([]interface{})
    assert.Len(t, messages, 1)
}
```

- [ ] **Step 5.2: Запустить тест — убедиться, что падает**

```bash
go test ./internal/gateway/client/handlers/... -run TestListScheduled -v
```

Ожидаем: `FAIL`

- [ ] **Step 5.3: Реализовать HTTP handler**

Добавить в `internal/gateway/client/handlers/sms.go` рядом с `GetHistory`:

```go
// ListScheduled returns paginated list of scheduled messages for the authenticated client.
// GET /api/v1/sms/scheduled?limit=100&offset=0
func (h *SMSHandlers) ListScheduled(w http.ResponseWriter, r *http.Request) {
    clientID := getClientIDFromContext(r.Context()) // используй тот же хелпер, что GetHistory

    limit := 100
    offset := 0

    if v := r.URL.Query().Get("limit"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
            limit = n
        }
    }
    if v := r.URL.Query().Get("offset"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n >= 0 {
            offset = n
        }
    }

    resp, err := h.messagingClient.ListScheduledMessages(r.Context(), &pb.ListScheduledMessagesRequest{
        ClientId: clientID,
        Limit:    int32(limit),
        Offset:   int32(offset),
    })
    if err != nil {
        h.handleGRPCError(w, err) // используй существующий хелпер обработки ошибок
        return
    }

    type messageItem struct {
        MessageID   string  `json:"message_id"`
        Source      string  `json:"source"`
        Destination string  `json:"destination"`
        Text        string  `json:"text"`
        Status      string  `json:"status"`
        ExternalID  string  `json:"external_id,omitempty"`
        ScheduledAt *string `json:"scheduled_at,omitempty"`
        CreatedAt   string  `json:"created_at"`
    }

    items := make([]messageItem, len(resp.Messages))
    for i, m := range resp.Messages {
        item := messageItem{
            MessageID:   m.MessageId,
            Source:      m.Source,
            Destination: m.Destination,
            Text:        m.Text,
            Status:      m.Status,
            ExternalID:  m.ExternalId,
            CreatedAt:   m.CreatedAt.AsTime().Format(time.RFC3339),
        }
        if m.ScheduledAt != nil {
            t := m.ScheduledAt.AsTime().Format(time.RFC3339)
            item.ScheduledAt = &t
        }
        items[i] = item
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "messages": items,
        "total":    resp.Total,
        "limit":    resp.Limit,
        "offset":   resp.Offset,
    })
}
```

> **Важно:** Найди в `sms.go` функции `getClientIDFromContext`, `handleGRPCError`, `writeJSON` (или их аналоги) и используй точные имена.

- [ ] **Step 5.4: Зарегистрировать роут**

В `internal/gateway/client/router/router.go` добавить рядом с другими `/sms/` роутами:

```go
r.Handle("/api/v1/sms/scheduled", authMiddleware(http.HandlerFunc(h.sms.ListScheduled))).Methods(http.MethodGet)
```

> Посмотри, как зарегистрированы `GetHistory` и `CancelSMS` — используй точно такой же паттерн с middleware.

- [ ] **Step 5.5: Запустить тесты**

```bash
go test ./internal/gateway/client/... -run TestListScheduled -v
```

Ожидаем: `PASS`

- [ ] **Step 5.6: Проверить компиляцию всего проекта**

```bash
go build ./...
```

Ожидаем: ошибок нет.

- [ ] **Step 5.7: Коммит**

```bash
git add internal/gateway/client/handlers/sms.go
git add internal/gateway/client/router/router.go
git add internal/gateway/client/handlers/sms_test.go
git commit -m "feat(gateway): add GET /api/v1/sms/scheduled endpoint"
```

---

## Task 6: Обновить OpenAPI spec

**Files:**
- Modify: `api/openapi/openapi.yaml`

- [ ] **Step 6.1: Добавить эндпоинт /sms/scheduled**

В `api/openapi/openapi.yaml` найди секцию с `/sms/history` и добавить рядом:

```yaml
  /sms/scheduled:
    get:
      summary: List scheduled messages
      description: Returns a paginated list of messages scheduled for future delivery.
      operationId: listScheduledMessages
      tags:
        - SMS
      security:
        - ApiKeyAuth: []
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 1000
            default: 100
          description: Number of results per page
        - name: offset
          in: query
          schema:
            type: integer
            minimum: 0
            default: 0
          description: Pagination offset
      responses:
        '200':
          description: Paginated list of scheduled messages
          content:
            application/json:
              schema:
                type: object
                properties:
                  messages:
                    type: array
                    items:
                      $ref: '#/components/schemas/ScheduledMessageItem'
                  total:
                    type: integer
                    example: 42
                  limit:
                    type: integer
                    example: 100
                  offset:
                    type: integer
                    example: 0
        '401':
          $ref: '#/components/responses/Unauthorized'
```

- [ ] **Step 6.2: Добавить схему ScheduledMessageItem**

В секцию `components.schemas` добавить:

```yaml
    ScheduledMessageItem:
      type: object
      properties:
        message_id:
          type: string
          format: uuid
          example: "550e8400-e29b-41d4-a716-446655440000"
        source:
          type: string
          example: "MyCompany"
        destination:
          type: string
          example: "+79991234567"
        text:
          type: string
          example: "Your appointment is tomorrow"
        status:
          type: string
          enum: [scheduled]
          example: "scheduled"
        external_id:
          type: string
          example: "order-123"
        scheduled_at:
          type: string
          format: date-time
          example: "2026-04-15T10:00:00Z"
        created_at:
          type: string
          format: date-time
          example: "2026-04-12T08:30:00Z"
```

- [ ] **Step 6.3: Обновить status enum во всех схемах**

Найди в файле все `enum` для поля `status` и убедись, что `scheduled` включён в список (если ещё нет). Поищи по строке `enum:` рядом с `pending`, `sent`, `delivered`.

- [ ] **Step 6.4: Валидировать YAML (опционально)**

```bash
# Если установлен swagger-cli или openapi-generator:
npx swagger-cli validate api/openapi/openapi.yaml
# Или просто проверь, что сервис стартует и /docs доступен
```

- [ ] **Step 6.5: Коммит**

```bash
git add api/openapi/openapi.yaml
git commit -m "docs(openapi): add GET /sms/scheduled endpoint and ScheduledMessageItem schema"
```

---

## Task 7: Frontend — статус scheduled + колонка "Запланировано"

**Files:**
- Modify: `portal-frontend/src/pages/messages/MessagesPage.tsx` (или аналогичный файл страницы истории)

> **Перед началом:** прочитай файл страницы messages/history и найди, как там рендерятся статусы (скорее всего через StatusBadge компонент или switch/map). Адаптируй шаги ниже под реальную структуру файла.

- [ ] **Step 7.1: Найти статус-компонент и добавить "scheduled"**

Найди в проекте компонент или функцию для отображения статуса сообщения:

```bash
grep -r "StatusBadge\|messageStatus\|status.*badge\|getStatusColor" portal-frontend/src --include="*.tsx" -l
```

Добавить в map/switch статусов:

```tsx
// Пример если используется объект-маппинг:
const STATUS_CONFIG: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  // ... существующие статусы ...
  scheduled: { label: 'Запланировано', variant: 'secondary' },
};

// Пример если используется switch:
case 'scheduled':
  return <Badge variant="secondary" className="bg-blue-100 text-blue-800">Запланировано</Badge>;
```

- [ ] **Step 7.2: Добавить колонку "Запланировано" в таблицу сообщений**

Найди определение колонок таблицы messages и добавить:

```tsx
{
  accessorKey: 'scheduled_at',
  header: 'Запланировано',
  cell: ({ row }) => {
    const scheduledAt = row.original.scheduled_at;
    if (!scheduledAt) return <span className="text-muted-foreground">—</span>;
    return (
      <span className="text-sm">
        {new Date(scheduledAt).toLocaleString('ru-RU', {
          day: '2-digit',
          month: '2-digit',
          year: 'numeric',
          hour: '2-digit',
          minute: '2-digit',
        })}
      </span>
    );
  },
},
```

- [ ] **Step 7.3: Добавить фильтр по статусу "scheduled"**

Найди фильтр статусов в форме фильтрации и добавить опцию:

```tsx
{ value: 'scheduled', label: 'Запланировано' },
```

- [ ] **Step 7.4: Запустить фронтенд и проверить вручную**

```bash
cd portal-frontend
npm run dev
```

Открыть в браузере страницу истории сообщений. Убедиться что:
- Статус "scheduled" отображается синим бейджиком "Запланировано"
- Колонка "Запланировано" отображает дату/время
- Фильтр по статусу "Запланировано" работает

- [ ] **Step 7.5: Коммит**

```bash
git add portal-frontend/src/
git commit -m "feat(frontend): display scheduled message status and scheduled_at column"
```

---

## Task 8: Финальная проверка

- [ ] **Step 8.1: Запустить все тесты**

```bash
go test ./internal/services/messaging/... -v
go test ./internal/gateway/client/... -v
```

Ожидаем: все тесты `PASS`.

- [ ] **Step 8.2: Проверить сборку**

```bash
go build ./...
cd portal-frontend && npm run build
```

- [ ] **Step 8.3: Финальный коммит если есть незакомиченные изменения**

```bash
git status
# Если есть изменения:
git add -p  # добавлять по частям
git commit -m "feat(messaging): complete scheduled messages API and frontend"
```
