package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

// DLRService предоставляет бизнес-логику для обработки DLR receipts
type DLRService struct {
	messageRepo   domain.MessageRepository
	dlrRepo       domain.DLRRepository
	eventPublisher domain.EventPublisher
}

// NewDLRService создает новый сервис DLR
func NewDLRService(
	messageRepo domain.MessageRepository,
	dlrRepo domain.DLRRepository,
	eventPublisher domain.EventPublisher,
) *DLRService {
	return &DLRService{
		messageRepo:    messageRepo,
		dlrRepo:        dlrRepo,
		eventPublisher: eventPublisher,
	}
}

// ProcessDLR обрабатывает delivery receipt
func (s *DLRService) ProcessDLR(
	ctx context.Context,
	messageID uuid.UUID,
	smppMessageID string,
	stat string,
	doneDate *time.Time,
	errCode *int,
	errText string,
	providerID *uuid.UUID,
) error {
	log.Info().
		Str("message_id", messageID.String()).
		Str("smpp_message_id", smppMessageID).
		Str("stat", stat).
		Msg("обработка DLR")

	// Получаем сообщение по SMPP message_id или по message_id
	var msg *domain.Message
	var err error

	if smppMessageID != "" {
		msg, err = s.messageRepo.GetBySMPPMessageID(ctx, smppMessageID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("smpp_message_id", smppMessageID).
				Msg("сообщение не найдено по SMPP message_id, пробуем по message_id")
			// Пробуем по message_id
			msg, err = s.messageRepo.GetByID(ctx, messageID)
			if err != nil {
				return fmt.Errorf("message not found: %w", err)
			}
		}
	} else {
		msg, err = s.messageRepo.GetByID(ctx, messageID)
		if err != nil {
			return fmt.Errorf("message not found: %w", err)
		}
	}

	// Создаем DLR receipt
	dlr := domain.NewDLRReceipt(msg.ID, smppMessageID, stat)
	if doneDate != nil {
		dlr.DoneDate = doneDate
	}
	if errCode != nil {
		dlr.Err = errCode
	}
	if errText != "" {
		dlr.Text = errText
	}
	if providerID != nil {
		dlr.ProviderID = providerID
	}

	// Сохраняем DLR receipt
	if err := s.dlrRepo.Create(ctx, dlr); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения DLR receipt")
		return fmt.Errorf("failed to save DLR receipt: %w", err)
	}

	// Обновляем статус сообщения на основе DLR
	oldStatus := string(msg.Status)

	switch stat {
	case "DELIVRD":
		msg.MarkAsDelivered()
	case "EXPIRED":
		msg.MarkAsExpired()
	case "REJECTD", "UNDELIV":
		msg.MarkAsFailed(errText)
	default:
		// Для других статусов оставляем текущий статус или обрабатываем по коду ошибки
		if errCode != nil && *errCode != 0 {
			msg.MarkAsFailed(errText)
		}
		log.Debug().
			Str("message_id", msg.ID.String()).
			Str("stat", stat).
			Msg("неизвестный статус DLR")
	}

	// Обновляем сообщение
	if err := s.messageRepo.Update(ctx, msg); err != nil {
		log.Error().
			Err(err).
			Str("message_id", msg.ID.String()).
			Msg("ошибка обновления статуса сообщения по DLR")
		return fmt.Errorf("failed to update message status: %w", err)
	}

	// Публикуем событие изменения статуса, если он изменился
	if oldStatus != string(msg.Status) {
		if err := s.eventPublisher.PublishMessageStatusChanged(ctx, msg, oldStatus); err != nil {
			log.Warn().Err(err).Msg("ошибка публикации события message.status.changed")
		}
	}

	log.Info().
		Str("message_id", msg.ID.String()).
		Str("old_status", oldStatus).
		Str("new_status", string(msg.Status)).
		Msg("статус сообщения обновлен по DLR")

	return nil
}
