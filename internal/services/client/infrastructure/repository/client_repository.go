package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var (
	ErrClientNotFound = errors.New("client not found")
)

// ClientRepository предоставляет методы для работы с клиентами
type ClientRepository struct {
	db *sqlx.DB
}

// NewClientRepository создает новый репозиторий клиентов
func NewClientRepository(db *database.DB) *ClientRepository {
	return &ClientRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает нового клиента
func (r *ClientRepository) Create(ctx context.Context, client *domain.Client) error {
	query := `
		INSERT INTO clients (
			id, name, email, contact_person, phone, active, metadata,
			parent_client_id, is_reseller, max_sub_accounts, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.Email, client.ContactPerson, client.Phone,
		client.Active, client.Metadata,
		client.ParentClientID, client.IsReseller, client.MaxSubAccounts,
		client.CreatedAt, client.UpdatedAt,
	)

	if err != nil {
		log.Error().Err(err).Msg("ошибка создания клиента")
		return err
	}

	return nil
}

// GetByID получает клиента по ID
func (r *ClientRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Client, error) {
	var client domain.Client
	query := `
		SELECT id, name, email, contact_person, phone, active, metadata,
		       parent_client_id, is_reseller, max_sub_accounts, created_at, updated_at
		FROM clients WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
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

// Update обновляет клиента
func (r *ClientRepository) Update(ctx context.Context, client *domain.Client) error {
	query := `
		UPDATE clients SET
			name = $2, email = $3, contact_person = $4, phone = $5,
			active = $6, metadata = $7,
			parent_client_id = $8, is_reseller = $9, max_sub_accounts = $10,
			updated_at = $11
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.Email, client.ContactPerson, client.Phone,
		client.Active, client.Metadata,
		client.ParentClientID, client.IsReseller, client.MaxSubAccounts,
		client.UpdatedAt,
	)

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

	return nil
}

// Delete удаляет клиента (мягкое удаление через active = false)
func (r *ClientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE clients SET active = false, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
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

	return nil
}

// List получает список клиентов
func (r *ClientRepository) List(ctx context.Context, activeOnly bool, search string, limit, offset int) ([]*domain.Client, int, error) {
	var clients []*domain.Client
	var count int
	var err error

	// Базовый запрос для получения клиентов
	baseQuery := `
		SELECT id, name, email, contact_person, phone, active, metadata,
		       parent_client_id, is_reseller, max_sub_accounts, created_at, updated_at
		FROM clients
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM clients WHERE 1=1`

	args := []interface{}{}
	argPos := 1

	// Добавляем фильтры
	if activeOnly {
		baseQuery += " AND active = true"
		countQuery += " AND active = true"
	}

	if search != "" {
		searchPattern := "%" + search + "%"
		baseQuery += " AND (name ILIKE $" + string(rune('0'+argPos)) + " OR email ILIKE $" + string(rune('0'+argPos)) + ")"
		countQuery += " AND (name ILIKE $" + string(rune('0'+argPos)) + " OR email ILIKE $" + string(rune('0'+argPos)) + ")"
		args = append(args, searchPattern)
		argPos++
	}

	// Получаем количество (используем те же аргументы, что и для основного запроса, без limit/offset)
	err = r.db.GetContext(ctx, &count, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// Добавляем пагинацию
	baseQuery += " ORDER BY created_at DESC LIMIT $" + string(rune('0'+argPos)) + " OFFSET $" + string(rune('0'+argPos+1))
	args = append(args, limit, offset)

	// Получаем список
	err = r.db.SelectContext(ctx, &clients, baseQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return clients, count, nil
}
