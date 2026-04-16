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
	"github.com/smpp-server/smpp-server/internal/storage"
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

func (m *mockMessageRepo) ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Message, int, error) {
	args := m.Called(ctx, clientID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Message), args.Int(1), args.Error(2)
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

		msgRepo.On("GetByID", mock.Anything, messageID).Return(nil, storage.ErrNotFound)

		resp, err := srv.GetMessageStatus(context.Background(), &messagingv1.GetMessageStatusRequest{
			MessageId: messageID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
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

func TestServer_ListScheduledMessages(t *testing.T) {
	msgRepo := new(mockMessageRepo)
	dlrRepo := new(mockDLRRepo)
	pub := new(mockEventPublisher)
	srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

	clientID := uuid.New()
	messageID := uuid.New()
	now := time.Now()
	scheduledAt := now.Add(time.Hour)

	expectedMsg := &domain.Message{
		ID:           messageID,
		Source:       "Sender",
		Destination:  "+79001234567",
		Text:         "Hello scheduled",
		Status:       shared.MessageStatusScheduled,
		ClientID:     &clientID,
		CreatedAt:    now,
		ScheduledAt:  &scheduledAt,
		SegmentCount: 1,
	}

	// ListScheduledMessages normalizes limit=10 (valid), so repo gets 10, 0
	msgRepo.On("ListScheduled", mock.Anything, clientID, 10, 0).
		Return([]*domain.Message{expectedMsg}, 1, nil)

	req := &messagingv1.ListScheduledMessagesRequest{
		ClientId: clientID.String(),
		Limit:    10,
		Offset:   0,
	}

	resp, err := srv.ListScheduledMessages(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Total)
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, messageID.String(), resp.Messages[0].MessageId)
	assert.Equal(t, "scheduled", resp.Messages[0].Status)
	assert.Equal(t, int32(10), resp.Limit)
	assert.Equal(t, int32(0), resp.Offset)

	msgRepo.AssertExpectations(t)
}

func TestServer_ListScheduledMessages_InvalidClientID(t *testing.T) {
	msgRepo := new(mockMessageRepo)
	dlrRepo := new(mockDLRRepo)
	pub := new(mockEventPublisher)
	srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

	req := &messagingv1.ListScheduledMessagesRequest{
		ClientId: "not-a-valid-uuid",
		Limit:    10,
		Offset:   0,
	}

	resp, err := srv.ListScheduledMessages(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMessagingServer_SendBatch(t *testing.T) {
	t.Run("happy path returns results for all messages", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		clientID := uuid.New()

		// SendBatch calls SendMessage for each item; non-scheduled messages only publish
		pub.On("PublishMessageQueued", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil).Times(2)

		resp, err := srv.SendBatch(context.Background(), &messagingv1.SendBatchRequest{
			ClientId: clientID.String(),
			Messages: []*messagingv1.SendMessageRequest{
				{Source: "Sender", Destination: "+79001234567", Text: "Hello 1"},
				{Source: "Sender", Destination: "+79007654321", Text: "Hello 2"},
			},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(2), resp.SuccessCount)
		assert.Equal(t, int32(0), resp.FailedCount)
		assert.Len(t, resp.Results, 2)
		for _, r := range resp.Results {
			assert.Equal(t, "queued", r.Status)
			assert.NotEmpty(t, r.MessageId)
		}

		pub.AssertExpectations(t)
	})

	t.Run("empty messages list returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendBatch(context.Background(), &messagingv1.SendBatchRequest{
			ClientId: uuid.New().String(),
			Messages: []*messagingv1.SendMessageRequest{},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "messages list is empty")
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.SendBatch(context.Background(), &messagingv1.SendBatchRequest{
			ClientId: "not-a-uuid",
			Messages: []*messagingv1.SendMessageRequest{
				{Source: "Sender", Destination: "+79001234567", Text: "Hello"},
			},
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestMessagingServer_GetMessageHistory(t *testing.T) {
	t.Run("happy path returns messages list", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		clientID := uuid.New()
		msgID1 := uuid.New()
		msgID2 := uuid.New()
		now := time.Now()

		messages := []*domain.Message{
			{
				ID:           msgID1,
				Source:       "Sender",
				Destination:  "+79001234567",
				Text:         "Hello 1",
				Status:       shared.MessageStatusDelivered,
				ClientID:     &clientID,
				CreatedAt:    now,
				SegmentCount: 1,
			},
			{
				ID:           msgID2,
				Source:       "Sender",
				Destination:  "+79007654321",
				Text:         "Hello 2",
				Status:       shared.MessageStatusQueued,
				ClientID:     &clientID,
				CreatedAt:    now,
				SegmentCount: 1,
			},
		}

		// GetMessageHistory calls GetByClientID with limit=100, offset=0, status=nil (no filter)
		msgRepo.On("GetByClientID", mock.Anything, clientID, 100, 0, (*string)(nil)).
			Return(messages, nil)

		resp, err := srv.GetMessageHistory(context.Background(), &messagingv1.GetMessageHistoryRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(2), resp.Total)
		assert.Len(t, resp.Messages, 2)
		assert.Equal(t, msgID1.String(), resp.Messages[0].MessageId)
		assert.Equal(t, "delivered", resp.Messages[0].Status)
		assert.Equal(t, msgID2.String(), resp.Messages[1].MessageId)
		assert.Equal(t, "queued", resp.Messages[1].Status)

		msgRepo.AssertExpectations(t)
	})

	t.Run("repo error returns Internal", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		clientID := uuid.New()

		msgRepo.On("GetByClientID", mock.Anything, clientID, 100, 0, (*string)(nil)).
			Return(nil, errors.New("db connection failed"))

		resp, err := srv.GetMessageHistory(context.Background(), &messagingv1.GetMessageHistoryRequest{
			ClientId: clientID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())

		msgRepo.AssertExpectations(t)
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.GetMessageHistory(context.Background(), &messagingv1.GetMessageHistoryRequest{
			ClientId: "bad-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestMessagingServer_CancelMessage(t *testing.T) {
	t.Run("happy path returns success", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		messageID := uuid.New()
		clientID := uuid.New()

		msgRepo.On("CancelByIDAndStatus", mock.Anything, messageID, clientID).Return(nil)

		resp, err := srv.CancelMessage(context.Background(), &messagingv1.CancelMessageRequest{
			MessageId: messageID.String(),
			ClientId:  clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		msgRepo.AssertExpectations(t)
	})

	t.Run("not found or wrong status returns FailedPrecondition", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		messageID := uuid.New()
		clientID := uuid.New()

		msgRepo.On("CancelByIDAndStatus", mock.Anything, messageID, clientID).
			Return(errors.New("no scheduled message found"))

		resp, err := srv.CancelMessage(context.Background(), &messagingv1.CancelMessageRequest{
			MessageId: messageID.String(),
			ClientId:  clientID.String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
		assert.Contains(t, st.Message(), "message not found or not in scheduled status")

		msgRepo.AssertExpectations(t)
	})

	t.Run("empty message_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.CancelMessage(context.Background(), &messagingv1.CancelMessageRequest{
			MessageId: "",
			ClientId:  uuid.New().String(),
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

		resp, err := srv.CancelMessage(context.Background(), &messagingv1.CancelMessageRequest{
			MessageId: "not-a-uuid",
			ClientId:  uuid.New().String(),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestMessagingServer_ProcessDLR(t *testing.T) {
	t.Run("happy path returns success with updated status", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		messageID := uuid.New()
		smppMessageID := "smpp-12345"
		now := time.Now()

		existingMsg := &domain.Message{
			ID:           messageID,
			Source:       "Sender",
			Destination:  "+79001234567",
			Text:         "Hello",
			Status:       shared.MessageStatusSent,
			CreatedAt:    now,
			SegmentCount: 1,
		}

		updatedMsg := &domain.Message{
			ID:           messageID,
			Source:       "Sender",
			Destination:  "+79001234567",
			Text:         "Hello",
			Status:       shared.MessageStatusDelivered,
			CreatedAt:    now,
			SegmentCount: 1,
		}

		// ProcessDLR tries GetBySMPPMessageID first
		msgRepo.On("GetBySMPPMessageID", mock.Anything, smppMessageID).Return(existingMsg, nil)
		dlrRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)
		msgRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Message")).Return(nil)
		pub.On("PublishMessageStatusChanged", mock.Anything, mock.AnythingOfType("*domain.Message"), string(shared.MessageStatusSent)).Return(nil)
		// After ProcessDLR, server calls GetMessageStatus → GetByID
		msgRepo.On("GetByID", mock.Anything, messageID).Return(updatedMsg, nil)

		resp, err := srv.ProcessDLR(context.Background(), &messagingv1.ProcessDLRRequest{
			MessageId:     messageID.String(),
			SmppMessageId: smppMessageID,
			Stat:          "DELIVRD",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.Equal(t, "delivered", resp.UpdatedStatus)

		msgRepo.AssertExpectations(t)
		dlrRepo.AssertExpectations(t)
		pub.AssertExpectations(t)
	})

	t.Run("empty message_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.ProcessDLR(context.Background(), &messagingv1.ProcessDLRRequest{
			MessageId:     "",
			SmppMessageId: "smpp-123",
			Stat:          "DELIVRD",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "message_id is required")
	})

	t.Run("empty smpp_message_id returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.ProcessDLR(context.Background(), &messagingv1.ProcessDLRRequest{
			MessageId:     uuid.New().String(),
			SmppMessageId: "",
			Stat:          "DELIVRD",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "smpp_message_id is required")
	})

	t.Run("empty stat returns InvalidArgument", func(t *testing.T) {
		msgRepo := new(mockMessageRepo)
		dlrRepo := new(mockDLRRepo)
		pub := new(mockEventPublisher)
		srv := newTestMessagingServer(msgRepo, dlrRepo, pub)

		resp, err := srv.ProcessDLR(context.Background(), &messagingv1.ProcessDLRRequest{
			MessageId:     uuid.New().String(),
			SmppMessageId: "smpp-123",
			Stat:          "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "stat is required")
	})
}
