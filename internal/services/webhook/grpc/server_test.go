package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	webhookv1 "github.com/smpp-server/smpp-server/api/proto/webhookv1"
	"github.com/smpp-server/smpp-server/internal/services/webhook/application"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
	grpcserver "github.com/smpp-server/smpp-server/internal/services/webhook/grpc"
	"github.com/smpp-server/smpp-server/internal/services/webhook/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// helpers

func newWebhookServer(repo application.SubscriptionRepository, cache application.CacheInvalidator) *grpcserver.Server {
	svc := application.NewWebhookService(repo, cache)
	return grpcserver.NewServer(svc)
}

func validClientID() string { return uuid.New().String() }
func validID() string       { return uuid.New().String() }

func makeSub(id, clientID uuid.UUID) *domain.Subscription {
	return &domain.Subscription{
		ID:         id,
		ClientID:   clientID,
		URL:        "https://example.com/webhook",
		EventTypes: []string{"delivered", "failed"},
		Secret:     "secret",
		Active:     true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// ─── CreateSubscription ───────────────────────────────────────────────────────

func TestCreateSubscription_MissingClientID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   "",
		Url:        "https://example.com/webhook",
		EventTypes: []string{"delivered"},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "client_id")
}

func TestCreateSubscription_MissingURL(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   validClientID(),
		Url:        "",
		EventTypes: []string{"delivered"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "url")
}

func TestCreateSubscription_MissingEventTypes(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   validClientID(),
		Url:        "https://example.com/webhook",
		EventTypes: nil,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "event_types")
}

func TestCreateSubscription_InvalidClientIDFormat(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   "not-a-uuid",
		Url:        "https://example.com/webhook",
		EventTypes: []string{"delivered"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCreateSubscription_InvalidURL(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        "http://not-https.com/webhook", // must be HTTPS
		EventTypes: []string{"delivered"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCreateSubscription_InvalidEventType(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	repo.On("CountByClientID", mock.Anything, clientID).Return(0, nil)

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        "https://example.com/webhook",
		EventTypes: []string{"invalid_type"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCreateSubscription_Success(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	id := uuid.New()
	sub := makeSub(id, clientID)

	repo.On("CountByClientID", mock.Anything, clientID).Return(0, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Subscription")).Return(sub, nil)
	cache.On("InvalidateCache", clientID).Return()

	resp, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        "https://example.com/webhook",
		EventTypes: []string{"delivered", "failed"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Subscription)
	assert.Equal(t, id.String(), resp.Subscription.Id)
	assert.Equal(t, clientID.String(), resp.Subscription.ClientId)
	// Secret is returned on create
	assert.NotEmpty(t, resp.Secret)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestCreateSubscription_MaxReached(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	repo.On("CountByClientID", mock.Anything, clientID).Return(domain.MaxSubscriptionsPerClient, nil)

	_, err := srv.CreateSubscription(context.Background(), &webhookv1.CreateSubscriptionRequest{
		ClientId:   clientID.String(),
		Url:        "https://example.com/webhook",
		EventTypes: []string{"delivered"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

// ─── GetSubscription ──────────────────────────────────────────────────────────

func TestGetSubscription_MissingID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.GetSubscription(context.Background(), &webhookv1.GetSubscriptionRequest{
		Id:       "",
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetSubscription_MissingClientID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.GetSubscription(context.Background(), &webhookv1.GetSubscriptionRequest{
		Id:       validID(),
		ClientId: "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetSubscription_NotFound(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	repo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(nil, domain.ErrSubscriptionNotFound)

	_, err := srv.GetSubscription(context.Background(), &webhookv1.GetSubscriptionRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())

	repo.AssertExpectations(t)
}

func TestGetSubscription_Success(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	id := uuid.New()
	sub := makeSub(id, clientID)

	repo.On("GetByID", mock.Anything, id, clientID).Return(sub, nil)

	resp, err := srv.GetSubscription(context.Background(), &webhookv1.GetSubscriptionRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Subscription)
	assert.Equal(t, id.String(), resp.Subscription.Id)
	assert.Equal(t, clientID.String(), resp.Subscription.ClientId)

	repo.AssertExpectations(t)
}

// ─── ListSubscriptions ────────────────────────────────────────────────────────

func TestListSubscriptions_MissingClientID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.ListSubscriptions(context.Background(), &webhookv1.ListSubscriptionsRequest{
		ClientId: "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestListSubscriptions_InvalidClientIDFormat(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.ListSubscriptions(context.Background(), &webhookv1.ListSubscriptionsRequest{
		ClientId: "not-a-uuid",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestListSubscriptions_Success(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	subs := []*domain.Subscription{
		makeSub(uuid.New(), clientID),
		makeSub(uuid.New(), clientID),
	}

	repo.On("ListByClientID", mock.Anything, clientID).Return(subs, nil)

	resp, err := srv.ListSubscriptions(context.Background(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.Len(t, resp.Subscriptions, 2)

	repo.AssertExpectations(t)
}

func TestListSubscriptions_Empty(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	repo.On("ListByClientID", mock.Anything, clientID).Return([]*domain.Subscription{}, nil)

	resp, err := srv.ListSubscriptions(context.Background(), &webhookv1.ListSubscriptionsRequest{
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Subscriptions)
}

// ─── UpdateSubscription ───────────────────────────────────────────────────────

func TestUpdateSubscription_MissingID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.UpdateSubscription(context.Background(), &webhookv1.UpdateSubscriptionRequest{
		Id:       "",
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUpdateSubscription_Success_URLOnly(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	id := uuid.New()
	existing := makeSub(id, clientID)
	newURL := "https://new.example.com/webhook"
	updated := makeSub(id, clientID)
	updated.URL = newURL

	repo.On("GetByID", mock.Anything, id, clientID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Subscription")).Return(updated, nil)
	cache.On("InvalidateCache", clientID).Return()

	resp, err := srv.UpdateSubscription(context.Background(), &webhookv1.UpdateSubscriptionRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
		Url:      newURL,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Subscription)
	assert.Equal(t, newURL, resp.Subscription.Url)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestUpdateSubscription_Success_ActiveFlag(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	id := uuid.New()
	existing := makeSub(id, clientID)
	disabled := makeSub(id, clientID)
	disabled.Active = false

	repo.On("GetByID", mock.Anything, id, clientID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Subscription")).Return(disabled, nil)
	cache.On("InvalidateCache", clientID).Return()

	active := false
	resp, err := srv.UpdateSubscription(context.Background(), &webhookv1.UpdateSubscriptionRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
		Active:   &active,
	})
	require.NoError(t, err)
	assert.False(t, resp.Subscription.Active)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestUpdateSubscription_NotFound(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	repo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(nil, domain.ErrSubscriptionNotFound)

	_, err := srv.UpdateSubscription(context.Background(), &webhookv1.UpdateSubscriptionRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ─── DeleteSubscription ───────────────────────────────────────────────────────

func TestDeleteSubscription_MissingID(t *testing.T) {
	srv := newWebhookServer(new(mocks.MockSubscriptionRepository), new(mocks.MockCacheInvalidator))

	_, err := srv.DeleteSubscription(context.Background(), &webhookv1.DeleteSubscriptionRequest{
		Id:       "",
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestDeleteSubscription_Success(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	clientID := uuid.New()
	id := uuid.New()

	repo.On("Delete", mock.Anything, id, clientID).Return(nil)
	cache.On("InvalidateCache", clientID).Return()

	resp, err := srv.DeleteSubscription(context.Background(), &webhookv1.DeleteSubscriptionRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	repo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	repo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(domain.ErrSubscriptionNotFound)

	_, err := srv.DeleteSubscription(context.Background(), &webhookv1.DeleteSubscriptionRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ─── error mapping ────────────────────────────────────────────────────────────

func TestErrorMapping_InternalError(t *testing.T) {
	repo := new(mocks.MockSubscriptionRepository)
	cache := new(mocks.MockCacheInvalidator)
	srv := newWebhookServer(repo, cache)

	id := uuid.New()
	clientID := uuid.New()
	repo.On("GetByID", mock.Anything, id, clientID).Return(nil, assert.AnError)

	_, err := srv.GetSubscription(context.Background(), &webhookv1.GetSubscriptionRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}
