// internal/services/webhook/application/ports.go
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

// SubscriptionRepository defines the persistence operations used by WebhookService.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)
	GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Subscription, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*domain.Subscription, error)
	Update(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)
	Delete(ctx context.Context, id, clientID uuid.UUID) error
	CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error)
}
