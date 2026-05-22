package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCompanyRepo struct {
	CreateFunc         func(ctx context.Context, c *shared.Company) (*shared.Company, error)
	GetByIDFunc        func(ctx context.Context, id uuid.UUID) (*shared.Company, error)
	UpdateFunc         func(ctx context.Context, c *shared.Company) (*shared.Company, error)
	ListByClientIDFunc func(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error)
	AttachCompanyFunc  func(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error
	DetachCompanyFunc  func(ctx context.Context, clientID, companyID uuid.UUID) error
	SetDefaultFunc     func(ctx context.Context, clientID, companyID uuid.UUID) error
	HasSenderNamesFunc func(ctx context.Context, clientID, companyID uuid.UUID) (bool, error)
}

func (m *mockCompanyRepo) Create(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	return m.CreateFunc(ctx, c)
}
func (m *mockCompanyRepo) GetByID(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	return m.GetByIDFunc(ctx, id)
}
func (m *mockCompanyRepo) Update(ctx context.Context, c *shared.Company) (*shared.Company, error) {
	return m.UpdateFunc(ctx, c)
}
func (m *mockCompanyRepo) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]*shared.Company, []bool, error) {
	return m.ListByClientIDFunc(ctx, clientID)
}
func (m *mockCompanyRepo) AttachCompany(ctx context.Context, clientID, companyID uuid.UUID, isDefault bool) error {
	return m.AttachCompanyFunc(ctx, clientID, companyID, isDefault)
}
func (m *mockCompanyRepo) DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return m.DetachCompanyFunc(ctx, clientID, companyID)
}
func (m *mockCompanyRepo) SetDefault(ctx context.Context, clientID, companyID uuid.UUID) error {
	return m.SetDefaultFunc(ctx, clientID, companyID)
}
func (m *mockCompanyRepo) HasSenderNames(ctx context.Context, clientID, companyID uuid.UUID) (bool, error) {
	return m.HasSenderNamesFunc(ctx, clientID, companyID)
}

func newTestService(repo CompanyRepository) *CompanyService {
	return NewCompanyService(repo, nil)
}

func TestCreateCompany_EmptyName(t *testing.T) {
	svc := newTestService(&mockCompanyRepo{})
	_, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: uuid.New(),
		Name:     "",
	})
	assert.ErrorIs(t, err, domain.ErrCompanyNameRequired)
}

func TestCreateCompany_InvalidINN(t *testing.T) {
	svc := newTestService(&mockCompanyRepo{})
	_, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: uuid.New(),
		Name:     "ООО Тест",
		INN:      "1234567890",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidINN)
}

func TestCreateCompany_Success(t *testing.T) {
	clientID := uuid.New()
	companyID := uuid.New()
	repo := &mockCompanyRepo{
		CreateFunc: func(_ context.Context, c *shared.Company) (*shared.Company, error) {
			c.ID = companyID
			return c, nil
		},
		AttachCompanyFunc: func(_ context.Context, cID, coID uuid.UUID, def bool) error {
			assert.Equal(t, clientID, cID)
			assert.Equal(t, companyID, coID)
			assert.False(t, def)
			return nil
		},
	}
	svc := newTestService(repo)
	got, err := svc.CreateCompany(context.Background(), CreateCompanyInput{
		ClientID: clientID,
		Name:     "ООО Тест",
		INN:      "",
	})
	require.NoError(t, err)
	assert.Equal(t, "ООО Тест", got.Name)
}

func TestUpdateCompany_InvalidINN(t *testing.T) {
	companyID := uuid.New()
	repo := &mockCompanyRepo{
		GetByIDFunc: func(_ context.Context, id uuid.UUID) (*shared.Company, error) {
			return &shared.Company{ID: id, Name: "Old"}, nil
		},
	}
	svc := newTestService(repo)
	_, err := svc.UpdateCompany(context.Background(), UpdateCompanyInput{
		ID:   companyID,
		Name: "New",
		INN:  "7707083890", // Сбербанк с неверной контрольной суммой
	})
	assert.ErrorIs(t, err, domain.ErrInvalidINN)
}
