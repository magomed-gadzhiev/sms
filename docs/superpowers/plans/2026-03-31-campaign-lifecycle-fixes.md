# Campaign Lifecycle Bug Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two bugs that permanently strand campaigns in `running` status: (1) messages written to DB but never published to Kafka, (2) `campaign_recipients` stuck in `pending` after service restarts cause campaigns to never complete.

**Architecture:** Add two periodic reconciliation workers to the campaign-service. Worker 1 (`processStaleQueued`) re-publishes messages stuck in `queued` state for >5 min. Worker 2 (`reconcileRunningCampaigns`) syncs `campaign_recipients.status` from the `messages` table and closes campaigns whose recipients have all reached terminal status. Logic is extracted into pure functions tested without DB.

**Tech Stack:** Go 1.25, `github.com/IBM/sarama`, `github.com/jmoiron/sqlx`, `github.com/stretchr/testify`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/services/campaign-service/materialize.go` | **Modify** | Add `processStaleQueued` worker + pure helpers |
| `cmd/services/campaign-service/main.go` | **Modify** | Add `reconcileRunningCampaigns` worker + extract testable helpers |
| `cmd/services/campaign-service/materialize_test.go` | **Create** | Unit tests for stale-queued logic |
| `cmd/services/campaign-service/reconcile_test.go` | **Create** | Unit tests for reconciler and status-consumer helpers |

---

## Task 1: Extract pure helpers in `main.go` and unit-test them

The two functions `mapToRecipientStatus` and `statusToCounterColumn` already exist but have no tests. We also need to test `processStatusMessage`'s completion logic. We extract the SQL-free logic into testable form first.

**Files:**
- Test: `cmd/services/campaign-service/reconcile_test.go`

- [ ] **Step 1: Create test file with package declaration and imports**

Create `cmd/services/campaign-service/reconcile_test.go`:

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: Write tests for `mapToRecipientStatus`**

Append to `cmd/services/campaign-service/reconcile_test.go`:

```go
func TestMapToRecipientStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"sent", "sent"},
		{"accepted", "sent"},
		{"delivered", "delivered"},
		{"failed", "failed"},
		{"rejected", "failed"},
		{"expired", "failed"},
		{"undeliverable", "failed"},
		{"unknown", ""},
		{"queued", ""},
		{"", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, mapToRecipientStatus(tt.input))
		})
	}
}
```

- [ ] **Step 3: Write tests for `statusToCounterColumn`**

Append to `cmd/services/campaign-service/reconcile_test.go`:

```go
func TestStatusToCounterColumn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"sent", "sent_count"},
		{"delivered", "delivered_count"},
		{"failed", "failed_count"},
		{"pending", ""},
		{"queued", ""},
		{"", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, statusToCounterColumn(tt.input))
		})
	}
}
```

- [ ] **Step 4: Run tests — expect PASS (functions already exist)**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestMapToRecipientStatus|TestStatusToCounterColumn' -v
```

Expected:
```
--- PASS: TestMapToRecipientStatus (0.00s)
--- PASS: TestStatusToCounterColumn (0.00s)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add cmd/services/campaign-service/reconcile_test.go
git commit -m "test: unit tests for mapToRecipientStatus and statusToCounterColumn"
```

---

## Task 2: Add `processStaleQueued` and unit-test it

This worker finds messages in `queued` state older than 5 minutes and re-publishes them to Kafka. The SQL query and Kafka message construction are extracted into a helper that's tested via a mock producer.

**Files:**
- Modify: `cmd/services/campaign-service/materialize.go`
- Create: `cmd/services/campaign-service/materialize_test.go`

- [ ] **Step 1: Write failing tests for `buildStaleKafkaMsg`**

Create `cmd/services/campaign-service/materialize_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: staleRow is defined in materialize.go — no re-declaration needed here.

func TestBuildStaleKafkaMsg_ValidRow(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	msgID := uuid.New()
	now := time.Now()

	row := staleRow{
		ID:          msgID.String(),
		Source:      "TestSender",
		Destination: "+79001234567",
		Text:        "Hello",
		ClientID:    clientID.String(),
		CreatedAt:   now,
	}

	msg, err := buildStaleKafkaMsg(row)
	require.NoError(t, err)

	assert.Equal(t, msgID.String(), msg.ID)
	assert.Equal(t, msgID, msg.MessageID)
	assert.Equal(t, "TestSender", msg.Source)
	assert.Equal(t, "+79001234567", msg.Destination)
	assert.Equal(t, "Hello", msg.Text)
	require.NotNil(t, msg.ClientID)
	assert.Equal(t, clientID, *msg.ClientID)
	assert.Equal(t, now, msg.CreatedAt)
	assert.Equal(t, 5, msg.MaxRetries)
}

func TestBuildStaleKafkaMsg_InvalidMessageID(t *testing.T) {
	t.Parallel()

	row := staleRow{
		ID:          "not-a-uuid",
		Source:      "S",
		Destination: "+7",
		Text:        "X",
		ClientID:    uuid.New().String(),
		CreatedAt:   time.Now(),
	}

	_, err := buildStaleKafkaMsg(row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message_id")
}

func TestBuildStaleKafkaMsg_InvalidClientID(t *testing.T) {
	t.Parallel()

	row := staleRow{
		ID:          uuid.New().String(),
		Source:      "S",
		Destination: "+7",
		Text:        "X",
		ClientID:    "not-a-uuid",
		CreatedAt:   time.Now(),
	}

	_, err := buildStaleKafkaMsg(row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client_id")
}
```

- [ ] **Step 2: Run tests — expect FAIL (function not defined)**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestBuildStaleKafkaMsg' -v 2>&1 | head -20
```

Expected: compile error — `buildStaleKafkaMsg undefined`.

- [ ] **Step 3: Implement `buildStaleKafkaMsg` and `processStaleQueued` in `materialize.go`**

Add the following to the end of `cmd/services/campaign-service/materialize.go`, after the existing `processUnsentRecipients` function:

```go
// staleRow holds the DB columns fetched for re-queuing.
type staleRow struct {
	ID          string
	Source      string
	Destination string
	Text        string
	ClientID    string
	CreatedAt   time.Time
}

// buildStaleKafkaMsg converts a staleRow into a kafkaOutgoingMsg ready for publishing.
// Extracted as a pure function so it can be unit-tested without DB.
func buildStaleKafkaMsg(row staleRow) (kafkaOutgoingMsg, error) {
	msgID, err := uuid.Parse(row.ID)
	if err != nil {
		return kafkaOutgoingMsg{}, fmt.Errorf("invalid message_id %q: %w", row.ID, err)
	}
	clientID, err := uuid.Parse(row.ClientID)
	if err != nil {
		return kafkaOutgoingMsg{}, fmt.Errorf("invalid client_id %q: %w", row.ClientID, err)
	}
	return kafkaOutgoingMsg{
		ID:          msgID.String(),
		MessageID:   msgID,
		Source:      row.Source,
		Destination: row.Destination,
		Text:        row.Text,
		ClientID:    &clientID,
		MaxRetries:  5,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// processStaleQueued finds messages in 'queued' state older than staleThreshold
// and re-publishes them to sms.outgoing. This recovers messages whose Kafka
// publish failed during materialization.
func processStaleQueued(
	ctx context.Context,
	dbx *sqlx.DB,
	producer sarama.SyncProducer,
	topicOutgoing string,
	staleThreshold time.Duration,
	logger zerolog.Logger,
) error {
	rows, err := dbx.QueryContext(ctx, `
		SELECT id::text, source, destination, text, client_id::text, created_at
		FROM messages
		WHERE status = 'queued'
		  AND updated_at < now() - $1::interval
		LIMIT 100`,
		staleThreshold.String(),
	)
	if err != nil {
		return fmt.Errorf("query stale queued: %w", err)
	}
	defer rows.Close()

	var stale []staleRow
	for rows.Next() {
		var r staleRow
		if err := rows.Scan(&r.ID, &r.Source, &r.Destination, &r.Text, &r.ClientID, &r.CreatedAt); err != nil {
			logger.Error().Err(err).Msg("ошибка сканирования stale row")
			continue
		}
		stale = append(stale, r)
	}
	rows.Close()

	var published int
	for _, row := range stale {
		msg, err := buildStaleKafkaMsg(row)
		if err != nil {
			logger.Error().Err(err).Str("message_id", row.ID).Msg("ошибка построения Kafka сообщения для stale")
			continue
		}
		data, _ := json.Marshal(msg)
		if _, _, kafkaErr := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topicOutgoing,
			Value: sarama.ByteEncoder(data),
		}); kafkaErr != nil {
			logger.Error().Err(kafkaErr).Str("message_id", row.ID).Msg("ошибка публикации stale сообщения")
			continue // do not refresh updated_at so it will be retried
		}
		// Refresh updated_at so we don't re-publish on the next tick.
		_, _ = dbx.ExecContext(ctx,
			`UPDATE messages SET updated_at = now() WHERE id = $1`, row.ID)
		published++
	}

	if published > 0 {
		logger.Info().Int("count", published).Msg("переотправлены зависшие queued сообщения")
	}
	return nil
}
```

- [ ] **Step 4: Wire `processStaleQueued` into `startMaterializationWorker`**

In `cmd/services/campaign-service/materialize.go`, add a third goroutine inside `startMaterializationWorker`, after the `processUnsentRecipients` goroutine:

```go
	// Worker to re-publish messages that are stuck in 'queued' state
	// (Kafka publish failed during materialization).
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processStaleQueued(ctx, dbx, producer, topicOutgoing, 5*time.Minute, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка переотправки stale queued сообщений")
				}
			}
		}
	}()
```

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestBuildStaleKafkaMsg' -v
```

Expected:
```
--- PASS: TestBuildStaleKafkaMsg_ValidRow (0.00s)
--- PASS: TestBuildStaleKafkaMsg_InvalidMessageID (0.00s)
--- PASS: TestBuildStaleKafkaMsg_InvalidClientID (0.00s)
PASS
```

- [ ] **Step 6: Verify build**

```bash
cd /home/magomed/projects/sms
go build ./cmd/services/campaign-service/
```

Expected: no output (success).

- [ ] **Step 7: Commit**

```bash
git add cmd/services/campaign-service/materialize.go cmd/services/campaign-service/materialize_test.go
git commit -m "fix: re-publish stale queued messages that missed Kafka during materialization"
```

---

## Task 3: Add `reconcileRunningCampaigns` and unit-test it

This worker syncs `campaign_recipients.status` from `messages.status` and closes fully-processed campaigns. The completion check is extracted as `isCampaignComplete` — a pure function over recipient status counts — so it can be tested without DB.

**Files:**
- Modify: `cmd/services/campaign-service/main.go`
- Modify: `cmd/services/campaign-service/reconcile_test.go`

- [ ] **Step 1: Write failing tests for `isCampaignComplete`**

Append to `cmd/services/campaign-service/reconcile_test.go`:

```go
func TestIsCampaignComplete_AllDelivered(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"delivered": 85,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_AllFailed(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"failed": 10,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_MixedFinalStatuses(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"delivered": 70,
		"failed":    15,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_HasPending(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"pending":   5,
		"delivered": 80,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_HasSent(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"sent":      3,
		"delivered": 82,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_AllPending(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"pending": 85,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_Empty(t *testing.T) {
	t.Parallel()
	assert.True(t, isCampaignComplete(map[string]int{}))
}
```

- [ ] **Step 2: Run tests — expect FAIL (function not defined)**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestIsCampaignComplete' -v 2>&1 | head -10
```

Expected: compile error — `isCampaignComplete undefined`.

- [ ] **Step 3: Implement `isCampaignComplete` and `reconcileRunningCampaigns` in `main.go`**

Add at the end of `cmd/services/campaign-service/main.go`, before the final closing brace (after `statusToCounterColumn`):

```go
// isCampaignComplete returns true when there are no recipients in a non-terminal
// state (pending or sent). Extracted as a pure function for testability.
func isCampaignComplete(counts map[string]int) bool {
	return counts["pending"] == 0 && counts["sent"] == 0
}

// reconcileRunningCampaigns fixes campaigns whose campaign_recipients.status
// has diverged from messages.status (e.g. after a service restart that missed
// sms.status Kafka events).
//
// It runs three SQL steps:
//  1. Sync recipient status from messages table.
//  2. Recount per-campaign status totals and update counters.
//  3. Mark campaigns with no pending/sent recipients as completed.
func reconcileRunningCampaigns(ctx context.Context, dbx *sqlx.DB, logger zerolog.Logger) error {
	// Step 1: sync campaign_recipients.status from messages for running campaigns.
	res, err := dbx.ExecContext(ctx, `
		UPDATE campaign_recipients cr
		SET    status     = m.status,
		       updated_at = now()
		FROM   messages m
		WHERE  m.id = cr.message_id
		  AND  m.status IN ('sent', 'delivered', 'failed', 'expired', 'rejected')
		  AND  cr.status  = 'pending'
		  AND  cr.campaign_id IN (
		           SELECT id FROM campaigns WHERE status = 'running'
		       )
	`)
	if err != nil {
		return fmt.Errorf("reconcile sync recipients: %w", err)
	}
	synced, _ := res.RowsAffected()
	if synced == 0 {
		return nil // nothing to do
	}
	logger.Info().Int64("synced", synced).Msg("reconcile: обновлены статусы получателей")

	// Step 2: recount counters for affected campaigns and check completion.
	rows, err := dbx.QueryContext(ctx, `
		SELECT campaign_id, status, COUNT(*) as cnt
		FROM   campaign_recipients
		WHERE  campaign_id IN (SELECT id FROM campaigns WHERE status = 'running')
		GROUP BY campaign_id, status
	`)
	if err != nil {
		return fmt.Errorf("reconcile count recipients: %w", err)
	}
	defer rows.Close()

	// Aggregate counts per campaign.
	type campaignCounts struct {
		counts map[string]int
	}
	campaignMap := map[string]*campaignCounts{}
	for rows.Next() {
		var campaignID, status string
		var cnt int
		if err := rows.Scan(&campaignID, &status, &cnt); err != nil {
			continue
		}
		if campaignMap[campaignID] == nil {
			campaignMap[campaignID] = &campaignCounts{counts: map[string]int{}}
		}
		campaignMap[campaignID].counts[status] = cnt
	}
	rows.Close()

	for campaignID, cc := range campaignMap {
		// Update counters.
		_, err := dbx.ExecContext(ctx, `
			UPDATE campaigns SET
				sent_count      = $1,
				delivered_count = $2,
				failed_count    = $3,
				updated_at      = now()
			WHERE id = $4`,
			cc.counts["sent"]+cc.counts["delivered"]+cc.counts["failed"],
			cc.counts["delivered"],
			cc.counts["failed"],
			campaignID,
		)
		if err != nil {
			logger.Error().Err(err).Str("campaign_id", campaignID).Msg("reconcile: ошибка обновления счётчиков")
			continue
		}

		// Step 3: mark complete if no pending/sent remain.
		if isCampaignComplete(cc.counts) {
			res, err := dbx.ExecContext(ctx, `
				UPDATE campaigns
				SET    status       = 'completed',
				       completed_at = now(),
				       updated_at   = now()
				WHERE  id     = $1
				  AND  status = 'running'`,
				campaignID,
			)
			if err != nil {
				logger.Error().Err(err).Str("campaign_id", campaignID).Msg("reconcile: ошибка завершения кампании")
				continue
			}
			if n, _ := res.RowsAffected(); n > 0 {
				logger.Info().Str("campaign_id", campaignID).Msg("reconcile: кампания завершена")
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Wire `reconcileRunningCampaigns` into `main` startup**

In `cmd/services/campaign-service/main.go`, inside the `if kafkaProducer != nil` block, after the call to `startMaterializationWorker`, add a new goroutine:

```go
	// Reconciler: syncs campaign_recipients status from messages table
	// and closes fully-processed running campaigns.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := reconcileRunningCampaigns(ctx, dbx, logger); err != nil {
					logger.Error().Err(err).Msg("ошибка reconcile running campaigns")
				}
			}
		}
	}()
```

- [ ] **Step 5: Run tests — expect PASS**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestIsCampaignComplete' -v
```

Expected:
```
--- PASS: TestIsCampaignComplete_AllDelivered (0.00s)
--- PASS: TestIsCampaignComplete_AllFailed (0.00s)
--- PASS: TestIsCampaignComplete_MixedFinalStatuses (0.00s)
--- PASS: TestIsCampaignComplete_HasPending (0.00s)
--- PASS: TestIsCampaignComplete_HasSent (0.00s)
--- PASS: TestIsCampaignComplete_AllPending (0.00s)
--- PASS: TestIsCampaignComplete_Empty (0.00s)
PASS
```

- [ ] **Step 6: Run all campaign-service tests**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -v
```

Expected: all tests PASS.

- [ ] **Step 7: Verify build**

```bash
cd /home/magomed/projects/sms
go build ./cmd/services/campaign-service/
```

Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add cmd/services/campaign-service/main.go cmd/services/campaign-service/reconcile_test.go
git commit -m "fix: reconcile running campaigns — sync recipient status and auto-complete"
```

---

## Task 4: Test the `processStatusMessage` completion path

The existing `processStatusMessage` handler in `main.go` already uses `mapToRecipientStatus` and `statusToCounterColumn`. Add tests that verify the completion trigger logic via the `isCampaignComplete` helper.

**Files:**
- Modify: `cmd/services/campaign-service/reconcile_test.go`

- [ ] **Step 1: Write tests for completion detection via `isCampaignComplete`**

These tests document the exact condition under which `processStatusMessage` would trigger campaign completion. Append to `cmd/services/campaign-service/reconcile_test.go`:

```go
// TestCompletionCondition_PendingBlocksCompletion verifies that a single
// pending recipient prevents campaign from being marked complete —
// mirroring the SQL WHERE status IN ('pending','sent') check in processStatusMessage.
func TestCompletionCondition_PendingBlocksCompletion(t *testing.T) {
	t.Parallel()

	// Scenario: 84 delivered, 1 still pending → not complete
	counts := map[string]int{"delivered": 84, "pending": 1}
	assert.False(t, isCampaignComplete(counts),
		"campaign must not complete while a recipient is still pending")
}

// TestCompletionCondition_SentBlocksCompletion verifies that a recipient in
// 'sent' state (awaiting DLR) also blocks completion.
func TestCompletionCondition_SentBlocksCompletion(t *testing.T) {
	t.Parallel()

	counts := map[string]int{"delivered": 84, "sent": 1}
	assert.False(t, isCampaignComplete(counts),
		"campaign must not complete while a recipient is awaiting DLR (sent)")
}

// TestCompletionCondition_LastRecipientDelivered verifies that transitioning
// the last pending recipient to delivered triggers completion.
func TestCompletionCondition_LastRecipientDelivered(t *testing.T) {
	t.Parallel()

	before := map[string]int{"delivered": 84, "pending": 1}
	assert.False(t, isCampaignComplete(before))

	after := map[string]int{"delivered": 85}
	assert.True(t, isCampaignComplete(after),
		"campaign must complete once all recipients reach terminal status")
}

// TestCompletionCondition_LastRecipientFailed verifies that a final 'failed'
// recipient also completes the campaign (failure is terminal).
func TestCompletionCondition_LastRecipientFailed(t *testing.T) {
	t.Parallel()

	counts := map[string]int{"delivered": 80, "failed": 5}
	assert.True(t, isCampaignComplete(counts),
		"failed is a terminal status — campaign should complete")
}
```

- [ ] **Step 2: Run tests — expect PASS**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -run 'TestCompletionCondition' -v
```

Expected:
```
--- PASS: TestCompletionCondition_PendingBlocksCompletion (0.00s)
--- PASS: TestCompletionCondition_SentBlocksCompletion (0.00s)
--- PASS: TestCompletionCondition_LastRecipientDelivered (0.00s)
--- PASS: TestCompletionCondition_LastRecipientFailed (0.00s)
PASS
```

- [ ] **Step 3: Run full test suite**

```bash
cd /home/magomed/projects/sms
go test ./cmd/services/campaign-service/ -v
```

Expected: all tests PASS, no compilation errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/services/campaign-service/reconcile_test.go
git commit -m "test: completion condition edge cases for campaign lifecycle"
```

---

## Task 5: Deploy and verify on server

- [ ] **Step 1: Push to GitHub**

```bash
git push origin master
```

- [ ] **Step 2: Deploy campaign-service**

```bash
bash scripts/server.sh deploy campaign-service
```

Expected output includes: `[OK] campaign-service rebuilt and restarted`

- [ ] **Step 3: Wait for reconciler first tick (30 seconds) and check logs**

```bash
bash scripts/server.sh logs campaign-service 2>&1 | grep -E "reconcile|завершена|synced" | head -20
```

Expected (within ~30 seconds of startup):
```
... reconcile: обновлены статусы получателей synced=N
... reconcile: кампания завершена campaign_id=...
```

(For each of the 6 stuck campaigns.)

- [ ] **Step 4: Verify stuck campaigns are now completed in DB**

```bash
bash scripts/server.sh exec "docker exec postgres psql -U smpp smpp_db -c \"
SELECT id, status, sent_count, delivered_count, failed_count
FROM campaigns
WHERE status = 'running'
ORDER BY created_at;
\""
```

Expected: **0 rows** (all previously stuck campaigns are now `completed`).

- [ ] **Step 5: Verify the stale-queued worker runs without errors**

```bash
bash scripts/server.sh logs campaign-service 2>&1 | grep -i "stale\|queued\|переотправлены" | head -10
```

Expected: either no output (nothing stale) or:
```
... переотправлены зависшие queued сообщения count=85
```
for campaign `0ad1d0c2`.

- [ ] **Step 6: Confirm campaign `0ad1d0c2` messages processed (within ~5 min)**

After the stale worker re-publishes the 85 messages and the pipeline processes them:

```bash
bash scripts/server.sh exec "docker exec postgres psql -U smpp smpp_db -c \"
SELECT c.status, c.sent_count, c.delivered_count
FROM campaigns c
WHERE c.id = '0ad1d0c2-da37-4afd-840c-4ce19778c19e';
\""
```

Expected: `status=completed`, `sent_count=85`, `delivered_count=85`.

- [ ] **Step 7: Final status check — no campaigns stuck in running**

```bash
bash scripts/server.sh exec "docker exec postgres psql -U smpp smpp_db -c \"
SELECT COUNT(*) as stuck_running
FROM campaigns
WHERE status = 'running'
  AND updated_at < now() - INTERVAL '10 minutes';
\""
```

Expected: `stuck_running = 0`.
