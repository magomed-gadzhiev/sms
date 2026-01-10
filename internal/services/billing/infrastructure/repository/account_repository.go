package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
)

// AccountRepository реализует domain.AccountRepository
type AccountRepository struct {
	db *sqlx.DB
}

// NewAccountRepository создает новый репозиторий счетов
func NewAccountRepository(db *sqlx.DB) *AccountRepository {
	return &AccountRepository{
		db: db,
	}
}

// Create создает новый счет
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (
			id, client_id, balance, currency, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		account.ID,
		account.ClientID,
		account.Balance,
		account.Currency,
		account.CreatedAt,
		account.UpdatedAt,
	)

	return err
}

// GetByClientID получает счет по client_id
func (r *AccountRepository) GetByClientID(ctx context.Context, clientID uuid.UUID) (*domain.Account, error) {
	var account domain.Account
	query := `
		SELECT id, client_id, balance, currency, created_at, updated_at
		FROM accounts
		WHERE client_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&account.ID,
		&account.ClientID,
		&account.Balance,
		&account.Currency,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil
}

// Update обновляет счет
func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts
		SET balance = $1, currency = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query,
		account.Balance,
		account.Currency,
		account.UpdatedAt,
		account.ID,
	)

	return err
}

// UpdateBalance обновляет баланс счета
func (r *AccountRepository) UpdateBalance(ctx context.Context, clientID uuid.UUID, newBalance string) error {
	query := `
		UPDATE accounts
		SET balance = $1, updated_at = NOW()
		WHERE client_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, newBalance, clientID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrAccountNotFound
	}

	return nil
}
