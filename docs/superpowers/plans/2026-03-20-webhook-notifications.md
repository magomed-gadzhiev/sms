# Webhook Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add webhook notification delivery for SMS status events (delivered, failed, expired, rejected) as a new Webhook Service microservice.

**Architecture:** New `webhook-service` microservice following existing DDD patterns (application/domain/grpc/infrastructure layers). Consumes existing Kafka DLR/failed topics, enriches events via messages table lookup, delivers HTTP POST callbacks to client-registered endpoints with HMAC-SHA256 signing and in-process retry.

**Tech Stack:** Go 1.24, PostgreSQL, Kafka (Sarama), gRPC, gorilla/mux, zerolog, Prometheus

**Spec:** `docs/superpowers/specs/2026-03-20-webhook-notifications-design.md`

---

## File Map

### New files (Webhook Service)
| File | Responsibility |
|------|---------------|
| `api/proto/webhook/webhook.proto` | gRPC service definition |
| `api/proto/webhookv1/webhook.pb.go` | Generated protobuf (via protoc) |
| `api/proto/webhookv1/webhook_grpc.pb.go` | Generated gRPC stubs (via protoc) |
| `migrations/000008_create_webhook_tables.up.sql` | Create webhook_subscriptions table |
| `migrations/000008_create_webhook_tables.down.sql` | Drop webhook_subscriptions table |
| `internal/services/webhook/domain/models.go` | Subscription, WebhookEvent domain models |
| `internal/services/webhook/infrastructure/repository/subscription_repository.go` | Subscription CRUD in PostgreSQL |
| `internal/services/webhook/infrastructure/repository/message_repository.go` | Read-only message lookup |
| `internal/services/webhook/infrastructure/http/delivery_client.go` | HTTP client for webhook delivery + HMAC signing |
| `internal/services/webhook/application/webhook_service.go` | Subscription CRUD business logic |
| `internal/services/webhook/application/delivery_service.go` | Kafka consumer → enrichment → delivery → retry |
| `internal/services/webhook/grpc/server.go` | gRPC server implementing WebhookService |
| `cmd/services/webhook-service/main.go` | Service entrypoint |

### New files (Gateway handlers)
| File | Responsibility |
|------|---------------|
| `internal/gateway/client/handlers/webhooks.go` | Client gateway HTTP handlers for /api/v1/webhooks |
| `internal/gateway/admin/handlers/webhooks.go` | Admin gateway HTTP handlers for /admin/v1/webhooks |

### Modified files
| File | Change |
|------|--------|
| `internal/gateway/client/clients.go` | Add WebhookClient field + connection |
| `internal/gateway/client/router/router.go` | Add webhookHandlers param + routes |
| `internal/gateway/client/handlers/common.go` | Add `codes.ResourceExhausted` → HTTP 429 mapping |
| `cmd/client-gateway/main.go` | Add WEBHOOK_SERVICE_ADDR, create webhook handlers |
| `internal/gateway/admin/clients.go` | Add WebhookClient field + connection |
| `internal/gateway/admin/router/router.go` | Add webhookHandlers param + routes |
| `internal/gateway/admin/handlers/common.go` | Add `codes.ResourceExhausted` → HTTP 429 mapping |
| `cmd/admin-gateway/main.go` | Add WEBHOOK_SERVICE_ADDR, create webhook handlers |
| `deployments/docker-compose.yml` | Add webhook-service, add env vars to gateways |

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/000008_create_webhook_tables.up.sql`
- Create: `migrations/000008_create_webhook_tables.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000008_create_webhook_tables.up.sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL,
    secret TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_webhook_subscriptions_client_id ON webhook_subscriptions(client_id);
CREATE INDEX idx_webhook_subscriptions_active ON webhook_subscriptions(client_id, active) WHERE active = true;

CREATE TRIGGER update_webhook_subscriptions_updated_at BEFORE UPDATE ON webhook_subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000008_create_webhook_tables.down.sql
DROP TRIGGER IF EXISTS update_webhook_subscriptions_updated_at ON webhook_subscriptions;
DROP TABLE IF EXISTS webhook_subscriptions;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000008_create_webhook_tables.up.sql migrations/000008_create_webhook_tables.down.sql
git commit -m "feat(webhook): add database migration for webhook_subscriptions table"
```

---

## Task 2: Proto Definition + Code Generation

**Files:**
- Create: `api/proto/webhook/webhook.proto`
- Create: `api/proto/webhookv1/` (generated)

- [ ] **Step 1: Create proto file**

```protobuf
// api/proto/webhook/webhook.proto
syntax = "proto3";

package webhook.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/webhookv1";

import "google/protobuf/timestamp.proto";

service WebhookService {
  // CreateSubscription creates a new webhook subscription
  rpc CreateSubscription(CreateSubscriptionRequest) returns (CreateSubscriptionResponse);

  // UpdateSubscription updates an existing subscription
  rpc UpdateSubscription(UpdateSubscriptionRequest) returns (UpdateSubscriptionResponse);

  // DeleteSubscription removes a subscription
  rpc DeleteSubscription(DeleteSubscriptionRequest) returns (DeleteSubscriptionResponse);

  // GetSubscription returns subscription details
  rpc GetSubscription(GetSubscriptionRequest) returns (GetSubscriptionResponse);

  // ListSubscriptions lists subscriptions for a client
  rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
}

message CreateSubscriptionRequest {
  string client_id = 1;
  string url = 2;
  repeated string event_types = 3;
}

message CreateSubscriptionResponse {
  SubscriptionInfo subscription = 1;
  string secret = 2; // returned only on creation
}

message UpdateSubscriptionRequest {
  string id = 1;
  string client_id = 2;
  string url = 3;
  repeated string event_types = 4;
  optional bool active = 5; // optional to distinguish "not sent" from "set to false"
}

message UpdateSubscriptionResponse {
  SubscriptionInfo subscription = 1;
}

message DeleteSubscriptionRequest {
  string id = 1;
  string client_id = 2;
}

message DeleteSubscriptionResponse {
  bool success = 1;
}

message GetSubscriptionRequest {
  string id = 1;
  string client_id = 2;
}

message GetSubscriptionResponse {
  SubscriptionInfo subscription = 1;
}

message ListSubscriptionsRequest {
  string client_id = 1;
}

message ListSubscriptionsResponse {
  repeated SubscriptionInfo subscriptions = 1;
}

message SubscriptionInfo {
  string id = 1;
  string client_id = 2;
  string url = 3;
  repeated string event_types = 4;
  bool active = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
}
```

- [ ] **Step 2: Generate Go code**

Run:
```bash
mkdir -p api/proto/webhookv1
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  -I api/proto \
  api/proto/webhook/webhook.proto
```

If `protoc` is not available, manually create the output directory and generate later. The generated files go to `api/proto/webhookv1/`.

Alternatively, check how existing protos were generated:
```bash
ls api/proto/billingv1/
```
Follow the same generation pattern.

- [ ] **Step 3: Commit**

```bash
git add api/proto/webhook/ api/proto/webhookv1/
git commit -m "feat(webhook): add protobuf definition and generated code"
```

---

## Task 3: Domain Models

**Files:**
- Create: `internal/services/webhook/domain/models.go`

- [ ] **Step 1: Create domain models**

```go
// internal/services/webhook/domain/models.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSubscriptionNotFound   = errors.New("subscription not found")
	ErrMaxSubscriptionsReached = errors.New("maximum subscriptions per client reached")
	ErrInvalidURL             = errors.New("invalid webhook URL: must be HTTPS")
	ErrInvalidEventType       = errors.New("invalid event type")
)

var ValidEventTypes = map[string]bool{
	"delivered": true,
	"failed":    true,
	"expired":   true,
	"rejected":  true,
}

const MaxSubscriptionsPerClient = 10

// Subscription represents a webhook subscription
type Subscription struct {
	ID         uuid.UUID
	ClientID   uuid.UUID
	URL        string
	EventTypes []string
	Secret     string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WebhookEvent represents an event to be delivered via webhook
type WebhookEvent struct {
	EventID     string    `json:"event_id"`
	EventType   string    `json:"event_type"`
	Timestamp   time.Time `json:"timestamp"`
	Data        EventData `json:"data"`
}

// EventData contains the message delivery status data
type EventData struct {
	MessageID     string     `json:"message_id"`
	ExternalID    string     `json:"external_id,omitempty"`
	Source        string     `json:"source"`
	Destination   string     `json:"destination"`
	Status        string     `json:"status"`
	StatusMessage string     `json:"status_message,omitempty"`
	ProviderID    string     `json:"provider_id,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	FailedAt      *time.Time `json:"failed_at,omitempty"`
}

// SMPPStatToEventType maps SMPP DLR stat values to webhook event types
var SMPPStatToEventType = map[string]string{
	"DELIVRD": "delivered",
	"UNDELIV": "failed",
	"EXPIRED": "expired",
	"DELETED": "failed",
	"REJECTD": "rejected",
	"UNKNOWN": "failed",
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/domain/models.go
git commit -m "feat(webhook): add domain models and error definitions"
```

---

## Task 4: Subscription Repository

**Files:**
- Create: `internal/services/webhook/infrastructure/repository/subscription_repository.go`

- [ ] **Step 1: Create subscription repository**

```go
// internal/services/webhook/infrastructure/repository/subscription_repository.go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/infrastructure/repository/subscription_repository.go
git commit -m "feat(webhook): add subscription repository"
```

---

## Task 5: Message Repository (Read-Only Enrichment)

**Files:**
- Create: `internal/services/webhook/infrastructure/repository/message_repository.go`

- [ ] **Step 1: Create message repository**

```go
// internal/services/webhook/infrastructure/repository/message_repository.go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// MessageEnrichment contains fields needed to build webhook payload
type MessageEnrichment struct {
	ClientID    *uuid.UUID
	ExternalID  string
	Source      string
	Destination string
	SubmittedAt *time.Time
}

type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) GetEnrichment(ctx context.Context, messageID uuid.UUID) (*MessageEnrichment, error) {
	query := `SELECT client_id, external_id, source, destination, submitted_at
		FROM messages WHERE id = $1 LIMIT 1`

	var clientID *uuid.UUID
	var externalID sql.NullString
	var source, destination string
	var submittedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&clientID, &externalID, &source, &destination, &submittedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // message not found, caller handles this
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message enrichment: %w", err)
	}

	enrichment := &MessageEnrichment{
		ClientID:    clientID,
		Source:      source,
		Destination: destination,
	}
	if externalID.Valid {
		enrichment.ExternalID = externalID.String
	}
	if submittedAt.Valid {
		enrichment.SubmittedAt = &submittedAt.Time
	}
	return enrichment, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/infrastructure/repository/message_repository.go
git commit -m "feat(webhook): add read-only message repository for enrichment"
```

---

## Task 6: HTTP Delivery Client with HMAC Signing

**Files:**
- Create: `internal/services/webhook/infrastructure/http/delivery_client.go`

- [ ] **Step 1: Create delivery client**

```go
// internal/services/webhook/infrastructure/http/delivery_client.go
package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type DeliveryClient struct {
	httpClient *http.Client
}

func NewDeliveryClient(timeout time.Duration) *DeliveryClient {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &DeliveryClient{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // do not follow redirects (SSRF prevention)
			},
		},
	}
}

// UPDATED 2026-05-02: replay-protected variant — signature теперь включает
// X-Webhook-Timestamp в HMAC. Этот блок ниже — историческая версия плана,
// не копировать как reference. Актуальный код:
// internal/services/webhook/infrastructure/http/delivery_client.go.

// Deliver sends a webhook event to the subscription URL
func (c *DeliveryClient) Deliver(ctx context.Context, sub *domain.Subscription, event *domain.WebhookEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Sign payload with HMAC-SHA256
	signature := signPayload(payload, sub.Secret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SMS-Platform-Webhook/1.0")
	req.Header.Set("X-Webhook-Signature", "sha256="+signature)
	req.Header.Set("X-Webhook-Event", event.EventType)
	req.Header.Set("X-Webhook-ID", event.EventID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deliver webhook: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain body for connection reuse

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook delivery failed: HTTP %d", resp.StatusCode)
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateURL checks that a URL is HTTPS and not targeting private IPs
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return domain.ErrInvalidURL
	}
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL exceeds maximum length of 2048 characters")
	}

	host := u.Hostname()
	// Block private/internal ranges
	privateHosts := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, ph := range privateHosts {
		if strings.EqualFold(host, ph) {
			return fmt.Errorf("private/internal URLs are not allowed")
		}
	}
	// Block 10.x, 172.16-31.x, 192.168.x
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return fmt.Errorf("private/internal IP addresses are not allowed")
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/infrastructure/http/delivery_client.go
git commit -m "feat(webhook): add HTTP delivery client with HMAC-SHA256 signing"
```

---

## Task 7: Webhook Application Service (Subscription CRUD)

**Files:**
- Create: `internal/services/webhook/application/webhook_service.go`

- [ ] **Step 1: Create webhook service**

```go
// internal/services/webhook/application/webhook_service.go
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	webhookhttp "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/http"
	"github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
)

// CacheInvalidator allows WebhookService to invalidate the delivery cache on CRUD ops
type CacheInvalidator interface {
	InvalidateCache(clientID uuid.UUID)
}

type WebhookService struct {
	subRepo          *repository.SubscriptionRepository
	cacheInvalidator CacheInvalidator
	logger           zerolog.Logger
}

func NewWebhookService(subRepo *repository.SubscriptionRepository, cacheInvalidator CacheInvalidator) *WebhookService {
	return &WebhookService{
		subRepo:          subRepo,
		cacheInvalidator: cacheInvalidator,
		logger:           log.With().Str("component", "webhook-service").Logger(),
	}
}

func (s *WebhookService) CreateSubscription(ctx context.Context, clientID uuid.UUID, url string, eventTypes []string) (*domain.Subscription, error) {
	// Validate URL
	if err := webhookhttp.ValidateURL(url); err != nil {
		return nil, err
	}

	// Validate event types
	for _, et := range eventTypes {
		if !domain.ValidEventTypes[et] {
			return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, et)
		}
	}

	// Check limit
	count, err := s.subRepo.CountByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if count >= domain.MaxSubscriptionsPerClient {
		return nil, domain.ErrMaxSubscriptionsReached
	}

	// Generate secret (32 bytes = 64 hex chars)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)

	sub := &domain.Subscription{
		ID:         uuid.New(),
		ClientID:   clientID,
		URL:        url,
		EventTypes: eventTypes,
		Secret:     secret,
		Active:     true,
	}

	created, err := s.subRepo.Create(ctx, sub)
	if err != nil {
		return nil, err
	}
	s.cacheInvalidator.InvalidateCache(clientID)
	subscriptionsTotal.Inc()
	// Preserve the secret for the response (repo returns it but caller needs it)
	created.Secret = secret
	return created, nil
}

func (s *WebhookService) GetSubscription(ctx context.Context, id, clientID uuid.UUID) (*domain.Subscription, error) {
	sub, err := s.subRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	// Clear secret - not returned after creation
	sub.Secret = ""
	return sub, nil
}

func (s *WebhookService) ListSubscriptions(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	subs, err := s.subRepo.ListByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	// Clear secrets
	for _, sub := range subs {
		sub.Secret = ""
	}
	return subs, nil
}

func (s *WebhookService) UpdateSubscription(ctx context.Context, id, clientID uuid.UUID, url *string, eventTypes []string, active *bool) (*domain.Subscription, error) {
	existing, err := s.subRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}

	if url != nil {
		if err := webhookhttp.ValidateURL(*url); err != nil {
			return nil, err
		}
		existing.URL = *url
	}
	if eventTypes != nil {
		for _, et := range eventTypes {
			if !domain.ValidEventTypes[et] {
				return nil, fmt.Errorf("%w: %s", domain.ErrInvalidEventType, et)
			}
		}
		existing.EventTypes = eventTypes
	}
	if active != nil {
		existing.Active = *active
	}

	updated, err := s.subRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	s.cacheInvalidator.InvalidateCache(clientID)
	updated.Secret = ""
	return updated, nil
}

func (s *WebhookService) DeleteSubscription(ctx context.Context, id, clientID uuid.UUID) error {
	if err := s.subRepo.Delete(ctx, id, clientID); err != nil {
		return err
	}
	s.cacheInvalidator.InvalidateCache(clientID)
	subscriptionsTotal.Dec()
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/application/webhook_service.go
git commit -m "feat(webhook): add webhook application service for subscription CRUD"
```

---

## Task 8: Delivery Service (Kafka Consumer + Enrichment + Retry)

**Files:**
- Create: `internal/services/webhook/application/delivery_service.go`

- [ ] **Step 1: Create delivery service**

```go
// internal/services/webhook/application/delivery_service.go
package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	webhookhttp "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/http"
	"github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
)

var (
	deliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_deliveries_total",
		Help: "Total webhook delivery attempts",
	}, []string{"event_type", "status"})

	deliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webhook_delivery_duration_seconds",
		Help:    "Webhook HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"event_type"})

	retryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_retry_total",
		Help: "Webhook retry attempts",
	}, []string{"attempt"})

	deliveryFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_delivery_failed",
		Help: "Webhooks that exhausted all retries",
	}, []string{"event_type"})

	workerPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_worker_pool_size",
		Help: "Current worker pool utilization",
	})

	subscriptionsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_subscriptions_total",
		Help: "Active subscriptions count",
	})
)

var retryBackoff = []time.Duration{15 * time.Second, 30 * time.Second, 1 * time.Minute, 5 * time.Minute, 15 * time.Minute}

type DeliveryService struct {
	subRepo    *repository.SubscriptionRepository
	msgRepo    *repository.MessageRepository
	httpClient *webhookhttp.DeliveryClient
	logger     zerolog.Logger

	// In-memory subscription cache
	cacheMu  sync.RWMutex
	cache    map[uuid.UUID]cachedSubs
	cacheTTL time.Duration

	// Worker pool
	workCh chan deliveryJob
	wg     sync.WaitGroup
}

type cachedSubs struct {
	subs      []*domain.Subscription
	expiresAt time.Time
}

type deliveryJob struct {
	sub   *domain.Subscription
	event *domain.WebhookEvent
}

func NewDeliveryService(
	subRepo *repository.SubscriptionRepository,
	msgRepo *repository.MessageRepository,
	httpClient *webhookhttp.DeliveryClient,
	workerPoolSize int,
) *DeliveryService {
	ds := &DeliveryService{
		subRepo:    subRepo,
		msgRepo:    msgRepo,
		httpClient: httpClient,
		logger:     log.With().Str("component", "webhook-delivery").Logger(),
		cache:      make(map[uuid.UUID]cachedSubs),
		cacheTTL:   60 * time.Second,
		workCh:     make(chan deliveryJob, workerPoolSize*2),
	}

	// Start worker pool
	for i := 0; i < workerPoolSize; i++ {
		ds.wg.Add(1)
		go ds.worker()
	}

	return ds
}

func (ds *DeliveryService) worker() {
	defer ds.wg.Done()
	for job := range ds.workCh {
		workerPoolSize.Inc()
		ds.deliverWithRetry(job.sub, job.event, 0)
		workerPoolSize.Dec()
	}
}

func (ds *DeliveryService) deliverWithRetry(sub *domain.Subscription, event *domain.WebhookEvent, attempt int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := ds.httpClient.Deliver(ctx, sub, event)
	duration := time.Since(start)

	deliveryDuration.WithLabelValues(event.EventType).Observe(duration.Seconds())

	if err == nil {
		deliveriesTotal.WithLabelValues(event.EventType, "success").Inc()
		return
	}

	deliveriesTotal.WithLabelValues(event.EventType, "failed").Inc()

	if attempt >= len(retryBackoff) {
		deliveryFailed.WithLabelValues(event.EventType).Inc()
		ds.logger.Error().
			Err(err).
			Str("subscription_id", sub.ID.String()).
			Str("url", sub.URL).
			Str("event_type", event.EventType).
			Int("attempts", attempt+1).
			Msg("webhook delivery exhausted all retries")
		return
	}

	retryTotal.WithLabelValues(fmt.Sprintf("%d", attempt+1)).Inc()
	delay := retryBackoff[attempt]

	ds.logger.Warn().
		Err(err).
		Str("subscription_id", sub.ID.String()).
		Dur("retry_after", delay).
		Int("attempt", attempt+1).
		Msg("webhook delivery failed, scheduling retry")

	time.AfterFunc(delay, func() {
		ds.deliverWithRetry(sub, event, attempt+1)
	})
}

// HandleDLR processes a DLR Kafka message
func (ds *DeliveryService) HandleDLR(ctx context.Context, dlr *queue.DLRMessage) error {
	eventType, ok := domain.SMPPStatToEventType[dlr.Stat]
	if !ok {
		// ACCEPTED and other non-final states are ignored
		return nil
	}

	enrichment, err := ds.msgRepo.GetEnrichment(ctx, dlr.MessageID)
	if err != nil {
		ds.logger.Error().Err(err).Str("message_id", dlr.MessageID.String()).Msg("failed to enrich DLR")
		return nil // don't block consumer on enrichment errors
	}
	if enrichment == nil || enrichment.ClientID == nil {
		ds.logger.Warn().Str("message_id", dlr.MessageID.String()).Msg("message not found for DLR, skipping")
		return nil
	}

	// Prefer enriched values from DB (always populated); DLR fields may be empty (omitempty)
	source := enrichment.Source
	if source == "" {
		source = dlr.Source
	}
	destination := enrichment.Destination
	if destination == "" {
		destination = dlr.Destination
	}

	event := &domain.WebhookEvent{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Timestamp: time.Now(),
		Data: domain.EventData{
			MessageID:   dlr.MessageID.String(),
			ExternalID:  enrichment.ExternalID,
			Source:      source,
			Destination: destination,
			Status:      eventType,
		},
	}

	if enrichment.SubmittedAt != nil {
		event.Data.SubmittedAt = enrichment.SubmittedAt
	}
	if dlr.ProviderID != nil {
		event.Data.ProviderID = dlr.ProviderID.String()
	}
	if dlr.DoneDate != nil {
		if eventType == "delivered" {
			event.Data.DeliveredAt = dlr.DoneDate
		} else {
			event.Data.FailedAt = dlr.DoneDate
		}
	}
	if dlr.Text != "" {
		event.Data.StatusMessage = dlr.Text
	}

	ds.dispatchToSubscriptions(ctx, *enrichment.ClientID, eventType, event)
	return nil
}

// HandleFailed processes a failed message from Kafka
func (ds *DeliveryService) HandleFailed(ctx context.Context, failed *queue.FailedMessage) error {
	enrichment, err := ds.msgRepo.GetEnrichment(ctx, failed.MessageID)
	if err != nil {
		ds.logger.Error().Err(err).Str("message_id", failed.MessageID.String()).Msg("failed to enrich failed message")
		return nil
	}
	if enrichment == nil || enrichment.ClientID == nil {
		ds.logger.Warn().Str("message_id", failed.MessageID.String()).Msg("message not found for failed event, skipping")
		return nil
	}

	event := &domain.WebhookEvent{
		EventID:   uuid.New().String(),
		EventType: "failed",
		Timestamp: time.Now(),
		Data: domain.EventData{
			MessageID:     failed.MessageID.String(),
			Source:        enrichment.Source,
			Destination:   enrichment.Destination,
			Status:        "failed",
			StatusMessage: failed.Error,
			FailedAt:      &failed.FailedAt,
		},
	}
	if enrichment.ExternalID != "" {
		event.Data.ExternalID = enrichment.ExternalID
	}
	if enrichment.SubmittedAt != nil {
		event.Data.SubmittedAt = enrichment.SubmittedAt
	}

	ds.dispatchToSubscriptions(ctx, *enrichment.ClientID, "failed", event)
	return nil
}

func (ds *DeliveryService) dispatchToSubscriptions(ctx context.Context, clientID uuid.UUID, eventType string, event *domain.WebhookEvent) {
	subs, err := ds.getSubscriptions(ctx, clientID)
	if err != nil {
		ds.logger.Error().Err(err).Str("client_id", clientID.String()).Msg("failed to get subscriptions")
		return
	}

	for _, sub := range subs {
		if !sub.Active {
			continue
		}
		// Check if subscription is interested in this event type
		matched := false
		for _, et := range sub.EventTypes {
			if et == eventType {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		select {
		case ds.workCh <- deliveryJob{sub: sub, event: event}:
		default:
			ds.logger.Warn().
				Str("subscription_id", sub.ID.String()).
				Msg("worker pool full, dropping webhook delivery")
		}
	}
}

func (ds *DeliveryService) getSubscriptions(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error) {
	ds.cacheMu.RLock()
	if cached, ok := ds.cache[clientID]; ok && time.Now().Before(cached.expiresAt) {
		ds.cacheMu.RUnlock()
		return cached.subs, nil
	}
	ds.cacheMu.RUnlock()

	subs, err := ds.subRepo.ListActiveByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	ds.cacheMu.Lock()
	ds.cache[clientID] = cachedSubs{
		subs:      subs,
		expiresAt: time.Now().Add(ds.cacheTTL),
	}
	ds.cacheMu.Unlock()

	return subs, nil
}

// InvalidateCache removes cached subscriptions for a client (called by WebhookService on CRUD)
func (ds *DeliveryService) InvalidateCache(clientID uuid.UUID) {
	ds.cacheMu.Lock()
	delete(ds.cache, clientID)
	ds.cacheMu.Unlock()
}

// Close drains the worker pool
func (ds *DeliveryService) Close() {
	close(ds.workCh)
	ds.wg.Wait()
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/application/delivery_service.go
git commit -m "feat(webhook): add delivery service with Kafka handling, enrichment, and retry"
```

---

## Task 9: gRPC Server

**Files:**
- Create: `internal/services/webhook/grpc/server.go`

- [ ] **Step 1: Create gRPC server**

```go
// internal/services/webhook/grpc/server.go
package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type Server struct {
	webhookv1.UnimplementedWebhookServiceServer
	webhookService *application.WebhookService
	logger         zerolog.Logger
}

func NewServer(webhookService *application.WebhookService) *Server {
	return &Server{
		webhookService: webhookService,
		logger:         log.With().Str("component", "webhook-grpc-server").Logger(),
	}
}

func (s *Server) CreateSubscription(ctx context.Context, req *webhookv1.CreateSubscriptionRequest) (*webhookv1.CreateSubscriptionResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}
	if len(req.EventTypes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "event_types is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	sub, err := s.webhookService.CreateSubscription(ctx, clientID, req.Url, req.EventTypes)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.CreateSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
		Secret:       sub.Secret,
	}, nil
}

func (s *Server) GetSubscription(ctx context.Context, req *webhookv1.GetSubscriptionRequest) (*webhookv1.GetSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	sub, err := s.webhookService.GetSubscription(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.GetSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
	}, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, req *webhookv1.ListSubscriptionsRequest) (*webhookv1.ListSubscriptionsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	subs, err := s.webhookService.ListSubscriptions(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}

	protoSubs := make([]*webhookv1.SubscriptionInfo, len(subs))
	for i, sub := range subs {
		protoSubs[i] = subscriptionToProto(sub)
	}

	return &webhookv1.ListSubscriptionsResponse{
		Subscriptions: protoSubs,
	}, nil
}

func (s *Server) UpdateSubscription(ctx context.Context, req *webhookv1.UpdateSubscriptionRequest) (*webhookv1.UpdateSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	var urlPtr *string
	if req.Url != "" {
		urlPtr = &req.Url
	}
	var activePtr *bool
	if req.Active != nil {
		activePtr = req.Active
	}

	var eventTypes []string
	if len(req.EventTypes) > 0 {
		eventTypes = req.EventTypes
	}

	sub, err := s.webhookService.UpdateSubscription(ctx, id, clientID, urlPtr, eventTypes, activePtr)
	if err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.UpdateSubscriptionResponse{
		Subscription: subscriptionToProto(sub),
	}, nil
}

func (s *Server) DeleteSubscription(ctx context.Context, req *webhookv1.DeleteSubscriptionRequest) (*webhookv1.DeleteSubscriptionResponse, error) {
	id, clientID, err := s.parseIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.webhookService.DeleteSubscription(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}

	return &webhookv1.DeleteSubscriptionResponse{Success: true}, nil
}

func (s *Server) parseIDs(idStr, clientIDStr string) (uuid.UUID, uuid.UUID, error) {
	if idStr == "" {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if clientIDStr == "" {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid id format")
	}
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}
	return id, clientID, nil
}

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrMaxSubscriptionsReached):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, domain.ErrInvalidURL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidEventType):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}

func subscriptionToProto(sub *domain.Subscription) *webhookv1.SubscriptionInfo {
	return &webhookv1.SubscriptionInfo{
		Id:         sub.ID.String(),
		ClientId:   sub.ClientID.String(),
		Url:        sub.URL,
		EventTypes: sub.EventTypes,
		Active:     sub.Active,
		CreatedAt:  timestamppb.New(sub.CreatedAt),
		UpdatedAt:  timestamppb.New(sub.UpdatedAt),
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/webhook/grpc/server.go
git commit -m "feat(webhook): add gRPC server for webhook subscription management"
```

---

## Task 10: Webhook Service Main Entrypoint

**Files:**
- Create: `cmd/services/webhook-service/main.go`

- [ ] **Step 1: Create main.go**

Follow the same pattern as `cmd/services/billing-service/main.go`:

```go
// cmd/services/webhook-service/main.go
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	webhookgrpc "github.com/smpp-server/smpp-server/internal/services/webhook/grpc"
	webhookhttp "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/http"
	webhookrepo "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("webhook-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Webhook Service")

	// Wait for database
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	dbConn, err := database.NewDBWithConfig(database.Config{
		DSN:             cfg.Database.GetDSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer dbConn.Close()
	logger.Info().Msg("подключение к базе данных установлено")

	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// Wait for Kafka
	logger.Info().Msg("ожидание готовности Kafka")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka недоступен")
	}

	// Repositories
	subRepo := webhookrepo.NewSubscriptionRepository(dbx)
	msgRepo := webhookrepo.NewMessageRepository(dbx)

	// HTTP delivery client
	httpClient := webhookhttp.NewDeliveryClient(5 * time.Second)

	// Application services
	deliveryService := application.NewDeliveryService(subRepo, msgRepo, httpClient, 50)
	webhookService := application.NewWebhookService(subRepo, deliveryService)
	defer deliveryService.Close()

	// Kafka consumer
	kafkaCfg := cfg.Kafka
	kafkaCfg.ConsumerGroup = "webhook-service"

	// No-op handler for outgoing messages (webhook service doesn't process them)
	messageHandler := func(ctx context.Context, kafkaMsg *queue.KafkaMessage) error {
		return nil
	}

	kafkaConsumer, err := queue.NewConsumer(&kafkaCfg, messageHandler, deliveryService.HandleDLR, deliveryService.HandleFailed)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer")
	}
	logger.Info().Msg("Kafka consumer инициализирован")

	// Start consuming DLR and failed topics
	go func() {
		if err := kafkaConsumer.ConsumeDLR(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для DLR")
		}
		if err := kafkaConsumer.ConsumeFailed(); err != nil {
			logger.Error().Err(err).Msg("ошибка запуска consumer для failed")
		}
	}()

	// Health checker
	healthChecker := monitoring.NewHealthChecker("webhook-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	webhookGrpcServer := webhookgrpc.NewServer(webhookService)
	webhookv1.RegisterWebhookServiceServer(grpcServer, webhookGrpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	grpcListener, err := net.Listen("tcp", ":9098")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Str("addr", grpcListener.Addr().String()).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// Metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().Str("path", cfg.Monitoring.Prometheus.Path).Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         ":2119",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("HTTP сервер для метрик запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал завершения, остановка сервиса")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Webhook Service остановлен")
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/services/webhook-service/main.go
git commit -m "feat(webhook): add webhook service entrypoint"
```

---

## Task 11: Client Gateway — Webhook HTTP Handlers

**Files:**
- Create: `internal/gateway/client/handlers/webhooks.go`
- Modify: `internal/gateway/client/clients.go` — add Webhook field
- Modify: `internal/gateway/client/router/router.go` — add webhook routes
- Modify: `cmd/client-gateway/main.go` — add WEBHOOK_SERVICE_ADDR

- [ ] **Step 1: Create client gateway webhook handlers**

```go
// internal/gateway/client/handlers/webhooks.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type WebhookHandlers struct {
	webhookClient webhookv1.WebhookServiceClient
}

func NewWebhookHandlers(webhookClient webhookv1.WebhookServiceClient) *WebhookHandlers {
	return &WebhookHandlers{webhookClient: webhookClient}
}

func (h *WebhookHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.URL == "" {
		respondError(w, shared.ErrInvalidInput("url обязателен"))
		return
	}
	if len(req.EventTypes) == 0 {
		respondError(w, shared.ErrInvalidInput("event_types обязателен"))
		return
	}

	resp, err := h.webhookClient.CreateSubscription(r.Context(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        req.URL,
		EventTypes: req.EventTypes,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания webhook подписки")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"id":          resp.Subscription.Id,
		"client_id":   resp.Subscription.ClientId,
		"url":         resp.Subscription.Url,
		"event_types": resp.Subscription.EventTypes,
		"active":      resp.Subscription.Active,
		"secret":      resp.Secret,
		"created_at":  resp.Subscription.CreatedAt.AsTime(),
	}
	respondJSON(w, http.StatusCreated, result)
}

func (h *WebhookHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	resp, err := h.webhookClient.ListSubscriptions(r.Context(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	subs := make([]map[string]interface{}, len(resp.Subscriptions))
	for i, sub := range resp.Subscriptions {
		subs[i] = subscriptionToMap(sub)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs})
}

func (h *WebhookHandlers) GetWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	resp, err := h.webhookClient.GetSubscription(r.Context(), &webhookv1.GetSubscriptionRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, subscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	var req struct {
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
		Active     *bool    `json:"active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	grpcReq := &webhookv1.UpdateSubscriptionRequest{
		Id:         id,
		ClientId:   clientID.String(),
		Url:        req.URL,
		EventTypes: req.EventTypes,
	}
	if req.Active != nil {
		grpcReq.Active = req.Active
	}

	resp, err := h.webhookClient.UpdateSubscription(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, subscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	id := mux.Vars(r)["id"]
	_, err := h.webhookClient.DeleteSubscription(r.Context(), &webhookv1.DeleteSubscriptionRequest{
		Id:       id,
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func subscriptionToMap(sub *webhookv1.SubscriptionInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":          sub.Id,
		"client_id":   sub.ClientId,
		"url":         sub.Url,
		"event_types": sub.EventTypes,
		"active":      sub.Active,
	}
	if sub.CreatedAt != nil {
		result["created_at"] = sub.CreatedAt.AsTime()
	}
	if sub.UpdatedAt != nil {
		result["updated_at"] = sub.UpdatedAt.AsTime()
	}
	return result
}
```

- [ ] **Step 2: Add `codes.ResourceExhausted` to respondGRPCError**

In both `internal/gateway/client/handlers/common.go` and `internal/gateway/admin/handlers/common.go`, add a case to the `respondGRPCError` switch:

```go
	case codes.ResourceExhausted:
		appErr = &shared.AppError{Code: "TOO_MANY_REQUESTS", Message: st.Message(), HTTPStatus: http.StatusTooManyRequests}
```

Add this after the `codes.AlreadyExists` case.

- [ ] **Step 3: Add WebhookClient to client gateway ServiceClients**

In `internal/gateway/client/clients.go`, add:
- Import `webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"`
- Field `WebhookClient webhookv1.WebhookServiceClient` in `ServiceClients`
- Field `Webhook string` in `ServiceAddresses`
- Connection block for Webhook (same pattern as Billing)

- [ ] **Step 4: Add webhook routes to client router**

In `internal/gateway/client/router/router.go`:
- Add `webhookHandlers *handlers.WebhookHandlers` parameter to `SetupRouter`
- Add routes:

```go
// Webhook endpoints
webhooks := apiV1.PathPrefix("/webhooks").Subrouter()
webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
webhooks.HandleFunc("/{id}", webhookHandlers.GetWebhook).Methods("GET")
webhooks.HandleFunc("/{id}", webhookHandlers.UpdateWebhook).Methods("PUT")
webhooks.HandleFunc("/{id}", webhookHandlers.DeleteWebhook).Methods("DELETE")
```

- [ ] **Step 5: Update client-gateway main.go**

In `cmd/client-gateway/main.go`:
- Add `Webhook` to `serviceAddresses`: `Webhook: getEnvOrDefault("WEBHOOK_SERVICE_ADDR", "localhost:9098")`
- Create webhook handlers: `webhookHandlers := handlers.NewWebhookHandlers(serviceClients.WebhookClient)`
- Pass to `clientrouter.SetupRouter(..., webhookHandlers)`

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/client/handlers/webhooks.go \
  internal/gateway/client/handlers/common.go \
  internal/gateway/client/clients.go \
  internal/gateway/client/router/router.go \
  cmd/client-gateway/main.go
git commit -m "feat(webhook): add webhook endpoints to client gateway"
```

---

## Task 12: Admin Gateway — Webhook HTTP Handlers

**Files:**
- Create: `internal/gateway/admin/handlers/webhooks.go`
- Modify: `internal/gateway/admin/clients.go` — add Webhook field
- Modify: `internal/gateway/admin/router/router.go` — add webhook routes
- Modify: `cmd/admin-gateway/main.go` — add WEBHOOK_SERVICE_ADDR

- [ ] **Step 1: Create admin gateway webhook handlers**

```go
// internal/gateway/admin/handlers/webhooks.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type WebhookHandlers struct {
	webhookClient webhookv1.WebhookServiceClient
}

func NewWebhookHandlers(webhookClient webhookv1.WebhookServiceClient) *WebhookHandlers {
	return &WebhookHandlers{webhookClient: webhookClient}
}

func (h *WebhookHandlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID   string   `json:"client_id"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ClientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}
	if req.URL == "" {
		respondError(w, shared.ErrInvalidInput("url обязателен"))
		return
	}
	if len(req.EventTypes) == 0 {
		respondError(w, shared.ErrInvalidInput("event_types обязателен"))
		return
	}

	resp, err := h.webhookClient.CreateSubscription(r.Context(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   req.ClientID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания webhook подписки")
		respondGRPCError(w, err)
		return
	}

	result := map[string]interface{}{
		"id":          resp.Subscription.Id,
		"client_id":   resp.Subscription.ClientId,
		"url":         resp.Subscription.Url,
		"event_types": resp.Subscription.EventTypes,
		"active":      resp.Subscription.Active,
		"secret":      resp.Secret,
		"created_at":  resp.Subscription.CreatedAt.AsTime(),
	}
	respondJSON(w, http.StatusCreated, result)
}

func (h *WebhookHandlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.webhookClient.ListSubscriptions(r.Context(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	subs := make([]map[string]interface{}, len(resp.Subscriptions))
	for i, sub := range resp.Subscriptions {
		subs[i] = adminSubscriptionToMap(sub)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": subs})
}

func (h *WebhookHandlers) GetWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	resp, err := h.webhookClient.GetSubscription(r.Context(), &webhookv1.GetSubscriptionRequest{
		Id:       id,
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, adminSubscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		ClientID   string   `json:"client_id"`
		URL        string   `json:"url"`
		EventTypes []string `json:"event_types"`
		Active     *bool    `json:"active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ClientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	grpcReq := &webhookv1.UpdateSubscriptionRequest{
		Id:         id,
		ClientId:   req.ClientID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
	}
	if req.Active != nil {
		grpcReq.Active = req.Active
	}

	resp, err := h.webhookClient.UpdateSubscription(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, adminSubscriptionToMap(resp.Subscription))
}

func (h *WebhookHandlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		respondError(w, shared.ErrInvalidInput("client_id обязателен"))
		return
	}

	_, err := h.webhookClient.DeleteSubscription(r.Context(), &webhookv1.DeleteSubscriptionRequest{
		Id:       id,
		ClientId: clientID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSubscriptionToMap(sub *webhookv1.SubscriptionInfo) map[string]interface{} {
	result := map[string]interface{}{
		"id":          sub.Id,
		"client_id":   sub.ClientId,
		"url":         sub.Url,
		"event_types": sub.EventTypes,
		"active":      sub.Active,
	}
	if sub.CreatedAt != nil {
		result["created_at"] = sub.CreatedAt.AsTime()
	}
	if sub.UpdatedAt != nil {
		result["updated_at"] = sub.UpdatedAt.AsTime()
	}
	return result
}
```

- [ ] **Step 2: Add WebhookClient to admin gateway ServiceClients**

In `internal/gateway/admin/clients.go`, add:
- Import `webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"`
- Field `WebhookClient webhookv1.WebhookServiceClient` in `ServiceClients`
- Field `Webhook string` in `ServiceAddresses`
- Connection block for Webhook

- [ ] **Step 3: Add webhook routes to admin router**

In `internal/gateway/admin/router/router.go`:
- Add `webhookHandlers *handlers.WebhookHandlers` parameter to `SetupRouter`
- Add routes:

```go
// Webhook endpoints
webhooks := adminV1.PathPrefix("/webhooks").Subrouter()
webhooks.HandleFunc("", webhookHandlers.CreateWebhook).Methods("POST")
webhooks.HandleFunc("", webhookHandlers.ListWebhooks).Methods("GET")
webhooks.HandleFunc("/{id}", webhookHandlers.GetWebhook).Methods("GET")
webhooks.HandleFunc("/{id}", webhookHandlers.UpdateWebhook).Methods("PUT")
webhooks.HandleFunc("/{id}", webhookHandlers.DeleteWebhook).Methods("DELETE")
```

- [ ] **Step 4: Update admin-gateway main.go**

In `cmd/admin-gateway/main.go`:
- Add `Webhook` to `serviceAddresses`: `Webhook: getEnvOrDefault("WEBHOOK_SERVICE_ADDR", "localhost:9098")`
- Create webhook handlers: `webhookHandlers := handlers.NewWebhookHandlers(serviceClients.WebhookClient)`
- Pass to `SetupRouter(..., webhookHandlers, ...)`

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/admin/handlers/webhooks.go \
  internal/gateway/admin/clients.go \
  internal/gateway/admin/router/router.go \
  cmd/admin-gateway/main.go
git commit -m "feat(webhook): add webhook endpoints to admin gateway"
```

---

## Task 13: Docker Compose Integration

**Files:**
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Add webhook-service to docker-compose.yml**

Add after the billing-service definition (follow same pattern):

```yaml
  webhook-service:
    build:
      context: ..
      dockerfile: deployments/docker/service-base.Dockerfile
      args:
        - SERVICE_NAME=webhook-service
    container_name: sms-webhook-service
    ports:
      - "9098:9098"
      - "2119:2119"
    environment:
      - SERVICE_NAME=webhook-service
      - SERVICE_ENV=development
      - DATABASE_HOST=postgres
      - DATABASE_PORT=5432
      - DATABASE_USER=sms_user
      - DATABASE_PASSWORD=sms_password
      - DATABASE_NAME=sms_platform
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      postgres:
        condition: service_healthy
      kafka:
        condition: service_healthy
    networks:
      - smpp-network
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:2119/health/live"]
      interval: 10s
      timeout: 5s
      retries: 5
```

- [ ] **Step 2: Add WEBHOOK_SERVICE_ADDR to gateway services**

Add to both `client-gateway-1`, `client-gateway-2`, `admin-gateway-1`, `admin-gateway-2` environment sections:

```yaml
      - WEBHOOK_SERVICE_ADDR=webhook-service:9098
```

- [ ] **Step 3: Add webhook-service dependency to gateways**

Add to the `depends_on` section of each gateway:

```yaml
      webhook-service:
        condition: service_healthy
```

- [ ] **Step 4: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "feat(webhook): add webhook-service to docker-compose"
```

---

## Task 14: Build Verification

- [ ] **Step 1: Verify proto generation works**

```bash
cd /home/magomed/projects/sms
ls api/proto/webhookv1/
```

If generated files don't exist yet, generate them now.

- [ ] **Step 2: Update Go module dependencies**

```bash
go mod tidy
```

Expected: `go.mod` and `go.sum` updated with any new dependencies (e.g., `github.com/lib/pq` for `pq.StringArray`).

- [ ] **Step 3: Verify Go build**

```bash
go build ./...
```

Expected: no compilation errors.

- [ ] **Step 4: Verify go vet**

```bash
go vet ./...
```

Expected: no issues.

- [ ] **Step 5: Fix any issues found**

Address any compilation or vet errors.

- [ ] **Step 6: Commit fixes if any**

```bash
git add -A
git commit -m "fix(webhook): address build issues"
```
