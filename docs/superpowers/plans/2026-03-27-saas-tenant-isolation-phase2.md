# SaaS Tenant Isolation — Phase 2: Логирование + Self-Service Registration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Завершить tenant-aware логирование (остаток раздела 5.1 спеки) и реализовать публичную self-service регистрацию с выбором плана и sandbox-режимом (раздел 5.2).

**Architecture:** Tenant-aware логирование достигается добавлением `client_id` в zerolog-контекст после прохождения auth middleware — это enrichment-middleware между auth и бизнес-логикой. Публичная регистрация — новый gRPC RPC `RegisterClient` в auth.proto + публичный HTTP-эндпоинт в portal gateway. Sandbox-режим — флаг `is_sandbox` на клиенте, который маршрутизирует SMS в фейковый провайдер.

**Tech Stack:** Go 1.24.0, gorilla/mux, gRPC, zerolog, pgx/v5, Redis, React 19 + TypeScript + Vite

---

## Структура файлов

| Действие | Файл | Ответственность |
|----------|------|-----------------|
| Modify | `internal/api/middleware/logging.go` | Добавить tenant_id enrichment после auth |
| Modify | `internal/gateway/client/middleware/logging.go` | Добавить tenant_id enrichment после auth |
| Create | `internal/api/middleware/tenant_logger.go` | Отдельный middleware для обогащения логов tenant_id |
| Create | `internal/gateway/client/middleware/tenant_logger.go` | Аналог для client gateway |
| Create | `internal/gateway/portal/middleware/tenant_logger.go` | Аналог для portal gateway |
| Modify | `internal/gateway/client/router/router.go` | Подключить tenant_logger middleware |
| Modify | `internal/gateway/portal/router/router.go` | Подключить tenant_logger middleware |
| Modify | `cmd/api/main.go` | Подключить tenant_logger middleware |
| Create | `internal/api/middleware/tenant_logger_test.go` | Тесты tenant logger |
| Modify | `api/proto/auth/auth.proto` | Добавить RPC RegisterClient |
| Modify | `api/proto/client/client.proto` | Добавить RPC RegisterWithPlan |
| Modify | `internal/services/auth/grpc/server.go` | Реализация RegisterClient |
| Modify | `internal/services/client/application/client_service.go` | Метод RegisterWithPlan |
| Modify | `internal/services/client/grpc/server.go` | gRPC handler RegisterWithPlan |
| Modify | `internal/gateway/portal/handlers/auth.go` | HTTP handler Register |
| Modify | `internal/gateway/portal/router/router.go` | Маршрут POST /auth/register |
| Modify | `internal/services/client/domain/client.go` | Флаг IsSandbox |
| Create | `migrations/000032_add_client_sandbox_flag.up.sql` | Миграция is_sandbox |
| Create | `migrations/000032_add_client_sandbox_flag.down.sql` | Откат |
| Modify | `internal/services/client/infrastructure/repository/client_repository.go` | Поддержка is_sandbox |
| Modify | `portal-frontend/src/pages/auth/RegisterPage.tsx` | Форма регистрации |
| Modify | `portal-frontend/src/App.tsx` | Маршрут /register |
| Create | `tests/integration/registration_test.go` | Интеграционные тесты |

---

### Task 1: Tenant-Aware Logging Middleware (API Gateway)

**Files:**
- Create: `internal/api/middleware/tenant_logger.go`
- Create: `internal/api/middleware/tenant_logger_test.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/api/middleware/tenant_logger_test.go
package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestTenantLoggerMiddleware_AddsClientID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	clientID := uuid.MustParse("a1111111-1111-1111-1111-111111111111")

	handler := TenantLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем логгер из контекста и пишем сообщение
		l := zerolog.Ctx(r.Context())
		l.Info().Msg("test")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), ClientIDKey, clientID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Contains(t, buf.String(), `"tenant_id":"a1111111-1111-1111-1111-111111111111"`)
}

func TestTenantLoggerMiddleware_NoClientID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	handler := TenantLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := zerolog.Ctx(r.Context())
		l.Info().Msg("test")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Без client_id — логгер всё равно должен работать, но без tenant_id
	assert.NotContains(t, buf.String(), `"tenant_id"`)
	assert.Contains(t, buf.String(), `"test"`)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/magomed/projects/sms && go test ./internal/api/middleware/ -run TestTenantLoggerMiddleware -v`
Expected: FAIL — `TenantLoggerMiddleware` не определён.

- [ ] **Step 3: Write the TenantLoggerMiddleware**

```go
// internal/api/middleware/tenant_logger.go
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TenantLoggerMiddleware обогащает zerolog-контекст полем tenant_id,
// извлекая client_id из request context (установленный auth middleware).
// Должен стоять ПОСЛЕ auth middleware в цепочке.
func TenantLoggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, ok := r.Context().Value(ClientIDKey).(uuid.UUID)
			if ok && clientID != uuid.Nil {
				enriched := logger.With().Str("tenant_id", clientID.String()).Logger()
				ctx := enriched.WithContext(r.Context())
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/magomed/projects/sms && go test ./internal/api/middleware/ -run TestTenantLoggerMiddleware -v`
Expected: PASS

- [ ] **Step 5: Wire into API gateway**

In `cmd/api/main.go`, add `tenantLoggerMiddleware` after `authMiddleware`:

```go
tenantLoggerMiddleware := middleware.TenantLoggerMiddleware(logger)
```

And pass it to `SetupRouter`. В `internal/api/http/router.go` (или аналогичном файле), добавить `router.Use(tenantLoggerMiddleware)` после `authMiddleware`.

- [ ] **Step 6: Commit**

```bash
git add internal/api/middleware/tenant_logger.go internal/api/middleware/tenant_logger_test.go cmd/api/main.go
git commit -m "feat(logging): add tenant-aware logging middleware for API gateway"
```

---

### Task 2: Tenant-Aware Logging Middleware (Client Gateway + Portal Gateway)

**Files:**
- Create: `internal/gateway/client/middleware/tenant_logger.go`
- Create: `internal/gateway/portal/middleware/tenant_logger.go`
- Modify: `internal/gateway/client/router/router.go`
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/client-gateway/main.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Create client gateway tenant logger**

```go
// internal/gateway/client/middleware/tenant_logger.go
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TenantLoggerMiddleware обогащает zerolog-контекст полем tenant_id.
// Должен стоять ПОСЛЕ auth middleware в цепочке.
func TenantLoggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, ok := r.Context().Value(ClientIDKey).(uuid.UUID)
			if ok && clientID != uuid.Nil {
				enriched := logger.With().Str("tenant_id", clientID.String()).Logger()
				ctx := enriched.WithContext(r.Context())
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Create portal gateway tenant logger**

```go
// internal/gateway/portal/middleware/tenant_logger.go
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type contextKey string

const ClientIDKey contextKey = "client_id"

// TenantLoggerMiddleware обогащает zerolog-контекст полем tenant_id.
// Должен стоять ПОСЛЕ session auth middleware.
func TenantLoggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, ok := r.Context().Value(ClientIDKey).(uuid.UUID)
			if ok && clientID != uuid.Nil {
				enriched := logger.With().Str("tenant_id", clientID.String()).Logger()
				ctx := enriched.WithContext(r.Context())
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

**Note:** Проверить, какой тип `contextKey` используется в portal gateway auth middleware. Если это `string`, а не `contextKey`, использовать тот же тип. Проверить `internal/gateway/portal/middleware/auth.go` для точного типа.

- [ ] **Step 3: Wire into client gateway router**

In `internal/gateway/client/router/router.go`, добавить параметр `tenantLoggerMiddleware func(http.Handler) http.Handler` и вставить его после `authMiddleware`:

```go
router.Use(recoveryMiddleware)
router.Use(loggingMiddleware)
router.Use(corsMiddleware)
router.Use(authMiddleware)
router.Use(tenantLoggerMiddleware) // NEW: после auth, до rate-limit
router.Use(rateLimitMiddleware)
router.Use(quotaMiddleware)
```

- [ ] **Step 4: Wire into portal gateway router**

In `internal/gateway/portal/router/router.go`, добавить `tenantLoggerMiddleware` после `sessionAuthMiddleware` в protected subrouter:

```go
protected := portalV1.PathPrefix("").Subrouter()
protected.Use(sessionAuthMiddleware)
protected.Use(tenantLoggerMiddleware) // NEW
protected.Use(csrfMiddleware)
```

- [ ] **Step 5: Update main.go files to create and pass tenant logger**

В `cmd/client-gateway/main.go` и `cmd/portal-gateway/main.go`:
```go
tenantLoggerMiddleware := middleware.TenantLoggerMiddleware(logger)
```
И передать в SetupRouter.

- [ ] **Step 6: Run existing tests**

Run: `cd /home/magomed/projects/sms && go build ./...`
Expected: компиляция без ошибок.

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/client/middleware/tenant_logger.go internal/gateway/portal/middleware/tenant_logger.go internal/gateway/client/router/router.go internal/gateway/portal/router/router.go cmd/client-gateway/main.go cmd/portal-gateway/main.go
git commit -m "feat(logging): add tenant-aware logging to client and portal gateways"
```

---

### Task 3: Sandbox Flag — Миграция и Домен

**Files:**
- Create: `migrations/000032_add_client_sandbox_flag.up.sql`
- Create: `migrations/000032_add_client_sandbox_flag.down.sql`
- Modify: `internal/services/client/domain/client.go`
- Modify: `internal/services/client/infrastructure/repository/client_repository.go`

- [ ] **Step 1: Create migration**

```sql
-- migrations/000032_add_client_sandbox_flag.up.sql
ALTER TABLE clients ADD COLUMN is_sandbox BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN clients.is_sandbox IS 'Sandbox mode: SMS отправляются в фейковый провайдер, не тратят реальный баланс';
```

```sql
-- migrations/000032_add_client_sandbox_flag.down.sql
ALTER TABLE clients DROP COLUMN IF EXISTS is_sandbox;
```

- [ ] **Step 2: Add IsSandbox to domain model**

In `internal/services/client/domain/client.go`, добавить поле в структуру `Client`:

```go
// После поля MonthlySMSResetAt:
IsSandbox bool `json:"is_sandbox" db:"is_sandbox"`
```

- [ ] **Step 3: Update client repository — Create**

In `internal/services/client/infrastructure/repository/client_repository.go`, добавить `is_sandbox` во все SQL-запросы:
- INSERT: добавить колонку и placeholder
- SELECT: добавить в список колонок и Scan
- UPDATE: добавить в SET

Найти INSERT запрос в методе `Create` и добавить `is_sandbox` в колонки и значения.
Найти SELECT в `GetByID` и `List` и добавить `is_sandbox` в Scan.

- [ ] **Step 4: Update proto — add is_sandbox to ClientInfo**

In `api/proto/client/client.proto`, добавить в `ClientInfo` message:

```protobuf
bool is_sandbox = 20; // Sandbox mode
```

И в `CreateClientRequest`:
```protobuf
bool is_sandbox = 8; // Create in sandbox mode
```

- [ ] **Step 5: Regenerate proto**

Run: `cd /home/magomed/projects/sms && make proto` (или `protoc` команда, используемая в проекте)

- [ ] **Step 6: Verify compilation**

Run: `cd /home/magomed/projects/sms && go build ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add migrations/000032_* internal/services/client/domain/client.go internal/services/client/infrastructure/repository/client_repository.go api/proto/client/client.proto api/proto/clientv1/
git commit -m "feat(client): add is_sandbox flag for sandbox mode"
```

---

### Task 4: Public Registration — Auth Proto + gRPC

**Files:**
- Modify: `api/proto/auth/auth.proto`
- Modify: `internal/services/auth/grpc/server.go`

- [ ] **Step 1: Add RegisterClient RPC to auth.proto**

In `api/proto/auth/auth.proto`, добавить в `AuthService`:

```protobuf
// RegisterClient регистрирует нового клиента (public self-service)
rpc RegisterClient(RegisterClientRequest) returns (RegisterClientResponse);
```

И добавить messages:

```protobuf
// RegisterClientRequest — запрос на self-service регистрацию
message RegisterClientRequest {
  string email = 1;           // Email (будет логином)
  string password = 2;        // Пароль
  string company_name = 3;    // Название компании/клиента
  string contact_person = 4;  // Контактное лицо
  string phone = 5;           // Телефон
  string plan_name = 6;       // Название плана: "free", "starter", "business", "pro"
}

// RegisterClientResponse — ответ на self-service регистрацию
message RegisterClientResponse {
  string client_id = 1;       // ID созданного клиента
  string user_id = 2;         // ID созданного пользователя
  string session_id = 3;      // Сессия (сразу залогинен)
  UserInfo user = 4;          // Информация о пользователе
}
```

- [ ] **Step 2: Regenerate auth proto**

Run: `cd /home/magomed/projects/sms && make proto`

- [ ] **Step 3: Implement RegisterClient in auth gRPC server**

In `internal/services/auth/grpc/server.go`, добавить метод:

```go
func (s *Server) RegisterClient(ctx context.Context, req *authv1.RegisterClientRequest) (*authv1.RegisterClientResponse, error) {
	// 1. Валидация
	if req.Email == "" || req.Password == "" || req.CompanyName == "" {
		return nil, status.Error(codes.InvalidArgument, "email, password and company_name are required")
	}

	// 2. Проверить, что email не занят
	_, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "email already registered")
	}

	// 3. Создать клиента через client service gRPC
	clientResp, err := s.clientService.CreateClient(ctx, &clientv1.CreateClientRequest{
		Name:          req.CompanyName,
		Email:         req.Email,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		Active:        true,
		IsSandbox:     true, // Новые клиенты начинают в sandbox
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create client during registration")
		return nil, status.Error(codes.Internal, "failed to create client")
	}

	// 4. Назначить план
	if req.PlanName != "" {
		_, err = s.clientService.AssignPlan(ctx, &clientv1.AssignPlanRequest{
			ClientId: clientResp.ClientId,
			PlanName: req.PlanName,
		})
		if err != nil {
			log.Warn().Err(err).Str("plan", req.PlanName).Msg("failed to assign plan, using default")
		}
	}

	// 5. Хешировать пароль
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	// 6. Создать пользователя с ролью "client"
	clientRole, err := s.roleRepo.GetByName(ctx, "client")
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get client role")
	}

	user := &domain.User{
		ID:           uuid.New(),
		Username:     req.Email,
		Email:        req.Email,
		PasswordHash: passwordHash,
		RoleID:       clientRole.ID,
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	// 7. Создать сессию (авто-логин)
	sessionID, err := s.sessionRepo.Create(ctx, user.ID, clientResp.ClientId, req.Email, "")
	if err != nil {
		log.Warn().Err(err).Msg("failed to create session after registration")
	}

	return &authv1.RegisterClientResponse{
		ClientId:  clientResp.ClientId,
		UserId:    user.ID.String(),
		SessionId: sessionID,
		User: &authv1.UserInfo{
			Id:       user.ID.String(),
			Username: user.Username,
			Email:    user.Email,
			Role:     &authv1.Role{Name: "client"},
			Active:   true,
		},
	}, nil
}
```

**Note:** Нужно убедиться, что auth gRPC server имеет доступ к `clientService` (gRPC клиент к client service). Если нет — добавить поле в Server struct и передать при инициализации в `main.go`.

- [ ] **Step 4: Verify compilation**

Run: `cd /home/magomed/projects/sms && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add api/proto/auth/ api/proto/authv1/ internal/services/auth/grpc/server.go
git commit -m "feat(auth): add RegisterClient RPC for self-service registration"
```

---

### Task 5: Portal Registration HTTP Handler

**Files:**
- Modify: `internal/gateway/portal/handlers/auth.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add Register handler**

In `internal/gateway/portal/handlers/auth.go`, добавить:

```go
// registerRequest представляет запрос на регистрацию
type registerRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	CompanyName   string `json:"company_name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	PlanName      string `json:"plan_name"`
}

// Register обрабатывает POST /auth/register
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Email == "" || req.Password == "" || req.CompanyName == "" {
		respondError(w, shared.ErrInvalidInput("Поля email, password и company_name обязательны"))
		return
	}

	if len(req.Password) < 8 {
		respondError(w, shared.ErrInvalidInput("Пароль должен содержать минимум 8 символов"))
		return
	}

	resp, err := h.authClient.RegisterClient(r.Context(), &authv1.RegisterClientRequest{
		Email:         req.Email,
		Password:      req.Password,
		CompanyName:   req.CompanyName,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		PlanName:      req.PlanName,
	})
	if err != nil {
		log.Error().Err(err).Str("email", req.Email).Msg("ошибка регистрации")
		respondGRPCError(w, err)
		return
	}

	// Устанавливаем сессию (авто-логин после регистрации)
	if resp.SessionId != "" {
		setSessionCookies(w, resp.SessionId)
	}

	// Audit event
	h.publishAuditEvent(r, resp.User, audit.ActionRegister, resp.ClientId)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"client_id": resp.ClientId,
		"user":      buildUserInfoResponse(resp.User),
	})
}
```

- [ ] **Step 2: Add audit.ActionRegister constant**

Проверить `internal/shared/audit/` — если `ActionRegister` не определён, добавить:

```go
const ActionRegister = "register"
```

- [ ] **Step 3: Add route to portal router**

In `internal/gateway/portal/router/router.go`, добавить в секцию публичных маршрутов auth:

```go
auth.HandleFunc("/register", authHandlers.Register).Methods("POST")
```

- [ ] **Step 4: Verify compilation**

Run: `cd /home/magomed/projects/sms && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/auth.go internal/gateway/portal/router/router.go internal/shared/audit/
git commit -m "feat(portal): add public registration endpoint POST /portal/v1/auth/register"
```

---

### Task 6: Frontend — Страница регистрации

**Files:**
- Create: `portal-frontend/src/pages/auth/RegisterPage.tsx`
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/api/auth.ts` (или аналогичный файл API)

- [ ] **Step 1: Create API function for registration**

Найти файл с API-функциями для auth (вероятно `portal-frontend/src/api/auth.ts` или `portal-frontend/src/api/portal.ts`) и добавить:

```typescript
export interface RegisterRequest {
  email: string;
  password: string;
  company_name: string;
  contact_person?: string;
  phone?: string;
  plan_name: string;
}

export interface RegisterResponse {
  client_id: string;
  user: {
    id: string;
    username: string;
    email: string;
    role: string;
  };
}

export async function register(data: RegisterRequest): Promise<RegisterResponse> {
  const resp = await fetch('/portal/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  if (!resp.ok) {
    const err = await resp.json();
    throw new Error(err.error?.message || 'Ошибка регистрации');
  }
  return resp.json();
}
```

- [ ] **Step 2: Create RegisterPage component**

```tsx
// portal-frontend/src/pages/auth/RegisterPage.tsx
import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { register } from '../../api/auth';
import { useAuth } from '../../contexts/AuthContext';

const PLANS = [
  { name: 'free', display: 'Free', price: '0 ₽/мес', desc: '1 000 SMS, 1 подключение' },
  { name: 'starter', display: 'Starter', price: '5 000 ₽/мес', desc: '50 000 SMS, 1 подключение' },
  { name: 'business', display: 'Business', price: '15 000 ₽/мес', desc: '300 000 SMS, 5 подключений' },
  { name: 'pro', display: 'Pro', price: '40 000 ₽/мес', desc: '1 500 000 SMS, безлимит подключений' },
];

export default function RegisterPage() {
  const navigate = useNavigate();
  const { refreshUser } = useAuth();
  const [form, setForm] = useState({
    email: '',
    password: '',
    company_name: '',
    contact_person: '',
    phone: '',
    plan_name: 'free',
  });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await register(form);
      await refreshUser();
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const update = (field: string) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm(prev => ({ ...prev, [field]: e.target.value }));

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>Регистрация</h1>
        <p className="auth-subtitle">Создайте аккаунт и начните работу за 10 минут</p>

        {error && <div className="alert alert-error">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="company_name">Название компании *</label>
            <input id="company_name" type="text" required value={form.company_name} onChange={update('company_name')} />
          </div>

          <div className="form-group">
            <label htmlFor="email">Email *</label>
            <input id="email" type="email" required value={form.email} onChange={update('email')} />
          </div>

          <div className="form-group">
            <label htmlFor="password">Пароль * (мин. 8 символов)</label>
            <input id="password" type="password" required minLength={8} value={form.password} onChange={update('password')} />
          </div>

          <div className="form-group">
            <label htmlFor="contact_person">Контактное лицо</label>
            <input id="contact_person" type="text" value={form.contact_person} onChange={update('contact_person')} />
          </div>

          <div className="form-group">
            <label htmlFor="phone">Телефон</label>
            <input id="phone" type="tel" value={form.phone} onChange={update('phone')} />
          </div>

          <div className="form-group">
            <label>Тарифный план</label>
            <div className="plan-selector">
              {PLANS.map(plan => (
                <label key={plan.name} className={`plan-option ${form.plan_name === plan.name ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="plan"
                    value={plan.name}
                    checked={form.plan_name === plan.name}
                    onChange={() => setForm(prev => ({ ...prev, plan_name: plan.name }))}
                  />
                  <div className="plan-info">
                    <strong>{plan.display}</strong>
                    <span className="plan-price">{plan.price}</span>
                    <span className="plan-desc">{plan.desc}</span>
                  </div>
                </label>
              ))}
            </div>
          </div>

          <button type="submit" className="btn btn-primary" disabled={loading}>
            {loading ? 'Регистрация...' : 'Создать аккаунт'}
          </button>
        </form>

        <p className="auth-footer">
          Уже есть аккаунт? <Link to="/login">Войти</Link>
        </p>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Add route in App.tsx**

In `portal-frontend/src/App.tsx`, добавить импорт и маршрут:

```tsx
import RegisterPage from './pages/auth/RegisterPage';

// В Routes, рядом с /login:
<Route path="/register" element={<RegisterPage />} />
```

- [ ] **Step 4: Add link to registration from login page**

In `portal-frontend/src/pages/auth/LoginPage.tsx`, добавить ссылку:

```tsx
<p className="auth-footer">
  Нет аккаунта? <Link to="/register">Зарегистрироваться</Link>
</p>
```

- [ ] **Step 5: Verify frontend builds**

Run: `cd /home/magomed/projects/sms/portal-frontend && npm run build`
Expected: PASS (или `npx tsc --noEmit` для type-check)

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/auth/RegisterPage.tsx portal-frontend/src/App.tsx portal-frontend/src/pages/auth/LoginPage.tsx portal-frontend/src/api/
git commit -m "feat(frontend): add registration page with plan selection"
```

---

### Task 7: Integration Test — Registration Flow

**Files:**
- Create: `tests/integration/registration_test.go`

- [ ] **Step 1: Write integration test**

```go
// tests/integration/registration_test.go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistration_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	portalURL := getPortalURL(t) // helper: читает из env PORTAL_URL или default

	body := map[string]string{
		"email":          "test-reg@example.com",
		"password":       "securePassword123",
		"company_name":   "Test Company",
		"contact_person": "Test User",
		"phone":          "+79001234567",
		"plan_name":      "free",
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(portalURL+"/portal/v1/auth/register", "application/json", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["client_id"])
	assert.NotNil(t, result["user"])

	// Проверяем, что сессионная cookie установлена
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "portal_session" {
			sessionCookie = c
			break
		}
	}
	assert.NotNil(t, sessionCookie, "portal_session cookie should be set")
}

func TestRegistration_DuplicateEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	portalURL := getPortalURL(t)

	body := map[string]string{
		"email":        "duplicate@example.com",
		"password":     "securePassword123",
		"company_name": "Duplicate Co",
		"plan_name":    "free",
	}
	jsonBody, _ := json.Marshal(body)

	// Первая регистрация — успех
	resp1, err := http.Post(portalURL+"/portal/v1/auth/register", "application/json", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	resp1.Body.Close()
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	// Вторая регистрация с тем же email — ошибка
	resp2, err := http.Post(portalURL+"/portal/v1/auth/register", "application/json", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusConflict, resp2.StatusCode)
}

func TestRegistration_InvalidInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	portalURL := getPortalURL(t)

	body := map[string]string{
		"email":    "invalid@example.com",
		"password": "short",
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(portalURL+"/portal/v1/auth/register", "application/json", bytes.NewReader(jsonBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func getPortalURL(t *testing.T) string {
	t.Helper()
	url := "http://localhost:8082" // default portal gateway port
	// Override from env if needed
	return url
}
```

- [ ] **Step 2: Run integration tests (requires running services)**

Run: `cd /home/magomed/projects/sms && go test ./tests/integration/ -run TestRegistration -v -count=1`
Expected: PASS (если сервисы запущены), иначе SKIP (short mode)

- [ ] **Step 3: Commit**

```bash
git add tests/integration/registration_test.go
git commit -m "test(integration): add registration flow integration tests"
```

---

### Task 8: Sandbox Mode — Routing Logic

**Files:**
- Modify: `internal/services/client/application/client_service.go` — метод для toggle sandbox
- Modify: routing/message processing code — проверка is_sandbox перед отправкой

- [ ] **Step 1: Add ToggleSandbox method to ClientService**

In `internal/services/client/application/client_service.go`:

```go
// ToggleSandbox переключает sandbox-режим для клиента
func (s *ClientService) ToggleSandbox(ctx context.Context, clientID uuid.UUID, enable bool) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		if err == clientrepo.ErrClientNotFound {
			return ErrClientNotFound
		}
		return err
	}

	client.IsSandbox = enable
	client.UpdatedAt = time.Now()

	return s.clientRepo.Update(ctx, client)
}
```

- [ ] **Step 2: Add sandbox check to SMS sending pipeline**

Найти место, где SMS передаётся в Kafka/SMPP провайдеру. Если клиент `is_sandbox=true`, SMS должен:
1. Записываться в БД со статусом `delivered` (мгновенно)
2. НЕ отправляться через SMPP

Конкретная реализация зависит от пайплайна. Проверить `internal/api/http/handlers.go` или `internal/gateway/client/handlers/sms.go` — где SendSMS обрабатывается.

В месте, где сообщение отправляется в очередь, добавить:

```go
// Если sandbox — не отправляем в реальную очередь, а сразу пишем как delivered
if client.IsSandbox {
	msg.Status = "delivered"
	msg.DeliveredAt = time.Now()
	// Сохранить в БД без отправки в Kafka
	if err := s.messageRepo.UpdateStatus(ctx, msg.ID, msg.Status); err != nil {
		log.Error().Err(err).Msg("failed to save sandbox message")
	}
	return // не отправляем в Kafka
}
```

**Note:** Точная имплементация зависит от архитектуры отправки. Инженер должен найти точку интеграции в пайплайне.

- [ ] **Step 3: Add gRPC RPC for toggling sandbox**

In `api/proto/client/client.proto`, добавить:

```protobuf
rpc ToggleSandbox(ToggleSandboxRequest) returns (ToggleSandboxResponse);

message ToggleSandboxRequest {
  string client_id = 1;
  bool enable = 2;
}

message ToggleSandboxResponse {
  bool is_sandbox = 1;
}
```

- [ ] **Step 4: Implement gRPC handler**

In `internal/services/client/grpc/server.go`:

```go
func (s *Server) ToggleSandbox(ctx context.Context, req *clientv1.ToggleSandboxRequest) (*clientv1.ToggleSandboxResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if err := s.clientService.ToggleSandbox(ctx, clientID, req.Enable); err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		return nil, status.Error(codes.Internal, "failed to toggle sandbox")
	}

	return &clientv1.ToggleSandboxResponse{
		IsSandbox: req.Enable,
	}, nil
}
```

- [ ] **Step 5: Regenerate proto and verify**

Run: `cd /home/magomed/projects/sms && make proto && go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/services/client/application/client_service.go api/proto/client/ api/proto/clientv1/ internal/services/client/grpc/server.go
git commit -m "feat(sandbox): add sandbox mode toggle and routing bypass"
```

---

### Task 9: Portal — Sandbox Toggle UI + Profile Display

**Files:**
- Modify: `portal-frontend/src/pages/ProfilePage.tsx` (или DashboardPage)
- Modify: portal API layer

- [ ] **Step 1: Add sandbox toggle to profile/dashboard**

Добавить на страницу дашборда или профиля индикатор sandbox-режима и кнопку выхода из sandbox (переключение в production mode):

```tsx
{profile.is_sandbox && (
  <div className="sandbox-banner">
    <span>🔬 Sandbox Mode — SMS не отправляются реально</span>
    <button onClick={handleDisableSandbox} className="btn btn-sm">
      Перейти в Production
    </button>
  </div>
)}
```

- [ ] **Step 2: Add portal handler for sandbox toggle**

In portal gateway, добавить endpoint:
```
PUT /portal/v1/profile/sandbox
Body: { "enable": false }
```

Который вызывает `clientService.ToggleSandbox` gRPC.

- [ ] **Step 3: Verify frontend builds**

Run: `cd /home/magomed/projects/sms/portal-frontend && npm run build`

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/ internal/gateway/portal/
git commit -m "feat(portal): add sandbox mode indicator and toggle"
```

---

### Task 10: Plans Listing — Public API Endpoint

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Create: `internal/gateway/portal/handlers/plans.go`

- [ ] **Step 1: Create plans handler**

```go
// internal/gateway/portal/handlers/plans.go
package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/api/proto/clientv1"
)

// PlansHandlers содержит handlers для работы с тарифными планами
type PlansHandlers struct {
	clientService clientv1.ClientServiceClient
}

// NewPlansHandlers создает PlansHandlers
func NewPlansHandlers(clientService clientv1.ClientServiceClient) *PlansHandlers {
	return &PlansHandlers{clientService: clientService}
}

// ListPlans обрабатывает GET /plans — публичный список тарифов
func (h *PlansHandlers) ListPlans(w http.ResponseWriter, r *http.Request) {
	resp, err := h.clientService.ListPlans(r.Context(), &clientv1.ListPlansRequest{})
	if err != nil {
		log.Error().Err(err).Msg("failed to list plans")
		respondError(w, &shared.AppError{
			HTTPStatus: http.StatusInternalServerError,
			Code:       "PLANS_ERROR",
			Message:    "Не удалось загрузить тарифы",
		})
		return
	}

	plans := make([]map[string]interface{}, 0, len(resp.Plans))
	for _, p := range resp.Plans {
		plans = append(plans, map[string]interface{}{
			"name":                p.Name,
			"display_name":       p.DisplayName,
			"monthly_price_rub":  p.MonthlyPriceRub,
			"max_sms_per_month":  p.MaxSmsPerMonth,
			"max_smpp_connections": p.MaxSmppConnections,
			"max_users":          p.MaxUsers,
			"features":           p.Features,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plans": plans,
	})
}
```

- [ ] **Step 2: Add public route**

In `internal/gateway/portal/router/router.go`, в секцию публичных маршрутов:

```go
// Plans — публичный список тарифов (без auth)
portalV1.HandleFunc("/plans", plansHandlers.ListPlans).Methods("GET")
```

Добавить `plansHandlers *handlers.PlansHandlers` в параметры `SetupRouter`.

- [ ] **Step 3: Wire plansHandlers in main.go**

In `cmd/portal-gateway/main.go`:

```go
plansHandlers := handlers.NewPlansHandlers(clientServiceClient)
```

И передать в `SetupRouter`.

- [ ] **Step 4: Verify compilation**

Run: `cd /home/magomed/projects/sms && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/plans.go internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(portal): add public GET /plans endpoint for pricing page"
```
