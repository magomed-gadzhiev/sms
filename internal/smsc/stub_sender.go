package smsc

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// StubProviderConfig — конфигурация поведения stub-провайдера.
type StubProviderConfig struct {
	ProviderID     uuid.UUID
	MinDelayMs     int
	MaxDelayMs     int
	FailureRatePct int
	DLRDelayMs     int
	DLRSuccessRate int
	DLRStatuses    []string
}

// StubConfigRepository загружает конфигурацию stub-провайдера из БД.
type StubConfigRepository interface {
	GetByProviderID(ctx context.Context, providerID uuid.UUID) (*StubProviderConfig, error)
}

// StubSender реализует async-отправку для SIMULATOR-провайдеров.
type StubSender struct {
	configRepo  StubConfigRepository
	dlrProducer *queue.AsyncProducer
	dlrTopic    string
	logger      zerolog.Logger
}

func NewStubSender(configRepo StubConfigRepository, dlrProducer *queue.AsyncProducer, dlrTopic string) *StubSender {
	return &StubSender{
		configRepo:  configRepo,
		dlrProducer: dlrProducer,
		dlrTopic:    dlrTopic,
		logger:      log.With().Str("component", "stub_sender").Logger(),
	}
}

// SendMessageAsync имитирует отправку: задержка, случайная ошибка, планирование DLR.
func (s *StubSender) SendMessageAsync(ctx context.Context, msg *shared.Message, provider *shared.Provider, _ *AsyncConnection) (string, error) {
	cfg := s.defaultConfig(provider.ID)
	if s.configRepo != nil {
		if loaded, err := s.configRepo.GetByProviderID(ctx, provider.ID); err == nil && loaded != nil {
			cfg = loaded
		}
	}

	// Имитация задержки сети
	delayRange := cfg.MaxDelayMs - cfg.MinDelayMs
	if delayRange < 1 {
		delayRange = 1
	}
	delay := cfg.MinDelayMs + rand.Intn(delayRange)
	select {
	case <-time.After(time.Duration(delay) * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Имитация случайной ошибки
	if rand.Intn(100) < cfg.FailureRatePct {
		return "", fmt.Errorf("stub: simulated failure (rate=%d%%)", cfg.FailureRatePct)
	}

	smppMsgID := "stub-" + uuid.New().String()
	go s.scheduleDLR(msg, smppMsgID, cfg)

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("smpp_msg_id", smppMsgID).
		Str("provider", provider.Name).
		Msg("stub: сообщение отправлено")

	return smppMsgID, nil
}

func (s *StubSender) scheduleDLR(msg *shared.Message, smppMsgID string, cfg *StubProviderConfig) {
	time.Sleep(time.Duration(cfg.DLRDelayMs) * time.Millisecond)

	stat := "DELIVRD"
	if rand.Intn(100) >= cfg.DLRSuccessRate {
		statuses := cfg.DLRStatuses
		if len(statuses) == 0 {
			statuses = []string{"UNDELIV"}
		}
		stat = statuses[rand.Intn(len(statuses))]
	}

	if s.dlrProducer == nil {
		return
	}

	now := time.Now()
	dlrMsg := &queue.DLRMessage{
		MessageID:     msg.ID,
		SMPPMessageID: smppMsgID,
		ClientID:      msg.ClientID,
		Stat:          stat,
		Source:        msg.Source,
		Destination:   msg.Destination,
		CreatedAt:     now,
		DoneDate:      &now,
	}
	if data, err := dlrMsg.Serialize(); err == nil {
		s.dlrProducer.PublishAsync(s.dlrTopic, msg.ID.String(), data, nil)
	}
}

func (s *StubSender) defaultConfig(providerID uuid.UUID) *StubProviderConfig {
	return &StubProviderConfig{
		ProviderID:     providerID,
		MinDelayMs:     100,
		MaxDelayMs:     500,
		FailureRatePct: 0,
		DLRDelayMs:     1000,
		DLRSuccessRate: 100,
		DLRStatuses:    []string{"DELIVRD"},
	}
}
