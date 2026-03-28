# Portal Complete Features — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Довести клиентский портал до полноценного UX — подключить все бэкенд-сервисы (Templates, Billing, Tariffs, Lookup) и реализовать недостающие компоненты (payment adapter, email notifications, 2 новых RPC).

**Architecture:** Послойный подход — сначала proto + codegen, затем backend service changes, потом gateway handlers, и наконец frontend pages. Все 7 блоков независимы друг от друга на уровне gateway/frontend и могут выполняться параллельно.

**Tech Stack:** Go 1.24 (gorilla/mux, gRPC, pgx/v5, zerolog), React 19 + TypeScript + Tailwind CSS 4 + Radix UI, PostgreSQL 15+, Redis 7+, Kafka.

---

## File Structure

### Proto changes
- Modify: `api/proto/auth/auth.proto` — add UpdateAPIKey + ChangePassword RPCs
- Regenerate: `api/proto/authv1/` — generated Go code

### Backend service changes
- Modify: `internal/services/auth/grpc/server.go` — add UpdateAPIKey + ChangePassword handlers
- Modify: `internal/services/auth/application/auth_service.go` — add UpdateAPIKey + ChangePassword methods
- Modify: `internal/services/auth/application/ports.go` — add Update to APIKeyRepository
- Modify: `internal/services/auth/infrastructure/repository/api_key_repository.go` — add Update method

### Database migrations
- Create: `migrations/000044_add_low_balance_threshold.up.sql`
- Create: `migrations/000044_add_low_balance_threshold.down.sql`
- Create: `migrations/000045_add_free_plan.up.sql`
- Create: `migrations/000045_add_free_plan.down.sql`

### Portal Gateway — new handlers
- Create: `internal/gateway/portal/handlers/templates.go` — 7 handlers
- Create: `internal/gateway/portal/handlers/billing.go` — 4 handlers
- Create: `internal/gateway/portal/handlers/tariffs.go` — 4 handlers
- Create: `internal/gateway/portal/payment/provider.go` — PaymentProvider interface
- Create: `internal/gateway/portal/payment/stub.go` — StubPaymentProvider
- Modify: `internal/gateway/portal/handlers/messages.go` — add GetMessage + ExportCSV
- Modify: `internal/gateway/portal/handlers/api_keys.go` — add GetAPIKey + UpdateAPIKey
- Modify: `internal/gateway/portal/handlers/lookup.go` — add NumberLookup + BulkLookup + GetLookupStatus
- Modify: `internal/gateway/portal/handlers/profile.go` — add ChangePassword

### Portal Gateway — wiring
- Modify: `internal/gateway/portal/clients.go` — add TemplateClient, TarificationClient
- Modify: `internal/gateway/portal/router/router.go` — add all new routes
- Modify: `cmd/portal-gateway/main.go` — wire new handlers + clients

### Frontend — API client
- Modify: `portal-frontend/src/api/client.ts` — add templates, billing, tariffs, lookup, password APIs

### Frontend — new pages
- Create: `portal-frontend/src/pages/templates/TemplatesPage.tsx`
- Create: `portal-frontend/src/pages/billing/BillingPage.tsx`
- Create: `portal-frontend/src/pages/messages/MessageDetailPage.tsx`
- Create: `portal-frontend/src/pages/tariffs/TariffsPage.tsx`
- Create: `portal-frontend/src/pages/lookup/LookupPage.tsx`

### Frontend — modified pages
- Modify: `portal-frontend/src/pages/messages/MessagesPage.tsx` — add filters, CSV export, row click
- Modify: `portal-frontend/src/pages/api-keys/APIKeysPage.tsx` — add detail modal, edit form
- Modify: `portal-frontend/src/pages/profile/ProfilePage.tsx` — add change password form

### Frontend — routing & navigation
- Modify: `portal-frontend/src/App.tsx` — add new routes
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx` — add sidebar items

---

## Task 1: Proto — Add UpdateAPIKey and ChangePassword RPCs

**Files:**
- Modify: `api/proto/auth/auth.proto`

- [ ] **Step 1: Add UpdateAPIKey and ChangePassword to auth.proto**

Add after the `ListAPIKeys` RPC definition (line 30) and before `SetupTOTP`:

```protobuf
  // UpdateAPIKey обновляет API ключ (имя, scopes, IP, срок действия)
  rpc UpdateAPIKey(UpdateAPIKeyRequest) returns (UpdateAPIKeyResponse);

  // ChangePassword меняет пароль пользователя (требует текущий пароль)
  rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);
```

Add at the end of the file (after `RegisterClientResponse`):

```protobuf
// UpdateAPIKeyRequest представляет запрос на обновление API ключа
message UpdateAPIKeyRequest {
  string key_id = 1;
  string user_id = 2;
  string name = 3;
  repeated string scopes = 4;
  repeated string allowed_ips = 5;
  google.protobuf.Timestamp expires_at = 6;
}

// UpdateAPIKeyResponse представляет ответ на обновление API ключа
message UpdateAPIKeyResponse {
  APIKeyInfo key = 1;
}

// ChangePasswordRequest представляет запрос на смену пароля
message ChangePasswordRequest {
  string user_id = 1;
  string current_password = 2;
  string new_password = 3;
}

// ChangePasswordResponse представляет ответ на смену пароля
message ChangePasswordResponse {
  bool success = 1;
}
```

- [ ] **Step 2: Regenerate Go code**

```bash
cd /home/magomed/projects/sms
protoc --go_out=. --go-grpc_out=. api/proto/auth/auth.proto
```

If protoc is not installed or configured, manually add the method stubs to the generated files. The key requirement is that `authv1.AuthServiceServer` interface includes `UpdateAPIKey` and `ChangePassword` methods.

- [ ] **Step 3: Commit**

```bash
git add api/proto/auth/auth.proto api/proto/authv1/
git commit -m "proto: add UpdateAPIKey and ChangePassword RPCs to auth service"
```

---

## Task 2: Auth Service — Implement UpdateAPIKey and ChangePassword

**Files:**
- Modify: `internal/services/auth/application/ports.go`
- Modify: `internal/services/auth/infrastructure/repository/api_key_repository.go`
- Modify: `internal/services/auth/application/auth_service.go`
- Modify: `internal/services/auth/grpc/server.go`

- [ ] **Step 1: Add Update method to APIKeyRepository interface in ports.go**

Add to the `APIKeyRepository` interface:

```go
	Update(ctx context.Context, apiKey *domain.APIKey) error
```

- [ ] **Step 2: Implement Update in api_key_repository.go**

```go
// Update обновляет API ключ (имя, scopes, IP whitelist, срок действия)
func (r *APIKeyRepository) Update(ctx context.Context, apiKey *domain.APIKey) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE api_keys SET name = $1, expires_at = $2, updated_at = NOW()
		WHERE id = $3 AND active = true`,
		apiKey.Name, apiKey.ExpiresAt, apiKey.ID)
	if err != nil {
		return fmt.Errorf("update api_key: %w", err)
	}

	// Обновляем allowed_ips
	_, err = tx.ExecContext(ctx, `DELETE FROM api_key_allowed_ips WHERE api_key_id = $1`, apiKey.ID)
	if err != nil {
		return fmt.Errorf("delete old ips: %w", err)
	}
	for _, ip := range apiKey.AllowedIPs {
		_, err = tx.ExecContext(ctx, `INSERT INTO api_key_allowed_ips (api_key_id, ip_address) VALUES ($1, $2)`, apiKey.ID, ip)
		if err != nil {
			return fmt.Errorf("insert ip: %w", err)
		}
	}

	// Обновляем scopes
	_, err = tx.ExecContext(ctx, `DELETE FROM api_key_scopes WHERE api_key_id = $1`, apiKey.ID)
	if err != nil {
		return fmt.Errorf("delete old scopes: %w", err)
	}
	for _, scope := range apiKey.Scopes {
		_, err = tx.ExecContext(ctx, `INSERT INTO api_key_scopes (api_key_id, scope) VALUES ($1, $2)`, apiKey.ID, scope)
		if err != nil {
			return fmt.Errorf("insert scope: %w", err)
		}
	}

	return tx.Commit()
}
```

- [ ] **Step 3: Add UpdateAPIKey and ChangePassword methods to AuthService**

In `auth_service.go`:

```go
// UpdateAPIKey обновляет API ключ
func (s *AuthService) UpdateAPIKey(ctx context.Context, keyID, userID uuid.UUID, name string, scopes, allowedIPs []string, expiresAt *time.Time) (*domain.APIKey, error) {
	key, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if key.UserID != userID {
		return nil, fmt.Errorf("api key does not belong to user")
	}
	if !key.Active {
		return nil, fmt.Errorf("api key is revoked")
	}

	key.Name = name
	key.Scopes = scopes
	key.AllowedIPs = allowedIPs
	key.ExpiresAt = expiresAt

	if err := s.apiKeyRepo.Update(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

// ChangePassword меняет пароль пользователя
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if !s.passwordHasher.CheckPassword(currentPassword, user.PasswordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}
	if currentPassword == newPassword {
		return fmt.Errorf("new password must differ from current")
	}

	hash, err := s.passwordHasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.PasswordHash = hash
	return s.userRepo.Update(ctx, user)
}
```

- [ ] **Step 4: Add gRPC handlers in server.go**

```go
// UpdateAPIKey обрабатывает UpdateAPIKey RPC
func (s *Server) UpdateAPIKey(ctx context.Context, req *authv1.UpdateAPIKeyRequest) (*authv1.UpdateAPIKeyResponse, error) {
	if req.KeyId == "" {
		return nil, status.Error(codes.InvalidArgument, "key_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	keyID, err := uuid.Parse(req.KeyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key_id format")
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.AsTime()
		expiresAt = &t
	}

	key, err := s.authService.UpdateAPIKey(ctx, keyID, userID, req.Name, req.Scopes, req.AllowedIps, expiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if strings.Contains(err.Error(), "does not belong") || strings.Contains(err.Error(), "revoked") {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		log.Error().Err(err).Msg("ошибка обновления API ключа")
		return nil, status.Error(codes.Internal, "failed to update API key")
	}

	resp := &authv1.UpdateAPIKeyResponse{
		Key: &authv1.APIKeyInfo{
			Id:         key.ID.String(),
			Name:       key.Name,
			Prefix:     key.Prefix,
			Active:     key.Active,
			CreatedAt:  timestamppb.New(key.CreatedAt),
			Scopes:     key.Scopes,
			AllowedIps: key.AllowedIPs,
		},
	}
	if key.ExpiresAt != nil {
		resp.Key.ExpiresAt = timestamppb.New(*key.ExpiresAt)
	}
	if key.LastUsedAt != nil {
		resp.Key.LastUsedAt = timestamppb.New(*key.LastUsedAt)
	}
	return resp, nil
}

// ChangePassword обрабатывает ChangePassword RPC
func (s *Server) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.CurrentPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "current_password is required")
	}
	if req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "new_password is required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	err = s.authService.ChangePassword(ctx, userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if strings.Contains(err.Error(), "incorrect") {
			return nil, status.Error(codes.PermissionDenied, "current password is incorrect")
		}
		if strings.Contains(err.Error(), "at least") || strings.Contains(err.Error(), "must differ") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		log.Error().Err(err).Msg("ошибка смены пароля")
		return nil, status.Error(codes.Internal, "failed to change password")
	}

	return &authv1.ChangePasswordResponse{Success: true}, nil
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/auth/
git commit -m "feat: implement UpdateAPIKey and ChangePassword RPCs in auth service"
```

---

## Task 3: Database Migrations

**Files:**
- Create: `migrations/000044_add_low_balance_threshold.up.sql`
- Create: `migrations/000044_add_low_balance_threshold.down.sql`
- Create: `migrations/000045_add_free_plan.up.sql`
- Create: `migrations/000045_add_free_plan.down.sql`

- [ ] **Step 1: Create low_balance_threshold migration**

`migrations/000044_add_low_balance_threshold.up.sql`:
```sql
ALTER TABLE clients ADD COLUMN IF NOT EXISTS low_balance_threshold DECIMAL DEFAULT 100;
```

`migrations/000044_add_low_balance_threshold.down.sql`:
```sql
ALTER TABLE clients DROP COLUMN IF EXISTS low_balance_threshold;
```

- [ ] **Step 2: Create Free plan migration**

`migrations/000045_add_free_plan.up.sql`:
```sql
INSERT INTO subscription_plans (id, name, display_name, monthly_price_rub, max_sms_per_month, max_smpp_connections, max_users, active)
VALUES (gen_random_uuid(), 'free', 'Free', 0, 100, 1, 1, true)
ON CONFLICT (name) DO NOTHING;

UPDATE clients SET plan_id = (SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1)
WHERE plan_id IS NULL;
```

`migrations/000045_add_free_plan.down.sql`:
```sql
UPDATE clients SET plan_id = NULL WHERE plan_id = (SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1);
DELETE FROM subscription_plans WHERE name = 'free';
```

- [ ] **Step 3: Commit**

```bash
git add migrations/
git commit -m "feat: add migrations for low_balance_threshold and free plan"
```

---

## Task 4: Portal Gateway — Add TemplateClient and TarificationClient

**Files:**
- Modify: `internal/gateway/portal/clients.go`

- [ ] **Step 1: Add TemplateClient and TarificationClient to ServiceClients**

Add imports:
```go
templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
```

Add to `ServiceClients` struct:
```go
TemplateClient     templatev1.TemplateServiceClient
TarificationClient tarificationv1.TarificationServiceClient
```

Add to `ServiceAddresses` struct:
```go
Template      string
Tarification  string
```

Add connection blocks in `NewServiceClients` (same pattern as other services):
```go
// Подключение к Template Service
if addresses.Template != "" {
    conn, err := grpc.Dial(addresses.Template, opts...)
    if err != nil {
        clients.Close()
        return nil, fmt.Errorf("не удалось подключиться к Template Service: %w", err)
    }
    clients.TemplateClient = templatev1.NewTemplateServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}

// Подключение к Tarification Service
if addresses.Tarification != "" {
    conn, err := grpc.Dial(addresses.Tarification, opts...)
    if err != nil {
        clients.Close()
        return nil, fmt.Errorf("не удалось подключиться к Tarification Service: %w", err)
    }
    clients.TarificationClient = tarificationv1.NewTarificationServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/clients.go
git commit -m "feat: add TemplateClient and TarificationClient to portal gateway"
```

---

## Task 5: Gateway Handlers — Templates (Block 1)

**Files:**
- Create: `internal/gateway/portal/handlers/templates.go`

- [ ] **Step 1: Create templates.go with all 7 handlers**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// TemplateHandlers содержит handlers для работы с шаблонами
type TemplateHandlers struct {
	templateClient templatev1.TemplateServiceClient
}

// NewTemplateHandlers создает новый TemplateHandlers
func NewTemplateHandlers(templateClient templatev1.TemplateServiceClient) *TemplateHandlers {
	return &TemplateHandlers{templateClient: templateClient}
}

type createTemplateRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// CreateTemplate обрабатывает POST /templates
func (h *TemplateHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req createTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	if req.Body == "" {
		respondError(w, shared.ErrInvalidInput("Поле body обязательно"))
		return
	}

	resp, err := h.templateClient.CreateTemplate(r.Context(), &templatev1.CreateTemplateRequest{
		ClientId: clientID.String(),
		Name:     req.Name,
		Body:     req.Body,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания шаблона")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, templateToJSON(resp.Template))
}

// ListTemplates обрабатывает GET /templates
func (h *TemplateHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	statusFilter := r.URL.Query().Get("status")

	resp, err := h.templateClient.ListTemplates(r.Context(), &templatev1.ListTemplatesRequest{
		ClientId: clientID.String(),
		Status:   statusFilter,
		Limit:    perPage,
		Offset:   offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка шаблонов")
		respondGRPCError(w, err)
		return
	}

	templates := make([]map[string]interface{}, 0, len(resp.Templates))
	for _, t := range resp.Templates {
		templates = append(templates, templateToJSON(t))
	}

	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"templates":   templates,
		"total":       resp.Total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// GetTemplate обрабатывает GET /templates/{id}
func (h *TemplateHandlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}

	resp, err := h.templateClient.GetTemplate(r.Context(), &templatev1.GetTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToJSON(resp.Template))
}

type updateTemplateRequest struct {
	Name *string `json:"name,omitempty"`
	Body *string `json:"body,omitempty"`
}

// UpdateTemplate обрабатывает PUT /templates/{id}
func (h *TemplateHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}

	var req updateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &templatev1.UpdateTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	}
	if req.Name != nil {
		grpcReq.Name = req.Name
	}
	if req.Body != nil {
		grpcReq.Body = req.Body
	}

	resp, err := h.templateClient.UpdateTemplate(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, templateToJSON(resp.Template))
}

// DeleteTemplate обрабатывает DELETE /templates/{id}
func (h *TemplateHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}

	_, err := h.templateClient.DeleteTemplate(r.Context(), &templatev1.DeleteTemplateRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type renderTemplateRequest struct {
	Variables map[string]string `json:"variables"`
}

// RenderTemplate обрабатывает POST /templates/{id}/render
func (h *TemplateHandlers) RenderTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}

	var req renderTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	resp, err := h.templateClient.RenderTemplate(r.Context(), &templatev1.RenderTemplateRequest{
		TemplateId: id,
		ClientId:   clientID.String(),
		Variables:  req.Variables,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_text":  resp.RenderedText,
		"template_name": resp.TemplateName,
	})
}

// GetTemplateAuditLog обрабатывает GET /templates/{id}/audit
func (h *TemplateHandlers) GetTemplateAuditLog(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID шаблона обязателен"))
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage

	resp, err := h.templateClient.GetTemplateAuditLog(r.Context(), &templatev1.GetTemplateAuditLogRequest{
		TemplateId: id,
		Limit:      perPage,
		Offset:     offset,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := map[string]interface{}{
			"id":          e.Id,
			"template_id": e.TemplateId,
			"action":      e.Action,
			"actor_id":    e.ActorId,
			"actor_type":  e.ActorType,
		}
		if e.OldBody != "" {
			entry["old_body"] = e.OldBody
		}
		if e.NewBody != "" {
			entry["new_body"] = e.NewBody
		}
		if e.Reason != "" {
			entry["reason"] = e.Reason
		}
		if e.CreatedAt != nil {
			entry["created_at"] = e.CreatedAt.AsTime()
		}
		entries = append(entries, entry)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   resp.Total,
	})
}

func templateToJSON(t *templatev1.TemplateInfo) map[string]interface{} {
	if t == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":        t.Id,
		"client_id": t.ClientId,
		"name":      t.Name,
		"body":      t.Body,
		"variables": t.Variables,
		"status":    t.Status,
	}
	if t.RejectionReason != "" {
		m["rejection_reason"] = t.RejectionReason
	}
	if t.CreatedAt != nil {
		m["created_at"] = t.CreatedAt.AsTime()
	}
	if t.UpdatedAt != nil {
		m["updated_at"] = t.UpdatedAt.AsTime()
	}
	return m
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/templates.go
git commit -m "feat: add template handlers to portal gateway"
```

---

## Task 6: Gateway Handlers — Billing (Block 2)

**Files:**
- Create: `internal/gateway/portal/payment/provider.go`
- Create: `internal/gateway/portal/payment/stub.go`
- Create: `internal/gateway/portal/handlers/billing.go`

- [ ] **Step 1: Create payment provider interface**

`internal/gateway/portal/payment/provider.go`:

```go
package payment

import (
	"context"
	"time"
)

// PaymentProvider абстрактный интерфейс платёжной системы
type PaymentProvider interface {
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*PaymentSession, error)
	HandleCallback(ctx context.Context, raw []byte, signature string) (*PaymentResult, error)
}

// CreatePaymentRequest запрос на создание платежа
type CreatePaymentRequest struct {
	ClientID  string
	Amount    string // decimal as string
	Currency  string
	ReturnURL string
}

// PaymentSession сессия платежа
type PaymentSession struct {
	PaymentID  string
	PaymentURL string
	ExpiresAt  time.Time
}

// PaymentResult результат платежа
type PaymentResult struct {
	PaymentID string
	Status    string // success / failed / pending
	Amount    string
	Currency  string
	ClientID  string
}
```

- [ ] **Step 2: Create stub payment provider**

`internal/gateway/portal/payment/stub.go`:

```go
package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StubPaymentProvider эмулирует успешную оплату для разработки/тестов
type StubPaymentProvider struct{}

func NewStubPaymentProvider() *StubPaymentProvider {
	return &StubPaymentProvider{}
}

func (s *StubPaymentProvider) CreatePayment(_ context.Context, req CreatePaymentRequest) (*PaymentSession, error) {
	paymentID := uuid.New().String()
	return &PaymentSession{
		PaymentID:  paymentID,
		PaymentURL: fmt.Sprintf("/portal/v1/billing/top-up/callback?payment_id=%s&status=success", paymentID),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (s *StubPaymentProvider) HandleCallback(_ context.Context, _ []byte, _ string) (*PaymentResult, error) {
	return &PaymentResult{
		PaymentID: "stub",
		Status:    "success",
		Amount:    "0",
		Currency:  "RUB",
	}, nil
}
```

- [ ] **Step 3: Create billing handlers**

`internal/gateway/portal/handlers/billing.go`:

```go
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/payment"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// BillingHandlers содержит handlers для биллинга
type BillingHandlers struct {
	billingClient   billingv1.BillingServiceClient
	paymentProvider payment.PaymentProvider
}

// NewBillingHandlers создает новый BillingHandlers
func NewBillingHandlers(billingClient billingv1.BillingServiceClient, paymentProvider payment.PaymentProvider) *BillingHandlers {
	return &BillingHandlers{
		billingClient:   billingClient,
		paymentProvider: paymentProvider,
	}
}

// GetBalance обрабатывает GET /billing/balance
func (h *BillingHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	resp, err := h.billingClient.GetBalance(r.Context(), &billingv1.GetBalanceRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения баланса")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"client_id": resp.ClientId,
		"balance":   resp.Balance,
		"currency":  resp.Currency,
	}
	if resp.UpdatedAt != nil {
		result["updated_at"] = resp.UpdatedAt.AsTime()
	}
	respondJSON(w, http.StatusOK, result)
}

// GetTransactions обрабатывает GET /billing/transactions
func (h *BillingHandlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	page, perPage := parsePagination(r)
	offset := (page - 1) * perPage
	query := r.URL.Query()
	txType := query.Get("type")

	var dateFrom, dateTo *timestamppb.Timestamp
	if fromStr := query.Get("date_from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		}
	}
	if toStr := query.Get("date_to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			dateTo = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			dateTo = timestamppb.New(t.Add(24*time.Hour - time.Second))
		}
	}

	resp, err := h.billingClient.GetTransactionHistory(r.Context(), &billingv1.GetTransactionHistoryRequest{
		ClientId:        clientID.String(),
		From:            dateFrom,
		To:              dateTo,
		TransactionType: txType,
		Limit:           perPage,
		Offset:          offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории транзакций")
		respondGRPCError(w, err)
		return
	}

	transactions := make([]map[string]interface{}, 0, len(resp.Transactions))
	for _, tx := range resp.Transactions {
		t := map[string]interface{}{
			"transaction_id": tx.TransactionId,
			"type":           tx.Type,
			"amount":         tx.Amount,
			"currency":       tx.Currency,
			"balance_before": tx.BalanceBefore,
			"balance_after":  tx.BalanceAfter,
			"description":    tx.Description,
		}
		if tx.MessageId != "" {
			t["message_id"] = tx.MessageId
		}
		if tx.CreatedAt != nil {
			t["created_at"] = tx.CreatedAt.AsTime()
		}
		transactions = append(transactions, t)
	}

	totalPages := int32(0)
	if perPage > 0 && resp.Total > 0 {
		totalPages = (resp.Total + perPage - 1) / perPage
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"total":        resp.Total,
		"page":         page,
		"per_page":     perPage,
		"total_pages":  totalPages,
	})
}

type topUpRequest struct {
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
	ReturnURL string `json:"return_url"`
}

// TopUp обрабатывает POST /billing/top-up
func (h *BillingHandlers) TopUp(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req topUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Amount == "" {
		respondError(w, shared.ErrInvalidInput("Поле amount обязательно"))
		return
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}

	session, err := h.paymentProvider.CreatePayment(r.Context(), payment.CreatePaymentRequest{
		ClientID:  clientID.String(),
		Amount:    req.Amount,
		Currency:  req.Currency,
		ReturnURL: req.ReturnURL,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания платежа")
		respondError(w, shared.ErrInternalServer("Ошибка инициализации платежа"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"payment_id":  session.PaymentID,
		"payment_url": session.PaymentURL,
		"expires_at":  session.ExpiresAt,
	})
}

// TopUpCallback обрабатывает POST /billing/top-up/callback (public, без session)
func (h *BillingHandlers) TopUpCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Ошибка чтения тела запроса"))
		return
	}

	signature := r.Header.Get("X-Signature")
	result, err := h.paymentProvider.HandleCallback(r.Context(), body, signature)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обработки callback платежа")
		respondError(w, shared.ErrInvalidInput("Невалидный callback"))
		return
	}

	if result.Status == "success" && result.ClientID != "" {
		_, err = h.billingClient.AddCredits(r.Context(), &billingv1.AddCreditsRequest{
			ClientId:      result.ClientID,
			Amount:        result.Amount,
			Currency:      result.Currency,
			Description:   "Пополнение баланса (payment: " + result.PaymentID + ")",
			PaymentMethod: "online",
		})
		if err != nil {
			log.Error().Err(err).Str("payment_id", result.PaymentID).Msg("ошибка зачисления средств")
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/payment/ internal/gateway/portal/handlers/billing.go
git commit -m "feat: add billing handlers and payment adapter to portal gateway"
```

---

## Task 7: Gateway Handlers — Messages (Block 3)

**Files:**
- Modify: `internal/gateway/portal/handlers/messages.go`

- [ ] **Step 1: Add GetMessage and ExportCSV handlers**

Add to `messages.go` after existing handlers:

```go
// GetMessage обрабатывает GET /messages/{id}
func (h *MessageHandlers) GetMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID сообщения обязателен"))
		return
	}

	resp, err := h.messagingClient.GetMessageStatus(r.Context(), &messagingv1.GetMessageStatusRequest{
		MessageId: id,
		ClientId:  clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"message_id":    resp.MessageId,
		"source":        resp.Source,
		"destination":   resp.Destination,
		"text":          resp.Text,
		"status":        resp.Status,
		"segment_count": resp.SegmentCount,
	}
	if resp.ExternalId != "" {
		result["external_id"] = resp.ExternalId
	}
	if resp.CreatedAt != nil {
		result["created_at"] = resp.CreatedAt.AsTime()
	}
	if resp.SubmittedAt != nil {
		result["submitted_at"] = resp.SubmittedAt.AsTime()
	}
	if resp.DeliveredAt != nil {
		result["delivered_at"] = resp.DeliveredAt.AsTime()
	}
	if resp.FailedAt != nil {
		result["failed_at"] = resp.FailedAt.AsTime()
	}
	if resp.ProviderId != "" {
		result["provider_id"] = resp.ProviderId
	}
	if resp.RouteId != "" {
		result["route_id"] = resp.RouteId
	}
	if resp.SmppMessageId != "" {
		result["smpp_message_id"] = resp.SmppMessageId
	}

	respondJSON(w, http.StatusOK, result)
}

// ExportCSV обрабатывает GET /messages/export
func (h *MessageHandlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	query := r.URL.Query()
	statusFilter := query.Get("status")
	destination := query.Get("destination")

	var dateFrom, dateTo *timestamppb.Timestamp
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			dateFrom = timestamppb.New(t)
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			dateTo = timestamppb.New(t)
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			dateTo = timestamppb.New(t.Add(24*time.Hour - time.Second))
		}
	}

	resp, err := h.messagingClient.GetMessageHistory(r.Context(), &messagingv1.GetMessageHistoryRequest{
		ClientId:    clientID.String(),
		From:        dateFrom,
		To:          dateTo,
		Status:      statusFilter,
		Destination: destination,
		Limit:       50000,
		Offset:      0,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка экспорта сообщений")
		respondGRPCError(w, err)
		return
	}

	now := time.Now().Format("2006-01-02")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=messages_"+now+".csv")

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"id", "source", "destination", "text", "status", "segment_count", "created_at", "delivered_at"})

	for _, msg := range resp.Messages {
		createdAt := ""
		if msg.CreatedAt != nil {
			createdAt = msg.CreatedAt.AsTime().Format(time.RFC3339)
		}
		deliveredAt := ""
		if msg.DeliveredAt != nil {
			deliveredAt = msg.DeliveredAt.AsTime().Format(time.RFC3339)
		}
		csvWriter.Write([]string{
			msg.MessageId,
			msg.Source,
			msg.Destination,
			msg.Text,
			msg.Status,
			fmt.Sprintf("%d", msg.SegmentCount),
			createdAt,
			deliveredAt,
		})
	}

	csvWriter.Flush()
}
```

Add imports at top of file:
```go
"encoding/csv"
"fmt"

"github.com/gorilla/mux"
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/messages.go
git commit -m "feat: add message detail and CSV export handlers"
```

---

## Task 8: Gateway Handlers — API Keys (Block 4)

**Files:**
- Modify: `internal/gateway/portal/handlers/api_keys.go`

- [ ] **Step 1: Add GetAPIKey and UpdateAPIKey handlers**

Add to `api_keys.go`:

```go
// GetAPIKey обрабатывает GET /api-keys/{id}
func (h *APIKeyHandlers) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	keyID := mux.Vars(r)["id"]
	if keyID == "" {
		respondError(w, shared.ErrInvalidInput("ID ключа обязателен"))
		return
	}

	resp, err := h.authClient.ListAPIKeys(r.Context(), &authv1.ListAPIKeysRequest{
		UserId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	for _, key := range resp.Keys {
		if key.Id == keyID {
			result := map[string]interface{}{
				"id":          key.Id,
				"name":        key.Name,
				"prefix":      key.Prefix,
				"active":      key.Active,
				"scopes":      key.Scopes,
				"allowed_ips": key.AllowedIps,
				"created_at":  key.CreatedAt.AsTime(),
			}
			if key.ExpiresAt != nil {
				result["expires_at"] = key.ExpiresAt.AsTime()
			}
			if key.LastUsedAt != nil {
				result["last_used_at"] = key.LastUsedAt.AsTime()
			}
			respondJSON(w, http.StatusOK, result)
			return
		}
	}

	respondError(w, shared.ErrNotFound("API ключ не найден"))
}

type updateAPIKeyRequest struct {
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	AllowedIPs []string `json:"allowed_ips"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
}

// UpdateAPIKey обрабатывает PUT /api-keys/{id}
func (h *APIKeyHandlers) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	keyID := mux.Vars(r)["id"]
	if keyID == "" {
		respondError(w, shared.ErrInvalidInput("ID ключа обязателен"))
		return
	}

	var req updateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}

	grpcReq := &authv1.UpdateAPIKeyRequest{
		KeyId:      keyID,
		UserId:     userID.String(),
		Name:       req.Name,
		Scopes:     req.Scopes,
		AllowedIps: req.AllowedIPs,
	}
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат expires_at"))
			return
		}
		grpcReq.ExpiresAt = timestamppb.New(t)
	}

	resp, err := h.authClient.UpdateAPIKey(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"id":          resp.Key.Id,
		"name":        resp.Key.Name,
		"prefix":      resp.Key.Prefix,
		"active":      resp.Key.Active,
		"scopes":      resp.Key.Scopes,
		"allowed_ips": resp.Key.AllowedIps,
		"created_at":  resp.Key.CreatedAt.AsTime(),
	}
	if resp.Key.ExpiresAt != nil {
		result["expires_at"] = resp.Key.ExpiresAt.AsTime()
	}
	if resp.Key.LastUsedAt != nil {
		result["last_used_at"] = resp.Key.LastUsedAt.AsTime()
	}

	if h.auditPublisher != nil {
		event := audit.NewAuditEvent("", userID.String(), "api_key.updated", audit.ResourceAPIKey, keyID)
		event.IPAddress = getIPAddress(r)
		h.auditPublisher.Publish(r.Context(), event)
	}

	respondJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/api_keys.go
git commit -m "feat: add GetAPIKey and UpdateAPIKey handlers"
```

---

## Task 9: Gateway Handlers — Tariffs (Block 5)

**Files:**
- Create: `internal/gateway/portal/handlers/tariffs.go`

- [ ] **Step 1: Create tariffs.go**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// TariffHandlers содержит handlers для работы с тарифами
type TariffHandlers struct {
	clientClient       clientv1.ClientServiceClient
	tarificationClient tarificationv1.TarificationServiceClient
}

// NewTariffHandlers создает новый TariffHandlers
func NewTariffHandlers(clientClient clientv1.ClientServiceClient, tarificationClient tarificationv1.TarificationServiceClient) *TariffHandlers {
	return &TariffHandlers{
		clientClient:       clientClient,
		tarificationClient: tarificationClient,
	}
}

// GetCurrentTariff обрабатывает GET /tariffs/current
func (h *TariffHandlers) GetCurrentTariff(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	clientResp, err := h.clientClient.GetClient(r.Context(), &clientv1.GetClientRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения информации о клиенте")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"client_id":         clientResp.Client.ClientId,
		"plan_id":           clientResp.Client.PlanId,
		"monthly_sms_count": clientResp.Client.MonthlySmsCount,
	}

	if clientResp.Client.Plan != nil {
		result["plan"] = map[string]interface{}{
			"id":                   clientResp.Client.Plan.Id,
			"name":                 clientResp.Client.Plan.Name,
			"display_name":         clientResp.Client.Plan.DisplayName,
			"monthly_price_rub":    clientResp.Client.Plan.MonthlyPriceRub,
			"max_sms_per_month":    clientResp.Client.Plan.MaxSmsPerMonth,
			"max_smpp_connections": clientResp.Client.Plan.MaxSmppConnections,
			"max_users":            clientResp.Client.Plan.MaxUsers,
			"features":             clientResp.Client.Plan.Features,
		}
	}

	// Получаем usage counters если есть plan_id
	if clientResp.Client.PlanId != "" && h.tarificationClient != nil {
		usageResp, err := h.tarificationClient.ListUsageCounters(r.Context(), &tarificationv1.ListUsageCountersRequest{
			ClientId: clientID.String(),
			Limit:    100,
			Offset:   0,
		})
		if err == nil {
			counters := make([]map[string]interface{}, 0, len(usageResp.Counters))
			for _, c := range usageResp.Counters {
				counter := map[string]interface{}{
					"id":              c.Id,
					"tariff_plan_id":  c.TariffPlanId,
					"segment_count":   c.SegmentCount,
				}
				if c.UpdatedAt != nil {
					counter["updated_at"] = c.UpdatedAt.AsTime()
				}
				counters = append(counters, counter)
			}
			result["usage_counters"] = counters
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// ListAvailablePlans обрабатывает GET /tariffs/plans
func (h *TariffHandlers) ListAvailablePlans(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	resp, err := h.clientClient.ListPlans(r.Context(), &clientv1.ListPlansRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка планов")
		respondGRPCError(w, err)
		return
	}

	plans := make([]map[string]interface{}, 0, len(resp.Plans))
	for _, p := range resp.Plans {
		plans = append(plans, map[string]interface{}{
			"id":                   p.Id,
			"name":                 p.Name,
			"display_name":         p.DisplayName,
			"monthly_price_rub":    p.MonthlyPriceRub,
			"max_sms_per_month":    p.MaxSmsPerMonth,
			"max_smpp_connections": p.MaxSmppConnections,
			"max_users":            p.MaxUsers,
			"features":             p.Features,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"plans": plans})
}

type changePlanRequest struct {
	PlanID string `json:"plan_id"`
}

// ChangePlan обрабатывает POST /tariffs/change
func (h *TariffHandlers) ChangePlan(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req changePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.PlanID == "" {
		respondError(w, shared.ErrInvalidInput("Поле plan_id обязательно"))
		return
	}

	_, err := h.clientClient.AssignPlan(r.Context(), &clientv1.AssignPlanRequest{
		ClientId: clientID.String(),
		PlanId:   req.PlanID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Тарифный план успешно изменён",
	})
}

// GetUsage обрабатывает GET /tariffs/usage
func (h *TariffHandlers) GetUsage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	if h.tarificationClient == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"counters": []interface{}{}})
		return
	}

	resp, err := h.tarificationClient.ListUsageCounters(r.Context(), &tarificationv1.ListUsageCountersRequest{
		ClientId: clientID.String(),
		Limit:    100,
		Offset:   0,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения usage counters")
		respondGRPCError(w, err)
		return
	}

	counters := make([]map[string]interface{}, 0, len(resp.Counters))
	for _, c := range resp.Counters {
		counter := map[string]interface{}{
			"id":             c.Id,
			"client_id":     c.ClientId,
			"tariff_plan_id": c.TariffPlanId,
			"segment_count":  c.SegmentCount,
		}
		if c.UpdatedAt != nil {
			counter["updated_at"] = c.UpdatedAt.AsTime()
		}
		counters = append(counters, counter)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"counters": counters,
		"total":    resp.Total,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/tariffs.go
git commit -m "feat: add tariff handlers to portal gateway"
```

---

## Task 10: Gateway Handlers — Change Password (Block 6) + Lookup (Block 7)

**Files:**
- Modify: `internal/gateway/portal/handlers/profile.go`
- Modify: `internal/gateway/portal/handlers/lookup.go`

- [ ] **Step 1: Add ChangePassword handler to profile.go**

```go
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword обрабатывает PUT /profile/password
func (h *ProfileHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}

	clientID, _ := middleware.GetClientID(r.Context())

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.CurrentPassword == "" {
		respondError(w, shared.ErrInvalidInput("Поле current_password обязательно"))
		return
	}
	if req.NewPassword == "" {
		respondError(w, shared.ErrInvalidInput("Поле new_password обязательно"))
		return
	}
	if len(req.NewPassword) < 8 {
		respondError(w, shared.ErrInvalidInput("Новый пароль должен содержать минимум 8 символов"))
		return
	}

	_, err := h.authClient.ChangePassword(r.Context(), &authv1.ChangePasswordRequest{
		UserId:          userID.String(),
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	if h.auditPublisher != nil {
		event := audit.NewAuditEvent(clientID.String(), userID.String(), "password.changed", audit.ResourceAuth, userID.String())
		event.IPAddress = getIPAddress(r)
		h.auditPublisher.Publish(r.Context(), event)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Пароль успешно изменён",
	})
}
```

- [ ] **Step 2: Add NumberLookup, BulkLookup, GetLookupStatus to lookup.go**

```go
type numberLookupRequest struct {
	Phone string `json:"phone"`
}

// NumberLookup обрабатывает POST /lookup
func (h *LookupHandlers) NumberLookup(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req numberLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Phone == "" {
		respondError(w, shared.ErrInvalidInput("Поле phone обязательно"))
		return
	}

	resp, err := h.routingClient.NumberLookup(r.Context(), &routingv1.NumberLookupRequest{
		Msisdn:   req.Phone,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка number lookup")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"msisdn":          resp.Msisdn,
		"operator_mccmnc": resp.OperatorMccmnc,
		"operator_name":   resp.OperatorName,
		"number_status":   resp.NumberStatus.String(),
		"country_code":    resp.CountryCode,
		"number_type":     resp.NumberType.String(),
		"is_ported":       resp.IsPorted,
		"cached":          resp.Cached,
	}
	if resp.QueriedAt != nil {
		result["queried_at"] = resp.QueriedAt.AsTime()
	}

	respondJSON(w, http.StatusOK, result)
}

// BulkLookup обрабатывает POST /lookup/bulk
func (h *LookupHandlers) BulkLookup(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Парсинг multipart/form-data с CSV
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, shared.ErrInvalidInput("Ошибка парсинга формы"))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Файл не найден в запросе"))
		return
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	var msisdns []string
	for {
		record, err := csvReader.Read()
		if err != nil {
			break
		}
		if len(record) > 0 && record[0] != "" {
			msisdns = append(msisdns, strings.TrimSpace(record[0]))
		}
	}

	if len(msisdns) == 0 {
		respondError(w, shared.ErrInvalidInput("CSV файл пустой"))
		return
	}
	if len(msisdns) > 10000 {
		respondError(w, shared.ErrInvalidInput("Максимум 10 000 номеров"))
		return
	}

	resp, err := h.routingClient.BulkNumberLookup(r.Context(), &routingv1.BulkNumberLookupRequest{
		Msisdns:  msisdns,
		ClientId: clientID.String(),
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка bulk lookup")
		respondGRPCError(w, err)
		return
	}

	results := make([]map[string]interface{}, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, map[string]interface{}{
			"msisdn":          r.Msisdn,
			"operator_mccmnc": r.OperatorMccmnc,
			"operator_name":   r.OperatorName,
			"number_status":   r.NumberStatus.String(),
			"country_code":    r.CountryCode,
			"number_type":     r.NumberType.String(),
			"is_ported":       r.IsPorted,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results":       results,
		"total_count":   resp.TotalCount,
		"success_count": resp.SuccessCount,
		"failed_count":  resp.FailedCount,
	})
}
```

Add imports to lookup.go:
```go
"encoding/csv"
"encoding/json"
"strings"
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/profile.go internal/gateway/portal/handlers/lookup.go
git commit -m "feat: add ChangePassword, NumberLookup, and BulkLookup handlers"
```

---

## Task 11: Router + Main Wiring

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Update router.go — add new handler params and routes**

Add to `SetupRouter` function signature:
```go
templateHandlers *handlers.TemplateHandlers,
billingHandlers *handlers.BillingHandlers,
tariffHandlers *handlers.TariffHandlers,
```

Replace `notImplemented` references with actual handlers:

```go
// Profile — replace notImplemented
profile.HandleFunc("/password", profileHandlers.ChangePassword).Methods("PUT")
```

Replace message `notImplemented`:
```go
messages.HandleFunc("/export", messageHandlers.ExportCSV).Methods("GET")
messages.HandleFunc("/{id}", messageHandlers.GetMessage).Methods("GET")
```

Replace api-keys `notImplemented`:
```go
apiKeys.HandleFunc("/{id}", apiKeyHandlers.GetAPIKey).Methods("GET")
apiKeys.HandleFunc("/{id}", apiKeyHandlers.UpdateAPIKey).Methods("PUT")
```

Add new route groups:
```go
// Templates endpoints
templates := protected.PathPrefix("/templates").Subrouter()
templates.HandleFunc("", templateHandlers.CreateTemplate).Methods("POST")
templates.HandleFunc("", templateHandlers.ListTemplates).Methods("GET")
templates.HandleFunc("/{id}", templateHandlers.GetTemplate).Methods("GET")
templates.HandleFunc("/{id}", templateHandlers.UpdateTemplate).Methods("PUT")
templates.HandleFunc("/{id}", templateHandlers.DeleteTemplate).Methods("DELETE")
templates.HandleFunc("/{id}/render", templateHandlers.RenderTemplate).Methods("POST")
templates.HandleFunc("/{id}/audit", templateHandlers.GetTemplateAuditLog).Methods("GET")

// Billing endpoints
billing := protected.PathPrefix("/billing").Subrouter()
billing.HandleFunc("/balance", billingHandlers.GetBalance).Methods("GET")
billing.HandleFunc("/transactions", billingHandlers.GetTransactions).Methods("GET")
billing.HandleFunc("/top-up", billingHandlers.TopUp).Methods("POST")

// Tariff endpoints
tariffs := protected.PathPrefix("/tariffs").Subrouter()
tariffs.HandleFunc("/current", tariffHandlers.GetCurrentTariff).Methods("GET")
tariffs.HandleFunc("/plans", tariffHandlers.ListAvailablePlans).Methods("GET")
tariffs.HandleFunc("/change", tariffHandlers.ChangePlan).Methods("POST")
tariffs.HandleFunc("/usage", tariffHandlers.GetUsage).Methods("GET")

// Lookup endpoints — добавить к существующим
lookup.HandleFunc("", lookupHandlers.NumberLookup).Methods("POST")
lookup.HandleFunc("/bulk", lookupHandlers.BulkLookup).Methods("POST")
```

Add public route for billing callback (before protected block):
```go
// Billing callback — public, без session auth
portalV1.HandleFunc("/billing/top-up/callback", billingHandlers.TopUpCallback).Methods("POST")
```

- [ ] **Step 2: Update main.go — wire new services**

Add to serviceAddresses:
```go
Template:      getEnvOrDefault("TEMPLATE_SERVICE_ADDR", "localhost:9099"),
Tarification:  getEnvOrDefault("TARIFICATION_SERVICE_ADDR", "localhost:9100"),
```

Create new handlers:
```go
templateHandlers := handlers.NewTemplateHandlers(serviceClients.TemplateClient)
billingHandlers := handlers.NewBillingHandlers(serviceClients.BillingClient, payment.NewStubPaymentProvider())
tariffHandlers := handlers.NewTariffHandlers(serviceClients.ClientClient, serviceClients.TarificationClient)
```

Add imports:
```go
"github.com/smpp-server/smpp-server/internal/gateway/portal/payment"
```

Pass new handlers to `SetupRouter`:
```go
router := portalrouter.SetupRouter(
    // ... existing params ...
    templateHandlers,
    billingHandlers,
    tariffHandlers,
)
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat: wire all new handlers in portal gateway router and main"
```

---

## Task 12: Frontend — API Client Extensions

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add all new API functions**

Add to `client.ts`:

```typescript
// Templates API
export interface TemplateInfo {
  id: string;
  client_id: string;
  name: string;
  body: string;
  variables: string[];
  status: string;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
}

export const templatesApi = {
  list: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ templates: TemplateInfo[]; total: number; page: number; per_page: number; total_pages: number }>(`/templates?${qs}`);
  },
  get: (id: string) => apiFetch<TemplateInfo>(`/templates/${id}`),
  create: (data: { name: string; body: string }) =>
    apiFetch<TemplateInfo>('/templates', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { name?: string; body?: string }) =>
    apiFetch<TemplateInfo>(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/templates/${id}`, { method: 'DELETE' }),
  render: (id: string, variables: Record<string, string>) =>
    apiFetch<{ rendered_text: string; template_name: string }>(`/templates/${id}/render`, {
      method: 'POST',
      body: JSON.stringify({ variables }),
    }),
  auditLog: (id: string, params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ entries: unknown[]; total: number }>(`/templates/${id}/audit?${qs}`);
  },
};

// Billing API
export const billingApi = {
  getBalance: () => apiFetch<{ balance: string; currency: string; updated_at: string }>('/billing/balance'),
  getTransactions: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ transactions: unknown[]; total: number; page: number; per_page: number; total_pages: number }>(`/billing/transactions?${qs}`);
  },
  topUp: (amount: string, currency?: string, returnUrl?: string) =>
    apiFetch<{ payment_id: string; payment_url: string; expires_at: string }>('/billing/top-up', {
      method: 'POST',
      body: JSON.stringify({ amount, currency: currency || 'RUB', return_url: returnUrl || window.location.href }),
    }),
};

// Tariffs API
export const tariffsApi = {
  getCurrent: () => apiFetch<unknown>('/tariffs/current'),
  listPlans: () => apiFetch<{ plans: unknown[] }>('/tariffs/plans'),
  changePlan: (planId: string) =>
    apiFetch<unknown>('/tariffs/change', { method: 'POST', body: JSON.stringify({ plan_id: planId }) }),
  getUsage: () => apiFetch<{ counters: unknown[]; total: number }>('/tariffs/usage'),
};

// Lookup API
export const lookupApi = {
  single: (phone: string) =>
    apiFetch<unknown>('/lookup', { method: 'POST', body: JSON.stringify({ phone }) }),
  bulk: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return fetch(`${API_BASE}/lookup/bulk`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': getCookie('csrf_token') || '' },
      body: formData,
    }).then(async (res) => {
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
        throw new ApiError(res.status, err.error?.message || res.statusText, err.error);
      }
      return res.json();
    });
  },
  history: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/lookup/history?${qs}`);
  },
  stats: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/lookup/stats?${qs}`);
  },
};
```

Add to existing APIs:

```typescript
// Add to messagesApi:
export const messagesApi = {
  // ... existing
  get: (id: string) => apiFetch<unknown>(`/messages/${id}`),
  exportCsv: (params: Record<string, string> = {}) => {
    const qs = new URLSearchParams(params).toString();
    return fetch(`${API_BASE}/messages/export?${qs}`, {
      credentials: 'include',
      headers: { 'X-CSRF-Token': getCookie('csrf_token') || '' },
    });
  },
};

// Add to apiKeysApi:
export const apiKeysApi = {
  // ... existing
  get: (id: string) => apiFetch<APIKeyInfo>(`/api-keys/${id}`),
  update: (id: string, data: { name: string; scopes: string[]; allowed_ips: string[]; expires_at?: string }) =>
    apiFetch<APIKeyInfo>(`/api-keys/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
};

// Add to profileApi:
export const profileApi = {
  // ... existing
  changePassword: (currentPassword: string, newPassword: string) =>
    apiFetch<void>('/profile/password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    }),
};
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat: add templates, billing, tariffs, lookup API functions"
```

---

## Task 13: Frontend — TemplatesPage

**Files:**
- Create: `portal-frontend/src/pages/templates/TemplatesPage.tsx`

- [ ] **Step 1: Create TemplatesPage**

See full implementation in the spec. This page needs:
- Table of templates with status badges (pending/approved/rejected)
- Create/Edit modal with name, body (textarea with placeholder hints), category
- Preview button → modal for entering test variables → show rendered result
- Edit hidden for `approved` status
- Rejection reason shown for `rejected` status

The page follows the same pattern as APIKeysPage/WebhooksPage — CRUD with modals.

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/templates/
git commit -m "feat: add TemplatesPage to portal frontend"
```

---

## Task 14: Frontend — BillingPage

**Files:**
- Create: `portal-frontend/src/pages/billing/BillingPage.tsx`

- [ ] **Step 1: Create BillingPage**

Page includes:
- Balance card (current balance, currency, last update)
- "Пополнить" button → modal with amount input → redirect to payment
- Transactions table with date, type, amount, balance_after, description, message_id
- Filters by date range and transaction type
- Pagination

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/billing/
git commit -m "feat: add BillingPage to portal frontend"
```

---

## Task 15: Frontend — MessageDetailPage + MessagesPage enhancements

**Files:**
- Create: `portal-frontend/src/pages/messages/MessageDetailPage.tsx`
- Modify: `portal-frontend/src/pages/messages/MessagesPage.tsx`

- [ ] **Step 1: Create MessageDetailPage**

Shows:
- Message details card: source, destination, text, status, segment_count
- Delivery timeline (visual): created → submitted → delivered/failed
- Provider info: provider_id, route_id, smpp_message_id

- [ ] **Step 2: Update MessagesPage**

Add:
- Row click → navigate to `/messages/{id}`
- Filter panel: status multiselect, date range, destination
- "Export CSV" button with current filters

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/messages/
git commit -m "feat: add MessageDetailPage and enhance MessagesPage"
```

---

## Task 16: Frontend — TariffsPage

**Files:**
- Create: `portal-frontend/src/pages/tariffs/TariffsPage.tsx`

- [ ] **Step 1: Create TariffsPage**

Page includes:
- Current plan card: name, price, limits, progress bar (X of Y SMS)
- Available plans section: pricing cards with "Выбрать" button → ConfirmDialog
- Usage counters display

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/tariffs/
git commit -m "feat: add TariffsPage to portal frontend"
```

---

## Task 17: Frontend — LookupPage

**Files:**
- Create: `portal-frontend/src/pages/lookup/LookupPage.tsx`

- [ ] **Step 1: Create LookupPage**

Page uses Radix Tabs with two tabs:
- "Одиночный": phone input → "Проверить" → result card (operator, country, MNP, status)
- "Массовый": file upload (drag-and-drop or button) → results table → download CSV
- History table from `/lookup/history`
- Stats card from `/lookup/stats`

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/lookup/
git commit -m "feat: add LookupPage to portal frontend"
```

---

## Task 18: Frontend — API Keys + Profile enhancements

**Files:**
- Modify: `portal-frontend/src/pages/api-keys/APIKeysPage.tsx`
- Modify: `portal-frontend/src/pages/profile/ProfilePage.tsx`

- [ ] **Step 1: Enhance APIKeysPage**

Add:
- Click on key row → detail modal showing: name, prefix, scopes, allowed_ips, expires_at, last_used_at
- "Редактировать" button → edit form: name, scopes checkboxes, IP whitelist textarea, expires date picker

- [ ] **Step 2: Add change password form to ProfilePage**

Add a "Сменить пароль" section after existing content:
- Current password field
- New password field
- Confirm new password field
- Client-side validation (match check, min 8 chars)
- Submit calls `profileApi.changePassword()`

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/api-keys/ portal-frontend/src/pages/profile/
git commit -m "feat: enhance APIKeysPage and ProfilePage with editing and password change"
```

---

## Task 19: Frontend — Routing + Sidebar Navigation

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add routes to App.tsx**

Add imports:
```typescript
import { TemplatesPage } from './pages/templates/TemplatesPage';
import { BillingPage } from './pages/billing/BillingPage';
import { MessageDetailPage } from './pages/messages/MessageDetailPage';
import { TariffsPage } from './pages/tariffs/TariffsPage';
import { LookupPage } from './pages/lookup/LookupPage';
```

Add routes inside `<Route element={<RequireAuth />}>`:
```tsx
<Route path="/templates" element={<TemplatesPage />} />
<Route path="/billing" element={<BillingPage />} />
<Route path="/messages/:id" element={<MessageDetailPage />} />
<Route path="/tariffs" element={<TariffsPage />} />
<Route path="/lookup" element={<LookupPage />} />
```

- [ ] **Step 2: Update sidebar navigation in UserLayout.tsx**

Update `USER_NAV` array — add new items after existing ones in logical order:
```typescript
const USER_NAV: NavItem[] = [
  { path: '/dashboard', label: 'Дашборд' },
  { path: '/messages', label: 'Сообщения' },
  { path: '/templates', label: 'Шаблоны' },
  { path: '/contact-lists', label: 'Контакты' },
  { path: '/campaigns', label: 'Рассылки' },
  { path: '/lookup', label: 'Lookup' },
  { path: '/providers', label: 'Провайдеры' },
  { path: '/billing', label: 'Биллинг' },
  { path: '/tariffs', label: 'Тарифы' },
  { path: '/api-keys', label: 'API Ключи' },
  { path: '/webhooks', label: 'Вебхуки' },
  { path: '/analytics', label: 'Аналитика' },
  { path: '/sub-accounts', label: 'Суб-аккаунты' },
  { path: '/profile', label: 'Профиль' },
  { path: '/audit-log', label: 'Журнал аудита' },
];
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat: add routes and sidebar navigation for all new pages"
```

---

## Task 20: Verify Build

- [ ] **Step 1: Build Go gateway**

```bash
cd /home/magomed/projects/sms
go build ./cmd/portal-gateway/...
```

- [ ] **Step 2: Build frontend**

```bash
cd /home/magomed/projects/sms/portal-frontend
npm run build
```

- [ ] **Step 3: Fix any compilation errors**

Address imports, type mismatches, or missing dependencies.

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "fix: resolve build errors for portal complete features"
```
