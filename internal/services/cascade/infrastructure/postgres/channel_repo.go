package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

type ChannelRepo struct {
	pool *pgxpool.Pool
}

func NewChannelRepo(pool *pgxpool.Pool) *ChannelRepo {
	return &ChannelRepo{pool: pool}
}

// NewChannelRepository создаёт новый ChannelRepo (алиас NewChannelRepo для совместимости)
func NewChannelRepository(pool *pgxpool.Pool) *ChannelRepo {
	return NewChannelRepo(pool)
}

func (r *ChannelRepo) List(ctx context.Context) ([]*domain.ChannelConfig, error) {
	query := `SELECT id, channel_type, name, description, config, active, created_at, updated_at
		FROM delivery_channels ORDER BY created_at`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var result []*domain.ChannelConfig
	for rows.Next() {
		ch, err := r.scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel row: %w", err)
		}
		result = append(result, ch)
	}
	return result, nil
}

func (r *ChannelRepo) Get(ctx context.Context, id uuid.UUID) (*domain.ChannelConfig, error) {
	query := `SELECT id, channel_type, name, description, config, active, created_at, updated_at
		FROM delivery_channels WHERE id = $1`
	ch := &domain.ChannelConfig{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&ch.ID, &ch.ChannelType, &ch.Name, &ch.Description, &configJSON,
		&ch.Active, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	if err := json.Unmarshal(configJSON, &ch.Config); err != nil {
		return nil, fmt.Errorf("unmarshal channel config: %w", err)
	}
	return ch, nil
}

func (r *ChannelRepo) GetByType(ctx context.Context, ct domain.ChannelType) (*domain.ChannelConfig, error) {
	query := `SELECT id, channel_type, name, description, config, active, created_at, updated_at
		FROM delivery_channels WHERE channel_type = $1`
	ch := &domain.ChannelConfig{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx, query, string(ct)).Scan(
		&ch.ID, &ch.ChannelType, &ch.Name, &ch.Description, &configJSON,
		&ch.Active, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get channel by type: %w", err)
	}
	if err := json.Unmarshal(configJSON, &ch.Config); err != nil {
		return nil, fmt.Errorf("unmarshal channel config: %w", err)
	}
	return ch, nil
}

func (r *ChannelRepo) Create(ctx context.Context, ch *domain.ChannelConfig) error {
	if ch.ID == uuid.Nil {
		ch.ID = uuid.New()
	}
	now := time.Now()
	ch.CreatedAt = now
	ch.UpdatedAt = now

	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return fmt.Errorf("marshal channel config: %w", err)
	}

	query := `INSERT INTO delivery_channels (id, channel_type, name, description, config, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err = r.pool.Exec(ctx, query,
		ch.ID, string(ch.ChannelType), ch.Name, ch.Description, configJSON,
		ch.Active, ch.CreatedAt, ch.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}
	return nil
}

func (r *ChannelRepo) Update(ctx context.Context, ch *domain.ChannelConfig) error {
	ch.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return fmt.Errorf("marshal channel config: %w", err)
	}

	query := `UPDATE delivery_channels SET name = $1, description = $2, config = $3, updated_at = $4
		WHERE id = $5`
	tag, err := r.pool.Exec(ctx, query, ch.Name, ch.Description, configJSON, ch.UpdatedAt, ch.ID)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func (r *ChannelRepo) Toggle(ctx context.Context, id uuid.UUID, active bool) error {
	query := `UPDATE delivery_channels SET active = $1, updated_at = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, active, time.Now(), id)
	if err != nil {
		return fmt.Errorf("toggle channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func (r *ChannelRepo) scanChannel(rows pgx.Rows) (*domain.ChannelConfig, error) {
	ch := &domain.ChannelConfig{}
	var configJSON []byte
	if err := rows.Scan(
		&ch.ID, &ch.ChannelType, &ch.Name, &ch.Description, &configJSON,
		&ch.Active, &ch.CreatedAt, &ch.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(configJSON, &ch.Config); err != nil {
		return nil, fmt.Errorf("unmarshal channel config: %w", err)
	}
	return ch, nil
}
