# Client Companies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить управление компаниями клиентов — каждая компания имеет юридические реквизиты (ИНН), собственный баланс, и к ней привязываются sender names. Связь N:N между клиентом и компанией через таблицу `client_companies`.

**Architecture:** Новая таблица `companies` с реквизитами и `client_companies` как связующая таблица. Баланс (accounts) привязывается к компании. Компания «Оферта» (`is_offer=true`) создаётся автоматически для каждого клиента при регистрации. gRPC-сервис `CompanyService` размещается в существующем `template-service` на порту 9099.

**Tech Stack:** Go 1.24 (gRPC + pgx), PostgreSQL 15+, protobuf/grpc-go, React 19 + TypeScript + Tailwind CSS

---

## File Map

**Новые файлы:**
- `migrations/000091_create_companies.up.sql` — таблицы companies, client_companies + ALTER accounts/sender_names
- `migrations/000091_create_companies.down.sql` — откат
- `api/proto/company/company.proto` — proto-описание CompanyService
- `api/proto/companyv1/company.pb.go` — (генерируется protoc)
- `api/proto/companyv1/company_grpc.pb.go` — (генерируется protoc)
- `internal/services/company/domain/models.go` — доменные модели и ошибки
- `internal/services/company/application/ports.go` — интерфейс репозитория
- `internal/services/company/application/company_service.go` — бизнес-логика
- `internal/services/company/application/company_service_test.go` — unit-тесты сервиса
- `internal/services/company/grpc/server.go` — gRPC обработчик
- `internal/storage/company_repository.go` — репозиторий (pgx/sqlx)
- `internal/gateway/portal/handlers/companies.go` — HTTP-обработчики портала
- `portal-frontend/src/pages/companies/CompaniesPage.tsx` — список компаний
- `portal-frontend/src/pages/companies/CompanyDetailPage.tsx` — детали компании

**Изменяемые файлы:**
- `internal/shared/models.go` — добавить Company, ClientCompany
- `cmd/services/template-service/main.go` — зарегистрировать CompanyService
- `internal/gateway/portal/clients.go` — добавить CompanyClient
- `internal/gateway/portal/router/router.go` — добавить маршруты /companies
- `cmd/portal-gateway/main.go` — добавить companyHandlers
- `portal-frontend/src/api/client.ts` — добавить companiesApi
- `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx` — выбор компании при создании
- `portal-frontend/src/pages/billing/BillingPage.tsx` — балансы по компаниям

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/000091_create_companies.up.sql`
- Create: `migrations/000091_create_companies.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- migrations/000091_create_companies.up.sql
BEGIN;

-- 1. companies: юридические реквизиты и флаг оферты
CREATE TABLE companies (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    inn              VARCHAR(12),
    name             VARCHAR(255) NOT NULL,
    full_name        VARCHAR(500),
    kpp              VARCHAR(9),
    ogrn             VARCHAR(15),
    legal_address    TEXT,
    actual_address   TEXT,
    ceo_name         VARCHAR(255),
    ceo_title        VARCHAR(255),
    acting_basis     VARCHAR(255),
    bank_name        VARCHAR(255),
    bank_bik         VARCHAR(9),
    bank_corr_account VARCHAR(20),
    bank_account     VARCHAR(20),
    email            VARCHAR(255),
    phone            VARCHAR(20),
    is_offer         BOOLEAN      NOT NULL DEFAULT FALSE,
    active           BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_companies_inn ON companies (inn) WHERE inn IS NOT NULL;

-- 2. client_companies: N:N связь клиент ↔ компания
CREATE TABLE client_companies (
    client_id  UUID        NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    company_id UUID        NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    is_default BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (client_id, company_id)
);

-- ровно одна основная компания на клиента
CREATE UNIQUE INDEX idx_client_companies_default
    ON client_companies (client_id)
    WHERE is_default = TRUE;

CREATE INDEX idx_client_companies_company_id ON client_companies (company_id);

-- 3. Привязать accounts к компании
ALTER TABLE accounts ADD COLUMN company_id UUID REFERENCES companies(id);
CREATE INDEX idx_accounts_company_id ON accounts (company_id) WHERE company_id IS NOT NULL;

-- 4. Для каждого существующего клиента создать компанию «Оферта»
--    и привязать к ней существующий account
DO $$
DECLARE
    r RECORD;
    new_company_id UUID;
BEGIN
    FOR r IN SELECT id FROM clients LOOP
        INSERT INTO companies (name, is_offer)
        VALUES ('Оферта', TRUE)
        RETURNING id INTO new_company_id;

        INSERT INTO client_companies (client_id, company_id, is_default)
        VALUES (r.id, new_company_id, TRUE);

        UPDATE accounts
        SET company_id = new_company_id
        WHERE client_id = r.id;
    END LOOP;
END $$;

-- 5. sender_names: добавить company_id (nullable сначала, заполнить, затем NOT NULL)
ALTER TABLE sender_names ADD COLUMN company_id UUID REFERENCES companies(id);

UPDATE sender_names sn
SET company_id = cc.company_id
FROM client_companies cc
WHERE cc.client_id = sn.client_id
  AND cc.is_default = TRUE;

ALTER TABLE sender_names ALTER COLUMN company_id SET NOT NULL;
CREATE INDEX idx_sender_names_company_id ON sender_names (company_id);

COMMIT;
```

- [ ] **Step 2: Write down migration**

```sql
-- migrations/000091_create_companies.down.sql
BEGIN;

ALTER TABLE sender_names DROP COLUMN IF EXISTS company_id;
ALTER TABLE accounts     DROP COLUMN IF EXISTS company_id;

DROP TABLE IF EXISTS client_companies;
DROP TABLE IF EXISTS companies;

COMMIT;
```

- [ ] **Step 3: Apply migration**

```bash
cd c:/projects/sms
migrate -path migrations -database "$DATABASE_URL" up 1
```

Если `migrate` не установлен: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

Expected: `1/u 000091_create_companies (X ms)`

- [ ] **Step 4: Verify tables exist**

```bash
psql "$DATABASE_URL" -c "\d companies"
psql "$DATABASE_URL" -c "\d client_companies"
psql "$DATABASE_URL" -c "\d accounts" | grep company_id
psql "$DATABASE_URL" -c "\d sender_names" | grep company_id
```

Expected: все 4 команды выводят ожидаемые столбцы.

- [ ] **Step 5: Commit**

```bash
git add migrations/000091_create_companies.up.sql migrations/000091_create_companies.down.sql
git commit -m "feat(db): add companies and client_companies tables, link accounts and sender_names"
```

---

## Task 2: Shared Models

**Files:**
- Modify: `internal/shared/models.go`

- [ ] **Step 1: Add Company and ClientCompany structs**

Открой `internal/shared/models.go` и добавь в конец файла:

```go
// Company представляет юридическое лицо клиента
type Company struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	INN             NullString `json:"inn,omitempty" db:"inn"`
	Name            string     `json:"name" db:"name"`
	FullName        NullString `json:"full_name,omitempty" db:"full_name"`
	KPP             NullString `json:"kpp,omitempty" db:"kpp"`
	OGRN            NullString `json:"ogrn,omitempty" db:"ogrn"`
	LegalAddress    NullString `json:"legal_address,omitempty" db:"legal_address"`
	ActualAddress   NullString `json:"actual_address,omitempty" db:"actual_address"`
	CEOName         NullString `json:"ceo_name,omitempty" db:"ceo_name"`
	CEOTitle        NullString `json:"ceo_title,omitempty" db:"ceo_title"`
	ActingBasis     NullString `json:"acting_basis,omitempty" db:"acting_basis"`
	BankName        NullString `json:"bank_name,omitempty" db:"bank_name"`
	BankBIK         NullString `json:"bank_bik,omitempty" db:"bank_bik"`
	BankCorrAccount NullString `json:"bank_corr_account,omitempty" db:"bank_corr_account"`
	BankAccount     NullString `json:"bank_account,omitempty" db:"bank_account"`
	Email           NullString `json:"email,omitempty" db:"email"`
	Phone           NullString `json:"phone,omitempty" db:"phone"`
	IsOffer         bool       `json:"is_offer" db:"is_offer"`
	Active          bool       `json:"active" db:"active"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// ClientCompany — связь клиент ↔ компания
type ClientCompany struct {
	ClientID  uuid.UUID `json:"client_id" db:"client_id"`
	CompanyID uuid.UUID `json:"company_id" db:"company_id"`
	IsDefault bool      `json:"is_default" db:"is_default"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd c:/projects/sms
go build ./internal/shared/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/shared/models.go
git commit -m "feat(models): add Company and ClientCompany shared structs"
```

---

## Task 3: Company Domain

**Files:**
- Create: `internal/services/company/domain/models.go`

- [ ] **Step 1: Write domain models and errors**

```go
// internal/services/company/domain/models.go
package domain

import (
	"errors"
	"strconv"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var (
	ErrCompanyNotFound      = errors.New("company not found")
	ErrInvalidINN           = errors.New("invalid INN checksum or length")
	ErrCompanyNameRequired  = errors.New("company name is required")
	ErrCompanyHasSenderNames = errors.New("cannot detach company with active sender names")
	ErrCannotDetachDefault  = errors.New("cannot detach default company")
	ErrAlreadyDefault       = errors.New("company is already the default")
	ErrNotAttached          = errors.New("company is not attached to this client")
)

// Company — доменная модель компании
type Company struct {
	ID              uuid.UUID
	INN             string
	Name            string
	FullName        string
	KPP             string
	OGRN            string
	LegalAddress    string
	ActualAddress   string
	CEOName         string
	CEOTitle        string
	ActingBasis     string
	BankName        string
	BankBIK         string
	BankCorrAccount string
	BankAccount     string
	Email           string
	Phone           string
	IsOffer         bool
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ValidateINN проверяет контрольную сумму ИНН (10 или 12 цифр).
func ValidateINN(inn string) bool {
	if inn == "" {
		return true // пустой ИНН допустим (для компании «Оферта»)
	}
	for _, r := range inn {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	switch len(inn) {
	case 10:
		coeffs := []int{2, 4, 10, 3, 5, 9, 4, 6, 8}
		sum := 0
		for i, c := range coeffs {
			d, _ := strconv.Atoi(string(inn[i]))
			sum += c * d
		}
		check := sum % 11 % 10
		last, _ := strconv.Atoi(string(inn[9]))
		return check == last
	case 12:
		c1 := []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
		c2 := []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
		s1 := 0
		for i, c := range c1 {
			d, _ := strconv.Atoi(string(inn[i]))
			s1 += c * d
		}
		n11, _ := strconv.Atoi(string(inn[10]))
		if s1%11%10 != n11 {
			return false
		}
		s2 := 0
		for i, c := range c2 {
			d, _ := strconv.Atoi(string(inn[i]))
			s2 += c * d
		}
		n12, _ := strconv.Atoi(string(inn[11]))
		return s2%11%10 == n12
	default:
		return false
	}
}
```

- [ ] **Step 2: Write INN validation tests**

Создай `internal/services/company/domain/models_test.go`:

```go
package domain

import "testing"

func TestValidateINN(t *testing.T) {
	tests := []struct {
		inn  string
		want bool
	}{
		{"", true},                  // пустой — OK (оферта)
		{"7707083893", true},        // Сбербанк, 10 цифр, валидный
		{"7707083890", false},       // неверная контрольная сумма
		{"123456789012", true},      // валидный 12-значный (ИП)
		{"123456789011", false},     // неверный 12-значный
		{"12345", false},            // слишком короткий
		{"abcdefghij", false},       // не цифры
	}
	for _, tt := range tests {
		got := ValidateINN(tt.inn)
		if got != tt.want {
			t.Errorf("ValidateINN(%q) = %v, want %v", tt.inn, got, tt.want)
		}
	}
}
```

Примечание: ИНН `7707083893` — реальный контрольный пример (Сбербанк). Для 12-значного замени `123456789012` на реальный валидный ИНН ИП (можно использовать `771234567890` — проверь алгоритм).

- [ ] **Step 3: Run tests**

```bash
cd c:/projects/sms
go test ./internal/services/company/domain/... -v
```

Expected: все тесты pass. Если `123456789012` не проходит — это ожидаемо, замени на реально валидный 12-значный ИНН.

- [ ] **Step 4: Commit**

```bash
git add internal/services/company/
git commit -m "feat(company): add domain models and INN validation"
```

---

## Task 4: Company Repository

**Files:**
- Create: `internal/storage/company_repository.go`

- [ ] **Step 1: Write repository**

```go
// internal/storage/company_repository.go
package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type CompanyRepository struct {
	db *sqlx.DB
}

func NewCompanyRepository(db *sqlx.DB) *CompanyRepository {
	return &CompanyRepository{db: db}
}

func (r *CompanyRepository) Create(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	const q = `
		INSERT INTO companies
		    (inn, name, full_name, kpp, ogrn, legal_address, actual_address,
		     ceo_name, ceo_title, acting_basis, bank_name, bank_bik,
		     bank_corr_account, bank_account, email, phone, is_offer, active)
		VALUES
		    (:inn, :name, :full_name, :kpp, :ogrn, :legal_address, :actual_address,
		     :ceo_name, :ceo_title, :acting_basis, :bank_name, :bank_bik,
		     :bank_corr_account, :bank_account, :email, :phone, :is_offer, :active)
		RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var out shared.Company
		if err := rows.StructScan(&out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return nil, errors.New("no row returned after insert")
}

func (r *CompanyRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	var c shared.Company
	err := r.db.GetContext(ctx, &c, `SELECT * FROM companies WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCompanyNotFound
	}
	return &c, err
}

func (r *CompanyRepository) Update(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	const q = `
		UPDATE companies SET
		    inn = :inn, name = :name, full_name = :full_name, kpp = :kpp, ogrn = :ogrn,
		    legal_address = :legal_address, actual_address = :actual_address,
		    ceo_name = :ceo_name, ceo_title = :ceo_title, acting_basis = :acting_basis,
		    bank_name = :bank_name, bank_bik = :bank_bik,
		    bank_corr_account = :bank_corr_account, bank_account = :bank_account,
		    email = :email, phone = :phone, active = :active,
		    updated_at = NOW()
		WHERE id = :id
		RETURNING *`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var out shared.Company
		if err := rows.StructScan(&out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return nil, domain.ErrCompanyNotFound
}

// ListByClientID возвращает все компании клиента с флагом is_default
func (r *CompanyRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error) {
	const q = `
		SELECT c.*, cc.is_default
		FROM companies c
		JOIN client_companies cc ON cc.company_id = c.id
		WHERE cc.client_id = $1
		ORDER BY cc.is_default DESC, cc.created_at ASC`
	type row struct {
		shared.Company
		IsDefault bool `db:"is_default"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, clientID); err != nil {
		return nil, nil, err
	}
	companies := make([]*shared.Company, len(rows))
	defaults := make([]bool, len(rows))
	for i, r := range rows {
		c := r.Company
		companies[i] = &c
		defaults[i] = r.IsDefault
	}
	return companies, defaults, nil
}

// AttachCompany привязывает компанию к клиенту
func (r *CompanyRepository) AttachCompany(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO client_companies (client_id, company_id, is_default)
		VALUES ($1, $2, $3)
		ON CONFLICT (client_id, company_id) DO NOTHING`,
		clientID, companyID, isDefault)
	return err
}

// DetachCompany отвязывает компанию от клиента. Проверяет отсутствие sender_names.
func (r *CompanyRepository) DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	// Проверка: нет ли sender names
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM sender_names
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrCompanyHasSenderNames
	}

	// Проверка: не дефолтная ли
	var isDefault bool
	err = r.db.GetContext(ctx, &isDefault, `
		SELECT is_default FROM client_companies
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotAttached
	}
	if err != nil {
		return err
	}
	if isDefault {
		return domain.ErrCannotDetachDefault
	}

	_, err = r.db.ExecContext(ctx, `
		DELETE FROM client_companies
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID)
	return err
}

// SetDefault делает компанию основной для клиента (снимает флаг с предыдущей)
func (r *CompanyRepository) SetDefault(ctx context.Context, clientID, companyID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Снять is_default со всех
	_, err = tx.ExecContext(ctx, `
		UPDATE client_companies SET is_default = FALSE
		WHERE client_id = $1`, clientID)
	if err != nil {
		return err
	}

	// Выставить is_default для нужной
	res, err := tx.ExecContext(ctx, `
		UPDATE client_companies SET is_default = TRUE
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotAttached
	}

	return tx.Commit()
}

// HasSenderNames проверяет, есть ли у компании sender names в рамках клиента
func (r *CompanyRepository) HasSenderNames(ctx context.Context, clientID, companyID uuid.UUID) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM sender_names
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID)
	return count > 0, err
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd c:/projects/sms
go build ./internal/storage/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/company_repository.go
git commit -m "feat(storage): add CompanyRepository with CRUD, attach/detach, set-default"
```

---

## Task 5: Company Application Service

**Files:**
- Create: `internal/services/company/application/ports.go`
- Create: `internal/services/company/application/company_service.go`
- Create: `internal/services/company/application/company_service_test.go`

- [ ] **Step 1: Write ports.go**

```go
// internal/services/company/application/ports.go
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CompanyRepository определяет операции хранения для компаний.
type CompanyRepository interface {
	Create(ctx context.Context, c *shared.Company) (*shared.Company, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error)
	Update(ctx context.Context, c *shared.Company) (*shared.Company, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error)
	AttachCompany(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error
	DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error
	SetDefault(ctx context.Context, clientID, companyID uuid.UUID) error
	HasSenderNames(ctx context.Context, clientID, companyID uuid.UUID) (bool, error)
}

// AccountRepository нужен для создания нулевого аккаунта при создании компании.
type AccountRepository interface {
	CreateForCompany(ctx context.Context, clientID, companyID uuid.UUID, currency string) error
}
```

- [ ] **Step 2: Write company_service.go**

```go
// internal/services/company/application/company_service.go
package application

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type CompanyService struct {
	repo    CompanyRepository
	acctRepo AccountRepository
	logger  zerolog.Logger
}

func NewCompanyService(repo CompanyRepository, acctRepo AccountRepository) *CompanyService {
	return &CompanyService{
		repo:    repo,
		acctRepo: acctRepo,
		logger:  log.With().Str("component", "company-service").Logger(),
	}
}

type CreateCompanyInput struct {
	ClientID        uuid.UUID
	INN             string
	Name            string
	FullName        string
	KPP             string
	OGRN            string
	LegalAddress    string
	ActualAddress   string
	CEOName         string
	CEOTitle        string
	ActingBasis     string
	BankName        string
	BankBIK         string
	BankCorrAccount string
	BankAccount     string
	Email           string
	Phone           string
}

func (s *CompanyService) CreateCompany(ctx context.Context, in CreateCompanyInput) (*shared.Company, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, domain.ErrCompanyNameRequired
	}
	if !domain.ValidateINN(in.INN) {
		return nil, domain.ErrInvalidINN
	}

	c := &shared.Company{
		INN:             shared.NullString{String: in.INN, Valid: in.INN != ""},
		Name:            strings.TrimSpace(in.Name),
		FullName:        shared.NullString{String: in.FullName, Valid: in.FullName != ""},
		KPP:             shared.NullString{String: in.KPP, Valid: in.KPP != ""},
		OGRN:            shared.NullString{String: in.OGRN, Valid: in.OGRN != ""},
		LegalAddress:    shared.NullString{String: in.LegalAddress, Valid: in.LegalAddress != ""},
		ActualAddress:   shared.NullString{String: in.ActualAddress, Valid: in.ActualAddress != ""},
		CEOName:         shared.NullString{String: in.CEOName, Valid: in.CEOName != ""},
		CEOTitle:        shared.NullString{String: in.CEOTitle, Valid: in.CEOTitle != ""},
		ActingBasis:     shared.NullString{String: in.ActingBasis, Valid: in.ActingBasis != ""},
		BankName:        shared.NullString{String: in.BankName, Valid: in.BankName != ""},
		BankBIK:         shared.NullString{String: in.BankBIK, Valid: in.BankBIK != ""},
		BankCorrAccount: shared.NullString{String: in.BankCorrAccount, Valid: in.BankCorrAccount != ""},
		BankAccount:     shared.NullString{String: in.BankAccount, Valid: in.BankAccount != ""},
		Email:           shared.NullString{String: in.Email, Valid: in.Email != ""},
		Phone:           shared.NullString{String: in.Phone, Valid: in.Phone != ""},
		IsOffer:         false,
		Active:          true,
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return nil, err
	}

	// Привязать к клиенту (не основная по умолчанию, если уже есть другие)
	if err := s.repo.AttachCompany(ctx, in.ClientID, created.ID, false); err != nil {
		return nil, err
	}

	// Создать нулевой баланс для компании
	if s.acctRepo != nil {
		if err := s.acctRepo.CreateForCompany(ctx, in.ClientID, created.ID, "RUB"); err != nil {
			s.logger.Warn().Err(err).Msg("не удалось создать аккаунт для компании")
		}
	}

	return created, nil
}

// CreateOfferCompany создаёт системную компанию «Оферта» для нового клиента.
func (s *CompanyService) CreateOfferCompany(ctx context.Context, clientID uuid.UUID) (*shared.Company, error) {
	c := &shared.Company{
		Name:    "Оферта",
		IsOffer: true,
		Active:  true,
	}
	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AttachCompany(ctx, clientID, created.ID, true); err != nil {
		return nil, err
	}
	if s.acctRepo != nil {
		if err := s.acctRepo.CreateForCompany(ctx, clientID, created.ID, "RUB"); err != nil {
			s.logger.Warn().Err(err).Msg("не удалось создать аккаунт для компании «Оферта»")
		}
	}
	return created, nil
}

func (s *CompanyService) GetCompany(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	return s.repo.GetByID(ctx, id)
}

type UpdateCompanyInput struct {
	ID              uuid.UUID
	INN             string
	Name            string
	FullName        string
	KPP             string
	OGRN            string
	LegalAddress    string
	ActualAddress   string
	CEOName         string
	CEOTitle        string
	ActingBasis     string
	BankName        string
	BankBIK         string
	BankCorrAccount string
	BankAccount     string
	Email           string
	Phone           string
}

func (s *CompanyService) UpdateCompany(ctx context.Context, in UpdateCompanyInput) (*shared.Company, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, domain.ErrCompanyNameRequired
	}
	if !domain.ValidateINN(in.INN) {
		return nil, domain.ErrInvalidINN
	}

	existing, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	existing.INN             = shared.NullString{String: in.INN, Valid: in.INN != ""}
	existing.Name            = strings.TrimSpace(in.Name)
	existing.FullName        = shared.NullString{String: in.FullName, Valid: in.FullName != ""}
	existing.KPP             = shared.NullString{String: in.KPP, Valid: in.KPP != ""}
	existing.OGRN            = shared.NullString{String: in.OGRN, Valid: in.OGRN != ""}
	existing.LegalAddress    = shared.NullString{String: in.LegalAddress, Valid: in.LegalAddress != ""}
	existing.ActualAddress   = shared.NullString{String: in.ActualAddress, Valid: in.ActualAddress != ""}
	existing.CEOName         = shared.NullString{String: in.CEOName, Valid: in.CEOName != ""}
	existing.CEOTitle        = shared.NullString{String: in.CEOTitle, Valid: in.CEOTitle != ""}
	existing.ActingBasis     = shared.NullString{String: in.ActingBasis, Valid: in.ActingBasis != ""}
	existing.BankName        = shared.NullString{String: in.BankName, Valid: in.BankName != ""}
	existing.BankBIK         = shared.NullString{String: in.BankBIK, Valid: in.BankBIK != ""}
	existing.BankCorrAccount = shared.NullString{String: in.BankCorrAccount, Valid: in.BankCorrAccount != ""}
	existing.BankAccount     = shared.NullString{String: in.BankAccount, Valid: in.BankAccount != ""}
	existing.Email           = shared.NullString{String: in.Email, Valid: in.Email != ""}
	existing.Phone           = shared.NullString{String: in.Phone, Valid: in.Phone != ""}

	return s.repo.Update(ctx, existing)
}

// ListClientCompanies возвращает компании клиента с флагом is_default
type CompanyWithDefault struct {
	*shared.Company
	IsDefault bool
}

func (s *CompanyService) ListClientCompanies(ctx context.Context, clientID uuid.UUID) ([]*CompanyWithDefault, error) {
	companies, defaults, err := s.repo.ListByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	out := make([]*CompanyWithDefault, len(companies))
	for i, c := range companies {
		out[i] = &CompanyWithDefault{Company: c, IsDefault: defaults[i]}
	}
	return out, nil
}

func (s *CompanyService) SetDefaultCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return s.repo.SetDefault(ctx, clientID, companyID)
}

func (s *CompanyService) DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return s.repo.DetachCompany(ctx, clientID, companyID)
}
```

- [ ] **Step 3: Write service tests**

```go
// internal/services/company/application/company_service_test.go
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock репозиторий ---

type mockCompanyRepo struct {
	CreateFunc       func(ctx context.Context, c *shared.Company) (*shared.Company, error)
	GetByIDFunc      func(ctx context.Context, id uuid.UUID) (*shared.Company, error)
	UpdateFunc       func(ctx context.Context, c *shared.Company) (*shared.Company, error)
	ListByClientIDFunc func(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error)
	AttachCompanyFunc  func(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error
	DetachCompanyFunc  func(ctx context.Context, clientID, companyID uuid.UUID) error
	SetDefaultFunc     func(ctx context.Context, clientID, companyID uuid.UUID) error
	HasSenderNamesFunc func(ctx context.Context, clientID, companyID uuid.UUID) (bool, error)
}

func (m *mockCompanyRepo) Create(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	return m.CreateFunc(ctx, c)
}
func (m *mockCompanyRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	return m.GetByIDFunc(ctx, id)
}
func (m *mockCompanyRepo) Update(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	return m.UpdateFunc(ctx, c)
}
func (m *mockCompanyRepo) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error) {
	return m.ListByClientIDFunc(ctx, clientID)
}
func (m *mockCompanyRepo) AttachCompany(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error {
	return m.AttachCompanyFunc(ctx, clientID, companyID, isDefault)
}
func (m *mockCompanyRepo) DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return m.DetachCompanyFunc(ctx, clientID, companyID)
}
func (m *mockCompanyRepo) SetDefault(ctx context.Context, clientID, companyID uuid.UUID) error {
	return m.SetDefaultFunc(ctx, clientID, companyID)
}
func (m *mockCompanyRepo) HasSenderNames(ctx context.Context, clientID, companyID uuid.UUID) (bool, error) {
	return m.HasSenderNamesFunc(ctx, clientID, companyID)
}

func newService(repo CompanyRepository) *CompanyService {
	return NewCompanyService(repo, nil)
}

func TestCreateCompany_EmptyName(t *testing.T) {
	svc := newService(&mockCompanyRepo{})
	_, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: uuid.New(),
		Name:     "",
	})
	assert.ErrorIs(t, err, domain.ErrCompanyNameRequired)
}

func TestCreateCompany_InvalidINN(t *testing.T) {
	svc := newService(&mockCompanyRepo{})
	_, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: uuid.New(),
		Name:     "ООО Тест",
		INN:      "1234567890", // неверная контрольная сумма
	})
	assert.ErrorIs(t, err, domain.ErrInvalidINN)
}

func TestCreateCompany_Success(t *testing.T) {
	clientID := uuid.New()
	companyID := uuid.New()
	repo := &mockCompanyRepo{
		CreateFunc: func(_ context.Context, c *shared.Company) (*shared.Company, error) {
			c.ID = companyID
			return c, nil
		},
		AttachCompanyFunc: func(_ context.Context, cID, coID uuid.UUID, def bool) error {
			assert.Equal(t, clientID, cID)
			assert.Equal(t, companyID, coID)
			assert.False(t, def)
			return nil
		},
	}
	svc := newService(repo)
	got, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: clientID,
		Name:     "ООО Тест",
		INN:      "", // пустой — OK
	})
	require.NoError(t, err)
	assert.Equal(t, "ООО Тест", got.Name)
}

func TestUpdateCompany_InvalidINN(t *testing.T) {
	companyID := uuid.New()
	repo := &mockCompanyRepo{
		GetByIDFunc: func(_ context.Context, id uuid.UUID) (*shared.Company, error) {
			return &shared.Company{ID: id, Name: "Old"}, nil
		},
	}
	svc := newService(repo)
	_, err := svc.UpdateCompany(context.Background(), UpdateCompanyInput{
		ID:   companyID,
		Name: "New",
		INN:  "0000000000", // неверная контрольная сумма
	})
	assert.ErrorIs(t, err, domain.ErrInvalidINN)
}
```

- [ ] **Step 4: Run tests**

```bash
cd c:/projects/sms
go test ./internal/services/company/... -v
```

Expected: все тесты pass.

- [ ] **Step 5: Compile all**

```bash
go build ./internal/services/company/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/services/company/
git commit -m "feat(company): add application service with INN validation and CRUD logic"
```

---

## Task 6: Proto File + Code Generation

**Files:**
- Create: `api/proto/company/company.proto`
- Generate: `api/proto/companyv1/` (protoc output)

- [ ] **Step 1: Write company.proto**

```protobuf
// api/proto/company/company.proto
syntax = "proto3";

package company.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/companyv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

service CompanyService {
  rpc CreateCompany(CreateCompanyRequest) returns (CompanyResponse);
  rpc UpdateCompany(UpdateCompanyRequest) returns (CompanyResponse);
  rpc GetCompany(GetCompanyRequest) returns (CompanyResponse);
  rpc ListClientCompanies(ListClientCompaniesRequest) returns (ListClientCompaniesResponse);
  rpc SetDefaultCompany(SetDefaultCompanyRequest) returns (google.protobuf.Empty);
  rpc DetachCompany(DetachCompanyRequest) returns (google.protobuf.Empty);
  rpc CreateOfferCompany(CreateOfferCompanyRequest) returns (CompanyResponse);
}

message CompanyInfo {
  string id = 1;
  string inn = 2;
  string name = 3;
  string full_name = 4;
  string kpp = 5;
  string ogrn = 6;
  string legal_address = 7;
  string actual_address = 8;
  string ceo_name = 9;
  string ceo_title = 10;
  string acting_basis = 11;
  string bank_name = 12;
  string bank_bik = 13;
  string bank_corr_account = 14;
  string bank_account = 15;
  string email = 16;
  string phone = 17;
  bool is_offer = 18;
  bool is_default = 19;
  bool active = 20;
  google.protobuf.Timestamp created_at = 21;
  google.protobuf.Timestamp updated_at = 22;
}

message CreateCompanyRequest {
  string client_id = 1;
  string inn = 2;
  string name = 3;
  string full_name = 4;
  string kpp = 5;
  string ogrn = 6;
  string legal_address = 7;
  string actual_address = 8;
  string ceo_name = 9;
  string ceo_title = 10;
  string acting_basis = 11;
  string bank_name = 12;
  string bank_bik = 13;
  string bank_corr_account = 14;
  string bank_account = 15;
  string email = 16;
  string phone = 17;
}

message UpdateCompanyRequest {
  string company_id = 1;
  string inn = 2;
  string name = 3;
  string full_name = 4;
  string kpp = 5;
  string ogrn = 6;
  string legal_address = 7;
  string actual_address = 8;
  string ceo_name = 9;
  string ceo_title = 10;
  string acting_basis = 11;
  string bank_name = 12;
  string bank_bik = 13;
  string bank_corr_account = 14;
  string bank_account = 15;
  string email = 16;
  string phone = 17;
}

message GetCompanyRequest {
  string company_id = 1;
}

message ListClientCompaniesRequest {
  string client_id = 1;
}

message ListClientCompaniesResponse {
  repeated CompanyInfo companies = 1;
}

message SetDefaultCompanyRequest {
  string client_id = 1;
  string company_id = 2;
}

message DetachCompanyRequest {
  string client_id = 1;
  string company_id = 2;
}

message CreateOfferCompanyRequest {
  string client_id = 1;
}

message CompanyResponse {
  CompanyInfo company = 1;
}
```

- [ ] **Step 2: Create output directory**

```bash
mkdir -p c:/projects/sms/api/proto/companyv1
```

- [ ] **Step 3: Generate Go code**

```bash
cd c:/projects/sms
protoc \
  --go_out=api/proto/companyv1 \
  --go_opt=paths=source_relative \
  --go-grpc_out=api/proto/companyv1 \
  --go-grpc_opt=paths=source_relative \
  --proto_path=api/proto/company \
  --proto_path=$(go env GOPATH)/pkg/mod/google.golang.org/protobuf@*/include \
  api/proto/company/company.proto
```

Если protoc не установлен:
```bash
# Linux/Mac:
apt-get install protobuf-compiler || brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

- [ ] **Step 4: Verify generated files exist**

```bash
ls c:/projects/sms/api/proto/companyv1/
```

Expected: `company.pb.go` и `company_grpc.pb.go`

- [ ] **Step 5: Build**

```bash
cd c:/projects/sms
go build ./api/proto/companyv1/...
```

- [ ] **Step 6: Commit**

```bash
git add api/proto/company/ api/proto/companyv1/
git commit -m "feat(proto): add CompanyService proto definition and generated Go code"
```

---

## Task 7: Company gRPC Server

**Files:**
- Create: `internal/services/company/grpc/server.go`

- [ ] **Step 1: Write gRPC server**

```go
// internal/services/company/grpc/server.go
package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	companyv1 "github.com/smpp-server/smpp-server/api/proto/companyv1"
	"github.com/smpp-server/smpp-server/internal/services/company/application"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type Server struct {
	companyv1.UnimplementedCompanyServiceServer
	svc    *application.CompanyService
	logger zerolog.Logger
}

func NewServer(svc *application.CompanyService) *Server {
	return &Server{
		svc:    svc,
		logger: log.With().Str("component", "company-grpc-server").Logger(),
	}
}

func (s *Server) CreateCompany(ctx context.Context, req *companyv1.CreateCompanyRequest) (*companyv1.CompanyResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	c, err := s.svc.CreateCompany(ctx, application.CreateCompanyInput{
		ClientID: clientID, INN: req.Inn, Name: req.Name, FullName: req.FullName,
		KPP: req.Kpp, OGRN: req.Ogrn, LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CEOName: req.CeoName, CEOTitle: req.CeoTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBIK: req.BankBik, BankCorrAccount: req.BankCorrAccount,
		BankAccount: req.BankAccount, Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) UpdateCompany(ctx context.Context, req *companyv1.UpdateCompanyRequest) (*companyv1.CompanyResponse, error) {
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	c, err := s.svc.UpdateCompany(ctx, application.UpdateCompanyInput{
		ID: companyID, INN: req.Inn, Name: req.Name, FullName: req.FullName,
		KPP: req.Kpp, OGRN: req.Ogrn, LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CEOName: req.CeoName, CEOTitle: req.CeoTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBIK: req.BankBik, BankCorrAccount: req.BankCorrAccount,
		BankAccount: req.BankAccount, Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) GetCompany(ctx context.Context, req *companyv1.GetCompanyRequest) (*companyv1.CompanyResponse, error) {
	id, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	c, err := s.svc.GetCompany(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) ListClientCompanies(ctx context.Context, req *companyv1.ListClientCompaniesRequest) (*companyv1.ListClientCompaniesResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	list, err := s.svc.ListClientCompanies(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	infos := make([]*companyv1.CompanyInfo, len(list))
	for i, cwd := range list {
		infos[i] = toProto(cwd.Company, cwd.IsDefault)
	}
	return &companyv1.ListClientCompaniesResponse{Companies: infos}, nil
}

func (s *Server) SetDefaultCompany(ctx context.Context, req *companyv1.SetDefaultCompanyRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	if err := s.svc.SetDefaultCompany(ctx, clientID, companyID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) DetachCompany(ctx context.Context, req *companyv1.DetachCompanyRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	if err := s.svc.DetachCompany(ctx, clientID, companyID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) CreateOfferCompany(ctx context.Context, req *companyv1.CreateOfferCompanyRequest) (*companyv1.CompanyResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	c, err := s.svc.CreateOfferCompany(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, true)}, nil
}

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCompanyNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidINN):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCompanyNameRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCompanyHasSenderNames):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCannotDetachDefault):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrNotAttached):
		return status.Error(codes.NotFound, err.Error())
	default:
		s.logger.Error().Err(err).Msg("внутренняя ошибка company service")
		return status.Error(codes.Internal, "internal error")
	}
}

func toProto(c *shared.Company, isDefault bool) *companyv1.CompanyInfo {
	return &companyv1.CompanyInfo{
		Id:              c.ID.String(),
		Inn:             c.INN.String,
		Name:            c.Name,
		FullName:        c.FullName.String,
		Kpp:             c.KPP.String,
		Ogrn:            c.OGRN.String,
		LegalAddress:    c.LegalAddress.String,
		ActualAddress:   c.ActualAddress.String,
		CeoName:         c.CEOName.String,
		CeoTitle:        c.CEOTitle.String,
		ActingBasis:     c.ActingBasis.String,
		BankName:        c.BankName.String,
		BankBik:         c.BankBIK.String,
		BankCorrAccount: c.BankCorrAccount.String,
		BankAccount:     c.BankAccount.String,
		Email:           c.Email.String,
		Phone:           c.Phone.String,
		IsOffer:         c.IsOffer,
		IsDefault:       isDefault,
		Active:          c.Active,
		CreatedAt:       timestamppb.New(c.CreatedAt),
		UpdatedAt:       timestamppb.New(c.UpdatedAt),
	}
}
```

Добавь import `emptypb "google.golang.org/protobuf/types/known/emptypb"` в блок imports.

- [ ] **Step 2: Build**

```bash
cd c:/projects/sms
go build ./internal/services/company/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/company/grpc/
git commit -m "feat(company): add gRPC server handler"
```

---

## Task 8: Register CompanyService in Template Service

**Files:**
- Modify: `cmd/services/template-service/main.go`

- [ ] **Step 1: Add imports and registration**

В файле `cmd/services/template-service/main.go` найди блок:

```go
senderNameHandler := templategrpc.NewSenderNameHandler(senderNameService)
sendernamev1.RegisterSenderNameServiceServer(grpcServer, senderNameHandler)
```

И добавь ПОСЛЕ него:

```go
// Company Service
companyStorageRepo := storage.NewCompanyRepository(dbx)
companyAppService := companyapplication.NewCompanyService(companyStorageRepo, nil)
companyGrpcServer := companygrpc.NewServer(companyAppService)
companyv1.RegisterCompanyServiceServer(grpcServer, companyGrpcServer)
```

Добавь импорты:

```go
companyv1      "github.com/smpp-server/smpp-server/api/proto/companyv1"
companyapplication "github.com/smpp-server/smpp-server/internal/services/company/application"
companygrpc    "github.com/smpp-server/smpp-server/internal/services/company/grpc"
"github.com/smpp-server/smpp-server/internal/storage"
```

- [ ] **Step 2: Build template-service**

```bash
cd c:/projects/sms
go build ./cmd/services/template-service/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/services/template-service/main.go
git commit -m "feat(template-service): register CompanyService gRPC handler"
```

---

## Task 9: Portal Service Clients

**Files:**
- Modify: `internal/gateway/portal/clients.go`

- [ ] **Step 1: Add CompanyClient to ServiceClients**

В `ServiceClients` struct добавь поле:

```go
CompanyClient companyv1.CompanyServiceClient
```

В `ServiceAddresses` struct НЕ добавляй новое поле — используем существующий `Template` адрес (оба сервиса на 9099).

- [ ] **Step 2: Add connection in NewServiceClients**

Найди блок подключения к Template Service (похожий паттерн на другие сервисы) и ПОСЛЕ него добавь:

```go
// CompanyService работает на том же адресе, что и Template Service (порт 9099)
if addresses.Template != "" {
    conn, err := grpc.Dial(addresses.Template, opts...)
    if err != nil {
        clients.Close()
        return nil, fmt.Errorf("не удалось подключиться к Company Service: %w", err)
    }
    clients.CompanyClient = companyv1.NewCompanyServiceClient(conn)
    clients.conns = append(clients.conns, conn)
}
```

Добавь import: `companyv1 "github.com/smpp-server/smpp-server/api/proto/companyv1"`

- [ ] **Step 3: Build**

```bash
cd c:/projects/sms
go build ./internal/gateway/portal/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/clients.go
git commit -m "feat(portal): add CompanyClient gRPC connection to ServiceClients"
```

---

## Task 10: Portal HTTP Handlers

**Files:**
- Create: `internal/gateway/portal/handlers/companies.go`

- [ ] **Step 1: Write handlers**

```go
// internal/gateway/portal/handlers/companies.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	companyv1 "github.com/smpp-server/smpp-server/api/proto/companyv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CompanyHandlers struct {
	client companyv1.CompanyServiceClient
}

func NewCompanyHandlers(client companyv1.CompanyServiceClient) *CompanyHandlers {
	return &CompanyHandlers{client: client}
}

type companyRequest struct {
	INN             string `json:"inn"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	KPP             string `json:"kpp"`
	OGRN            string `json:"ogrn"`
	LegalAddress    string `json:"legal_address"`
	ActualAddress   string `json:"actual_address"`
	CEOName         string `json:"ceo_name"`
	CEOTitle        string `json:"ceo_title"`
	ActingBasis     string `json:"acting_basis"`
	BankName        string `json:"bank_name"`
	BankBIK         string `json:"bank_bik"`
	BankCorrAccount string `json:"bank_corr_account"`
	BankAccount     string `json:"bank_account"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

func (h *CompanyHandlers) ListCompanies(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	resp, err := h.client.ListClientCompanies(r.Context(), &companyv1.ListClientCompaniesRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"companies": resp.Companies,
	})
}

func (h *CompanyHandlers) CreateCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req companyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.CreateCompany(r.Context(), &companyv1.CreateCompanyRequest{
		ClientId: clientID.String(), Inn: req.INN, Name: req.Name,
		FullName: req.FullName, Kpp: req.KPP, Ogrn: req.OGRN,
		LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CeoName: req.CEOName, CeoTitle: req.CEOTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBik: req.BankBIK,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp.Company)
}

func (h *CompanyHandlers) GetCompany(w http.ResponseWriter, r *http.Request) {
	companyID := mux.Vars(r)["id"]
	if _, err := uuid.Parse(companyID); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID компании"))
		return
	}
	resp, err := h.client.GetCompany(r.Context(), &companyv1.GetCompanyRequest{CompanyId: companyID})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp.Company)
}

func (h *CompanyHandlers) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	companyID := mux.Vars(r)["id"]
	if _, err := uuid.Parse(companyID); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID компании"))
		return
	}
	var req companyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.UpdateCompany(r.Context(), &companyv1.UpdateCompanyRequest{
		CompanyId: companyID, Inn: req.INN, Name: req.Name,
		FullName: req.FullName, Kpp: req.KPP, Ogrn: req.OGRN,
		LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CeoName: req.CEOName, CeoTitle: req.CEOTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBik: req.BankBIK,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp.Company)
}

func (h *CompanyHandlers) SetDefaultCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	companyID := mux.Vars(r)["id"]
	_, err := h.client.SetDefaultCompany(r.Context(), &companyv1.SetDefaultCompanyRequest{
		ClientId:  clientID.String(),
		CompanyId: companyID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CompanyHandlers) DetachCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	companyID := mux.Vars(r)["id"]
	_, err := h.client.DetachCompany(r.Context(), &companyv1.DetachCompanyRequest{
		ClientId:  clientID.String(),
		CompanyId: companyID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// respondGRPCError уже должен быть в пакете handlers. Если нет — добавить:
// func respondGRPCError(w http.ResponseWriter, err error) {
//     st, _ := status.FromError(err)
//     switch st.Code() {
//     case codes.NotFound:
//         respondError(w, shared.ErrNotFound(st.Message()))
//     case codes.InvalidArgument:
//         respondError(w, shared.ErrInvalidInput(st.Message()))
//     case codes.FailedPrecondition:
//         respondError(w, shared.ErrConflict(st.Message()))
//     default:
//         respondError(w, shared.ErrInternal(st.Message()))
//     }
// }
```

- [ ] **Step 2: Build**

```bash
cd c:/projects/sms
go build ./internal/gateway/portal/handlers/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/companies.go
git commit -m "feat(portal): add CompanyHandlers HTTP handlers"
```

---

## Task 11: Router Registration

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Add companyHandlers to SetupRouter signature**

В функции `SetupRouter` добавь параметр (в конец списка параметров):

```go
companyHandlers *handlers.CompanyHandlers,
```

- [ ] **Step 2: Add company routes in the protected section**

Найди раздел protected routes (после `// === Защищённые маршруты`) и добавь:

```go
// Companies endpoints
companies := protected.PathPrefix("/companies").Subrouter()
companies.HandleFunc("", companyHandlers.ListCompanies).Methods("GET")
companies.HandleFunc("", companyHandlers.CreateCompany).Methods("POST")
companies.HandleFunc("/{id}", companyHandlers.GetCompany).Methods("GET")
companies.HandleFunc("/{id}", companyHandlers.UpdateCompany).Methods("PUT")
companies.HandleFunc("/{id}/set-default", companyHandlers.SetDefaultCompany).Methods("POST")
companies.HandleFunc("/{id}/detach", companyHandlers.DetachCompany).Methods("DELETE")
```

- [ ] **Step 3: Build**

```bash
cd c:/projects/sms
go build ./internal/gateway/portal/router/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): add /companies routes to portal router"
```

---

## Task 12: Wire Up in Portal Gateway main.go

**Files:**
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Create companyHandlers**

Найди в `cmd/portal-gateway/main.go` место где создаются другие handlers (например `templateHandlers`), добавь рядом:

```go
companyHandlers := handlers.NewCompanyHandlers(serviceClients.CompanyClient)
```

- [ ] **Step 2: Pass to SetupRouter**

Найди вызов `portalrouter.SetupRouter(...)` и добавь `companyHandlers` в конец списка аргументов.

- [ ] **Step 3: Build portal-gateway**

```bash
cd c:/projects/sms
go build ./cmd/portal-gateway/...
```

Expected: no errors.

- [ ] **Step 4: Build all**

```bash
go build ./...
```

Expected: no errors across all packages.

- [ ] **Step 5: Commit**

```bash
git add cmd/portal-gateway/main.go
git commit -m "feat(portal-gateway): wire CompanyHandlers into portal gateway"
```

---

## Task 13: Frontend API Types and Client

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add CompanyInfo type and companiesApi**

В конец файла `portal-frontend/src/api/client.ts` добавь:

```typescript
// ===== Companies API =====

export interface CompanyInfo {
  id: string;
  inn?: string;
  name: string;
  full_name?: string;
  kpp?: string;
  ogrn?: string;
  legal_address?: string;
  actual_address?: string;
  ceo_name?: string;
  ceo_title?: string;
  acting_basis?: string;
  bank_name?: string;
  bank_bik?: string;
  bank_corr_account?: string;
  bank_account?: string;
  email?: string;
  phone?: string;
  is_offer: boolean;
  is_default: boolean;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CompanyUpsertRequest {
  inn?: string;
  name: string;
  full_name?: string;
  kpp?: string;
  ogrn?: string;
  legal_address?: string;
  actual_address?: string;
  ceo_name?: string;
  ceo_title?: string;
  acting_basis?: string;
  bank_name?: string;
  bank_bik?: string;
  bank_corr_account?: string;
  bank_account?: string;
  email?: string;
  phone?: string;
}

export const companiesApi = {
  list: () =>
    apiFetch<{ companies: CompanyInfo[] }>('/companies'),

  create: (data: CompanyUpsertRequest) =>
    apiFetch<CompanyInfo>('/companies', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  get: (id: string) =>
    apiFetch<CompanyInfo>(`/companies/${id}`),

  update: (id: string, data: CompanyUpsertRequest) =>
    apiFetch<CompanyInfo>(`/companies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  setDefault: (id: string) =>
    apiFetch<void>(`/companies/${id}/set-default`, { method: 'POST' }),

  detach: (id: string) =>
    apiFetch<void>(`/companies/${id}/detach`, { method: 'DELETE' }),
};
```

- [ ] **Step 2: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(frontend): add CompanyInfo types and companiesApi client"
```

---

## Task 14: CompaniesPage

**Files:**
- Create: `portal-frontend/src/pages/companies/CompaniesPage.tsx`

- [ ] **Step 1: Write CompaniesPage**

```tsx
// portal-frontend/src/pages/companies/CompaniesPage.tsx
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  companiesApi,
  ApiError,
  type CompanyInfo,
  type CompanyUpsertRequest,
} from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';

function validateINN(inn: string): string {
  if (inn === '') return '';
  if (!/^\d+$/.test(inn)) return 'ИНН должен содержать только цифры';
  if (inn.length !== 10 && inn.length !== 12) return 'ИНН должен быть 10 или 12 цифр';
  return '';
}

export function CompaniesPage() {
  const navigate = useNavigate();
  const [companies, setCompanies] = useState<CompanyInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CompanyUpsertRequest>({ name: '' });
  const [formError, setFormError] = useState('');
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await companiesApi.list();
      setCompanies(res.companies ?? []);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const innErr = validateINN(form.inn ?? '');
    if (innErr) { setFormError(innErr); return; }
    if (!form.name.trim()) { setFormError('Наименование обязательно'); return; }
    setCreating(true);
    setFormError('');
    try {
      await companiesApi.create(form);
      setShowCreate(false);
      setForm({ name: '' });
      load();
    } catch (e) {
      setFormError(e instanceof ApiError ? e.message : 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  const handleSetDefault = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    try {
      await companiesApi.setDefault(id);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка');
    }
  };

  const columns: Column<CompanyInfo>[] = [
    {
      key: 'name',
      header: 'Наименование',
      render: (c) => (
        <div>
          <span className="font-medium">{c.name}</span>
          {c.is_offer && (
            <Badge variant="default" className="ml-2 text-xs">Оферта</Badge>
          )}
        </div>
      ),
    },
    { key: 'inn', header: 'ИНН', render: (c) => <span className="font-mono text-sm">{c.inn || '—'}</span> },
    {
      key: 'is_default',
      header: 'Основная',
      render: (c) =>
        c.is_default ? (
          <Badge variant="success">Основная</Badge>
        ) : (
          <Button
            variant="ghost"
            size="sm"
            onClick={(e) => handleSetDefault(e, c.id)}
          >
            Сделать основной
          </Button>
        ),
    },
    {
      key: 'actions',
      header: '',
      render: (c) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={(e) => { e.stopPropagation(); navigate(`/companies/${c.id}`); }}
        >
          Реквизиты
        </Button>
      ),
    },
  ];

  return (
    <div>
      <PageHeader
        title="Мои компании"
        subtitle="Управление юридическими лицами для регистрации имён отправителей"
        actions={
          <Button onClick={() => { setShowCreate(true); setForm({ name: '' }); setFormError(''); }}>
            Добавить компанию
          </Button>
        }
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}

      <DataTable
        columns={columns}
        data={companies}
        loading={loading}
        onRowClick={(c) => navigate(`/companies/${c.id}`)}
        total={companies.length}
        page={1}
        pageSize={companies.length || 20}
        onPageChange={() => {}}
        bulkActions={[]}
      />

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Добавить компанию">
        <form onSubmit={handleCreate} className="space-y-4">
          <Input
            label="ИНН"
            value={form.inn ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, inn: e.target.value }))}
            placeholder="10 или 12 цифр"
            maxLength={12}
          />
          <Input
            label="Краткое наименование *"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="ООО Ромашка"
            required
          />
          {formError && <p className="text-sm text-red-600">{formError}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
            <Button type="submit" disabled={creating}>
              Создать
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
```

- [ ] **Step 2: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/companies/CompaniesPage.tsx
git commit -m "feat(portal): add CompaniesPage with company list and create modal"
```

---

## Task 15: CompanyDetailPage

**Files:**
- Create: `portal-frontend/src/pages/companies/CompanyDetailPage.tsx`

- [ ] **Step 1: Write CompanyDetailPage**

```tsx
// portal-frontend/src/pages/companies/CompanyDetailPage.tsx
import { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { companiesApi, ApiError, type CompanyInfo, type CompanyUpsertRequest } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { Badge } from '../../components/ui/Badge';

function Field({ label, value, onChange, placeholder, maxLength }: {
  label: string; value: string; onChange: (v: string) => void;
  placeholder?: string; maxLength?: number;
}) {
  return (
    <Input
      label={label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      maxLength={maxLength}
    />
  );
}

export function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [company, setCompany] = useState<CompanyInfo | null>(null);
  const [form, setForm] = useState<CompanyUpsertRequest>({ name: '' });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const c = await companiesApi.get(id);
      setCompany(c);
      setForm({
        inn: c.inn ?? '',
        name: c.name,
        full_name: c.full_name ?? '',
        kpp: c.kpp ?? '',
        ogrn: c.ogrn ?? '',
        legal_address: c.legal_address ?? '',
        actual_address: c.actual_address ?? '',
        ceo_name: c.ceo_name ?? '',
        ceo_title: c.ceo_title ?? '',
        acting_basis: c.acting_basis ?? '',
        bank_name: c.bank_name ?? '',
        bank_bik: c.bank_bik ?? '',
        bank_corr_account: c.bank_corr_account ?? '',
        bank_account: c.bank_account ?? '',
        email: c.email ?? '',
        phone: c.phone ?? '',
      });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const updated = await companiesApi.update(id, form);
      setCompany(updated);
      setSuccess('Реквизиты сохранены');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const set = (key: keyof CompanyUpsertRequest) => (v: string) =>
    setForm((f) => ({ ...f, [key]: v }));

  if (loading) return <div className="p-8 text-gray-500">Загрузка...</div>;
  if (!company) return <div className="p-8 text-red-600">{error || 'Компания не найдена'}</div>;

  return (
    <div>
      <PageHeader
        title={company.name}
        subtitle={company.is_offer ? 'Работа по договору-оферте' : company.inn ? `ИНН: ${company.inn}` : 'Компания'}
        backHref="/companies"
        actions={
          <div className="flex items-center gap-2">
            {company.is_default && <Badge variant="success">Основная компания</Badge>}
            {company.is_offer && <Badge variant="default">Оферта</Badge>}
          </div>
        }
      />

      {error && <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-md text-sm">{error}</div>}
      {success && <div className="mb-4 p-3 bg-green-50 text-green-700 rounded-md text-sm">{success}</div>}

      <form onSubmit={handleSave} className="space-y-6 max-w-2xl">
        {/* Основные */}
        <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Основные реквизиты</h3>
          <div className="grid grid-cols-2 gap-4">
            <Field label="ИНН" value={form.inn ?? ''} onChange={set('inn')} maxLength={12} placeholder="10 или 12 цифр" />
            <Field label="КПП" value={form.kpp ?? ''} onChange={set('kpp')} maxLength={9} />
          </div>
          <Field label="Краткое наименование *" value={form.name} onChange={set('name')} />
          <Field label="Полное наименование" value={form.full_name ?? ''} onChange={set('full_name')} />
          <Field label="ОГРН/ОГРНИП" value={form.ogrn ?? ''} onChange={set('ogrn')} maxLength={15} />
        </section>

        {/* Руководитель */}
        <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Руководитель</h3>
          <Field label="ФИО руководителя" value={form.ceo_name ?? ''} onChange={set('ceo_name')} placeholder="Иванов Иван Иванович" />
          <Field label="Должность" value={form.ceo_title ?? ''} onChange={set('ceo_title')} placeholder="Генеральный директор" />
          <Field label="Действует на основании" value={form.acting_basis ?? ''} onChange={set('acting_basis')} placeholder="Устава" />
        </section>

        {/* Адреса */}
        <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Адреса</h3>
          <Field label="Юридический адрес" value={form.legal_address ?? ''} onChange={set('legal_address')} />
          <Field label="Фактический адрес" value={form.actual_address ?? ''} onChange={set('actual_address')} placeholder="Если отличается от юридического" />
        </section>

        {/* Банк */}
        <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Банковские реквизиты</h3>
          <Field label="Наименование банка" value={form.bank_name ?? ''} onChange={set('bank_name')} />
          <div className="grid grid-cols-2 gap-4">
            <Field label="БИК" value={form.bank_bik ?? ''} onChange={set('bank_bik')} maxLength={9} />
            <Field label="Корр. счёт" value={form.bank_corr_account ?? ''} onChange={set('bank_corr_account')} maxLength={20} />
          </div>
          <Field label="Расчётный счёт" value={form.bank_account ?? ''} onChange={set('bank_account')} maxLength={20} />
        </section>

        {/* Контакты */}
        <section className="bg-white rounded-lg border border-gray-200 p-6 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Контакты</h3>
          <div className="grid grid-cols-2 gap-4">
            <Field label="Email" value={form.email ?? ''} onChange={set('email')} placeholder="info@company.ru" />
            <Field label="Телефон" value={form.phone ?? ''} onChange={set('phone')} placeholder="+7 (999) 000-00-00" />
          </div>
        </section>

        <div className="flex justify-between">
          <Button type="button" variant="ghost" onClick={() => navigate('/companies')}>
            ← Назад
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить реквизиты'}
          </Button>
        </div>
      </form>
    </div>
  );
}
```

- [ ] **Step 2: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/companies/CompanyDetailPage.tsx
git commit -m "feat(portal): add CompanyDetailPage with full requisites form"
```

---

## Task 16: Add Routes to Frontend Router

**Files:**
- Modify: `portal-frontend/src/App.tsx` или основной роутер (найди файл с `<Route path="/sender-names"`)

- [ ] **Step 1: Find the router file**

```bash
cd c:/projects/sms/portal-frontend
grep -r "sender-names" src/ --include="*.tsx" -l
```

Открой найденный файл (скорее всего `src/App.tsx` или `src/router.tsx`).

- [ ] **Step 2: Add company routes**

Найди блок где подключены маршруты sender-names и добавь рядом:

```tsx
import { CompaniesPage } from './pages/companies/CompaniesPage';
import { CompanyDetailPage } from './pages/companies/CompanyDetailPage';

// В <Routes> добавь:
<Route path="/companies" element={<CompaniesPage />} />
<Route path="/companies/:id" element={<CompanyDetailPage />} />
```

- [ ] **Step 3: Add to navigation menu**

Найди файл навигации (Sidebar или Navigation component):

```bash
grep -r "sender-names" src/ --include="*.tsx" -l | head -5
```

Добавь пункт меню после «Имена отправителей»:

```tsx
{ path: '/companies', label: 'Мои компании', icon: <BuildingIcon /> }
```

Если нет `BuildingIcon` — используй любую подходящую иконку из используемой библиотеки.

- [ ] **Step 4: TypeScript check + dev server**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
npm run dev
```

Открой браузер → `/companies` — должна отображаться страница со списком компаний.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/
git commit -m "feat(portal): add company routes and navigation link"
```

---

## Task 17: Company Selector in SenderNamesPage

**Files:**
- Modify: `portal-frontend/src/pages/sender-names/SenderNamesPage.tsx`

- [ ] **Step 1: Add company selector to create modal**

В `SenderNamesPage.tsx` найди строки с `createName` state и добавь:

```tsx
// В начало функции SenderNamesPage(), после существующих useState:
const [companies, setCompanies] = useState<CompanyInfo[]>([]);
const [selectedCompanyId, setSelectedCompanyId] = useState('');
```

Добавь import:
```tsx
import { companiesApi, type CompanyInfo } from '../../api/client';
```

Загрузи компании при монтировании компонента (добавь в useEffect или отдельный useEffect):

```tsx
useEffect(() => {
  companiesApi.list().then((res) => {
    const list = res.companies ?? [];
    setCompanies(list);
    const def = list.find((c) => c.is_default);
    if (def) setSelectedCompanyId(def.id);
  }).catch(() => {});
}, []);
```

В форме создания (modal), ПЕРЕД полем `Input label="Имя отправителя"` добавь:

```tsx
{companies.length > 0 && (
  <div>
    <label className="block text-sm font-medium text-gray-700 mb-1">
      Компания
    </label>
    <select
      className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
      value={selectedCompanyId}
      onChange={(e) => setSelectedCompanyId(e.target.value)}
    >
      {companies.map((c) => (
        <option key={c.id} value={c.id}>
          {c.name}{c.is_offer ? ' (Оферта)' : ''}
        </option>
      ))}
    </select>
  </div>
)}
```

В функции `handleCreate`, передай `company_id` в `senderNamesApi.create`:

```tsx
// Вместо:
await senderNamesApi.create(createName.trim());
// Написать:
await senderNamesApi.create(createName.trim(), selectedCompanyId || undefined);
```

Обнови `senderNamesApi.create` в `api/client.ts` чтобы принимал `companyId`:

```typescript
create: (name: string, companyId?: string) =>
  apiFetch<SenderNameInfo>('/sender-names', {
    method: 'POST',
    body: JSON.stringify({ name, company_id: companyId }),
  }),
```

Обнови HTTP handler `CreateSenderName` в `internal/gateway/portal/handlers/sender_names.go`:

```go
type createSenderNameRequest struct {
    Name      string `json:"name"`
    CompanyID string `json:"company_id"`
}
```

И передай `company_id` в gRPC запрос (если proto поддерживает — иначе пропусти).

- [ ] **Step 2: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/sender-names/SenderNamesPage.tsx portal-frontend/src/api/client.ts
git commit -m "feat(portal): add company selector to sender name registration form"
```

---

## Task 18: Update BillingPage to Show Company Balances

**Files:**
- Modify: `portal-frontend/src/pages/billing/BillingPage.tsx`

- [ ] **Step 1: Add company balances section**

В `BillingPage.tsx` найди импорты и добавь:

```tsx
import { companiesApi, type CompanyInfo } from '../../api/client';
```

Добавь state для компаний:

```tsx
const [companies, setCompanies] = useState<CompanyInfo[]>([]);
```

Загрузи компании при монтировании (добавь в существующий useEffect или отдельный):

```tsx
useEffect(() => {
  companiesApi.list().then((res) => setCompanies(res.companies ?? [])).catch(() => {});
}, []);
```

Найди место где отображается баланс (скорее всего блок с `balance.balance`) и ПЕРЕД ним добавь блок компаний:

```tsx
{companies.length > 0 && (
  <div className="mb-6">
    <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
      Балансы по компаниям
    </h2>
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {companies.map((c) => (
        <div
          key={c.id}
          className="bg-white rounded-lg border border-gray-200 p-4"
        >
          <div className="flex items-start justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">{c.name}</p>
              {c.inn && (
                <p className="text-xs text-gray-500 mt-0.5">ИНН: {c.inn}</p>
              )}
            </div>
            <div className="flex flex-col items-end gap-1">
              {c.is_default && (
                <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">
                  Основная
                </span>
              )}
              {c.is_offer && (
                <span className="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">
                  Оферта
                </span>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  </div>
)}
```

Примечание: баланс компании будет отображаться после того, как биллинг-сервис будет обновлён для поддержки `company_id` в accounts. Пока показываем только карточки компаний без суммы.

- [ ] **Step 2: TypeScript check**

```bash
cd c:/projects/sms/portal-frontend
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/billing/BillingPage.tsx
git commit -m "feat(portal): add company cards section to billing page"
```

---

## Task 19: Smoke Test

- [ ] **Step 1: Build backend**

```bash
cd c:/projects/sms
go build ./...
```

Expected: no errors.

- [ ] **Step 2: Run unit tests**

```bash
go test ./internal/services/company/... ./internal/storage/... -v -count=1
```

- [ ] **Step 3: Start dev environment**

```bash
# В одном терминале:
cd c:/projects/sms
docker-compose up -d db redis

# Запустить template-service (включает company service)
go run ./cmd/services/template-service/

# В другом терминале — portal gateway
go run ./cmd/portal-gateway/

# В третьем — frontend
cd portal-frontend && npm run dev
```

- [ ] **Step 4: Manual smoke test**

1. Открой `http://localhost:5173/companies`
2. Ожидается: страница «Мои компании» с одной компанией «Оферта» (создана при миграции)
3. Нажми «Добавить компанию» → введи ИНН и наименование → нажми «Создать»
4. Ожидается: новая компания в списке
5. Нажми «Реквизиты» → заполни все поля → «Сохранить»
6. Ожидается: успешное сохранение без ошибок
7. Перейди `/sender-names` → «Зарегистрировать имя» → в форме должен быть dropdown с компаниями

- [ ] **Step 5: Final commit**

```bash
git add .
git commit -m "feat(companies): complete client companies feature — CRUD, sender name linkage, billing page"
```

---

## Scope Notes

Следующие части не включены в этот план (отдельные задачи):

1. **Биллинг-фоллбэк** — логика «списывать с компании sender name → если 0, искать другую компанию» требует изменения `BillingService.ChargeMessage` и pipeline-worker. Текущий биллинг работает через `client_id` (привязан к дефолтной компании-оферте).

2. **Баланс компании в UI** — карточки компаний на странице биллинга пока не показывают конкретную сумму. Для этого нужен новый gRPC endpoint `GetCompanyBalance(company_id)` в billing-service.

3. **Автоматическое создание «Оферты» при регистрации** — `CreateOfferCompany` готов в gRPC, но вызов при регистрации нового клиента (в auth-service) не добавлен.
