package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type DomainRepository struct {
	pool *pgxpool.Pool
}

func NewDomainRepository(pool *pgxpool.Pool) *DomainRepository {
	return &DomainRepository{pool: pool}
}

func (r *DomainRepository) Create(ctx context.Context, d *domain.ClientDomain) error {
	d.ID = uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO client_domains (id, client_id, domain, status, dns_txt_record)
		 VALUES ($1, $2, $3, $4, $5)`,
		d.ID, d.ClientID, d.Domain, d.Status, d.DNSTxtRecord,
	)
	if err != nil {
		return fmt.Errorf("insert client_domain: %w", err)
	}
	return nil
}

func (r *DomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ClientDomain, error) {
	var d domain.ClientDomain
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at,
			COALESCE(ssl_cert_path, '') AS ssl_cert_path, ssl_expires_at, created_at, updated_at
		 FROM client_domains WHERE id = $1`, id,
	).Scan(&d.ID, &d.ClientID, &d.Domain, &d.Status, &d.DNSTxtRecord, &d.DNSVerifiedAt,
		&d.SSLCertPath, &d.SSLExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, domain.ErrDomainNotFound
	}
	return &d, nil
}

func (r *DomainRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientDomain, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at,
			COALESCE(ssl_cert_path, '') AS ssl_cert_path, ssl_expires_at, created_at, updated_at
		 FROM client_domains WHERE client_id = $1 ORDER BY created_at DESC`, clientID,
	)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []*domain.ClientDomain
	for rows.Next() {
		var d domain.ClientDomain
		if err := rows.Scan(&d.ID, &d.ClientID, &d.Domain, &d.Status, &d.DNSTxtRecord, &d.DNSVerifiedAt,
			&d.SSLCertPath, &d.SSLExpiresAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		domains = append(domains, &d)
	}
	return domains, nil
}

func (r *DomainRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE client_domains SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (r *DomainRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM client_domains WHERE id = $1`, id)
	return err
}
