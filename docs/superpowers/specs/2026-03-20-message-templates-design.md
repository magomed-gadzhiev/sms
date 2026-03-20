# Message Templates & Variables — Design Spec

## Problem

Clients form SMS text on their side, leading to errors, no content control, and inability to optimize encoding. There is no way for the platform to manage, moderate, or reuse message content.

## Solution Overview

A new **Template Service** microservice for CRUD, moderation, and rendering of message templates with `{{variable}}` placeholders. Clients create templates, admins approve them, and sending happens via `template_id` + `variables` map in the existing SendMessage API.

## Scope

### In Scope
- Template CRUD (Client API + Admin API)
- Placeholder syntax: `{{variable_name}}` (double curly braces)
- Moderation workflow: draft → approved / rejected
- Rendering: variable substitution, strict validation
- Audit log for all template changes
- Integration with existing SendMessage and SendBatch flows

### Out of Scope
- Template categories / tags
- Template sharing between clients
- Automatic content moderation (stop-words, regex)
- Template analytics (how often used)
- Localization / multi-language templates

## Architecture

### Approach: Dedicated Template Service with Render API

New microservice `template-service` following existing DDD patterns. Provides synchronous gRPC API for CRUD, moderation, and rendering. No Kafka dependency — pure request/response service.

**Rendering flow:** Client Gateway calls `RenderTemplate(template_id, variables)` → gets rendered text → passes it to Messaging Service as regular `text`. Messaging Service has no knowledge of templates. The `template_id` is **not** stored in the messages table — traceability is not in scope for this feature (can be added later if needed).

### Service Structure

```
cmd/services/template-service/main.go
internal/services/template/
  ├── application/
  │   └── template_service.go       # CRUD + moderation + rendering
  ├── domain/
  │   └── models.go                 # Template, TemplateAudit
  ├── grpc/
  │   └── server.go                 # gRPC API
  └── infrastructure/
      └── repository/
          ├── template_repository.go
          └── audit_repository.go
```

- gRPC port: **9099**
- Metrics port: **2120** (Prometheus)
- No Kafka consumer/producer
- No Redis dependency

### Proto Convention

Following the existing codebase pattern:
- Source proto: `api/proto/template/template.proto`
- Generated Go code: `api/proto/templatev1/`
- Go import path: `github.com/smpp-server/smpp-server/api/proto/templatev1`

## Data Model

### Table: `templates`

```sql
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
```

### Table: `template_audit_log`

```sql
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

### Key Behaviors
- `variables` auto-extracted from `body` via regex `\{\{(\w+)\}\}` on create/update
- Unique constraint `(client_id, name)` — no duplicate template names per client
- Updating `body` resets `status` to `draft` (requires re-approval)
- Updating only `name` does not reset status
- No limit on templates per client
- **Audit log on delete:** before deleting a template, a `deleted` audit entry is written. The FK uses `ON DELETE SET NULL` so audit entries survive template deletion (with `template_id` becoming NULL).
- **Ownership validation:** all client-facing operations (CRUD) verify that `template.client_id` matches the authenticated client. Admin operations bypass this check.

### Migration

Up: `migrations/000009_create_template_tables.up.sql`
Down: `migrations/000009_create_template_tables.down.sql`

Note: migration number must be verified at implementation time to avoid collisions.

## gRPC API

### Proto: `api/proto/template/template.proto`

```protobuf
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
```

### Key Methods

**CreateTemplate:** `client_id`, `name`, `body` → creates template in `draft` status, auto-extracts variables, writes audit log entry.

**UpdateTemplate:** `id`, `client_id`, `name` (optional), `body` (optional) → updates template. `client_id` used for ownership validation. If `body` changes, status resets to `draft`, writes audit log with old/new body.

**DeleteTemplate:** `id`, `client_id` → deletes template after writing `deleted` audit entry. `client_id` used for ownership validation.

**ApproveTemplate:** `id`, `actor_id` → sets status to `approved`, writes audit log. Only works on `draft` templates.

**RejectTemplate:** `id`, `actor_id`, `reason` → sets status to `rejected` with reason, writes audit log. Only works on `draft` templates.

**RenderTemplate:** `template_id`, `client_id`, `variables` (map<string, string>) → validates template is `approved`, validates all required variables are provided, validates rendered text length <= 1600 chars, performs substitution, returns `rendered_text` and `template_name`.

**ListTemplates:** `client_id`, `status` (optional filter), `limit`, `offset` → paginated list of templates.

**GetTemplateAuditLog:** `template_id`, `limit`, `offset` → paginated list of audit entries ordered by created_at DESC.

## HTTP API

### Client Gateway

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/templates` | Create template (status: draft) |
| GET | `/api/v1/templates` | List own templates (filter by status, pagination) |
| GET | `/api/v1/templates/{id}` | Get template details |
| PUT | `/api/v1/templates/{id}` | Update template (name, body) |
| DELETE | `/api/v1/templates/{id}` | Delete template |
| GET | `/api/v1/templates/{id}/audit` | Template audit log (paginated) |

### Admin Gateway

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/v1/templates` | List templates (filter by client_id, status, pagination) |
| GET | `/admin/v1/templates/{id}` | Get template details |
| POST | `/admin/v1/templates/{id}/approve` | Approve template |
| POST | `/admin/v1/templates/{id}/reject` | Reject template (with reason) |
| GET | `/admin/v1/templates/{id}/audit` | Template audit log (paginated) |

### SendMessage Integration

Existing `POST /api/v1/sms/send` extended with optional fields:

```json
{
    "template_id": "uuid",
    "variables": {"name": "Ivan", "code": "1234"},
    "source": "+70001234567",
    "destination": "+79991234567"
}
```

**Logic in Client Gateway SMS handler:**
1. If `template_id` is set → call `RenderTemplate` gRPC → use `rendered_text` as `text`
2. If `text` and `template_id` both set → error (mutually exclusive)
3. If neither `text` nor `template_id` → error

### SendBatch Integration

Existing `POST /api/v1/sms/batch` extended similarly. Each message in the batch array can independently use either `text` or `template_id` + `variables`. The gateway renders each template-based message via `RenderTemplate` before passing to Messaging Service. This allows sending the same template to multiple recipients with different variables.

## Rendering and Validation

### Variable Parsing
- Regex: `\{\{(\w+)\}\}` extracts variable names from template body
- Example: `"Hello, {{name}}! Your code: {{code}}"` → `["name", "code"]`
- Stored in `variables` array for fast validation at send time
- Only `\w+` allowed (letters, digits, underscore)
- Duplicate variables in body allowed (same variable used multiple times)

### Rendering (RenderTemplate)
1. Load template from DB
2. Check `status == approved` → if not, error
3. Compare keys of `variables` map with `template.variables`:
   - Missing variables → error listing which are missing
   - Extra variables (not in template) → silently ignored
4. Validate each variable value: max 500 characters per value
5. `strings.NewReplacer` for substitution: `{{name}}` → value from map
6. Validate rendered text length <= 1600 characters → if exceeded, error
7. Return rendered text

### Validation on Create/Update
- `name`: required, 1-255 characters
- `body`: required, 1-1600 characters (10 SMS parts × 160 chars)
- Variable names: only `\w+`

### Status Reset on Update
- If `body` changes → status resets to `draft`, audit log entry written
- If only `name` changes → status preserved

## Integration Points

### Template Service (new)
- PostgreSQL read/write
- No dependencies on other services

### Client Gateway Changes
- New gRPC client to Template Service (port 9099)
- New handler file: `internal/gateway/client/handlers/templates.go`
- Route registration: `/api/v1/templates/*` in `router.go` — add `templateHandlers *handlers.TemplateHandlers` parameter to `SetupRouter`
- **Modify `handlers/sms.go`:** add `templatev1.TemplateServiceClient` to `SMSHandlers` struct. In `SendSMS` and `SendBatch`, if `template_id` present, call `RenderTemplate` and use result as `text`.
- `TEMPLATE_SERVICE_ADDR` environment variable
- Wire template client in `cmd/client-gateway/main.go`

### Admin Gateway Changes
- New gRPC client to Template Service
- New handler file: `internal/gateway/admin/handlers/templates.go`
- Route registration: `/admin/v1/templates/*` in `router.go` — add `templateHandlers *handlers.TemplateHandlers` parameter to `SetupRouter`
- `TEMPLATE_SERVICE_ADDR` environment variable
- Wire template client in `cmd/admin-gateway/main.go`

### Messaging Service — No Changes
Receives `text` as before. No knowledge of templates. No `template_id` stored in messages table.

### Docker Compose
New service `template-service` in `deployments/docker-compose.yml`:
- Image: `service-base.Dockerfile`
- Port: 9099 (gRPC), 2120 (metrics)
- Dependencies: postgres
- Build args: `SERVICE_NAME: template-service`, `SERVICE_PATH: cmd/services/template-service`
- Environment: `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` (matching existing services)

Add `TEMPLATE_SERVICE_ADDR=template-service:9099` to both gateway services.

### Dockerfile
Add protoc generation step for `api/proto/template/template.proto` in `service-base.Dockerfile`, following the existing pattern for other services.

## Configuration

Configuration via environment variables (matching existing service pattern):

```yaml
template:
  grpc_port: 9099
  metrics_port: 2120
  max_body_length: 1600
  max_variable_value_length: 500
  max_rendered_length: 1600
```
