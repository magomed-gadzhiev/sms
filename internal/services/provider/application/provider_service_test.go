package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/services/provider/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validProvider returns a fully valid Provider for use in tests.
func validProvider() *domain.Provider {
	return &domain.Provider{
		Name:           "test-provider",
		Host:           "smpp.example.com",
		Port:           2775,
		SystemID:       "sysid",
		Password:       "secret",
		BindType:       domain.BindTypeTransceiver,
		MaxConnections: 5,
		Active:         true,
	}
}

// ─── CreateProvider ────────────────────────────────────────────────────────────

func TestCreateProvider_Success(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	p := validProvider()

	// No existing provider with this name.
	repo.On("GetByName", ctx, p.Name).Return(nil, domain.ErrProviderNotFound)
	repo.On("Create", ctx, p).Return(nil)

	err := svc.CreateProvider(ctx, p)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, p.ID)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
	repo.AssertExpectations(t)
}

func TestCreateProvider_ValidationError_EmptyName(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	p := validProvider()
	p.Name = ""

	err := svc.CreateProvider(ctx, p)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProviderNameRequired)
	repo.AssertNotCalled(t, "GetByName")
	repo.AssertNotCalled(t, "Create")
}

func TestCreateProvider_ValidationError_EmptyHost(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	p := validProvider()
	p.Host = ""

	err := svc.CreateProvider(ctx, p)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProviderHostRequired)
	repo.AssertNotCalled(t, "GetByName")
	repo.AssertNotCalled(t, "Create")
}

func TestCreateProvider_DuplicateName(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	p := validProvider()
	existing := validProvider()
	existing.ID = uuid.New()

	repo.On("GetByName", ctx, p.Name).Return(existing, nil)

	err := svc.CreateProvider(ctx, p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), p.Name)
	repo.AssertNotCalled(t, "Create")
	repo.AssertExpectations(t)
}

// ─── GetProvider ───────────────────────────────────────────────────────────────

func TestGetProvider_Found(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()
	expected := validProvider()
	expected.ID = id

	repo.On("GetByID", ctx, id).Return(expected, nil)

	got, err := svc.GetProvider(ctx, id)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
	repo.AssertExpectations(t)
}

func TestGetProvider_NotFound(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, domain.ErrProviderNotFound)

	got, err := svc.GetProvider(ctx, id)

	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, domain.ErrProviderNotFound)
	repo.AssertExpectations(t)
}

// ─── ListProviders ─────────────────────────────────────────────────────────────

func TestListProviders_DefaultsForNegativeLimit(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	providers := []*domain.Provider{validProvider()}
	// Negative limit must be normalized to 100, negative offset to 0.
	repo.On("List", ctx, false, 100, 0).Return(providers, 1, nil)

	got, total, err := svc.ListProviders(ctx, false, -5, -3)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, got, 1)
	repo.AssertExpectations(t)
}

func TestListProviders_CapsLimitAt1000(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	repo.On("List", ctx, true, 1000, 0).Return([]*domain.Provider{}, 0, nil)

	_, _, err := svc.ListProviders(ctx, true, 9999, 0)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestListProviders_NormalParams(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	providers := []*domain.Provider{validProvider(), validProvider()}
	repo.On("List", ctx, false, 50, 10).Return(providers, 2, nil)

	got, total, err := svc.ListProviders(ctx, false, 50, 10)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, got, 2)
	repo.AssertExpectations(t)
}

func TestListProviders_RepoError(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	repoErr := errors.New("db error")
	repo.On("List", ctx, false, 100, 0).Return(nil, 0, repoErr)

	got, total, err := svc.ListProviders(ctx, false, 0, 0)

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, 0, total)
	repo.AssertExpectations(t)
}

// ─── DeleteProvider ────────────────────────────────────────────────────────────

func TestDeleteProvider_SoftDelete(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()
	p := validProvider()
	p.ID = id
	p.Active = true

	repo.On("GetByID", ctx, id).Return(p, nil)
	repo.On("Update", ctx, p).Return(nil)

	err := svc.DeleteProvider(ctx, id)

	require.NoError(t, err)
	// Soft delete must set Active to false.
	assert.False(t, p.Active)
	repo.AssertExpectations(t)
}

func TestDeleteProvider_NotFound(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, domain.ErrProviderNotFound)

	err := svc.DeleteProvider(ctx, id)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProviderNotFound)
	repo.AssertNotCalled(t, "Update")
	repo.AssertExpectations(t)
}

// ─── UpdateProvider ────────────────────────────────────────────────────────────

func TestUpdateProvider_Success(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()
	existing := validProvider()
	existing.ID = id

	updates := &domain.Provider{
		Host:     "new-host.example.com",
		Port:     2776,
		Active:   true,
		BindType: domain.BindTypeTransmitter,
	}

	repo.On("GetByID", ctx, id).Return(existing, nil)
	repo.On("Update", ctx, existing).Return(nil)

	err := svc.UpdateProvider(ctx, id, updates)

	require.NoError(t, err)
	assert.Equal(t, "new-host.example.com", existing.Host)
	assert.Equal(t, 2776, existing.Port)
	repo.AssertExpectations(t)
}

func TestUpdateProvider_NotFound(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, domain.ErrProviderNotFound)

	err := svc.UpdateProvider(ctx, id, &domain.Provider{})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrProviderNotFound)
	repo.AssertNotCalled(t, "Update")
	repo.AssertExpectations(t)
}

func TestUpdateProvider_DuplicateName(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()
	existing := validProvider()
	existing.ID = id
	existing.Name = "original-name"

	otherID := uuid.New()
	other := validProvider()
	other.ID = otherID
	other.Name = "taken-name"

	updates := &domain.Provider{
		Name: "taken-name",
	}

	repo.On("GetByID", ctx, id).Return(existing, nil)
	repo.On("GetByName", ctx, "taken-name").Return(other, nil)

	err := svc.UpdateProvider(ctx, id, updates)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "taken-name")
	repo.AssertNotCalled(t, "Update")
	repo.AssertExpectations(t)
}

func TestUpdateProvider_SameNameAllowed(t *testing.T) {
	repo := &mocks.MockProviderRepository{}
	svc := NewProviderService(repo)
	ctx := context.Background()

	id := uuid.New()
	existing := validProvider()
	existing.ID = id
	existing.Name = "same-name"

	// Updating with the same name — no GetByName call expected.
	updates := &domain.Provider{
		Name:   "same-name",
		Active: true,
	}

	repo.On("GetByID", ctx, id).Return(existing, nil)
	repo.On("Update", ctx, existing).Return(nil)

	err := svc.UpdateProvider(ctx, id, updates)

	require.NoError(t, err)
	repo.AssertNotCalled(t, "GetByName")
	repo.AssertExpectations(t)
}
