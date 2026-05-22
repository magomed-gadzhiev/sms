package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// OptOut represents a subscriber phone that has opted out.
type OptOut struct {
	ID         uuid.UUID  `db:"id"`
	ClientID   uuid.UUID  `db:"client_id"`
	Phone      string     `db:"phone"`
	Keyword    string     `db:"keyword"`
	OptedOutAt time.Time  `db:"opted_out_at"`
}

// OptOutRepository provides opt_out_list CRUD for the SMPP gateway.
type OptOutRepository struct {
	db *sqlx.DB
}

// NewOptOutRepository creates a new OptOutRepository.
func NewOptOutRepository(db *DB) *OptOutRepository {
	return &OptOutRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

// Add records an opt-out for a subscriber phone under a client.
// Duplicate entries (same client_id + phone) are silently ignored via ON CONFLICT DO NOTHING.
func (r *OptOutRepository) Add(ctx context.Context, clientID uuid.UUID, phone, keyword string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO opt_out_list (client_id, phone, keyword)
		VALUES ($1, $2, $3)
		ON CONFLICT (client_id, phone) DO NOTHING`,
		clientID, phone, keyword,
	)
	if err != nil {
		return fmt.Errorf("opt_out add: %w", err)
	}
	return nil
}

// IsOptedOut returns true if the given phone has opted out under the given client.
func (r *OptOutRepository) IsOptedOut(ctx context.Context, clientID uuid.UUID, phone string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM opt_out_list WHERE client_id = $1 AND phone = $2`,
		clientID, phone,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("opt_out IsOptedOut: %w", err)
	}
	return count > 0, nil
}
