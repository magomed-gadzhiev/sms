# Sender Names: company_id Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Пробросить `company_id` через весь бэкенд-стек (proto → gRPC handler → service → repo → DB) при создании sender name; фронтенд уже готов.

**Architecture:** Portal HTTP handler читает `company_id` из тела запроса (с fallback на дефолтную компанию клиента), передаёт в gRPC. gRPC handler парсит UUID и вызывает service, который передаёт его в repo. Repo включает `company_id` в INSERT.

**Tech Stack:** Go 1.24, google.golang.org/protobuf (pb.go ручной патч), jackc/pgx/v5 (portal pool), jmoiron/sqlx (template service repo), github.com/google/uuid

---

## Файлы

| Файл | Действие | Что меняется |
|------|----------|--------------|
| `api/proto/sendernamev1/sender_name.pb.go` | Modify | Добавить `CompanyId` поле + геттер в `SenderNameInfo` и `CreateSenderNameRequest` |
| `internal/services/template/domain/sender_name.go` | Modify | Добавить `CompanyID uuid.UUID` в `SenderName` |
| `internal/services/template/infrastructure/repository/sender_name_repo.go` | Modify | `senderNameRow` + `Create()` + `toDomain()` |
| `internal/services/template/application/sender_name_service.go` | Modify | Сигнатура `RegisterSenderName` + передача в repo |
| `internal/services/template/grpc/sender_name_handler.go` | Modify | `CreateSenderName` + `senderNameToProto` |
| `internal/gateway/portal/handlers/sender_names.go` | Modify | `createSenderNameRequest` + `CreateSenderName` fallback |

---

## Task 1: Патч proto — добавить `company_id` в pb.go

**Files:**
- Modify: `api/proto/sendernamev1/sender_name.pb.go:38-41` (SenderNameInfo struct)
- Modify: `api/proto/sendernamev1/sender_name.pb.go:134` (после GetUpdatedAt)
- Modify: `api/proto/sendernamev1/sender_name.pb.go:237-244` (CreateSenderNameRequest struct)
- Modify: `api/proto/sendernamev1/sender_name.pb.go:287` (после GetName)

- [ ] **Шаг 1: Добавить поле CompanyId в SenderNameInfo**

В файле `api/proto/sendernamev1/sender_name.pb.go` найти:
```go
	UpdatedAt       *timestamppb.Timestamp `protobuf:"bytes,9,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}
```
Заменить на:
```go
	UpdatedAt       *timestamppb.Timestamp `protobuf:"bytes,9,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	CompanyId       string                 `protobuf:"bytes,10,opt,name=company_id,json=companyId,proto3" json:"company_id,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}
```

- [ ] **Шаг 2: Добавить геттер GetCompanyId для SenderNameInfo**

Найти:
```go
func (x *SenderNameInfo) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}

type SenderNameHistoryEntry struct {
```
Заменить на:
```go
func (x *SenderNameInfo) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}

func (x *SenderNameInfo) GetCompanyId() string {
	if x != nil {
		return x.CompanyId
	}
	return ""
}

type SenderNameHistoryEntry struct {
```

- [ ] **Шаг 3: Добавить поле CompanyId в CreateSenderNameRequest**

Найти:
```go
type CreateSenderNameRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ClientId      string                 `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}
```
Заменить на:
```go
type CreateSenderNameRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ClientId      string                 `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	CompanyId     string                 `protobuf:"bytes,3,opt,name=company_id,json=companyId,proto3" json:"company_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}
```

- [ ] **Шаг 4: Добавить геттер GetCompanyId для CreateSenderNameRequest**

Найти:
```go
func (x *CreateSenderNameRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type CreateSenderNameResponse struct {
```
Заменить на:
```go
func (x *CreateSenderNameRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CreateSenderNameRequest) GetCompanyId() string {
	if x != nil {
		return x.CompanyId
	}
	return ""
}

type CreateSenderNameResponse struct {
```

- [ ] **Шаг 5: Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./api/proto/sendernamev1/...
```
Ожидание: нет ошибок.

- [ ] **Шаг 6: Коммит**

```bash
git add api/proto/sendernamev1/sender_name.pb.go
git commit -m "feat(proto): add company_id to SenderNameInfo and CreateSenderNameRequest"
```

---

## Task 2: Domain model — добавить CompanyID в SenderName

**Files:**
- Modify: `internal/services/template/domain/sender_name.go:37-47`

- [ ] **Шаг 1: Добавить поле CompanyID**

В файле `internal/services/template/domain/sender_name.go` найти:
```go
type SenderName struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Status          string
	RejectionReason string
	ReviewerID      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
```
Заменить на:
```go
type SenderName struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	CompanyID       uuid.UUID
	Name            string
	Status          string
	RejectionReason string
	ReviewerID      *uuid.UUID
	ReviewedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
```

- [ ] **Шаг 2: Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/services/template/...
```
Ожидание: компилятор укажет на места, где нужно дополнить (нет ошибок если CompanyID пока не используется).

- [ ] **Шаг 3: Коммит**

```bash
git add internal/services/template/domain/sender_name.go
git commit -m "feat(domain): add CompanyID to SenderName"
```

---

## Task 3: Repository — включить company_id в SQL

**Files:**
- Modify: `internal/services/template/infrastructure/repository/sender_name_repo.go`

- [ ] **Шаг 1: Добавить CompanyID в senderNameRow**

Найти:
```go
type senderNameRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	ReviewerID      *uuid.UUID     `db:"reviewer_id"`
	ReviewedAt      sql.NullTime   `db:"reviewed_at"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}
```
Заменить на:
```go
type senderNameRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	CompanyID       uuid.UUID      `db:"company_id"`
	Name            string         `db:"name"`
	Status          string         `db:"status"`
	RejectionReason sql.NullString `db:"rejection_reason"`
	ReviewerID      *uuid.UUID     `db:"reviewer_id"`
	ReviewedAt      sql.NullTime   `db:"reviewed_at"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}
```

- [ ] **Шаг 2: Маппить CompanyID в toDomain**

Найти:
```go
func (r *senderNameRow) toDomain() *domain.SenderName {
	sn := &domain.SenderName{
		ID:        r.ID,
		ClientID:  r.ClientID,
		Name:      r.Name,
```
Заменить на:
```go
func (r *senderNameRow) toDomain() *domain.SenderName {
	sn := &domain.SenderName{
		ID:        r.ID,
		ClientID:  r.ClientID,
		CompanyID: r.CompanyID,
		Name:      r.Name,
```

- [ ] **Шаг 3: Обновить Create() — INSERT и RETURNING**

Найти:
```go
	const q = `
		INSERT INTO sender_names (id, client_id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`

	var row senderNameRow
	err := r.db.QueryRowxContext(ctx, q, sn.ID, sn.ClientID, sn.Name, sn.Status).StructScan(&row)
```
Заменить на:
```go
	const q = `
		INSERT INTO sender_names (id, client_id, company_id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`

	var row senderNameRow
	err := r.db.QueryRowxContext(ctx, q, sn.ID, sn.ClientID, sn.CompanyID, sn.Name, sn.Status).StructScan(&row)
```

- [ ] **Шаг 4: Обновить SELECT в GetByID и GetByClientAndName**

Найти в `GetByID`:
```go
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE id = $1`
```
Заменить на:
```go
		SELECT id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE id = $1`
```

Найти в `GetByClientAndName`:
```go
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE client_id = $1 AND name = $2`
```
Заменить на:
```go
		SELECT id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names WHERE client_id = $1 AND name = $2`
```

- [ ] **Шаг 5: Обновить SELECT в ListByClient и ListAll**

Найти в `ListByClient`:
```go
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))
```
Заменить на:
```go
		SELECT id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))
```

Найти в `ListAll`:
```go
		SELECT id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))
```
Заменить на:
```go
		SELECT id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at
		FROM sender_names %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))
```

- [ ] **Шаг 6: Обновить RETURNING в UpdateStatus и Update**

Найти в `UpdateStatus`:
```go
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`
```
(в методе `UpdateStatus`)
Заменить на:
```go
		RETURNING id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`
```

Найти в `Update`:
```go
		RETURNING id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`
```
(в методе `Update`)
Заменить на:
```go
		RETURNING id, client_id, company_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at`
```

- [ ] **Шаг 7: Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/services/template/infrastructure/...
```
Ожидание: нет ошибок.

- [ ] **Шаг 8: Коммит**

```bash
git add internal/services/template/infrastructure/repository/sender_name_repo.go
git commit -m "feat(repo): include company_id in all sender_names queries"
```

---

## Task 4: Application Service — принять companyID в RegisterSenderName

**Files:**
- Modify: `internal/services/template/application/sender_name_service.go:38-63`

- [ ] **Шаг 1: Обновить сигнатуру и тело RegisterSenderName**

Найти:
```go
func (s *SenderNameService) RegisterSenderName(ctx context.Context, clientID uuid.UUID, name string) (*domain.SenderName, error) {
	if err := domain.ValidateSenderName(name); err != nil {
		return nil, err
	}

	sn := &domain.SenderName{
		ID:       uuid.New(),
		ClientID: clientID,
		Name:     name,
		Status:   domain.SenderNameStatusPending,
	}
```
Заменить на:
```go
func (s *SenderNameService) RegisterSenderName(ctx context.Context, clientID, companyID uuid.UUID, name string) (*domain.SenderName, error) {
	if err := domain.ValidateSenderName(name); err != nil {
		return nil, err
	}

	sn := &domain.SenderName{
		ID:        uuid.New(),
		ClientID:  clientID,
		CompanyID: companyID,
		Name:      name,
		Status:    domain.SenderNameStatusPending,
	}
```

- [ ] **Шаг 2: Проверить компиляцию (ожидаем ошибку в gRPC handler)**

```bash
cd c:/projects/sms && go build ./internal/services/template/...
```
Ожидание: ошибка компилятора в `grpc/sender_name_handler.go` — `RegisterSenderName` вызывается со старой сигнатурой. Это нормально — исправим в Task 5.

- [ ] **Шаг 3: Коммит**

```bash
git add internal/services/template/application/sender_name_service.go
git commit -m "feat(service): add companyID param to RegisterSenderName"
```

---

## Task 5: gRPC Handler — передать company_id через слой

**Files:**
- Modify: `internal/services/template/grpc/sender_name_handler.go:32-51,290-307`

- [ ] **Шаг 1: Обновить CreateSenderName — валидация и вызов сервиса**

Найти:
```go
func (h *SenderNameHandler) CreateSenderName(ctx context.Context, req *sendernamev1.CreateSenderNameRequest) (*sendernamev1.CreateSenderNameResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	sn, err := h.svc.RegisterSenderName(ctx, clientID, req.Name)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.CreateSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}
```
Заменить на:
```go
func (h *SenderNameHandler) CreateSenderName(ctx context.Context, req *sendernamev1.CreateSenderNameRequest) (*sendernamev1.CreateSenderNameResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id format")
	}

	sn, err := h.svc.RegisterSenderName(ctx, clientID, companyID, req.Name)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &sendernamev1.CreateSenderNameResponse{SenderName: senderNameToProto(sn)}, nil
}
```

- [ ] **Шаг 2: Добавить CompanyId в senderNameToProto**

Найти:
```go
func senderNameToProto(sn *domain.SenderName) *sendernamev1.SenderNameInfo {
	info := &sendernamev1.SenderNameInfo{
		Id:              sn.ID.String(),
		ClientId:        sn.ClientID.String(),
		Name:            sn.Name,
		Status:          sn.Status,
		RejectionReason: sn.RejectionReason,
		CreatedAt:       timestamppb.New(sn.CreatedAt),
		UpdatedAt:       timestamppb.New(sn.UpdatedAt),
	}
```
Заменить на:
```go
func senderNameToProto(sn *domain.SenderName) *sendernamev1.SenderNameInfo {
	info := &sendernamev1.SenderNameInfo{
		Id:              sn.ID.String(),
		ClientId:        sn.ClientID.String(),
		CompanyId:       sn.CompanyID.String(),
		Name:            sn.Name,
		Status:          sn.Status,
		RejectionReason: sn.RejectionReason,
		CreatedAt:       timestamppb.New(sn.CreatedAt),
		UpdatedAt:       timestamppb.New(sn.UpdatedAt),
	}
```

- [ ] **Шаг 3: Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/services/template/...
```
Ожидание: нет ошибок.

- [ ] **Шаг 4: Коммит**

```bash
git add internal/services/template/grpc/sender_name_handler.go
git commit -m "feat(grpc): pass company_id in CreateSenderName handler"
```

---

## Task 6: Portal HTTP Handler — читать company_id, fallback к дефолтной компании

**Files:**
- Modify: `internal/gateway/portal/handlers/sender_names.go:47-76`

- [ ] **Шаг 1: Добавить CompanyID в createSenderNameRequest**

Найти:
```go
type createSenderNameRequest struct {
	Name string `json:"name"`
}
```
Заменить на:
```go
type createSenderNameRequest struct {
	Name      string `json:"name"`
	CompanyID string `json:"company_id"`
}
```

- [ ] **Шаг 2: Обновить CreateSenderName — fallback и передача company_id в gRPC**

Найти:
```go
	var req createSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	resp, err := h.client.CreateSenderName(r.Context(), &sendernamev1.CreateSenderNameRequest{
		ClientId: clientID.String(),
		Name:     req.Name,
	})
```
Заменить на:
```go
	var req createSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}

	companyID := req.CompanyID
	if companyID == "" {
		if h.pool == nil {
			respondError(w, shared.ErrInternalServer("database pool недоступен"))
			return
		}
		if err := h.pool.QueryRow(r.Context(),
			`SELECT company_id FROM client_companies WHERE client_id = $1 AND is_default = TRUE LIMIT 1`,
			clientID,
		).Scan(&companyID); err != nil {
			log.Error().Err(err).Str("client_id", clientID.String()).Msg("не найдена дефолтная компания клиента")
			respondError(w, shared.ErrInvalidInput("у клиента не найдена компания по умолчанию"))
			return
		}
	}

	resp, err := h.client.CreateSenderName(r.Context(), &sendernamev1.CreateSenderNameRequest{
		ClientId:  clientID.String(),
		Name:      req.Name,
		CompanyId: companyID,
	})
```

- [ ] **Шаг 3: Проверить компиляцию всего проекта**

```bash
cd c:/projects/sms && go build ./...
```
Ожидание: нет ошибок компиляции.

- [ ] **Шаг 4: Коммит**

```bash
git add internal/gateway/portal/handlers/sender_names.go
git commit -m "feat(portal): read company_id in CreateSenderName, fallback to default company"
```

---

## Task 7: Финальная проверка

- [ ] **Шаг 1: Запустить тесты пакета template**

```bash
cd c:/projects/sms && go test ./internal/services/template/... -v 2>&1 | tail -30
```
Ожидание: все тесты проходят (или пропущены).

- [ ] **Шаг 2: Запустить все Go-тесты**

```bash
cd c:/projects/sms && go test ./... 2>&1 | grep -E "FAIL|ok" | head -40
```
Ожидание: нет строк FAIL.

- [ ] **Шаг 3: Задеплоить на сервер и проверить вручную**

```bash
bash scripts/server.sh deploy portal
```
Затем в браузере: Имена отправителей → Зарегистрировать имя → выбрать компанию → ввести имя → нажать «Зарегистрировать».

Ожидание: имя создаётся без ошибки, появляется в списке.

- [ ] **Шаг 4: Итоговый коммит (если деплой прошёл успешно)**

Все коммиты уже сделаны в предыдущих задачах.
