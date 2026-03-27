package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

type ClientRoutingStrategyRepo struct {
	pool *pgxpool.Pool
}

func NewClientRoutingStrategyRepo(pool *pgxpool.Pool) *ClientRoutingStrategyRepo {
	return &ClientRoutingStrategyRepo{pool: pool}
}

func (r *ClientRoutingStrategyRepo) Upsert(ctx context.Context, s *domain.ClientRoutingStrategy) error {
	if s.OperatorID == nil {
		query := `INSERT INTO client_routing_strategies (id, client_id, operator_id, strategy, created_at, updated_at)
			VALUES ($1, $2, NULL, $3, $4, $5)
			ON CONFLICT (client_id) WHERE operator_id IS NULL
			DO UPDATE SET strategy = EXCLUDED.strategy, updated_at = EXCLUDED.updated_at`
		_, err := r.pool.Exec(ctx, query, s.ID, s.ClientID, s.Strategy, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("upsert client_routing_strategy (default): %w", err)
		}
		return nil
	}

	query := `INSERT INTO client_routing_strategies (id, client_id, operator_id, strategy, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (client_id, operator_id)
		DO UPDATE SET strategy = EXCLUDED.strategy, updated_at = EXCLUDED.updated_at`
	_, err := r.pool.Exec(ctx, query, s.ID, s.ClientID, s.OperatorID, s.Strategy, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert client_routing_strategy: %w", err)
	}
	return nil
}

func (r *ClientRoutingStrategyRepo) Get(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) (*domain.ClientRoutingStrategy, error) {
	s := &domain.ClientRoutingStrategy{}
	var err error

	if operatorID != nil {
		query := `SELECT id, client_id, operator_id, strategy, created_at, updated_at
			FROM client_routing_strategies WHERE client_id = $1 AND operator_id = $2`
		err = r.pool.QueryRow(ctx, query, clientID, *operatorID).Scan(
			&s.ID, &s.ClientID, &s.OperatorID, &s.Strategy, &s.CreatedAt, &s.UpdatedAt,
		)
	} else {
		query := `SELECT id, client_id, operator_id, strategy, created_at, updated_at
			FROM client_routing_strategies WHERE client_id = $1 AND operator_id IS NULL`
		err = r.pool.QueryRow(ctx, query, clientID).Scan(
			&s.ID, &s.ClientID, &s.OperatorID, &s.Strategy, &s.CreatedAt, &s.UpdatedAt,
		)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrRoutingStrategyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client_routing_strategy: %w", err)
	}
	return s, nil
}

func (r *ClientRoutingStrategyRepo) Delete(ctx context.Context, clientID uuid.UUID, operatorID *uuid.UUID) error {
	var tag interface{ RowsAffected() int64 }
	var err error

	if operatorID != nil {
		tag, err = r.pool.Exec(ctx, `DELETE FROM client_routing_strategies WHERE client_id = $1 AND operator_id = $2`, clientID, *operatorID)
	} else {
		tag, err = r.pool.Exec(ctx, `DELETE FROM client_routing_strategies WHERE client_id = $1 AND operator_id IS NULL`, clientID)
	}

	if err != nil {
		return fmt.Errorf("delete client_routing_strategy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRoutingStrategyNotFound
	}
	return nil
}
