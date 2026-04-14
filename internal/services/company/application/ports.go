package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CompanyRepository определяет операции хранения для компаний.
type CompanyRepository interface {
	Create(ctx context.Context, c *shared.Company) (*shared.Company, error)
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error)
	Update(ctx context.Context, c *shared.Company) (*shared.Company, error)
	ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error)
	AttachCompany(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error
	DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error
	SetDefault(ctx context.Context, clientID, companyID uuid.UUID) error
	HasSenderNames(ctx context.Context, clientID, companyID uuid.UUID) (bool, error)
}

// AccountRepository нужен для создания нулевого аккаунта при создании компании.
type AccountRepository interface {
	CreateForCompany(ctx context.Context, clientID, companyID uuid.UUID, currency string) error
}
