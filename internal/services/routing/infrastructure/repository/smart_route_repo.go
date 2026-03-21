package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// SmartRouteWeightRepository implements domain.SmartRouteWeightRepository
type SmartRouteWeightRepository struct {
	db *sqlx.DB
}

// NewSmartRouteWeightRepository creates a new smart route weight repository
func NewSmartRouteWeightRepository(db *sqlx.DB) *SmartRouteWeightRepository {
	return &SmartRouteWeightRepository{db: db}
}

func (r *SmartRouteWeightRepository) Upsert(ctx context.Context, weight *domain.SmartRouteWeight) error {
	query := `
		INSERT INTO smart_route_weights (id, operator_code, country_code, cost_weight, quality_weight, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (operator_code, country_code) DO UPDATE SET
			cost_weight = EXCLUDED.cost_weight,
			quality_weight = EXCLUDED.quality_weight,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(ctx, query,
		weight.ID, weight.OperatorCode, weight.CountryCode,
		weight.CostWeight, weight.QualityWeight, weight.Active,
		weight.CreatedAt, weight.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка upsert smart route weights: %w", err)
	}
	return nil
}

func (r *SmartRouteWeightRepository) GetByOperatorAndCountry(ctx context.Context, operatorCode, countryCode string) (*domain.SmartRouteWeight, error) {
	query := `SELECT id, operator_code, country_code, cost_weight, quality_weight, active,
		EXTRACT(EPOCH FROM created_at)::bigint as created_at,
		EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
	FROM smart_route_weights
	WHERE country_code = $1 AND (operator_code = $2 OR operator_code IS NULL)
	AND active = true
	ORDER BY operator_code DESC NULLS LAST
	LIMIT 1`

	var row struct {
		ID            uuid.UUID `db:"id"`
		OperatorCode  *string   `db:"operator_code"`
		CountryCode   string    `db:"country_code"`
		CostWeight    float64   `db:"cost_weight"`
		QualityWeight float64   `db:"quality_weight"`
		Active        bool      `db:"active"`
		CreatedAt     int64     `db:"created_at"`
		UpdatedAt     int64     `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &row, query, countryCode, operatorCode); err != nil {
		return nil, domain.ErrSmartRouteWeightNotFound
	}

	weight := &domain.SmartRouteWeight{
		ID:            row.ID,
		CountryCode:   row.CountryCode,
		CostWeight:    row.CostWeight,
		QualityWeight: row.QualityWeight,
		Active:        row.Active,
		CreatedAt:     timeFromUnix(row.CreatedAt),
		UpdatedAt:     timeFromUnix(row.UpdatedAt),
	}
	if row.OperatorCode != nil {
		weight.OperatorCode = *row.OperatorCode
	}

	return weight, nil
}

func (r *SmartRouteWeightRepository) List(ctx context.Context, countryCode string) ([]*domain.SmartRouteWeight, error) {
	var query string
	var args []interface{}

	if countryCode != "" {
		query = `SELECT id, operator_code, country_code, cost_weight, quality_weight, active,
			EXTRACT(EPOCH FROM created_at)::bigint as created_at,
			EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
		FROM smart_route_weights WHERE country_code = $1 AND active = true ORDER BY operator_code`
		args = append(args, countryCode)
	} else {
		query = `SELECT id, operator_code, country_code, cost_weight, quality_weight, active,
			EXTRACT(EPOCH FROM created_at)::bigint as created_at,
			EXTRACT(EPOCH FROM updated_at)::bigint as updated_at
		FROM smart_route_weights WHERE active = true ORDER BY country_code, operator_code`
	}

	type row struct {
		ID            uuid.UUID `db:"id"`
		OperatorCode  *string   `db:"operator_code"`
		CountryCode   string    `db:"country_code"`
		CostWeight    float64   `db:"cost_weight"`
		QualityWeight float64   `db:"quality_weight"`
		Active        bool      `db:"active"`
		CreatedAt     int64     `db:"created_at"`
		UpdatedAt     int64     `db:"updated_at"`
	}

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("ошибка получения smart route weights: %w", err)
	}

	weights := make([]*domain.SmartRouteWeight, len(rows))
	for i, r := range rows {
		w := &domain.SmartRouteWeight{
			ID:            r.ID,
			CountryCode:   r.CountryCode,
			CostWeight:    r.CostWeight,
			QualityWeight: r.QualityWeight,
			Active:        r.Active,
			CreatedAt:     timeFromUnix(r.CreatedAt),
			UpdatedAt:     timeFromUnix(r.UpdatedAt),
		}
		if r.OperatorCode != nil {
			w.OperatorCode = *r.OperatorCode
		}
		weights[i] = w
	}
	return weights, nil
}

func (r *SmartRouteWeightRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM smart_route_weights WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления smart route weights: %w", err)
	}
	return nil
}

// timeFromUnix converts a Unix epoch timestamp to time.Time
func timeFromUnix(epoch int64) time.Time {
	return time.Unix(epoch, 0)
}
