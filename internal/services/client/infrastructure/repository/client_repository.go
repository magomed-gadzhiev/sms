package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
			id, name, api_key, secret, email, contact_person, phone, active, metadata,
			parent_client_id, is_reseller, max_sub_accounts, is_sandbox, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.APIKey, client.Secret,
		client.Email, client.ContactPerson, client.Phone,
		client.Active, string(client.Metadata),
		client.ParentClientID, client.IsReseller, client.MaxSubAccounts,
		client.IsSandbox,
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
	query := `SELECT c.id, c.name, COALESCE(c.email, '') AS email,
		COALESCE(c.contact_person, '') AS contact_person, COALESCE(c.phone, '') AS phone, c.active,
		c.metadata, c.created_at, c.updated_at, c.parent_client_id, c.is_reseller,
		c.max_sub_accounts, c.plan_id, c.monthly_sms_count, c.monthly_sms_reset_at,
		c.is_sandbox,
		p.id, p.name, p.display_name, p.monthly_price_rub, p.max_sms_per_month,
		p.max_smpp_connections, p.max_users, p.rate_limit_per_second, p.rate_limit_per_minute,
		p.rate_limit_per_hour, p.rate_limit_per_day, p.features, p.active
		FROM clients c
		LEFT JOIN subscription_plans p ON c.plan_id = p.id
		WHERE c.id = $1`

	client := &domain.Client{}
	var planID, planName, planDisplayName sql.NullString
	var planMonthlyPrice sql.NullFloat64
	var planMaxSMS, planMaxSMPP, planMaxUsers sql.NullInt64
	var planRLSec, planRLMin, planRLHour, planRLDay sql.NullInt64
	var featuresJSON []byte
	var planActive sql.NullBool
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&client.ID, &client.Name, &client.Email, &client.ContactPerson,
		&client.Phone, &client.Active, &client.Metadata, &client.CreatedAt,
		&client.UpdatedAt, &client.ParentClientID, &client.IsReseller,
		&client.MaxSubAccounts, &client.PlanID, &client.MonthlySMSCount,
		&client.MonthlySMSResetAt, &client.IsSandbox,
		&planID, &planName, &planDisplayName, &planMonthlyPrice,
		&planMaxSMS, &planMaxSMPP, &planMaxUsers,
		&planRLSec, &planRLMin,
		&planRLHour, &planRLDay,
		&featuresJSON, &planActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrClientNotFound
		}
		return nil, fmt.Errorf("get client by id: %w", err)
	}
	if planID.Valid {
		plan := &domain.Plan{}
		plan.ID, _ = uuid.Parse(planID.String)
		plan.Name = planName.String
		plan.DisplayName = planDisplayName.String
		plan.MonthlyPriceRub = planMonthlyPrice.Float64
		plan.MaxSMSPerMonth = int(planMaxSMS.Int64)
		plan.MaxSMPPConnections = int(planMaxSMPP.Int64)
		plan.MaxUsers = int(planMaxUsers.Int64)
		plan.RateLimitPerSecond = int(planRLSec.Int64)
		plan.RateLimitPerMinute = int(planRLMin.Int64)
		plan.RateLimitPerHour = int(planRLHour.Int64)
		plan.RateLimitPerDay = int(planRLDay.Int64)
		plan.Active = planActive.Bool
		if featuresJSON != nil {
			_ = json.Unmarshal(featuresJSON, &plan.Features)
		}
		client.Plan = plan
	}
	return client, nil
}

// Update обновляет клиента
func (r *ClientRepository) Update(ctx context.Context, client *domain.Client) error {
	query := `
		UPDATE clients SET
			name = $2, email = $3, contact_person = $4, phone = $5,
			active = $6, metadata = $7,
			parent_client_id = $8, is_reseller = $9, max_sub_accounts = $10,
			is_sandbox = $11, updated_at = $12
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		client.ID, client.Name, client.Email, client.ContactPerson, client.Phone,
		client.Active, string(client.Metadata),
		client.ParentClientID, client.IsReseller, client.MaxSubAccounts,
		client.IsSandbox, client.UpdatedAt,
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

// IncrementMonthlySMSCount увеличивает счётчик SMS за месяц (с авто-сбросом)
func (r *ClientRepository) IncrementMonthlySMSCount(ctx context.Context, clientID uuid.UUID, count int) error {
	query := `UPDATE clients
		SET monthly_sms_count = CASE
			WHEN monthly_sms_reset_at <= NOW() THEN $2
			ELSE monthly_sms_count + $2
		END,
		monthly_sms_reset_at = CASE
			WHEN monthly_sms_reset_at <= NOW() THEN date_trunc('month', NOW()) + INTERVAL '1 month'
			ELSE monthly_sms_reset_at
		END,
		updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, clientID, count)
	if err != nil {
		return fmt.Errorf("increment monthly sms count: %w", err)
	}
	return nil
}

// AssignPlan назначает тарифный план клиенту
func (r *ClientRepository) AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error {
	query := `UPDATE clients SET plan_id = $2, updated_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, clientID, planID)
	if err != nil {
		return fmt.Errorf("assign plan: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("assign plan rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", clientID)
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
		SELECT id, name, COALESCE(email, '') AS email, COALESCE(contact_person, '') AS contact_person,
		       COALESCE(phone, '') AS phone, active, metadata,
		       parent_client_id, is_reseller, max_sub_accounts, is_sandbox, created_at, updated_at
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
