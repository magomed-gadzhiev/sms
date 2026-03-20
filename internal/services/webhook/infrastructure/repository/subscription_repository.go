package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type SubscriptionRepository struct {
	db *sqlx.DB
}

func NewSubscriptionRepository(db *sqlx.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

type subscriptionRow struct {
	ID         uuid.UUID      `db:"id"`
	ClientID   uuid.UUID      `db:"client_id"`
	URL        string         `db:"url"`
	EventTypes pq.StringArray `db:"event_types"`
	Secret     string         `db:"secret"`
	Active     bool           `db:"active"`
	CreatedAt  sql.NullTime   `db:"created_at"`
	UpdatedAt  sql.NullTime   `db:"updated_at"`
}

func (r *subscriptionRow) toDomain() *domain.Subscription {
	s := &domain.Subscription{
		ID:         r.ID,
		ClientID:   r.ClientID,
		URL:        r.URL,
		EventTypes: []string(r.EventTypes),
		Secret:     r.Secret,
		Active:     r.Active,
	}
	if r.CreatedAt.Valid {
		s.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		s.UpdatedAt = r.UpdatedAt.Time
	}
	return s
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	query := `INSERT INTO webhook_subscriptions (id, client_id, url, event_types, secret, active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, client_id, url, event_types, secret, active, created_at, updated_at`

	var row subscriptionRow
	err := r.db.QueryRowxContext(ctx, query,
		sub.ID, sub.ClientID, sub.URL, pq.StringArray(sub.EventTypes), sub.Secret, sub.Active,
	).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Subscription, error) {
	query := `SELECT id, client_id, url, event_types, secret, active, created_at, updated_at
		FROM webhook_subscriptions WHERE id = $1 AND client_id = $2`

	var row subscriptionRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SubscriptionRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	query := `SELECT id, client_id, url, event_types, secret, active, created_at, updated_at
		FROM webhook_subscriptions WHERE client_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryxContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		var row subscriptionRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subs = append(subs, row.toDomain())
	}
	return subs, nil
}

func (r *SubscriptionRepository) ListActiveByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	query := `SELECT id, client_id, url, event_types, secret, active, created_at, updated_at
		FROM webhook_subscriptions WHERE client_id = $1 AND active = true`

	rows, err := r.db.QueryxContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		var row subscriptionRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subs = append(subs, row.toDomain())
	}
	return subs, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error) {
	query := `UPDATE webhook_subscriptions SET url = $1, event_types = $2, active = $3
		WHERE id = $4 AND client_id = $5
		RETURNING id, client_id, url, event_types, secret, active, created_at, updated_at`

	var row subscriptionRow
	err := r.db.QueryRowxContext(ctx, query,
		sub.URL, pq.StringArray(sub.EventTypes), sub.Active, sub.ID, sub.ClientID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	query := `DELETE FROM webhook_subscriptions WHERE id = $1 AND client_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}

func (r *SubscriptionRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhook_subscriptions WHERE client_id = $1`, clientID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count subscriptions: %w", err)
	}
	return count, nil
}
