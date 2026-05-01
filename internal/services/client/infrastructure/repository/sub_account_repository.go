package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

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

// ListByParentID получает список суб-аккаунтов по ID родительского клиента.
// LEFT JOIN с client_configs нужен, чтобы заполнить domain.Client.Config —
// иначе grpc-маппинг в SubAccount даёт DailyLimit=0, MonthlyLimit=0 даже если
// в БД лимиты выставлены (rate_limit_per_day и settings.monthly_limit).
func (r *SubAccountRepository) ListByParentID(ctx context.Context, parentID uuid.UUID) ([]*domain.Client, error) {
	var clients []*domain.Client
	query := `
		SELECT c.id, c.name, COALESCE(c.email, ''), COALESCE(c.contact_person, ''), COALESCE(c.phone, ''), c.active, c.metadata,
		       c.parent_client_id, c.is_reseller, c.max_sub_accounts, c.created_at, c.updated_at,
		       cfg.rate_limit_per_day, cfg.settings
		FROM clients c
		LEFT JOIN client_configs cfg ON cfg.client_id = c.id
		WHERE c.parent_client_id = $1 AND c.active = true
		ORDER BY c.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var client domain.Client
		var rateLimitPerDay sql.NullInt64
		var settingsRaw sql.NullString
		err := rows.Scan(
			&client.ID, &client.Name, &client.Email, &client.ContactPerson, &client.Phone,
			&client.Active, &client.Metadata,
			&client.ParentClientID, &client.IsReseller, &client.MaxSubAccounts,
			&client.CreatedAt, &client.UpdatedAt,
			&rateLimitPerDay, &settingsRaw,
		)
		if err != nil {
			return nil, err
		}
		if rateLimitPerDay.Valid || settingsRaw.Valid {
			client.Config = &domain.ClientConfig{
				ClientID:        client.ID,
				RateLimitPerDay: int(rateLimitPerDay.Int64),
				Settings:        json.RawMessage("{}"),
			}
			if settingsRaw.Valid && settingsRaw.String != "" {
				client.Config.Settings = json.RawMessage(settingsRaw.String)
			}
		}
		clients = append(clients, &client)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return clients, nil
}

// ExistsByEmailUnderParent проверяет, есть ли уже суб-аккаунт с таким
// email у того же родителя — включая soft-deleted. UNIQUE constraint на
// clients.email отсутствует (миграция требует эскалации), поэтому уникальность
// проверяется на application-уровне. Race condition при двух параллельных POST
// остаётся открытым — фиксируется в observation.
//
// Filter active=true НЕ ставим: иначе после delete+recreate с тем же email в
// БД остаются 2 строки (одна inactive, одна active), и downstream login-by-email
// получает ambiguity.
func (r *SubAccountRepository) ExistsByEmailUnderParent(ctx context.Context, parentID uuid.UUID, email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	var count int
	query := `SELECT COUNT(*) FROM clients
	          WHERE parent_client_id = $1 AND lower(email) = lower($2)`
	if err := r.db.QueryRowContext(ctx, query, parentID, email).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
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

// GetSubAccount получает суб-аккаунт с проверкой принадлежности к родителю.
// LEFT JOIN с client_configs нужен, чтобы заполнить Config — см. комментарий
// в ListByParentID.
func (r *SubAccountRepository) GetSubAccount(ctx context.Context, subAccountID, parentID uuid.UUID) (*domain.Client, error) {
	var client domain.Client
	var rateLimitPerDay sql.NullInt64
	var settingsRaw sql.NullString
	query := `
		SELECT c.id, c.name, COALESCE(c.email, ''), COALESCE(c.contact_person, ''), COALESCE(c.phone, ''), c.active, c.metadata,
		       c.parent_client_id, c.is_reseller, c.max_sub_accounts, c.created_at, c.updated_at,
		       cfg.rate_limit_per_day, cfg.settings
		FROM clients c
		LEFT JOIN client_configs cfg ON cfg.client_id = c.id
		WHERE c.id = $1 AND c.parent_client_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, subAccountID, parentID).Scan(
		&client.ID, &client.Name, &client.Email, &client.ContactPerson, &client.Phone,
		&client.Active, &client.Metadata,
		&client.ParentClientID, &client.IsReseller, &client.MaxSubAccounts,
		&client.CreatedAt, &client.UpdatedAt,
		&rateLimitPerDay, &settingsRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	if rateLimitPerDay.Valid || settingsRaw.Valid {
		client.Config = &domain.ClientConfig{
			ClientID:        client.ID,
			RateLimitPerDay: int(rateLimitPerDay.Int64),
			Settings:        json.RawMessage("{}"),
		}
		if settingsRaw.Valid && settingsRaw.String != "" {
			client.Config.Settings = json.RawMessage(settingsRaw.String)
		}
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
