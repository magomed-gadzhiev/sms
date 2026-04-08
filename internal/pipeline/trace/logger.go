package trace

import (
	"github.com/rs/zerolog"
)

// Log starts a structured trace log event at Info level.
// Every trace event includes trace_id, message_id, stage, and event fields
// for consistent filtering and correlation across pipeline stages.
//
// Usage:
//
//	trace.Log(s.logger, traceID, messageID, "router", "route_matched").
//	    Str("provider_id", providerID).
//	    Msg("route selected")
func Log(logger zerolog.Logger, traceID, messageID, stage, event string) *zerolog.Event {
	return logger.Info().
		Str("trace_id", traceID).
		Str("message_id", messageID).
		Str("stage", stage).
		Str("event", event)
}

// Debug starts a trace log event at Debug level for verbose diagnostics.
func Debug(logger zerolog.Logger, traceID, messageID, stage, event string) *zerolog.Event {
	return logger.Debug().
		Str("trace_id", traceID).
		Str("message_id", messageID).
		Str("stage", stage).
		Str("event", event)
}

// Warn starts a trace log event at Warn level for non-fatal issues.
func Warn(logger zerolog.Logger, traceID, messageID, stage, event string) *zerolog.Event {
	return logger.Warn().
		Str("trace_id", traceID).
		Str("message_id", messageID).
		Str("stage", stage).
		Str("event", event)
}
