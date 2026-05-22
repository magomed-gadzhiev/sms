package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTariffPlanRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTariffPlanRepo(pool *pgxpool.Pool) *ProviderTariffPlanRepo {
	return &ProviderTariffPlanRepo{pool: pool}
}

func (r *ProviderTariffPlanRepo) Create(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	query := `INSERT INTO provider_tariff_plans (id, provider_id, operator_id, strategy, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, query, plan.ID, plan.ProviderID, plan.OperatorID, plan.Strategy, plan.Active, plan.CreatedAt, plan.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create provider_tariff_plan: %w", err)
	}
	return nil
}

func (r *ProviderTariffPlanRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPlan, error) {
	query := `SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at FROM provider_tariff_plans WHERE id = $1`
	p := &domain.ProviderTariffPlan{}
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProviderTariffPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tariff_plan: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPlanRepo) GetActiveByProviderAndOperator(ctx context.Context, providerID, operatorID uuid.UUID) (*domain.ProviderTariffPlan, error) {
	query := `SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at
		FROM provider_tariff_plans WHERE provider_id = $1 AND operator_id = $2 AND active = true`
	p := &domain.ProviderTariffPlan{}
	err := r.pool.QueryRow(ctx, query, providerID, operatorID).Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNoActiveProviderTariffPlan
	}
	if err != nil {
		return nil, fmt.Errorf("get active provider_tariff_plan: %w", err)
	}
	return p, nil
}

func (r *ProviderTariffPlanRepo) Update(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	query := `UPDATE provider_tariff_plans SET strategy = $1, active = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, plan.Strategy, plan.Active, plan.ID)
	if err != nil {
		return fmt.Errorf("update provider_tariff_plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrProviderTariffPlanNotFound
	}
	return nil
}

func (r *ProviderTariffPlanRepo) List(ctx context.Context, providerID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.ProviderTariffPlan, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if providerID != nil {
		where += fmt.Sprintf(" AND provider_id = $%d", argN)
		args = append(args, *providerID)
		argN++
	}
	if activeOnly {
		where += " AND active = true"
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM provider_tariff_plans %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count provider_tariff_plans: %w", err)
	}

	query := fmt.Sprintf(`SELECT id, provider_id, operator_id, strategy, active, created_at, updated_at
		FROM provider_tariff_plans %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list provider_tariff_plans: %w", err)
	}
	defer rows.Close()

	var plans []*domain.ProviderTariffPlan
	for rows.Next() {
		p := &domain.ProviderTariffPlan{}
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.OperatorID, &p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan provider_tariff_plan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, total, nil
}
