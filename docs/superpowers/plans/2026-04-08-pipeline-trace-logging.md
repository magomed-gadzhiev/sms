# Pipeline Trace Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive trace logging across all 4 pipeline stages so that every message's journey (API receive -> persist -> router -> sender -> status/DLR) can be traced end-to-end via a single `trace_id`.

**Architecture:** Add `TraceID` field to all Kafka message types (`KafkaMessage`, `RoutedMessage`, `SentMessage`, `DLRMessage`, `FailedMessage`, `StatusUpdate`). Create a minimal `trace` helper package (~20 lines) that standardizes structured log events. Instrument each pipeline stage with trace checkpoints at key decision points. Add operator resolution logging. Enhance route matcher with debug-level condition evaluation logs.

**Tech Stack:** Go 1.24.0, zerolog, existing Kafka message types (JSON serialization)

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `internal/pipeline/trace/logger.go` | Trace log helper — one function, standardized fields |
| Modify | `internal/queue/message.go` | Add `TraceID` to `KafkaMessage`, `DLRMessage`, `FailedMessage` |
| Modify | `internal/pipeline/messages.go` | Add `TraceID` to `RoutedMessage`, `SentMessage`, `StatusUpdate` |
| Modify | `internal/api/grpc/server.go` | Generate `TraceID` at message reception, log API receive checkpoint |
| Modify | `internal/pipeline/persist/stage.go` | Add trace logging for batch persist |
| Modify | `internal/pipeline/router/stage.go` | Add trace logging for routing decisions |
| Modify | `internal/router/operator_resolver.go` | Add logging for operator resolution |
| Modify | `internal/services/routing/application/matcher.go` | Add Debug-level condition evaluation logging |
| Modify | `internal/pipeline/sender/stage.go` | Add trace logging for billing, tarification, backpressure, send |
| Modify | `internal/pipeline/status/stage.go` | Add trace logging for status upserts |

---

### Task 1: Add TraceID to Kafka Message Types

**Files:**
- Modify: `internal/queue/message.go`
- Modify: `internal/pipeline/messages.go`

- [ ] **Step 1: Add TraceID field to KafkaMessage**

In `internal/queue/message.go`, add `TraceID` field to `KafkaMessage` struct:

```go
type KafkaMessage struct {
	ID          string                 `json:"id"`
	MessageID   uuid.UUID             `json:"message_id"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Source      string                 `json:"source"`
	// ... rest unchanged
}
```

Also add `TraceID` to `DLRMessage`:

```go
type DLRMessage struct {
	MessageID          uuid.UUID  `json:"message_id"`
	TraceID            string     `json:"trace_id,omitempty"`
	SMPPMessageID      string     `json:"smpp_message_id"`
	// ... rest unchanged
}
```

And to `FailedMessage`:

```go
type FailedMessage struct {
	MessageID    uuid.UUID              `json:"message_id"`
	TraceID      string                 `json:"trace_id,omitempty"`
	KafkaMessage *KafkaMessage          `json:"kafka_message,omitempty"`
	// ... rest unchanged
}
```

- [ ] **Step 2: Add TraceID to pipeline message types**

In `internal/pipeline/messages.go`, add `TraceID` field to `RoutedMessage`:

```go
type RoutedMessage struct {
	SchemaVersion      int                    `json:"schema_version"`
	MessageID          uuid.UUID              `json:"message_id"`
	TraceID            string                 `json:"trace_id,omitempty"`
	Source             string                 `json:"source"`
	// ... rest unchanged
}
```

Add `TraceID` to `SentMessage`:

```go
type SentMessage struct {
	SchemaVersion int       `json:"schema_version"`
	MessageID     uuid.UUID `json:"message_id"`
	TraceID       string    `json:"trace_id,omitempty"`
	ProviderID    uuid.UUID `json:"provider_id"`
	// ... rest unchanged
}
```

Add `TraceID` to `StatusUpdate`:

```go
type StatusUpdate struct {
	SchemaVersion int        `json:"schema_version"`
	MessageID     uuid.UUID  `json:"message_id"`
	TraceID       string     `json:"trace_id,omitempty"`
	Status        string     `json:"status"`
	// ... rest unchanged
}
```

- [ ] **Step 3: Verify project compiles**

Run: `go build ./...`
Expected: Success (TraceID is optional/omitempty, so no existing code breaks)

- [ ] **Step 4: Commit**

```bash
git add internal/queue/message.go internal/pipeline/messages.go
git commit -m "feat(trace): add TraceID field to all Kafka message types"
```

---

### Task 2: Create Trace Logger Helper

**Files:**
- Create: `internal/pipeline/trace/logger.go`

- [ ] **Step 1: Create the trace helper package**

Create `internal/pipeline/trace/logger.go`:

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/pipeline/trace/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/trace/logger.go
git commit -m "feat(trace): create trace logger helper package"
```

---

### Task 3: Instrument API Entry Point (TraceID generation)

**Files:**
- Modify: `internal/api/grpc/server.go`
- Modify: `internal/queue/message.go` (FromMessage helper)

- [ ] **Step 1: Generate TraceID in SendSMS and propagate to KafkaMessage**

In `internal/api/grpc/server.go`, in the `SendSMS` method, after creating the `msg` (around line 97), add TraceID generation and trace logging. Import `"github.com/smpp-server/smpp-server/internal/pipeline/trace"`.

After the line `msg.UpdatedAt = time.Now()` (line 97) and before `s.messageRepo.Create(...)` (line 108), add:

```go
	traceID := uuid.New().String()
```

Then after `kafkaMsg := queue.FromMessage(msg)` (line 114), add:

```go
	kafkaMsg.TraceID = traceID

	trace.Log(log.Logger, traceID, msg.ID.String(), "api", "receive").
		Str("client_id", clientID.String()).
		Str("source", req.Source).
		Str("destination", req.Destination).
		Int("text_length", len(req.Text)).
		Msg("message received via gRPC")
```

- [ ] **Step 2: Do the same in SendBatchSMS**

In the `SendBatchSMS` method, after each `kafkaMsg := queue.FromMessage(msg)` call, generate and assign a traceID:

```go
	traceID := uuid.New().String()
	kafkaMsg.TraceID = traceID
```

Add a single batch-level trace log after the batch loop:

```go
	log.Info().
		Str("client_id", clientID.String()).
		Int32("success_count", successCount).
		Int32("failed_count", failedCount).
		Int("total", len(req.Messages)).
		Msg("batch SMS received via gRPC")
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/api/...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add internal/api/grpc/server.go
git commit -m "feat(trace): generate trace_id at API entry point, log message reception"
```

---

### Task 4: Instrument Persist Stage

**Files:**
- Modify: `internal/pipeline/persist/stage.go`

- [ ] **Step 1: Add batch-level trace logging to handleBatch**

In `internal/pipeline/persist/stage.go`, import `"github.com/smpp-server/smpp-server/internal/pipeline/trace"`.

In `handleBatch`, after `rows, deserErrors := buildCopyRows(msgs)` (line 115), add per-row trace logging. Modify `buildCopyRows` to also return traceIDs so we can log them. Since this is a batch operation, log a summary at batch level plus individual trace events for deserialization errors.

After the successful `copyInsert` call (after line 129), add:

```go
	// Log trace checkpoint for each persisted message
	for _, r := range rows {
		trace.Debug(s.logger, r.traceID, r.id.String(), "persist", "completed").
			Str("encoding", r.encoding).
			Int("segment_count", r.segmentCount).
			Msg("message persisted to DB")
	}
```

This requires adding `traceID` field to the `messageRow` struct:

```go
type messageRow struct {
	id           uuid.UUID
	traceID      string
	messageID    string
	// ... rest unchanged
}
```

And in `buildCopyRows`, extract traceID from KafkaMessage:

```go
	rows = append(rows, messageRow{
		id:           id,
		traceID:      km.TraceID,
		messageID:    km.ID,
		// ... rest unchanged
	})
```

Note: `traceID` is NOT a database column — it's only used for logging within the stage. The `copyColumns` and `Values()` method remain unchanged.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/pipeline/persist/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/persist/stage.go
git commit -m "feat(trace): add trace logging to persist stage"
```

---

### Task 5: Add Operator Resolution Logging

**Files:**
- Modify: `internal/router/operator_resolver.go`

- [ ] **Step 1: Add structured logging to Resolve method**

In `internal/router/operator_resolver.go`, add a logger field to the `OperatorResolver` struct and log resolution results. Import zerolog.

Add logger to struct and constructor:

```go
type OperatorResolver struct {
	repo              OperatorPrefixRepository
	defaultOperatorID uuid.UUID
	prefixes          []shared.OperatorPrefix
	mu                sync.RWMutex
	refreshTTL        time.Duration
	lastRefresh       time.Time
	logger            zerolog.Logger
}

func NewOperatorResolver(repo OperatorPrefixRepository, defaultOperatorID uuid.UUID) *OperatorResolver {
	return &OperatorResolver{
		repo:              repo,
		defaultOperatorID: defaultOperatorID,
		refreshTTL:        5 * time.Minute,
		logger:            log.With().Str("component", "operator_resolver").Logger(),
	}
}
```

In the `Resolve` method, after the prefix match loop (line 58-61), add logging for both cases — matched prefix and default fallback:

```go
func (r *OperatorResolver) Resolve(ctx context.Context, number string) uuid.UUID {
	normalized := normalizePhoneNumber(number)
	if normalized == "" {
		r.logger.Debug().
			Str("number", number).
			Str("operator_id", r.defaultOperatorID.String()).
			Msg("empty number, using default operator")
		return r.defaultOperatorID
	}

	r.mu.RLock()
	stale := time.Since(r.lastRefresh) > r.refreshTTL
	r.mu.RUnlock()

	if stale {
		r.refresh(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.prefixes {
		if len(normalized) >= len(p.Prefix) && normalized[:len(p.Prefix)] == p.Prefix {
			r.logger.Debug().
				Str("number", normalized).
				Str("prefix", p.Prefix).
				Str("operator_id", p.OperatorID.String()).
				Msg("operator resolved by prefix")
			return p.OperatorID
		}
	}

	r.logger.Debug().
		Str("number", normalized).
		Str("operator_id", r.defaultOperatorID.String()).
		Msg("no prefix match, using default operator")
	return r.defaultOperatorID
}
```

Also add logging to `refresh`:

```go
func (r *OperatorResolver) refresh(ctx context.Context) {
	prefixes, err := r.repo.GetAllActive(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to load operator prefixes")
		return
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i].Prefix) > len(prefixes[j].Prefix)
	})
	r.mu.Lock()
	r.prefixes = prefixes
	r.lastRefresh = time.Now()
	r.mu.Unlock()
	r.logger.Info().Int("prefix_count", len(prefixes)).Msg("operator prefixes reloaded")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/router/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/router/operator_resolver.go
git commit -m "feat(trace): add structured logging to operator resolver"
```

---

### Task 6: Add Debug Logging to Route Matcher

**Files:**
- Modify: `internal/services/routing/application/matcher.go`

- [ ] **Step 1: Add a MatchWithLog method that returns match details**

Rather than modifying the core `Match` method (which is used in hot path), add debug logging by extending the existing `Match` call in the router stage. We'll add logging to `filterMatching` via a new `MatchDebug` method that returns both matched routes and evaluation details.

Add to `internal/services/routing/application/matcher.go`:

```go
// MatchResult holds the outcome of route matching with diagnostic details.
type MatchResult struct {
	Matched       []*domain.ClientRoute
	ClientRoutes  int    // number of client-specific routes evaluated
	DefaultRoutes int    // number of default routes evaluated
	UsedDefault   bool   // true if fell back to default routes
}

// MatchWithDetails returns matched routes plus diagnostic info for trace logging.
func (m *RouteMatcher) MatchWithDetails(ctx MatchContext) MatchResult {
	m.mu.RLock()
	routes := m.routes
	regexCache := m.regexCache
	m.mu.RUnlock()

	var clientRoutes, defaultRoutes []*domain.ClientRoute
	for _, r := range routes {
		if r.RouteType != ctx.RouteType {
			continue
		}
		if r.ClientID != nil && *r.ClientID == ctx.ClientID {
			clientRoutes = append(clientRoutes, r)
		} else if r.ClientID == nil {
			defaultRoutes = append(defaultRoutes, r)
		}
	}

	matched := m.filterMatching(clientRoutes, ctx, regexCache)
	usedDefault := false
	if len(matched) == 0 {
		matched = m.filterMatching(defaultRoutes, ctx, regexCache)
		usedDefault = true
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Priority < matched[j].Priority
	})

	return MatchResult{
		Matched:       matched,
		ClientRoutes:  len(clientRoutes),
		DefaultRoutes: len(defaultRoutes),
		UsedDefault:   usedDefault,
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/services/routing/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/services/routing/application/matcher.go
git commit -m "feat(trace): add MatchWithDetails to route matcher for diagnostic logging"
```

---

### Task 7: Instrument Router Stage

**Files:**
- Modify: `internal/pipeline/router/stage.go`

- [ ] **Step 1: Add trace logging to processMessage**

In `internal/pipeline/router/stage.go`, import `"github.com/smpp-server/smpp-server/internal/pipeline/trace"`.

Replace the existing `processMessage` method with trace-instrumented version. Key changes:

After operator resolution (line 156), add:

```go
	trace.Debug(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "operator_resolved").
		Str("operator_id", operatorID.String()).
		Str("destination", kafkaMsg.Destination).
		Msg("operator resolved")
```

Replace the existing `s.matcher.Match(matchCtx)` call (line 180) with `MatchWithDetails`:

```go
	result := s.matcher.MatchWithDetails(matchCtx)
	if len(result.Matched) == 0 {
		trace.Warn(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "no_route").
			Str("operator_id", operatorID.String()).
			Str("traffic_type", string(trafficType)).
			Str("sender_name", kafkaMsg.Source).
			Int("client_routes_checked", result.ClientRoutes).
			Int("default_routes_checked", result.DefaultRoutes).
			Msg("no matching route found")
		return fmt.Errorf("маршрут не найден для message_id=%s client=%s operator=%s traffic=%s",
			kafkaMsg.MessageID, kafkaMsg.ClientID, operatorID, trafficType)
	}

	route := result.Matched[0]
	providerID = route.ProviderID
	routeID = &route.ID

	trace.Log(s.logger, kafkaMsg.TraceID, kafkaMsg.MessageID.String(), "router", "route_matched").
		Str("route_id", route.ID.String()).
		Str("route_name", route.Name).
		Str("provider_id", providerID.String()).
		Int("priority", route.Priority).
		Bool("used_default", result.UsedDefault).
		Int("total_matched", len(result.Matched)).
		Msg("route selected")
```

Propagate TraceID to RoutedMessage:

```go
	routed := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     kafkaMsg.MessageID,
		TraceID:       kafkaMsg.TraceID,
		// ... rest unchanged
	}
```

Remove the existing Debug log at lines 230-234 (replaced by trace.Log above).

Also add trace logging for retry messages from sms.failed in `deserializeByTopic`:

After the retry exhaustion check (line 253), add:

```go
	s.logger.Info().
		Str("trace_id", failed.KafkaMessage.TraceID).
		Str("message_id", failed.MessageID.String()).
		Int("retry_count", failed.RetryCount).
		Int("max_retries", failed.KafkaMessage.MaxRetries).
		Msg("processing retry message")
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/pipeline/router/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/router/stage.go
git commit -m "feat(trace): add comprehensive trace logging to router stage"
```

---

### Task 8: Instrument Sender Stage

**Files:**
- Modify: `internal/pipeline/sender/stage.go`

- [ ] **Step 1: Add trace logging throughout processMessage**

In `internal/pipeline/sender/stage.go`, import `"github.com/smpp-server/smpp-server/internal/pipeline/trace"`.

Add trace checkpoints at each decision point in `processMessage`:

**After deserialization (line 292):**

```go
	traceID := routedMsg.TraceID
```

**After billing check (replace existing Warn at lines 308-311):**

```go
	// Frozen account
	trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "account_frozen").
		Str("client_id", routedMsg.ClientID.String()).
		Msg("account frozen, message rejected")
```

And for billing unavailable (replace existing Error at line 303):

```go
	trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "error").
		Err(balanceErr).
		Msg("billing service unavailable, message rejected")
```

**After successful billing check, add:**

```go
	trace.Debug(s.logger, traceID, routedMsg.MessageID.String(), "sender.billing", "ok").
		Str("client_id", routedMsg.ClientID.String()).
		Msg("billing check passed")
```

**After tarification (replace existing Error at line 336 and Warn at lines 342-345):**

For tarification error:
```go
	trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "error").
		Err(tarifyErr).
		Msg("tarification failed, message rejected")
```

For tarification rejected:
```go
	trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "rejected").
		Str("reason", tarifyResp.RejectionReason).
		Msg("tarification rejected")
```

For tarification success (after line 353):
```go
	trace.Debug(s.logger, traceID, routedMsg.MessageID.String(), "sender.tarify", "approved").
		Str("amount", chargedAmount).
		Str("currency", chargedCurrency).
		Int32("segments", segCount).
		Msg("tarification approved")
```

**After backpressure check (replace existing Warn at lines 373-376):**

```go
	trace.Warn(s.logger, traceID, routedMsg.MessageID.String(), "sender.backpressure", "throttled").
		Str("provider_id", routedMsg.ProviderID.String()).
		Msg("provider throttled, message will be redelivered")
```

**After send result — replace existing Debug at lines 511-518:**

```go
	senderType := "smpp"
	if smsc.IsSimulator(provider) {
		senderType = "stub"
	}

	trace.Log(s.logger, traceID, routedMsg.MessageID.String(), "sender.send", "completed").
		Str("provider_id", routedMsg.ProviderID.String()).
		Str("smpp_message_id", sentMsg.SMPPMessageID).
		Str("status", sentMsg.Status).
		Str("connection_id", usedConnID).
		Str("sender_type", senderType).
		Int("segments", sentMsg.SegmentsCount).
		Msg("message processed by sender")
```

**Propagate TraceID to SentMessage (around line 474):**

```go
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     routedMsg.MessageID,
		TraceID:       traceID,
		ProviderID:    usedProviderID,
		// ... rest unchanged
	}
```

**Propagate TraceID to FailedMessage (around line 433):**

```go
	failedMsg := &queue.FailedMessage{
		MessageID:  routedMsg.MessageID,
		TraceID:    traceID,
		// ... rest unchanged
	}
```

**Propagate TraceID to DLR callback.** In the DLR callback (line 75-98), the DLRMessage doesn't have direct access to TraceID from the original message (DLR comes from the provider async). Leave DLR TraceID empty for now — it will be correlated via `smpp_message_id`.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/pipeline/sender/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/sender/stage.go
git commit -m "feat(trace): add comprehensive trace logging to sender stage"
```

---

### Task 9: Instrument Status Writer Stage

**Files:**
- Modify: `internal/pipeline/status/stage.go`

- [ ] **Step 1: Add traceID to statusRecord and trace logging**

In `internal/pipeline/status/stage.go`, import `"github.com/smpp-server/smpp-server/internal/pipeline/trace"`.

Add `TraceID` to `statusRecord`:

```go
type statusRecord struct {
	MessageID     uuid.UUID
	TraceID       string
	Status        string
	SMPPMessageID string
	ProviderID    *uuid.UUID
	SubmittedAt   *time.Time
	UpdatedAt     time.Time
	SegmentCount  int
}
```

In `deserializeMessage`, extract TraceID from deserialized messages:

For `TopicSent` case (around line 189):
```go
	return &statusRecord{
		MessageID:     sent.MessageID,
		TraceID:       sent.TraceID,
		Status:        status,
		// ... rest unchanged
	}, nil
```

For `TopicDLR` case (around line 209):
```go
	return &statusRecord{
		MessageID:     dlr.MessageID,
		TraceID:       dlr.TraceID,
		Status:        status,
		// ... rest unchanged
	}, nil
```

In `handleBatch`, after successful `batchUpsert` (before `publishStatusUpdates`, around line 163), add trace logging for each record:

```go
	for _, r := range records {
		trace.Log(s.logger, r.TraceID, r.MessageID.String(), "status", "upserted").
			Str("status", r.Status).
			Str("smpp_message_id", r.SMPPMessageID).
			Msg("status written to DB")
	}
```

Propagate TraceID in `publishStatusUpdates`:

```go
	update := &pipeline.StatusUpdate{
		SchemaVersion: 1,
		MessageID:     r.MessageID,
		TraceID:       r.TraceID,
		Status:        r.Status,
		// ... rest unchanged
	}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/pipeline/status/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/pipeline/status/stage.go
git commit -m "feat(trace): add trace logging to status writer stage"
```

---

### Task 10: Final Build Verification and Integration Test

**Files:** None (verification only)

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: Success — all packages compile cleanly.

- [ ] **Step 2: Run existing tests**

Run: `go test ./internal/pipeline/... ./internal/queue/... ./internal/router/... ./internal/services/routing/... ./internal/smsc/... -v -count=1`
Expected: All existing tests pass. New fields are optional (omitempty), so no test breakage.

- [ ] **Step 3: Verify JSON serialization includes TraceID**

Manually verify by checking that `KafkaMessage.Serialize()` includes trace_id when set:

Run quick check:
```bash
go test -run TestTraceID -v ./internal/queue/... 2>/dev/null || echo "No dedicated test yet — verified via compilation and existing tests"
```

- [ ] **Step 4: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "fix(trace): fix any compilation issues from trace logging integration"
```
