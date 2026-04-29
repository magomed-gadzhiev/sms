package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
)

// periodActiveCache caches (plan_id) → *TariffPeriod. Periods rarely change.
// TTL kept shorter (30s) because period boundaries depend on wall-clock and
// a stale cached period might outlive its end_date.
var periodActiveCache = cache.NewHardCache(30 * time.Second)

// TariffPeriodRepository реализует domain.TariffPeriodRepository
type TariffPeriodRepository struct {
	db *sqlx.DB
}

// NewTariffPeriodRepository создает новый репозиторий тарифных периодов
func NewTariffPeriodRepository(db *sqlx.DB) *TariffPeriodRepository {
	return &TariffPeriodRepository{db: db}
}

// Create создает новый тарифный период
func (r *TariffPeriodRepository) Create(ctx context.Context, period *domain.TariffPeriod) error {
	query := `
		INSERT INTO tariff_periods (
			id, tariff_plan_id, start_date, end_date, created_at
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		period.ID,
		period.TariffPlanID,
		period.StartDate,
		period.EndDate,
		period.CreatedAt,
	)

	return err
}

// GetByID получает тарифный период по ID
func (r *TariffPeriodRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TariffPeriod, error) {
	var period domain.TariffPeriod
	query := `
		SELECT id, tariff_plan_id, start_date, end_date, created_at
		FROM tariff_periods
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&period.ID,
		&period.TariffPlanID,
		&period.StartDate,
		&period.EndDate,
		&period.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTariffPeriodNotFound
		}
		return nil, err
	}

	return &period, nil
}

// GetActiveByPlanID получает активный тарифный период для плана на указанную дату.
// Hard-cache по plan_id. На hot-path `now` меняется каждую мс, поэтому ключ —
// только plan_id. TTL 30s компенсирует риск stale-hit на границе периода.
func (r *TariffPeriodRepository) GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*domain.TariffPeriod, error) {
	cacheKey := planID.String()
	if periodActiveCache.Enabled() {
		if v, ok := periodActiveCache.Get(cacheKey); ok {
			if v == nil {
				return nil, domain.ErrTariffPeriodNotFound
			}
			p := v.(*domain.TariffPeriod)
			// Defensive: если кэшированный период уже кончился, инвалидируем.
			if p.EndDate != nil && p.EndDate.Before(now) {
				periodActiveCache.Invalidate(cacheKey)
			} else {
				return p, nil
			}
		}
	}

	var period domain.TariffPeriod
	query := `
		SELECT id, tariff_plan_id, start_date, end_date, created_at
		FROM tariff_periods
		WHERE tariff_plan_id = $1 AND start_date <= $2 AND (end_date IS NULL OR end_date >= $2)
	`

	err := r.db.QueryRowContext(ctx, query, planID, now).Scan(
		&period.ID,
		&period.TariffPlanID,
		&period.StartDate,
		&period.EndDate,
		&period.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			if periodActiveCache.Enabled() {
				periodActiveCache.Set(cacheKey, nil)
			}
			return nil, domain.ErrTariffPeriodNotFound
		}
		return nil, err
	}

	if periodActiveCache.Enabled() {
		periodActiveCache.Set(cacheKey, &period)
	}
	return &period, nil
}

// ListByPlanID получает все тарифные периоды для плана
func (r *TariffPeriodRepository) ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.TariffPeriod, error) {
	query := `
		SELECT id, tariff_plan_id, start_date, end_date, created_at
		FROM tariff_periods
		WHERE tariff_plan_id = $1
		ORDER BY start_date ASC
	`

	rows, err := r.db.QueryContext(ctx, query, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []*domain.TariffPeriod
	for rows.Next() {
		var period domain.TariffPeriod
		if err := rows.Scan(
			&period.ID,
			&period.TariffPlanID,
			&period.StartDate,
			&period.EndDate,
			&period.CreatedAt,
		); err != nil {
			return nil, err
		}
		periods = append(periods, &period)
	}

	return periods, rows.Err()
}

// HasActivePeriod проверяет, есть ли активный период для плана на текущую дату
func (r *TariffPeriodRepository) HasActivePeriod(ctx context.Context, planID uuid.UUID, now time.Time) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*)
		FROM tariff_periods
		WHERE tariff_plan_id = $1 AND start_date <= $2 AND (end_date IS NULL OR end_date >= $2)
	`

	err := r.db.QueryRowContext(ctx, query, planID, now).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
