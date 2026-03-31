# Design: Campaign Lifecycle Bug Fixes

**Date:** 2026-03-31
**Status:** Approved

---

## Problem Statement

Two bugs cause campaigns to get permanently stuck:

1. **Bug 1 — "queued in DB, not in Kafka"**: During `materializeCampaign`, each message is inserted into the DB (status=`queued`) and then published to Kafka separately. If the Kafka publish fails, the message stays in DB as `queued` forever. `processUnsentRecipients` won't help because the recipient already has a `message_id` (filter is `message_id IS NULL`). Observed: campaign `0ad1d0c2` — 85 messages stuck in `queued`.

2. **Bug 2 — "campaign_recipients stuck at `pending`"**: `processStatusMessage` updates `campaign_recipients.status` only when a `sms.status` event arrives. If campaign-service restarts and the Kafka consumer group offset has advanced past those events, the recipients stay `pending` forever. Campaign completion checks `COUNT(*) WHERE status IN ('pending', 'sent')` — never triggers. Observed: 6 campaigns stuck in `running`.

---

## Architecture

### Fix 1: `processStaleQueued` worker (in `materialize.go`)

A new periodic goroutine running every 2 minutes:

```
SELECT id, source, destination, text, client_id, created_at
FROM messages
WHERE status = 'queued'
  AND updated_at < now() - INTERVAL '5 minutes'
LIMIT 100
```

For each found message:
- Re-serialize into `kafkaOutgoingMsg`
- Publish to `sms.outgoing`
- UPDATE `messages SET updated_at = now()` to prevent re-queuing on next tick

**Idempotency**: the pipeline-router and pipeline-sender are already idempotent (persist stage uses `ON CONFLICT ... WHERE updated_at <`). Re-publishing a message that's already been processed will be a no-op.

**Threshold**: 5 minutes. Messages should normally transit from `queued` to `sent` in under 30 seconds. 5 minutes means a clear failure with no false positives.

### Fix 2: `reconcileRunningCampaigns` worker (in `main.go`)

A new periodic goroutine running every 30 seconds:

**Step 1 — sync recipient status from messages:**
```sql
UPDATE campaign_recipients cr
SET status    = m.status,
    updated_at = now()
FROM messages m
WHERE m.id = cr.message_id
  AND m.status IN ('sent', 'delivered', 'failed', 'expired', 'rejected')
  AND cr.status = 'pending'
  AND cr.campaign_id IN (SELECT id FROM campaigns WHERE status = 'running')
```

**Step 2 — rebuild campaign counters:**
For each affected campaign, recount recipients by status and update `sent_count`, `delivered_count`, `failed_count`.

**Step 3 — check completion:**
```sql
UPDATE campaigns
SET status = 'completed', completed_at = now(), updated_at = now()
WHERE id = $1
  AND status = 'running'
  AND NOT EXISTS (
    SELECT 1 FROM campaign_recipients
    WHERE campaign_id = $1
      AND status IN ('pending', 'sent')
  )
```

**Retroactive**: on first run after deploy, fixes all 6 currently stuck campaigns.

---

## Data Flow

```
materializeCampaign
  ├── INSERT message (status=queued)
  ├── producer.SendMessage → [failure path]
  │   └── message stays queued > 5min
  │       └── processStaleQueued re-publishes → pipeline picks up
  └── [success path] → pipeline processes → sms.status event
        └── processStatusMessage updates recipient status (existing)
            [if event missed due to restart]
                └── reconcileRunningCampaigns syncs from messages table
```

---

## Tests

### Unit tests — `materialize.go`

- `TestProcessStaleQueued_PublishesStaleMessages`: mock producer, messages older than threshold → published
- `TestProcessStaleQueued_SkipsRecentMessages`: messages newer than threshold → not published
- `TestProcessStaleQueued_UpdatesUpdatedAt`: after publish, `updated_at` is refreshed
- `TestProcessStaleQueued_LimitRespected`: max 100 messages per run

### Unit tests — `main.go` (statusConsumerHandler)

- `TestReconcileRunningCampaigns_SyncsPendingToSent`: recipient `pending` + message `sent` → becomes `sent`
- `TestReconcileRunningCampaigns_SyncsPendingToDelivered`: recipient `pending` + message `delivered` → becomes `delivered`, campaign completes
- `TestReconcileRunningCampaigns_NoChangeForAlreadyFinal`: recipient already `delivered` → unchanged
- `TestReconcileRunningCampaigns_CompletesWhenAllDone`: all recipients in final state → campaign `completed`
- `TestReconcileRunningCampaigns_SkipsNonRunningCampaigns`: paused/completed campaigns → not touched
- `TestMapToRecipientStatus_AllMappings`: full coverage of `mapToRecipientStatus`
- `TestStatusToCounterColumn_AllMappings`: full coverage of `statusToCounterColumn`
- `TestProcessStatusMessage_CompletionTriggered`: after last pending/sent recipient transitions → campaign marked `completed`
- `TestProcessStatusMessage_NoCompletionIfPendingRemain`: still has pending → not completed

---

## Error Handling

- Both workers log errors and continue (non-fatal).
- `processStaleQueued`: if Kafka publish fails, `updated_at` is NOT refreshed → message will be retried next cycle.
- `reconcileRunningCampaigns`: if DB update fails for one campaign, continue to next.

---

## Deployment

- No schema changes required.
- Both workers start automatically on campaign-service startup.
- Retroactively fixes existing stuck campaigns on first run.
- No data loss risk: all operations are UPDATE-only, no DELETEs.
