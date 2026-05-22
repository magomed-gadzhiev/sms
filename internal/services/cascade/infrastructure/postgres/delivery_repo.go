package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

type DeliveryRepo struct {
	pool *pgxpool.Pool
}

func NewDeliveryRepo(pool *pgxpool.Pool) *DeliveryRepo {
	return &DeliveryRepo{pool: pool}
}

func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepo {
	return NewDeliveryRepo(pool)
}

func (r *DeliveryRepo) Create(ctx context.Context, d *domain.Delivery) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	var messageID *string
	if d.MessageID != nil {
		s := d.MessageID.String()
		messageID = &s
	}

	query := `INSERT INTO deliveries
		(id, client_id, message_id, strategy_id, recipient, text, sender_name,
		 status, current_step, delivered_via, total_cost, currency, request_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err := r.pool.Exec(ctx, query,
		d.ID, d.ClientID, messageID, d.StrategyID,
		d.Recipient, d.Text, d.SenderName,
		string(d.Status), d.CurrentStep, d.DeliveredVia,
		d.TotalCost, d.Currency, d.RequestID,
		d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create delivery: %w", err)
	}
	return nil
}

func (r *DeliveryRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Delivery, error) {
	query := `SELECT id, client_id, message_id, strategy_id, recipient, text, sender_name,
		status, current_step, delivered_via, total_cost, currency, request_id, created_at, updated_at
		FROM deliveries WHERE id = $1`

	d, err := r.scanDelivery(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDeliveryNotFound
	}
	return d, err
}

func (r *DeliveryRepo) GetByClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Delivery, error) {
	query := `SELECT id, client_id, message_id, strategy_id, recipient, text, sender_name,
		status, current_step, delivered_via, total_cost, currency, request_id, created_at, updated_at
		FROM deliveries WHERE id = $1 AND client_id = $2`

	d, err := r.scanDelivery(r.pool.QueryRow(ctx, query, id, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDeliveryNotFound
	}
	return d, err
}

func (r *DeliveryRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeliveryStatus, deliveredVia string) error {
	query := `UPDATE deliveries SET status = $1, delivered_via = $2, updated_at = $3 WHERE id = $4`
	tag, err := r.pool.Exec(ctx, query, string(status), deliveredVia, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryRepo) UpdateStatusCAS(ctx context.Context, id uuid.UUID, expectedStatus domain.DeliveryStatus, newStatus domain.DeliveryStatus, deliveredVia string) error {
	query := `UPDATE deliveries SET status = $1, delivered_via = $2, updated_at = $3
	          WHERE id = $4 AND status = $5`
	tag, err := r.pool.Exec(ctx, query, string(newStatus), deliveredVia, time.Now(), id, string(expectedStatus))
	if err != nil {
		return fmt.Errorf("update delivery status cas: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryConflict
	}
	return nil
}

func (r *DeliveryRepo) UpdateStep(ctx context.Context, id uuid.UUID, step int) error {
	query := `UPDATE deliveries SET current_step = $1, updated_at = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, step, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update delivery step: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryRepo) UpdateCost(ctx context.Context, id uuid.UUID, totalCost float64) error {
	query := `UPDATE deliveries SET total_cost = $1, updated_at = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, totalCost, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update delivery cost: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryRepo) List(ctx context.Context, filter domain.DeliveryFilter) ([]*domain.Delivery, int, error) {
	conditions := []string{"client_id = $1"}
	args := []interface{}{filter.ClientID}
	argIdx := 2

	if filter.StrategyID != nil {
		conditions = append(conditions, fmt.Sprintf("strategy_id = $%d", argIdx))
		args = append(args, *filter.StrategyID)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.DateTo)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM deliveries %s`, where)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count deliveries: %w", err)
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	args = append(args, pageSize, offset)
	query := fmt.Sprintf(`SELECT id, client_id, message_id, strategy_id, recipient, text, sender_name,
		status, current_step, delivered_via, total_cost, currency, request_id, created_at, updated_at
		FROM deliveries %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []*domain.Delivery
	for rows.Next() {
		d, err := r.scanDeliveryRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, total, nil
}

func (r *DeliveryRepo) HasActiveByStrategy(ctx context.Context, strategyID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM deliveries WHERE strategy_id = $1 AND status IN ('pending', 'in_progress')
	)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, strategyID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check active deliveries: %w", err)
	}
	return exists, nil
}

func (r *DeliveryRepo) Stats(ctx context.Context, filter domain.StatsFilter) (*domain.DeliveryStats, error) {
	query := `SELECT
		COUNT(*) as total,
		COUNT(*) FILTER (WHERE status = 'delivered') as delivered_count,
		COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
		COALESCE(AVG(total_cost) FILTER (WHERE status = 'delivered'), 0) as avg_cost,
		COALESCE(SUM(total_cost), 0) as total_cost,
		COALESCE(MAX(currency), 'RUB') as currency
		FROM deliveries
		WHERE client_id = $1 AND created_at BETWEEN $2 AND $3`

	args := []interface{}{filter.ClientID, filter.DateFrom, filter.DateTo}
	if filter.StrategyID != nil {
		query += " AND strategy_id = $4"
		args = append(args, *filter.StrategyID)
	}

	stats := &domain.DeliveryStats{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&stats.Total, &stats.DeliveredCount, &stats.FailedCount,
		&stats.AvgCost, &stats.TotalCost, &stats.Currency,
	)
	if err != nil {
		return nil, fmt.Errorf("delivery stats: %w", err)
	}

	if stats.Total > 0 {
		stats.DeliveryRate = float64(stats.DeliveredCount) / float64(stats.Total) * 100
	}

	// Per-channel stats
	channelQuery := `SELECT
		da.channel_type,
		COUNT(*) as attempt_count,
		COUNT(*) FILTER (WHERE da.status = 'delivered') as delivered_count,
		COALESCE(AVG(EXTRACT(EPOCH FROM (da.result_at - da.sent_at)) * 1000) FILTER (WHERE da.result_at IS NOT NULL AND da.sent_at IS NOT NULL), 0) as avg_latency_ms,
		COALESCE(AVG(da.cost) FILTER (WHERE da.status = 'delivered'), 0) as avg_cost
		FROM delivery_attempts da
		JOIN deliveries d ON d.id = da.delivery_id
		WHERE d.client_id = $1 AND d.created_at BETWEEN $2 AND $3
		GROUP BY da.channel_type`

	chArgs := []interface{}{filter.ClientID, filter.DateFrom, filter.DateTo}
	rows, err := r.pool.Query(ctx, channelQuery, chArgs...)
	if err != nil {
		return nil, fmt.Errorf("channel stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cs domain.ChannelStat
		if err := rows.Scan(&cs.ChannelType, &cs.AttemptCount, &cs.DeliveredCount, &cs.AvgLatencyMs, &cs.AvgCost); err != nil {
			return nil, fmt.Errorf("scan channel stat: %w", err)
		}
		if cs.AttemptCount > 0 {
			cs.DeliveryRate = float64(cs.DeliveredCount) / float64(cs.AttemptCount) * 100
		}
		stats.ChannelStats = append(stats.ChannelStats, cs)
	}

	return stats, nil
}

func (r *DeliveryRepo) scanDelivery(row pgx.Row) (*domain.Delivery, error) {
	d := &domain.Delivery{}
	var messageID *string
	var deliveredVia *string
	err := row.Scan(
		&d.ID, &d.ClientID, &messageID, &d.StrategyID,
		&d.Recipient, &d.Text, &d.SenderName,
		&d.Status, &d.CurrentStep, &deliveredVia,
		&d.TotalCost, &d.Currency, &d.RequestID,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if messageID != nil {
		id, err := uuid.Parse(*messageID)
		if err == nil {
			d.MessageID = &id
		}
	}
	if deliveredVia != nil {
		d.DeliveredVia = *deliveredVia
	}
	return d, nil
}

func (r *DeliveryRepo) scanDeliveryRow(rows pgx.Rows) (*domain.Delivery, error) {
	d := &domain.Delivery{}
	var messageID *string
	var deliveredVia *string
	err := rows.Scan(
		&d.ID, &d.ClientID, &messageID, &d.StrategyID,
		&d.Recipient, &d.Text, &d.SenderName,
		&d.Status, &d.CurrentStep, &deliveredVia,
		&d.TotalCost, &d.Currency, &d.RequestID,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if messageID != nil {
		id, err := uuid.Parse(*messageID)
		if err == nil {
			d.MessageID = &id
		}
	}
	if deliveredVia != nil {
		d.DeliveredVia = *deliveredVia
	}
	return d, nil
}
