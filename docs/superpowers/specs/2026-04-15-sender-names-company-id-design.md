# Sender Names: привязка company_id

**Дата:** 2026-04-15
**Статус:** Approved

---

## Проблема

Миграция `000091_create_companies.up.sql` добавила `company_id UUID NOT NULL` в таблицу `sender_names`. Весь UI компаний работает, форма создания sender name уже содержит дропдаун выбора компании и передаёт `company_id` в теле запроса. Однако бэкенд-стек (proto → service → repo) не обновлён — при создании sender name возникает ошибка:

```
null value in column "company_id" of relation "sender_names" violates not-null constraint (SQLSTATE 23502)
```

---

## Что уже готово

- **Frontend (`SenderNamesPage.tsx`):** дропдаун компаний, по умолчанию — дефолтная компания клиента; `senderNamesApi.create(name, companyId?)` передаёт `company_id` в теле
- **Страницы компаний:** создание, редактирование реквизитов, смена дефолтной компании — работают
- **DB:** `sender_names.company_id UUID NOT NULL FK → companies` — уже в проде

---

## Архитектура и поток данных

```
Frontend (готово)
  → POST /portal/v1/sender-names { name, company_id? }
  → Portal HTTP Handler  [читает company_id; если пусто — fallback к дефолтной компании клиента]
  → gRPC CreateSenderNameRequest { client_id, name, company_id }  [pb.go ручной патч]
  → SenderNameHandler (gRPC)  [парсит UUID, передаёт в service]
  → SenderNameService.RegisterSenderName(ctx, clientID, companyID uuid.UUID, name string)
  → SenderNameRepository.Create  [company_id в INSERT/RETURNING]
  → DB: sender_names.company_id NOT NULL ✓
```

### Fallback логика в Portal Handler

Если `company_id` не пришёл в теле запроса (клиент не выбрал компанию или не загрузил список):

```sql
SELECT company_id FROM client_companies
WHERE client_id = $1 AND is_default = TRUE
LIMIT 1
```

Компания-оферта создаётся при регистрации каждого клиента (миграция 000091), поэтому запись гарантированно есть. Если запись не найдена — возвращаем HTTP 400 с сообщением «компания не найдена».

---

## Изменения по файлам

### 1. `api/proto/sendernamev1/sender_name.pb.go` — ручной патч

`protoc` и `buf` не установлены. Поскольку gRPC-стек полностью на Go, сериализацию контролируют struct tags, а не rawDesc. Ручное добавление поля с правильными тегами корректно работает для Marshal/Unmarshal между Go-сервисами.

**`CreateSenderNameRequest`:** добавить поле с тегом field=3:
```go
CompanyId string `protobuf:"bytes,3,opt,name=company_id,json=companyId,proto3" json:"company_id,omitempty"`
```
+ геттер `GetCompanyId() string`

**`SenderNameInfo`:** добавить поле с тегом field=10 (после `UpdatedAt` = field 9):
```go
CompanyId string `protobuf:"bytes,10,opt,name=company_id,json=companyId,proto3" json:"company_id,omitempty"`
```
+ геттер `GetCompanyId() string`

### 2. `internal/services/template/domain/sender_name.go`

`SenderName` struct: добавить `CompanyID uuid.UUID`

### 3. `internal/services/template/infrastructure/repository/sender_name_repo.go`

- `senderNameRow`: добавить `CompanyID uuid.UUID \`db:"company_id"\``
- `toDomain()`: маппить `CompanyID: r.CompanyID`
- `Create()`: включить `company_id` в INSERT и RETURNING:
  ```sql
  INSERT INTO sender_names (id, client_id, company_id, name, status, created_at, updated_at)
  VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
  RETURNING id, client_id, company_id, name, status, ...
  ```
  Аргументы: `sn.ID, sn.ClientID, sn.CompanyID, sn.Name, sn.Status`

### 4. `internal/services/template/application/sender_name_service.go`

Сигнатура:
```go
func (s *SenderNameService) RegisterSenderName(ctx context.Context, clientID, companyID uuid.UUID, name string) (*domain.SenderName, error)
```

В теле: `sn.CompanyID = companyID` перед `s.repo.Create(ctx, sn)`

### 5. `internal/services/template/grpc/sender_name_handler.go`

`CreateSenderName`:
- Валидировать `req.CompanyId != ""` (обязательное поле)
- `companyID, err := uuid.Parse(req.CompanyId)` — ошибка → `codes.InvalidArgument`
- Передать в `svc.RegisterSenderName(ctx, clientID, companyID, req.Name)`

`senderNameToProto`:
- Добавить `CompanyId: sn.CompanyID.String()`

### 6. `internal/gateway/portal/handlers/sender_names.go`

`createSenderNameRequest`:
```go
type createSenderNameRequest struct {
    Name      string `json:"name"`
    CompanyID string `json:"company_id"`
}
```

`CreateSenderName` — логика до gRPC-вызова:
```go
companyID := req.CompanyID
if companyID == "" {
    // fallback: дефолтная компания клиента
    err := h.pool.QueryRow(ctx,
        `SELECT company_id FROM client_companies WHERE client_id=$1 AND is_default=TRUE LIMIT 1`,
        clientID,
    ).Scan(&companyID)
    if err != nil {
        respondError(w, shared.ErrInvalidInput("у клиента не найдена компания по умолчанию"))
        return
    }
}
```

Затем передать `CompanyId: companyID` в `sendernamev1.CreateSenderNameRequest`.

---

## Граничные случаи

| Случай | Поведение |
|--------|-----------|
| `company_id` не пришёл, дефолтная компания есть | auto-fallback, создаётся с дефолтной |
| `company_id` не пришёл, дефолтной нет | HTTP 400 «у клиента не найдена компания» |
| `company_id` пришёл, невалидный UUID | HTTP 400 от gRPC handler |
| `company_id` пришёл, валидный UUID | создаётся с указанной компанией |

---

## Не затрагивается

- Миграции БД — не нужны
- Frontend — уже готов
- Остальные методы SenderNameService (Update, Resubmit, Approve, Reject, Deactivate) — не трогают company_id
- Admin gateway — не трогаем (admin не создаёт sender names от имени клиента)
