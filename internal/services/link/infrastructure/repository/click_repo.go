package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type ClickRepository struct {
	pool *pgxpool.Pool
}

func NewClickRepository(pool *pgxpool.Pool) *ClickRepository {
	return &ClickRepository{pool: pool}
}

func (r *ClickRepository) Insert(ctx context.Context, event *domain.ClickEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO click_events (id, short_link_id, client_id, campaign_id, phone, clicked_at, ip_address, user_agent, referer, country_code, is_unique)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::inet, $8, $9, $10, $11)`,
		event.ID, event.ShortLinkID, event.ClientID, event.CampaignID, event.Phone,
		event.ClickedAt, event.IPAddress, event.UserAgent, event.Referer, event.CountryCode, event.IsUnique,
	)
	if err != nil {
		return fmt.Errorf("insert click_event: %w", err)
	}
	return nil
}

type ClickStats struct {
	TotalClicks  int32
	UniqueClicks int32
}

func (r *ClickRepository) GetStatsByLink(ctx context.Context, linkID string) (*ClickStats, error) {
	var stats ClickStats
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_unique) FROM click_events WHERE short_link_id = $1`, linkID,
	).Scan(&stats.TotalClicks, &stats.UniqueClicks)
	if err != nil {
		return nil, fmt.Errorf("get click stats: %w", err)
	}
	return &stats, nil
}
