# Message Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add message templates with `{{variable}}` placeholders, admin moderation, and rendering integration into the existing SendMessage/SendBatch API.

**Architecture:** New `template-service` microservice (DDD, gRPC) for CRUD, moderation, and rendering. Client Gateway renders templates before passing text to Messaging Service. Messaging Service unchanged.

**Tech Stack:** Go 1.24, PostgreSQL, gRPC, gorilla/mux, zerolog, Prometheus

**Spec:** `docs/superpowers/specs/2026-03-20-message-templates-design.md`

---

## File Map

### New files (Template Service)
| File | Responsibility |
|------|---------------|
| `api/proto/template/template.proto` | gRPC service definition |
| `api/proto/templatev1/template.pb.go` | Generated protobuf |
| `api/proto/templatev1/template_grpc.pb.go` | Generated gRPC stubs |
| `migrations/000009_create_template_tables.up.sql` | Create templates + audit tables |
| `migrations/000009_create_template_tables.down.sql` | Drop template tables |
| `internal/services/template/domain/models.go` | Template, TemplateAudit domain models |
| `internal/services/template/infrastructure/repository/template_repository.go` | Template CRUD in PostgreSQL |
| `internal/services/template/infrastructure/repository/audit_repository.go` | Audit log repository |
| `internal/services/template/application/template_service.go` | CRUD + moderation + rendering logic |
| `internal/services/template/grpc/server.go` | gRPC server |
| `cmd/services/template-service/main.go` | Service entrypoint |

### New files (Gateway handlers)
| File | Responsibility |
|------|---------------|
| `internal/gateway/client/handlers/templates.go` | Client gateway template CRUD handlers |
| `internal/gateway/admin/handlers/templates.go` | Admin gateway moderation handlers |

### Modified files
| File | Change |
|------|--------|
| `internal/gateway/client/handlers/sms.go` | Add template_id + variables to SendSMS/SendBatch, inject TemplateServiceClient |
| `internal/gateway/client/clients.go` | Add TemplateClient field + connection |
| `internal/gateway/client/router/router.go` | Add templateHandlers param + routes |
| `cmd/client-gateway/main.go` | Add TEMPLATE_SERVICE_ADDR, wire template client to SMSHandlers |
| `internal/gateway/admin/clients.go` | Add TemplateClient field + connection |
| `internal/gateway/admin/router/router.go` | Add templateHandlers param + routes |
| `cmd/admin-gateway/main.go` | Add TEMPLATE_SERVICE_ADDR, create template handlers |
| `deployments/docker-compose.yml` | Add template-service, add env vars to gateways |
| `deployments/docker/service-base.Dockerfile` | Add protoc step for template proto |

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/000009_create_template_tables.up.sql`
- Create: `migrations/000009_create_template_tables.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000009_create_template_tables.up.sql
CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    variables TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'approved', 'rejected')),
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_client_id ON templates(client_id);
CREATE INDEX idx_templates_client_status ON templates(client_id, status);
CREATE UNIQUE INDEX idx_templates_client_name ON templates(client_id, name);

CREATE TRIGGER update_templates_updated_at BEFORE UPDATE ON templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE template_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID REFERENCES templates(id) ON DELETE SET NULL,
    action VARCHAR(20) NOT NULL
        CHECK (action IN ('created', 'updated', 'approved', 'rejected', 'deleted')),
    old_body TEXT,
    new_body TEXT,
    actor_id UUID,
    actor_type VARCHAR(20),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_template_audit_template_id ON template_audit_log(template_id);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000009_create_template_tables.down.sql
DROP TRIGGER IF EXISTS update_templates_updated_at ON templates;
DROP TABLE IF EXISTS template_audit_log;
DROP TABLE IF EXISTS templates;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000009_create_template_tables.up.sql migrations/000009_create_template_tables.down.sql
git commit -m "feat(template): add database migration for templates and audit tables"
```

---

## Task 2: Proto Definition + Code Generation

**Files:**
- Create: `api/proto/template/template.proto`
- Create: `api/proto/templatev1/` (generated)

- [ ] **Step 1: Create proto file**

```protobuf
// api/proto/template/template.proto
syntax = "proto3";

package template.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/templatev1";

import "google/protobuf/timestamp.proto";

service TemplateService {
  rpc CreateTemplate(CreateTemplateRequest) returns (CreateTemplateResponse);
  rpc UpdateTemplate(UpdateTemplateRequest) returns (UpdateTemplateResponse);
  rpc DeleteTemplate(DeleteTemplateRequest) returns (DeleteTemplateResponse);
  rpc GetTemplate(GetTemplateRequest) returns (GetTemplateResponse);
  rpc ListTemplates(ListTemplatesRequest) returns (ListTemplatesResponse);
  rpc ApproveTemplate(ApproveTemplateRequest) returns (ApproveTemplateResponse);
  rpc RejectTemplate(RejectTemplateRequest) returns (RejectTemplateResponse);
  rpc RenderTemplate(RenderTemplateRequest) returns (RenderTemplateResponse);
  rpc GetTemplateAuditLog(GetTemplateAuditLogRequest) returns (GetTemplateAuditLogResponse);
}

message CreateTemplateRequest {
  string client_id = 1;
  string name = 2;
  string body = 3;
}

message CreateTemplateResponse {
  TemplateInfo template = 1;
}

message UpdateTemplateRequest {
  string id = 1;
  string client_id = 2;
  optional string name = 3;
  optional string body = 4;
}

message UpdateTemplateResponse {
  TemplateInfo template = 1;
}

message DeleteTemplateRequest {
  string id = 1;
  string client_id = 2;
}

message DeleteTemplateResponse {
  bool success = 1;
}

message GetTemplateRequest {
  string id = 1;
  string client_id = 2;
}

message GetTemplateResponse {
  TemplateInfo template = 1;
}

message ListTemplatesRequest {
  string client_id = 1;
  string status = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message ListTemplatesResponse {
  repeated TemplateInfo templates = 1;
  int32 total = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message ApproveTemplateRequest {
  string id = 1;
  string actor_id = 2;
}

message ApproveTemplateResponse {
  TemplateInfo template = 1;
}

message RejectTemplateRequest {
  string id = 1;
  string actor_id = 2;
  string reason = 3;
}

message RejectTemplateResponse {
  TemplateInfo template = 1;
}

message RenderTemplateRequest {
  string template_id = 1;
  string client_id = 2;
  map<string, string> variables = 3;
}

message RenderTemplateResponse {
  string rendered_text = 1;
  string template_name = 2;
}

message GetTemplateAuditLogRequest {
  string template_id = 1;
  int32 limit = 2;
  int32 offset = 3;
}

message GetTemplateAuditLogResponse {
  repeated AuditEntry entries = 1;
  int32 total = 2;
}

message TemplateInfo {
  string id = 1;
  string client_id = 2;
  string name = 3;
  string body = 4;
  repeated string variables = 5;
  string status = 6;
  string rejection_reason = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message AuditEntry {
  string id = 1;
  string template_id = 2;
  string action = 3;
  string old_body = 4;
  string new_body = 5;
  string actor_id = 6;
  string actor_type = 7;
  string reason = 8;
  google.protobuf.Timestamp created_at = 9;
}
```

- [ ] **Step 2: Generate Go code**

```bash
mkdir -p api/proto/templatev1
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  -I api/proto \
  api/proto/template/template.proto
```

If `protoc` is not available, generate manually following the pattern from `api/proto/webhookv1/`. The generated files must go to `api/proto/templatev1/`.

- [ ] **Step 3: Commit**

```bash
git add api/proto/template/ api/proto/templatev1/
git commit -m "feat(template): add protobuf definition and generated code"
```

---

## Task 3: Domain Models

**Files:**
- Create: `internal/services/template/domain/models.go`

- [ ] **Step 1: Create domain models**

```go
package domain

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTemplateNotApproved = errors.New("template is not approved")
	ErrInvalidTemplateName = errors.New("invalid template name")
	ErrInvalidTemplateBody = errors.New("invalid template body")
	ErrMissingVariables    = errors.New("missing required variables")
	ErrRenderedTooLong     = errors.New("rendered text exceeds maximum length")
	ErrVariableValueTooLong = errors.New("variable value exceeds maximum length")
	ErrInvalidStatus       = errors.New("invalid template status for this operation")
	ErrDuplicateTemplateName = errors.New("template with this name already exists")
)

const (
	StatusDraft    = "draft"
	StatusApproved = "approved"
	StatusRejected = "rejected"

	MaxBodyLength          = 1600
	MaxNameLength          = 255
	MaxVariableValueLength = 500
	MaxRenderedLength      = 1600
)

var variableRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Template represents a message template
type Template struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Body            string
	Variables       []string
	Status          string
	RejectionReason string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AuditEntry represents a template audit log entry
type AuditEntry struct {
	ID         uuid.UUID
	TemplateID *uuid.UUID
	Action     string
	OldBody    string
	NewBody    string
	ActorID    *uuid.UUID
	ActorType  string
	Reason     string
	CreatedAt  time.Time
}

// ExtractVariables parses {{variable}} placeholders from template body
func ExtractVariables(body string) []string {
	matches := variableRegex.FindAllStringSubmatch(body, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, match := range matches {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/services/template/domain/
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/template/domain/models.go
git commit -m "feat(template): add domain models and error definitions"
```

---

## Task 4: Template Repository

**Files:**
- Create: `internal/services/template/infrastructure/repository/template_repository.go`

- [ ] **Step 1: Create template repository**

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type TemplateRepository struct {
	db *sqlx.DB
}

func NewTemplateRepository(db *sqlx.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

type templateRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Body            string         `db:"body"`
	Variables       pq.StringArray `db:"variables"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	CreatedAt       sql.NullTime   `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
}

func (r *templateRow) toDomain() *domain.Template {
	t := &domain.Template{
		ID:        r.ID,
		ClientID:  r.ClientID,
		Name:      r.Name,
		Body:      r.Body,
		Variables: []string(r.Variables),
		Status:    r.Status,
	}
	if r.RejectionReason.Valid {
		t.RejectionReason = r.RejectionReason.String
	}
	if r.CreatedAt.Valid {
		t.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		t.UpdatedAt = r.UpdatedAt.Time
	}
	return t
}

func (r *TemplateRepository) Create(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	query := `INSERT INTO templates (id, client_id, name, body, variables, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query,
		t.ID, t.ClientID, t.Name, t.Body, pq.StringArray(t.Variables), t.Status,
	).StructScan(&row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateTemplateName
		}
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	query := `SELECT id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at
		FROM templates WHERE id = $1 AND client_id = $2`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return row.toDomain(), nil
}

// GetByIDAdmin retrieves a template without client_id check (for admin operations)
func (r *TemplateRepository) GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	query := `SELECT id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at
		FROM templates WHERE id = $1`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, id).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	countQuery := `SELECT COUNT(*) FROM templates WHERE client_id = $1`
	listQuery := `SELECT id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at
		FROM templates WHERE client_id = $1`
	args := []interface{}{clientID}

	if status != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND status = $2`
		args = append(args, status)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count templates: %w", err)
	}

	listQuery += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		var row templateRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan template: %w", err)
		}
		templates = append(templates, row.toDomain())
	}
	return templates, total, nil
}

func (r *TemplateRepository) Update(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	query := `UPDATE templates SET name = $1, body = $2, variables = $3, status = $4, rejection_reason = $5
		WHERE id = $6 AND client_id = $7
		RETURNING id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at`

	var rejReason sql.NullString
	if t.RejectionReason != "" {
		rejReason = sql.NullString{String: t.RejectionReason, Valid: true}
	}

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query,
		t.Name, t.Body, pq.StringArray(t.Variables), t.Status, rejReason, t.ID, t.ClientID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateTemplateName
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}
	return row.toDomain(), nil
}

// UpdateStatus updates only the status and rejection_reason fields (for admin moderation)
func (r *TemplateRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string) (*domain.Template, error) {
	var rejReason sql.NullString
	if rejectionReason != "" {
		rejReason = sql.NullString{String: rejectionReason, Valid: true}
	}

	query := `UPDATE templates SET status = $1, rejection_reason = $2
		WHERE id = $3
		RETURNING id, client_id, name, body, variables, status, rejection_reason, created_at, updated_at`

	var row templateRow
	err := r.db.QueryRowxContext(ctx, query, status, rejReason, id).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update template status: %w", err)
	}
	return row.toDomain(), nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	query := `DELETE FROM templates WHERE id = $1 AND client_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/template/infrastructure/repository/template_repository.go
git commit -m "feat(template): add template repository"
```

---

## Task 5: Audit Repository

**Files:**
- Create: `internal/services/template/infrastructure/repository/audit_repository.go`

- [ ] **Step 1: Create audit repository**

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

type AuditRepository struct {
	db *sqlx.DB
}

func NewAuditRepository(db *sqlx.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

type auditRow struct {
	ID         uuid.UUID      `db:"id"`
	TemplateID *uuid.UUID     `db:"template_id"`
	Action     string         `db:"action"`
	OldBody    sql.NullString `db:"old_body"`
	NewBody    sql.NullString `db:"new_body"`
	ActorID    *uuid.UUID     `db:"actor_id"`
	ActorType  sql.NullString `db:"actor_type"`
	Reason     sql.NullString `db:"reason"`
	CreatedAt  sql.NullTime   `db:"created_at"`
}

func (r *auditRow) toDomain() *domain.AuditEntry {
	e := &domain.AuditEntry{
		ID:         r.ID,
		TemplateID: r.TemplateID,
		Action:     r.Action,
	}
	if r.OldBody.Valid {
		e.OldBody = r.OldBody.String
	}
	if r.NewBody.Valid {
		e.NewBody = r.NewBody.String
	}
	if r.ActorID != nil {
		e.ActorID = r.ActorID
	}
	if r.ActorType.Valid {
		e.ActorType = r.ActorType.String
	}
	if r.Reason.Valid {
		e.Reason = r.Reason.String
	}
	if r.CreatedAt.Valid {
		e.CreatedAt = r.CreatedAt.Time
	}
	return e
}

func (r *AuditRepository) Create(ctx context.Context, entry *domain.AuditEntry) error {
	query := `INSERT INTO template_audit_log (id, template_id, action, old_body, new_body, actor_id, actor_type, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	var oldBody, newBody, actorType, reason sql.NullString
	if entry.OldBody != "" {
		oldBody = sql.NullString{String: entry.OldBody, Valid: true}
	}
	if entry.NewBody != "" {
		newBody = sql.NullString{String: entry.NewBody, Valid: true}
	}
	if entry.ActorType != "" {
		actorType = sql.NullString{String: entry.ActorType, Valid: true}
	}
	if entry.Reason != "" {
		reason = sql.NullString{String: entry.Reason, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		uuid.New(), entry.TemplateID, entry.Action, oldBody, newBody, entry.ActorID, actorType, reason,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit entry: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM template_audit_log WHERE template_id = $1`, templateID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count audit entries: %w", err)
	}

	query := `SELECT id, template_id, action, old_body, new_body, actor_id, actor_type, reason, created_at
		FROM template_audit_log WHERE template_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, templateID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		var row auditRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit entry: %w", err)
		}
		entries = append(entries, row.toDomain())
	}
	return entries, total, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/template/infrastructure/repository/audit_repository.go
git commit -m "feat(template): add audit log repository"
```

---

## Task 6: Template Application Service

**Files:**
- Create: `internal/services/template/application/template_service.go`

- [ ] **Step 1: Create template service**

```go
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/smpp-server/smpp-server/internal/services/template/infrastructure/repository"
)

type TemplateService struct {
	templateRepo *repository.TemplateRepository
	auditRepo    *repository.AuditRepository
	logger       zerolog.Logger
}

func NewTemplateService(templateRepo *repository.TemplateRepository, auditRepo *repository.AuditRepository) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		auditRepo:    auditRepo,
		logger:       log.With().Str("component", "template-service").Logger(),
	}
}

func (s *TemplateService) CreateTemplate(ctx context.Context, clientID uuid.UUID, name, body string) (*domain.Template, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateBody(body); err != nil {
		return nil, err
	}

	variables := domain.ExtractVariables(body)

	tmpl := &domain.Template{
		ID:        uuid.New(),
		ClientID:  clientID,
		Name:      name,
		Body:      body,
		Variables: variables,
		Status:    domain.StatusDraft,
	}

	created, err := s.templateRepo.Create(ctx, tmpl)
	if err != nil {
		return nil, err
	}

	// Audit log
	templateID := created.ID
	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &templateID,
		Action:     "created",
		NewBody:    body,
		ActorType:  "client",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", templateID.String()).Msg("failed to write audit log")
	}

	return created, nil
}

func (s *TemplateService) GetTemplate(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	return s.templateRepo.GetByID(ctx, id, clientID)
}

func (s *TemplateService) ListTemplates(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.templateRepo.ListByClientID(ctx, clientID, status, limit, offset)
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, id, clientID uuid.UUID, name, body *string) (*domain.Template, error) {
	existing, err := s.templateRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	oldBody := existing.Body
	bodyChanged := false

	if name != nil {
		if err := validateName(*name); err != nil {
			return nil, err
		}
		existing.Name = *name
	}
	if body != nil {
		if err := validateBody(*body); err != nil {
			return nil, err
		}
		existing.Body = *body
		existing.Variables = domain.ExtractVariables(*body)
		bodyChanged = true
		// Reset status to draft when body changes
		existing.Status = domain.StatusDraft
		existing.RejectionReason = ""
	}

	updated, err := s.templateRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	// Audit log
	entry := &domain.AuditEntry{
		TemplateID: &id,
		Action:     "updated",
		ActorType:  "client",
	}
	if bodyChanged {
		entry.OldBody = oldBody
		entry.NewBody = existing.Body
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id, clientID uuid.UUID) error {
	// Write audit entry before delete (FK becomes NULL via ON DELETE SET NULL)
	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "deleted",
		ActorType:  "client",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return s.templateRepo.Delete(ctx, id, clientID)
}

func (s *TemplateService) ApproveTemplate(ctx context.Context, id uuid.UUID, actorID *uuid.UUID) (*domain.Template, error) {
	tmpl, err := s.templateRepo.GetByIDAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl.Status != domain.StatusDraft {
		return nil, fmt.Errorf("%w: can only approve draft templates", domain.ErrInvalidStatus)
	}

	updated, err := s.templateRepo.UpdateStatus(ctx, id, domain.StatusApproved, "")
	if err != nil {
		return nil, err
	}

	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "approved",
		ActorID:    actorID,
		ActorType:  "admin",
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) RejectTemplate(ctx context.Context, id uuid.UUID, actorID *uuid.UUID, reason string) (*domain.Template, error) {
	tmpl, err := s.templateRepo.GetByIDAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl.Status != domain.StatusDraft {
		return nil, fmt.Errorf("%w: can only reject draft templates", domain.ErrInvalidStatus)
	}

	updated, err := s.templateRepo.UpdateStatus(ctx, id, domain.StatusRejected, reason)
	if err != nil {
		return nil, err
	}

	if err := s.auditRepo.Create(ctx, &domain.AuditEntry{
		TemplateID: &id,
		Action:     "rejected",
		ActorID:    actorID,
		ActorType:  "admin",
		Reason:     reason,
	}); err != nil {
		s.logger.Error().Err(err).Str("template_id", id.String()).Msg("failed to write audit log")
	}

	return updated, nil
}

func (s *TemplateService) RenderTemplate(ctx context.Context, templateID, clientID uuid.UUID, variables map[string]string) (string, string, error) {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID, clientID)
	if err != nil {
		return "", "", err
	}

	if tmpl.Status != domain.StatusApproved {
		return "", "", domain.ErrTemplateNotApproved
	}

	// Check all required variables are provided
	var missing []string
	for _, v := range tmpl.Variables {
		if _, ok := variables[v]; !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("%w: %s", domain.ErrMissingVariables, strings.Join(missing, ", "))
	}

	// Validate variable value lengths
	for k, v := range variables {
		if len(v) > domain.MaxVariableValueLength {
			return "", "", fmt.Errorf("%w: variable '%s' is %d chars (max %d)", domain.ErrVariableValueTooLong, k, len(v), domain.MaxVariableValueLength)
		}
	}

	// Render: replace {{var}} with value
	replacerArgs := make([]string, 0, len(variables)*2)
	for k, v := range variables {
		replacerArgs = append(replacerArgs, "{{"+k+"}}", v)
	}
	rendered := strings.NewReplacer(replacerArgs...).Replace(tmpl.Body)

	// Validate rendered length
	if len(rendered) > domain.MaxRenderedLength {
		return "", "", fmt.Errorf("%w: %d chars (max %d)", domain.ErrRenderedTooLong, len(rendered), domain.MaxRenderedLength)
	}

	return rendered, tmpl.Name, nil
}

func (s *TemplateService) GetAuditLog(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.auditRepo.ListByTemplateID(ctx, templateID, limit, offset)
}

func validateName(name string) error {
	if name == "" || len(name) > domain.MaxNameLength {
		return fmt.Errorf("%w: must be 1-%d characters", domain.ErrInvalidTemplateName, domain.MaxNameLength)
	}
	return nil
}

func validateBody(body string) error {
	if body == "" || len(body) > domain.MaxBodyLength {
		return fmt.Errorf("%w: must be 1-%d characters", domain.ErrInvalidTemplateBody, domain.MaxBodyLength)
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/services/template/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/template/application/template_service.go
git commit -m "feat(template): add template application service with CRUD, moderation, and rendering"
```

---

## Task 7: gRPC Server

**Files:**
- Create: `internal/services/template/grpc/server.go`

- [ ] **Step 1: Create gRPC server**

Follow the webhook gRPC server pattern. The server wraps `*application.TemplateService` and implements all 9 RPCs defined in the proto.

Key methods:
- `CreateTemplate`, `GetTemplate`, `ListTemplates`, `UpdateTemplate`, `DeleteTemplate` — standard CRUD with UUID parsing, client_id validation
- `ApproveTemplate`, `RejectTemplate` — admin moderation, parse actor_id
- `RenderTemplate` — calls service.RenderTemplate, returns rendered_text + template_name
- `GetTemplateAuditLog` — paginated audit entries
- `mapError` using `errors.Is()` — maps domain errors to gRPC status codes: `ErrTemplateNotFound` → NotFound, `ErrTemplateNotApproved` → FailedPrecondition, `ErrInvalidTemplateName/Body` → InvalidArgument, `ErrMissingVariables` → InvalidArgument, `ErrRenderedTooLong/VariableValueTooLong` → InvalidArgument, `ErrInvalidStatus` → FailedPrecondition, `ErrDuplicateTemplateName` → AlreadyExists
- `templateToProto` converter — maps domain.Template to templatev1.TemplateInfo
- `auditToProto` converter — maps domain.AuditEntry to templatev1.AuditEntry
- `UpdateTemplate` uses `req.Name` and `req.Body` as `*string` (proto optional fields)

Read the plan spec and the webhook gRPC server at `internal/services/webhook/grpc/server.go` for the exact pattern.

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/services/template/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/template/grpc/server.go
git commit -m "feat(template): add gRPC server for template management"
```

---

## Task 8: Template Service Main Entrypoint

**Files:**
- Create: `cmd/services/template-service/main.go`

- [ ] **Step 1: Create main.go**

Follow `cmd/services/webhook-service/main.go` pattern but **simpler** — no Kafka, no Redis:

1. Init logger with `shared.WithService("template-service")`
2. Load config via `config.Load("")`
3. Wait for database, connect with `database.NewDBWithConfig`
4. Create `sqlx.NewDb(dbConn.DB, "pgx")`
5. Create repositories: `templateRepo`, `auditRepo`
6. Create `TemplateService(templateRepo, auditRepo)`
7. Create gRPC server on `:9099`
8. Register `templatev1.RegisterTemplateServiceServer`
9. Health checker with DB, metrics on `:2120`
10. Graceful shutdown: grpcServer.GracefulStop(), metricsServer.Shutdown()

**No Kafka initialization** — this service is purely synchronous gRPC.

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/services/template-service/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/services/template-service/main.go
git commit -m "feat(template): add template service entrypoint"
```

---

## Task 9: Client Gateway — Template HTTP Handlers

**Files:**
- Create: `internal/gateway/client/handlers/templates.go`
- Modify: `internal/gateway/client/clients.go` — add TemplateClient
- Modify: `internal/gateway/client/router/router.go` — add routes + param
- Modify: `cmd/client-gateway/main.go` — wire template client

- [ ] **Step 1: Create client gateway template handlers**

`TemplateHandlers` struct with `templatev1.TemplateServiceClient`. Methods:
- `CreateTemplate` — POST, decodes `{name, body}`, calls gRPC CreateTemplate with client_id from context → 201
- `ListTemplates` — GET, query params: `status`, `limit`, `offset` → paginated list
- `GetTemplate` — GET `/{id}` → template details
- `UpdateTemplate` — PUT `/{id}`, decodes `{name, body}` (both optional) → updated template
- `DeleteTemplate` — DELETE `/{id}` → 204
- `GetTemplateAudit` — GET `/{id}/audit`, query params: `limit`, `offset` → paginated audit

All methods use `middleware.GetClientID(r.Context())` for ownership. Follow `webhooks.go` handler pattern exactly.

- [ ] **Step 2: Add TemplateClient to ServiceClients**

In `internal/gateway/client/clients.go`:
- Import `templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"`
- Add `TemplateClient templatev1.TemplateServiceClient` to `ServiceClients`
- Add `Template string` to `ServiceAddresses`
- Add connection block for Template service

- [ ] **Step 3: Add template routes to router**

In `internal/gateway/client/router/router.go`:
- Add `templateHandlers *handlers.TemplateHandlers` parameter to `SetupRouter`
- Add routes:
```go
templates := apiV1.PathPrefix("/templates").Subrouter()
templates.HandleFunc("", templateHandlers.CreateTemplate).Methods("POST")
templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
templates.HandleFunc("/{id}", templateHandlers.UpdateTemplate).Methods("PUT")
templates.HandleFunc("/{id}", templateHandlers.DeleteTemplate).Methods("DELETE")
templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAudit).Methods("GET")
```

- [ ] **Step 4: Wire in main.go**

In `cmd/client-gateway/main.go`:
- Add `Template: getEnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099")` to serviceAddresses
- Create `templateHandlers := handlers.NewTemplateHandlers(serviceClients.TemplateClient)`
- Pass to `clientrouter.SetupRouter(..., templateHandlers)`

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/client/handlers/templates.go \
  internal/gateway/client/clients.go \
  internal/gateway/client/router/router.go \
  cmd/client-gateway/main.go
git commit -m "feat(template): add template CRUD endpoints to client gateway"
```

---

## Task 10: Modify SMS Handlers for Template Rendering

**Files:**
- Modify: `internal/gateway/client/handlers/sms.go`

- [ ] **Step 1: Add TemplateServiceClient to SMSHandlers**

In `sms.go`:
- Add `templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"` import
- Add `templateClient templatev1.TemplateServiceClient` field to `SMSHandlers`
- Update `NewSMSHandlers` to accept `templateClient templatev1.TemplateServiceClient` as second parameter

- [ ] **Step 2: Add template fields to SendSMSRequest**

```go
type SendSMSRequest struct {
    // ... existing fields ...
    TemplateID string            `json:"template_id,omitempty"`
    Variables  map[string]string `json:"variables,omitempty"`
}
```

- [ ] **Step 3: Add template rendering logic to SendSMS**

Replace the validation block `if req.Source == "" || req.Destination == "" || req.Text == ""` with:

```go
// Validate source and destination
if req.Source == "" || req.Destination == "" {
    respondError(w, shared.ErrInvalidInput("Поля source и destination обязательны"))
    return
}

// Resolve text: either from template or direct
text := req.Text
if req.TemplateID != "" && req.Text != "" {
    respondError(w, shared.ErrInvalidInput("Нельзя указать одновременно text и template_id"))
    return
}
if req.TemplateID == "" && req.Text == "" {
    respondError(w, shared.ErrInvalidInput("Необходимо указать text или template_id"))
    return
}
if req.TemplateID != "" {
    renderResp, err := h.templateClient.RenderTemplate(r.Context(), &templatev1.RenderTemplateRequest{
        TemplateId: req.TemplateID,
        ClientId:   clientID.String(),
        Variables:  req.Variables,
    })
    if err != nil {
        respondGRPCError(w, err)
        return
    }
    text = renderResp.RenderedText
}
```

Then in the `protoReq` construction, change `Text: req.Text,` to `Text: text,`:

```go
protoReq := &messagingv1.SendMessageRequest{
    // ... all existing fields ...
    Text: text,  // was req.Text — now uses rendered template text if applicable
    // ...
}
```

- [ ] **Step 4: Apply same logic to SendBatch**

In the `SendBatch` handler, for each message in the batch:
- If `msg.TemplateID != ""` and `msg.Text != ""` → skip (or error)
- If `msg.TemplateID != ""` → call RenderTemplate, use rendered text
- If `msg.Text != ""` → use as-is
- If neither → skip

- [ ] **Step 5: Update NewSMSHandlers call in main.go**

In `cmd/client-gateway/main.go`, change:
```go
smsHandlers := handlers.NewSMSHandlers(serviceClients.MessagingClient)
```
to:
```go
smsHandlers := handlers.NewSMSHandlers(serviceClients.MessagingClient, serviceClients.TemplateClient)
```

- [ ] **Step 6: Verify compilation**

```bash
go build ./cmd/client-gateway/
```

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/client/handlers/sms.go cmd/client-gateway/main.go
git commit -m "feat(template): integrate template rendering into SendSMS and SendBatch"
```

---

## Task 11: Admin Gateway — Template Handlers

**Files:**
- Create: `internal/gateway/admin/handlers/templates.go`
- Modify: `internal/gateway/admin/clients.go`
- Modify: `internal/gateway/admin/router/router.go`
- Modify: `cmd/admin-gateway/main.go`

- [ ] **Step 1: Create admin template handlers**

`TemplateHandlers` struct with `templatev1.TemplateServiceClient`. Methods:
- `ListTemplates` — GET, query params: `client_id` (required), `status`, `limit`, `offset`
- `GetTemplate` — GET `/{id}`, query param: `client_id` (required)
- `ApproveTemplate` — POST `/{id}/approve`, body: `{actor_id}` (optional)
- `RejectTemplate` — POST `/{id}/reject`, body: `{reason}` (required)
- `GetTemplateAudit` — GET `/{id}/audit`, query params: `limit`, `offset`

Note: Admin handlers take `client_id` from request (not from auth context). For approve/reject, the template is fetched by ID without client_id scoping (uses `GetByIDAdmin` via gRPC — the gRPC server should handle this by passing the request through to the service which uses `GetByIDAdmin`).

Actually, for Approve/Reject the gRPC proto does NOT include `client_id` — it only takes `id` and `actor_id`. The gRPC server calls `service.ApproveTemplate` which internally uses `GetByIDAdmin`. For Get/List from admin, we still need client_id for scoping.

- [ ] **Step 2: Add TemplateClient to admin ServiceClients + routes + main.go**

Same pattern as client gateway (Task 9, Steps 2-4) but for admin.

Routes:
```go
templates := adminV1.PathPrefix("/templates").Subrouter()
templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
templates.HandleFunc("/{id}/approve", templateHandlers.ApproveTemplate).Methods("POST")
templates.HandleFunc("/{id}/reject", templateHandlers.RejectTemplate).Methods("POST")
templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAudit).Methods("GET")
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/handlers/templates.go \
  internal/gateway/admin/clients.go \
  internal/gateway/admin/router/router.go \
  cmd/admin-gateway/main.go
git commit -m "feat(template): add template moderation endpoints to admin gateway"
```

---

## Task 12: Docker Compose + Dockerfile

**Files:**
- Modify: `deployments/docker-compose.yml`
- Modify: `deployments/docker/service-base.Dockerfile`

- [ ] **Step 1: Add template-service to docker-compose**

After webhook-service, add (following billing-service pattern exactly):

```yaml
  template-service:
    build:
      context: ..
      dockerfile: deployments/docker/service-base.Dockerfile
      args:
        SERVICE_NAME: template-service
        SERVICE_PATH: cmd/services/template-service
    container_name: template-service
    ports:
      - "9099:9099"
      - "2120:2120"
    environment:
      - SERVICE_NAME=template-service
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=smpp
      - POSTGRES_PASSWORD=smpp_password
      - POSTGRES_DB=smpp_db
    networks:
      - smpp-network
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:2120/health"]
      interval: 10s
      timeout: 5s
      retries: 3
```

- [ ] **Step 2: Add TEMPLATE_SERVICE_ADDR to gateways**

Add `- TEMPLATE_SERVICE_ADDR=template-service:9099` to all 4 gateway environment sections.

- [ ] **Step 3: Add template-service dependency to gateways**

Add `template-service: condition: service_healthy` to gateway depends_on.

- [ ] **Step 4: Add protoc step to Dockerfile**

In `deployments/docker/service-base.Dockerfile`, after the webhook protoc block, add:

```dockerfile
RUN mkdir -p api/proto/templatev1 && \
    protoc \
    --go_out=api/proto/templatev1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/proto/templatev1 \
    --go-grpc_opt=paths=source_relative \
    --proto_path=api/proto \
    api/proto/template/template.proto && \
    mv api/proto/templatev1/template/* api/proto/templatev1/ 2>/dev/null || true && \
    rm -rf api/proto/templatev1/template || true
```

Also add `api/proto/templatev1` to the initial `mkdir -p` line.

- [ ] **Step 5: Commit**

```bash
git add deployments/docker-compose.yml deployments/docker/service-base.Dockerfile
git commit -m "feat(template): add template-service to docker-compose and Dockerfile"
```

---

## Task 13: Build Verification

- [ ] **Step 1: Run go mod tidy**

```bash
go mod tidy
```

- [ ] **Step 2: Verify Go build**

```bash
go build ./...
```

- [ ] **Step 3: Verify go vet**

```bash
go vet ./...
```

- [ ] **Step 4: Fix any issues**

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: run go mod tidy after template service implementation"
```
