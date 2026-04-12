package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/services/messaging/mocks"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// newTestMessageService создает MessageService с чистыми моками для каждого теста
func newTestMessageService() (*MessageService, *mocks.MockMessageRepository, *mocks.MockDLRRepository, *mocks.MockEventPublisher) {
	msgRepo := new(mocks.MockMessageRepository)
	dlrRepo := new(mocks.MockDLRRepository)
	publisher := new(mocks.MockEventPublisher)
	svc := NewMessageService(msgRepo, dlrRepo, publisher)
	return svc, msgRepo, dlrRepo, publisher
}

func TestMessageService(t *testing.T) {
	t.Run("SendMessage", func(t *testing.T) {
		t.Run("valid message publishes to Kafka without DB write", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			// Non-scheduled: no DB writes, only PublishMessageQueued
			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)

			msg, err := svc.SendMessage(ctx, clientID, "TestSender", "+79001234567", "Hello, World!", nil)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, "TestSender", msg.Source)
			assert.Equal(t, "+79001234567", msg.Destination)
			assert.Equal(t, "Hello, World!", msg.Text)
			assert.Equal(t, shared.MessageStatusQueued, msg.Status)
			assert.NotEqual(t, uuid.Nil, msg.ID)
			assert.Equal(t, shared.MessageEncodingGSM7, msg.Encoding)
			assert.Equal(t, 1, msg.SegmentCount)

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertExpectations(t)
		})

		t.Run("valid message with options applies options correctly", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			regDelivery := true
			options := &SendMessageOptions{
				ExternalID:         "ext-123",
				Priority:           2,
				RegisteredDelivery: &regDelivery,
				ServiceType:        "transactional",
				MaxRetries:         10,
			}

			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "Test message", options)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, "ext-123", msg.ExternalID)
			assert.Equal(t, 2, msg.PriorityFlag)
			assert.Equal(t, 1, msg.RegisteredDelivery)
			assert.Equal(t, "transactional", msg.ServiceType)
			assert.Equal(t, 10, msg.MaxRetries)

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertExpectations(t)
		})

		t.Run("empty destination returns validation error", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "", "Hello", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "validation failed")
			assert.Contains(t, err.Error(), "destination")

			// Не должно быть вызовов Create и Publish — валидация провалилась до них
			msgRepo.AssertNotCalled(t, "Create")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertNotCalled(t, "PublishMessageQueued")
		})

		t.Run("empty source returns validation error", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msg, err := svc.SendMessage(ctx, clientID, "", "+79001234567", "Hello", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "validation failed")
			assert.Contains(t, err.Error(), "source")

			msgRepo.AssertNotCalled(t, "Create")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
		})

		t.Run("empty text returns validation error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "validation failed")
			assert.Contains(t, err.Error(), "text")

			msgRepo.AssertNotCalled(t, "Create")
		})

		t.Run("PublishMessageQueued error returns error without DB write", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).
				Return(errors.New("kafka unavailable"))

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "Hello", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "failed to publish message to queue")

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertExpectations(t)
		})


		t.Run("UCS2 encoding detected for non-ASCII text", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "Привет мир!", nil)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, shared.MessageEncodingUCS2, msg.Encoding)

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertExpectations(t)
		})

		t.Run("destination with invalid characters returns validation error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "abc_invalid", "Hello", nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "validation failed")

			msgRepo.AssertNotCalled(t, "Create")
		})

		t.Run("scheduled message saves without publishing events", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			scheduledAt := time.Now().Add(1 * time.Hour)
			options := &SendMessageOptions{
				ScheduledAt: &scheduledAt,
			}

			msgRepo.On("Create", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusScheduled
			})).Return(nil)

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "Scheduled msg", options)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, shared.MessageStatusScheduled, msg.Status)

			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertNotCalled(t, "PublishMessageQueued")
			msgRepo.AssertExpectations(t)
		})

		t.Run("scheduled message beyond 30 days returns error", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			scheduledAt := time.Now().Add(31 * 24 * time.Hour)
			options := &SendMessageOptions{
				ScheduledAt: &scheduledAt,
			}

			msg, err := svc.SendMessage(ctx, clientID, "Sender", "+79001234567", "Too far ahead", options)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "30 days")

			msgRepo.AssertNotCalled(t, "Create")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertNotCalled(t, "PublishMessageQueued")
		})

	})

	t.Run("GetMessageStatus", func(t *testing.T) {
		t.Run("existing message returns message", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			clientID := uuid.New()

			expectedMsg := &domain.Message{
				ID:          messageID,
				ClientID:    &clientID,
				Source:      "Sender",
				Destination: "+79001234567",
				Text:        "Test",
				Status:      shared.MessageStatusDelivered,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(expectedMsg, nil)

			msg, err := svc.GetMessageStatus(ctx, messageID, &clientID)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, messageID, msg.ID)
			assert.Equal(t, shared.MessageStatusDelivered, msg.Status)
			assert.Equal(t, "Sender", msg.Source)
			assert.Equal(t, "+79001234567", msg.Destination)

			msgRepo.AssertExpectations(t)
		})

		t.Run("non-existing message returns error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			msgRepo.On("GetByID", ctx, messageID).Return(nil, errors.New("not found"))

			msg, err := svc.GetMessageStatus(ctx, messageID, nil)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "message not found")

			msgRepo.AssertExpectations(t)
		})

		t.Run("access denied for different clientID", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			ownerID := uuid.New()
			otherID := uuid.New()

			expectedMsg := &domain.Message{
				ID:       messageID,
				ClientID: &ownerID,
				Status:   shared.MessageStatusSent,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(expectedMsg, nil)

			msg, err := svc.GetMessageStatus(ctx, messageID, &otherID)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.Contains(t, err.Error(), "access denied")

			msgRepo.AssertExpectations(t)
		})

		t.Run("nil clientID skips access check", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			ownerID := uuid.New()

			expectedMsg := &domain.Message{
				ID:       messageID,
				ClientID: &ownerID,
				Status:   shared.MessageStatusSent,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(expectedMsg, nil)

			msg, err := svc.GetMessageStatus(ctx, messageID, nil)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, messageID, msg.ID)

			msgRepo.AssertExpectations(t)
		})
	})

	t.Run("CancelMessage", func(t *testing.T) {
		t.Run("pending message cancels successfully", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			clientID := uuid.New()

			// CancelByIDAndStatus делегирует логику проверки статуса в репозиторий
			msgRepo.On("CancelByIDAndStatus", ctx, messageID, clientID).Return(nil)

			err := svc.CancelMessage(ctx, messageID, clientID)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
		})

		t.Run("delivered message returns error (can't cancel)", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			clientID := uuid.New()

			// Репозиторий отклоняет отмену, т.к. сообщение уже доставлено
			msgRepo.On("CancelByIDAndStatus", ctx, messageID, clientID).
				Return(errors.New("message cannot be cancelled: status is delivered"))

			err := svc.CancelMessage(ctx, messageID, clientID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be cancelled")

			msgRepo.AssertExpectations(t)
		})

		t.Run("non-existing message returns error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()
			clientID := uuid.New()

			msgRepo.On("CancelByIDAndStatus", ctx, messageID, clientID).
				Return(errors.New("message not found"))

			err := svc.CancelMessage(ctx, messageID, clientID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not found")

			msgRepo.AssertExpectations(t)
		})
	})

	t.Run("UpdateMessageStatus", func(t *testing.T) {
		t.Run("updates to delivered and publishes status change event", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusSent,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "delivered", "").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "delivered", "")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("updates to failed with reason", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusSent,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "failed", "network timeout").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "failed", "network timeout")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("invalid status returns error", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusPending,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "nonexistent_status", "")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid status")

			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageStatusChanged")
			msgRepo.AssertExpectations(t)
		})

		t.Run("message not found returns error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			msgRepo.On("GetByID", ctx, messageID).Return(nil, errors.New("not found"))

			err := svc.UpdateMessageStatus(ctx, messageID, "delivered", "")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "message not found")

			msgRepo.AssertExpectations(t)
		})

		t.Run("updates to queued status", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusPending,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "queued", "").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "pending").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "queued", "")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("updates to sent status", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusQueued,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "sent", "").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "queued").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "sent", "")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("updates to expired status", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusSent,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "expired", "").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "expired", "")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("updates to rejected status", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusPending,
			}

			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)
			msgRepo.On("UpdateStatus", ctx, messageID, "rejected", "blocked").Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "pending").Return(nil)

			err := svc.UpdateMessageStatus(ctx, messageID, "rejected", "blocked")

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	})

	t.Run("GetMessageHistory", func(t *testing.T) {
		t.Run("returns messages with default pagination", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			expectedMessages := []*domain.Message{
				{ID: uuid.New(), Status: shared.MessageStatusDelivered},
				{ID: uuid.New(), Status: shared.MessageStatusSent},
			}

			// Без фильтров — значения по умолчанию: limit=100, offset=0, status=nil
			msgRepo.On("GetByClientID", ctx, clientID, 100, 0, (*string)(nil)).Return(expectedMessages, nil)

			messages, err := svc.GetMessageHistory(ctx, clientID, nil)

			require.NoError(t, err)
			require.Len(t, messages, 2)

			msgRepo.AssertExpectations(t)
		})

		t.Run("applies filters correctly", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			statusStr := "delivered"
			filters := &MessageHistoryFilters{
				Status: "delivered",
				Limit:  50,
				Offset: 10,
			}

			expectedMessages := []*domain.Message{
				{ID: uuid.New(), Status: shared.MessageStatusDelivered},
			}

			msgRepo.On("GetByClientID", ctx, clientID, 50, 10, &statusStr).Return(expectedMessages, nil)

			messages, err := svc.GetMessageHistory(ctx, clientID, filters)

			require.NoError(t, err)
			require.Len(t, messages, 1)

			msgRepo.AssertExpectations(t)
		})

		t.Run("repository error returns error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msgRepo.On("GetByClientID", ctx, clientID, 100, 0, (*string)(nil)).
				Return(nil, errors.New("db error"))

			messages, err := svc.GetMessageHistory(ctx, clientID, nil)

			require.Error(t, err)
			assert.Nil(t, messages)
			assert.Contains(t, err.Error(), "failed to get message history")

			msgRepo.AssertExpectations(t)
		})
	})

	t.Run("ListScheduledMessages", func(t *testing.T) {
		t.Run("happy path returns messages and total count", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			expectedMessages := []*domain.Message{
				{ID: uuid.New(), Status: shared.MessageStatusScheduled},
				{ID: uuid.New(), Status: shared.MessageStatusScheduled},
			}
			expectedTotal := 5

			msgRepo.On("ListScheduled", ctx, clientID, 10, 0).Return(expectedMessages, expectedTotal, nil)

			messages, total, err := svc.ListScheduledMessages(ctx, clientID, 10, 0)

			require.NoError(t, err)
			require.Len(t, messages, 2)
			assert.Equal(t, expectedTotal, total)
			assert.Equal(t, shared.MessageStatusScheduled, messages[0].Status)
			assert.Equal(t, shared.MessageStatusScheduled, messages[1].Status)

			msgRepo.AssertExpectations(t)
		})

		t.Run("invalid limit uses default 100", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msgRepo.On("ListScheduled", ctx, clientID, 100, 0).Return([]*domain.Message{}, 0, nil)

			messages, total, err := svc.ListScheduledMessages(ctx, clientID, 0, 0)

			require.NoError(t, err)
			assert.Empty(t, messages)
			assert.Equal(t, 0, total)

			msgRepo.AssertExpectations(t)
		})

		t.Run("limit above 1000 uses default 100", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msgRepo.On("ListScheduled", ctx, clientID, 100, 0).Return([]*domain.Message{}, 0, nil)

			messages, total, err := svc.ListScheduledMessages(ctx, clientID, 9999, 0)

			require.NoError(t, err)
			assert.Empty(t, messages)
			assert.Equal(t, 0, total)

			msgRepo.AssertExpectations(t)
		})

		t.Run("negative offset uses 0", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msgRepo.On("ListScheduled", ctx, clientID, 10, 0).Return([]*domain.Message{}, 0, nil)

			messages, total, err := svc.ListScheduledMessages(ctx, clientID, 10, -5)

			require.NoError(t, err)
			assert.Empty(t, messages)
			assert.Equal(t, 0, total)

			msgRepo.AssertExpectations(t)
		})

		t.Run("repository error returns error", func(t *testing.T) {
			svc, msgRepo, _, _ := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			msgRepo.On("ListScheduled", ctx, clientID, 10, 0).
				Return(nil, 0, errors.New("db error"))

			messages, total, err := svc.ListScheduledMessages(ctx, clientID, 10, 0)

			require.Error(t, err)
			assert.Nil(t, messages)
			assert.Equal(t, 0, total)
			assert.Contains(t, err.Error(), "db error")

			msgRepo.AssertExpectations(t)
		})
	})

	t.Run("SendBatch", func(t *testing.T) {
		t.Run("sends multiple messages and returns results", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			requests := []*SendMessageRequest{
				{Source: "Sender", Destination: "+79001234567", Text: "Message 1"},
				{Source: "Sender", Destination: "+79002345678", Text: "Message 2"},
			}

			// Non-scheduled: only PublishMessageQueued, no DB writes
			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).Return(nil).Times(2)

			results, err := svc.SendBatch(ctx, clientID, requests, nil)

			require.NoError(t, err)
			require.Len(t, results, 2)
			assert.True(t, results[0].Success)
			assert.True(t, results[1].Success)
			assert.NotEqual(t, uuid.Nil, results[0].MessageID)
			assert.NotEqual(t, uuid.Nil, results[1].MessageID)

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertExpectations(t)
		})

		t.Run("empty batch returns empty results", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			requests := []*SendMessageRequest{}

			results, err := svc.SendBatch(ctx, clientID, requests, nil)

			require.NoError(t, err)
			require.Len(t, results, 0)

			msgRepo.AssertNotCalled(t, "Create")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertNotCalled(t, "PublishMessageQueued")
		})

		t.Run("partial failures in batch do not stop processing", func(t *testing.T) {
			svc, msgRepo, _, publisher := newTestMessageService()
			ctx := context.Background()
			clientID := uuid.New()

			requests := []*SendMessageRequest{
				{Source: "Sender", Destination: "", Text: "Bad message"},           // Невалидный — пустой destination
				{Source: "Sender", Destination: "+79002345678", Text: "Good msg"}, // Валидный
			}

			// Only second message reaches PublishMessageQueued; no DB writes
			publisher.On("PublishMessageQueued", ctx, mock.AnythingOfType("*domain.Message")).Return(nil).Once()

			results, err := svc.SendBatch(ctx, clientID, requests, nil)

			require.NoError(t, err)
			require.Len(t, results, 2)
			assert.False(t, results[0].Success)
			assert.Contains(t, results[0].Error, "validation failed")
			assert.True(t, results[1].Success)

			msgRepo.AssertNotCalled(t, "Create")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			publisher.AssertNotCalled(t, "PublishMessageCreated")
			publisher.AssertExpectations(t)
		})
	})
}
