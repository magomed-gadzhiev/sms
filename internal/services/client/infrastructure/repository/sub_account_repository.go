package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// SubAccountRepository предоставляет методы для работы с суб-аккаунтами
type SubAccountRepository struct {
	db *sqlx.DB
}

// NewSubAccountRepository создает новый репозиторий суб-аккаунтов
func NewSubAccountRepository(db *database.DB) *SubAccountRepository {
	return &SubAccountRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// ListByParentID получает список суб-аккаунтов по ID родительского клиента
func (r *SubAccountRepository) ListByParentID(ctx context.Context, parentID uuid.UUID) ([]*domain.Client, error) {
	var clients []*domain.Client
	query := `
		SELECT id, name, COALESCE(email, ''), COALESCE(contact_person, ''), COALESCE(phone, ''), active, metadata,
		       parent_client_id, is_reseller, max_sub_accounts, created_at, updated_at
		FROM clients
		WHERE parent_client_id = $1 AND active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var client domain.Client
		err := rows.Scan(
			&client.ID, &client.Name, &client.Email, &client.ContactPerson, &client.Phone,
			&client.Active, &client.Metadata,
			&client.ParentClientID, &client.IsReseller, &client.MaxSubAccounts,
			&client.CreatedAt, &client.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		clients = append(clients, &client)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return clients, nil
}

// CountByParentID считает количество суб-аккаунтов для родительского клиента
func (r *SubAccountRepository) CountByParentID(ctx context.Context, parentID uuid.UUID) (int, error) {
	var count int
	// Не считаем soft-deleted — иначе max_sub_accounts лимит исчерпывается
	// удалёнными записями и пользователь не может создать новый субакк.
	query := `SELECT COUNT(*) FROM clients WHERE parent_client_id = $1 AND active = true`

	err := r.db.QueryRowContext(ctx, query, parentID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetSubAccount получает суб-аккаунт с проверкой принадлежности к родителю
func (r *SubAccountRepository) GetSubAccount(ctx context.Context, subAccountID, parentID uuid.UUID) (*domain.Client, error) {
	var client domain.Client
	query := `
		SELECT id, name, COALESCE(email, ''), COALESCE(contact_person, ''), COALESCE(phone, ''), active, metadata,
		       parent_client_id, is_reseller, max_sub_accounts, created_at, updated_at
		FROM clients
		WHERE id = $1 AND parent_client_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, subAccountID, parentID).Scan(
		&client.ID, &client.Name, &client.Email, &client.ContactPerson, &client.Phone,
		&client.Active, &client.Metadata,
		&client.ParentClientID, &client.IsReseller, &client.MaxSubAccounts,
		&client.CreatedAt, &client.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrClientNotFound
		}
		return nil, err
	}

	return &client, nil
}

// DeleteSubAccount удаляет суб-аккаунт (мягкое удаление через active = false)
func (r *SubAccountRepository) DeleteSubAccount(ctx context.Context, subAccountID uuid.UUID) error {
	query := `
		UPDATE clients SET active = false, updated_at = NOW()
		WHERE id = $1 AND parent_client_id IS NOT NULL
	`

	result, err := r.db.ExecContext(ctx, query, subAccountID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrClientNotFound
	}

	log.Info().Str("sub_account_id", subAccountID.String()).Msg("суб-аккаунт удален")
	return nil
}
