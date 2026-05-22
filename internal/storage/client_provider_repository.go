package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ClientProviderWithTPS — запись client_providers с TPS-лимитом для pipeline.
type ClientProviderWithTPS struct {
	ClientID   uuid.UUID `db:"client_id"`
	ProviderID uuid.UUID `db:"provider_id"`
	TPSLimit   *int      `db:"tps_limit"`
}

// ClientProviderRepository предоставляет запросы к client_providers для pipeline.
type ClientProviderRepository struct {
	db *sqlx.DB
}

func NewClientProviderRepository(db *DB) *ClientProviderRepository {
	return &ClientProviderRepository{db: sqlx.NewDb(db.DB, "pgx")}
}

// GetAllActiveWithTPS возвращает все активные записи client_providers с установленным tps_limit.
func (r *ClientProviderRepository) GetAllActiveWithTPS(ctx context.Context) ([]ClientProviderWithTPS, error) {
	var result []ClientProviderWithTPS
	err := r.db.SelectContext(ctx, &result,
		`SELECT client_id, provider_id, tps_limit
		 FROM client_providers
		 WHERE active = true AND tps_limit IS NOT NULL`)
	return result, err
}
