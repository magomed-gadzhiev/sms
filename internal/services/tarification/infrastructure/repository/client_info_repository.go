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

	err := r.db.QueryRowContext(ctx,
		`SELECT id, account_type, parent_client_id FROM clients WHERE id = $1`,
		clientID,
	).Scan(&info.ID, &info.AccountType, &parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("client info lookup: %w", err)
	}
	info.ParentClientID = parentID

	return &info, nil
}
