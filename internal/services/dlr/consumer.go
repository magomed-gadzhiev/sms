package dlr

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"

	"github.com/smpp-server/smpp-server/internal/shared/messagestatus"
)

// finalStatuses defines statuses that trigger DLR dispatch: terminal
// Message statuses from the vocabulary module.
func finalStatuses(status string) bool {
	return messagestatus.IsTerminal(messagestatus.Status(status))
}

// MessageMappingReader provides read access to message mappings and session bindings in Redis.
type MessageMappingReader interface {
	GetMessageMapping(ctx context.Context, messageID string) (*server.MessageMapping, error)
	GetSessionBinding(ctx context.Context, systemID string) (*server.SessionBinding, error)
}

// DLRDispatcher dispatches DLR receipts to SMPP clients.
type DLRDispatcher interface {
	Dispatch(ctx context.Context, req DispatchRequest) (bool, error)
}

// Processor handles pipeline.StatusUpdate events and dispatches DLR receipts.
type Processor struct {
	store      MessageMappingReader
	dispatcher DLRDispatcher
	logger     zerolog.Logger
}

// NewProcessor creates a new DLR Processor.
func NewProcessor(store MessageMappingReader, dispatcher DLRDispatcher, logger zerolog.Logger) *Processor {
	return &Processor{
		store:      store,
		dispatcher: dispatcher,
		logger:     logger.With().Str("component", "dlr_processor").Logger(),
	}
}

// ProcessStatusUpdate processes a single StatusUpdate event.
// It skips non-final statuses, looks up message mapping and session binding in Redis,
// formats an SMSC receipt, and dispatches it to the SMPP client.
func (p *Processor) ProcessStatusUpdate(ctx context.Context, update *pipeline.StatusUpdate) error {
	// 1. Skip non-final statuses
	if !finalStatuses(update.Status) {
		return nil
	}

	monitoring.DLREventsConsumed.WithLabelValues(update.Status).Inc()

	msgIDStr := update.MessageID.String()

	// 2. Look up message mapping
	mapping, err := p.store.GetMessageMapping(ctx, msgIDStr)
	if err != nil {
		if errors.Is(err, server.ErrRedisKeyNotFound) {
			p.logger.Debug().Str("message_id", msgIDStr).Msg("message mapping not found, skipping (not from SMPP)")
			monitoring.DLRRedisLookupMiss.WithLabelValues("msg").Inc()
			return nil
		}
		return err
	}

	// 3. Look up session binding
	_, err = p.store.GetSessionBinding(ctx, mapping.SystemID)
	if err != nil {
		if errors.Is(err, server.ErrRedisKeyNotFound) {
			p.logger.Debug().Str("system_id", mapping.SystemID).Msg("session binding not found, client disconnected")
			monitoring.DLRRedisLookupMiss.WithLabelValues("session").Inc()
			return nil
		}
		return err
	}

	// 4. Format SMSC receipt
	doneDate := update.UpdatedAt
	if update.DoneDate != nil {
		doneDate = *update.DoneDate
	}
	submitDate := mapping.SubmitDate
	if update.SubmitDate != nil {
		submitDate = *update.SubmitDate
	}

	var errCode int
	if update.ErrorCode != nil {
		errCode = *update.ErrorCode
	}

	receipt := FormatReceipt(ReceiptParams{
		MessageID:  msgIDStr,
		Status:     update.Status,
		SubmitDate: submitDate,
		DoneDate:   doneDate,
		ErrorCode:  errCode,
	})

	// 5. Dispatch with source/dest swapped (deliver_sm convention)
	_, err = p.dispatcher.Dispatch(ctx, DispatchRequest{
		SystemID:   mapping.SystemID,
		SourceAddr: mapping.DestAddr,
		DestAddr:   mapping.SourceAddr,
		Receipt:    receipt,
	})
	if err != nil {
		p.logger.Error().Err(err).
			Str("message_id", msgIDStr).
			Str("system_id", mapping.SystemID).
			Msg("failed to dispatch DLR")
		return err
	}

	p.logger.Info().
		Str("message_id", msgIDStr).
		Str("system_id", mapping.SystemID).
		Str("status", update.Status).
		Msg("DLR dispatched")

	return nil
}
