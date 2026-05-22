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

type StrategyRepo struct {
	pool *pgxpool.Pool
}

func NewStrategyRepo(pool *pgxpool.Pool) *StrategyRepo {
	return &StrategyRepo{pool: pool}
}

// NewStrategyRepository создаёт новый StrategyRepo (алиас NewStrategyRepo для совместимости)
func NewStrategyRepository(pool *pgxpool.Pool) *StrategyRepo {
	return NewStrategyRepo(pool)
}

func (r *StrategyRepo) List(ctx context.Context, activeOnly bool) ([]*domain.DeliveryStrategy, error) {
	query := `SELECT id, name, description, mode, active, created_at, updated_at
		FROM delivery_strategies`
	if activeOnly {
		query += " WHERE active = true"
	}
	query += " ORDER BY created_at"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}
	defer rows.Close()

	var strategies []*domain.DeliveryStrategy
	for rows.Next() {
		s := &domain.DeliveryStrategy{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Mode, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan strategy row: %w", err)
		}
		strategies = append(strategies, s)
	}

	// Load steps for each strategy
	for _, s := range strategies {
		steps, err := r.loadSteps(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		s.Steps = steps
	}

	return strategies, nil
}

func (r *StrategyRepo) Get(ctx context.Context, id uuid.UUID) (*domain.DeliveryStrategy, error) {
	query := `SELECT id, name, description, mode, active, created_at, updated_at
		FROM delivery_strategies WHERE id = $1`
	s := &domain.DeliveryStrategy{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.Mode, &s.Active, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrStrategyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get strategy: %w", err)
	}

	steps, err := r.loadSteps(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	s.Steps = steps

	return s, nil
}

func (r *StrategyRepo) Create(ctx context.Context, s *domain.DeliveryStrategy) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	query := `INSERT INTO delivery_strategies (id, name, description, mode, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = tx.Exec(ctx, query, s.ID, s.Name, s.Description, string(s.Mode), s.Active, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert strategy: %w", err)
	}

	if err := r.insertSteps(ctx, tx, s.ID, s.Steps); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *StrategyRepo) Update(ctx context.Context, s *domain.DeliveryStrategy) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	s.UpdatedAt = time.Now()

	query := `UPDATE delivery_strategies SET name = $1, description = $2, mode = $3, active = $4, updated_at = $5
		WHERE id = $6`
	tag, err := tx.Exec(ctx, query, s.Name, s.Description, string(s.Mode), s.Active, s.UpdatedAt, s.ID)
	if err != nil {
		return fmt.Errorf("update strategy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStrategyNotFound
	}

	// Delete old steps and insert new ones
	_, err = tx.Exec(ctx, `DELETE FROM delivery_strategy_steps WHERE strategy_id = $1`, s.ID)
	if err != nil {
		return fmt.Errorf("delete old steps: %w", err)
	}

	if err := r.insertSteps(ctx, tx, s.ID, s.Steps); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *StrategyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM delivery_strategies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete strategy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrStrategyNotFound
	}
	return nil
}

func (r *StrategyRepo) loadSteps(ctx context.Context, strategyID uuid.UUID) ([]domain.StrategyStep, error) {
	query := `SELECT dss.id, dss.strategy_id, dss.channel_id, dc.channel_type, dc.name,
			dss.step_order, dss.timeout_s, dss.billable, dss.created_at
		FROM delivery_strategy_steps dss
		JOIN delivery_channels dc ON dc.id = dss.channel_id
		WHERE dss.strategy_id = $1
		ORDER BY dss.step_order`

	rows, err := r.pool.Query(ctx, query, strategyID)
	if err != nil {
		return nil, fmt.Errorf("load strategy steps: %w", err)
	}
	defer rows.Close()

	var steps []domain.StrategyStep
	for rows.Next() {
		var step domain.StrategyStep
		if err := rows.Scan(
			&step.ID, &step.StrategyID, &step.ChannelID, &step.ChannelType,
			&step.ChannelName, &step.StepOrder, &step.TimeoutS, &step.Billable, &step.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan strategy step row: %w", err)
		}
		steps = append(steps, step)
	}

	if steps == nil {
		steps = make([]domain.StrategyStep, 0)
	}
	return steps, nil
}

func (r *StrategyRepo) insertSteps(ctx context.Context, tx pgx.Tx, strategyID uuid.UUID, steps []domain.StrategyStep) error {
	query := `INSERT INTO delivery_strategy_steps (id, strategy_id, channel_id, step_order, timeout_s, billable, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, step := range steps {
		if step.ID == uuid.Nil {
			step.ID = uuid.New()
		}
		step.StrategyID = strategyID
		if step.CreatedAt.IsZero() {
			step.CreatedAt = time.Now()
		}
		_, err := tx.Exec(ctx, query,
			step.ID, step.StrategyID, step.ChannelID, step.StepOrder, step.TimeoutS, step.Billable, step.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert strategy step: %w", err)
		}
	}
	return nil
}
