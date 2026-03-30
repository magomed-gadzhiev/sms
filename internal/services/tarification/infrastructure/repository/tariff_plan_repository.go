package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// TariffPlanRepository реализует domain.TariffPlanRepository
type TariffPlanRepository struct {
	db *sqlx.DB
}

// NewTariffPlanRepository создает новый репозиторий тарифных планов
func NewTariffPlanRepository(db *sqlx.DB) *TariffPlanRepository {
	return &TariffPlanRepository{db: db}
}

// Create создает новый тарифный план
func (r *TariffPlanRepository) Create(ctx context.Context, plan *domain.TariffPlan) error {
	query := `
		INSERT INTO tariff_plans (
			id, operator_id, sender_category, strategy, active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		plan.ID,
		plan.OperatorID,
		plan.SenderCategory,
		plan.Strategy,
		plan.Active,
		plan.CreatedAt,
		plan.UpdatedAt,
	)

	return err
}

// GetByID получает тарифный план по ID
func (r *TariffPlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffPlan, error) {
	var plan domain.TariffPlan
	query := `
		SELECT tp.id, tp.operator_id, tp.sender_category, tp.strategy, tp.active, tp.created_at, tp.updated_at,
		       COALESCE(c.currency, '')
		FROM tariff_plans tp
		LEFT JOIN operators o ON o.id = tp.operator_id
		LEFT JOIN countries c ON c.id = o.country_id
		WHERE tp.id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&plan.ID,
		&plan.OperatorID,
		&plan.SenderCategory,
		&plan.Strategy,
		&plan.Active,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&plan.Currency,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTariffPlanNotFound
		}
		return nil, err
	}

	return &plan, nil
}

// GetActiveByOperatorAndCategory получает активный тарифный план по оператору и категории
func (r *TariffPlanRepository) GetActiveByOperatorAndCategory(ctx context.Context, operatorID uuid.UUID, category domain.SenderCategory) (*domain.TariffPlan, error) {
	var plan domain.TariffPlan
	query := `
		SELECT tp.id, tp.operator_id, tp.sender_category, tp.strategy, tp.active, tp.created_at, tp.updated_at,
		       COALESCE(c.currency, '')
		FROM tariff_plans tp
		LEFT JOIN operators o ON o.id = tp.operator_id
		LEFT JOIN countries c ON c.id = o.country_id
		WHERE tp.operator_id = $1 AND tp.sender_category = $2 AND tp.active = true
	`

	err := r.db.QueryRowContext(ctx, query, operatorID, category).Scan(
		&plan.ID,
		&plan.OperatorID,
		&plan.SenderCategory,
		&plan.Strategy,
		&plan.Active,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&plan.Currency,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTariffPlanNotFound
		}
		return nil, err
	}

	return &plan, nil
}

// Update обновляет тарифный план
func (r *TariffPlanRepository) Update(ctx context.Context, plan *domain.TariffPlan) error {
	query := `
		UPDATE tariff_plans
		SET operator_id = $1, sender_category = $2, strategy = $3, active = $4, updated_at = $5
		WHERE id = $6
	`

	result, err := r.db.ExecContext(ctx, query,
		plan.OperatorID,
		plan.SenderCategory,
		plan.Strategy,
		plan.Active,
		plan.UpdatedAt,
		plan.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrTariffPlanNotFound
	}

	return nil
}

// List получает список тарифных планов с фильтрацией и пагинацией
func (r *TariffPlanRepository) List(ctx context.Context, operatorID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.TariffPlan, int, error) {
	var plans []*domain.TariffPlan
	var total int

	countQuery := `SELECT COUNT(*) FROM tariff_plans WHERE 1=1`
	listQuery := `
		SELECT tp.id, tp.operator_id, tp.sender_category, tp.strategy, tp.active, tp.created_at, tp.updated_at,
		       COALESCE(c.currency, '')
		FROM tariff_plans tp
		LEFT JOIN operators o ON o.id = tp.operator_id
		LEFT JOIN countries c ON c.id = o.country_id
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	if operatorID != nil {
		filter := fmt.Sprintf(` AND tp.operator_id = $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND operator_id = $%d`, argIdx)
		listQuery += filter
		args = append(args, *operatorID)
		argIdx++
	}

	if activeOnly {
		countQuery += ` AND active = true`
		listQuery += ` AND tp.active = true`
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	listQuery += fmt.Sprintf(` ORDER BY tp.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var plan domain.TariffPlan
		if err := rows.Scan(
			&plan.ID,
			&plan.OperatorID,
			&plan.SenderCategory,
			&plan.Strategy,
			&plan.Active,
			&plan.CreatedAt,
			&plan.UpdatedAt,
			&plan.Currency,
		); err != nil {
			return nil, 0, err
		}
		plans = append(plans, &plan)
	}

	return plans, total, rows.Err()
}
