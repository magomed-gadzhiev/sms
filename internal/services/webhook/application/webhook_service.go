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
