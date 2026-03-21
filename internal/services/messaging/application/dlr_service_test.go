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

// newTestDLRService создает DLRService с чистыми моками для каждого теста
func newTestDLRService() (*DLRService, *mocks.MockMessageRepository, *mocks.MockDLRRepository, *mocks.MockEventPublisher) {
	msgRepo := new(mocks.MockMessageRepository)
	dlrRepo := new(mocks.MockDLRRepository)
	publisher := new(mocks.MockEventPublisher)
	svc := NewDLRService(msgRepo, dlrRepo, publisher)
	return svc, msgRepo, dlrRepo, publisher
}

func TestDLRService(t *testing.T) {
	t.Run("ProcessDLR", func(t *testing.T) {
		t.Run("DELIVRD updates message to DELIVERED and creates DLR receipt", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-12345"
			now := time.Now()

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			// Поиск по SMPP message ID
			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			// Создание DLR receipt
			dlrRepo.On("Create", ctx, mock.MatchedBy(func(dlr *domain.DLRReceipt) bool {
				return dlr.MessageID == messageID &&
					dlr.SMPPMessageID == smppMsgID &&
					dlr.Stat == "DELIVRD" &&
					dlr.DoneDate != nil
			})).Return(nil)

			// Обновление сообщения с новым статусом
			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == messageID &&
					msg.Status == shared.MessageStatusDelivered &&
					msg.DeliveredAt != nil
			})).Return(nil)

			// Публикация события смены статуса (sent -> delivered)
			publisher.On("PublishMessageStatusChanged", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusDelivered
			}), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", &now, nil, "", nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("UNDELIV updates message to FAILED", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-67890"
			errCode := 42
			errText := "subscriber not reachable"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.MatchedBy(func(dlr *domain.DLRReceipt) bool {
				return dlr.MessageID == messageID &&
					dlr.Stat == "UNDELIV" &&
					dlr.Err != nil && *dlr.Err == 42 &&
					dlr.Text == "subscriber not reachable"
			})).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.ID == messageID &&
					msg.Status == shared.MessageStatusFailed &&
					msg.StatusMessage == "subscriber not reachable" &&
					msg.FailedAt != nil
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusFailed
			}), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "UNDELIV", nil, &errCode, errText, nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("EXPIRED updates message to EXPIRED", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-expired"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.MatchedBy(func(dlr *domain.DLRReceipt) bool {
				return dlr.Stat == "EXPIRED"
			})).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusExpired &&
					msg.ExpiredAt != nil
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "EXPIRED", nil, nil, "", nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("REJECTD updates message to FAILED", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-rejected"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.MatchedBy(func(dlr *domain.DLRReceipt) bool {
				return dlr.Stat == "REJECTD"
			})).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusFailed
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "REJECTD", nil, nil, "rejected by operator", nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("fallback to GetByID when SMPP message ID lookup fails", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-unknown"

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusSent,
			}

			// GetBySMPPMessageID не находит — фолбэк на GetByID
			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(nil, errors.New("not found"))
			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusDelivered
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", nil, nil, "", nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("empty SMPP message ID uses GetByID directly", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()

			existingMsg := &domain.Message{
				ID:     messageID,
				Status: shared.MessageStatusSent,
			}

			// Пустой smppMessageID — напрямую GetByID
			msgRepo.On("GetByID", ctx, messageID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusDelivered
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, "", "DELIVRD", nil, nil, "", nil)

			require.NoError(t, err)

			// GetBySMPPMessageID не должен быть вызван
			msgRepo.AssertNotCalled(t, "GetBySMPPMessageID")
			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("message not found returns error", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-nonexistent"

			// Оба способа поиска провалились
			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(nil, errors.New("not found"))
			msgRepo.On("GetByID", ctx, messageID).Return(nil, errors.New("not found"))

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", nil, nil, "", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "message not found")

			dlrRepo.AssertNotCalled(t, "Create")
			publisher.AssertNotCalled(t, "PublishMessageStatusChanged")
			msgRepo.AssertExpectations(t)
		})

		t.Run("DLR repository Create error returns error", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-dlr-fail"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).
				Return(errors.New("db write error"))

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", nil, nil, "", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to save DLR receipt")

			// Update не должен быть вызван, т.к. сохранение DLR провалилось
			msgRepo.AssertNotCalled(t, "Update")
			publisher.AssertNotCalled(t, "PublishMessageStatusChanged")

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
		})

		t.Run("message Update error returns error", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-update-fail"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)
			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)
			msgRepo.On("Update", ctx, mock.AnythingOfType("*domain.Message")).
				Return(errors.New("db update error"))

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", nil, nil, "", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update message status")

			// PublishMessageStatusChanged не вызывается, т.к. Update упал
			publisher.AssertNotCalled(t, "PublishMessageStatusChanged")

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
		})

		t.Run("unknown stat with non-zero error code marks message as FAILED", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-unknown-stat"
			errCode := 99

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)

			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusFailed
			})).Return(nil)

			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "UNKNOWN_STAT", nil, &errCode, "custom error", nil)

			require.NoError(t, err)

			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})

		t.Run("unknown stat without error code does not change message status", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-unknown-no-err"

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.AnythingOfType("*domain.DLRReceipt")).Return(nil)

			// Статус не меняется → Update вызывается, но статус остается sent
			msgRepo.On("Update", ctx, mock.MatchedBy(func(msg *domain.Message) bool {
				return msg.Status == shared.MessageStatusSent
			})).Return(nil)

			// Статус не изменился → PublishMessageStatusChanged не вызывается
			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "ACCEPTD", nil, nil, "", nil)

			require.NoError(t, err)

			publisher.AssertNotCalled(t, "PublishMessageStatusChanged")
			msgRepo.AssertExpectations(t)
			dlrRepo.AssertExpectations(t)
		})

		t.Run("DLR with providerID sets providerID on receipt", func(t *testing.T) {
			svc, msgRepo, dlrRepo, publisher := newTestDLRService()
			ctx := context.Background()
			messageID := uuid.New()
			smppMsgID := "smpp-with-provider"
			providerID := uuid.New()

			existingMsg := &domain.Message{
				ID:            messageID,
				Status:        shared.MessageStatusSent,
				SMPPMessageID: smppMsgID,
			}

			msgRepo.On("GetBySMPPMessageID", ctx, smppMsgID).Return(existingMsg, nil)

			dlrRepo.On("Create", ctx, mock.MatchedBy(func(dlr *domain.DLRReceipt) bool {
				return dlr.ProviderID != nil && *dlr.ProviderID == providerID
			})).Return(nil)

			msgRepo.On("Update", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)
			publisher.On("PublishMessageStatusChanged", ctx, mock.AnythingOfType("*domain.Message"), "sent").Return(nil)

			err := svc.ProcessDLR(ctx, messageID, smppMsgID, "DELIVRD", nil, nil, "", &providerID)

			require.NoError(t, err)

			dlrRepo.AssertExpectations(t)
			msgRepo.AssertExpectations(t)
			publisher.AssertExpectations(t)
		})
	})
}
