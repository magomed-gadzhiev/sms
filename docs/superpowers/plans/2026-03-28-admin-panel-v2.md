# Admin Panel v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade admin panel with compact sidebar, live dashboard, extended billing, tarification UI, template workflow, user management, and granular permissions.

**Architecture:** Extends existing portal-frontend (React 19 + TS + Vite + Tailwind 4 + Radix UI) with new admin modules and shared components. Backend adds gRPC methods to billing/template/auth services, new REST endpoints to admin-gateway, and DB migrations. All admin routes wrapped in permission checks.

**Tech Stack:** Go 1.24 (backend), React 19 + TypeScript + Vite + Tailwind 4 + Radix UI + Recharts (frontend), PostgreSQL 15+ (pgx), Redis 7+ (sessions), gRPC + gorilla/mux

---

## File Structure Overview

### New Files
```
# Backend
migrations/000053_add_billing_extensions.up.sql
migrations/000053_add_billing_extensions.down.sql
migrations/000054_add_template_workflow.up.sql
migrations/000054_add_template_workflow.down.sql

# Proto (modify existing)
api/proto/billing/billing.proto          ← add FreezeAccount, UnfreezeAccount, SetCreditLimit, SetLowBalanceThreshold, ListBalances
api/proto/template/template.proto        ← add AssignReviewer, RequestRevision
api/proto/auth/auth.proto                ← add CreateUser, UpdateUser, DeactivateUser, ResetUser2FA, ResetUserPassword, CreateRole, UpdateRole, DeleteRole, ListRoles, ListAllPermissions

# Backend services (modify existing)
internal/services/billing/grpc/server.go
internal/services/billing/application/billing_service.go
internal/services/billing/infrastructure/repository/account_repository.go
internal/services/template/grpc/server.go
internal/services/auth/grpc/server.go
internal/services/auth/application/auth_service.go
internal/services/auth/infrastructure/repository/user_repository.go
internal/services/auth/infrastructure/repository/role_repository.go

# Admin gateway (modify + new)
internal/gateway/admin/handlers/billing.go      ← add freeze/unfreeze/credit-limit/threshold/list-balances
internal/gateway/admin/handlers/templates.go    ← add assign/request-revision
internal/gateway/admin/handlers/users.go        ← NEW
internal/gateway/admin/handlers/roles.go        ← NEW
internal/gateway/admin/router/router.go         ← add new routes

# Frontend
portal-frontend/src/hooks/usePolling.ts                          ← NEW
portal-frontend/src/hooks/usePermission.ts                       ← NEW
portal-frontend/src/components/auth/RequirePermission.tsx         ← NEW
portal-frontend/src/components/layout/AdminSidebar.tsx            ← NEW
portal-frontend/src/components/data/PermissionMatrix.tsx          ← NEW
portal-frontend/src/components/data/TimelineEvent.tsx             ← NEW
portal-frontend/src/pages/admin/AdminLayout.tsx                   ← REWRITE
portal-frontend/src/pages/admin/dashboard/DashboardPage.tsx       ← NEW
portal-frontend/src/pages/admin/dashboard/MetricsGrid.tsx         ← NEW
portal-frontend/src/pages/admin/dashboard/TrafficChart.tsx        ← NEW
portal-frontend/src/pages/admin/dashboard/AlertsFeed.tsx          ← NEW
portal-frontend/src/pages/admin/billing/BillingPage.tsx           ← NEW (replaces old)
portal-frontend/src/pages/admin/billing/BalancesTab.tsx           ← NEW
portal-frontend/src/pages/admin/billing/TransactionsTab.tsx       ← NEW (extract from old)
portal-frontend/src/pages/admin/billing/CreditLimitsTab.tsx       ← NEW
portal-frontend/src/pages/admin/billing/PricingTab.tsx            ← NEW (extract from old)
portal-frontend/src/pages/admin/tarification/TarificationPage.tsx ← NEW
portal-frontend/src/pages/admin/tarification/PlansTab.tsx         ← NEW
portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx       ← NEW
portal-frontend/src/pages/admin/tarification/TiersTab.tsx         ← NEW
portal-frontend/src/pages/admin/tarification/SenderRegistrationsTab.tsx ← NEW
portal-frontend/src/pages/admin/templates/TemplatesPage.tsx       ← NEW (replaces old)
portal-frontend/src/pages/admin/templates/TemplateReviewModal.tsx ← NEW
portal-frontend/src/pages/admin/users/UsersPage.tsx               ← NEW
portal-frontend/src/pages/admin/users/RolesPage.tsx               ← NEW
portal-frontend/src/contexts/AuthContext.tsx                      ← MODIFY
portal-frontend/src/api/admin.ts                                  ← MODIFY
portal-frontend/src/App.tsx                                       ← MODIFY
```

---

## Phase 1: Backend — Database Migrations

### Task 1: Billing Extensions Migration

**Files:**
- Create: `migrations/000053_add_billing_extensions.up.sql`
- Create: `migrations/000053_add_billing_extensions.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000053_add_billing_extensions.up.sql
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS frozen BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS credit_limit DECIMAL(15,2) NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS low_balance_threshold DECIMAL(15,2) NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMPTZ;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS frozen_by UUID REFERENCES users(id);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000053_add_billing_extensions.down.sql
ALTER TABLE accounts DROP COLUMN IF EXISTS frozen;
ALTER TABLE accounts DROP COLUMN IF EXISTS credit_limit;
ALTER TABLE accounts DROP COLUMN IF EXISTS low_balance_threshold;
ALTER TABLE accounts DROP COLUMN IF EXISTS frozen_at;
ALTER TABLE accounts DROP COLUMN IF EXISTS frozen_by;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000053_*
git commit -m "feat(billing): add frozen, credit_limit, low_balance_threshold columns to accounts"
```

---

### Task 2: Template Workflow Migration

**Files:**
- Create: `migrations/000054_add_template_workflow.up.sql`
- Create: `migrations/000054_add_template_workflow.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000054_add_template_workflow.up.sql
ALTER TABLE templates ADD COLUMN IF NOT EXISTS reviewer_id UUID REFERENCES users(id);
ALTER TABLE templates ADD COLUMN IF NOT EXISTS review_comment TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000054_add_template_workflow.down.sql
ALTER TABLE templates DROP COLUMN IF EXISTS reviewer_id;
ALTER TABLE templates DROP COLUMN IF EXISTS review_comment;
ALTER TABLE templates DROP COLUMN IF EXISTS reviewed_at;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000054_*
git commit -m "feat(templates): add reviewer_id, review_comment, reviewed_at columns"
```

---

## Phase 2: Backend — Proto & gRPC Updates

### Task 3: Billing Proto — Add New Methods

**Files:**
- Modify: `api/proto/billing/billing.proto`

- [ ] **Step 1: Add new RPC methods and messages to billing.proto**

Add after the `TransferBalance` rpc in the service definition:

```protobuf
  // FreezeAccount замораживает аккаунт клиента
  rpc FreezeAccount(FreezeAccountRequest) returns (FreezeAccountResponse);

  // UnfreezeAccount размораживает аккаунт клиента
  rpc UnfreezeAccount(UnfreezeAccountRequest) returns (UnfreezeAccountResponse);

  // SetCreditLimit устанавливает кредитный лимит клиента
  rpc SetCreditLimit(SetCreditLimitRequest) returns (SetCreditLimitResponse);

  // SetLowBalanceThreshold устанавливает порог низкого баланса
  rpc SetLowBalanceThreshold(SetLowBalanceThresholdRequest) returns (SetLowBalanceThresholdResponse);

  // ListBalances получает список балансов всех клиентов
  rpc ListBalances(ListBalancesRequest) returns (ListBalancesResponse);
```

Add message definitions at the end of the file:

```protobuf
message FreezeAccountRequest {
  string client_id = 1;
  string admin_id = 2;
}

message FreezeAccountResponse {
  bool success = 1;
  google.protobuf.Timestamp frozen_at = 2;
}

message UnfreezeAccountRequest {
  string client_id = 1;
  string admin_id = 2;
}

message UnfreezeAccountResponse {
  bool success = 1;
}

message SetCreditLimitRequest {
  string client_id = 1;
  string credit_limit = 2;
}

message SetCreditLimitResponse {
  bool success = 1;
  string credit_limit = 2;
}

message SetLowBalanceThresholdRequest {
  string client_id = 1;
  string threshold = 2;
}

message SetLowBalanceThresholdResponse {
  bool success = 1;
  string threshold = 2;
}

message ListBalancesRequest {
  string search = 1;
  string status = 2;
  bool below_threshold = 3;
  int32 limit = 4;
  int32 offset = 5;
}

message BalanceInfo {
  string client_id = 1;
  string client_name = 2;
  string balance = 3;
  string currency = 4;
  bool frozen = 5;
  string credit_limit = 6;
  string low_balance_threshold = 7;
  google.protobuf.Timestamp frozen_at = 8;
  string frozen_by = 9;
  google.protobuf.Timestamp updated_at = 10;
}

message ListBalancesResponse {
  repeated BalanceInfo balances = 1;
  int32 total = 2;
  int32 limit = 3;
  int32 offset = 4;
}
```

- [ ] **Step 2: Regenerate Go code from proto**

```bash
cd /home/magomed/projects/sms && protoc --go_out=. --go-grpc_out=. api/proto/billing/billing.proto
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/billing/ github.com/smpp-server/smpp-server/api/proto/billingv1/
git commit -m "feat(billing): add freeze/unfreeze/credit-limit/threshold/list-balances proto methods"
```

---

### Task 4: Template Proto — Add Workflow Methods

**Files:**
- Modify: `api/proto/template/template.proto`

- [ ] **Step 1: Add new RPC methods and messages to template.proto**

Add after `GetTemplateAuditLog` rpc:

```protobuf
  // AssignReviewer назначает ревьюера шаблону
  rpc AssignReviewer(AssignReviewerRequest) returns (AssignReviewerResponse);

  // RequestRevision запрашивает доработку шаблона
  rpc RequestRevision(RequestRevisionRequest) returns (RequestRevisionResponse);
```

Add message definitions:

```protobuf
message AssignReviewerRequest {
  string template_id = 1;
  string reviewer_id = 2;
}

message AssignReviewerResponse {
  TemplateInfo template = 1;
}

message RequestRevisionRequest {
  string template_id = 1;
  string reviewer_id = 2;
  string comment = 3;
}

message RequestRevisionResponse {
  TemplateInfo template = 1;
}
```

Add new fields to existing `TemplateInfo` message:

```protobuf
// Add to TemplateInfo after existing fields:
  string reviewer_id = 10;
  string review_comment = 11;
  google.protobuf.Timestamp reviewed_at = 12;
```

- [ ] **Step 2: Regenerate Go code**

```bash
protoc --go_out=. --go-grpc_out=. api/proto/template/template.proto
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/template/ github.com/smpp-server/smpp-server/api/proto/templatev1/
git commit -m "feat(templates): add AssignReviewer, RequestRevision proto methods and workflow fields"
```

---

### Task 5: Auth Proto — Add User/Role Management Methods

**Files:**
- Modify: `api/proto/auth/auth.proto`

- [ ] **Step 1: Add new RPC methods to AuthService**

Add after `RegisterClient` rpc:

```protobuf
  // CreateUser создает нового пользователя (admin)
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);

  // UpdateUser обновляет пользователя
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);

  // DeactivateUser деактивирует пользователя
  rpc DeactivateUser(DeactivateUserRequest) returns (DeactivateUserResponse);

  // ResetUser2FA сбрасывает 2FA пользователя
  rpc ResetUser2FA(ResetUser2FARequest) returns (ResetUser2FAResponse);

  // ResetUserPassword сбрасывает пароль пользователя
  rpc ResetUserPassword(ResetUserPasswordRequest) returns (ResetUserPasswordResponse);

  // ListUsers получает список пользователей
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);

  // GetUser получает пользователя по ID
  rpc GetUser(GetUserRequest) returns (GetUserResponse);

  // CreateRole создает роль
  rpc CreateRole(CreateRoleRequest) returns (CreateRoleResponse);

  // UpdateRole обновляет роль
  rpc UpdateRole(UpdateRoleRequest) returns (UpdateRoleResponse);

  // DeleteRole удаляет роль
  rpc DeleteRole(DeleteRoleRequest) returns (DeleteRoleResponse);

  // ListRoles получает список ролей
  rpc ListRoles(ListRolesRequest) returns (ListRolesResponse);

  // GetRole получает роль по ID
  rpc GetRole(GetRoleRequest) returns (GetRoleResponse);

  // ListAllPermissions получает все доступные permissions
  rpc ListAllPermissions(ListAllPermissionsRequest) returns (ListAllPermissionsResponse);

  // GetUserPermissions получает permissions пользователя
  rpc GetUserPermissions(GetUserPermissionsRequest) returns (GetUserPermissionsResponse);
```

Add message definitions:

```protobuf
message CreateUserRequest {
  string username = 1;
  string email = 2;
  string password = 3;
  string role_id = 4;
  bool active = 5;
}

message CreateUserResponse {
  UserInfo user = 1;
}

message UpdateUserRequest {
  string user_id = 1;
  string email = 2;
  string role_id = 3;
  bool active = 4;
}

message UpdateUserResponse {
  UserInfo user = 1;
}

message DeactivateUserRequest {
  string user_id = 1;
}

message DeactivateUserResponse {
  bool success = 1;
}

message ResetUser2FARequest {
  string user_id = 1;
}

message ResetUser2FAResponse {
  bool success = 1;
}

message ResetUserPasswordRequest {
  string user_id = 1;
}

message ResetUserPasswordResponse {
  string temporary_password = 1;
}

message ListUsersRequest {
  string search = 1;
  string role_id = 2;
  bool active_only = 3;
  int32 limit = 4;
  int32 offset = 5;
}

message ListUsersResponse {
  repeated UserDetailInfo users = 1;
  int32 total = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message GetUserRequest {
  string user_id = 1;
}

message GetUserResponse {
  UserDetailInfo user = 1;
}

message UserDetailInfo {
  string id = 1;
  string username = 2;
  string email = 3;
  Role role = 4;
  bool active = 5;
  bool totp_enabled = 6;
  google.protobuf.Timestamp last_login_at = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

message CreateRoleRequest {
  string name = 1;
  string description = 2;
  repeated string permission_ids = 3;
}

message CreateRoleResponse {
  RoleDetail role = 1;
}

message UpdateRoleRequest {
  string role_id = 1;
  string name = 2;
  string description = 3;
  repeated string permission_ids = 4;
}

message UpdateRoleResponse {
  RoleDetail role = 1;
}

message DeleteRoleRequest {
  string role_id = 1;
}

message DeleteRoleResponse {
  bool success = 1;
}

message ListRolesRequest {
  int32 limit = 1;
  int32 offset = 2;
}

message ListRolesResponse {
  repeated RoleDetail roles = 1;
  int32 total = 2;
}

message GetRoleRequest {
  string role_id = 1;
}

message GetRoleResponse {
  RoleDetail role = 1;
}

message RoleDetail {
  string id = 1;
  string name = 2;
  string description = 3;
  bool builtin = 4;
  int32 user_count = 5;
  repeated Permission permissions = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
}

message ListAllPermissionsRequest {}

message ListAllPermissionsResponse {
  repeated Permission permissions = 1;
}

message GetUserPermissionsRequest {
  string user_id = 1;
}

message GetUserPermissionsResponse {
  repeated Permission permissions = 1;
  Role role = 2;
}
```

- [ ] **Step 2: Regenerate Go code**

```bash
protoc --go_out=. --go-grpc_out=. api/proto/auth/auth.proto
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/auth/ github.com/smpp-server/smpp-server/api/proto/authv1/
git commit -m "feat(auth): add user/role management and permission listing proto methods"
```

---

### Task 6: Billing Service — Implement New gRPC Methods

**Files:**
- Modify: `internal/services/billing/grpc/server.go`
- Modify: `internal/services/billing/application/billing_service.go`
- Modify: `internal/services/billing/infrastructure/repository/account_repository.go`

- [ ] **Step 1: Add repository methods for account operations**

Add to `account_repository.go`:

```go
func (r *AccountRepository) FreezeAccount(ctx context.Context, clientID, adminID uuid.UUID) (time.Time, error) {
	frozenAt := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE accounts SET frozen = true, frozen_at = $1, frozen_by = $2 WHERE client_id = $3`,
		frozenAt, adminID, clientID,
	)
	return frozenAt, err
}

func (r *AccountRepository) UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE accounts SET frozen = false, frozen_at = NULL, frozen_by = NULL WHERE client_id = $1`,
		clientID,
	)
	return err
}

func (r *AccountRepository) SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE accounts SET credit_limit = $1 WHERE client_id = $2`,
		limit, clientID,
	)
	return err
}

func (r *AccountRepository) SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE accounts SET low_balance_threshold = $1 WHERE client_id = $2`,
		threshold, clientID,
	)
	return err
}

func (r *AccountRepository) ListBalances(ctx context.Context, search, status string, belowThreshold bool, limit, offset int32) ([]BalanceInfo, int32, error) {
	query := `SELECT a.client_id, c.name, a.balance, a.currency, a.frozen, a.credit_limit, a.low_balance_threshold, a.frozen_at, a.frozen_by, a.updated_at
		FROM accounts a JOIN clients c ON c.client_id = a.client_id WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM accounts a JOIN clients c ON c.client_id = a.client_id WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		query += fmt.Sprintf(` AND (c.name ILIKE $%d OR a.client_id::text ILIKE $%d)`, argIdx, argIdx)
		countQuery += fmt.Sprintf(` AND (c.name ILIKE $%d OR a.client_id::text ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status == "frozen" {
		query += ` AND a.frozen = true`
		countQuery += ` AND a.frozen = true`
	} else if status == "active" {
		query += ` AND a.frozen = false`
		countQuery += ` AND a.frozen = false`
	}
	if belowThreshold {
		query += ` AND a.low_balance_threshold > 0 AND a.balance < a.low_balance_threshold`
		countQuery += ` AND a.low_balance_threshold > 0 AND a.balance < a.low_balance_threshold`
	}

	var total int32
	r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)

	query += fmt.Sprintf(` ORDER BY c.name LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var balances []BalanceInfo
	for rows.Next() {
		var b BalanceInfo
		err := rows.Scan(&b.ClientID, &b.ClientName, &b.Balance, &b.Currency, &b.Frozen, &b.CreditLimit, &b.LowBalanceThreshold, &b.FrozenAt, &b.FrozenBy, &b.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		balances = append(balances, b)
	}
	return balances, total, nil
}
```

Define the `BalanceInfo` struct in the same package:

```go
type BalanceInfo struct {
	ClientID            uuid.UUID
	ClientName          string
	Balance             string
	Currency            string
	Frozen              bool
	CreditLimit         string
	LowBalanceThreshold string
	FrozenAt            *time.Time
	FrozenBy            *uuid.UUID
	UpdatedAt           time.Time
}
```

- [ ] **Step 2: Add application service methods**

Add to `billing_service.go` the methods `FreezeAccount`, `UnfreezeAccount`, `SetCreditLimit`, `SetLowBalanceThreshold`, `ListBalances` that delegate to the repository.

- [ ] **Step 3: Implement gRPC server methods**

Add to `internal/services/billing/grpc/server.go`:

```go
func (s *Server) FreezeAccount(ctx context.Context, req *billingv1.FreezeAccountRequest) (*billingv1.FreezeAccountResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	adminID, err := uuid.Parse(req.AdminId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid admin_id")
	}
	frozenAt, err := s.service.FreezeAccount(ctx, clientID, adminID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &billingv1.FreezeAccountResponse{
		Success:  true,
		FrozenAt: timestamppb.New(frozenAt),
	}, nil
}

func (s *Server) UnfreezeAccount(ctx context.Context, req *billingv1.UnfreezeAccountRequest) (*billingv1.UnfreezeAccountResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	err = s.service.UnfreezeAccount(ctx, clientID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &billingv1.UnfreezeAccountResponse{Success: true}, nil
}

func (s *Server) SetCreditLimit(ctx context.Context, req *billingv1.SetCreditLimitRequest) (*billingv1.SetCreditLimitResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	err = s.service.SetCreditLimit(ctx, clientID, req.CreditLimit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &billingv1.SetCreditLimitResponse{Success: true, CreditLimit: req.CreditLimit}, nil
}

func (s *Server) SetLowBalanceThreshold(ctx context.Context, req *billingv1.SetLowBalanceThresholdRequest) (*billingv1.SetLowBalanceThresholdResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	err = s.service.SetLowBalanceThreshold(ctx, clientID, req.Threshold)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &billingv1.SetLowBalanceThresholdResponse{Success: true, Threshold: req.Threshold}, nil
}

func (s *Server) ListBalances(ctx context.Context, req *billingv1.ListBalancesRequest) (*billingv1.ListBalancesResponse, error) {
	limit := req.Limit
	if limit <= 0 { limit = 50 }
	balances, total, err := s.service.ListBalances(ctx, req.Search, req.Status, req.BelowThreshold, limit, req.Offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &billingv1.ListBalancesResponse{Total: total, Limit: limit, Offset: req.Offset}
	for _, b := range balances {
		info := &billingv1.BalanceInfo{
			ClientId:             b.ClientID.String(),
			ClientName:           b.ClientName,
			Balance:              b.Balance,
			Currency:             b.Currency,
			Frozen:               b.Frozen,
			CreditLimit:          b.CreditLimit,
			LowBalanceThreshold:  b.LowBalanceThreshold,
			UpdatedAt:            timestamppb.New(b.UpdatedAt),
		}
		if b.FrozenAt != nil { info.FrozenAt = timestamppb.New(*b.FrozenAt) }
		if b.FrozenBy != nil { info.FrozenBy = b.FrozenBy.String() }
		resp.Balances = append(resp.Balances, info)
	}
	return resp, nil
}
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/magomed/projects/sms && go build ./internal/services/billing/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/billing/
git commit -m "feat(billing): implement FreezeAccount, UnfreezeAccount, SetCreditLimit, SetLowBalanceThreshold, ListBalances"
```

---

### Task 7: Template Service — Implement Workflow Methods

**Files:**
- Modify: `internal/services/template/grpc/server.go`
- Modify: `internal/services/template/application/` (template_service.go or equivalent)
- Modify: `internal/services/template/infrastructure/repository/` (template_repository.go)

- [ ] **Step 1: Add repository methods for reviewer assignment and revision**

```go
func (r *TemplateRepository) AssignReviewer(ctx context.Context, templateID, reviewerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE templates SET reviewer_id = $1, status = 'review', reviewed_at = NOW() WHERE id = $1`,
		reviewerID, templateID,
	)
	// Fix: templateID should be $2
	_, err = r.pool.Exec(ctx,
		`UPDATE templates SET reviewer_id = $1, status = 'review', reviewed_at = NOW() WHERE id = $2`,
		reviewerID, templateID,
	)
	return err
}

func (r *TemplateRepository) RequestRevision(ctx context.Context, templateID, reviewerID uuid.UUID, comment string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE templates SET status = 'revision_requested', review_comment = $1, reviewer_id = $2, reviewed_at = NOW() WHERE id = $3`,
		comment, reviewerID, templateID,
	)
	return err
}
```

- [ ] **Step 2: Implement gRPC server methods**

Add to template gRPC server:

```go
func (s *Server) AssignReviewer(ctx context.Context, req *templatev1.AssignReviewerRequest) (*templatev1.AssignReviewerResponse, error) {
	templateID, err := uuid.Parse(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template_id")
	}
	reviewerID, err := uuid.Parse(req.ReviewerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reviewer_id")
	}
	err = s.service.AssignReviewer(ctx, templateID, reviewerID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	tmpl, err := s.service.GetTemplate(ctx, templateID, uuid.Nil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &templatev1.AssignReviewerResponse{Template: s.templateToProto(tmpl)}, nil
}

func (s *Server) RequestRevision(ctx context.Context, req *templatev1.RequestRevisionRequest) (*templatev1.RequestRevisionResponse, error) {
	templateID, err := uuid.Parse(req.TemplateId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid template_id")
	}
	reviewerID, err := uuid.Parse(req.ReviewerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reviewer_id")
	}
	err = s.service.RequestRevision(ctx, templateID, reviewerID, req.Comment)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	tmpl, err := s.service.GetTemplate(ctx, templateID, uuid.Nil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &templatev1.RequestRevisionResponse{Template: s.templateToProto(tmpl)}, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/services/template/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/template/
git commit -m "feat(templates): implement AssignReviewer, RequestRevision gRPC methods"
```

---

### Task 8: Auth Service — Implement User/Role Management

**Files:**
- Modify: `internal/services/auth/grpc/server.go`
- Modify: `internal/services/auth/application/auth_service.go`
- Modify: `internal/services/auth/infrastructure/repository/user_repository.go`
- Modify: `internal/services/auth/infrastructure/repository/role_repository.go`

- [ ] **Step 1: Add user management repository methods**

Add to `user_repository.go`:

```go
func (r *UserRepository) List(ctx context.Context, search, roleID string, activeOnly bool, limit, offset int32) ([]*domain.User, int32, error) {
	query := `SELECT u.id, u.username, u.email, u.role_id, u.active, u.created_at, u.updated_at
		FROM users u WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM users u WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		query += fmt.Sprintf(` AND (u.username ILIKE $%d OR u.email ILIKE $%d)`, argIdx, argIdx)
		countQuery += fmt.Sprintf(` AND (u.username ILIKE $%d OR u.email ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if roleID != "" {
		query += fmt.Sprintf(` AND u.role_id = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND u.role_id = $%d`, argIdx)
		args = append(args, roleID)
		argIdx++
	}
	if activeOnly {
		query += ` AND u.active = true`
		countQuery += ` AND u.active = true`
	}

	var total int32
	r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)

	query += fmt.Sprintf(` ORDER BY u.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.RoleID, &u.Active, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepository) Deactivate(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET active = false, updated_at = NOW() WHERE id = $1`, userID)
	return err
}
```

- [ ] **Step 2: Add role management repository methods**

Add to `role_repository.go`:

```go
func (r *RoleRepository) List(ctx context.Context, limit, offset int32) ([]*domain.Role, int32, error) {
	var total int32
	r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles`).Scan(&total)

	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.name, r.description, r.created_at, r.updated_at,
			(SELECT COUNT(*) FROM users u WHERE u.role_id = r.id) as user_count
		FROM roles r ORDER BY r.name LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		role := &domain.Role{}
		var userCount int32
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt, &userCount)
		if err != nil {
			return nil, 0, err
		}
		role.Permissions, _ = r.getPermissionsByRoleID(ctx, role.ID)
		roles = append(roles, role)
	}
	return roles, total, nil
}

func (r *RoleRepository) Create(ctx context.Context, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	roleID := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO roles (id, name, description, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`,
		roleID, name, description)
	if err != nil {
		return nil, err
	}
	for _, pid := range permissionIDs {
		r.pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, pid)
	}
	return r.GetByID(ctx, roleID)
}

func (r *RoleRepository) Update(ctx context.Context, roleID uuid.UUID, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE roles SET name = $1, description = $2, updated_at = NOW() WHERE id = $3`,
		name, description, roleID)
	if err != nil {
		return nil, err
	}
	r.pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	for _, pid := range permissionIDs {
		r.pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, pid)
	}
	return r.GetByID(ctx, roleID)
}

func (r *RoleRepository) Delete(ctx context.Context, roleID uuid.UUID) error {
	r.pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	_, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	return err
}

func (r *RoleRepository) ListAllPermissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, resource, action, description, created_at FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var p domain.Permission
		err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
```

- [ ] **Step 3: Add application service methods for user/role management**

Add methods `CreateUser`, `UpdateUser`, `DeactivateUser`, `ResetUser2FA`, `ResetUserPassword`, `ListUsers`, `GetUser`, `CreateRole`, `UpdateRole`, `DeleteRole`, `ListRoles`, `GetRole`, `ListAllPermissions`, `GetUserPermissions` to `auth_service.go`, delegating to repositories.

- [ ] **Step 4: Implement gRPC server methods**

Add all new methods to auth gRPC server following the existing pattern (validate input → parse UUIDs → call application service → return proto response).

- [ ] **Step 5: Verify compilation**

```bash
go build ./internal/services/auth/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/services/auth/
git commit -m "feat(auth): implement user/role management gRPC methods"
```

---

### Task 9: Admin Gateway — New REST Endpoints

**Files:**
- Create: `internal/gateway/admin/handlers/users.go`
- Create: `internal/gateway/admin/handlers/roles.go`
- Modify: `internal/gateway/admin/handlers/billing.go`
- Modify: `internal/gateway/admin/handlers/templates.go`
- Modify: `internal/gateway/admin/router/router.go`
- Modify: `cmd/admin-gateway/main.go`

- [ ] **Step 1: Add billing handler methods for freeze/unfreeze/credit-limit/threshold/list-balances**

Add to `internal/gateway/admin/handlers/billing.go`:

```go
func (h *BillingHandlers) FreezeAccount(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	adminID := middleware.GetUserID(r.Context()).String()
	resp, err := h.billingClient.FreezeAccount(r.Context(), &billingv1.FreezeAccountRequest{
		ClientId: clientID, AdminId: adminID,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}

func (h *BillingHandlers) UnfreezeAccount(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	adminID := middleware.GetUserID(r.Context()).String()
	resp, err := h.billingClient.UnfreezeAccount(r.Context(), &billingv1.UnfreezeAccountRequest{
		ClientId: clientID, AdminId: adminID,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}

func (h *BillingHandlers) SetCreditLimit(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct { CreditLimit string `json:"credit_limit"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request")); return
	}
	resp, err := h.billingClient.SetCreditLimit(r.Context(), &billingv1.SetCreditLimitRequest{
		ClientId: clientID, CreditLimit: req.CreditLimit,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}

func (h *BillingHandlers) SetLowBalanceThreshold(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	var req struct { Threshold string `json:"threshold"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request")); return
	}
	resp, err := h.billingClient.SetLowBalanceThreshold(r.Context(), &billingv1.SetLowBalanceThresholdRequest{
		ClientId: clientID, Threshold: req.Threshold,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}

func (h *BillingHandlers) ListBalances(w http.ResponseWriter, r *http.Request) {
	resp, err := h.billingClient.ListBalances(r.Context(), &billingv1.ListBalancesRequest{
		Search:         r.URL.Query().Get("search"),
		Status:         r.URL.Query().Get("status"),
		BelowThreshold: r.URL.Query().Get("below_threshold") == "true",
		Limit:          parseIntParam(r, "limit", 50),
		Offset:         parseIntParam(r, "offset", 0),
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Add template handler methods for assign/request-revision**

Add to `internal/gateway/admin/handlers/templates.go`:

```go
func (h *TemplateHandlers) AssignReviewer(w http.ResponseWriter, r *http.Request) {
	templateID := mux.Vars(r)["id"]
	reviewerID := middleware.GetUserID(r.Context()).String()
	resp, err := h.templateClient.AssignReviewer(r.Context(), &templatev1.AssignReviewerRequest{
		TemplateId: templateID, ReviewerId: reviewerID,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}

func (h *TemplateHandlers) RequestRevision(w http.ResponseWriter, r *http.Request) {
	templateID := mux.Vars(r)["id"]
	reviewerID := middleware.GetUserID(r.Context()).String()
	var req struct { Comment string `json:"comment"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request")); return
	}
	resp, err := h.templateClient.RequestRevision(r.Context(), &templatev1.RequestRevisionRequest{
		TemplateId: templateID, ReviewerId: reviewerID, Comment: req.Comment,
	})
	if err != nil { respondGRPCError(w, err); return }
	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 3: Create users handler**

Create `internal/gateway/admin/handlers/users.go` with handlers for `ListUsers`, `GetUser`, `CreateUser`, `UpdateUser`, `DeactivateUser`, `ResetUser2FA`, `ResetUserPassword` — following the same pattern as other handlers (extract params → call authClient gRPC methods → respondJSON).

- [ ] **Step 4: Create roles handler**

Create `internal/gateway/admin/handlers/roles.go` with handlers for `ListRoles`, `GetRole`, `CreateRole`, `UpdateRole`, `DeleteRole`, `ListPermissions` — following the same handler pattern.

- [ ] **Step 5: Register new routes in router.go**

Add to `internal/gateway/admin/router/router.go`:

```go
// Add parameters to SetupRouter:
// userHandlers *handlers.UserHandlers,
// roleHandlers *handlers.RoleHandlers,

// Billing extensions
billing.HandleFunc("/clients/{id}/freeze", billingHandlers.FreezeAccount).Methods("POST")
billing.HandleFunc("/clients/{id}/unfreeze", billingHandlers.UnfreezeAccount).Methods("POST")
billing.HandleFunc("/clients/{id}/credit-limit", billingHandlers.SetCreditLimit).Methods("PUT")
billing.HandleFunc("/clients/{id}/low-balance-threshold", billingHandlers.SetLowBalanceThreshold).Methods("PUT")
billing.HandleFunc("/balances", billingHandlers.ListBalances).Methods("GET")

// Template extensions
templates.HandleFunc("/{id}/assign", templateHandlers.AssignReviewer).Methods("POST")
templates.HandleFunc("/{id}/request-revision", templateHandlers.RequestRevision).Methods("POST")

// User endpoints
users := adminV1.PathPrefix("/users").Subrouter()
users.HandleFunc("", userHandlers.ListUsers).Methods("GET")
users.HandleFunc("", userHandlers.CreateUser).Methods("POST")
users.HandleFunc("/{id}", userHandlers.GetUser).Methods("GET")
users.HandleFunc("/{id}", userHandlers.UpdateUser).Methods("PUT")
users.HandleFunc("/{id}/deactivate", userHandlers.DeactivateUser).Methods("POST")
users.HandleFunc("/{id}/reset-2fa", userHandlers.ResetUser2FA).Methods("POST")
users.HandleFunc("/{id}/reset-password", userHandlers.ResetUserPassword).Methods("POST")

// Role endpoints
roles := adminV1.PathPrefix("/roles").Subrouter()
roles.HandleFunc("", roleHandlers.ListRoles).Methods("GET")
roles.HandleFunc("", roleHandlers.CreateRole).Methods("POST")
roles.HandleFunc("/{id}", roleHandlers.GetRole).Methods("GET")
roles.HandleFunc("/{id}", roleHandlers.UpdateRole).Methods("PUT")
roles.HandleFunc("/{id}", roleHandlers.DeleteRole).Methods("DELETE")

// Permissions endpoint
adminV1.HandleFunc("/permissions", roleHandlers.ListPermissions).Methods("GET")
```

- [ ] **Step 6: Update admin-gateway main.go to initialize new handlers**

Add UserHandlers and RoleHandlers initialization in `cmd/admin-gateway/main.go` and pass them to `SetupRouter`.

- [ ] **Step 7: Verify compilation**

```bash
go build ./cmd/admin-gateway/...
```

- [ ] **Step 8: Commit**

```bash
git add internal/gateway/admin/ cmd/admin-gateway/
git commit -m "feat(admin-gateway): add billing, template, user, role REST endpoints"
```

---

### Task 10: Worker — Add Frozen & Credit Limit Checks

**Files:**
- Modify: `cmd/worker/main.go`

- [ ] **Step 1: Add frozen/credit_limit check before tarification**

In the `createOutgoingHandler` function, before calling `tarificationClient.TarifyMessage()`, add a check:

```go
// Check if account is frozen
balanceResp, balErr := billingClient.GetBalance(ctx, &billingv1.GetBalanceRequest{ClientId: dbMsg.ClientID.String()})
if balErr == nil && balanceResp != nil {
	// The frozen check requires a new field in GetBalanceResponse or a separate call
	// For now, use ListBalances to check frozen status
}
```

Since we need the `frozen` field in `GetBalanceResponse`, add `bool frozen = 5;` and `string credit_limit = 6;` to the `GetBalanceResponse` in `billing.proto`, regenerate, and update the billing service `GetBalance` implementation to include these fields.

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/worker/...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/worker/ api/proto/billing/ internal/services/billing/
git commit -m "feat(worker): check frozen status and credit_limit before charging"
```

---

## Phase 3: Frontend Foundation

### Task 11: usePolling Hook

**Files:**
- Create: `portal-frontend/src/hooks/usePolling.ts`

- [ ] **Step 1: Create the hook**

```tsx
import { useEffect, useRef, useState, useCallback } from 'react';

export function usePolling(fn: () => Promise<void>, intervalMs: number) {
  const [isPaused, setIsPaused] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const execute = useCallback(async () => {
    await fnRef.current();
    setLastUpdated(new Date());
  }, []);

  useEffect(() => {
    execute();
  }, [execute]);

  useEffect(() => {
    if (isPaused) {
      if (intervalRef.current) clearInterval(intervalRef.current);
      return;
    }
    intervalRef.current = setInterval(execute, intervalMs);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isPaused, intervalMs, execute]);

  const pause = useCallback(() => setIsPaused(true), []);
  const resume = useCallback(() => setIsPaused(false), []);

  return { pause, resume, isPaused, lastUpdated };
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/hooks/usePolling.ts
git commit -m "feat(frontend): add usePolling hook for auto-refresh"
```

---

### Task 12: Permission System — usePermission Hook + RequirePermission Component + AuthContext Extension

**Files:**
- Create: `portal-frontend/src/hooks/usePermission.ts`
- Create: `portal-frontend/src/components/auth/RequirePermission.tsx`
- Modify: `portal-frontend/src/contexts/AuthContext.tsx`
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Add permissions API to admin.ts**

Add to `portal-frontend/src/api/admin.ts`:

```tsx
// Types
export interface UserDetailInfo {
  id: string;
  username: string;
  email: string;
  role: { id: string; name: string; description: string };
  active: boolean;
  totp_enabled: boolean;
  last_login_at: string;
  created_at: string;
  updated_at: string;
}

export interface RoleDetail {
  id: string;
  name: string;
  description: string;
  builtin: boolean;
  user_count: number;
  permissions: PermissionInfo[];
  created_at: string;
  updated_at: string;
}

export interface PermissionInfo {
  id: string;
  resource: string;
  action: string;
}

export interface BalanceInfoItem {
  client_id: string;
  client_name: string;
  balance: string;
  currency: string;
  frozen: boolean;
  credit_limit: string;
  low_balance_threshold: string;
  frozen_at: string;
  frozen_by: string;
  updated_at: string;
}

// API modules
export const usersApi = {
  list: (params?: { search?: string; role_id?: string; active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ users: UserDetailInfo[]; total: number; limit: number; offset: number }>(`/users${qs(params || {})}`),
  get: (id: string) => adminFetch<{ user: UserDetailInfo }>(`/users/${id}`),
  create: (data: { username: string; email: string; password: string; role_id: string; active?: boolean }) =>
    adminFetch<{ user: UserDetailInfo }>('/users', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { email?: string; role_id?: string; active?: boolean }) =>
    adminFetch<{ user: UserDetailInfo }>(`/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deactivate: (id: string) =>
    adminFetch<void>(`/users/${id}/deactivate`, { method: 'POST' }),
  reset2fa: (id: string) =>
    adminFetch<void>(`/users/${id}/reset-2fa`, { method: 'POST' }),
  resetPassword: (id: string) =>
    adminFetch<{ temporary_password: string }>(`/users/${id}/reset-password`, { method: 'POST' }),
};

export const rolesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ roles: RoleDetail[]; total: number }>(`/roles${qs(params || {})}`),
  get: (id: string) => adminFetch<{ role: RoleDetail }>(`/roles/${id}`),
  create: (data: { name: string; description: string; permission_ids: string[] }) =>
    adminFetch<{ role: RoleDetail }>('/roles', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: { name: string; description: string; permission_ids: string[] }) =>
    adminFetch<{ role: RoleDetail }>(`/roles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/roles/${id}`, { method: 'DELETE' }),
};

export const permissionsApi = {
  list: () => adminFetch<{ permissions: PermissionInfo[] }>('/permissions'),
};

// Add to existing billingApi:
// freeze: (clientId: string) => adminFetch<void>(`/billing/clients/${clientId}/freeze`, { method: 'POST' }),
// unfreeze: (clientId: string) => adminFetch<void>(`/billing/clients/${clientId}/unfreeze`, { method: 'POST' }),
// setCreditLimit: (clientId: string, data: { credit_limit: string }) => adminFetch<void>(`/billing/clients/${clientId}/credit-limit`, { method: 'PUT', body: JSON.stringify(data) }),
// setLowBalanceThreshold: (clientId: string, data: { threshold: string }) => adminFetch<void>(`/billing/clients/${clientId}/low-balance-threshold`, { method: 'PUT', body: JSON.stringify(data) }),
// listBalances: (params?: { search?: string; status?: string; below_threshold?: boolean; limit?: number; offset?: number }) => adminFetch<{ balances: BalanceInfoItem[]; total: number; limit: number; offset: number }>(`/billing/balances${qs(params || {})}`),

// Add to existing templatesApi:
// assign: (id: string) => adminFetch<void>(`/templates/${id}/assign`, { method: 'POST' }),
// requestRevision: (id: string, data: { comment: string }) => adminFetch<void>(`/templates/${id}/request-revision`, { method: 'POST', body: JSON.stringify(data) }),

// Extend tarificationApi with missing methods:
// listTariffPeriods: (params?: { tariff_plan_id?: string }) => adminFetch<{ periods: TariffPeriod[]; total: number }>(`/tarification/tariff-periods${qs(params || {})}`),
// listTariffTiers: (params?: { tariff_period_id?: string }) => adminFetch<{ tiers: TariffTier[]; total: number }>(`/tarification/tariff-tiers${qs(params || {})}`),
// listSenderRegistrations: (params?: { client_id?: string; operator_id?: string; limit?: number; offset?: number }) => adminFetch<{ registrations: SenderRegistration[]; total: number }>(`/tarification/sender-registrations${qs(params || {})}`),
```

Also add the missing type definitions for `TariffPeriod`, `TariffTier`, `SenderRegistration`.

- [ ] **Step 2: Extend AuthContext with permissions**

Modify `portal-frontend/src/contexts/AuthContext.tsx`:

```tsx
// Add Permission type
interface Permission {
  resource: string;
  action: string;
}

// Add to AuthState interface:
//   permissions: Permission[];
//   hasPermission: (resource: string, action: string) => boolean;

// In AuthProvider, add permissions state:
const [permissions, setPermissions] = useState<Permission[]>([]);

// After profileApi.get() calls (in login, login2fa, useEffect), also fetch permissions:
// For admin/superadmin users, load permissions from /admin/v1/permissions endpoint
// via the user's role

// Add hasPermission function:
const hasPermission = useCallback((resource: string, action: string): boolean => {
  if (role === 'superadmin') return true;
  return permissions.some(p => p.resource === resource && p.action === action);
}, [role, permissions]);

// Add permissions and hasPermission to the context value
```

- [ ] **Step 3: Create usePermission hook**

```tsx
// portal-frontend/src/hooks/usePermission.ts
import { useAuth } from '../contexts/AuthContext';

export function usePermission(resource: string, action: string): boolean {
  const { hasPermission } = useAuth();
  return hasPermission(resource, action);
}
```

- [ ] **Step 4: Create RequirePermission component**

```tsx
// portal-frontend/src/components/auth/RequirePermission.tsx
import { type ReactNode } from 'react';
import { useAuth } from '../../contexts/AuthContext';

interface RequirePermissionProps {
  resource: string;
  action: string;
  children: ReactNode;
  fallback?: ReactNode;
}

export function RequirePermission({ resource, action, children, fallback = null }: RequirePermissionProps) {
  const { hasPermission } = useAuth();
  if (!hasPermission(resource, action)) return <>{fallback}</>;
  return <>{children}</>;
}
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/hooks/usePermission.ts portal-frontend/src/components/auth/RequirePermission.tsx portal-frontend/src/contexts/AuthContext.tsx portal-frontend/src/api/admin.ts
git commit -m "feat(frontend): add permission system - usePermission hook, RequirePermission component, AuthContext extension"
```

---

### Task 13: AdminSidebar + AdminLayout v2

**Files:**
- Create: `portal-frontend/src/components/layout/AdminSidebar.tsx`
- Rewrite: `portal-frontend/src/pages/admin/AdminLayout.tsx`

- [ ] **Step 1: Create AdminSidebar component**

```tsx
// portal-frontend/src/components/layout/AdminSidebar.tsx
import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';

interface NavGroup {
  label?: string;
  items: { path: string; label: string; icon: string; resource: string }[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    items: [
      { path: '/admin/dashboard', label: 'Dashboard', icon: '📊', resource: 'analytics' },
    ],
  },
  {
    label: 'Основное',
    items: [
      { path: '/admin/clients', label: 'Клиенты', icon: '👥', resource: 'clients' },
      { path: '/admin/templates', label: 'Шаблоны', icon: '📝', resource: 'templates' },
      { path: '/admin/users', label: 'Пользователи', icon: '🔑', resource: 'users' },
    ],
  },
  {
    label: 'Финансы',
    items: [
      { path: '/admin/billing', label: 'Биллинг', icon: '💰', resource: 'billing' },
      { path: '/admin/tarification', label: 'Тарификация', icon: '📋', resource: 'tarification' },
    ],
  },
  {
    label: 'Инфраструктура',
    items: [
      { path: '/admin/providers', label: 'Провайдеры', icon: '🔌', resource: 'providers' },
      { path: '/admin/routes', label: 'Маршруты', icon: '🔀', resource: 'routes' },
      { path: '/admin/hlr', label: 'HLR', icon: '📡', resource: 'hlr' },
    ],
  },
  {
    label: 'Справочники',
    items: [
      { path: '/admin/countries', label: 'Страны', icon: '🌍', resource: 'countries' },
      { path: '/admin/webhooks', label: 'Вебхуки', icon: '🔗', resource: 'webhooks' },
    ],
  },
  {
    label: 'Наблюдение',
    items: [
      { path: '/admin/analytics', label: 'Аналитика', icon: '📈', resource: 'analytics' },
      { path: '/admin/monitoring', label: 'Мониторинг', icon: '⚡', resource: 'analytics' },
      { path: '/admin/audit', label: 'Аудит', icon: '📜', resource: 'audit' },
    ],
  },
];

export function AdminSidebar() {
  const location = useLocation();
  const { user, logout, hasPermission } = useAuth();
  const [expanded, setExpanded] = useState(false);

  return (
    <aside
      className={`fixed top-0 left-0 h-screen bg-gray-900 text-gray-300 flex flex-col z-50 transition-all duration-200 ${expanded ? 'w-[220px]' : 'w-14'}`}
      onMouseEnter={() => setExpanded(true)}
      onMouseLeave={() => setExpanded(false)}
    >
      <div className="h-14 flex items-center px-4 border-b border-gray-800">
        <span className="text-lg font-bold text-white whitespace-nowrap overflow-hidden">
          {expanded ? 'SMS Admin' : 'S'}
        </span>
      </div>
      <nav className="flex-1 overflow-y-auto py-2">
        {NAV_GROUPS.map((group, gi) => {
          const visibleItems = group.items.filter(item => hasPermission(item.resource, 'read'));
          if (visibleItems.length === 0) return null;
          return (
            <div key={gi}>
              {group.label && expanded && (
                <div className="px-4 pt-4 pb-1 text-[10px] uppercase tracking-wider text-gray-500 font-semibold">
                  {group.label}
                </div>
              )}
              {!group.label && gi > 0 && <div className="border-t border-gray-800 my-1" />}
              {visibleItems.map(item => {
                const isActive = location.pathname === item.path || (item.path !== '/admin/dashboard' && location.pathname.startsWith(item.path + '/'));
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={!expanded ? item.label : undefined}
                    className={`flex items-center gap-3 px-4 py-2 text-sm transition-colors ${
                      isActive
                        ? 'bg-primary/20 text-white font-medium border-l-2 border-primary'
                        : 'hover:bg-gray-800 hover:text-white border-l-2 border-transparent'
                    }`}
                  >
                    <span className="text-base w-5 text-center flex-shrink-0">{item.icon}</span>
                    {expanded && <span className="whitespace-nowrap overflow-hidden">{item.label}</span>}
                  </Link>
                );
              })}
            </div>
          );
        })}
      </nav>
      <div className="border-t border-gray-800 p-3">
        {expanded ? (
          <div>
            <div className="text-xs text-gray-500 truncate mb-1">{user?.email}</div>
            <button onClick={logout} className="text-xs text-gray-500 hover:text-white transition-colors">
              Выйти
            </button>
          </div>
        ) : (
          <button onClick={logout} title="Выйти" className="text-gray-500 hover:text-white text-sm">
            ⏻
          </button>
        )}
      </div>
    </aside>
  );
}
```

- [ ] **Step 2: Rewrite AdminLayout**

```tsx
// portal-frontend/src/pages/admin/AdminLayout.tsx
import { Outlet } from 'react-router-dom';
import { AdminSidebar } from '../../components/layout/AdminSidebar';

export function AdminLayout() {
  return (
    <div className="flex min-h-screen">
      <AdminSidebar />
      <main className="flex-1 ml-14 p-6 bg-gray-50/50 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/AdminSidebar.tsx portal-frontend/src/pages/admin/AdminLayout.tsx
git commit -m "feat(frontend): add compact sidebar with hover-expand and permission-filtered navigation"
```

---

### Task 14: Shared Components — TimelineEvent + PermissionMatrix

**Files:**
- Create: `portal-frontend/src/components/data/TimelineEvent.tsx`
- Create: `portal-frontend/src/components/data/PermissionMatrix.tsx`

- [ ] **Step 1: Create TimelineEvent**

```tsx
// portal-frontend/src/components/data/TimelineEvent.tsx
interface TimelineEventProps {
  icon: string;
  title: string;
  description?: string;
  author?: string;
  date: string;
  color?: 'gray' | 'green' | 'red' | 'yellow' | 'blue';
}

const colorMap = {
  gray: 'bg-gray-200 text-gray-600',
  green: 'bg-green-100 text-green-700',
  red: 'bg-red-100 text-red-700',
  yellow: 'bg-yellow-100 text-yellow-700',
  blue: 'bg-blue-100 text-blue-700',
};

export function TimelineEvent({ icon, title, description, author, date, color = 'gray' }: TimelineEventProps) {
  return (
    <div className="flex gap-3 pb-4 last:pb-0">
      <div className="flex flex-col items-center">
        <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm ${colorMap[color]}`}>
          {icon}
        </div>
        <div className="w-px flex-1 bg-gray-200 mt-1" />
      </div>
      <div className="flex-1 pt-1">
        <div className="text-sm font-medium text-gray-900">{title}</div>
        {description && <div className="text-sm text-gray-600 mt-0.5">{description}</div>}
        <div className="text-xs text-gray-400 mt-1">
          {author && <span>{author} &middot; </span>}
          {new Date(date).toLocaleString()}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Create PermissionMatrix**

```tsx
// portal-frontend/src/components/data/PermissionMatrix.tsx
import { useCallback } from 'react';
import type { PermissionInfo } from '../../api/admin';

const RESOURCES = ['messages', 'clients', 'providers', 'routes', 'analytics', 'billing', 'tarification', 'templates', 'users', 'webhooks', 'countries', 'hlr', 'audit'];
const ACTIONS = ['read', 'write', 'delete'];

interface PermissionMatrixProps {
  allPermissions: PermissionInfo[];
  selectedIds: Set<string>;
  onChange: (ids: Set<string>) => void;
  disabled?: boolean;
}

export function PermissionMatrix({ allPermissions, selectedIds, onChange, disabled }: PermissionMatrixProps) {
  const findPermId = useCallback((resource: string, action: string) => {
    return allPermissions.find(p => p.resource === resource && p.action === action)?.id;
  }, [allPermissions]);

  const toggle = (id: string) => {
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onChange(next);
  };

  const toggleRow = (resource: string) => {
    const next = new Set(selectedIds);
    const ids = ACTIONS.map(a => findPermId(resource, a)).filter(Boolean) as string[];
    const allSelected = ids.every(id => next.has(id));
    ids.forEach(id => allSelected ? next.delete(id) : next.add(id));
    onChange(next);
  };

  const toggleCol = (action: string) => {
    const next = new Set(selectedIds);
    const ids = RESOURCES.map(r => findPermId(r, action)).filter(Boolean) as string[];
    const allSelected = ids.every(id => next.has(id));
    ids.forEach(id => allSelected ? next.delete(id) : next.add(id));
    onChange(next);
  };

  return (
    <div className="border border-gray-200 rounded-lg overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-3 py-2 text-left font-medium text-gray-700">Ресурс</th>
            {ACTIONS.map(action => (
              <th key={action} className="px-3 py-2 text-center font-medium text-gray-700">
                <button onClick={() => !disabled && toggleCol(action)} className="hover:text-primary" disabled={disabled}>
                  {action}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {RESOURCES.map(resource => (
            <tr key={resource} className="hover:bg-gray-50">
              <td className="px-3 py-2 text-gray-800">
                <button onClick={() => !disabled && toggleRow(resource)} className="hover:text-primary" disabled={disabled}>
                  {resource}
                </button>
              </td>
              {ACTIONS.map(action => {
                const permId = findPermId(resource, action);
                return (
                  <td key={action} className="px-3 py-2 text-center">
                    {permId ? (
                      <input
                        type="checkbox"
                        checked={selectedIds.has(permId)}
                        onChange={() => toggle(permId)}
                        disabled={disabled}
                        className="rounded border-gray-300 text-primary focus:ring-primary/50"
                      />
                    ) : (
                      <span className="text-gray-300">-</span>
                    )}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/data/TimelineEvent.tsx portal-frontend/src/components/data/PermissionMatrix.tsx
git commit -m "feat(frontend): add TimelineEvent and PermissionMatrix shared components"
```

---

## Phase 4: New Frontend Pages

### Task 15: Live Dashboard Page

**Files:**
- Create: `portal-frontend/src/pages/admin/dashboard/DashboardPage.tsx`
- Create: `portal-frontend/src/pages/admin/dashboard/MetricsGrid.tsx`
- Create: `portal-frontend/src/pages/admin/dashboard/TrafficChart.tsx`
- Create: `portal-frontend/src/pages/admin/dashboard/AlertsFeed.tsx`

- [ ] **Step 1: Create MetricsGrid**

A grid of 8 StatCards in 2 rows of 4, rendering real-time metrics and business stats. Receives `metrics`, `clientCount`, `revenue`, `providerHealth`, `pendingTemplates` as props.

- [ ] **Step 2: Create TrafficChart**

A Recharts BarChart showing delivered (purple `#7c3aed`) and failed (red `#dc2626`) messages grouped by 5min intervals over the last hour. Receives `data` prop from `analyticsAdminApi.getStats({ period: '1h', group_by: '5min' })`.

- [ ] **Step 3: Create AlertsFeed**

A list of computed alerts (max 10) with color-coded severity. Alert types:
- Provider degraded (success_rate < 95%) — red
- Low balance (< threshold) — yellow
- Queue growth (> 1000) — yellow
- New client — blue

Receives `providers`, `balances`, `metrics` as props and computes alerts client-side.

- [ ] **Step 4: Create DashboardPage**

Ties everything together using `usePolling` with 30s interval. Fetches data from:
- `analyticsAdminApi.getRealTimeMetrics()`
- `clientsApi.list({ limit: 1 })`
- `billingApi.getTransactions({ transaction_type: 'charge', limit: 1 })`
- `providersApi.list({ active_only: true })` + health for each
- `templatesApi.list({ status: 'pending', limit: 1 })`
- `billingApi.listBalances({ below_threshold: true })`
- `analyticsAdminApi.getStats({ period: '1h', group_by: '5min' })`

Layout: PageHeader with breadcrumbs + pause/resume button → MetricsGrid → TrafficChart → AlertsFeed

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/admin/dashboard/
git commit -m "feat(frontend): add live dashboard with metrics, traffic chart, and alerts feed"
```

---

### Task 16: Billing Page — Refactor into Tabs

**Files:**
- Create: `portal-frontend/src/pages/admin/billing/BillingPage.tsx`
- Create: `portal-frontend/src/pages/admin/billing/BalancesTab.tsx`
- Create: `portal-frontend/src/pages/admin/billing/TransactionsTab.tsx`
- Create: `portal-frontend/src/pages/admin/billing/CreditLimitsTab.tsx`
- Create: `portal-frontend/src/pages/admin/billing/PricingTab.tsx`
- Delete old: `portal-frontend/src/pages/admin/BillingPage.tsx`

- [ ] **Step 1: Create BillingPage wrapper with 4 tabs**

Using Radix Tabs: Балансы, Транзакции, Кредитные лимиты, Правила цен. Each tab lazily renders its content component. PageHeader with breadcrumbs `[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Биллинг' }]`.

- [ ] **Step 2: Create BalancesTab**

DataTable with columns: Client, Balance, Status (frozen badge), Credit Limit, Alert Threshold. Row actions: Начислить/Списать/Заморозить-Разморозить. FilterBar with search, status (all/active/frozen), below_threshold. Modal for credit/debit operations.

Uses `billingApi.listBalances()`, `billingApi.addCredits()`, `billingApi.freeze()`, `billingApi.unfreeze()`.

- [ ] **Step 3: Create TransactionsTab**

Extract existing transactions tab from old BillingPage — same filters, columns, pagination.

- [ ] **Step 4: Create CreditLimitsTab**

DataTable: Client, Balance, Credit Limit, Available (balance + limit), Overdraft Usage. Edit credit limit inline or via modal. Uses `billingApi.listBalances()`, `billingApi.setCreditLimit()`.

- [ ] **Step 5: Create PricingTab**

Extract existing pricing rules tab from old BillingPage.

- [ ] **Step 6: Delete old BillingPage, update imports in App.tsx**

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/src/pages/admin/billing/
git commit -m "feat(frontend): refactor billing page into 4 tabs with balances, credit limits"
```

---

### Task 17: Tarification Page

**Files:**
- Create: `portal-frontend/src/pages/admin/tarification/TarificationPage.tsx`
- Create: `portal-frontend/src/pages/admin/tarification/PlansTab.tsx`
- Create: `portal-frontend/src/pages/admin/tarification/PeriodsTab.tsx`
- Create: `portal-frontend/src/pages/admin/tarification/TiersTab.tsx`
- Create: `portal-frontend/src/pages/admin/tarification/SenderRegistrationsTab.tsx`

- [ ] **Step 1: Create TarificationPage**

Radix Tabs wrapper with 4 tabs: Тарифные планы, Периоды, Тиры, Регистрация отправителей. PageHeader with breadcrumbs.

- [ ] **Step 2: Create PlansTab**

DataTable: name (sender_category), operator, strategy (flat/tiered/volume), status, active toggle. CRUD modal: operator (select), sender_category, strategy. Uses `tarificationApi.listTariffPlans()`, `tarificationApi.createTariffPlan()`, `tarificationApi.updateTariffPlan()`.

- [ ] **Step 3: Create PeriodsTab**

Filter by tariff plan. DataTable: plan, start_date, end_date, status (computed from dates). CRUD modal: plan (select), dates. Uses tarification API.

- [ ] **Step 4: Create TiersTab**

Filter by plan and period. DataTable: plan, period, from_count, price_per_segment. Recharts StepChart for price visualization. CRUD modal. Uses tarification API.

- [ ] **Step 5: Create SenderRegistrationsTab**

DataTable: sender_name, operator, client, type, status, dates. CRUD modal. Uses `tarificationApi.listSenderRegistrations()` etc.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/admin/tarification/
git commit -m "feat(frontend): add tarification page with plans, periods, tiers, sender registrations"
```

---

### Task 18: Templates Page — Workflow Extension

**Files:**
- Create: `portal-frontend/src/pages/admin/templates/TemplatesPage.tsx`
- Create: `portal-frontend/src/pages/admin/templates/TemplateReviewModal.tsx`
- Delete old: `portal-frontend/src/pages/admin/TemplatesPage.tsx`

- [ ] **Step 1: Create TemplateReviewModal**

Shows template body with highlighted variables (`{name}` in colored spans). Actions: Take for Review, Approve, Request Revision (textarea required), Reject (reason required). Timeline of audit log entries using TimelineEvent component.

Uses `templatesApi.assign()`, `templatesApi.approve()`, `templatesApi.reject()`, `templatesApi.requestRevision()`, `templatesApi.audit()`.

- [ ] **Step 2: Create new TemplatesPage**

DataTable with additional columns: Reviewer, Review Date. New filter for `revision_requested` status. Color-coded status badges: draft (gray), pending (yellow), review (blue), revision_requested (orange), approved (green), rejected (red).

Row click opens TemplateReviewModal. Breadcrumbs in PageHeader.

- [ ] **Step 3: Update StatusBadge to include new statuses**

Add to `statusMap` in `Badge.tsx`:
```tsx
draft: { variant: 'default', label: 'Draft' },
review: { variant: 'info', label: 'Review' },
revision_requested: { variant: 'warning', label: 'Revision Requested' },
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/templates/ portal-frontend/src/components/ui/Badge.tsx
git commit -m "feat(frontend): add template workflow with review modal and timeline"
```

---

### Task 19: Users & Roles Pages

**Files:**
- Create: `portal-frontend/src/pages/admin/users/UsersPage.tsx`
- Create: `portal-frontend/src/pages/admin/users/RolesPage.tsx`

- [ ] **Step 1: Create UsersPage**

DataTable columns: username, email, Role (Badge), Status (active/inactive), 2FA (enabled/disabled), Last Login, Created. Actions: Create (modal: username, email, password, role select, active), Edit (modal: role, email, active), Deactivate (confirm dialog), Reset 2FA (confirm), Reset Password (shows temp password).

FilterBar: search, role (select), active_only. Breadcrumbs.

Uses `usersApi`, `rolesApi.list()` for role select options.

- [ ] **Step 2: Create RolesPage**

DataTable: name, description, user_count, builtin (Yes/No). Create/Edit modal includes PermissionMatrix component. Delete with confirmation (blocked for builtin roles). Breadcrumbs: Админ / Пользователи / Роли.

Uses `rolesApi`, `permissionsApi.list()`.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/admin/users/
git commit -m "feat(frontend): add users and roles management pages with permission matrix"
```

---

## Phase 5: Routing & Integration

### Task 20: Update App.tsx Routing

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Add lazy imports for new pages**

```tsx
const AdminDashboardPage = lazy(() => import('./pages/admin/dashboard/DashboardPage').then(m => ({ default: m.DashboardPage })));
const AdminBillingPage = lazy(() => import('./pages/admin/billing/BillingPage').then(m => ({ default: m.BillingPage })));
const AdminTarificationPage = lazy(() => import('./pages/admin/tarification/TarificationPage').then(m => ({ default: m.TarificationPage })));
const AdminTemplatesPage = lazy(() => import('./pages/admin/templates/TemplatesPage').then(m => ({ default: m.TemplatesPage })));
const AdminUsersPage = lazy(() => import('./pages/admin/users/UsersPage').then(m => ({ default: m.UsersPage })));
const AdminRolesPage = lazy(() => import('./pages/admin/users/RolesPage').then(m => ({ default: m.RolesPage })));
```

- [ ] **Step 2: Update admin routes**

```tsx
<Route path="/admin/*" element={<RequireRole role="admin"><Suspense fallback={...}><AdminLayout /></Suspense></RequireRole>}>
  <Route index element={<Navigate to="/admin/dashboard" replace />} />
  <Route path="dashboard" element={<Suspense fallback={null}><AdminDashboardPage /></Suspense>} />
  <Route path="clients" element={<Suspense fallback={null}><AdminClientsPage /></Suspense>} />
  <Route path="billing" element={<Suspense fallback={null}><AdminBillingPage /></Suspense>} />
  <Route path="tarification" element={<Suspense fallback={null}><AdminTarificationPage /></Suspense>} />
  <Route path="providers" element={<Suspense fallback={null}><AdminProvidersPage /></Suspense>} />
  <Route path="routes" element={<Suspense fallback={null}><AdminRoutesPage /></Suspense>} />
  <Route path="templates" element={<Suspense fallback={null}><AdminTemplatesPage /></Suspense>} />
  <Route path="analytics" element={<Suspense fallback={null}><AdminAnalyticsPage /></Suspense>} />
  <Route path="monitoring" element={<Suspense fallback={null}><AdminMonitoringPage /></Suspense>} />
  <Route path="audit" element={<Suspense fallback={null}><AdminAuditLogPage /></Suspense>} />
  <Route path="users" element={<Suspense fallback={null}><AdminUsersPage /></Suspense>} />
  <Route path="users/roles" element={<Suspense fallback={null}><AdminRolesPage /></Suspense>} />
  <Route path="webhooks" element={<Suspense fallback={null}><AdminWebhooksPage /></Suspense>} />
  <Route path="hlr" element={<Suspense fallback={null}><AdminHLRPage /></Suspense>} />
  <Route path="countries" element={<Suspense fallback={null}><AdminCountriesPage /></Suspense>} />
</Route>
```

- [ ] **Step 3: Remove old page imports for replaced pages**

Remove the old single-file lazy imports for BillingPage and TemplatesPage. Remove old files if they still exist.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/App.tsx
git commit -m "feat(frontend): update routing for admin panel v2 with dashboard, tarification, users"
```

---

### Task 21: Update Existing Admin Pages — Breadcrumbs & Permissions

**Files:**
- Modify: all existing admin pages (ClientsPage, ProvidersPage, RoutesPage, AnalyticsPage, MonitoringPage, AuditLogPage, WebhooksPage, HLRPage, CountriesPage)

- [ ] **Step 1: Add breadcrumbs to each page**

For each page, add `breadcrumbs` prop to PageHeader. Pattern:

```tsx
<PageHeader
  title="Clients"
  breadcrumbs={[
    { label: 'Админ', href: '/admin/dashboard' },
    { label: 'Клиенты' },
  ]}
  // ... existing props
/>
```

- [ ] **Step 2: Refactor MonitoringPage to use usePolling**

Replace the manual `setInterval`/`clearInterval` logic with:

```tsx
const { isPaused, lastUpdated, pause, resume } = usePolling(fetchData, REFRESH_INTERVAL);
```

Remove `paused`, `intervalRef`, `lastUpdate` state and the interval useEffect.

- [ ] **Step 3: Add resource_type filter to AuditLogPage**

Add a new filter to the audit page's FilterBar:

```tsx
{ key: 'resource_type', label: 'Resource Type', type: 'select', options: [
  { value: 'client', label: 'Client' },
  { value: 'provider', label: 'Provider' },
  { value: 'template', label: 'Template' },
  { value: 'user', label: 'User' },
  { value: 'billing', label: 'Billing' },
  { value: 'route', label: 'Route' },
] },
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/
git commit -m "feat(frontend): add breadcrumbs to all admin pages, refactor monitoring to usePolling"
```

---

## Phase 6: Final Integration & Cleanup

### Task 22: Delete Old Single-File Pages

**Files:**
- Delete: `portal-frontend/src/pages/admin/BillingPage.tsx` (replaced by billing/ directory)
- Delete: `portal-frontend/src/pages/admin/TemplatesPage.tsx` (replaced by templates/ directory)

- [ ] **Step 1: Remove old files and verify no broken imports**

```bash
rm portal-frontend/src/pages/admin/BillingPage.tsx
rm portal-frontend/src/pages/admin/TemplatesPage.tsx
```

- [ ] **Step 2: Build frontend to verify no errors**

```bash
cd portal-frontend && npm run build
```

- [ ] **Step 3: Commit**

```bash
git add -A portal-frontend/
git commit -m "chore: remove old billing and templates pages replaced by directory modules"
```

---

### Task 23: Build & Verify Everything

- [ ] **Step 1: Build all backend services**

```bash
cd /home/magomed/projects/sms
go build ./...
```

- [ ] **Step 2: Build frontend**

```bash
cd portal-frontend && npm run build
```

- [ ] **Step 3: Run linter**

```bash
cd portal-frontend && npx tsc --noEmit
```

- [ ] **Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: resolve build issues from admin panel v2 integration"
```

---

## Execution Order & Dependencies

```
Task 1, 2 (migrations) → can run in parallel
Task 3, 4, 5 (proto) → sequential, each needs protoc
Task 6 (billing gRPC) → depends on Task 3
Task 7 (template gRPC) → depends on Task 4
Task 8 (auth gRPC) → depends on Task 5
Task 9 (admin gateway) → depends on Tasks 6, 7, 8
Task 10 (worker) → depends on Task 6

Task 11 (usePolling) → independent
Task 12 (permissions) → independent
Task 13 (sidebar/layout) → depends on Task 12
Task 14 (shared components) → independent

Task 15 (dashboard) → depends on Tasks 11, 13
Task 16 (billing UI) → depends on Tasks 12, 13
Task 17 (tarification UI) → depends on Tasks 13
Task 18 (templates UI) → depends on Tasks 12, 13, 14
Task 19 (users/roles UI) → depends on Tasks 12, 13, 14

Task 20 (routing) → depends on Tasks 15-19
Task 21 (existing pages) → depends on Tasks 11, 13
Task 22 (cleanup) → depends on Tasks 16, 18, 20
Task 23 (verify) → depends on everything
```

**Parallelizable groups:**
- Group A: Tasks 1, 2 (migrations)
- Group B: Tasks 3, 4, 5 (proto updates — sequential)
- Group C: Tasks 6, 7, 8, 9, 10 (backend implementation)
- Group D: Tasks 11, 12, 14 (frontend foundation — parallel)
- Group E: Task 13 (sidebar/layout)
- Group F: Tasks 15, 16, 17, 18, 19 (new pages — parallel after Group D+E)
- Group G: Tasks 20, 21, 22, 23 (integration — sequential)
