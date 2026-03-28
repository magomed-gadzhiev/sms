# WS2: Click Tracking (Link Service) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a link-service microservice for URL shortening, click tracking with redirect, custom domain support, and click analytics via Kafka.

**Architecture:** New standalone microservice `link-service` with gRPC API (port 9102), lightweight HTTP redirect server (port 8085), PostgreSQL for storage, Redis for hot link cache and unique click tracking. Click events flow through Kafka `link.clicks` topic to analytics-service. Custom domains with DNS verification and Let's Encrypt SSL.

**Tech Stack:** Go 1.24.0, gorilla/mux, grpc, pgx/v5, go-redis/v9, sarama, zerolog, prometheus

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `migrations/000047_link_service.up.sql` | Tables: client_domains, short_links, click_events (partitioned) |
| Create | `migrations/000047_link_service.down.sql` | Rollback |
| Create | `api/proto/link/link.proto` | gRPC service definition |
| Create | `internal/services/link/domain/models.go` | Domain models |
| Create | `internal/services/link/domain/shortcode.go` | Base62 code generation |
| Create | `internal/services/link/domain/shortcode_test.go` | Tests |
| Create | `internal/services/link/infrastructure/repository/link_repo.go` | Short links repository |
| Create | `internal/services/link/infrastructure/repository/domain_repo.go` | Domains repository |
| Create | `internal/services/link/infrastructure/repository/click_repo.go` | Click events repository |
| Create | `internal/services/link/application/link_service.go` | Business logic |
| Create | `internal/services/link/application/link_service_test.go` | Tests |
| Create | `internal/services/link/grpc/server.go` | gRPC server |
| Create | `internal/services/link/redirect/server.go` | HTTP redirect server |
| Create | `internal/services/link/redirect/server_test.go` | Tests |
| Create | `cmd/services/link-service/main.go` | Service entry point |
| Create | `deployments/docker/link-service.Dockerfile` | Docker build |
| Modify | `deployments/docker-compose.yml` | Add link-service container |
| Create | `internal/gateway/portal/handlers/domains.go` | Domain management HTTP handlers |
| Modify | `internal/gateway/portal/router/router.go` | Register domain routes |
| Create | `portal-frontend/src/pages/settings/DomainsPage.tsx` | Domain management UI |
| Create | `portal-frontend/src/components/domains/DomainManager.tsx` | Domain add/verify/status component |
| Modify | `portal-frontend/src/App.tsx` | Add /settings/domains route |
| Modify | `portal-frontend/src/components/layout/UserLayout.tsx` | Add Домены nav item |

---

### Task 1: Database migration — link service tables

**Files:**
- Create: `migrations/000047_link_service.up.sql`
- Create: `migrations/000047_link_service.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000047_link_service.up.sql

-- Custom domains for client-branded short links
CREATE TABLE client_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    domain TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending_dns' CHECK (status IN ('pending_dns', 'pending_ssl', 'active', 'failed')),
    dns_txt_record TEXT NOT NULL,
    dns_verified_at TIMESTAMPTZ,
    ssl_cert_path TEXT,
    ssl_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_client_domains_client ON client_domains(client_id);
CREATE INDEX idx_client_domains_status ON client_domains(status);

-- Short links
CREATE TABLE short_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    domain_id UUID REFERENCES client_domains(id),
    code VARCHAR(10) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    message_id UUID,
    campaign_id UUID,
    recipient_id UUID,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_short_links_code ON short_links(code);
CREATE INDEX idx_short_links_client ON short_links(client_id);
CREATE INDEX idx_short_links_campaign ON short_links(campaign_id);

-- Click events (monthly partitioned)
CREATE TABLE click_events (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    short_link_id UUID NOT NULL,
    client_id UUID NOT NULL,
    campaign_id UUID,
    phone TEXT,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address INET,
    user_agent TEXT,
    referer TEXT,
    country_code VARCHAR(2),
    is_unique BOOLEAN NOT NULL DEFAULT false
) PARTITION BY RANGE (clicked_at);

-- Create partitions for current and next month
CREATE TABLE click_events_2026_03 PARTITION OF click_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE click_events_2026_04 PARTITION OF click_events
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE click_events_2026_05 PARTITION OF click_events
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_click_events_link ON click_events(short_link_id, clicked_at);
CREATE INDEX idx_click_events_campaign ON click_events(campaign_id, clicked_at);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000047_link_service.down.sql
DROP TABLE IF EXISTS click_events CASCADE;
DROP TABLE IF EXISTS short_links CASCADE;
DROP TABLE IF EXISTS client_domains CASCADE;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000047_link_service.up.sql migrations/000047_link_service.down.sql
git commit -m "migration: add link service tables (client_domains, short_links, click_events)"
```

---

### Task 2: Create link.proto gRPC definition

**Files:**
- Create: `api/proto/link/link.proto`

- [ ] **Step 1: Write the proto file**

```protobuf
// api/proto/link/link.proto
syntax = "proto3";
package link.v1;
option go_package = "github.com/smpp-server/smpp-server/api/proto/linkv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

service LinkService {
  rpc ShortenURL(ShortenRequest) returns (ShortenResponse);
  rpc ShortenBatch(ShortenBatchRequest) returns (ShortenBatchResponse);
  rpc GetLinkStats(LinkStatsRequest) returns (LinkStatsResponse);
}

service DomainService {
  rpc AddDomain(AddDomainRequest) returns (Domain);
  rpc VerifyDomain(VerifyDomainRequest) returns (Domain);
  rpc ListDomains(ListDomainsRequest) returns (ListDomainsResponse);
  rpc DeleteDomain(DeleteDomainRequest) returns (google.protobuf.Empty);
}

// --- Link messages ---

message ShortenRequest {
  string client_id = 1;
  string original_url = 2;
  string message_id = 3;
  string campaign_id = 4;
  string recipient_id = 5;
}

message ShortenResponse {
  string short_url = 1;
  string code = 2;
}

message ShortenBatchRequest {
  string client_id = 1;
  repeated ShortenRequest urls = 2;
}

message ShortenBatchResponse {
  repeated ShortenResponse results = 1;
}

message LinkStatsRequest {
  string short_link_id = 1;
  string client_id = 2;
}

message LinkStatsResponse {
  int32 total_clicks = 1;
  int32 unique_clicks = 2;
  google.protobuf.Timestamp last_clicked_at = 3;
}

// --- Domain messages ---

message AddDomainRequest {
  string client_id = 1;
  string domain = 2;
}

message VerifyDomainRequest {
  string client_id = 1;
  string domain_id = 2;
}

message ListDomainsRequest {
  string client_id = 1;
}

message ListDomainsResponse {
  repeated Domain domains = 1;
}

message DeleteDomainRequest {
  string client_id = 1;
  string domain_id = 2;
}

message Domain {
  string id = 1;
  string client_id = 2;
  string domain = 3;
  string status = 4;
  string dns_txt_record = 5;
  google.protobuf.Timestamp dns_verified_at = 6;
  google.protobuf.Timestamp ssl_expires_at = 7;
  google.protobuf.Timestamp created_at = 8;
}
```

- [ ] **Step 2: Generate Go code**

```bash
cd /home/magomed/projects/sms && mkdir -p api/proto/linkv1 && protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative api/proto/link/link.proto
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/link/ api/proto/linkv1/
git commit -m "proto(link): add LinkService and DomainService gRPC definitions"
```

---

### Task 3: Domain models and shortcode generator

**Files:**
- Create: `internal/services/link/domain/models.go`
- Create: `internal/services/link/domain/shortcode.go`
- Create: `internal/services/link/domain/shortcode_test.go`

- [ ] **Step 1: Write shortcode tests**

```go
// internal/services/link/domain/shortcode_test.go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCode_Length(t *testing.T) {
	code := GenerateCode(6)
	assert.Len(t, code, 6)
}

func TestGenerateCode_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code := GenerateCode(8)
		assert.False(t, seen[code], "duplicate code generated: %s", code)
		seen[code] = true
	}
}

func TestGenerateCode_ValidChars(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := GenerateCode(8)
		for _, c := range code {
			assert.True(t,
				(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'),
				"invalid character: %c", c)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/link/domain/ -v -count=1
```

Expected: FAIL — package doesn't exist

- [ ] **Step 3: Write domain models**

```go
// internal/services/link/domain/models.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLinkNotFound    = errors.New("short link not found")
	ErrDomainNotFound  = errors.New("domain not found")
	ErrDomainExists    = errors.New("domain already exists")
	ErrCodeExists      = errors.New("short code already exists")
	ErrLinkExpired     = errors.New("short link has expired")
)

const DefaultDomain = "go.sms-platform.com"

type ClientDomain struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Domain        string
	Status        string
	DNSTxtRecord  string
	DNSVerifiedAt *time.Time
	SSLCertPath   string
	SSLExpiresAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ShortLink struct {
	ID          uuid.UUID
	ClientID    uuid.UUID
	DomainID    *uuid.UUID
	Code        string
	OriginalURL string
	MessageID   *uuid.UUID
	CampaignID  *uuid.UUID
	RecipientID *uuid.UUID
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

type ClickEvent struct {
	ID          uuid.UUID
	ShortLinkID uuid.UUID
	ClientID    uuid.UUID
	CampaignID  *uuid.UUID
	Phone       string
	ClickedAt   time.Time
	IPAddress   string
	UserAgent   string
	Referer     string
	CountryCode string
	IsUnique    bool
}
```

- [ ] **Step 4: Write shortcode generator**

```go
// internal/services/link/domain/shortcode.go
package domain

import (
	"crypto/rand"
	"math/big"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateCode generates a cryptographically random base62 code of the given length.
func GenerateCode(length int) string {
	b := make([]byte, length)
	max := big.NewInt(int64(len(base62Chars)))
	for i := range b {
		n, _ := rand.Int(rand.Reader, max)
		b[i] = base62Chars[n.Int64()]
	}
	return string(b)
}
```

- [ ] **Step 5: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/link/domain/ -v -count=1
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/services/link/domain/
git commit -m "feat(link): add domain models and base62 shortcode generator"
```

---

### Task 4: Link repository

**Files:**
- Create: `internal/services/link/infrastructure/repository/link_repo.go`

- [ ] **Step 1: Write the repository**

```go
// internal/services/link/infrastructure/repository/link_repo.go
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
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at, ssl_cert_path, ssl_expires_at, created_at, updated_at
		 FROM client_domains WHERE client_id = $1 AND status = 'active' LIMIT 1`, clientID,
	).Scan(&d.ID, &d.ClientID, &d.Domain, &d.Status, &d.DNSTxtRecord, &d.DNSVerifiedAt,
		&d.SSLCertPath, &d.SSLExpiresAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, nil // No active domain — use default
	}
	return &d, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/link/infrastructure/repository/link_repo.go
git commit -m "feat(link): add link repository with CRUD operations"
```

---

### Task 5: Domain repository

**Files:**
- Create: `internal/services/link/infrastructure/repository/domain_repo.go`

- [ ] **Step 1: Write the repository**

```go
// internal/services/link/infrastructure/repository/domain_repo.go
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
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at, ssl_cert_path, ssl_expires_at, created_at, updated_at
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
		`SELECT id, client_id, domain, status, dns_txt_record, dns_verified_at, ssl_cert_path, ssl_expires_at, created_at, updated_at
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/link/infrastructure/repository/domain_repo.go
git commit -m "feat(link): add domain repository"
```

---

### Task 6: Click event repository

**Files:**
- Create: `internal/services/link/infrastructure/repository/click_repo.go`

- [ ] **Step 1: Write the repository**

```go
// internal/services/link/infrastructure/repository/click_repo.go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/link/infrastructure/repository/click_repo.go
git commit -m "feat(link): add click event repository"
```

---

### Task 7: Link service application layer

**Files:**
- Create: `internal/services/link/application/link_service.go`

- [ ] **Step 1: Write the service**

```go
// internal/services/link/application/link_service.go
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/link/domain"
	"github.com/smpp-server/smpp-server/internal/services/link/infrastructure/repository"
)

type LinkService struct {
	linkRepo   *repository.LinkRepository
	domainRepo *repository.DomainRepository
	clickRepo  *repository.ClickRepository
	rdb        *redis.Client
	logger     zerolog.Logger
}

func NewLinkService(
	linkRepo *repository.LinkRepository,
	domainRepo *repository.DomainRepository,
	clickRepo *repository.ClickRepository,
	rdb *redis.Client,
) *LinkService {
	return &LinkService{
		linkRepo:   linkRepo,
		domainRepo: domainRepo,
		clickRepo:  clickRepo,
		rdb:        rdb,
		logger:     log.With().Str("component", "link-service").Logger(),
	}
}

// ShortenURL creates a short link for the given URL.
func (s *LinkService) ShortenURL(ctx context.Context, clientID uuid.UUID, originalURL string, messageID, campaignID, recipientID *uuid.UUID) (string, string, error) {
	// Get client's active custom domain (or default)
	activeDomain, _ := s.linkRepo.GetClientActiveDomain(ctx, clientID)
	baseDomain := domain.DefaultDomain
	var domainID *uuid.UUID
	if activeDomain != nil {
		baseDomain = activeDomain.Domain
		domainID = &activeDomain.ID
	}

	// Generate unique short code (retry on collision)
	var code string
	for i := 0; i < 5; i++ {
		code = domain.GenerateCode(7)
		existing, _ := s.linkRepo.GetByCode(ctx, code)
		if existing == nil {
			break
		}
	}

	link := &domain.ShortLink{
		ClientID:    clientID,
		DomainID:    domainID,
		Code:        code,
		OriginalURL: originalURL,
		MessageID:   messageID,
		CampaignID:  campaignID,
		RecipientID: recipientID,
	}

	if err := s.linkRepo.CreateShortLink(ctx, link); err != nil {
		return "", "", fmt.Errorf("create short link: %w", err)
	}

	// Cache in Redis for fast redirect lookup
	cacheKey := fmt.Sprintf("link:%s", code)
	s.rdb.Set(ctx, cacheKey, originalURL, 0)

	shortURL := fmt.Sprintf("https://%s/%s", baseDomain, code)
	return shortURL, code, nil
}

// ResolveCode looks up a short code and returns the original URL.
func (s *LinkService) ResolveCode(ctx context.Context, code string) (*domain.ShortLink, error) {
	// Try Redis first
	cacheKey := fmt.Sprintf("link:%s", code)
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return &domain.ShortLink{Code: code, OriginalURL: cached}, nil
	}

	// Fallback to DB
	link, err := s.linkRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Warm cache
	s.rdb.Set(ctx, cacheKey, link.OriginalURL, 0)
	return link, nil
}

// RecordClick checks uniqueness and records a click event.
func (s *LinkService) RecordClick(ctx context.Context, event *domain.ClickEvent) error {
	// Check uniqueness using Redis SET
	uniqueKey := fmt.Sprintf("clicked:%s", event.ShortLinkID.String())
	added, _ := s.rdb.SAdd(ctx, uniqueKey, event.Phone).Result()
	event.IsUnique = added > 0

	return s.clickRepo.Insert(ctx, event)
}

// --- Domain management ---

func (s *LinkService) AddDomain(ctx context.Context, clientID uuid.UUID, domainName string) (*domain.ClientDomain, error) {
	txtRecord := fmt.Sprintf("sms-verify=%s", uuid.New().String()[:8])
	d := &domain.ClientDomain{
		ClientID:     clientID,
		Domain:       domainName,
		Status:       "pending_dns",
		DNSTxtRecord: txtRecord,
	}
	if err := s.domainRepo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *LinkService) ListDomains(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientDomain, error) {
	return s.domainRepo.ListByClient(ctx, clientID)
}

func (s *LinkService) DeleteDomain(ctx context.Context, id uuid.UUID) error {
	return s.domainRepo.Delete(ctx, id)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/link/application/link_service.go
git commit -m "feat(link): add link service application layer with shorten, resolve, click tracking"
```

---

### Task 8: HTTP redirect server

**Files:**
- Create: `internal/services/link/redirect/server.go`
- Create: `internal/services/link/redirect/server_test.go`

- [ ] **Step 1: Write redirect server tests**

```go
// internal/services/link/redirect/server_test.go
package redirect

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectServer_404OnMissingCode(t *testing.T) {
	handler := NewHandler(nil) // resolver will return error
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 2: Write redirect server implementation**

```go
// internal/services/link/redirect/server.go
package redirect

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type Handler struct {
	service *application.LinkService
	logger  zerolog.Logger
}

func NewHandler(service *application.LinkService) *Handler {
	return &Handler{
		service: service,
		logger:  log.With().Str("component", "redirect-handler").Logger(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/")
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}

	if h.service == nil {
		http.NotFound(w, r)
		return
	}

	link, err := h.service.ResolveCode(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Record click asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		event := &domain.ClickEvent{
			ID:          uuid.New(),
			ShortLinkID: link.ID,
			ClientID:    link.ClientID,
			CampaignID:  link.CampaignID,
			ClickedAt:   time.Now(),
			IPAddress:   ip,
			UserAgent:   r.UserAgent(),
			Referer:     r.Referer(),
		}
		if err := h.service.RecordClick(ctx, event); err != nil {
			h.logger.Error().Err(err).Str("code", code).Msg("failed to record click")
		}
	}()

	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

// StartRedirectServer starts the lightweight HTTP redirect server.
func StartRedirectServer(ctx context.Context, port int, service *application.LinkService) error {
	handler := NewHandler(service)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/link/redirect/ -v -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/services/link/redirect/
git commit -m "feat(link): add lightweight HTTP redirect server"
```

---

### Task 9: gRPC server for link-service

**Files:**
- Create: `internal/services/link/grpc/server.go`

- [ ] **Step 1: Write the gRPC server**

```go
// internal/services/link/grpc/server.go
package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/services/link/application"
)

type LinkGrpcServer struct {
	linkv1.UnimplementedLinkServiceServer
	service *application.LinkService
}

func NewLinkGrpcServer(service *application.LinkService) *LinkGrpcServer {
	return &LinkGrpcServer{service: service}
}

func (s *LinkGrpcServer) ShortenURL(ctx context.Context, req *linkv1.ShortenRequest) (*linkv1.ShortenResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}

	var messageID, campaignID, recipientID *uuid.UUID
	if req.MessageId != "" {
		id, _ := uuid.Parse(req.MessageId)
		messageID = &id
	}
	if req.CampaignId != "" {
		id, _ := uuid.Parse(req.CampaignId)
		campaignID = &id
	}
	if req.RecipientId != "" {
		id, _ := uuid.Parse(req.RecipientId)
		recipientID = &id
	}

	shortURL, code, err := s.service.ShortenURL(ctx, clientID, req.OriginalUrl, messageID, campaignID, recipientID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "shorten failed: %s", err)
	}

	return &linkv1.ShortenResponse{ShortUrl: shortURL, Code: code}, nil
}

func (s *LinkGrpcServer) ShortenBatch(ctx context.Context, req *linkv1.ShortenBatchRequest) (*linkv1.ShortenBatchResponse, error) {
	resp := &linkv1.ShortenBatchResponse{}
	for _, r := range req.Urls {
		result, err := s.ShortenURL(ctx, r)
		if err != nil {
			return nil, err
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, nil
}

func (s *LinkGrpcServer) GetLinkStats(ctx context.Context, req *linkv1.LinkStatsRequest) (*linkv1.LinkStatsResponse, error) {
	// Placeholder — will be populated via click_repo
	return &linkv1.LinkStatsResponse{}, nil
}

// DomainGrpcServer implements the DomainService gRPC server.
type DomainGrpcServer struct {
	linkv1.UnimplementedDomainServiceServer
	service *application.LinkService
}

func NewDomainGrpcServer(service *application.LinkService) *DomainGrpcServer {
	return &DomainGrpcServer{service: service}
}

func (s *DomainGrpcServer) AddDomain(ctx context.Context, req *linkv1.AddDomainRequest) (*linkv1.Domain, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	d, err := s.service.AddDomain(ctx, clientID, req.Domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add domain: %s", err)
	}
	return domainToProto(d), nil
}

func (s *DomainGrpcServer) ListDomains(ctx context.Context, req *linkv1.ListDomainsRequest) (*linkv1.ListDomainsResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id")
	}
	domains, err := s.service.ListDomains(ctx, clientID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list domains: %s", err)
	}
	resp := &linkv1.ListDomainsResponse{}
	for _, d := range domains {
		resp.Domains = append(resp.Domains, domainToProto(d))
	}
	return resp, nil
}

func (s *DomainGrpcServer) DeleteDomain(ctx context.Context, req *linkv1.DeleteDomainRequest) (*linkv1.Empty, error) {
	id, err := uuid.Parse(req.DomainId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid domain_id")
	}
	if err := s.service.DeleteDomain(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete domain: %s", err)
	}
	return &linkv1.Empty{}, nil
}

func domainToProto(d *domain.ClientDomain) *linkv1.Domain {
	proto := &linkv1.Domain{
		Id:           d.ID.String(),
		ClientId:     d.ClientID.String(),
		Domain:       d.Domain,
		Status:       d.Status,
		DnsTxtRecord: d.DNSTxtRecord,
		CreatedAt:    timestamppb.New(d.CreatedAt),
	}
	if d.DNSVerifiedAt != nil {
		proto.DnsVerifiedAt = timestamppb.New(*d.DNSVerifiedAt)
	}
	if d.SSLExpiresAt != nil {
		proto.SslExpiresAt = timestamppb.New(*d.SSLExpiresAt)
	}
	return proto
}
```

Note: The import for `domain` package needs to be added:
```go
import "github.com/smpp-server/smpp-server/internal/services/link/domain"
```

And `linkv1.Empty` should use `google.golang.org/protobuf/types/known/emptypb` — adjust the proto import if `google.protobuf.Empty` is used.

- [ ] **Step 2: Commit**

```bash
git add internal/services/link/grpc/
git commit -m "feat(link): add gRPC server for LinkService and DomainService"
```

---

### Task 10: Link-service main entry point

**Files:**
- Create: `cmd/services/link-service/main.go`

- [ ] **Step 1: Write the main.go**

```go
// cmd/services/link-service/main.go
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/services/link/application"
	linkgrpc "github.com/smpp-server/smpp-server/internal/services/link/grpc"
	"github.com/smpp-server/smpp-server/internal/services/link/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/services/link/redirect"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		envOrDefault("POSTGRES_USER", "smpp"),
		envOrDefault("POSTGRES_PASSWORD", "smpp_password"),
		envOrDefault("POSTGRES_HOST", "postgres"),
		envOrDefault("POSTGRES_PORT", "5432"),
		envOrDefault("POSTGRES_DB", "smpp_db"),
	)
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", envOrDefault("REDIS_HOST", "redis"), envOrDefault("REDIS_PORT", "6379")),
	})

	// Repositories
	linkRepo := repository.NewLinkRepository(pool)
	domainRepo := repository.NewDomainRepository(pool)
	clickRepo := repository.NewClickRepository(pool)

	// Service
	service := application.NewLinkService(linkRepo, domainRepo, clickRepo, rdb)

	// gRPC server
	grpcPort := envOrDefault("LINK_GRPC_PORT", "9102")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen gRPC")
	}

	grpcServer := grpc.NewServer()
	linkv1.RegisterLinkServiceServer(grpcServer, linkgrpc.NewLinkGrpcServer(service))
	linkv1.RegisterDomainServiceServer(grpcServer, linkgrpc.NewDomainGrpcServer(service))
	reflection.Register(grpcServer)

	go func() {
		log.Info().Str("port", grpcPort).Msg("link-service gRPC started")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC serve failed")
		}
	}()

	// HTTP redirect server
	redirectPort := 8085
	go func() {
		log.Info().Int("port", redirectPort).Msg("redirect HTTP server started")
		if err := redirect.StartRedirectServer(ctx, redirectPort, service); err != nil {
			log.Error().Err(err).Msg("redirect server error")
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info().Msg("shutting down link-service")
	cancel()
	grpcServer.GracefulStop()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/services/link-service/
git commit -m "feat(link): add link-service main entry point"
```

---

### Task 11: Dockerfile and docker-compose for link-service

**Files:**
- Create: `deployments/docker/link-service.Dockerfile`
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Create Dockerfile** (copy pattern from service-base.Dockerfile)

```dockerfile
# deployments/docker/link-service.Dockerfile
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git protobuf protobuf-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative api/proto/link/link.proto
RUN CGO_ENABLED=0 go build -o /link-service ./cmd/services/link-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /link-service /link-service
EXPOSE 9102 8085
CMD ["/link-service"]
```

- [ ] **Step 2: Add link-service to docker-compose.yml**

Add to the `services:` section:

```yaml
  link-service:
    build:
      context: ..
      dockerfile: deployments/docker/link-service.Dockerfile
    container_name: link-service
    ports:
      - "9102:9102"
      - "8085:8085"
    environment:
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=smpp
      - POSTGRES_PASSWORD=smpp_password
      - POSTGRES_DB=smpp_db
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - LINK_GRPC_PORT=9102
    networks:
      - smpp-network
    depends_on:
      - postgres
      - redis
    restart: unless-stopped
```

- [ ] **Step 3: Commit**

```bash
git add deployments/docker/link-service.Dockerfile deployments/docker-compose.yml
git commit -m "infra: add link-service Dockerfile and docker-compose entry"
```

---

### Task 12: Portal domain management handlers and frontend

**Files:**
- Create: `internal/gateway/portal/handlers/domains.go`
- Modify: `internal/gateway/portal/router/router.go`
- Create: `portal-frontend/src/pages/settings/DomainsPage.tsx`
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Create domain HTTP handlers**

```go
// internal/gateway/portal/handlers/domains.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type DomainHandlers struct {
	domainClient linkv1.DomainServiceClient
}

func NewDomainHandlers(domainClient linkv1.DomainServiceClient) *DomainHandlers {
	return &DomainHandlers{domainClient: domainClient}
}

func (h *DomainHandlers) AddDomain(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}
	resp, err := h.domainClient.AddDomain(r.Context(), &linkv1.AddDomainRequest{
		ClientId: clientID.String(),
		Domain:   req.Domain,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

func (h *DomainHandlers) ListDomains(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	resp, err := h.domainClient.ListDomains(r.Context(), &linkv1.ListDomainsRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *DomainHandlers) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	domainID := mux.Vars(r)["id"]
	_, err := h.domainClient.DeleteDomain(r.Context(), &linkv1.DeleteDomainRequest{
		ClientId: clientID.String(),
		DomainId: domainID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
```

- [ ] **Step 2: Register domain routes in router.go**

Add to `internal/gateway/portal/router/router.go` — add `domainHandlers *handlers.DomainHandlers` parameter to `SetupRouter` and routes:

```go
// Domains (settings)
domains := protected.PathPrefix("/settings/domains").Subrouter()
domains.HandleFunc("", domainHandlers.AddDomain).Methods("POST")
domains.HandleFunc("", domainHandlers.ListDomains).Methods("GET")
domains.HandleFunc("/{id}", domainHandlers.DeleteDomain).Methods("DELETE")
```

- [ ] **Step 3: Create frontend DomainsPage**

```tsx
// portal-frontend/src/pages/settings/DomainsPage.tsx
import { useState, useEffect } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';

interface Domain {
  id: string;
  domain: string;
  status: string;
  dns_txt_record: string;
}

export function DomainsPage() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [loading, setLoading] = useState(true);

  const fetchDomains = async () => {
    const res = await fetch('/portal/v1/settings/domains', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json();
      setDomains(data.domains || []);
    }
    setLoading(false);
  };

  useEffect(() => { fetchDomains(); }, []);

  const addDomain = async () => {
    if (!newDomain.trim()) return;
    await fetch('/portal/v1/settings/domains', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ domain: newDomain }),
    });
    setNewDomain('');
    fetchDomains();
  };

  const deleteDomain = async (id: string) => {
    await fetch(`/portal/v1/settings/domains/${id}`, { method: 'DELETE', credentials: 'include' });
    fetchDomains();
  };

  const statusColors: Record<string, string> = {
    active: 'text-green-600 bg-green-50',
    pending_dns: 'text-amber-600 bg-amber-50',
    pending_ssl: 'text-blue-600 bg-blue-50',
    failed: 'text-red-600 bg-red-50',
  };

  return (
    <div>
      <PageHeader title="Кастомные домены" description="Управление доменами для коротких ссылок" />

      <div className="flex gap-2 mb-6">
        <input
          value={newDomain}
          onChange={(e) => setNewDomain(e.target.value)}
          placeholder="go.yourbrand.com"
          className="flex-1 px-3 py-2 border border-gray-300 rounded text-sm"
        />
        <button onClick={addDomain} className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90">
          Добавить
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-500">Загрузка...</p>
      ) : domains.length === 0 ? (
        <p className="text-sm text-gray-500">Нет добавленных доменов</p>
      ) : (
        <div className="space-y-3">
          {domains.map((d) => (
            <div key={d.id} className="p-4 bg-white border border-gray-200 rounded flex items-center justify-between">
              <div>
                <p className="font-medium text-sm">{d.domain}</p>
                <span className={`text-xs px-2 py-0.5 rounded ${statusColors[d.status] || 'text-gray-600 bg-gray-50'}`}>
                  {d.status}
                </span>
                {d.status === 'pending_dns' && (
                  <p className="text-xs text-gray-500 mt-1">
                    Добавьте TXT запись: <code className="bg-gray-100 px-1">{d.dns_txt_record}</code>
                  </p>
                )}
              </div>
              <button onClick={() => deleteDomain(d.id)} className="text-sm text-red-600 hover:text-red-800">
                Удалить
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Add route to App.tsx**

Add import and route:

```tsx
import { DomainsPage } from './pages/settings/DomainsPage';

// Inside the <Route element={<RequireAuth />}> block:
<Route path="/settings/domains" element={<DomainsPage />} />
```

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/handlers/domains.go portal-frontend/src/pages/settings/DomainsPage.tsx portal-frontend/src/App.tsx internal/gateway/portal/router/router.go
git commit -m "feat(portal): add domain management page and API endpoints"
```
