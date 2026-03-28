package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

// TemplateRepository defines the persistence operations required by TemplateService.
type TemplateRepository interface {
	Create(ctx context.Context, t *domain.Template) (*domain.Template, error)
	GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error)
	GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error)
	Update(ctx context.Context, t *domain.Template) (*domain.Template, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string) (*domain.Template, error)
	Delete(ctx context.Context, id, clientID uuid.UUID) error
	AssignReviewer(ctx context.Context, templateID, reviewerID uuid.UUID) error
	RequestRevision(ctx context.Context, templateID, reviewerID uuid.UUID, comment string) error
}

// AuditRepository defines the persistence operations required by TemplateService for audit logging.
type AuditRepository interface {
	Create(ctx context.Context, entry *domain.AuditEntry) error
	ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error)
}
