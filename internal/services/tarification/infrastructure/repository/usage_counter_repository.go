package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// UsageCounterRepository реализует domain.UsageCounterRepository
type UsageCounterRepository struct {
	db *sqlx.DB
}

// NewUsageCounterRepository создает новый репозиторий счетчиков использования
func NewUsageCounterRepository(db *sqlx.DB) *UsageCounterRepository {
	return &UsageCounterRepository{db: db}
}

// GetOrCreate получает или создает счетчик использования
func (r *UsageCounterRepository) GetOrCreate(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID) (*domain.UsageCounter, error) {
	var counter domain.UsageCounter
	query := `
		SELECT id, client_id, tariff_plan_id, tariff_period_id, segment_count, updated_at
		FROM usage_counters
		WHERE client_id = $1 AND tariff_plan_id = $2 AND tariff_period_id = $3
		FOR UPDATE
	`

	err := r.db.QueryRowContext(ctx, query, clientID, tariffPlanID, tariffPeriodID).Scan(
		&counter.ID,
		&counter.ClientID,
		&counter.TariffPlanID,
		&counter.TariffPeriodID,
		&counter.SegmentCount,
		&counter.UpdatedAt,
	)
	if err == nil {
		return &counter, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Создаем новый счетчик
	counter = *domain.NewUsageCounter(clientID, tariffPlanID, tariffPeriodID)
	insertQuery := `
		INSERT INTO usage_counters (id, client_id, tariff_plan_id, tariff_period_id, segment_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (client_id, tariff_plan_id, tariff_period_id)
		DO UPDATE SET updated_at = usage_counters.updated_at
		RETURNING id, client_id, tariff_plan_id, tariff_period_id, segment_count, updated_at
	`

	err = r.db.QueryRowContext(ctx, insertQuery,
		counter.ID, counter.ClientID, counter.TariffPlanID,
		counter.TariffPeriodID, counter.SegmentCount, counter.UpdatedAt,
	).Scan(
		&counter.ID, &counter.ClientID, &counter.TariffPlanID,
		&counter.TariffPeriodID, &counter.SegmentCount, &counter.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &counter, nil
}

// IncrementAndGet атомарно увеличивает счетчик и возвращает обновленное значение
func (r *UsageCounterRepository) IncrementAndGet(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID, segments int) (*domain.UsageCounter, error) {
	var counter domain.UsageCounter
	query := `
		UPDATE usage_counters
		SET segment_count = segment_count + $1, updated_at = NOW()
		WHERE client_id = $2 AND tariff_plan_id = $3 AND tariff_period_id = $4
		RETURNING id, client_id, tariff_plan_id, tariff_period_id, segment_count, updated_at
	`

	err := r.db.QueryRowContext(ctx, query, segments, clientID, tariffPlanID, tariffPeriodID).Scan(
		&counter.ID, &counter.ClientID, &counter.TariffPlanID,
		&counter.TariffPeriodID, &counter.SegmentCount, &counter.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUsageCounterNotFound
		}
		return nil, err
	}

	return &counter, nil
}

// GetByClient получает счетчики использования по клиенту
func (r *UsageCounterRepository) GetByClient(ctx context.Context, clientID uuid.UUID, tariffPlanID *uuid.UUID, limit, offset int) ([]*domain.UsageCounter, int, error) {
	var counters []*domain.UsageCounter
	var total int

	countQuery := `SELECT COUNT(*) FROM usage_counters WHERE client_id = $1`
	listQuery := `
		SELECT id, client_id, tariff_plan_id, tariff_period_id, segment_count, updated_at
		FROM usage_counters
		WHERE client_id = $1
	`

	args := []interface{}{clientID}
	if tariffPlanID != nil {
		countQuery += ` AND tariff_plan_id = $2`
		listQuery += ` AND tariff_plan_id = $2`
		args = append(args, *tariffPlanID)
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	listQuery += ` ORDER BY updated_at DESC LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var c domain.UsageCounter
		if err := rows.Scan(&c.ID, &c.ClientID, &c.TariffPlanID, &c.TariffPeriodID, &c.SegmentCount, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		counters = append(counters, &c)
	}

	return counters, total, rows.Err()
}

func itoa(i int) string {
	return string(rune('0'+i)) + ""
}
