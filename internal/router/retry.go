package router

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// MessageRepository описывает контракт репозитория сообщений, нужный для RetryManager.
type MessageRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Message, error)
	IncrementRetryCount(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error
	Update(ctx context.Context, msg *shared.Message) error
}

// RetryManager управляет логикой повторных попыток отправки сообщений
type RetryManager struct {
	config      *config.WorkerConfig
	messageRepo MessageRepository
	logger      zerolog.Logger
}

// NewRetryManager создает новый менеджер повторов
func NewRetryManager(cfg *config.WorkerConfig, messageRepo MessageRepository) *RetryManager {
	logger := log.With().Str("component", "retry_manager").Logger()
	return &RetryManager{
		config:      cfg,
		messageRepo: messageRepo,
		logger:      logger,
	}
}

// ShouldRetry определяет, нужно ли повторять попытку отправки
func (rm *RetryManager) ShouldRetry(msg *shared.Message) bool {
	return msg.RetryCount < msg.MaxRetries && msg.Status == shared.MessageStatusFailed
}

// CalculateNextRetry вычисляет время следующей попытки с экспоненциальной задержкой
func (rm *RetryManager) CalculateNextRetry(retryCount int) time.Time {
	// Экспоненциальная задержка: base * 2^retryCount
	baseSeconds := rm.config.RetryBackoffBase.Seconds()
	delaySeconds := baseSeconds * math.Pow(2, float64(retryCount))
	
	// Ограничиваем максимальной задержкой
	maxSeconds := rm.config.RetryBackoffMax.Seconds()
	if delaySeconds > maxSeconds {
		delaySeconds = maxSeconds
	}

	return time.Now().Add(time.Duration(delaySeconds) * time.Second)
}

// IsPermanentError определяет, является ли ошибка постоянной (не стоит повторять)
func (rm *RetryManager) IsPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Список ошибок, которые не стоит повторять
	permanentErrors := []string{
		"ESME_RINVDSTADR",  // Invalid destination address
		"ESME_RINVSRCADR",  // Invalid source address
		"ESME_RINVPASWD",   // Invalid password
		"ESME_RINVSYSID",   // Invalid system ID
		"ESME_RALYBND",     // Already bound (логическая ошибка)
		"ESME_RINVCMDID",   // Invalid command ID
		"ESME_RINVCMDLEN",  // Invalid command length
		"ESME_RINVMSGLEN",  // Invalid message length
	}

	for _, permErr := range permanentErrors {
		if contains(errStr, permErr) {
			return true
		}
	}

	return false
}

// contains проверяет, содержит ли строка подстроку (без учета регистра)
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower преобразует символ в нижний регистр
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// ScheduleRetry планирует повторную попытку отправки
func (rm *RetryManager) ScheduleRetry(ctx context.Context, messageID uuid.UUID, retryCount int) error {
	nextRetryAt := rm.CalculateNextRetry(retryCount)

	_, err := rm.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("ошибка получения сообщения: %w", err)
	}

	// Обновляем retry_count и next_retry_at
	err = rm.messageRepo.IncrementRetryCount(ctx, messageID, nextRetryAt)
	if err != nil {
		return fmt.Errorf("ошибка обновления retry_count: %w", err)
	}

	rm.logger.Info().
		Str("message_id", messageID.String()).
		Int("retry_count", retryCount+1).
		Time("next_retry_at", nextRetryAt).
		Msg("повторная попытка запланирована")

	return nil
}

// MarkAsFailed помечает сообщение как окончательно провалившееся
func (rm *RetryManager) MarkAsFailed(ctx context.Context, messageID uuid.UUID, errorMsg string) error {
	msg, err := rm.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("ошибка получения сообщения: %w", err)
	}

	msg.Status = shared.MessageStatusFailed
	msg.StatusMessage = errorMsg
	msg.FailedAt = &time.Time{}
	*msg.FailedAt = time.Now()
	msg.UpdatedAt = time.Now()

	err = rm.messageRepo.Update(ctx, msg)
	if err != nil {
		return fmt.Errorf("ошибка обновления сообщения: %w", err)
	}

	rm.logger.Info().
		Str("message_id", messageID.String()).
		Str("error", errorMsg).
		Msg("сообщение помечено как провалившееся")

	return nil
}
