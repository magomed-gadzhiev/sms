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

// CompanyRepository предоставляет методы для работы с компаниями
type CompanyRepository struct {
	db *sqlx.DB
}

// NewCompanyRepository создаёт новый репозиторий компаний
func NewCompanyRepository(db *sqlx.DB) *CompanyRepository {
	return &CompanyRepository{db: db}
}

// Create создаёт новую компанию
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

// GetByID возвращает компанию по ID
func (r *CompanyRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	var c shared.Company
	err := r.db.GetContext(ctx, &c, `SELECT * FROM companies WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCompanyNotFound
	}
	return &c, err
}

// Update обновляет данные компании
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

// ListByClientID возвращает все компании клиента и флаги is_default
func (r *CompanyRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error) {
	type row struct {
		shared.Company
		IsDefaultCol bool `db:"is_default"`
	}
	const q = `
		SELECT c.*, cc.is_default
		FROM companies c
		JOIN client_companies cc ON cc.company_id = c.id
		WHERE cc.client_id = $1
		ORDER BY cc.is_default DESC, cc.created_at ASC`
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, clientID); err != nil {
		return nil, nil, err
	}
	companies := make([]*shared.Company, len(rows))
	defaults := make([]bool, len(rows))
	for i, rr := range rows {
		c := rr.Company
		companies[i] = &c
		defaults[i] = rr.IsDefaultCol
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
	var count int
	if err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM sender_names
		WHERE client_id = $1 AND company_id = $2`, clientID, companyID); err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrCompanyHasSenderNames
	}

	var isDefault bool
	err := r.db.GetContext(ctx, &isDefault, `
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

	if _, err = tx.ExecContext(ctx, `
		UPDATE client_companies SET is_default = FALSE
		WHERE client_id = $1`, clientID); err != nil {
		return err
	}

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
