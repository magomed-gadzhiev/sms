# QA Testing Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Исправить 4 бага, выявленных при QA-тестировании: отсутствующий маршрут admin audit, несоответствие полей analytics API, отсутствие client_id при создании пользователя, UX валидации баланса суб-аккаунта.

**Architecture:** Задачи независимы, каждая — точечное исправление в конкретных файлах. Задачи 1-3 затрагивают Go backend + TypeScript frontend. Задача 4 — только frontend.

**Tech Stack:** Go 1.24, gorilla/mux, google.golang.org/grpc, TypeScript 5.7 + React 19, Tailwind CSS 4.2, Radix UI

---

## Карта файлов

### Задача 1: /admin/v1/audit
- **Создать:** `internal/gateway/admin/handlers/audit.go`
- **Изменить:** `internal/gateway/admin/clients.go` — добавить AuditClient + адрес
- **Изменить:** `internal/gateway/admin/router/router.go` — добавить параметр + маршрут

### Задача 2: Analytics groups/rows
- **Изменить:** `portal-frontend/src/api/admin.ts` — переименовать поле интерфейса
- **Изменить:** `portal-frontend/src/pages/admin/AnalyticsPage.tsx` — обновить использование

### Задача 3: client_id при создании пользователя
- **Изменить:** `api/proto/auth/auth.proto` — добавить поле в CreateUserRequest
- **Изменить:** `api/proto/authv1/auth.pb.go` + `auth_grpc.pb.go` — перегенерировать
- **Изменить:** `internal/services/auth/grpc/server.go` — использовать client_id из запроса
- **Изменить:** `internal/services/auth/application/auth_service.go` — принять clientID параметр
- **Изменить:** `internal/gateway/admin/handlers/users.go` — принять и передать client_id
- **Изменить:** `portal-frontend/src/api/admin.ts` — добавить client_id в тип create
- **Изменить:** `portal-frontend/src/pages/admin/users/UsersPage.tsx` — добавить поле в форму

### Задача 4: UX баланса суб-аккаунта
- **Изменить:** `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`

---

## Task 1: Admin Audit Route

**Files:**
- Create: `internal/gateway/admin/handlers/audit.go`
- Modify: `internal/gateway/admin/clients.go`
- Modify: `internal/gateway/admin/router/router.go`

- [ ] **Step 1: Создать файл admin audit handler**

```go
// internal/gateway/admin/handlers/audit.go
package handlers

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AdminAuditHandlers содержит handlers для аудит-логов (admin)
type AdminAuditHandlers struct {
	auditClient auditv1.AuditServiceClient
}

// NewAdminAuditHandlers создает новый AdminAuditHandlers
func NewAdminAuditHandlers(auditClient auditv1.AuditServiceClient) *AdminAuditHandlers {
	return &AdminAuditHandlers{
		auditClient: auditClient,
	}
}

// ListAuditLog обрабатывает GET /admin/v1/audit
// Admin видит все записи (без фильтрации по tenant_id).
func (h *AdminAuditHandlers) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	if h.auditClient == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"entries":     []interface{}{},
			"total":       0,
			"page":        1,
			"total_pages": 0,
		})
		return
	}

	query := r.URL.Query()
	page, perPage := parsePagination(r)

	req := &auditv1.QueryAuditLogRequest{
		TenantId: query.Get("client_id"), // опциональная фильтрация по клиенту
		Action:   query.Get("action"),
		UserId:   query.Get("user_id"),
		Page:     page,
		PerPage:  perPage,
	}

	if from := query.Get("date_from"); from != "" {
		parsed, err := time.Parse("2006-01-02", from)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_from, ожидается YYYY-MM-DD"))
			return
		}
		req.DateFrom = timestamppb.New(parsed)
	}
	if to := query.Get("date_to"); to != "" {
		parsed, err := time.Parse("2006-01-02", to)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат date_to, ожидается YYYY-MM-DD"))
			return
		}
		req.DateTo = timestamppb.New(parsed.Add(24*time.Hour - time.Second))
	}

	resp, err := h.auditClient.QueryAuditLog(r.Context(), req)
	if err != nil {
		log.Error().Err(err).Msg("ошибка запроса аудит-лога (admin)")
		respondGRPCError(w, err)
		return
	}

	entries := make([]map[string]interface{}, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		entry := map[string]interface{}{
			"id":            e.Id,
			"tenant_id":     e.TenantId,
			"user_id":       e.UserId,
			"action":        e.Action,
			"resource_type": e.ResourceType,
			"resource_id":   e.ResourceId,
			"details":       e.Details,
			"ip_address":    e.IpAddress,
		}
		if e.CreatedAt != nil {
			entry["created_at"] = e.CreatedAt.AsTime().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries":     entries,
		"total":       resp.Total,
		"page":        resp.Page,
		"total_pages": resp.TotalPages,
	})
}
```

- [ ] **Step 2: Добавить AuditClient в admin ServiceClients**

Открыть `internal/gateway/admin/clients.go`.

Добавить импорт в блок импортов:
```go
"github.com/smpp-server/smpp-server/api/proto/auditv1"
```

В структуру `ServiceClients` (после `SenderNameClient`):
```go
AuditClient        auditv1.AuditServiceClient
```

В структуру `ServiceAddresses` (после `Tarification string`):
```go
Audit          string
```

В функцию `NewServiceClients` (после блока Tarification):
```go
// Подключение к Audit Service
if addresses.Audit != "" {
    conn, err := grpc.Dial(addresses.Audit, opts...)
    if err != nil {
        clients.Close()
        return nil, fmt.Errorf("не удалось подключиться к Audit Service: %w", err)
    }
    clients.AuditClient = auditv1.NewAuditServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}
```

- [ ] **Step 3: Обновить admin router — добавить параметр и маршрут**

В `internal/gateway/admin/router/router.go`:

Добавить параметр в сигнатуру `SetupRouter` (после `connectionsHandlers`):
```go
auditHandlers *handlers.AdminAuditHandlers,
```

Добавить маршрут в конец блока `adminV1` (перед регистрацией health endpoints):
```go
// Audit log endpoint
adminV1.HandleFunc("/audit", auditHandlers.ListAuditLog).Methods("GET")
```

- [ ] **Step 4: Найти точку вызова SetupRouter в admin gateway и передать auditHandlers**

Выполнить: `grep -rn "router.SetupRouter\|SetupRouter(" internal/gateway/admin/ --include="*.go"`

В найденном файле создать и передать audit handlers:
```go
auditHandlers := handlers.NewAdminAuditHandlers(clients.AuditClient)
```
Добавить `auditHandlers` в вызов `router.SetupRouter(...)`.

- [ ] **Step 5: Убедиться что код компилируется**

```bash
cd c:/projects/sms && go build ./internal/gateway/admin/...
```

Ожидается: успешная компиляция без ошибок.

- [ ] **Step 6: Коммит**

```bash
git add internal/gateway/admin/handlers/audit.go \
        internal/gateway/admin/clients.go \
        internal/gateway/admin/router/router.go
git commit -m "feat(admin): add /admin/v1/audit route for audit log"
```

---

## Task 2: Analytics API — groups vs rows

**Files:**
- Modify: `portal-frontend/src/api/admin.ts:247-256`
- Modify: `portal-frontend/src/pages/admin/AnalyticsPage.tsx:99`

- [ ] **Step 1: Обновить интерфейс в admin.ts**

В `portal-frontend/src/api/admin.ts` найти (строки ~232-256):
```typescript
export interface GroupedStatRow {
```
Переименовать в `GroupedStatGroup`.

Найти (строки ~247-256):
```typescript
export interface GroupedStatsResponse {
  rows: GroupedStatRow[];
```
Заменить на:
```typescript
export interface GroupedStatsResponse {
  groups: GroupedStatGroup[];
```

- [ ] **Step 2: Обновить использование в AnalyticsPage.tsx**

В `portal-frontend/src/pages/admin/AnalyticsPage.tsx` строка ~99:
```typescript
const rows = grouped?.rows ?? [];
```
Заменить на:
```typescript
const rows = grouped?.groups ?? [];
```

- [ ] **Step 3: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Ожидается: 0 ошибок, связанных с `GroupedStatRow`/`GroupedStatGroup`/`rows`/`groups`.

- [ ] **Step 4: Коммит**

```bash
git add portal-frontend/src/api/admin.ts \
        portal-frontend/src/pages/admin/AnalyticsPage.tsx
git commit -m "fix(analytics): align frontend field name groups with backend response"
```

---

## Task 3: client_id при создании пользователя

**Files:**
- Modify: `api/proto/auth/auth.proto`
- Modify: `api/proto/authv1/auth.pb.go` + `auth_grpc.pb.go` (перегенерация)
- Modify: `internal/services/auth/grpc/server.go`
- Modify: `internal/services/auth/application/auth_service.go`
- Modify: `internal/gateway/admin/handlers/users.go`
- Modify: `portal-frontend/src/api/admin.ts`
- Modify: `portal-frontend/src/pages/admin/users/UsersPage.tsx`

- [ ] **Step 1: Обновить proto — добавить client_id в CreateUserRequest**

В `api/proto/auth/auth.proto` найти блок `CreateUserRequest` (строки ~374-380):
```proto
message CreateUserRequest {
  string username = 1;
  string email = 2;
  string password = 3;
  string role_id = 4;
  bool active = 5;
}
```
Заменить на:
```proto
message CreateUserRequest {
  string username = 1;
  string email = 2;
  string password = 3;
  string role_id = 4;
  bool active = 5;
  string client_id = 6;  // Опционально: привязать клиента при создании
}
```

- [ ] **Step 2: Перегенерировать proto**

```bash
cd c:/projects/sms && grep -r "protoc\|buf generate\|generate.go" Makefile scripts/ 2>/dev/null | head -5
```

Если есть Makefile-таргет:
```bash
make proto
```

Если нет — найти команду генерации:
```bash
grep -rn "protoc\|buf" Makefile 2>/dev/null | head -10
```

После перегенерации убедиться, что в `api/proto/authv1/auth.pb.go` появилось поле `ClientId string`.

- [ ] **Step 3: Обновить auth gRPC server — использовать client_id**

В `internal/services/auth/grpc/server.go` найти функцию `CreateUser` (строка ~871).

Заменить тело после валидации `roleID` (строка ~890):
```go
user, err := s.authService.CreateUser(ctx, req.Username, req.Email, req.Password, roleID, req.Active)
```
На:
```go
var clientID *uuid.UUID
if req.ClientId != "" {
    parsed, err := uuid.Parse(req.ClientId)
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
    }
    clientID = &parsed
}

user, err := s.authService.CreateUser(ctx, req.Username, req.Email, req.Password, roleID, req.Active, clientID)
```

- [ ] **Step 4: Обновить AuthService.CreateUser — принять clientID**

В `internal/services/auth/application/auth_service.go` найти функцию `CreateUser` (строка ~372).

Изменить сигнатуру с:
```go
func (s *AuthService) CreateUser(ctx context.Context, username, email, password string, roleID uuid.UUID, active bool) (*domain.User, error) {
```
На:
```go
func (s *AuthService) CreateUser(ctx context.Context, username, email, password string, roleID uuid.UUID, active bool, clientID *uuid.UUID) (*domain.User, error) {
```

В теле функции, после строки `Active: active,` добавить:
```go
ClientID:     clientID,
```

- [ ] **Step 5: Обновить admin HTTP handler — принять client_id**

В `internal/gateway/admin/handlers/users.go` в функции `CreateUser` (строка ~88):

Добавить поле `ClientID string` в структуру запроса:
```go
var req struct {
    Username string `json:"username"`
    Email    string `json:"email"`
    Password string `json:"password"`
    RoleID   string `json:"role_id"`
    Active   bool   `json:"active"`
    ClientID string `json:"client_id"`
}
```

Добавить `ClientId: req.ClientID` в gRPC-вызов:
```go
resp, err := h.authClient.CreateUser(r.Context(), &authv1.CreateUserRequest{
    Username: req.Username,
    Email:    req.Email,
    Password: req.Password,
    RoleId:   req.RoleID,
    Active:   req.Active,
    ClientId: req.ClientID,
})
```

- [ ] **Step 6: Убедиться что backend компилируется**

```bash
cd c:/projects/sms && go build ./...
```

Ожидается: компиляция без ошибок.

- [ ] **Step 7: Обновить frontend API client**

В `portal-frontend/src/api/admin.ts` строка ~630:
```typescript
create: (data: { username: string; email: string; password: string; role_id: string; active?: boolean }) =>
```
Заменить на:
```typescript
create: (data: { username: string; email: string; password: string; role_id: string; active?: boolean; client_id?: string }) =>
```

- [ ] **Step 8: Обновить форму создания пользователя**

В `portal-frontend/src/pages/admin/users/UsersPage.tsx`:

**8a.** Добавить `client_id: ''` в `createForm` state (строка ~47):
```typescript
const [createForm, setCreateForm] = useState({
  username: '',
  email: '',
  password: '',
  role_id: '',
  active: true,
  client_id: '',
});
```

**8b.** Добавить state для списка клиентов рядом с `roles` state:
```typescript
const [clients, setClients] = useState<{ id: string; name: string }[]>([]);
```

**8c.** В `useEffect` где загружаются roles (строка ~62), добавить загрузку клиентов:
```typescript
useEffect(() => {
  rolesApi.list().then((res) => setRoles(res.roles || [])).catch(() => {});
  clientsApi.list({ limit: 500 }).then((res) => setClients(res.clients || [])).catch(() => {});
}, []);
```

**8d.** Добавить `clientOptions` рядом с `roleOptions`:
```typescript
const clientOptions = clients.map((c) => ({ value: c.id, label: c.name }));
```

**8e.** В модальном окне создания (после Select роли, строка ~335), добавить поле клиента:
```tsx
<Select
  label="Клиент (опционально)"
  options={clientOptions}
  value={createForm.client_id}
  onChange={(v) => setCreateForm({ ...createForm, client_id: v })}
  placeholder="Без клиента"
/>
```

**8f.** В функции `handleCreate` — убедиться что `client_id` передаётся в API вызов. Найти вызов `usersApi.create(createForm)` и убедиться что `createForm` содержит `client_id` (уже передаётся через spread).

- [ ] **Step 9: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Ожидается: 0 новых ошибок.

- [ ] **Step 10: Коммит**

```bash
git add api/proto/auth/auth.proto \
        api/proto/authv1/ \
        internal/services/auth/grpc/server.go \
        internal/services/auth/application/auth_service.go \
        internal/gateway/admin/handlers/users.go \
        portal-frontend/src/api/admin.ts \
        portal-frontend/src/pages/admin/users/UsersPage.tsx
git commit -m "feat(admin): add client_id field to user creation flow"
```

---

## Task 4: Sub-account balance UX

**Files:**
- Modify: `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`

- [ ] **Step 1: Добавить state для баланса родительского аккаунта**

В `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`, после строки `const [formLimitError, setFormLimitError] = useState('');` (строка ~74):

```typescript
const [parentBalance, setParentBalance] = useState<string | null>(null);
const [loadingBalance, setLoadingBalance] = useState(false);
```

- [ ] **Step 2: Загружать баланс при открытии модалки**

Найти место где устанавливается `setShowCreateForm(true)` (строка ~167 в JSX кнопке) и создать обёртку:

```typescript
async function openCreateForm() {
  setShowCreateForm(true);
  setParentBalance(null);
  setLoadingBalance(true);
  try {
    const res = await billingApi.getBalance() as { balance: string };
    setParentBalance(res.balance);
  } catch {
    // не критично — просто не показываем баланс
  } finally {
    setLoadingBalance(false);
  }
}
```

Заменить `onClick={() => setShowCreateForm(true)}` на `onClick={openCreateForm}`.

- [ ] **Step 3: Добавить hint и предупреждение под полем баланса**

Найти блок `<Input label="Начальный баланс" .../>` (строки ~217-222) и заменить его на:

```tsx
<div>
  <Input
    label="Начальный баланс"
    value={formInitialBalance}
    onChange={(e) => setFormInitialBalance(e.target.value)}
    placeholder="0.00"
  />
  {parentBalance !== null && !loadingBalance && (
    <p className="mt-1 text-xs text-gray-500">
      Доступный баланс: {parseFloat(parentBalance).toFixed(2)} ₽
    </p>
  )}
  {loadingBalance && (
    <p className="mt-1 text-xs text-gray-400">Загрузка баланса...</p>
  )}
  {parentBalance !== null &&
    formInitialBalance !== '' &&
    parseFloat(formInitialBalance) > parseFloat(parentBalance) && (
      <p className="mt-1 text-xs text-amber-600">
        Сумма превышает доступный баланс. Суб-аккаунт будет создан, но перевод средств не выполнится.
      </p>
    )}
</div>
```

- [ ] **Step 4: Добавить импорт billingApi**

В начале файла убедиться что `billingApi` импортируется:
```typescript
import { subAccountsApi, billingApi, ApiError } from '../../api/client';
```

Если уже есть другой импорт из `client.ts`, добавить `billingApi` в деструктуризацию.

- [ ] **Step 5: Улучшить toast при ошибке трансфера**

Найти строки ~140-144:
```typescript
if (resp?.balance_transfer_error) {
  toast.success('Суб-аккаунт создан, но перевод начального баланса не удался — недостаточно средств');
```
Заменить на:
```typescript
if (resp?.balance_transfer_error) {
  toast.warning('Суб-аккаунт создан. Перевод начального баланса не выполнен — недостаточно средств');
```

- [ ] **Step 6: Проверить TypeScript**

```bash
cd c:/projects/sms/portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Ожидается: 0 новых ошибок.

- [ ] **Step 7: Коммит**

```bash
git add portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx
git commit -m "fix(sub-accounts): show parent balance and warn on insufficient funds"
```

---

## Self-Review Checklist

- [x] **Spec coverage:**
  - Task 1 → `/admin/v1/audit` маршрут ✓
  - Task 2 → `groups` vs `rows` ✓
  - Task 3 → `client_id` в создании пользователя (proto + backend + frontend) ✓
  - Task 4 → UX валидации баланса (баланс родителя + warning + toast) ✓
- [x] **Placeholders:** нет TBD/TODO
- [x] **Type consistency:** `GroupedStatGroup` используется везде после переименования в Task 2; `ClientId` в proto соответствует Go-именованию в Steps 3-5
- [x] **Важное уточнение:** В Task 3 Step 4 нужно проверить все вызовы `authService.CreateUser` — возможно их несколько (gRPC server и тесты). Добавить `nil` для `clientID` там где вызов не через admin создание.
