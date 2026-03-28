package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	"github.com/smpp-server/smpp-server/internal/services/webhook/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestService(repo *mocks.MockSubscriptionRepository, cache *mocks.MockCacheInvalidator) *application.WebhookService {
	return application.NewWebhookService(repo, cache)
}

// ─── CreateSubscription ───────────────────────────────────────────────────────

func TestCreateSubscription_Success(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	clientID := uuid.New()
	url := "https://example.com/webhook"
	eventTypes := []string{"delivered", "failed"}

	repo.On("CountByClientID", mock.Anything, clientID).Return(0, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(sub *domain.Subscription) bool {
		return sub.ClientID == clientID &&
			sub.URL == url &&
			sub.Active == true &&
			len(sub.Secret) == 64 // 32 bytes hex-encoded
	})).Return(&domain.Subscription{
		ID:         uuid.New(),
		ClientID:   clientID,
		URL:        url,
		EventTypes: eventTypes,
		Secret:     "stored-secret",
		Active:     true,
	}, nil)
	cache.On("InvalidateCache", clientID).Return()

	sub, err := svc.CreateSubscription(context.Background(), clientID, url, eventTypes)
	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.True(t, sub.Active)
	// Secret returned to caller is the generated one (64 hex chars), not the stored placeholder
	assert.Len(t, sub.Secret, 64)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestCreateSubscription_MaxSubscriptionsReached(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	clientID := uuid.New()
	repo.On("CountByClientID", mock.Anything, clientID).Return(domain.MaxSubscriptionsPerClient, nil)

	_, err := svc.CreateSubscription(context.Background(), clientID, "https://example.com/wh", []string{"delivered"})
	assert.ErrorIs(t, err, domain.ErrMaxSubscriptionsReached)

	repo.AssertExpectations(t)
	cache.AssertNotCalled(t, "InvalidateCache")
}

func TestCreateSubscription_InvalidURL(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	clientID := uuid.New()
	// http:// instead of https://
	_, err := svc.CreateSubscription(context.Background(), clientID, "http://example.com/wh", []string{"delivered"})
	assert.ErrorIs(t, err, domain.ErrInvalidURL)

	repo.AssertNotCalled(t, "CountByClientID")
	cache.AssertNotCalled(t, "InvalidateCache")
}

func TestCreateSubscription_InvalidEventType(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	clientID := uuid.New()
	// URL is valid; event type is not
	_, err := svc.CreateSubscription(context.Background(), clientID, "https://example.com/wh", []string{"not-a-real-event"})
	assert.ErrorIs(t, err, domain.ErrInvalidEventType)

	repo.AssertNotCalled(t, "CountByClientID")
	cache.AssertNotCalled(t, "InvalidateCache")
}

// ─── GetSubscription ──────────────────────────────────────────────────────────

func TestGetSubscription_Found(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	stored := &domain.Subscription{
		ID:       id,
		ClientID: clientID,
		URL:      "https://example.com/wh",
		Secret:   "secret-should-be-cleared",
		Active:   true,
	}
	repo.On("GetByID", mock.Anything, id, clientID).Return(stored, nil)

	sub, err := svc.GetSubscription(context.Background(), id, clientID)
	require.NoError(t, err)
	assert.Equal(t, id, sub.ID)
	assert.Empty(t, sub.Secret, "secret must be cleared on Get")

	repo.AssertExpectations(t)
}

func TestGetSubscription_NotFound(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	repo.On("GetByID", mock.Anything, id, clientID).Return(nil, domain.ErrSubscriptionNotFound)

	_, err := svc.GetSubscription(context.Background(), id, clientID)
	assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)

	repo.AssertExpectations(t)
}

// ─── ListSubscriptions ────────────────────────────────────────────────────────

func TestListSubscriptions_Success(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	clientID := uuid.New()
	stored := []*domain.Subscription{
		{ID: uuid.New(), ClientID: clientID, Secret: "s1"},
		{ID: uuid.New(), ClientID: clientID, Secret: "s2"},
	}
	repo.On("ListByClientID", mock.Anything, clientID).Return(stored, nil)

	list, err := svc.ListSubscriptions(context.Background(), clientID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	for _, s := range list {
		assert.Empty(t, s.Secret, "secrets must be cleared on List")
	}

	repo.AssertExpectations(t)
}

// ─── UpdateSubscription ───────────────────────────────────────────────────────

func TestUpdateSubscription_Success(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	newURL := "https://new.example.com/wh"
	active := false

	existing := &domain.Subscription{
		ID:         id,
		ClientID:   clientID,
		URL:        "https://old.example.com/wh",
		EventTypes: []string{"delivered"},
		Secret:     "sec",
		Active:     true,
	}
	updated := &domain.Subscription{
		ID:         id,
		ClientID:   clientID,
		URL:        newURL,
		EventTypes: []string{"delivered"},
		Secret:     "sec",
		Active:     false,
	}

	repo.On("GetByID", mock.Anything, id, clientID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(sub *domain.Subscription) bool {
		return sub.URL == newURL && sub.Active == false
	})).Return(updated, nil)
	cache.On("InvalidateCache", clientID).Return()

	result, err := svc.UpdateSubscription(context.Background(), id, clientID, &newURL, nil, &active)
	require.NoError(t, err)
	assert.Equal(t, newURL, result.URL)
	assert.False(t, result.Active)
	assert.Empty(t, result.Secret)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestUpdateSubscription_NotFound(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	repo.On("GetByID", mock.Anything, id, clientID).Return(nil, domain.ErrSubscriptionNotFound)

	_, err := svc.UpdateSubscription(context.Background(), id, clientID, nil, nil, nil)
	assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)

	repo.AssertNotCalled(t, "Update")
	cache.AssertNotCalled(t, "InvalidateCache")
}

// ─── DeleteSubscription ───────────────────────────────────────────────────────

func TestDeleteSubscription_Success(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	repo.On("Delete", mock.Anything, id, clientID).Return(nil)
	cache.On("InvalidateCache", clientID).Return()

	err := svc.DeleteSubscription(context.Background(), id, clientID)
	require.NoError(t, err)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := &mocks.MockSubscriptionRepository{}
	cache := &mocks.MockCacheInvalidator{}
	svc := newTestService(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	repo.On("Delete", mock.Anything, id, clientID).Return(domain.ErrSubscriptionNotFound)

	err := svc.DeleteSubscription(context.Background(), id, clientID)
	assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)

	cache.AssertNotCalled(t, "InvalidateCache")
	repo.AssertExpectations(t)
}
