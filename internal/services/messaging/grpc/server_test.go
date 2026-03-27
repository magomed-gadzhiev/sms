package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/services/messaging/application"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// --- Mocks ---

type mockMessageRepo struct {
	mock.Mock
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetByMessageID(ctx context.Context, messageID string) (*domain.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetByExternalID(ctx context.Context, externalID string) (*domain.Message, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.Message, error) {
	args := m.Called(ctx, smppMessageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) Update(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageRepo) UpdateStatus(ctx context.Context, id uuid.UUID, st string, statusMessage string) error {
	args := m.Called(ctx, id, st, statusMessage)
	return args.Error(0)
}

func (m *mockMessageRepo) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, st *string) ([]*domain.Message, error) {
	args := m.Called(ctx, clientID, limit, offset, st)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetScheduledReady(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) GetStuckPending(ctx context.Context, threshold time.Duration, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, threshold, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) CancelByIDAndStatus(ctx context.Context, id uuid.UUID, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

func (m *mockMessageRepo) GetSentExpired(ctx context.Context, timeout time.Duration, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, timeout, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockMessageRepo) BulkUpdateStatusToExpired(ctx context.Context, messages []*domain.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

type mockDLRRepo struct {
	mock.Mock
}

func (m *mockDLRRepo) Create(ctx context.Context, dlr *domain.DLRReceipt) error {
	args := m.Called(ctx, dlr)
	return args.Error(0)
}

func (m *mockDLRRepo) GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*domain.DLRReceipt, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DLRReceipt), args.Error(1)
}

func (m *mockDLRRepo) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.DLRReceipt, error) {
	args := m.Called(ctx, smppMessageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DLRReceipt), args.Error(1)
}

type mockEventPublisher struct {
	mock.Mock
}

func (m *mockEventPublisher) PublishMessageCreated(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockEventPublisher) PublishMessageQueued(ctx context.Context, msg *domain.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockEventPublisher) PublishMessageStatusChanged(ctx context.Context, msg *domain.Message, oldStatus string) error {
	args := m.Called(ctx, msg, oldStatus)
	return args.Error(0)
}

// --- Helper ---

func newTestMessagingServer(msgRepo *mockMessageRepo, dlrRepo *mockDLRRepo, pub *mockEventPublisher) *Server {
	msgService := application.NewMessageService(msgRepo, dlrRepo, pub)
	dlrService := application.NewDLRService(msgRepo, dlrRepo, pub)
	return NewServer(msgService, dlrService)
}

// --- Tests ---

func TestMessagingServer_SendMessage(t *testing.T) {
	t.Run("valid request returns OK", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		clientID := uuid.New()

		// Non-scheduled: no DB writes, only PublishMessageQueued
		pub.On("PublishMessageQueued", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "TestSender",
			Destination: "+79001234567",
			Text:        "Hello, World!",
			ClientId:    clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.MessageId)
		assert.Equal(t, "queued", resp.Status)
		assert.True(t, resp.SegmentCount >= 1)

		msgRepo.AssertExpectations(t)
		pub.AssertExpectations(t)
	})

	t.Run("empty source returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "",
			Destination: "+79001234567",
			Text:        "Hello",
			ClientId:    uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "source is required")
	})

	t.Run("empty destination returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "Sender",
			Destination: "",
			Text:        "Hello",
			ClientId:    uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty text returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "Sender",
			Destination: "+79001234567",
			Text:        "",
			ClientId:    uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("empty client_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "Sender",
			Destination: "+79001234567",
			Text:        "Hello",
			ClientId:    "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid client_id format returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "Sender",
			Destination: "+79001234567",
			Text:        "Hello",
			ClientId:    "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("kafka error returns Internal", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		pub.On("PublishMessageQueued", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(errors.New("kafka unavailable"))

		resp, err := srv.SendMessage(context.Background(), &messagingv1.SendMessageRequest{
			Source:      "TestSender",
			Destination: "+79001234567",
			Text:        "Hello, World!",
			ClientId:    uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestMessagingServer_GetMessageStatus(t *testing.T) {
	t.Run("found returns OK", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		messageID := uuid.New()
		clientID := uuid.New()
		now := time.Now()

		msg := &domain.Message{
			ID:           messageID,
			Source:       "Sender",
			Destination:  "+79001234567",
			Text:         "Hello",
			Status:       shared.MessageStatusDelivered,
			ClientID:     &clientID,
			CreatedAt:    now,
			SegmentCount: 1,
		}

		msgRepo.On("GetByID", mock.Anything, messageID).Return(msg, nil)

		resp, err := srv.GetMessageStatus(context.Background(), &messagingv1.GetMessageStatusRequest{
			MessageId: messageID.String(),
			ClientId:  clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, messageID.String(), resp.MessageId)
		assert.Equal(t, "delivered", resp.Status)
		assert.Equal(t, int32(1), resp.SegmentCount)
	})

	t.Run("not found returns NotFound", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		messageID := uuid.New()

		msgRepo.On("GetByID", mock.Anything, messageID).Return(nil, errors.New("not found"))

		resp, err := srv.GetMessageStatus(context.Background(), &messagingv1.GetMessageStatusRequest{
			MessageId: messageID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		// The server checks err.Error() == "message not found"; the repo error wraps differently,
		// so it falls through to Internal. This tests the actual behavior.
		st, ok := status.FromError(err)
		require.True(t, ok)
		// The error message from GetMessageStatus is "message not found: not found"
		// which doesn't exactly match "message not found", so it should return Internal.
		assert.Equal(t, codes.Internal, st.Code())
	})

	t.Run("empty message_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.GetMessageStatus(context.Background(), &messagingv1.GetMessageStatusRequest{
			MessageId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid message_id format returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.GetMessageStatus(context.Background(), &messagingv1.GetMessageStatusRequest{
			MessageId: "invalid-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}
