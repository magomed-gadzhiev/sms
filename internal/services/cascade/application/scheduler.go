package application

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	cascadekafka "github.com/smpp-server/smpp-server/internal/services/cascade/infrastructure/kafka"
)

// Scheduler опрашивает просроченные pending/sent attempts и публикует timeout events
type Scheduler struct {
	attempts   domain.AttemptRepository
	strategies domain.StrategyRepository
	producer   CascadeProducer
	interval   time.Duration
	logger     zerolog.Logger
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewScheduler(
	attempts domain.AttemptRepository,
	strategies domain.StrategyRepository,
	producer CascadeProducer,
	interval time.Duration,
	logger zerolog.Logger,
) *Scheduler {
	return &Scheduler{
		attempts:   attempts,
		strategies: strategies,
		producer:   producer,
		interval:   interval,
		logger:     logger.With().Str("component", "cascade_scheduler").Logger(),
	}
}

func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processTimedOut(ctx)
		}
	}
}

func (s *Scheduler) processTimedOut(ctx context.Context) {
	timedOut, err := s.attempts.FindPendingTimedOut(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("ошибка поиска просроченных попыток")
		return
	}

	if len(timedOut) == 0 {
		return
	}

	s.logger.Info().Int("count", len(timedOut)).Msg("найдены просроченные попытки")

	for _, attempt := range timedOut {
		s.publishTimeout(ctx, attempt)
	}
}

func (s *Scheduler) publishTimeout(ctx context.Context, attempt *domain.DeliveryAttempt) {
	evt := cascadekafka.NewCascadeAttemptResultEvent(
		attempt.ID.String(),
		attempt.DeliveryID.String(),
		attempt.ChannelType,
		"timeout",
		"",
		"scheduler timeout",
		time.Now().UTC().Format(time.RFC3339),
	)

	if err := s.producer.PublishAttemptResult(ctx, evt); err != nil {
		s.logger.Error().Err(err).
			Str("attempt_id", attempt.ID.String()).
			Str("delivery_id", attempt.DeliveryID.String()).
			Msg("ошибка публикации timeout события")
		return
	}

	s.logger.Debug().
		Str("attempt_id", attempt.ID.String()).
		Str("delivery_id", attempt.DeliveryID.String()).
		Str("channel_type", attempt.ChannelType).
		Msg("таймаут попытки опубликован")
}
