package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

// LinkRepository defines the operations for short link persistence.
type LinkRepo interface {
	CreateShortLink(ctx context.Context, link *domain.ShortLink) error
	GetByCode(ctx context.Context, code string) (*domain.ShortLink, error)
	GetClientActiveDomain(ctx context.Context, clientID uuid.UUID) (*domain.ClientDomain, error)
}

// DomainRepo defines the operations for custom domain persistence.
type DomainRepo interface {
	Create(ctx context.Context, d *domain.ClientDomain) error
	ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientDomain, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ClickRepo defines the operations for click event persistence.
type ClickRepo interface {
	Insert(ctx context.Context, event *domain.ClickEvent) error
}
