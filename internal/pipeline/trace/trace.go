package trace

import "github.com/rs/zerolog"

// Log returns a zerolog event builder pre-populated with standard trace fields.
// Callers chain additional Str/Int/etc. calls and finish with .Msg().
//
// Example:
//
//	trace.Log(log.Logger, traceID, msgID, "api", "receive").
//	    Str("source", src).
//	    Msg("message received")
func Log(logger zerolog.Logger, traceID, messageID, stage, event string) *zerolog.Event {
	return logger.Info().
		Str("trace_id", traceID).
		Str("message_id", messageID).
		Str("stage", stage).
		Str("event", event)
}
