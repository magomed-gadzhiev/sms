package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
		SELECT id, client_id, balance, currency, frozen, frozen_at, frozen_by,
			credit_limit, low_balance_threshold, created_at, updated_at
		FROM accounts
		WHERE client_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&account.ID,
		&account.ClientID,
		&account.Balance,
		&account.Currency,
		&account.Frozen,
		&account.FrozenAt,
		&account.FrozenBy,
		&account.CreditLimit,
		&account.LowBalanceThreshold,
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

// FreezeAccount замораживает счет клиента
func (r *AccountRepository) FreezeAccount(ctx context.Context, clientID uuid.UUID, adminID uuid.UUID) (time.Time, error) {
	query := `
		UPDATE accounts
		SET frozen = true, frozen_at = NOW(), frozen_by = $1, updated_at = NOW()
		WHERE client_id = $2
		RETURNING frozen_at
	`

	var frozenAt time.Time
	err := r.db.QueryRowContext(ctx, query, adminID, clientID).Scan(&frozenAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, domain.ErrAccountNotFound
		}
		return time.Time{}, err
	}

	return frozenAt, nil
}

// UnfreezeAccount размораживает счет клиента
func (r *AccountRepository) UnfreezeAccount(ctx context.Context, clientID uuid.UUID) error {
	query := `
		UPDATE accounts
		SET frozen = false, frozen_at = NULL, frozen_by = NULL, updated_at = NOW()
		WHERE client_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, clientID)
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

// SetCreditLimit устанавливает кредитный лимит для клиента
func (r *AccountRepository) SetCreditLimit(ctx context.Context, clientID uuid.UUID, limit string) error {
	query := `
		UPDATE accounts
		SET credit_limit = $1, updated_at = NOW()
		WHERE client_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, limit, clientID)
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

// SetLowBalanceThreshold устанавливает порог низкого баланса
func (r *AccountRepository) SetLowBalanceThreshold(ctx context.Context, clientID uuid.UUID, threshold string) error {
	query := `
		UPDATE accounts
		SET low_balance_threshold = $1, updated_at = NOW()
		WHERE client_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, threshold, clientID)
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

// ListBalances получает список балансов с фильтрацией
func (r *AccountRepository) ListBalances(ctx context.Context, search string, status string, belowThreshold bool, limit, offset int32) ([]domain.BalanceInfo, int32, error) {
	baseQuery := `
		SELECT a.client_id, c.name AS client_name, a.balance, a.currency, a.frozen,
			a.credit_limit, a.low_balance_threshold, a.frozen_at, a.frozen_by, a.updated_at
		FROM accounts a
		JOIN clients c ON c.id = a.client_id
	`
	countQuery := `
		SELECT COUNT(*)
		FROM accounts a
		JOIN clients c ON c.id = a.client_id
	`

	var conditions []string
	var args []interface{}
	paramIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(c.client_name ILIKE $%d OR a.client_id::text ILIKE $%d)", paramIdx, paramIdx))
		args = append(args, "%"+search+"%")
		paramIdx++
	}

	if status == "frozen" {
		conditions = append(conditions, "a.frozen = true")
	} else if status == "active" {
		conditions = append(conditions, "a.frozen = false")
	}

	if belowThreshold {
		conditions = append(conditions, "a.low_balance_threshold != '' AND a.balance::numeric < a.low_balance_threshold::numeric")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				whereClause += " AND "
			}
			whereClause += cond
		}
	}

	// Получаем общее количество
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Получаем данные с пагинацией
	dataQuery := baseQuery + whereClause + fmt.Sprintf(" ORDER BY a.updated_at DESC LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var balances []domain.BalanceInfo
	for rows.Next() {
		var info domain.BalanceInfo
		err := rows.Scan(
			&info.ClientID,
			&info.ClientName,
			&info.Balance,
			&info.Currency,
			&info.Frozen,
			&info.CreditLimit,
			&info.LowBalanceThreshold,
			&info.FrozenAt,
			&info.FrozenBy,
			&info.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		balances = append(balances, info)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return balances, total, nil
}
