package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// SenderRegistrationRepository реализует domain.SenderRegistrationRepository
type SenderRegistrationRepository struct {
	db *sqlx.DB
}

// NewSenderRegistrationRepository создает новый репозиторий регистраций отправителей
func NewSenderRegistrationRepository(db *sqlx.DB) *SenderRegistrationRepository {
	return &SenderRegistrationRepository{db: db}
}

// Create создает новую регистрацию отправителя
func (r *SenderRegistrationRepository) Create(ctx context.Context, reg *domain.SenderRegistration) error {
	query := `
		INSERT INTO sender_registrations (
			id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		reg.ID,
		reg.ClientID,
		reg.OperatorID,
		reg.SenderName,
		reg.Type,
		reg.Status,
		reg.CreatedAt,
		reg.UpdatedAt,
	)

	return err
}

// GetByID получает регистрацию отправителя по ID
func (r *SenderRegistrationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderRegistration, error) {
	var reg domain.SenderRegistration
	query := `
		SELECT id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		FROM sender_registrations
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&reg.ID,
		&reg.ClientID,
		&reg.OperatorID,
		&reg.SenderName,
		&reg.Type,
		&reg.Status,
		&reg.CreatedAt,
		&reg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderRegistrationNotFound
		}
		return nil, err
	}

	return &reg, nil
}

// GetByClientAndOperator получает активные регистрации отправителя для клиента и оператора
func (r *SenderRegistrationRepository) GetByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*domain.SenderRegistration, error) {
	query := `
		SELECT id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		FROM sender_registrations
		WHERE client_id = $1 AND operator_id = $2 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, clientID, operatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []*domain.SenderRegistration
	for rows.Next() {
		var reg domain.SenderRegistration
		if err := rows.Scan(
			&reg.ID,
			&reg.ClientID,
			&reg.OperatorID,
			&reg.SenderName,
			&reg.Type,
			&reg.Status,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		regs = append(regs, &reg)
	}

	return regs, rows.Err()
}

// GetActiveByClientOperatorName получает активную регистрацию по клиенту, оператору и имени
func (r *SenderRegistrationRepository) GetActiveByClientOperatorName(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (*domain.SenderRegistration, error) {
	var reg domain.SenderRegistration
	query := `
		SELECT id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		FROM sender_registrations
		WHERE client_id = $1 AND operator_id = $2 AND sender_name = $3 AND status = 'active'
	`

	err := r.db.QueryRowContext(ctx, query, clientID, operatorID, senderName).Scan(
		&reg.ID,
		&reg.ClientID,
		&reg.OperatorID,
		&reg.SenderName,
		&reg.Type,
		&reg.Status,
		&reg.CreatedAt,
		&reg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSenderRegistrationNotFound
		}
		return nil, err
	}

	return &reg, nil
}

// Update обновляет регистрацию отправителя
func (r *SenderRegistrationRepository) Update(ctx context.Context, reg *domain.SenderRegistration) error {
	query := `
		UPDATE sender_registrations
		SET client_id = $1, operator_id = $2, sender_name = $3, type = $4, status = $5, updated_at = $6
		WHERE id = $7
	`

	result, err := r.db.ExecContext(ctx, query,
		reg.ClientID,
		reg.OperatorID,
		reg.SenderName,
		reg.Type,
		reg.Status,
		reg.UpdatedAt,
		reg.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrSenderRegistrationNotFound
	}

	return nil
}

// List получает список регистраций отправителей с фильтрацией и пагинацией
func (r *SenderRegistrationRepository) List(ctx context.Context, clientID, operatorID *uuid.UUID, limit, offset int) ([]*domain.SenderRegistration, int, error) {
	var regs []*domain.SenderRegistration
	var total int

	countQuery := `SELECT COUNT(*) FROM sender_registrations WHERE 1=1`
	listQuery := `
		SELECT id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		FROM sender_registrations
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	if clientID != nil {
		filter := fmt.Sprintf(` AND client_id = $%d`, argIdx)
		countQuery += filter
		listQuery += filter
		args = append(args, *clientID)
		argIdx++
	}

	if operatorID != nil {
		filter := fmt.Sprintf(` AND operator_id = $%d`, argIdx)
		countQuery += filter
		listQuery += filter
		args = append(args, *operatorID)
		argIdx++
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var reg domain.SenderRegistration
		if err := rows.Scan(
			&reg.ID,
			&reg.ClientID,
			&reg.OperatorID,
			&reg.SenderName,
			&reg.Type,
			&reg.Status,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		regs = append(regs, &reg)
	}

	return regs, total, rows.Err()
}

// ListActivePaid возвращает все активные платные регистрации для планировщика
func (r *SenderRegistrationRepository) ListActivePaid(ctx context.Context) ([]*domain.SenderRegistration, error) {
	query := `
		SELECT id, client_id, operator_id, sender_name, type, status, created_at, updated_at
		FROM sender_registrations
		WHERE type = 'paid' AND status = 'active'
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []*domain.SenderRegistration
	for rows.Next() {
		var reg domain.SenderRegistration
		if err := rows.Scan(
			&reg.ID,
			&reg.ClientID,
			&reg.OperatorID,
			&reg.SenderName,
			&reg.Type,
			&reg.Status,
			&reg.CreatedAt,
			&reg.UpdatedAt,
		); err != nil {
			return nil, err
		}
		regs = append(regs, &reg)
	}
	return regs, rows.Err()
}
