package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
)

// operatorByIDCache caches operators by id. Operators change very rarely.
var operatorByIDCache = cache.NewHardCache(300 * time.Second)
var operatorByCodeCache = cache.NewHardCache(300 * time.Second)

// OperatorRepository реализует domain.OperatorRepository
type OperatorRepository struct {
	db *sqlx.DB
}

// NewOperatorRepository создает новый репозиторий операторов
func NewOperatorRepository(db *sqlx.DB) *OperatorRepository {
	return &OperatorRepository{
		db: db,
	}
}

// Create создает нового оператора
func (r *OperatorRepository) Create(ctx context.Context, operator *domain.Operator) error {
	query := `
		INSERT INTO operators (
			id, country_id, name, code, supports_paid_sender, supports_free_sender,
			monthly_tariff_amount, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		operator.ID,
		operator.CountryID,
		operator.Name,
		operator.Code,
		operator.SupportsPaidSender,
		operator.SupportsFreeSender,
		operator.MonthlyTariffAmount,
		operator.Active,
		operator.CreatedAt,
		operator.UpdatedAt,
	)

	return err
}

// GetByID получает оператора по ID. Hard-cache TTL 5 мин.
func (r *OperatorRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Operator, error) {
	cacheKey := id.String()
	if operatorByIDCache.Enabled() {
		if v, ok := operatorByIDCache.Get(cacheKey); ok {
			if v == nil {
				return nil, domain.ErrOperatorNotFound
			}
			return v.(*domain.Operator), nil
		}
	}

	var operator domain.Operator
	query := `
		SELECT id, country_id, name, code, supports_paid_sender, supports_free_sender,
			monthly_tariff_amount, active, created_at, updated_at
		FROM operators
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&operator.ID,
		&operator.CountryID,
		&operator.Name,
		&operator.Code,
		&operator.SupportsPaidSender,
		&operator.SupportsFreeSender,
		&operator.MonthlyTariffAmount,
		&operator.Active,
		&operator.CreatedAt,
		&operator.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if operatorByIDCache.Enabled() {
				operatorByIDCache.Set(cacheKey, nil)
			}
			return nil, domain.ErrOperatorNotFound
		}
		return nil, err
	}

	if operatorByIDCache.Enabled() {
		operatorByIDCache.Set(cacheKey, &operator)
	}
	return &operator, nil
}

// GetByCode получает оператора по коду. Hard-cache TTL 5 мин.
func (r *OperatorRepository) GetByCode(ctx context.Context, code string) (*domain.Operator, error) {
	if operatorByCodeCache.Enabled() {
		if v, ok := operatorByCodeCache.Get(code); ok {
			if v == nil {
				return nil, domain.ErrOperatorNotFound
			}
			return v.(*domain.Operator), nil
		}
	}

	var operator domain.Operator
	query := `
		SELECT id, country_id, name, code, supports_paid_sender, supports_free_sender,
			monthly_tariff_amount, active, created_at, updated_at
		FROM operators
		WHERE code = $1
	`

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&operator.ID,
		&operator.CountryID,
		&operator.Name,
		&operator.Code,
		&operator.SupportsPaidSender,
		&operator.SupportsFreeSender,
		&operator.MonthlyTariffAmount,
		&operator.Active,
		&operator.CreatedAt,
		&operator.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if operatorByCodeCache.Enabled() {
				operatorByCodeCache.Set(code, nil)
			}
			return nil, domain.ErrOperatorNotFound
		}
		return nil, err
	}

	if operatorByCodeCache.Enabled() {
		operatorByCodeCache.Set(code, &operator)
	}
	return &operator, nil
}

// Update обновляет оператора
func (r *OperatorRepository) Update(ctx context.Context, operator *domain.Operator) error {
	query := `
		UPDATE operators
		SET name = $1, code = $2, supports_paid_sender = $3, supports_free_sender = $4,
			monthly_tariff_amount = $5, active = $6, updated_at = $7
		WHERE id = $8
	`

	result, err := r.db.ExecContext(ctx, query,
		operator.Name,
		operator.Code,
		operator.SupportsPaidSender,
		operator.SupportsFreeSender,
		operator.MonthlyTariffAmount,
		operator.Active,
		operator.UpdatedAt,
		operator.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrOperatorNotFound
	}

	return nil
}

// List получает список операторов с фильтрацией и пагинацией
func (r *OperatorRepository) List(ctx context.Context, countryID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.Operator, int, error) {
	var total int
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if countryID != nil {
		whereClause += fmt.Sprintf(" AND country_id = $%d", argIdx)
		args = append(args, *countryID)
		argIdx++
	}
	if activeOnly {
		whereClause += " AND active = true"
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM operators %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, country_id, name, code, supports_paid_sender, supports_free_sender,
			monthly_tariff_amount, active, created_at, updated_at
		FROM operators
		%s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var operators []*domain.Operator
	for rows.Next() {
		var operator domain.Operator
		err := rows.Scan(
			&operator.ID,
			&operator.CountryID,
			&operator.Name,
			&operator.Code,
			&operator.SupportsPaidSender,
			&operator.SupportsFreeSender,
			&operator.MonthlyTariffAmount,
			&operator.Active,
			&operator.CreatedAt,
			&operator.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		operators = append(operators, &operator)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return operators, total, nil
}
