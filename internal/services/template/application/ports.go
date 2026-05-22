package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
)

// SenderNameRepository defines the persistence operations for SenderName.
type SenderNameRepository interface {
	Create(ctx context.Context, sn *domain.SenderName) (*domain.SenderName, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.SenderName, error)
	GetByClientAndName(ctx context.Context, clientID uuid.UUID, name string) (*domain.SenderName, error)
	ListByClient(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.SenderName, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string, reviewerID *uuid.UUID) (*domain.SenderName, error)
	Update(ctx context.Context, sn *domain.SenderName) (*domain.SenderName, error)
	ListAll(ctx context.Context, clientID *uuid.UUID, status, nameQuery string, limit, offset int) ([]*domain.SenderName, int, error)
	AddHistoryEntry(ctx context.Context, entry *domain.SenderNameStatusHistory) error
	GetHistory(ctx context.Context, senderNameID uuid.UUID, limit, offset int) ([]*domain.SenderNameStatusHistory, int, error)
}

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
