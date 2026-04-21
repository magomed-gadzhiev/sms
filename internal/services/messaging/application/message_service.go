package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageService предоставляет бизнес-логику для работы с сообщениями
type MessageService struct {
	messageRepo   domain.MessageRepository
	dlrRepo       domain.DLRRepository
	validator     *domain.MessageValidator
	eventPublisher domain.EventPublisher
}

// NewMessageService создает новый сервис сообщений
func NewMessageService(
	messageRepo domain.MessageRepository,
	dlrRepo domain.DLRRepository,
	eventPublisher domain.EventPublisher,
) *MessageService {
	return &MessageService{
		messageRepo:    messageRepo,
		dlrRepo:        dlrRepo,
		validator:      domain.NewMessageValidator(),
		eventPublisher: eventPublisher,
	}
}

// SendMessage создает и отправляет новое сообщение
func (s *MessageService) SendMessage(
	ctx context.Context,
	clientID uuid.UUID,
	source, destination, text string,
	options *SendMessageOptions,
) (*domain.Message, error) {
	// Создаем доменную модель
	msg := domain.NewMessage(clientID, source, destination, text)

	// Применяем опции
	if options != nil {
		if options.ExternalID != "" {
			msg.ExternalID = options.ExternalID
		}
		if options.Priority >= 0 && options.Priority <= 3 {
			msg.PriorityFlag = options.Priority
		}
		if options.RegisteredDelivery != nil {
			if *options.RegisteredDelivery {
				msg.RegisteredDelivery = 1
			} else {
				msg.RegisteredDelivery = 0
			}
		}
		if options.ValidityPeriod != nil {
			msg.ValidityPeriod = options.ValidityPeriod
		}
		if options.ServiceType != "" {
			msg.ServiceType = options.ServiceType
		}
		if options.SourceAddrTON >= 0 {
			msg.SourceAddrTON = options.SourceAddrTON
		}
		if options.SourceAddrNPI >= 0 {
			msg.SourceAddrNPI = options.SourceAddrNPI
		}
		if options.DestAddrTON >= 0 {
			msg.DestAddrTON = options.DestAddrTON
		}
		if options.DestAddrNPI >= 0 {
			msg.DestAddrNPI = options.DestAddrNPI
		}
		if options.DataCoding >= 0 {
			msg.DataCoding = options.DataCoding
		}
		if options.MaxRetries > 0 {
			msg.MaxRetries = options.MaxRetries
		}
		if options.TemplateID != nil {
			msg.TemplateID = options.TemplateID
		}
		if options.SenderNameID != nil {
			msg.SenderNameID = options.SenderNameID
		}
	}

	// Определяем кодировку
	msg.Encoding = domain.DetectEncoding(msg.Text)

	// Подсчитываем количество сегментов
	msg.SegmentCount = shared.CountSegments(msg.Text)

	// Валидация
	if err := s.validator.Validate(msg); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Apply scheduled_at from options
	if options != nil && options.ScheduledAt != nil {
		scheduledAt := *options.ScheduledAt
		tolerance := 30 * time.Second

		// If scheduled_at is more than tolerance in the future → schedule it
		if scheduledAt.After(time.Now().Add(tolerance)) {
			// Validate: not more than 30 days ahead
			maxSchedule := time.Now().Add(30 * 24 * time.Hour)
			if scheduledAt.After(maxSchedule) {
				return nil, fmt.Errorf("scheduled_at cannot be more than 30 days in the future")
			}

			msg.MarkAsScheduled(scheduledAt)

			// Save to DB but do NOT publish to Kafka
			if err := s.messageRepo.Create(ctx, msg); err != nil {
				return nil, fmt.Errorf("failed to create scheduled message: %w", err)
			}

			// Skip PublishMessageCreated and PublishMessageQueued
			// These publish to sms.outgoing which would bypass scheduling
			return msg, nil
		}
		// If within tolerance, treat as immediate send (fall through to normal flow)
	}

	// Sandbox handling is now a provider-layer concern, not a service-layer
	// short-circuit. Previously IsSandbox bypassed Kafka entirely, marking
	// the message delivered without any routing or tarification. That caused
	// divergence between /sms/send (honoured IsSandbox) and /sms/batch
	// (ignored IsSandbox) — revenue leak + inconsistent contract. Bug #10
	// (QA 2026-04-22). Sandbox clients must now be routed to a simulator
	// provider via their client_routes (spec 2026-04-10 sandbox-mode-v2).
	// IsSandbox is accepted for API compatibility but no longer changes
	// service-layer behaviour.

	// Non-scheduled: publish directly to Kafka, no DB write.
	// Persist stage will batch-insert into DB asynchronously via COPY protocol.
	msg.MarkAsQueued()
	if err := s.eventPublisher.PublishMessageQueued(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to publish message to queue: %w", err)
	}

	return msg, nil
}

// CancelMessage cancels a scheduled message
func (s *MessageService) CancelMessage(ctx context.Context, messageID, clientID uuid.UUID) error {
	return s.messageRepo.CancelByIDAndStatus(ctx, messageID, clientID)
}

// ListScheduledMessages returns paginated list of scheduled (not yet sent) messages for a client.
func (s *MessageService) ListScheduledMessages(
	ctx context.Context,
	clientID uuid.UUID,
	limit, offset int,
) ([]*domain.Message, int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.messageRepo.ListScheduled(ctx, clientID, limit, offset)
}

// SendBatch отправляет пакет сообщений
func (s *MessageService) SendBatch(
	ctx context.Context,
	clientID uuid.UUID,
	requests []*SendMessageRequest,
	scheduledAt *time.Time,
) ([]*BatchResult, error) {
	results := make([]*BatchResult, 0, len(requests))

	for _, req := range requests {
		options := &SendMessageOptions{
			ExternalID:         req.ExternalID,
			Priority:           req.Priority,
			RegisteredDelivery: req.RegisteredDelivery,
			ValidityPeriod:     req.ValidityPeriod,
			ServiceType:        req.ServiceType,
			SourceAddrTON:      req.SourceAddrTON,
			SourceAddrNPI:      req.SourceAddrNPI,
			DestAddrTON:        req.DestAddrTON,
			DestAddrNPI:        req.DestAddrNPI,
			DataCoding:         req.DataCoding,
			MaxRetries:         req.MaxRetries,
			ScheduledAt:        scheduledAt,
			TemplateID:         req.TemplateID,
			SenderNameID:       req.SenderNameID,
		}

		msg, err := s.SendMessage(ctx, clientID, req.Source, req.Destination, req.Text, options)
		if err != nil {
			results = append(results, &BatchResult{
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		results = append(results, &BatchResult{
			Success:      true,
			MessageID:    msg.ID,
			SegmentCount: msg.SegmentCount,
		})
	}

	return results, nil
}

// GetMessageStatus получает статус сообщения
func (s *MessageService) GetMessageStatus(
	ctx context.Context,
	messageID uuid.UUID,
	clientID *uuid.UUID,
) (*domain.Message, error) {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	// Проверка прав доступа
	if clientID != nil && msg.ClientID != nil && *msg.ClientID != *clientID {
		return nil, fmt.Errorf("access denied")
	}

	return msg, nil
}

// GetMessageHistory получает историю сообщений клиента
func (s *MessageService) GetMessageHistory(
	ctx context.Context,
	clientID uuid.UUID,
	filters *MessageHistoryFilters,
) ([]*domain.Message, error) {
	limit := 100
	offset := 0
	var status *string

	if filters != nil {
		if filters.Limit > 0 {
			limit = filters.Limit
		}
		if filters.Offset > 0 {
			offset = filters.Offset
		}
		if filters.Status != "" {
			statusStr := filters.Status
			status = &statusStr
		}
	}

	messages, err := s.messageRepo.GetByClientID(ctx, clientID, limit, offset, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get message history: %w", err)
	}

	return messages, nil
}

// UpdateMessageStatus обновляет статус сообщения
func (s *MessageService) UpdateMessageStatus(
	ctx context.Context,
	messageID uuid.UUID,
	status string,
	statusMessage string,
) error {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	oldStatus := string(msg.Status)

	switch status {
	case "queued":
		msg.MarkAsQueued()
	case "sent":
		// Требуется SMPP message ID для sent статуса
		// Здесь устанавливаем только статус, SMPP message ID устанавливается отдельно
		msg.Status = "sent"
		now := time.Now()
		msg.SubmittedAt = &now
		msg.UpdatedAt = now
	case "delivered":
		msg.MarkAsDelivered()
	case "failed":
		msg.MarkAsFailed(statusMessage)
	case "expired":
		msg.MarkAsExpired()
	case "rejected":
		msg.MarkAsRejected(statusMessage)
	default:
		return fmt.Errorf("invalid status: %s", status)
	}

	if err := s.messageRepo.UpdateStatus(ctx, messageID, string(msg.Status), msg.StatusMessage); err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}

	// Публикуем событие изменения статуса
	if oldStatus != string(msg.Status) {
		if err := s.eventPublisher.PublishMessageStatusChanged(ctx, msg, oldStatus); err != nil {
			log.Warn().Err(err).Msg("ошибка публикации события message.status.changed")
		}
	}

	return nil
}

// SendMessageOptions содержит опции для создания сообщения
type SendMessageOptions struct {
	ExternalID         string
	Priority           int
	RegisteredDelivery *bool
	ValidityPeriod     *time.Time
	ServiceType        string
	SourceAddrTON      int
	SourceAddrNPI      int
	DestAddrTON        int
	DestAddrNPI        int
	DataCoding         int
	MaxRetries         int
	ScheduledAt        *time.Time
	IsSandbox          bool
	// Audit linkage: populated by the send handler after the sender/template
	// have been validated. Propagated through Kafka → persist stage into
	// messages.{template_id,sender_name_id}.
	TemplateID   *uuid.UUID
	SenderNameID *uuid.UUID
}

// SendMessageRequest представляет запрос на отправку сообщения
type SendMessageRequest struct {
	Source            string
	Destination       string
	Text              string
	ExternalID        string
	Priority          int
	RegisteredDelivery *bool
	ValidityPeriod     *time.Time
	ServiceType        string
	SourceAddrTON      int
	SourceAddrNPI      int
	DestAddrTON        int
	DestAddrNPI        int
	DataCoding         int
	MaxRetries         int
	ScheduledAt        *time.Time
	TemplateID         *uuid.UUID
	SenderNameID       *uuid.UUID
}

// BatchResult представляет результат обработки одного сообщения в пакете
type BatchResult struct {
	Success      bool
	MessageID    uuid.UUID
	SegmentCount int
	Error        string
}

// MessageHistoryFilters содержит фильтры для истории сообщений
type MessageHistoryFilters struct {
	From       *time.Time
	To         *time.Time
	Status     string
	Destination string
	Limit      int
	Offset     int
}
