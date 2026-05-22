package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

type PlanRepository struct {
	db *sqlx.DB
}

func NewPlanRepository(db *database.DB) *PlanRepository {
	return &PlanRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

func (r *PlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE id = $1`

	plan := &domain.Plan{}
	var featuresJSON []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
		&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
		&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
		&plan.RateLimitPerHour, &plan.RateLimitPerDay,
		&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan by id: %w", err)
	}
	if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
		return nil, fmt.Errorf("unmarshal plan features: %w", err)
	}
	return plan, nil
}

func (r *PlanRepository) GetByName(ctx context.Context, name string) (*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE name = $1`

	plan := &domain.Plan{}
	var featuresJSON []byte
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
		&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
		&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
		&plan.RateLimitPerHour, &plan.RateLimitPerDay,
		&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan by name: %w", err)
	}
	if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
		return nil, fmt.Errorf("unmarshal plan features: %w", err)
	}
	return plan, nil
}

func (r *PlanRepository) ListActive(ctx context.Context) ([]*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE active = true ORDER BY monthly_price_rub ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active plans: %w", err)
	}
	defer rows.Close()

	var plans []*domain.Plan
	for rows.Next() {
		plan := &domain.Plan{}
		var featuresJSON []byte
		err := rows.Scan(
			&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
			&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
			&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
			&plan.RateLimitPerHour, &plan.RateLimitPerDay,
			&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
			return nil, fmt.Errorf("unmarshal plan features: %w", err)
		}
		plans = append(plans, plan)
	}
	return plans, nil
}
