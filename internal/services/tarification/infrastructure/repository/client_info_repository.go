package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ClientInfoRepository получает базовую информацию о клиенте
type ClientInfoRepository struct {
	db *sqlx.DB
}

// NewClientInfoRepository создает новый репозиторий
func NewClientInfoRepository(db *sqlx.DB) *ClientInfoRepository {
	return &ClientInfoRepository{db: db}
}

// GetAccountInfo возвращает тип аккаунта и parent_client_id
func (r *ClientInfoRepository) GetAccountInfo(ctx context.Context, clientID uuid.UUID) (*domain.ClientAccountInfo, error) {
	var info domain.ClientAccountInfo
	var parentID *uuid.UUID
	var billingMode string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, account_type, parent_client_id, billing_mode, spending_limit_monthly, spending_limit_daily FROM clients WHERE id = $1`,
		clientID,
	).Scan(&info.ID, &info.AccountType, &parentID, &billingMode, &info.SpendingLimitMonthly, &info.SpendingLimitDaily)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("client info lookup: %w", err)
	}
	info.ParentClientID = parentID
	info.BillingMode = domain.BillingMode(billingMode)

	return &info, nil
}
