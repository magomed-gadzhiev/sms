package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type LinkRepository struct {
	pool *pgxpool.Pool
}

func NewLinkRepository(pool *pgxpool.Pool) *LinkRepository {
	return &LinkRepository{pool: pool}
}

func (r *LinkRepository) CreateShortLink(ctx context.Context, link *domain.ShortLink) error {
	link.ID = uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO short_links (id, client_id, domain_id, code, original_url, message_id, campaign_id, recipient_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		link.ID, link.ClientID, link.DomainID, link.Code, link.OriginalURL,
		link.MessageID, link.CampaignID, link.RecipientID, link.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert short_link: %w", err)
	}
	return nil
}

func (r *LinkRepository) GetByCode(ctx context.Context, code string) (*domain.ShortLink, error) {
	var link domain.ShortLink
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, domain_id, code, original_url, message_id, campaign_id, recipient_id, expires_at, created_at
		 FROM short_links WHERE code = $1`, code,
	).Scan(&link.ID, &link.ClientID, &link.DomainID, &link.Code, &link.OriginalURL,
		&link.MessageID, &link.CampaignID, &link.RecipientID, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		return nil, domain.ErrLinkNotFound
	}
	return &link, nil
}

func (r *LinkRepository) GetClientActiveDomain(ctx context.Context, clientID uuid.UUID) (*domain.ClientDomain, error) {
	var d domain.ClientDomain
	// COALESCE: ssl_cert_path is NULL until SSL is provisioned, and a plain
	// string scan target errors on NULL — the caller would then treat the
	// client's active branded domain as absent (mirrors the DomainRepository fix).
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at, COALESCE(ssl_cert_path, ''), ssl_expires_at, created_at, updated_at
		 FROM client_domains WHERE client_id = $1 AND status = 'active' LIMIT 1`, clientID,
	).Scan(&d.ID, &d.ClientID, &d.Domain, &d.Status, &d.DNSTxtRecord, &d.DNSVerifiedAt,
		&d.SSLCertPath, &d.SSLExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, nil // No active domain — use default
	}
	return &d, nil
}
