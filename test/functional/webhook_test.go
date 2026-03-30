//go:build functional

package functional_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	webhookRepo "github.com/smpp-server/smpp-server/internal/services/webhook/infrastructure/repository"
	webhookMocks "github.com/smpp-server/smpp-server/internal/services/webhook/mocks"
)

func TestWebhookSubscriptionChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-webhook', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM webhook_subscriptions WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	subRepo := webhookRepo.NewSubscriptionRepository(db)
	mockCache := &webhookMocks.MockCacheInvalidator{}
	mockCache.On("InvalidateCache", mock.Anything).Return()

	svc := application.NewWebhookService(subRepo, mockCache)

	t.Run("CreateSubscription", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/webhook", []string{"delivered", "failed"})
		require.NoError(t, err)
		require.NotNil(t, sub)

		assert.Equal(t, clientID, sub.ClientID)
		assert.Equal(t, "https://example.com/webhook", sub.URL)
		assert.ElementsMatch(t, []string{"delivered", "failed"}, sub.EventTypes)
		assert.True(t, sub.Active)
		assert.NotEmpty(t, sub.Secret, "secret should be returned on creation")
		assert.Len(t, sub.Secret, 64, "secret should be 64 hex chars (32 bytes)")

		// Cache invalidation must have been called.
		mockCache.AssertCalled(t, "InvalidateCache", clientID)
	})

	t.Run("CreateSubscriptionHTTPFails", func(t *testing.T) {
		_, err := svc.CreateSubscription(ctx, clientID, "http://example.com/webhook", []string{"delivered"})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidURL)
	})

	t.Run("CreateSubscriptionInvalidEventTypeFails", func(t *testing.T) {
		_, err := svc.CreateSubscription(ctx, clientID, "https://example.com/webhook", []string{"invalid_event"})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidEventType)
	})

	t.Run("GetSubscription", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/get-test", []string{"delivered"})
		require.NoError(t, err)

		fetched, err := svc.GetSubscription(ctx, sub.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, sub.ID, fetched.ID)
		assert.Equal(t, "https://example.com/get-test", fetched.URL)
		assert.Empty(t, fetched.Secret, "secret must be cleared on get")
	})

	t.Run("GetSubscriptionNotFoundForOtherClient", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/owned", []string{"delivered"})
		require.NoError(t, err)

		_, err = svc.GetSubscription(ctx, sub.ID, uuid.New())
		assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)
	})

	t.Run("ListSubscriptions", func(t *testing.T) {
		subs, err := svc.ListSubscriptions(ctx, clientID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(subs), 1)

		// Secrets must be cleared.
		for _, sub := range subs {
			assert.Empty(t, sub.Secret, "secrets must be cleared on list")
		}
	})

	t.Run("UpdateSubscription", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/to-update", []string{"delivered"})
		require.NoError(t, err)

		newURL := "https://new.example.com/webhook"
		newActive := false
		updated, err := svc.UpdateSubscription(ctx, sub.ID, clientID, &newURL, []string{"delivered", "expired"}, &newActive)
		require.NoError(t, err)
		assert.Equal(t, "https://new.example.com/webhook", updated.URL)
		assert.ElementsMatch(t, []string{"delivered", "expired"}, updated.EventTypes)
		assert.False(t, updated.Active)
		assert.Empty(t, updated.Secret, "secret must be cleared on update response")
	})

	t.Run("UpdateSubscriptionInvalidURLFails", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/valid", []string{"delivered"})
		require.NoError(t, err)

		badURL := "http://insecure.com/webhook"
		_, err = svc.UpdateSubscription(ctx, sub.ID, clientID, &badURL, nil, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidURL)
	})

	t.Run("UpdateSubscriptionInvalidEventTypeFails", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/events", []string{"delivered"})
		require.NoError(t, err)

		_, err = svc.UpdateSubscription(ctx, sub.ID, clientID, nil, []string{"bogus"}, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidEventType)
	})

	t.Run("DeleteSubscription", func(t *testing.T) {
		sub, err := svc.CreateSubscription(ctx, clientID, "https://example.com/to-delete", []string{"delivered"})
		require.NoError(t, err)

		err = svc.DeleteSubscription(ctx, sub.ID, clientID)
		require.NoError(t, err)

		_, err = svc.GetSubscription(ctx, sub.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)
	})

	t.Run("DeleteSubscriptionNotFoundFails", func(t *testing.T) {
		err := svc.DeleteSubscription(ctx, uuid.New(), clientID)
		assert.ErrorIs(t, err, domain.ErrSubscriptionNotFound)
	})
}

func TestWebhookSubscriptionLimit(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-limit', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM webhook_subscriptions WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	subRepo := webhookRepo.NewSubscriptionRepository(db)
	mockCache := &webhookMocks.MockCacheInvalidator{}
	mockCache.On("InvalidateCache", mock.Anything).Return()

	svc := application.NewWebhookService(subRepo, mockCache)

	// Create up to the limit.
	for i := 0; i < domain.MaxSubscriptionsPerClient; i++ {
		_, err := svc.CreateSubscription(ctx, clientID,
			"https://example.com/webhook-"+uuid.New().String()[:8],
			[]string{"delivered"})
		require.NoError(t, err, "creating subscription %d", i)
	}

	// Next creation must fail.
	_, err = svc.CreateSubscription(ctx, clientID, "https://example.com/one-too-many", []string{"delivered"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrMaxSubscriptionsReached)
}
