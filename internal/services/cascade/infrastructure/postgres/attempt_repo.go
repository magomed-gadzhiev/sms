package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

type AttemptRepo struct {
	pool *pgxpool.Pool
}

func NewAttemptRepo(pool *pgxpool.Pool) *AttemptRepo {
	return &AttemptRepo{pool: pool}
}

func NewAttemptRepository(pool *pgxpool.Pool) *AttemptRepo {
	return NewAttemptRepo(pool)
}

func (r *AttemptRepo) Create(ctx context.Context, a *domain.DeliveryAttempt) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()

	query := `INSERT INTO delivery_attempts
		(id, delivery_id, channel_id, channel_type, step_order, status,
		 provider_ref, cost, currency, error_message, sent_at, result_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (delivery_id, step_order) DO NOTHING`

	tag, err := r.pool.Exec(ctx, query,
		a.ID, a.DeliveryID, a.ChannelID, a.ChannelType, a.StepOrder,
		string(a.Status), a.ProviderRef, a.Cost, a.Currency, a.ErrorMessage,
		a.SentAt, a.ResultAt, a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAttemptAlreadyExists
	}
	return nil
}

func (r *AttemptRepo) Get(ctx context.Context, id uuid.UUID) (*domain.DeliveryAttempt, error) {
	query := `SELECT id, delivery_id, channel_id, channel_type, step_order, status,
		provider_ref, cost, currency, error_message, sent_at, result_at, created_at
		FROM delivery_attempts WHERE id = $1`

	a, err := r.scanAttempt(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDeliveryNotFound
	}
	return a, err
}

func (r *AttemptRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.AttemptStatus, providerRef string, errMsg string, resultAt *time.Time) error {
	query := `UPDATE delivery_attempts
		SET status = $1, provider_ref = $2, error_message = $3, result_at = $4
		WHERE id = $5`
	tag, err := r.pool.Exec(ctx, query, string(status), providerRef, errMsg, resultAt, id)
	if err != nil {
		return fmt.Errorf("update attempt status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *AttemptRepo) UpdateCost(ctx context.Context, id uuid.UUID, cost float64) error {
	query := `UPDATE delivery_attempts SET cost = $1 WHERE id = $2`
	tag, err := r.pool.Exec(ctx, query, cost, id)
	if err != nil {
		return fmt.Errorf("update attempt cost: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *AttemptRepo) ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	query := `SELECT id, delivery_id, channel_id, channel_type, step_order, status,
		provider_ref, cost, currency, error_message, sent_at, result_at, created_at
		FROM delivery_attempts WHERE delivery_id = $1 ORDER BY step_order, created_at`

	rows, err := r.pool.Query(ctx, query, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("list attempts by delivery: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.DeliveryAttempt
	for rows.Next() {
		a, err := r.scanAttemptRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}

func (r *AttemptRepo) FindPendingTimedOut(ctx context.Context) ([]*domain.DeliveryAttempt, error) {
	// Find attempts that are pending or sent and have exceeded their step timeout
	// We join with delivery_strategy_steps to get timeout_s
	query := `SELECT da.id, da.delivery_id, da.channel_id, da.channel_type, da.step_order, da.status,
		da.provider_ref, da.cost, da.currency, da.error_message, da.sent_at, da.result_at, da.created_at
		FROM delivery_attempts da
		JOIN deliveries d ON d.id = da.delivery_id
		JOIN delivery_strategies ds ON ds.id = d.strategy_id
		JOIN delivery_strategy_steps dss ON dss.strategy_id = d.strategy_id AND dss.step_order = da.step_order
		WHERE da.status IN ('pending', 'sent')
		AND da.created_at + (dss.timeout_s * interval '1 second') < now()
		AND d.status = 'in_progress'`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find timed out attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.DeliveryAttempt
	for rows.Next() {
		a, err := r.scanAttemptRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan timed out attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}

func (r *AttemptRepo) scanAttempt(row pgx.Row) (*domain.DeliveryAttempt, error) {
	a := &domain.DeliveryAttempt{}
	var status string
	err := row.Scan(
		&a.ID, &a.DeliveryID, &a.ChannelID, &a.ChannelType, &a.StepOrder, &status,
		&a.ProviderRef, &a.Cost, &a.Currency, &a.ErrorMessage,
		&a.SentAt, &a.ResultAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Status = domain.AttemptStatus(status)
	return a, nil
}

func (r *AttemptRepo) scanAttemptRow(rows pgx.Rows) (*domain.DeliveryAttempt, error) {
	a := &domain.DeliveryAttempt{}
	var status string
	err := rows.Scan(
		&a.ID, &a.DeliveryID, &a.ChannelID, &a.ChannelType, &a.StepOrder, &status,
		&a.ProviderRef, &a.Cost, &a.Currency, &a.ErrorMessage,
		&a.SentAt, &a.ResultAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Status = domain.AttemptStatus(status)
	return a, nil
}
