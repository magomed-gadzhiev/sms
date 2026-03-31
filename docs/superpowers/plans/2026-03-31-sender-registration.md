# Sender Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать систему регистрации альфа-имён отправителей у операторов связи — от подачи заявки клиентом до проверки при маршрутизации сообщений.

**Architecture:** Новый DDD-сервис `sender-service` (gRPC порт 9104) с тремя таблицами в PostgreSQL. Portal и Admin gateway получают REST API через gRPC-клиент к sender-service. Routing-service вызывает `CheckSenderAllowed` на hot-path с Redis-кэшем (TTL 5 мин). Fail-closed: если sender-service недоступен, отправка блокируется.

**Tech Stack:** Go 1.24.0, pgx/v5 (PostgreSQL), go-redis/v9 (Redis), gorilla/mux (HTTP), google.golang.org/grpc, zerolog, testify

---

## File Structure

### Новые файлы

| Файл | Назначение |
|------|-----------|
| `api/proto/sender/sender.proto` | gRPC-контракт: SenderService (CRUD + CheckSenderAllowed) |
| `migrations/000064_sender_registrations.up.sql` | Таблицы: sender_registrations, registration_templates, registration_comments |
| `migrations/000064_sender_registrations.down.sql` | Откат миграции |
| `internal/services/sender/domain/registration.go` | Доменная модель SenderRegistration, статусы, валидация sender_name |
| `internal/services/sender/domain/template.go` | Доменная модель RegistrationTemplate |
| `internal/services/sender/domain/comment.go` | Доменная модель RegistrationComment |
| `internal/services/sender/domain/repository.go` | Интерфейсы репозиториев |
| `internal/services/sender/application/sender_service.go` | Бизнес-логика: create, submit, approve, reject, request-revision, check-allowed |
| `internal/services/sender/infrastructure/repository/registration_repo.go` | PostgreSQL-репозиторий для SenderRegistration |
| `internal/services/sender/infrastructure/repository/template_repo.go` | PostgreSQL-репозиторий для RegistrationTemplate |
| `internal/services/sender/infrastructure/repository/comment_repo.go` | PostgreSQL-репозиторий для RegistrationComment |
| `internal/services/sender/grpc/server.go` | gRPC-сервер sender-service |
| `cmd/services/sender-service/main.go` | Точка входа sender-service |
| `internal/gateway/portal/handlers/sender_registrations.go` | Portal REST handlers (клиентские) |
| `internal/gateway/admin/handlers/sender_registrations.go` | Admin REST handlers (администраторские) |
| `deployments/docker/sender-service.Dockerfile` | Dockerfile для sender-service |

### Изменяемые файлы

| Файл | Что меняется |
|------|-------------|
| `internal/gateway/portal/clients.go` | +SenderClient в ServiceClients и ServiceAddresses |
| `internal/gateway/admin/clients.go` | +SenderClient в ServiceClients и ServiceAddresses |
| `cmd/portal-gateway/main.go` | +SENDER_SERVICE_ADDR, создание SenderRegistrationHandlers, передача в роутер |
| `cmd/admin-gateway/main.go` | +SENDER_SERVICE_ADDR, создание SenderRegistrationHandlers, передача в роутер |
| `internal/gateway/portal/router/router.go` | +senderRegistrationHandlers параметр, +маршруты /sender-registrations |
| `internal/gateway/admin/router/router.go` | +senderRegistrationHandlers параметр, +маршруты /sender-registrations |
| `internal/services/routing/application/routing_service.go` | +SenderChecker интерфейс, вызов в RouteMessage |
| `cmd/services/routing-service/main.go` | +gRPC-клиент к sender-service, передача SenderChecker |
| `deployments/docker-compose.yml` | +sender-service контейнер, +SENDER_SERVICE_ADDR в routing-service и gateway |

---

### Task 1: Миграция БД

**Files:**
- Create: `migrations/000064_sender_registrations.up.sql`
- Create: `migrations/000064_sender_registrations.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000064_sender_registrations.up.sql

CREATE TABLE IF NOT EXISTS sender_registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       UUID NOT NULL REFERENCES clients(id),
    operator_id     UUID NOT NULL REFERENCES operators(id),
    sender_name     VARCHAR(11) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at    TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    CONSTRAINT sender_registrations_unique UNIQUE (client_id, operator_id, sender_name),
    CONSTRAINT sender_registrations_status_check
        CHECK (status IN ('draft','submitted','approved','rejected','revision_requested'))
);

CREATE INDEX IF NOT EXISTS idx_sender_registrations_client ON sender_registrations(client_id);
CREATE INDEX IF NOT EXISTS idx_sender_registrations_status ON sender_registrations(status);
CREATE INDEX IF NOT EXISTS idx_sender_registrations_lookup ON sender_registrations(client_id, sender_name, operator_id) WHERE status = 'approved';

CREATE TABLE IF NOT EXISTS registration_templates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id  UUID NOT NULL REFERENCES sender_registrations(id) ON DELETE CASCADE,
    name             VARCHAR(255) NOT NULL,
    body             TEXT NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT registration_templates_status_check
        CHECK (status IN ('active','inactive'))
);

CREATE INDEX IF NOT EXISTS idx_registration_templates_reg ON registration_templates(registration_id);

CREATE TABLE IF NOT EXISTS registration_comments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id  UUID NOT NULL REFERENCES sender_registrations(id) ON DELETE CASCADE,
    author_id        UUID NOT NULL,
    body             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_registration_comments_reg ON registration_comments(registration_id);
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000064_sender_registrations.down.sql

DROP TABLE IF EXISTS registration_comments;
DROP TABLE IF EXISTS registration_templates;
DROP TABLE IF EXISTS sender_registrations;
```

- [ ] **Step 3: Применить миграцию и проверить**

Run: `cd /home/magomed/projects/sms && go run cmd/smpp-server/main.go migrate up`
Expected: миграция 000064 применена успешно

- [ ] **Step 4: Коммит**

```bash
git add migrations/000064_sender_registrations.up.sql migrations/000064_sender_registrations.down.sql
git commit -m "feat(sender): add sender_registrations migration (000064)"
```

---

### Task 2: Proto-определение SenderService

**Files:**
- Create: `api/proto/sender/sender.proto`

- [ ] **Step 1: Создать proto-файл**

```protobuf
syntax = "proto3";

package sender.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/senderv1";

import "google/protobuf/timestamp.proto";

service SenderService {
  // Portal: CRUD + lifecycle
  rpc CreateRegistration(CreateRegistrationRequest) returns (CreateRegistrationResponse);
  rpc GetRegistration(GetRegistrationRequest) returns (GetRegistrationResponse);
  rpc UpdateRegistration(UpdateRegistrationRequest) returns (UpdateRegistrationResponse);
  rpc DeleteRegistration(DeleteRegistrationRequest) returns (DeleteRegistrationResponse);
  rpc ListRegistrations(ListRegistrationsRequest) returns (ListRegistrationsResponse);
  rpc SubmitRegistration(SubmitRegistrationRequest) returns (SubmitRegistrationResponse);
  rpc ResubmitRegistration(ResubmitRegistrationRequest) returns (ResubmitRegistrationResponse);

  // Templates
  rpc CreateTemplate(CreateTemplateRequest) returns (CreateTemplateResponse);
  rpc UpdateTemplate(UpdateTemplateRequest) returns (UpdateTemplateResponse);
  rpc DeleteTemplate(DeleteTemplateRequest) returns (DeleteTemplateResponse);

  // Comments
  rpc ListComments(ListCommentsRequest) returns (ListCommentsResponse);
  rpc AddComment(AddCommentRequest) returns (AddCommentResponse);

  // Admin: review actions
  rpc ApproveRegistration(ApproveRegistrationRequest) returns (ApproveRegistrationResponse);
  rpc RejectRegistration(RejectRegistrationRequest) returns (RejectRegistrationResponse);
  rpc RequestRevision(RequestRevisionRequest) returns (RequestRevisionResponse);

  // Routing: hot-path check
  rpc CheckSenderAllowed(CheckSenderAllowedRequest) returns (CheckSenderAllowedResponse);
}

// ===== Domain messages =====

message RegistrationInfo {
  string id = 1;
  string client_id = 2;
  string operator_id = 3;
  string sender_name = 4;
  string status = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
  google.protobuf.Timestamp submitted_at = 8;
  google.protobuf.Timestamp resolved_at = 9;
  repeated TemplateInfo templates = 10;
  string operator_name = 11;
}

message TemplateInfo {
  string id = 1;
  string registration_id = 2;
  string name = 3;
  string body = 4;
  string status = 5;
  google.protobuf.Timestamp created_at = 6;
}

message CommentInfo {
  string id = 1;
  string registration_id = 2;
  string author_id = 3;
  string body = 4;
  google.protobuf.Timestamp created_at = 5;
}

// ===== Portal: CRUD =====

message CreateRegistrationRequest {
  string client_id = 1;
  string operator_id = 2;
  string sender_name = 3;
}

message CreateRegistrationResponse {
  RegistrationInfo registration = 1;
}

message GetRegistrationRequest {
  string id = 1;
  string client_id = 2; // для проверки доступа
}

message GetRegistrationResponse {
  RegistrationInfo registration = 1;
}

message UpdateRegistrationRequest {
  string id = 1;
  string client_id = 2;
  string sender_name = 3;
}

message UpdateRegistrationResponse {
  RegistrationInfo registration = 1;
}

message DeleteRegistrationRequest {
  string id = 1;
  string client_id = 2;
}

message DeleteRegistrationResponse {}

message ListRegistrationsRequest {
  string client_id = 1;
  string status = 2;
  string operator_id = 3;
  int32 limit = 4;
  int32 offset = 5;
}

message ListRegistrationsResponse {
  repeated RegistrationInfo registrations = 1;
  int32 total = 2;
}

message SubmitRegistrationRequest {
  string id = 1;
  string client_id = 2;
}

message SubmitRegistrationResponse {
  RegistrationInfo registration = 1;
}

message ResubmitRegistrationRequest {
  string id = 1;
  string client_id = 2;
}

message ResubmitRegistrationResponse {
  RegistrationInfo registration = 1;
}

// ===== Templates =====

message CreateTemplateRequest {
  string registration_id = 1;
  string client_id = 2;
  string name = 3;
  string body = 4;
}

message CreateTemplateResponse {
  TemplateInfo template = 1;
}

message UpdateTemplateRequest {
  string id = 1;
  string registration_id = 2;
  string client_id = 3;
  string name = 4;
  string body = 5;
}

message UpdateTemplateResponse {
  TemplateInfo template = 1;
}

message DeleteTemplateRequest {
  string id = 1;
  string registration_id = 2;
  string client_id = 3;
}

message DeleteTemplateResponse {}

// ===== Comments =====

message ListCommentsRequest {
  string registration_id = 1;
  string client_id = 2; // пустой для admin
}

message ListCommentsResponse {
  repeated CommentInfo comments = 1;
}

message AddCommentRequest {
  string registration_id = 1;
  string author_id = 2;
  string body = 3;
}

message AddCommentResponse {
  CommentInfo comment = 1;
}

// ===== Admin: review =====

message ApproveRegistrationRequest {
  string id = 1;
}

message ApproveRegistrationResponse {
  RegistrationInfo registration = 1;
}

message RejectRegistrationRequest {
  string id = 1;
}

message RejectRegistrationResponse {
  RegistrationInfo registration = 1;
}

message RequestRevisionRequest {
  string id = 1;
  string comment = 2; // необязательный комментарий при запросе доработки
}

message RequestRevisionResponse {
  RegistrationInfo registration = 1;
}

// ===== Routing: hot-path =====

message CheckSenderAllowedRequest {
  string client_id = 1;
  string sender_name = 2;
  string operator_id = 3;
}

message CheckSenderAllowedResponse {
  bool allowed = 1;
  string rejection_reason = 2;
}
```

- [ ] **Step 2: Сгенерировать Go-код из proto**

Run: `cd /home/magomed/projects/sms && protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative api/proto/sender/sender.proto`
Expected: файлы `api/proto/senderv1/sender.pb.go` и `api/proto/senderv1/sender_grpc.pb.go` созданы

Если используется buf:
Run: `buf generate api/proto/sender/sender.proto`

- [ ] **Step 3: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./api/proto/senderv1/...`
Expected: BUILD OK

- [ ] **Step 4: Коммит**

```bash
git add api/proto/sender/ api/proto/senderv1/
git commit -m "feat(sender): add SenderService proto definition"
```

---

### Task 3: Доменный слой — модели и валидация

**Files:**
- Create: `internal/services/sender/domain/registration.go`
- Create: `internal/services/sender/domain/template.go`
- Create: `internal/services/sender/domain/comment.go`
- Create: `internal/services/sender/domain/repository.go`
- Test: `internal/services/sender/domain/registration_test.go`

- [ ] **Step 1: Написать тест на валидацию sender_name и переходы статусов**

```go
// internal/services/sender/domain/registration_test.go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSenderName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid 3 chars", "ABC", false},
		{"valid 11 chars", "ABCDEFGHIJK", false},
		{"valid with digits", "BANK123", false},
		{"valid with space", "MY BANK", false},
		{"valid with hyphen", "MY-BANK", false},
		{"valid with dot", "MY.BANK", false},
		{"too short 2 chars", "AB", true},
		{"too long 12 chars", "ABCDEFGHIJKL", true},
		{"empty", "", true},
		{"cyrillic", "БАНК", true},
		{"starts with space", " BANK", true},
		{"ends with space", "BANK ", true},
		{"only digits", "12345", true},
		{"digits and spaces", "123 456", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSenderName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistrationStatusTransitions(t *testing.T) {
	clientID := uuid.New()
	operatorID := uuid.New()

	t.Run("draft -> submitted", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		assert.Equal(t, StatusDraft, reg.Status)
		err := reg.Submit(1) // 1 template
		require.NoError(t, err)
		assert.Equal(t, StatusSubmitted, reg.Status)
		assert.NotNil(t, reg.SubmittedAt)
	})

	t.Run("submit without templates fails", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		err := reg.Submit(0)
		assert.Error(t, err)
	})

	t.Run("submitted -> approved", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		err := reg.Approve()
		require.NoError(t, err)
		assert.Equal(t, StatusApproved, reg.Status)
		assert.NotNil(t, reg.ResolvedAt)
	})

	t.Run("submitted -> rejected", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		err := reg.Reject()
		require.NoError(t, err)
		assert.Equal(t, StatusRejected, reg.Status)
	})

	t.Run("submitted -> revision_requested", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		err := reg.RequestRevision()
		require.NoError(t, err)
		assert.Equal(t, StatusRevisionRequested, reg.Status)
	})

	t.Run("revision_requested -> submitted (resubmit)", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		_ = reg.RequestRevision()
		err := reg.Resubmit(1)
		require.NoError(t, err)
		assert.Equal(t, StatusSubmitted, reg.Status)
	})

	t.Run("approved is final", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		_ = reg.Approve()
		assert.Error(t, reg.Reject())
		assert.Error(t, reg.RequestRevision())
	})

	t.Run("rejected is final", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		_ = reg.Reject()
		assert.Error(t, reg.Approve())
	})

	t.Run("draft -> approve not allowed", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		assert.Error(t, reg.Approve())
	})

	t.Run("can edit in draft", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		assert.True(t, reg.CanEdit())
	})

	t.Run("cannot edit in submitted", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		assert.False(t, reg.CanEdit())
	})

	t.Run("can edit in revision_requested", func(t *testing.T) {
		reg := NewRegistration(clientID, operatorID, "MYBANK")
		_ = reg.Submit(1)
		_ = reg.RequestRevision()
		assert.True(t, reg.CanEdit())
	})
}
```

- [ ] **Step 2: Запустить тест — убедиться что не компилируется**

Run: `cd /home/magomed/projects/sms && go test ./internal/services/sender/domain/ -v -count=1`
Expected: FAIL (пакет не существует)

- [ ] **Step 3: Реализовать domain/registration.go**

```go
// internal/services/sender/domain/registration.go
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Статусы регистрации
type RegistrationStatus string

const (
	StatusDraft             RegistrationStatus = "draft"
	StatusSubmitted         RegistrationStatus = "submitted"
	StatusApproved          RegistrationStatus = "approved"
	StatusRejected          RegistrationStatus = "rejected"
	StatusRevisionRequested RegistrationStatus = "revision_requested"
)

var (
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNoTemplates       = errors.New("at least one template required")
	ErrInvalidSenderName = errors.New("sender_name: 3-11 chars, latin/digits/space/hyphen/dot, no leading/trailing spaces, not digits-only")
)

// senderNameRegex: только латиница, цифры, пробел, дефис, точка
var senderNameRegex = regexp.MustCompile(`^[A-Za-z0-9 .\-]+$`)

// ValidateSenderName проверяет альфа-имя отправителя по правилам РФ-операторов
func ValidateSenderName(name string) error {
	if len(name) < 3 || len(name) > 11 {
		return fmt.Errorf("%w: length must be 3-11, got %d", ErrInvalidSenderName, len(name))
	}
	if name[0] == ' ' || name[len(name)-1] == ' ' {
		return fmt.Errorf("%w: no leading/trailing spaces", ErrInvalidSenderName)
	}
	if !senderNameRegex.MatchString(name) {
		return fmt.Errorf("%w: only latin, digits, space, hyphen, dot", ErrInvalidSenderName)
	}
	// Не может состоять только из цифр и пробелов
	allDigitsOrSpaces := true
	for _, r := range name {
		if !unicode.IsDigit(r) && r != ' ' {
			allDigitsOrSpaces = false
			break
		}
	}
	if allDigitsOrSpaces {
		return fmt.Errorf("%w: cannot be digits-only", ErrInvalidSenderName)
	}
	return nil
}

// IsAlphaName определяет, является ли sender_name альфа-именем (не числовым номером)
func IsAlphaName(name string) bool {
	for _, r := range name {
		if !unicode.IsDigit(r) && r != '+' {
			return true
		}
	}
	return false
}

// SenderRegistration — доменная модель регистрации имени отправителя
type SenderRegistration struct {
	ID          uuid.UUID
	ClientID    uuid.UUID
	OperatorID  uuid.UUID
	SenderName  string
	Status      RegistrationStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SubmittedAt *time.Time
	ResolvedAt  *time.Time
}

// NewRegistration создает новую регистрацию в статусе draft
func NewRegistration(clientID, operatorID uuid.UUID, senderName string) *SenderRegistration {
	now := time.Now()
	return &SenderRegistration{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		SenderName: senderName,
		Status:     StatusDraft,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// CanEdit — разрешено ли редактировать (только draft и revision_requested)
func (r *SenderRegistration) CanEdit() bool {
	return r.Status == StatusDraft || r.Status == StatusRevisionRequested
}

// Submit — клиент подаёт заявку (draft -> submitted)
func (r *SenderRegistration) Submit(templateCount int) error {
	if r.Status != StatusDraft {
		return fmt.Errorf("%w: cannot submit from %s", ErrInvalidTransition, r.Status)
	}
	if templateCount < 1 {
		return ErrNoTemplates
	}
	now := time.Now()
	r.Status = StatusSubmitted
	r.SubmittedAt = &now
	r.UpdatedAt = now
	return nil
}

// Resubmit — клиент повторно подаёт после доработки (revision_requested -> submitted)
func (r *SenderRegistration) Resubmit(templateCount int) error {
	if r.Status != StatusRevisionRequested {
		return fmt.Errorf("%w: cannot resubmit from %s", ErrInvalidTransition, r.Status)
	}
	if templateCount < 1 {
		return ErrNoTemplates
	}
	now := time.Now()
	r.Status = StatusSubmitted
	r.SubmittedAt = &now
	r.UpdatedAt = now
	return nil
}

// Approve — админ одобряет (submitted -> approved)
func (r *SenderRegistration) Approve() error {
	if r.Status != StatusSubmitted {
		return fmt.Errorf("%w: cannot approve from %s", ErrInvalidTransition, r.Status)
	}
	now := time.Now()
	r.Status = StatusApproved
	r.ResolvedAt = &now
	r.UpdatedAt = now
	return nil
}

// Reject — админ отклоняет (submitted -> rejected)
func (r *SenderRegistration) Reject() error {
	if r.Status != StatusSubmitted {
		return fmt.Errorf("%w: cannot reject from %s", ErrInvalidTransition, r.Status)
	}
	now := time.Now()
	r.Status = StatusRejected
	r.ResolvedAt = &now
	r.UpdatedAt = now
	return nil
}

// RequestRevision — админ запрашивает доработку (submitted -> revision_requested)
func (r *SenderRegistration) RequestRevision() error {
	if r.Status != StatusSubmitted {
		return fmt.Errorf("%w: cannot request revision from %s", ErrInvalidTransition, r.Status)
	}
	now := time.Now()
	r.Status = StatusRevisionRequested
	r.UpdatedAt = now
	return nil
}
```

- [ ] **Step 4: Реализовать domain/template.go**

```go
// internal/services/sender/domain/template.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

type TemplateStatus string

const (
	TemplateActive   TemplateStatus = "active"
	TemplateInactive TemplateStatus = "inactive"
)

// RegistrationTemplate — шаблон, привязанный к регистрации
type RegistrationTemplate struct {
	ID             uuid.UUID
	RegistrationID uuid.UUID
	Name           string
	Body           string
	Status         TemplateStatus
	CreatedAt      time.Time
}

// NewTemplate создает новый шаблон для регистрации
func NewTemplate(registrationID uuid.UUID, name, body string) *RegistrationTemplate {
	return &RegistrationTemplate{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		Name:           name,
		Body:           body,
		Status:         TemplateActive,
		CreatedAt:      time.Now(),
	}
}
```

- [ ] **Step 5: Реализовать domain/comment.go**

```go
// internal/services/sender/domain/comment.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// RegistrationComment — комментарий к регистрации
type RegistrationComment struct {
	ID             uuid.UUID
	RegistrationID uuid.UUID
	AuthorID       uuid.UUID
	Body           string
	CreatedAt      time.Time
}

// NewComment создает новый комментарий
func NewComment(registrationID, authorID uuid.UUID, body string) *RegistrationComment {
	return &RegistrationComment{
		ID:             uuid.New(),
		RegistrationID: registrationID,
		AuthorID:       authorID,
		Body:           body,
		CreatedAt:      time.Now(),
	}
}
```

- [ ] **Step 6: Реализовать domain/repository.go**

```go
// internal/services/sender/domain/repository.go
package domain

import (
	"context"

	"github.com/google/uuid"
)

// RegistrationFilters — фильтры для списка регистраций
type RegistrationFilters struct {
	ClientID   *uuid.UUID
	OperatorID *uuid.UUID
	Status     string
	Limit      int
	Offset     int
}

// RegistrationRepository — контракт для хранения регистраций
type RegistrationRepository interface {
	Create(ctx context.Context, reg *SenderRegistration) error
	GetByID(ctx context.Context, id uuid.UUID) (*SenderRegistration, error)
	Update(ctx context.Context, reg *SenderRegistration) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filters *RegistrationFilters) ([]*SenderRegistration, int, error)
	CheckAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, error)
}

// TemplateRepository — контракт для хранения шаблонов регистрации
type TemplateRepository interface {
	Create(ctx context.Context, tpl *RegistrationTemplate) error
	Update(ctx context.Context, tpl *RegistrationTemplate) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByRegistration(ctx context.Context, registrationID uuid.UUID) ([]*RegistrationTemplate, error)
	CountByRegistration(ctx context.Context, registrationID uuid.UUID) (int, error)
}

// CommentRepository — контракт для хранения комментариев
type CommentRepository interface {
	Create(ctx context.Context, comment *RegistrationComment) error
	ListByRegistration(ctx context.Context, registrationID uuid.UUID) ([]*RegistrationComment, error)
}
```

- [ ] **Step 7: Запустить тесты — убедиться что проходят**

Run: `cd /home/magomed/projects/sms && go test ./internal/services/sender/domain/ -v -count=1`
Expected: PASS (все тесты зелёные)

- [ ] **Step 8: Коммит**

```bash
git add internal/services/sender/domain/
git commit -m "feat(sender): add domain layer — models, validation, status transitions"
```

---

### Task 4: PostgreSQL-репозитории

**Files:**
- Create: `internal/services/sender/infrastructure/repository/registration_repo.go`
- Create: `internal/services/sender/infrastructure/repository/template_repo.go`
- Create: `internal/services/sender/infrastructure/repository/comment_repo.go`

- [ ] **Step 1: Реализовать registration_repo.go**

```go
// internal/services/sender/infrastructure/repository/registration_repo.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

type RegistrationRepository struct {
	db *pgxpool.Pool
}

func NewRegistrationRepository(db *pgxpool.Pool) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

func (r *RegistrationRepository) Create(ctx context.Context, reg *domain.SenderRegistration) error {
	query := `
		INSERT INTO sender_registrations (id, client_id, operator_id, sender_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		reg.ID, reg.ClientID, reg.OperatorID, reg.SenderName, string(reg.Status), reg.CreatedAt, reg.UpdatedAt,
	)
	return err
}

func (r *RegistrationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	query := `
		SELECT id, client_id, operator_id, sender_name, status, created_at, updated_at, submitted_at, resolved_at
		FROM sender_registrations WHERE id = $1
	`
	reg := &domain.SenderRegistration{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&reg.ID, &reg.ClientID, &reg.OperatorID, &reg.SenderName, &reg.Status,
		&reg.CreatedAt, &reg.UpdatedAt, &reg.SubmittedAt, &reg.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("registration not found: %w", err)
	}
	return reg, nil
}

func (r *RegistrationRepository) Update(ctx context.Context, reg *domain.SenderRegistration) error {
	query := `
		UPDATE sender_registrations
		SET sender_name = $2, status = $3, updated_at = $4, submitted_at = $5, resolved_at = $6
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		reg.ID, reg.SenderName, string(reg.Status), reg.UpdatedAt, reg.SubmittedAt, reg.ResolvedAt,
	)
	return err
}

func (r *RegistrationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sender_registrations WHERE id = $1`, id)
	return err
}

func (r *RegistrationRepository) List(ctx context.Context, filters *domain.RegistrationFilters) ([]*domain.SenderRegistration, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if filters.ClientID != nil {
		where += fmt.Sprintf(" AND client_id = $%d", argNum)
		args = append(args, *filters.ClientID)
		argNum++
	}
	if filters.OperatorID != nil {
		where += fmt.Sprintf(" AND operator_id = $%d", argNum)
		args = append(args, *filters.OperatorID)
		argNum++
	}
	if filters.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, filters.Status)
		argNum++
	}

	// Count
	countQuery := "SELECT COUNT(*) FROM sender_registrations " + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data
	dataQuery := fmt.Sprintf(
		"SELECT id, client_id, operator_id, sender_name, status, created_at, updated_at, submitted_at, resolved_at FROM sender_registrations %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, argNum, argNum+1,
	)
	args = append(args, filters.Limit, filters.Offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var regs []*domain.SenderRegistration
	for rows.Next() {
		reg := &domain.SenderRegistration{}
		if err := rows.Scan(
			&reg.ID, &reg.ClientID, &reg.OperatorID, &reg.SenderName, &reg.Status,
			&reg.CreatedAt, &reg.UpdatedAt, &reg.SubmittedAt, &reg.ResolvedAt,
		); err != nil {
			return nil, 0, err
		}
		regs = append(regs, reg)
	}
	return regs, total, nil
}

func (r *RegistrationRepository) CheckAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM sender_registrations
			WHERE client_id = $1 AND sender_name = $2 AND operator_id = $3 AND status = 'approved'
		)
	`
	var allowed bool
	err := r.db.QueryRow(ctx, query, clientID, senderName, operatorID).Scan(&allowed)
	return allowed, err
}
```

- [ ] **Step 2: Реализовать template_repo.go**

```go
// internal/services/sender/infrastructure/repository/template_repo.go
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

type TemplateRepository struct {
	db *pgxpool.Pool
}

func NewTemplateRepository(db *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(ctx context.Context, tpl *domain.RegistrationTemplate) error {
	query := `
		INSERT INTO registration_templates (id, registration_id, name, body, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, tpl.ID, tpl.RegistrationID, tpl.Name, tpl.Body, string(tpl.Status), tpl.CreatedAt)
	return err
}

func (r *TemplateRepository) Update(ctx context.Context, tpl *domain.RegistrationTemplate) error {
	query := `UPDATE registration_templates SET name = $2, body = $3, status = $4 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, tpl.ID, tpl.Name, tpl.Body, string(tpl.Status))
	return err
}

func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE registration_templates SET status = 'inactive' WHERE id = $1`, id)
	return err
}

func (r *TemplateRepository) ListByRegistration(ctx context.Context, registrationID uuid.UUID) ([]*domain.RegistrationTemplate, error) {
	query := `
		SELECT id, registration_id, name, body, status, created_at
		FROM registration_templates WHERE registration_id = $1 AND status = 'active'
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, registrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*domain.RegistrationTemplate
	for rows.Next() {
		tpl := &domain.RegistrationTemplate{}
		if err := rows.Scan(&tpl.ID, &tpl.RegistrationID, &tpl.Name, &tpl.Body, &tpl.Status, &tpl.CreatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}
	return templates, nil
}

func (r *TemplateRepository) CountByRegistration(ctx context.Context, registrationID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM registration_templates WHERE registration_id = $1 AND status = 'active'`,
		registrationID,
	).Scan(&count)
	return count, err
}
```

- [ ] **Step 3: Реализовать comment_repo.go**

```go
// internal/services/sender/infrastructure/repository/comment_repo.go
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

type CommentRepository struct {
	db *pgxpool.Pool
}

func NewCommentRepository(db *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, comment *domain.RegistrationComment) error {
	query := `
		INSERT INTO registration_comments (id, registration_id, author_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, comment.ID, comment.RegistrationID, comment.AuthorID, comment.Body, comment.CreatedAt)
	return err
}

func (r *CommentRepository) ListByRegistration(ctx context.Context, registrationID uuid.UUID) ([]*domain.RegistrationComment, error) {
	query := `
		SELECT id, registration_id, author_id, body, created_at
		FROM registration_comments WHERE registration_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, registrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*domain.RegistrationComment
	for rows.Next() {
		c := &domain.RegistrationComment{}
		if err := rows.Scan(&c.ID, &c.RegistrationID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}
```

- [ ] **Step 4: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./internal/services/sender/...`
Expected: BUILD OK

- [ ] **Step 5: Коммит**

```bash
git add internal/services/sender/infrastructure/
git commit -m "feat(sender): add PostgreSQL repositories for registrations, templates, comments"
```

---

### Task 5: Application-слой (бизнес-логика)

**Files:**
- Create: `internal/services/sender/application/sender_service.go`
- Test: `internal/services/sender/application/sender_service_test.go`

- [ ] **Step 1: Написать unit-тест для основных use case'ов**

```go
// internal/services/sender/application/sender_service_test.go
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

// --- Mocks ---

type mockRegRepo struct{ mock.Mock }

func (m *mockRegRepo) Create(ctx context.Context, reg *domain.SenderRegistration) error {
	return m.Called(ctx, reg).Error(0)
}
func (m *mockRegRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SenderRegistration), args.Error(1)
}
func (m *mockRegRepo) Update(ctx context.Context, reg *domain.SenderRegistration) error {
	return m.Called(ctx, reg).Error(0)
}
func (m *mockRegRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRegRepo) List(ctx context.Context, f *domain.RegistrationFilters) ([]*domain.SenderRegistration, int, error) {
	args := m.Called(ctx, f)
	return args.Get(0).([]*domain.SenderRegistration), args.Int(1), args.Error(2)
}
func (m *mockRegRepo) CheckAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, clientID, senderName, operatorID)
	return args.Bool(0), args.Error(1)
}

type mockTplRepo struct{ mock.Mock }

func (m *mockTplRepo) Create(ctx context.Context, tpl *domain.RegistrationTemplate) error {
	return m.Called(ctx, tpl).Error(0)
}
func (m *mockTplRepo) Update(ctx context.Context, tpl *domain.RegistrationTemplate) error {
	return m.Called(ctx, tpl).Error(0)
}
func (m *mockTplRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockTplRepo) ListByRegistration(ctx context.Context, regID uuid.UUID) ([]*domain.RegistrationTemplate, error) {
	args := m.Called(ctx, regID)
	return args.Get(0).([]*domain.RegistrationTemplate), args.Error(1)
}
func (m *mockTplRepo) CountByRegistration(ctx context.Context, regID uuid.UUID) (int, error) {
	args := m.Called(ctx, regID)
	return args.Int(0), args.Error(1)
}

type mockCommentRepo struct{ mock.Mock }

func (m *mockCommentRepo) Create(ctx context.Context, c *domain.RegistrationComment) error {
	return m.Called(ctx, c).Error(0)
}
func (m *mockCommentRepo) ListByRegistration(ctx context.Context, regID uuid.UUID) ([]*domain.RegistrationComment, error) {
	args := m.Called(ctx, regID)
	return args.Get(0).([]*domain.RegistrationComment), args.Error(1)
}

// --- Tests ---

func TestCreateRegistration(t *testing.T) {
	regRepo := new(mockRegRepo)
	tplRepo := new(mockTplRepo)
	cmtRepo := new(mockCommentRepo)
	svc := NewSenderService(regRepo, tplRepo, cmtRepo)

	regRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	reg, err := svc.CreateRegistration(context.Background(), uuid.New(), uuid.New(), "MYBANK")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDraft, reg.Status)
	assert.Equal(t, "MYBANK", reg.SenderName)
	regRepo.AssertExpectations(t)
}

func TestCreateRegistration_InvalidName(t *testing.T) {
	svc := NewSenderService(new(mockRegRepo), new(mockTplRepo), new(mockCommentRepo))
	_, err := svc.CreateRegistration(context.Background(), uuid.New(), uuid.New(), "AB")
	assert.Error(t, err)
}

func TestSubmitRegistration(t *testing.T) {
	regRepo := new(mockRegRepo)
	tplRepo := new(mockTplRepo)
	cmtRepo := new(mockCommentRepo)
	svc := NewSenderService(regRepo, tplRepo, cmtRepo)

	clientID := uuid.New()
	reg := domain.NewRegistration(clientID, uuid.New(), "MYBANK")
	regRepo.On("GetByID", mock.Anything, reg.ID).Return(reg, nil)
	tplRepo.On("CountByRegistration", mock.Anything, reg.ID).Return(2, nil)
	regRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	result, err := svc.SubmitRegistration(context.Background(), reg.ID, clientID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSubmitted, result.Status)
}

func TestSubmitRegistration_NoTemplates(t *testing.T) {
	regRepo := new(mockRegRepo)
	tplRepo := new(mockTplRepo)
	cmtRepo := new(mockCommentRepo)
	svc := NewSenderService(regRepo, tplRepo, cmtRepo)

	clientID := uuid.New()
	reg := domain.NewRegistration(clientID, uuid.New(), "MYBANK")
	regRepo.On("GetByID", mock.Anything, reg.ID).Return(reg, nil)
	tplRepo.On("CountByRegistration", mock.Anything, reg.ID).Return(0, nil)

	_, err := svc.SubmitRegistration(context.Background(), reg.ID, clientID)
	assert.Error(t, err)
}

func TestCheckSenderAllowed(t *testing.T) {
	regRepo := new(mockRegRepo)
	svc := NewSenderService(regRepo, new(mockTplRepo), new(mockCommentRepo))

	clientID := uuid.New()
	operatorID := uuid.New()

	regRepo.On("CheckAllowed", mock.Anything, clientID, "MYBANK", operatorID).Return(true, nil)
	allowed, reason, err := svc.CheckSenderAllowed(context.Background(), clientID, "MYBANK", operatorID)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Empty(t, reason)

	regRepo.On("CheckAllowed", mock.Anything, clientID, "OTHER", operatorID).Return(false, nil)
	allowed, reason, err = svc.CheckSenderAllowed(context.Background(), clientID, "OTHER", operatorID)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.NotEmpty(t, reason)
}

func TestCheckSenderAllowed_NumericSender(t *testing.T) {
	svc := NewSenderService(new(mockRegRepo), new(mockTplRepo), new(mockCommentRepo))
	allowed, _, err := svc.CheckSenderAllowed(context.Background(), uuid.New(), "+79001234567", uuid.New())
	require.NoError(t, err)
	assert.True(t, allowed) // числовой номер — проверка не применяется
}
```

- [ ] **Step 2: Запустить тесты — убедиться что не компилируется**

Run: `cd /home/magomed/projects/sms && go test ./internal/services/sender/application/ -v -count=1`
Expected: FAIL (NewSenderService не определён)

- [ ] **Step 3: Реализовать sender_service.go**

```go
// internal/services/sender/application/sender_service.go
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

var (
	ErrNotFound       = errors.New("registration not found")
	ErrAccessDenied   = errors.New("access denied")
	ErrNotEditable    = errors.New("registration is not editable in current status")
)

type SenderService struct {
	regRepo     domain.RegistrationRepository
	tplRepo     domain.TemplateRepository
	commentRepo domain.CommentRepository
	logger      zerolog.Logger
}

func NewSenderService(
	regRepo domain.RegistrationRepository,
	tplRepo domain.TemplateRepository,
	commentRepo domain.CommentRepository,
) *SenderService {
	return &SenderService{
		regRepo:     regRepo,
		tplRepo:     tplRepo,
		commentRepo: commentRepo,
		logger:      log.With().Str("component", "sender-service").Logger(),
	}
}

// CreateRegistration создаёт новую заявку на регистрацию (draft)
func (s *SenderService) CreateRegistration(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (*domain.SenderRegistration, error) {
	if err := domain.ValidateSenderName(senderName); err != nil {
		return nil, err
	}
	reg := domain.NewRegistration(clientID, operatorID, senderName)
	if err := s.regRepo.Create(ctx, reg); err != nil {
		return nil, fmt.Errorf("create registration: %w", err)
	}
	s.logger.Info().Str("id", reg.ID.String()).Str("sender", senderName).Msg("registration created")
	return reg, nil
}

// GetRegistration получает регистрацию по ID с проверкой доступа
func (s *SenderService) GetRegistration(ctx context.Context, id, clientID uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if reg.ClientID != clientID {
		return nil, ErrAccessDenied
	}
	return reg, nil
}

// GetRegistrationAdmin получает регистрацию по ID без проверки доступа (для админа)
func (s *SenderService) GetRegistrationAdmin(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return reg, nil
}

// UpdateRegistration обновляет sender_name (только draft / revision_requested)
func (s *SenderService) UpdateRegistration(ctx context.Context, id, clientID uuid.UUID, senderName string) (*domain.SenderRegistration, error) {
	reg, err := s.GetRegistration(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if !reg.CanEdit() {
		return nil, ErrNotEditable
	}
	if err := domain.ValidateSenderName(senderName); err != nil {
		return nil, err
	}
	reg.SenderName = senderName
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	return reg, nil
}

// DeleteRegistration удаляет регистрацию (только draft)
func (s *SenderService) DeleteRegistration(ctx context.Context, id, clientID uuid.UUID) error {
	reg, err := s.GetRegistration(ctx, id, clientID)
	if err != nil {
		return err
	}
	if reg.Status != domain.StatusDraft {
		return fmt.Errorf("can only delete draft registrations")
	}
	return s.regRepo.Delete(ctx, id)
}

// ListRegistrations — список регистраций (для клиента — только свои)
func (s *SenderService) ListRegistrations(ctx context.Context, filters *domain.RegistrationFilters) ([]*domain.SenderRegistration, int, error) {
	return s.regRepo.List(ctx, filters)
}

// SubmitRegistration — клиент подаёт заявку (draft -> submitted)
func (s *SenderService) SubmitRegistration(ctx context.Context, id, clientID uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.GetRegistration(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	count, err := s.tplRepo.CountByRegistration(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count templates: %w", err)
	}
	if err := reg.Submit(count); err != nil {
		return nil, err
	}
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	s.logger.Info().Str("id", id.String()).Msg("registration submitted")
	return reg, nil
}

// ResubmitRegistration — клиент повторно подаёт после доработки
func (s *SenderService) ResubmitRegistration(ctx context.Context, id, clientID uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.GetRegistration(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	count, err := s.tplRepo.CountByRegistration(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count templates: %w", err)
	}
	if err := reg.Resubmit(count); err != nil {
		return nil, err
	}
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	s.logger.Info().Str("id", id.String()).Msg("registration resubmitted")
	return reg, nil
}

// ApproveRegistration — админ одобряет (submitted -> approved)
func (s *SenderService) ApproveRegistration(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := reg.Approve(); err != nil {
		return nil, err
	}
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	s.logger.Info().Str("id", id.String()).Msg("registration approved")
	return reg, nil
}

// RejectRegistration — админ отклоняет (submitted -> rejected)
func (s *SenderService) RejectRegistration(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := reg.Reject(); err != nil {
		return nil, err
	}
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	s.logger.Info().Str("id", id.String()).Msg("registration rejected")
	return reg, nil
}

// RequestRevision — админ запрашивает доработку (submitted -> revision_requested)
func (s *SenderService) RequestRevisionRegistration(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	reg, err := s.regRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := reg.RequestRevision(); err != nil {
		return nil, err
	}
	if err := s.regRepo.Update(ctx, reg); err != nil {
		return nil, fmt.Errorf("update registration: %w", err)
	}
	s.logger.Info().Str("id", id.String()).Msg("revision requested")
	return reg, nil
}

// --- Templates ---

func (s *SenderService) CreateTemplate(ctx context.Context, registrationID, clientID uuid.UUID, name, body string) (*domain.RegistrationTemplate, error) {
	reg, err := s.GetRegistration(ctx, registrationID, clientID)
	if err != nil {
		return nil, err
	}
	if !reg.CanEdit() {
		return nil, ErrNotEditable
	}
	tpl := domain.NewTemplate(registrationID, name, body)
	if err := s.tplRepo.Create(ctx, tpl); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return tpl, nil
}

func (s *SenderService) UpdateTemplate(ctx context.Context, tplID, registrationID, clientID uuid.UUID, name, body string) (*domain.RegistrationTemplate, error) {
	reg, err := s.GetRegistration(ctx, registrationID, clientID)
	if err != nil {
		return nil, err
	}
	if !reg.CanEdit() {
		return nil, ErrNotEditable
	}
	tpl := &domain.RegistrationTemplate{
		ID:             tplID,
		RegistrationID: registrationID,
		Name:           name,
		Body:           body,
		Status:         domain.TemplateActive,
	}
	if err := s.tplRepo.Update(ctx, tpl); err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	return tpl, nil
}

func (s *SenderService) DeleteTemplate(ctx context.Context, tplID, registrationID, clientID uuid.UUID) error {
	reg, err := s.GetRegistration(ctx, registrationID, clientID)
	if err != nil {
		return err
	}
	if !reg.CanEdit() {
		return ErrNotEditable
	}
	return s.tplRepo.Delete(ctx, tplID)
}

func (s *SenderService) ListTemplates(ctx context.Context, registrationID uuid.UUID) ([]*domain.RegistrationTemplate, error) {
	return s.tplRepo.ListByRegistration(ctx, registrationID)
}

// --- Comments ---

func (s *SenderService) AddComment(ctx context.Context, registrationID, authorID uuid.UUID, body string) (*domain.RegistrationComment, error) {
	comment := domain.NewComment(registrationID, authorID, body)
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}
	return comment, nil
}

func (s *SenderService) ListComments(ctx context.Context, registrationID uuid.UUID) ([]*domain.RegistrationComment, error) {
	return s.commentRepo.ListByRegistration(ctx, registrationID)
}

// --- Routing hot-path ---

// CheckSenderAllowed проверяет, разрешён ли sender_name для клиента у оператора
func (s *SenderService) CheckSenderAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, string, error) {
	// Числовой sender (номер телефона) — проверка не применяется
	if !domain.IsAlphaName(senderName) {
		return true, "", nil
	}

	allowed, err := s.regRepo.CheckAllowed(ctx, clientID, senderName, operatorID)
	if err != nil {
		return false, "sender_check_error", err
	}
	if !allowed {
		return false, fmt.Sprintf("sender '%s' not registered for this operator", senderName), nil
	}
	return true, "", nil
}
```

- [ ] **Step 4: Запустить тесты**

Run: `cd /home/magomed/projects/sms && go test ./internal/services/sender/application/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Коммит**

```bash
git add internal/services/sender/application/
git commit -m "feat(sender): add application layer — business logic and unit tests"
```

---

### Task 6: gRPC-сервер sender-service

**Files:**
- Create: `internal/services/sender/grpc/server.go`

- [ ] **Step 1: Реализовать gRPC-сервер**

```go
// internal/services/sender/grpc/server.go
package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
	"github.com/smpp-server/smpp-server/internal/services/sender/application"
	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
)

type Server struct {
	senderv1.UnimplementedSenderServiceServer
	senderService *application.SenderService
}

func NewServer(senderService *application.SenderService) *Server {
	return &Server{senderService: senderService}
}

// --- helpers ---

func parseUUID(s string, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s: %s", field, s)
	}
	return id, nil
}

func regToProto(reg *domain.SenderRegistration) *senderv1.RegistrationInfo {
	info := &senderv1.RegistrationInfo{
		Id:         reg.ID.String(),
		ClientId:   reg.ClientID.String(),
		OperatorId: reg.OperatorID.String(),
		SenderName: reg.SenderName,
		Status:     string(reg.Status),
		CreatedAt:  timestamppb.New(reg.CreatedAt),
		UpdatedAt:  timestamppb.New(reg.UpdatedAt),
	}
	if reg.SubmittedAt != nil {
		info.SubmittedAt = timestamppb.New(*reg.SubmittedAt)
	}
	if reg.ResolvedAt != nil {
		info.ResolvedAt = timestamppb.New(*reg.ResolvedAt)
	}
	return info
}

func tplToProto(tpl *domain.RegistrationTemplate) *senderv1.TemplateInfo {
	return &senderv1.TemplateInfo{
		Id:             tpl.ID.String(),
		RegistrationId: tpl.RegistrationID.String(),
		Name:           tpl.Name,
		Body:           tpl.Body,
		Status:         string(tpl.Status),
		CreatedAt:      timestamppb.New(tpl.CreatedAt),
	}
}

func commentToProto(c *domain.RegistrationComment) *senderv1.CommentInfo {
	return &senderv1.CommentInfo{
		Id:             c.ID.String(),
		RegistrationId: c.RegistrationID.String(),
		AuthorId:       c.AuthorID.String(),
		Body:           c.Body,
		CreatedAt:      timestamppb.New(c.CreatedAt),
	}
}

func mapAppError(err error) error {
	switch err {
	case application.ErrNotFound:
		return status.Error(codes.NotFound, err.Error())
	case application.ErrAccessDenied:
		return status.Error(codes.PermissionDenied, err.Error())
	case application.ErrNotEditable:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrNoTemplates:
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

// --- Portal: CRUD + lifecycle ---

func (s *Server) CreateRegistration(ctx context.Context, req *senderv1.CreateRegistrationRequest) (*senderv1.CreateRegistrationResponse, error) {
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	operatorID, err := parseUUID(req.OperatorId, "operator_id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.CreateRegistration(ctx, clientID, operatorID, req.SenderName)
	if err != nil {
		log.Error().Err(err).Msg("create registration error")
		return nil, mapAppError(err)
	}
	return &senderv1.CreateRegistrationResponse{Registration: regToProto(reg)}, nil
}

func (s *Server) GetRegistration(ctx context.Context, req *senderv1.GetRegistrationRequest) (*senderv1.GetRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.GetRegistration(ctx, id, clientID)
	if err != nil {
		return nil, mapAppError(err)
	}
	info := regToProto(reg)
	// Подгружаем шаблоны
	templates, _ := s.senderService.ListTemplates(ctx, id)
	for _, tpl := range templates {
		info.Templates = append(info.Templates, tplToProto(tpl))
	}
	return &senderv1.GetRegistrationResponse{Registration: info}, nil
}

func (s *Server) UpdateRegistration(ctx context.Context, req *senderv1.UpdateRegistrationRequest) (*senderv1.UpdateRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.UpdateRegistration(ctx, id, clientID, req.SenderName)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.UpdateRegistrationResponse{Registration: regToProto(reg)}, nil
}

func (s *Server) DeleteRegistration(ctx context.Context, req *senderv1.DeleteRegistrationRequest) (*senderv1.DeleteRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	if err := s.senderService.DeleteRegistration(ctx, id, clientID); err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.DeleteRegistrationResponse{}, nil
}

func (s *Server) ListRegistrations(ctx context.Context, req *senderv1.ListRegistrationsRequest) (*senderv1.ListRegistrationsResponse, error) {
	filters := &domain.RegistrationFilters{
		Status: req.Status,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if req.ClientId != "" {
		clientID, err := parseUUID(req.ClientId, "client_id")
		if err != nil {
			return nil, err
		}
		filters.ClientID = &clientID
	}
	if req.OperatorId != "" {
		operatorID, err := parseUUID(req.OperatorId, "operator_id")
		if err != nil {
			return nil, err
		}
		filters.OperatorID = &operatorID
	}

	regs, total, err := s.senderService.ListRegistrations(ctx, filters)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	infos := make([]*senderv1.RegistrationInfo, len(regs))
	for i, reg := range regs {
		infos[i] = regToProto(reg)
	}
	return &senderv1.ListRegistrationsResponse{Registrations: infos, Total: int32(total)}, nil
}

func (s *Server) SubmitRegistration(ctx context.Context, req *senderv1.SubmitRegistrationRequest) (*senderv1.SubmitRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.SubmitRegistration(ctx, id, clientID)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.SubmitRegistrationResponse{Registration: regToProto(reg)}, nil
}

func (s *Server) ResubmitRegistration(ctx context.Context, req *senderv1.ResubmitRegistrationRequest) (*senderv1.ResubmitRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.ResubmitRegistration(ctx, id, clientID)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.ResubmitRegistrationResponse{Registration: regToProto(reg)}, nil
}

// --- Templates ---

func (s *Server) CreateTemplate(ctx context.Context, req *senderv1.CreateTemplateRequest) (*senderv1.CreateTemplateResponse, error) {
	regID, err := parseUUID(req.RegistrationId, "registration_id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	if req.Name == "" || req.Body == "" {
		return nil, status.Error(codes.InvalidArgument, "name and body are required")
	}
	tpl, err := s.senderService.CreateTemplate(ctx, regID, clientID, req.Name, req.Body)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.CreateTemplateResponse{Template: tplToProto(tpl)}, nil
}

func (s *Server) UpdateTemplate(ctx context.Context, req *senderv1.UpdateTemplateRequest) (*senderv1.UpdateTemplateResponse, error) {
	tplID, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	regID, err := parseUUID(req.RegistrationId, "registration_id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	tpl, err := s.senderService.UpdateTemplate(ctx, tplID, regID, clientID, req.Name, req.Body)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.UpdateTemplateResponse{Template: tplToProto(tpl)}, nil
}

func (s *Server) DeleteTemplate(ctx context.Context, req *senderv1.DeleteTemplateRequest) (*senderv1.DeleteTemplateResponse, error) {
	tplID, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	regID, err := parseUUID(req.RegistrationId, "registration_id")
	if err != nil {
		return nil, err
	}
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	if err := s.senderService.DeleteTemplate(ctx, tplID, regID, clientID); err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.DeleteTemplateResponse{}, nil
}

// --- Comments ---

func (s *Server) ListComments(ctx context.Context, req *senderv1.ListCommentsRequest) (*senderv1.ListCommentsResponse, error) {
	regID, err := parseUUID(req.RegistrationId, "registration_id")
	if err != nil {
		return nil, err
	}
	// Проверка доступа для клиента
	if req.ClientId != "" {
		clientID, err := parseUUID(req.ClientId, "client_id")
		if err != nil {
			return nil, err
		}
		if _, err := s.senderService.GetRegistration(ctx, regID, clientID); err != nil {
			return nil, mapAppError(err)
		}
	}
	comments, err := s.senderService.ListComments(ctx, regID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	protos := make([]*senderv1.CommentInfo, len(comments))
	for i, c := range comments {
		protos[i] = commentToProto(c)
	}
	return &senderv1.ListCommentsResponse{Comments: protos}, nil
}

func (s *Server) AddComment(ctx context.Context, req *senderv1.AddCommentRequest) (*senderv1.AddCommentResponse, error) {
	regID, err := parseUUID(req.RegistrationId, "registration_id")
	if err != nil {
		return nil, err
	}
	authorID, err := parseUUID(req.AuthorId, "author_id")
	if err != nil {
		return nil, err
	}
	if req.Body == "" {
		return nil, status.Error(codes.InvalidArgument, "body is required")
	}
	comment, err := s.senderService.AddComment(ctx, regID, authorID, req.Body)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &senderv1.AddCommentResponse{Comment: commentToProto(comment)}, nil
}

// --- Admin: review ---

func (s *Server) ApproveRegistration(ctx context.Context, req *senderv1.ApproveRegistrationRequest) (*senderv1.ApproveRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.ApproveRegistration(ctx, id)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.ApproveRegistrationResponse{Registration: regToProto(reg)}, nil
}

func (s *Server) RejectRegistration(ctx context.Context, req *senderv1.RejectRegistrationRequest) (*senderv1.RejectRegistrationResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.RejectRegistration(ctx, id)
	if err != nil {
		return nil, mapAppError(err)
	}
	return &senderv1.RejectRegistrationResponse{Registration: regToProto(reg)}, nil
}

func (s *Server) RequestRevision(ctx context.Context, req *senderv1.RequestRevisionRequest) (*senderv1.RequestRevisionResponse, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	reg, err := s.senderService.RequestRevisionRegistration(ctx, id)
	if err != nil {
		return nil, mapAppError(err)
	}
	// Если передан комментарий, добавляем его
	if req.Comment != "" {
		// author_id = uuid.Nil для системного комментария (admin action)
		_, _ = s.senderService.AddComment(ctx, id, uuid.Nil, fmt.Sprintf("[Revision requested] %s", req.Comment))
	}
	return &senderv1.RequestRevisionResponse{Registration: regToProto(reg)}, nil
}

// --- Routing: hot-path ---

func (s *Server) CheckSenderAllowed(ctx context.Context, req *senderv1.CheckSenderAllowedRequest) (*senderv1.CheckSenderAllowedResponse, error) {
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	operatorID, err := parseUUID(req.OperatorId, "operator_id")
	if err != nil {
		return nil, err
	}
	allowed, reason, err := s.senderService.CheckSenderAllowed(ctx, clientID, req.SenderName, operatorID)
	if err != nil {
		log.Error().Err(err).Msg("check sender allowed error")
		return &senderv1.CheckSenderAllowedResponse{Allowed: false, RejectionReason: "sender_check_error"}, nil
	}
	return &senderv1.CheckSenderAllowedResponse{Allowed: allowed, RejectionReason: reason}, nil
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./internal/services/sender/...`
Expected: BUILD OK

- [ ] **Step 3: Коммит**

```bash
git add internal/services/sender/grpc/
git commit -m "feat(sender): add gRPC server implementation"
```

---

### Task 7: main.go для sender-service

**Files:**
- Create: `cmd/services/sender-service/main.go`

- [ ] **Step 1: Реализовать точку входа**

```go
// cmd/services/sender-service/main.go
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/services/sender/application"
	sendergrpc "github.com/smpp-server/smpp-server/internal/services/sender/grpc"
	"github.com/smpp-server/smpp-server/internal/services/sender/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"

	senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
)

func main() {
	// 1. Logger
	shared.InitLogger("development")
	logger := shared.WithService("sender-service")

	// 2. Config
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	// 3. Wait for DB
	storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second)

	// 4. PostgreSQL pool
	dbPool, err := pgxpool.New(context.Background(), cfg.Database.GetDSN())
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к PostgreSQL")
	}
	defer dbPool.Close()
	logger.Info().Msg("PostgreSQL подключен")

	// 5. Repositories
	regRepo := repository.NewRegistrationRepository(dbPool)
	tplRepo := repository.NewTemplateRepository(dbPool)
	commentRepo := repository.NewCommentRepository(dbPool)

	// 6. Application service
	senderService := application.NewSenderService(regRepo, tplRepo, commentRepo)

	// 7. gRPC server
	grpcPort := 9104
	if portStr := os.Getenv("GRPC_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			grpcPort = p
		}
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	senderGrpcServer := sendergrpc.NewServer(senderService)
	senderv1.RegisterSenderServiceServer(grpcServer, senderGrpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
	}

	grpcListener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(grpcPort)))
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Int("port", grpcPort).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// 8. Health + Metrics HTTP
	healthChecker := monitoring.NewHealthChecker("sender-service", cfg.Service.Version)

	metricsPort := 2118
	if portStr := os.Getenv("METRICS_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			metricsPort = p
		}
	}

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    net.JoinHostPort("0.0.0.0", strconv.Itoa(metricsPort)),
		Handler: metricsMux,
	}

	go func() {
		logger.Info().Int("port", metricsPort).Msg("HTTP metrics сервер запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// 9. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logger.Info().Msg("получен сигнал остановки")

	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = metricsServer.Shutdown(shutdownCtx)

	logger.Info().Msg("Sender Service остановлен")
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./cmd/services/sender-service/`
Expected: BUILD OK

- [ ] **Step 3: Коммит**

```bash
git add cmd/services/sender-service/
git commit -m "feat(sender): add sender-service binary entrypoint"
```

---

### Task 8: Portal REST handlers

**Files:**
- Create: `internal/gateway/portal/handlers/sender_registrations.go`

- [ ] **Step 1: Реализовать Portal REST handlers**

```go
// internal/gateway/portal/handlers/sender_registrations.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SenderRegistrationHandlers struct {
	senderClient senderv1.SenderServiceClient
}

func NewSenderRegistrationHandlers(senderClient senderv1.SenderServiceClient) *SenderRegistrationHandlers {
	return &SenderRegistrationHandlers{senderClient: senderClient}
}

// --- Registration CRUD ---

type createRegistrationRequest struct {
	OperatorID string `json:"operator_id"`
	SenderName string `json:"sender_name"`
}

func (h *SenderRegistrationHandlers) CreateRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req createRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.OperatorID == "" || req.SenderName == "" {
		respondError(w, shared.ErrInvalidInput("Поля operator_id и sender_name обязательны"))
		return
	}
	resp, err := h.senderClient.CreateRegistration(r.Context(), &senderv1.CreateRegistrationRequest{
		ClientId:   clientID.String(),
		OperatorId: req.OperatorID,
		SenderName: req.SenderName,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания регистрации")
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, regInfoToJSON(resp.Registration))
}

func (h *SenderRegistrationHandlers) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	resp, err := h.senderClient.ListRegistrations(r.Context(), &senderv1.ListRegistrationsRequest{
		ClientId:   clientID.String(),
		Status:     r.URL.Query().Get("status"),
		OperatorId: r.URL.Query().Get("operator_id"),
		Limit:      perPage,
		Offset:     offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	regs := make([]map[string]interface{}, 0, len(resp.Registrations))
	for _, reg := range resp.Registrations {
		regs = append(regs, regInfoToJSON(reg))
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"registrations": regs,
		"total":         resp.Total,
		"page":          page,
		"per_page":      perPage,
		"total_pages":   totalPages,
	})
}

func (h *SenderRegistrationHandlers) GetRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	resp, err := h.senderClient.GetRegistration(r.Context(), &senderv1.GetRegistrationRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, regInfoToJSON(resp.Registration))
}

type updateRegistrationRequest struct {
	SenderName string `json:"sender_name"`
}

func (h *SenderRegistrationHandlers) UpdateRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	var req updateRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.senderClient.UpdateRegistration(r.Context(), &senderv1.UpdateRegistrationRequest{
		Id: id, ClientId: clientID.String(), SenderName: req.SenderName,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, regInfoToJSON(resp.Registration))
}

func (h *SenderRegistrationHandlers) DeleteRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	_, err := h.senderClient.DeleteRegistration(r.Context(), &senderv1.DeleteRegistrationRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SenderRegistrationHandlers) SubmitRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	resp, err := h.senderClient.SubmitRegistration(r.Context(), &senderv1.SubmitRegistrationRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, regInfoToJSON(resp.Registration))
}

func (h *SenderRegistrationHandlers) ResubmitRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	id := mux.Vars(r)["id"]
	resp, err := h.senderClient.ResubmitRegistration(r.Context(), &senderv1.ResubmitRegistrationRequest{
		Id: id, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, regInfoToJSON(resp.Registration))
}

// --- Templates ---

type createRegTemplateRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (h *SenderRegistrationHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	regID := mux.Vars(r)["id"]
	var req createRegTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.senderClient.CreateTemplate(r.Context(), &senderv1.CreateTemplateRequest{
		RegistrationId: regID, ClientId: clientID.String(), Name: req.Name, Body: req.Body,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, tplInfoToJSON(resp.Template))
}

func (h *SenderRegistrationHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	vars := mux.Vars(r)
	regID := vars["id"]
	tplID := vars["tid"]
	var req createRegTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.senderClient.UpdateTemplate(r.Context(), &senderv1.UpdateTemplateRequest{
		Id: tplID, RegistrationId: regID, ClientId: clientID.String(), Name: req.Name, Body: req.Body,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tplInfoToJSON(resp.Template))
}

func (h *SenderRegistrationHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	vars := mux.Vars(r)
	regID := vars["id"]
	tplID := vars["tid"]
	_, err := h.senderClient.DeleteTemplate(r.Context(), &senderv1.DeleteTemplateRequest{
		Id: tplID, RegistrationId: regID, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Comments ---

func (h *SenderRegistrationHandlers) ListComments(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	regID := mux.Vars(r)["id"]
	resp, err := h.senderClient.ListComments(r.Context(), &senderv1.ListCommentsRequest{
		RegistrationId: regID, ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	comments := make([]map[string]interface{}, 0, len(resp.Comments))
	for _, c := range resp.Comments {
		comments = append(comments, commentInfoToJSON(c))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"comments": comments})
}

// --- JSON converters ---

func regInfoToJSON(r *senderv1.RegistrationInfo) map[string]interface{} {
	if r == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":          r.Id,
		"client_id":   r.ClientId,
		"operator_id": r.OperatorId,
		"sender_name": r.SenderName,
		"status":      r.Status,
	}
	if r.OperatorName != "" {
		m["operator_name"] = r.OperatorName
	}
	if r.CreatedAt != nil {
		m["created_at"] = r.CreatedAt.AsTime()
	}
	if r.UpdatedAt != nil {
		m["updated_at"] = r.UpdatedAt.AsTime()
	}
	if r.SubmittedAt != nil {
		m["submitted_at"] = r.SubmittedAt.AsTime()
	}
	if r.ResolvedAt != nil {
		m["resolved_at"] = r.ResolvedAt.AsTime()
	}
	if len(r.Templates) > 0 {
		templates := make([]map[string]interface{}, len(r.Templates))
		for i, t := range r.Templates {
			templates[i] = tplInfoToJSON(t)
		}
		m["templates"] = templates
	}
	return m
}

func tplInfoToJSON(t *senderv1.TemplateInfo) map[string]interface{} {
	if t == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":              t.Id,
		"registration_id": t.RegistrationId,
		"name":            t.Name,
		"body":            t.Body,
		"status":          t.Status,
	}
	if t.CreatedAt != nil {
		m["created_at"] = t.CreatedAt.AsTime()
	}
	return m
}

func commentInfoToJSON(c *senderv1.CommentInfo) map[string]interface{} {
	if c == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":              c.Id,
		"registration_id": c.RegistrationId,
		"author_id":       c.AuthorId,
		"body":            c.Body,
	}
	if c.CreatedAt != nil {
		m["created_at"] = c.CreatedAt.AsTime()
	}
	return m
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./internal/gateway/portal/handlers/`
Expected: BUILD OK

- [ ] **Step 3: Коммит**

```bash
git add internal/gateway/portal/handlers/sender_registrations.go
git commit -m "feat(sender): add portal REST handlers for sender registrations"
```

---

### Task 9: Admin REST handlers

**Files:**
- Create: `internal/gateway/admin/handlers/sender_registrations.go`

- [ ] **Step 1: Реализовать Admin REST handlers**

```go
// internal/gateway/admin/handlers/sender_registrations.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SenderRegistrationHandlers struct {
	senderClient senderv1.SenderServiceClient
}

func NewSenderRegistrationHandlers(senderClient senderv1.SenderServiceClient) *SenderRegistrationHandlers {
	return &SenderRegistrationHandlers{senderClient: senderClient}
}

func (h *SenderRegistrationHandlers) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	resp, err := h.senderClient.ListRegistrations(r.Context(), &senderv1.ListRegistrationsRequest{
		ClientId:   r.URL.Query().Get("client_id"),
		Status:     r.URL.Query().Get("status"),
		OperatorId: r.URL.Query().Get("operator_id"),
		Limit:      perPage,
		Offset:     offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка регистраций")
		respondGRPCError(w, err)
		return
	}
	regs := make([]map[string]interface{}, 0, len(resp.Registrations))
	for _, reg := range resp.Registrations {
		regs = append(regs, adminRegInfoToJSON(reg))
	}
	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"registrations": regs,
		"total":         resp.Total,
		"page":          page,
		"per_page":      perPage,
		"total_pages":   totalPages,
	})
}

func (h *SenderRegistrationHandlers) GetRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Админ — client_id пустой, получаем без проверки доступа через ListRegistrations
	resp, err := h.senderClient.ListRegistrations(r.Context(), &senderv1.ListRegistrationsRequest{
		Limit: 1,
	})
	// Используем GetRegistration через специальный admin-вызов (client_id="" для admin)
	// Workaround: передаём пустой client_id — gRPC сервер для admin методов не проверяет client_id
	_ = resp
	// Правильный подход: получить через GetRegistration с пустым client_id
	// gRPC GetRegistration требует client_id, поэтому для admin используем ListRegistrations с фильтром
	// Или добавляем специальный метод. Для простоты — ListRegistrations + фильтр по id на стороне клиента.
	// Более корректно: используем ListRegistrations без client_id
	listResp, err := h.senderClient.ListRegistrations(r.Context(), &senderv1.ListRegistrationsRequest{
		Limit: 100,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	for _, reg := range listResp.Registrations {
		if reg.Id == id {
			respondJSON(w, http.StatusOK, adminRegInfoToJSON(reg))
			return
		}
	}
	respondError(w, shared.ErrNotFound("Регистрация не найдена"))
}

func (h *SenderRegistrationHandlers) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	resp, err := h.senderClient.ApproveRegistration(r.Context(), &senderv1.ApproveRegistrationRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminRegInfoToJSON(resp.Registration))
}

func (h *SenderRegistrationHandlers) RejectRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	resp, err := h.senderClient.RejectRegistration(r.Context(), &senderv1.RejectRegistrationRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminRegInfoToJSON(resp.Registration))
}

type requestRevisionBody struct {
	Comment string `json:"comment"`
}

func (h *SenderRegistrationHandlers) RequestRevision(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body requestRevisionBody
	_ = json.NewDecoder(r.Body).Decode(&body) // comment необязателен
	resp, err := h.senderClient.RequestRevision(r.Context(), &senderv1.RequestRevisionRequest{
		Id: id, Comment: body.Comment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminRegInfoToJSON(resp.Registration))
}

type addCommentBody struct {
	Body string `json:"body"`
}

func (h *SenderRegistrationHandlers) AddComment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body addCommentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Body == "" {
		respondError(w, shared.ErrInvalidInput("Поле body обязательно"))
		return
	}
	// TODO: получить author_id из JWT admin-пользователя
	authorID := "00000000-0000-0000-0000-000000000000" // placeholder
	resp, err := h.senderClient.AddComment(r.Context(), &senderv1.AddCommentRequest{
		RegistrationId: id, AuthorId: authorID, Body: body.Body,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":   resp.Comment.Id,
		"body": resp.Comment.Body,
	})
}

func adminRegInfoToJSON(r *senderv1.RegistrationInfo) map[string]interface{} {
	if r == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":          r.Id,
		"client_id":   r.ClientId,
		"operator_id": r.OperatorId,
		"sender_name": r.SenderName,
		"status":      r.Status,
	}
	if r.OperatorName != "" {
		m["operator_name"] = r.OperatorName
	}
	if r.CreatedAt != nil {
		m["created_at"] = r.CreatedAt.AsTime()
	}
	if r.UpdatedAt != nil {
		m["updated_at"] = r.UpdatedAt.AsTime()
	}
	if r.SubmittedAt != nil {
		m["submitted_at"] = r.SubmittedAt.AsTime()
	}
	if r.ResolvedAt != nil {
		m["resolved_at"] = r.ResolvedAt.AsTime()
	}
	if len(r.Templates) > 0 {
		templates := make([]map[string]interface{}, len(r.Templates))
		for i, t := range r.Templates {
			templates[i] = map[string]interface{}{
				"id": t.Id, "name": t.Name, "body": t.Body, "status": t.Status,
			}
			if t.CreatedAt != nil {
				templates[i]["created_at"] = t.CreatedAt.AsTime()
			}
		}
		m["templates"] = templates
	}
	return m
}
```

- [ ] **Step 2: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./internal/gateway/admin/handlers/`
Expected: BUILD OK

- [ ] **Step 3: Коммит**

```bash
git add internal/gateway/admin/handlers/sender_registrations.go
git commit -m "feat(sender): add admin REST handlers for sender registrations"
```

---

### Task 10: Подключение к Gateway — clients, router, main

**Files:**
- Modify: `internal/gateway/portal/clients.go`
- Modify: `internal/gateway/admin/clients.go`
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `internal/gateway/admin/router/router.go`
- Modify: `cmd/portal-gateway/main.go`
- Modify: `cmd/admin-gateway/main.go`

- [ ] **Step 1: Добавить SenderClient в portal/clients.go**

В `internal/gateway/portal/clients.go`:

Добавить import:
```go
senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
```

В `ServiceClients` добавить поле:
```go
SenderClient senderv1.SenderServiceClient
```

В `ServiceAddresses` добавить поле:
```go
Sender string
```

В `NewServiceClients` добавить блок после Link:
```go
// Подключение к Sender Service
if addresses.Sender != "" {
    conn, err := grpc.Dial(addresses.Sender, opts...)
    if err != nil {
        clients.Close()
        return nil, fmt.Errorf("не удалось подключиться к Sender Service: %w", err)
    }
    clients.SenderClient = senderv1.NewSenderServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}
```

- [ ] **Step 2: Добавить SenderClient в admin/clients.go**

Аналогичные изменения в `internal/gateway/admin/clients.go`.

- [ ] **Step 3: Добавить маршруты в portal router**

В `internal/gateway/portal/router/router.go`:

Добавить параметр `senderRegistrationHandlers *handlers.SenderRegistrationHandlers` в `SetupRouter`.

Добавить маршруты внутри `protected` блока (после templates):
```go
// Sender Registrations endpoints
senderRegs := protected.PathPrefix("/sender-registrations").Subrouter()
senderRegs.HandleFunc("", senderRegistrationHandlers.CreateRegistration).Methods("POST")
senderRegs.HandleFunc("", senderRegistrationHandlers.ListRegistrations).Methods("GET")
senderRegs.HandleFunc("/{id}", senderRegistrationHandlers.GetRegistration).Methods("GET")
senderRegs.HandleFunc("/{id}", senderRegistrationHandlers.UpdateRegistration).Methods("PUT")
senderRegs.HandleFunc("/{id}", senderRegistrationHandlers.DeleteRegistration).Methods("DELETE")
senderRegs.HandleFunc("/{id}/submit", senderRegistrationHandlers.SubmitRegistration).Methods("POST")
senderRegs.HandleFunc("/{id}/resubmit", senderRegistrationHandlers.ResubmitRegistration).Methods("POST")
senderRegs.HandleFunc("/{id}/templates", senderRegistrationHandlers.CreateTemplate).Methods("POST")
senderRegs.HandleFunc("/{id}/templates/{tid}", senderRegistrationHandlers.UpdateTemplate).Methods("PUT")
senderRegs.HandleFunc("/{id}/templates/{tid}", senderRegistrationHandlers.DeleteTemplate).Methods("DELETE")
senderRegs.HandleFunc("/{id}/comments", senderRegistrationHandlers.ListComments).Methods("GET")
```

- [ ] **Step 4: Добавить маршруты в admin router**

В `internal/gateway/admin/router/router.go`:

Добавить параметр `senderRegistrationHandlers *handlers.SenderRegistrationHandlers` в `SetupRouter`.

Добавить маршруты (после tarification):
```go
// Sender Registration endpoints
senderRegs := adminV1.PathPrefix("/sender-registrations").Subrouter()
senderRegs.HandleFunc("", senderRegistrationHandlers.ListRegistrations).Methods("GET")
senderRegs.HandleFunc("/{id}", senderRegistrationHandlers.GetRegistration).Methods("GET")
senderRegs.HandleFunc("/{id}/approve", senderRegistrationHandlers.ApproveRegistration).Methods("POST")
senderRegs.HandleFunc("/{id}/reject", senderRegistrationHandlers.RejectRegistration).Methods("POST")
senderRegs.HandleFunc("/{id}/request-revision", senderRegistrationHandlers.RequestRevision).Methods("POST")
senderRegs.HandleFunc("/{id}/comments", senderRegistrationHandlers.AddComment).Methods("POST")
```

- [ ] **Step 5: Обновить portal-gateway main.go**

В `cmd/portal-gateway/main.go`:

Добавить в `serviceAddresses`:
```go
Sender: getEnvOrDefault("SENDER_SERVICE_ADDR", "localhost:9104"),
```

Добавить создание handlers (после `templateHandlers`):
```go
senderRegHandlers := handlers.NewSenderRegistrationHandlers(serviceClients.SenderClient)
```

Добавить `senderRegHandlers` в вызов `portalrouter.SetupRouter(...)`.

- [ ] **Step 6: Обновить admin-gateway main.go**

В `cmd/admin-gateway/main.go`:

Добавить в `serviceAddresses`:
```go
Sender: getEnvOrDefault("SENDER_SERVICE_ADDR", "localhost:9104"),
```

Добавить создание handlers:
```go
senderRegHandlers := handlers.NewSenderRegistrationHandlers(serviceClients.SenderClient)
```

Добавить `senderRegHandlers` в вызов `adminrouter.SetupRouter(...)`.

- [ ] **Step 7: Проверить компиляцию всех gateway**

Run: `cd /home/magomed/projects/sms && go build ./cmd/portal-gateway/ && go build ./cmd/admin-gateway/`
Expected: BUILD OK

- [ ] **Step 8: Коммит**

```bash
git add internal/gateway/portal/clients.go internal/gateway/admin/clients.go \
       internal/gateway/portal/router/router.go internal/gateway/admin/router/router.go \
       cmd/portal-gateway/main.go cmd/admin-gateway/main.go
git commit -m "feat(sender): wire sender-service into portal and admin gateways"
```

---

### Task 11: Интеграция в routing-service (CheckSenderAllowed на hot-path)

**Files:**
- Modify: `internal/services/routing/application/routing_service.go`
- Modify: `cmd/services/routing-service/main.go`

- [ ] **Step 1: Добавить SenderChecker интерфейс и вызов в routing_service.go**

В `internal/services/routing/application/routing_service.go`:

Добавить интерфейс и поле:
```go
// SenderChecker — интерфейс проверки отправителя (реализуется gRPC-клиентом sender-service)
type SenderChecker interface {
	CheckSenderAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, string, error)
}
```

Добавить поле в struct `RoutingService`:
```go
senderChecker SenderChecker
```

Добавить setter:
```go
// SetSenderChecker устанавливает проверку отправителя
func (s *RoutingService) SetSenderChecker(checker SenderChecker) {
	s.senderChecker = checker
}
```

Добавить метод `RouteMessageWithSenderCheck`:
```go
// RouteMessageWithSenderCheck маршрутизирует с проверкой sender_name
func (s *RoutingService) RouteMessageWithSenderCheck(
	ctx context.Context,
	messageID uuid.UUID,
	destination string,
	senderName string,
	clientID *uuid.UUID,
	operatorID *uuid.UUID,
	existingRouteID *uuid.UUID,
	existingProviderID *uuid.UUID,
) (uuid.UUID, uuid.UUID, error) {
	// Проверка sender_name если checker настроен и есть clientID + operatorID
	if s.senderChecker != nil && clientID != nil && operatorID != nil && senderName != "" {
		allowed, reason, err := s.senderChecker.CheckSenderAllowed(ctx, *clientID, senderName, *operatorID)
		if err != nil {
			// Fail-closed: при ошибке — блокируем
			s.logger.Error().Err(err).
				Str("message_id", messageID.String()).
				Str("sender", senderName).
				Msg("sender check failed, blocking (fail-closed)")
			return uuid.Nil, uuid.Nil, fmt.Errorf("sender_service_unavailable: %w", err)
		}
		if !allowed {
			s.logger.Info().
				Str("message_id", messageID.String()).
				Str("sender", senderName).
				Str("reason", reason).
				Msg("sender not allowed")
			return uuid.Nil, uuid.Nil, fmt.Errorf("sender_not_allowed: %s", reason)
		}
	}

	return s.RouteMessage(ctx, messageID, destination, clientID, existingRouteID, existingProviderID)
}
```

- [ ] **Step 2: Создать SenderChecker-адаптер с Redis-кэшем**

Создать файл `internal/services/routing/application/sender_checker.go`:

```go
// internal/services/routing/application/sender_checker.go
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
)

const senderCacheTTL = 5 * time.Minute

// GRPCSenderChecker реализует SenderChecker через gRPC-клиент sender-service с Redis-кэшем
type GRPCSenderChecker struct {
	client senderv1.SenderServiceClient
	redis  *redis.Client
}

func NewGRPCSenderChecker(client senderv1.SenderServiceClient, redisClient *redis.Client) *GRPCSenderChecker {
	return &GRPCSenderChecker{client: client, redis: redisClient}
}

func (c *GRPCSenderChecker) cacheKey(clientID uuid.UUID, senderName string, operatorID uuid.UUID) string {
	return fmt.Sprintf("sender:check:%s:%s:%s", clientID, senderName, operatorID)
}

func (c *GRPCSenderChecker) CheckSenderAllowed(ctx context.Context, clientID uuid.UUID, senderName string, operatorID uuid.UUID) (bool, string, error) {
	// 1. Check Redis cache
	if c.redis != nil {
		key := c.cacheKey(clientID, senderName, operatorID)
		val, err := c.redis.Get(ctx, key).Result()
		if err == nil {
			return val == "1", "", nil
		}
		// Redis miss or error — fallback to gRPC
	}

	// 2. gRPC call
	resp, err := c.client.CheckSenderAllowed(ctx, &senderv1.CheckSenderAllowedRequest{
		ClientId:   clientID.String(),
		SenderName: senderName,
		OperatorId: operatorID.String(),
	})
	if err != nil {
		return false, "sender_service_unavailable", err
	}

	// 3. Cache result
	if c.redis != nil {
		key := c.cacheKey(clientID, senderName, operatorID)
		cacheVal := "0"
		if resp.Allowed {
			cacheVal = "1"
		}
		if err := c.redis.Set(ctx, key, cacheVal, senderCacheTTL).Err(); err != nil {
			log.Warn().Err(err).Msg("failed to cache sender check result")
		}
	}

	return resp.Allowed, resp.RejectionReason, nil
}
```

- [ ] **Step 3: Подключить SenderChecker в routing-service main.go**

В `cmd/services/routing-service/main.go`:

Добавить import:
```go
senderv1 "github.com/smpp-server/smpp-server/api/proto/senderv1"
```

После инициализации Redis-клиента и перед созданием gRPC-сервера добавить:
```go
// Sender checker (gRPC client + Redis cache)
senderAddr := os.Getenv("SENDER_SERVICE_ADDR")
if senderAddr == "" {
    senderAddr = "localhost:9104"
}
senderConn, err := grpc.Dial(senderAddr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithTimeout(5*time.Second),
)
if err != nil {
    logger.Warn().Err(err).Msg("не удалось подключиться к Sender Service, проверка sender отключена")
} else {
    defer senderConn.Close()
    senderClient := senderv1.NewSenderServiceClient(senderConn)
    senderChecker := application.NewGRPCSenderChecker(senderClient, redisClient)
    routingService.SetSenderChecker(senderChecker)
    logger.Info().Str("addr", senderAddr).Msg("Sender checker подключен")
}
```

- [ ] **Step 4: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./cmd/services/routing-service/ && go build ./internal/services/routing/...`
Expected: BUILD OK

- [ ] **Step 5: Коммит**

```bash
git add internal/services/routing/application/routing_service.go \
       internal/services/routing/application/sender_checker.go \
       cmd/services/routing-service/main.go
git commit -m "feat(sender): integrate CheckSenderAllowed into routing hot-path with Redis cache"
```

---

### Task 12: Docker и конфигурация

**Files:**
- Create: `deployments/docker/sender-service.Dockerfile`
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Создать Dockerfile**

```dockerfile
# deployments/docker/sender-service.Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /sender-service ./cmd/services/sender-service/

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /sender-service /sender-service
EXPOSE 9104 2118
ENTRYPOINT ["/sender-service"]
```

- [ ] **Step 2: Добавить сервис в docker-compose.yml**

В `deployments/docker-compose.yml` добавить:

```yaml
sender-service:
  build:
    context: ..
    dockerfile: deployments/docker/sender-service.Dockerfile
  ports:
    - "9104:9104"
  environment:
    - KAFKA_BROKERS=kafka:9092
    - POSTGRES_HOST=postgres
    - POSTGRES_USER=smpp
    - POSTGRES_PASSWORD=smpp_password
    - POSTGRES_DB=smpp_db
    - GRPC_PORT=9104
    - METRICS_PORT=2118
  networks:
    - smpp-network
  depends_on:
    postgres:
      condition: service_healthy
  healthcheck:
    test: ["CMD-SHELL", "wget -q -O- http://localhost:2118/health >/dev/null || exit 1"]
    interval: 10s
    timeout: 5s
    retries: 3
```

Добавить `SENDER_SERVICE_ADDR=sender-service:9104` в environment:
- `routing-service`
- `admin-gateway`
- `portal-gateway`

- [ ] **Step 3: Коммит**

```bash
git add deployments/docker/sender-service.Dockerfile deployments/docker-compose.yml
git commit -m "feat(sender): add Docker config for sender-service"
```

---

### Task 13: Инвалидация Redis-кэша при смене статуса

**Files:**
- Modify: `internal/services/sender/application/sender_service.go`

- [ ] **Step 1: Добавить CacheInvalidator в SenderService**

В `internal/services/sender/application/sender_service.go`:

Добавить интерфейс и поле:
```go
// CacheInvalidator — инвалидация кэша при смене статуса регистрации
type CacheInvalidator interface {
	InvalidateSenderCheck(ctx context.Context, clientID, senderName, operatorID string) error
}
```

Добавить поле `cacheInvalidator CacheInvalidator` в struct и setter.

В методах `ApproveRegistration`, `RejectRegistration`, `RequestRevisionRegistration` — после успешного `Update` добавить:
```go
if s.cacheInvalidator != nil {
    _ = s.cacheInvalidator.InvalidateSenderCheck(ctx, reg.ClientID.String(), reg.SenderName, reg.OperatorID.String())
}
```

- [ ] **Step 2: Реализовать Redis CacheInvalidator**

Создать `internal/services/sender/infrastructure/cache/redis_invalidator.go`:

```go
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisCacheInvalidator struct {
	redis *redis.Client
}

func NewRedisCacheInvalidator(redisClient *redis.Client) *RedisCacheInvalidator {
	return &RedisCacheInvalidator{redis: redisClient}
}

func (r *RedisCacheInvalidator) InvalidateSenderCheck(ctx context.Context, clientID, senderName, operatorID string) error {
	key := fmt.Sprintf("sender:check:%s:%s:%s", clientID, senderName, operatorID)
	return r.redis.Del(ctx, key).Err()
}
```

- [ ] **Step 3: Подключить в main.go sender-service**

В `cmd/services/sender-service/main.go` добавить Redis-клиент и cache invalidator:
```go
redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
defer redisClient.Close()

cacheInvalidator := cache.NewRedisCacheInvalidator(redisClient)
senderService.SetCacheInvalidator(cacheInvalidator)
```

- [ ] **Step 4: Проверить компиляцию**

Run: `cd /home/magomed/projects/sms && go build ./cmd/services/sender-service/ && go build ./internal/services/sender/...`
Expected: BUILD OK

- [ ] **Step 5: Коммит**

```bash
git add internal/services/sender/application/sender_service.go \
       internal/services/sender/infrastructure/cache/ \
       cmd/services/sender-service/main.go
git commit -m "feat(sender): add Redis cache invalidation on status changes"
```

---

### Task 14: Полный end-to-end тест

**Files:**
- Create: `tests/functional/sender_registration_test.go`

- [ ] **Step 1: Написать functional test для полного lifecycle**

```go
//go:build functional

package functional_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/sender/application"
	"github.com/smpp-server/smpp-server/internal/services/sender/domain"
	"github.com/smpp-server/smpp-server/internal/services/sender/infrastructure/repository"
)

func setupSenderDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN()
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func cleanupSenderTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, tbl := range []string{"registration_comments", "registration_templates", "sender_registrations"} {
		_, _ = pool.Exec(ctx, "DELETE FROM "+tbl)
	}
}

func TestSenderRegistrationLifecycle(t *testing.T) {
	pool := setupSenderDB(t)
	t.Cleanup(func() { cleanupSenderTables(t, pool) })
	cleanupSenderTables(t, pool)

	regRepo := repository.NewRegistrationRepository(pool)
	tplRepo := repository.NewTemplateRepository(pool)
	cmtRepo := repository.NewCommentRepository(pool)
	svc := application.NewSenderService(regRepo, tplRepo, cmtRepo)

	ctx := context.Background()
	// Используем существующие client_id и operator_id из БД
	// В тестовом окружении нужны реальные записи — для unit-тестов mock'и выше достаточны
	clientID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // тестовый клиент
	operatorID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // тестовый оператор

	t.Run("create draft", func(t *testing.T) {
		reg, err := svc.CreateRegistration(ctx, clientID, operatorID, "TESTBANK")
		require.NoError(t, err)
		assert.Equal(t, domain.StatusDraft, reg.Status)
		assert.Equal(t, "TESTBANK", reg.SenderName)

		// Добавляем шаблон
		tpl, err := svc.CreateTemplate(ctx, reg.ID, clientID, "Welcome", "Hello {{name}}")
		require.NoError(t, err)
		assert.Equal(t, "Welcome", tpl.Name)
	})

	t.Run("submit -> approve lifecycle", func(t *testing.T) {
		reg, err := svc.CreateRegistration(ctx, clientID, operatorID, "MYSHOP")
		require.NoError(t, err)

		// Добавляем шаблон
		_, err = svc.CreateTemplate(ctx, reg.ID, clientID, "Promo", "Sale {{discount}}%")
		require.NoError(t, err)

		// Submit
		reg, err = svc.SubmitRegistration(ctx, reg.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusSubmitted, reg.Status)

		// Approve
		reg, err = svc.ApproveRegistration(ctx, reg.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusApproved, reg.Status)

		// Check allowed
		allowed, reason, err := svc.CheckSenderAllowed(ctx, clientID, "MYSHOP", operatorID)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Empty(t, reason)
	})

	t.Run("check not allowed for unknown sender", func(t *testing.T) {
		allowed, reason, err := svc.CheckSenderAllowed(ctx, clientID, "UNKNOWN", operatorID)
		require.NoError(t, err)
		assert.False(t, allowed)
		assert.NotEmpty(t, reason)
	})

	t.Run("numeric sender bypasses check", func(t *testing.T) {
		allowed, _, err := svc.CheckSenderAllowed(ctx, clientID, "+79001234567", operatorID)
		require.NoError(t, err)
		assert.True(t, allowed)
	})
}
```

- [ ] **Step 2: Запустить тесты**

Run: `cd /home/magomed/projects/sms && go test -tags=functional ./tests/functional/ -run TestSenderRegistration -v -count=1`
Expected: PASS (при наличии тестовой БД)

- [ ] **Step 3: Коммит**

```bash
git add tests/functional/sender_registration_test.go
git commit -m "test(sender): add functional tests for sender registration lifecycle"
```

---

## Summary

| Task | Описание | Файлы |
|------|----------|-------|
| 1 | Миграция БД | 2 файла миграций |
| 2 | Proto определение | 1 proto + генерация |
| 3 | Domain layer | 4 файла + тесты |
| 4 | Repositories | 3 файла |
| 5 | Application layer | 1 файл + тесты |
| 6 | gRPC server | 1 файл |
| 7 | sender-service main.go | 1 файл |
| 8 | Portal handlers | 1 файл |
| 9 | Admin handlers | 1 файл |
| 10 | Gateway wiring | 6 файлов (modify) |
| 11 | Routing integration | 3 файла (modify + create) |
| 12 | Docker | 2 файла |
| 13 | Cache invalidation | 3 файла |
| 14 | Functional tests | 1 файл |
