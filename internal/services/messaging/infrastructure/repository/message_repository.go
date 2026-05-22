package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// MessageRepository реализует domain.MessageRepository
type MessageRepository struct {
	repo *storage.MessageRepository
}

// NewMessageRepository создает новый репозиторий сообщений
func NewMessageRepository(db *sqlx.DB) *MessageRepository {
	// Создаем storage.DB обертку из sql.DB
	storageDB := &storage.DB{DB: db.DB}
	storageRepo := storage.NewMessageRepository(storageDB)
	return &MessageRepository{
		repo: storageRepo,
	}
}

// Create создает новое сообщение
func (r *MessageRepository) Create(ctx context.Context, msg *domain.Message) error {
	sharedMsg := msg.ToShared()
	return r.repo.Create(ctx, sharedMsg)
}

// GetByID получает сообщение по ID
func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	sharedMsg, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domain.MessageFromShared(sharedMsg), nil
}

// GetByMessageID получает сообщение по message_id
func (r *MessageRepository) GetByMessageID(ctx context.Context, messageID string) (*domain.Message, error) {
	sharedMsg, err := r.repo.GetByMessageID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return domain.MessageFromShared(sharedMsg), nil
}

// GetByExternalID получает сообщение по external_id
func (r *MessageRepository) GetByExternalID(ctx context.Context, externalID string) (*domain.Message, error) {
	sharedMsg, err := r.repo.GetByExternalID(ctx, externalID)
	if err != nil {
		return nil, err
	}
	return domain.MessageFromShared(sharedMsg), nil
}

// GetBySMPPMessageID получает сообщение по SMPP message_id
func (r *MessageRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.Message, error) {
	sharedMsg, err := r.repo.GetBySMPPMessageID(ctx, smppMessageID)
	if err != nil {
		return nil, err
	}
	return domain.MessageFromShared(sharedMsg), nil
}

// Update обновляет сообщение
func (r *MessageRepository) Update(ctx context.Context, msg *domain.Message) error {
	sharedMsg := msg.ToShared()
	return r.repo.Update(ctx, sharedMsg)
}

// UpdateStatus обновляет статус сообщения
func (r *MessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, statusMessage string) error {
	// Преобразуем строку статуса в shared.MessageStatus
	msgStatus := shared.MessageStatus(status)
	return r.repo.UpdateStatus(ctx, id, msgStatus, statusMessage)
}

// GetByClientID получает сообщения клиента
func (r *MessageRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *string) ([]*domain.Message, error) {
	var msgStatus *shared.MessageStatus
	if status != nil {
		s := shared.MessageStatus(*status)
		msgStatus = &s
	}

	sharedMessages, err := r.repo.GetByClientID(ctx, clientID, limit, offset, msgStatus)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, nil
}

// GetPendingForRetry получает сообщения, готовые для повторной попытки
func (r *MessageRepository) GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Message, error) {
	sharedMessages, err := r.repo.GetPendingForRetry(ctx, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, nil
}

// GetScheduledReady fetches scheduled messages ready for delivery
func (r *MessageRepository) GetScheduledReady(ctx context.Context, limit int) ([]*domain.Message, error) {
	sharedMessages, err := r.repo.GetScheduledReady(ctx, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, nil
}

// GetStuckPending fetches stuck pending messages for recovery
func (r *MessageRepository) GetStuckPending(ctx context.Context, threshold time.Duration, limit int) ([]*domain.Message, error) {
	sharedMessages, err := r.repo.GetStuckPending(ctx, threshold, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, nil
}

// CancelByIDAndStatus cancels a scheduled message
func (r *MessageRepository) CancelByIDAndStatus(ctx context.Context, id, clientID uuid.UUID) error {
	return r.repo.CancelByIDAndStatus(ctx, id, clientID)
}

// GetSentExpired fetches messages in "sent" status older than timeout
func (r *MessageRepository) GetSentExpired(ctx context.Context, timeout time.Duration, limit int) ([]*domain.Message, error) {
	sharedMessages, err := r.repo.GetSentExpired(ctx, timeout, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, nil
}

// ListScheduled returns paginated scheduled messages for a client with total count
func (r *MessageRepository) ListScheduled(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.Message, int, error) {
	sharedMessages, total, err := r.repo.ListScheduled(ctx, clientID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list scheduled messages: %w", err)
	}

	messages := make([]*domain.Message, len(sharedMessages))
	for i, sm := range sharedMessages {
		messages[i] = domain.MessageFromShared(sm)
	}

	return messages, total, nil
}

// BulkUpdateStatusToExpired updates a batch of messages to expired status
func (r *MessageRepository) BulkUpdateStatusToExpired(ctx context.Context, messages []*domain.Message) error {
	ids := make([]uuid.UUID, len(messages))
	for i, msg := range messages {
		ids[i] = msg.ID
	}
	return r.repo.BulkUpdateStatusToExpired(ctx, ids)
}
