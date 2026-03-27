package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type ProviderTarificationLogRepo struct {
	pool *pgxpool.Pool
}

func NewProviderTarificationLogRepo(pool *pgxpool.Pool) *ProviderTarificationLogRepo {
	return &ProviderTarificationLogRepo{pool: pool}
}

func (r *ProviderTarificationLogRepo) Create(ctx context.Context, log *domain.ProviderTarificationLog) error {
	query := `INSERT INTO provider_tarification_log
		(id, provider_id, operator_id, client_id, message_id, segment_count, price_per_segment, total_cost, strategy, provider_tariff_plan_id, provider_tariff_period_id, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
	_, err := r.pool.Exec(ctx, query,
		log.ID, log.ProviderID, log.OperatorID, log.ClientID, log.MessageID,
		log.SegmentCount, log.PricePerSegment, log.TotalCost, log.Strategy,
		log.ProviderTariffPlanID, log.ProviderTariffPeriodID, log.IdempotencyKey, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create provider_tarification_log: %w", err)
	}
	return nil
}

func (r *ProviderTarificationLogRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.ProviderTarificationLog, error) {
	query := `SELECT id, provider_id, operator_id, client_id, message_id, segment_count, price_per_segment, total_cost, strategy, provider_tariff_plan_id, provider_tariff_period_id, idempotency_key, created_at
		FROM provider_tarification_log WHERE idempotency_key = $1`
	l := &domain.ProviderTarificationLog{}
	err := r.pool.QueryRow(ctx, query, key).Scan(
		&l.ID, &l.ProviderID, &l.OperatorID, &l.ClientID, &l.MessageID,
		&l.SegmentCount, &l.PricePerSegment, &l.TotalCost, &l.Strategy,
		&l.ProviderTariffPlanID, &l.ProviderTariffPeriodID, &l.IdempotencyKey, &l.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get provider_tarification_log by key: %w", err)
	}
	return l, nil
}
