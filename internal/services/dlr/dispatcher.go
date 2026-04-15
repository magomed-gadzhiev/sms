package dlr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// GRPCClient defines the interface for delivering DLR via gRPC.
type GRPCClient interface {
	DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (delivered bool, errMsg string, err error)
}

// DispatchRequest contains the parameters for a DLR dispatch.
type DispatchRequest struct {
	SystemID   string
	SourceAddr string
	DestAddr   string
	Receipt    string
}

// Dispatcher sends DLR receipts to SMPP clients via gRPC with retry logic.
type Dispatcher struct {
	client       GRPCClient
	logger       zerolog.Logger
	maxRetries   int
	retryBackoff time.Duration
}

// NewDispatcher creates a new Dispatcher with default retry settings.
func NewDispatcher(client GRPCClient, logger zerolog.Logger) *Dispatcher {
	return &Dispatcher{
		client:       client,
		logger:       logger.With().Str("component", "dlr_dispatcher").Logger(),
		maxRetries:   3,
		retryBackoff: 1 * time.Second,
	}
}

// Dispatch sends a DLR receipt to the target SMPP client, retrying on transient failures.
func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (bool, error) {
	startTime := time.Now()
	defer func() {
		monitoring.DLRDispatchDuration.WithLabelValues().Observe(time.Since(startTime).Seconds())
	}()

	var lastErr error
	for attempt := 1; attempt <= d.maxRetries; attempt++ {
		delivered, errMsg, err := d.client.DeliverDLR(ctx, req.SystemID, req.SourceAddr, req.DestAddr, req.Receipt)
		if err != nil {
			lastErr = err
			d.logger.Warn().Err(err).Int("attempt", attempt).Str("system_id", req.SystemID).Msg("gRPC call failed, retrying")
			if attempt < d.maxRetries {
				backoff := d.retryBackoff * time.Duration(1<<(attempt-1))
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				case <-time.After(backoff):
				}
			}
			continue
		}

		if !delivered {
			d.logger.Warn().Str("system_id", req.SystemID).Str("error", errMsg).Msg("DLR not delivered")
			monitoring.DLREventsDispatched.WithLabelValues("", "not_delivered").Inc()
			return false, nil
		}

		monitoring.DLREventsDispatched.WithLabelValues("", "success").Inc()
		return true, nil
	}

	monitoring.DLREventsDispatched.WithLabelValues("", "exhausted_retries").Inc()
	return false, fmt.Errorf("all %d retries exhausted: %w", d.maxRetries, lastErr)
}
