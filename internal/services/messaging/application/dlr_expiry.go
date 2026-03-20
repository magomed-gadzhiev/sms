package application

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
)

// DLRExpiry polls for messages in "sent" status that have not received a DLR
// within the configured timeout and transitions them to "expired".
type DLRExpiry struct {
	messageRepo domain.MessageRepository
	logger      zerolog.Logger
	timeout     time.Duration // how long to wait for DLR before expiring (default 24h)
	interval    time.Duration // how often to check (default 5m)
	batchSize   int           // max messages per batch (default 500)
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewDLRExpiry creates a new DLRExpiry instance
func NewDLRExpiry(
	messageRepo domain.MessageRepository,
	timeout time.Duration,
	interval time.Duration,
	batchSize int,
) *DLRExpiry {
	ctx, cancel := context.WithCancel(context.Background())
	return &DLRExpiry{
		messageRepo: messageRepo,
		logger:      log.With().Str("component", "dlr-expiry").Logger(),
		timeout:     timeout,
		interval:    interval,
		batchSize:   batchSize,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start launches the DLR expiry goroutine
func (d *DLRExpiry) Start() {
	d.logger.Info().
		Dur("timeout", d.timeout).
		Dur("interval", d.interval).
		Int("batch_size", d.batchSize).
		Msg("DLR expiry started")

	go d.run()
}

// Stop signals the DLR expiry goroutine to shut down
func (d *DLRExpiry) Stop() {
	d.cancel()
	d.logger.Info().Msg("DLR expiry stopped")
}

func (d *DLRExpiry) run() {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.processBatch()
		}
	}
}

func (d *DLRExpiry) processBatch() {
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	messages, err := d.messageRepo.GetSentExpired(ctx, d.timeout, d.batchSize)
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to fetch sent expired messages")
		return
	}

	if len(messages) == 0 {
		return
	}

	d.logger.Info().Int("count", len(messages)).Msg("expiring sent messages without DLR")

	if err := d.messageRepo.BulkUpdateStatusToExpired(ctx, messages); err != nil {
		d.logger.Error().Err(err).Int("count", len(messages)).Msg("failed to bulk update messages to expired")
		return
	}

	d.logger.Debug().Int("count", len(messages)).Msg("messages expired successfully")
}
