package application

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"

	"github.com/smpp-server/smpp-server/internal/shared/messagestatus"
)

// Scheduler polls for scheduled messages and dispatches them to Kafka
type Scheduler struct {
	messageRepo    domain.MessageRepository
	eventPublisher domain.EventPublisher
	logger         zerolog.Logger
	interval       time.Duration
	batchSize      int
	stuckThreshold time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewScheduler creates a new Scheduler instance
func NewScheduler(
	messageRepo domain.MessageRepository,
	eventPublisher domain.EventPublisher,
	interval time.Duration,
	batchSize int,
	stuckThreshold time.Duration,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		messageRepo:    messageRepo,
		eventPublisher: eventPublisher,
		logger:         log.With().Str("component", "scheduler").Logger(),
		interval:       interval,
		batchSize:      batchSize,
		stuckThreshold: stuckThreshold,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start launches the scheduler goroutine
func (s *Scheduler) Start() {
	s.logger.Info().
		Dur("interval", s.interval).
		Int("batch_size", s.batchSize).
		Msg("scheduler started")

	go s.run()
}

// Stop signals the scheduler to shut down
func (s *Scheduler) Stop() {
	s.cancel()
	s.logger.Info().Msg("scheduler stopped")
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processBatch()
			s.recoverStuckMessages()
		}
	}
}

func (s *Scheduler) processBatch() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	messages, err := s.messageRepo.GetScheduledReady(ctx, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to fetch scheduled messages")
		return
	}

	if len(messages) == 0 {
		return
	}

	s.logger.Info().Int("count", len(messages)).Msg("processing scheduled messages")

	for _, msg := range messages {
		// Update status to pending
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, string(messagestatus.Pending), ""); err != nil {
			s.logger.Error().Err(err).Str("message_id", msg.ID.String()).Msg("failed to update status to pending")
			continue
		}

		// Publish to Kafka via PublishMessageQueued (not PublishMessageCreated — both send to
		// sms.outgoing, but Queued is semantically correct for the scheduler flow)
		msg.Status = messagestatus.Pending
		if err := s.eventPublisher.PublishMessageQueued(ctx, msg); err != nil {
			s.logger.Error().Err(err).Str("message_id", msg.ID.String()).Msg("failed to publish to Kafka, reverting to scheduled")
			// Revert status back to scheduled
			if revertErr := s.messageRepo.UpdateStatus(ctx, msg.ID, string(messagestatus.Scheduled), ""); revertErr != nil {
				s.logger.Error().Err(revertErr).Str("message_id", msg.ID.String()).Msg("failed to revert status to scheduled")
			}
			continue
		}

		// Update status to queued
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, string(messagestatus.Queued), ""); err != nil {
			s.logger.Warn().Err(err).Str("message_id", msg.ID.String()).Msg("failed to update status to queued (message already in Kafka)")
		}

		s.logger.Debug().Str("message_id", msg.ID.String()).Msg("scheduled message dispatched")
	}
}

func (s *Scheduler) recoverStuckMessages() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	messages, err := s.messageRepo.GetStuckPending(ctx, s.stuckThreshold, s.batchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to fetch stuck pending messages")
		return
	}
	if len(messages) == 0 {
		return
	}

	s.logger.Warn().Int("count", len(messages)).Msg("recovering stuck pending messages")

	for _, msg := range messages {
		if err := s.eventPublisher.PublishMessageQueued(ctx, msg); err != nil {
			s.logger.Error().Err(err).Str("message_id", msg.ID.String()).Msg("failed to re-publish stuck message")
			continue
		}
		// Update updated_at to prevent re-processing
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, string(messagestatus.Pending), ""); err != nil {
			s.logger.Warn().Err(err).Str("message_id", msg.ID.String()).Msg("failed to update stuck message timestamp")
		}
	}
}
