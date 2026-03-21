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

// newTestScheduler создает Scheduler с чистыми моками для каждого теста
func newTestScheduler() (*Scheduler, *mocks.MockMessageRepository, *mocks.MockEventPublisher) {
	msgRepo := new(mocks.MockMessageRepository)
	publisher := new(mocks.MockEventPublisher)
	scheduler := NewScheduler(msgRepo, publisher, 10*time.Second, 100, 5*time.Minute)
	return scheduler, msgRepo, publisher
}

func TestScheduler(t *testing.T) {
	t.Run("NewScheduler", func(t *testing.T) {
		t.Run("creates scheduler with correct configuration", func(t *testing.T) {
			msgRepo := new(mocks.MockMessageRepository)
			publisher := new(mocks.MockEventPublisher)

			scheduler := NewScheduler(msgRepo, publisher, 15*time.Second, 50, 10*time.Minute)

			require.NotNil(t, scheduler)
			assert.Equal(t, 15*time.Second, scheduler.interval)
			assert.Equal(t, 50, scheduler.batchSize)
			assert.Equal(t, 10*time.Minute, scheduler.stuckThreshold)
			assert.NotNil(t, scheduler.ctx)
			assert.NotNil(t, scheduler.cancel)
		})
	})

	t.Run("Stop", func(t *testing.T) {
		t.Run("cancels context", func(t *testing.T) {
			scheduler, _, _ := newTestScheduler()

			scheduler.Stop()

			// Контекст должен быть отменён
			assert.Error(t, scheduler.ctx.Err())
			assert.Equal(t, context.Canceled, scheduler.ctx.Err())
		})
	})

	t.Run("processBatch", func(t *testing.T) {
		t.Run("dispatches scheduled messages to Kafka", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msg1ID := uuid.New()
			msg2ID := uuid.New()

			scheduledMessages := []*domain.Message{
				{
					ID:     msg1ID,
					Status: shared.MessageStatusScheduled,
					Source: "Sender1", Destination: "+79001234567", Text: "Scheduled 1",
				},
				{
					ID:     msg2ID,
					Status: shared.MessageStatusScheduled,
					Source: "Sender2", Destination: "+79002345678", Text: "Scheduled 2",
				},
			}

			msgRepo.On("GetScheduledReady", mock.Anything, 100).Return(scheduledMessages, nil)

			// Для каждого сообщения: UpdateStatus(pending) -> PublishMessageQueued -> UpdateStatus(queued)
			msgRepo.On("UpdateStatus", mock.Anything, msg1ID, "pending", "").Return(nil)
			msgRepo.On("UpdateStatus", mock.Anything, msg2ID, "pending", "").Return(nil)

			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msg1ID && msg.Status == "pending"
			})).Return(nil)
			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msg2ID && msg.Status == "pending"
			})).Return(nil)

			msgRepo.On("UpdateStatus", mock.Anything, msg1ID, "queued", "").Return(nil)
			msgRepo.On("UpdateStatus", mock.Anything, msg2ID, "queued", "").Return(nil)

			scheduler.processBatch()

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("no scheduled messages does nothing", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgRepo.On("GetScheduledReady", mock.Anything, 100).Return([]*domain.Message{}, nil)

			scheduler.processBatch()

			// Ничего кроме GetScheduledReady не должно быть вызвано
			publisher.AssertNotCalled(t, "PublishMessageQueued")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			msgRepo.AssertExpectations(t)
		})

		t.Run("GetScheduledReady error logs and returns without processing", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgRepo.On("GetScheduledReady", mock.Anything, 100).
				Return(nil, errors.New("db connection error"))

			scheduler.processBatch()

			publisher.AssertNotCalled(t, "PublishMessageQueued")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			msgRepo.AssertExpectations(t)
		})

		t.Run("UpdateStatus to pending failure skips message", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msg1ID := uuid.New()
			msg2ID := uuid.New()

			scheduledMessages := []*domain.Message{
				{ID: msg1ID, Status: shared.MessageStatusScheduled},
				{ID: msg2ID, Status: shared.MessageStatusScheduled},
			}

			msgRepo.On("GetScheduledReady", mock.Anything, 100).Return(scheduledMessages, nil)

			// Первое сообщение — UpdateStatus("pending") проваливается
			msgRepo.On("UpdateStatus", mock.Anything, msg1ID, "pending", "").
				Return(errors.New("lock contention"))

			// Второе сообщение — обрабатывается нормально
			msgRepo.On("UpdateStatus", mock.Anything, msg2ID, "pending", "").Return(nil)
			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msg2ID
			})).Return(nil)
			msgRepo.On("UpdateStatus", mock.Anything, msg2ID, "queued", "").Return(nil)

			scheduler.processBatch()

			// PublishMessageQueued не должен быть вызван для первого сообщения
			publisher.AssertNumberOfCalls(t, "PublishMessageQueued", 1)
			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("PublishMessageQueued failure reverts status to scheduled", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgID := uuid.New()

			scheduledMessages := []*domain.Message{
				{ID: msgID, Status: shared.MessageStatusScheduled},
			}

			msgRepo.On("GetScheduledReady", mock.Anything, 100).Return(scheduledMessages, nil)

			// UpdateStatus(pending) — OK
			msgRepo.On("UpdateStatus", mock.Anything, msgID, "pending", "").Return(nil)

			// PublishMessageQueued — провал
			publisher.On("PublishMessageQueued", mock.Anything, mock.AnythingOfType("*domain.Message")).
				Return(errors.New("kafka unavailable"))

			// Откат статуса на scheduled
			msgRepo.On("UpdateStatus", mock.Anything, msgID, "scheduled", "").Return(nil)

			scheduler.processBatch()

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	})

	t.Run("recoverStuckMessages", func(t *testing.T) {
		t.Run("re-publishes stuck pending messages", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgID := uuid.New()

			stuckMessages := []*domain.Message{
				{ID: msgID, Status: shared.MessageStatusPending},
			}

			msgRepo.On("GetStuckPending", mock.Anything, 5*time.Minute, 100).Return(stuckMessages, nil)

			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msgID
			})).Return(nil)

			// Обновление updated_at через UpdateStatus
			msgRepo.On("UpdateStatus", mock.Anything, msgID, "pending", "").Return(nil)

			scheduler.recoverStuckMessages()

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("no stuck messages does nothing", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgRepo.On("GetStuckPending", mock.Anything, 5*time.Minute, 100).
				Return([]*domain.Message{}, nil)

			scheduler.recoverStuckMessages()

			publisher.AssertNotCalled(t, "PublishMessageQueued")
			msgRepo.AssertNotCalled(t, "UpdateStatus")
			msgRepo.AssertExpectations(t)
		})

		t.Run("GetStuckPending error logs and returns", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msgRepo.On("GetStuckPending", mock.Anything, 5*time.Minute, 100).
				Return(nil, errors.New("db error"))

			scheduler.recoverStuckMessages()

			publisher.AssertNotCalled(t, "PublishMessageQueued")
			msgRepo.AssertExpectations(t)
		})

		t.Run("re-publish failure skips message and continues", func(t *testing.T) {
			scheduler, msgRepo, publisher := newTestScheduler()

			msg1ID := uuid.New()
			msg2ID := uuid.New()

			stuckMessages := []*domain.Message{
				{ID: msg1ID, Status: shared.MessageStatusPending},
				{ID: msg2ID, Status: shared.MessageStatusPending},
			}

			msgRepo.On("GetStuckPending", mock.Anything, 5*time.Minute, 100).Return(stuckMessages, nil)

			// Первое сообщение — публикация провалилась
			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msg1ID
			})).Return(errors.New("kafka error"))

			// Второе сообщение — публикация ОК
			publisher.On("PublishMessageQueued", mock.Anything, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == msg2ID
			})).Return(nil)

			// UpdateStatus вызывается только для второго (успешного) сообщения
			msgRepo.On("UpdateStatus", mock.Anything, msg2ID, "pending", "").Return(nil)

			scheduler.recoverStuckMessages()

			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
			// UpdateStatus не должен быть вызван для первого сообщения
			msgRepo.AssertNumberOfCalls(t, "UpdateStatus", 1)
		})
	})
}
