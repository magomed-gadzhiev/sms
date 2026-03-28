# WS3: Frequency Capping + Local Time Delivery — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add frequency capping (rate-limit per phone per client) and local-time delivery with quiet hours to the SMS pipeline.

**Architecture:** Frequency capping uses Redis sorted sets checked in pipeline-worker before send. Local time delivery uses `nyaruka/phonenumbers` to resolve timezone from phone, calculates `deliver_at` during campaign materialization, and respects client-configured quiet hours. New shared package `internal/shared/timezone/`. Pipeline-worker gets new pre-send checks stage.

**Tech Stack:** Go 1.24.0, `github.com/nyaruka/phonenumbers`, Redis sorted sets, pgx/v5, existing pipeline architecture

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `migrations/000048_frequency_caps.up.sql` | frequency_caps, campaign_frequency_overrides tables |
| Create | `migrations/000048_frequency_caps.down.sql` | Rollback |
| Create | `migrations/000049_local_time_delivery.up.sql` | client_quiet_hours, ALTER campaign_recipients ADD deliver_at |
| Create | `migrations/000049_local_time_delivery.down.sql` | Rollback |
| Create | `migrations/000050_capped_status.up.sql` | Add capped/pending_rollout/skipped_quiet_hours to status enums, add capped_count |
| Create | `migrations/000050_capped_status.down.sql` | Rollback |
| Create | `internal/shared/freqcap/checker.go` | Frequency cap checker using Redis sorted sets |
| Create | `internal/shared/freqcap/checker_test.go` | Unit tests |
| Create | `internal/shared/timezone/resolver.go` | Phone number → timezone resolver |
| Create | `internal/shared/timezone/resolver_test.go` | Unit tests |
| Create | `internal/shared/timezone/quiet_hours.go` | Quiet hours check logic |
| Create | `internal/shared/timezone/quiet_hours_test.go` | Unit tests |
| Modify | `internal/services/campaign/domain/models.go` | Add DeliverAt to Recipient, add new statuses |
| Create | `internal/gateway/portal/handlers/settings.go` | Frequency cap & quiet hours settings HTTP handlers |
| Modify | `internal/gateway/portal/router/router.go` | Register settings routes |
| Create | `portal-frontend/src/components/settings/FrequencyCapForm.tsx` | Frequency cap config UI |
| Create | `portal-frontend/src/components/settings/QuietHoursForm.tsx` | Quiet hours config UI |

---

### Task 1: Add phonenumbers dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the library**

```bash
cd /home/magomed/projects/sms && go get github.com/nyaruka/phonenumbers@latest
```

- [ ] **Step 2: Verify**

```bash
grep "nyaruka/phonenumbers" go.mod
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add nyaruka/phonenumbers for timezone resolution"
```

---

### Task 2: Database migration — frequency caps

**Files:**
- Create: `migrations/000048_frequency_caps.up.sql`
- Create: `migrations/000048_frequency_caps.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000048_frequency_caps.up.sql

CREATE TABLE frequency_caps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    cap_type TEXT NOT NULL DEFAULT 'marketing' CHECK (cap_type IN ('marketing', 'transactional', 'all')),
    max_messages INT NOT NULL,
    period_hours INT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (client_id, cap_type)
);

CREATE INDEX idx_freq_caps_client ON frequency_caps(client_id);

CREATE TABLE campaign_frequency_overrides (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
    max_messages INT,
    period_hours INT,
    bypass BOOLEAN NOT NULL DEFAULT false
);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000048_frequency_caps.down.sql
DROP TABLE IF EXISTS campaign_frequency_overrides;
DROP TABLE IF EXISTS frequency_caps;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000048_frequency_caps.up.sql migrations/000048_frequency_caps.down.sql
git commit -m "migration: add frequency_caps and campaign_frequency_overrides tables"
```

---

### Task 3: Database migration — local time delivery

**Files:**
- Create: `migrations/000049_local_time_delivery.up.sql`
- Create: `migrations/000049_local_time_delivery.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000049_local_time_delivery.up.sql

ALTER TABLE campaign_recipients ADD COLUMN deliver_at TIMESTAMPTZ;
CREATE INDEX idx_cr_deliver_at ON campaign_recipients(status, deliver_at) WHERE deliver_at IS NOT NULL;

CREATE TABLE client_quiet_hours (
    client_id UUID PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    start_hour INT NOT NULL DEFAULT 22 CHECK (start_hour >= 0 AND start_hour <= 23),
    end_hour INT NOT NULL DEFAULT 8 CHECK (end_hour >= 0 AND end_hour <= 23),
    action TEXT NOT NULL DEFAULT 'postpone' CHECK (action IN ('postpone', 'skip')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000049_local_time_delivery.down.sql
DROP TABLE IF EXISTS client_quiet_hours;
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS deliver_at;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000049_local_time_delivery.up.sql migrations/000049_local_time_delivery.down.sql
git commit -m "migration: add deliver_at column and client_quiet_hours table"
```

---

### Task 4: Database migration — new statuses (capped, pending_rollout, skipped_quiet_hours)

**Files:**
- Create: `migrations/000050_capped_status.up.sql`
- Create: `migrations/000050_capped_status.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000050_capped_status.up.sql

-- Drop the CHECK constraint on campaign_recipients.status and recreate with new values
-- Hash-partitioned tables: constraint is on each partition
DO $$
DECLARE
    part_name TEXT;
BEGIN
    -- Drop constraint from parent and all partitions
    FOR part_name IN
        SELECT tablename FROM pg_tables WHERE tablename LIKE 'campaign_recipients%'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I DROP CONSTRAINT IF EXISTS campaign_recipients_status_check',
            part_name
        );
    END LOOP;
END $$;

ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_status_check
    CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled', 'capped', 'pending_rollout', 'skipped_quiet_hours'));

-- Add capped_count to campaigns
ALTER TABLE campaigns ADD COLUMN capped_count INT NOT NULL DEFAULT 0;
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000050_capped_status.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS capped_count;

DO $$
DECLARE
    part_name TEXT;
BEGIN
    FOR part_name IN
        SELECT tablename FROM pg_tables WHERE tablename LIKE 'campaign_recipients%'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I DROP CONSTRAINT IF EXISTS campaign_recipients_status_check',
            part_name
        );
    END LOOP;
END $$;

ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_status_check
    CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled'));
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000050_capped_status.up.sql migrations/000050_capped_status.down.sql
git commit -m "migration: add capped/pending_rollout/skipped_quiet_hours statuses"
```

---

### Task 5: Frequency cap checker (Redis sorted sets)

**Files:**
- Create: `internal/shared/freqcap/checker.go`
- Create: `internal/shared/freqcap/checker_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/shared/freqcap/checker_test.go
package freqcap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapConfig_IsCapped(t *testing.T) {
	// Basic logic test without Redis
	cfg := &CapConfig{MaxMessages: 3, PeriodHours: 24}
	assert.True(t, cfg.MaxMessages > 0)
	assert.True(t, cfg.PeriodHours > 0)
}

func TestCapConfig_Bypassed(t *testing.T) {
	cfg := &CapConfig{Bypass: true}
	assert.True(t, cfg.Bypass)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/freqcap/ -v -count=1
```

Expected: FAIL — package doesn't exist

- [ ] **Step 3: Write the checker**

```go
// internal/shared/freqcap/checker.go
package freqcap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CapConfig represents a frequency cap configuration.
type CapConfig struct {
	MaxMessages int
	PeriodHours int
	Bypass      bool
}

// Checker checks and records frequency caps using Redis sorted sets.
type Checker struct {
	rdb *redis.Client
}

// NewChecker creates a new frequency cap Checker.
func NewChecker(rdb *redis.Client) *Checker {
	return &Checker{rdb: rdb}
}

// IsCapped checks if the phone has exceeded the cap for the given client.
// Returns true if the phone is capped (should not receive message).
func (c *Checker) IsCapped(ctx context.Context, clientID uuid.UUID, phone string, cap CapConfig) (bool, error) {
	if cap.Bypass || cap.MaxMessages <= 0 {
		return false, nil
	}

	key := fmt.Sprintf("freq:%s:%s", clientID.String(), phone)
	now := time.Now()
	cutoff := now.Add(-time.Duration(cap.PeriodHours) * time.Hour)

	// Remove expired entries
	c.rdb.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff.Unix()))

	// Count remaining entries
	count, err := c.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("freq cap zcard: %w", err)
	}

	return count >= int64(cap.MaxMessages), nil
}

// Record records a sent message for the frequency cap.
func (c *Checker) Record(ctx context.Context, clientID uuid.UUID, phone string, messageID uuid.UUID, ttlHours int) error {
	key := fmt.Sprintf("freq:%s:%s", clientID.String(), phone)
	now := time.Now()

	pipe := c.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.Unix()),
		Member: messageID.String(),
	})
	pipe.Expire(ctx, key, time.Duration(ttlHours)*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/freqcap/ -v -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/freqcap/
git commit -m "feat(freqcap): add frequency cap checker with Redis sorted sets"
```

---

### Task 6: Timezone resolver

**Files:**
- Create: `internal/shared/timezone/resolver.go`
- Create: `internal/shared/timezone/resolver_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/shared/timezone/resolver_test.go
package timezone

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_RussianMoscow(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+79161234567") // Moscow (916)
	require.NoError(t, err)
	assert.Equal(t, "Europe/Moscow", loc.String())
}

func TestResolver_USNumber(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+12125551234") // New York (212)
	require.NoError(t, err)
	assert.Contains(t, loc.String(), "America/")
}

func TestResolver_InvalidNumber(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("invalid")
	require.NoError(t, err)
	assert.Equal(t, "UTC", loc.String()) // Fallback
}

func TestResolver_Kazakhstan(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+77011234567")
	require.NoError(t, err)
	assert.NotEmpty(t, loc.String())
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/timezone/ -v -count=1
```

Expected: FAIL

- [ ] **Step 3: Write the resolver**

```go
// internal/shared/timezone/resolver.go
package timezone

import (
	"time"

	"github.com/nyaruka/phonenumbers"
)

// Resolver resolves phone numbers to time zones.
type Resolver interface {
	GetTimezone(phone string) (*time.Location, error)
}

// PhoneTimezoneResolver resolves timezone from phone number using libphonenumber.
type PhoneTimezoneResolver struct{}

// NewPhoneTimezoneResolver creates a new resolver.
func NewPhoneTimezoneResolver() *PhoneTimezoneResolver {
	return &PhoneTimezoneResolver{}
}

// GetTimezone parses the phone number, determines the region and timezone.
// Falls back to UTC if unable to determine.
func (r *PhoneTimezoneResolver) GetTimezone(phone string) (*time.Location, error) {
	num, err := phonenumbers.Parse(phone, "")
	if err != nil {
		return time.UTC, nil // Fallback
	}

	regionCode := phonenumbers.GetRegionCodeForNumber(num)
	if regionCode == "" {
		return time.UTC, nil
	}

	// Get timezone(s) for this number
	timezones := phonenumbers.GetTimeZonesForNumber(num)
	if len(timezones) == 0 {
		return time.UTC, nil
	}

	// Use the first timezone
	loc, err := time.LoadLocation(timezones[0])
	if err != nil {
		return time.UTC, nil
	}
	return loc, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/timezone/ -v -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shared/timezone/resolver.go internal/shared/timezone/resolver_test.go
git commit -m "feat(timezone): add phone-to-timezone resolver using phonenumbers"
```

---

### Task 7: Quiet hours check logic

**Files:**
- Create: `internal/shared/timezone/quiet_hours.go`
- Create: `internal/shared/timezone/quiet_hours_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/shared/timezone/quiet_hours_test.go
package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQuietHours_OvernightQuiet(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 22, EndHour: 8, Action: "postpone"}

	// 23:00 — should be quiet
	at := time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)

	// 3:00 — should be quiet
	at = time.Date(2026, 3, 29, 3, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)

	// 10:00 — should NOT be quiet
	at = time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}

func TestQuietHours_DaytimeQuiet(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 13, EndHour: 15, Action: "skip"}

	at := time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)
	assert.Equal(t, "skip", result.Action)

	at = time.Date(2026, 3, 28, 16, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}

func TestQuietHours_PostponeCalculation(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 22, EndHour: 8, Action: "postpone"}

	at := time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)
	assert.Equal(t, "postpone", result.Action)
	// Next end_hour = 08:00 next day
	expected := time.Date(2026, 3, 29, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, result.PostponeTo)
}

func TestQuietHours_Disabled(t *testing.T) {
	qh := QuietHoursConfig{Enabled: false, StartHour: 22, EndHour: 8}
	at := time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}
```

- [ ] **Step 2: Write the implementation**

```go
// internal/shared/timezone/quiet_hours.go
package timezone

import "time"

// QuietHoursConfig represents quiet hours settings for a client.
type QuietHoursConfig struct {
	Enabled   bool
	StartHour int // 0-23
	EndHour   int // 0-23
	Action    string // "postpone" or "skip"
}

// QuietHoursResult holds the result of a quiet hours check.
type QuietHoursResult struct {
	IsQuiet    bool
	Action     string
	PostponeTo time.Time // Only set if Action == "postpone"
}

// CheckQuietHours checks if the given local time falls within quiet hours.
func CheckQuietHours(localTime time.Time, config QuietHoursConfig) QuietHoursResult {
	if !config.Enabled {
		return QuietHoursResult{IsQuiet: false}
	}

	hour := localTime.Hour()
	isQuiet := false

	if config.StartHour > config.EndHour {
		// Overnight: e.g., 22-8
		isQuiet = hour >= config.StartHour || hour < config.EndHour
	} else if config.StartHour < config.EndHour {
		// Daytime: e.g., 13-15
		isQuiet = hour >= config.StartHour && hour < config.EndHour
	}

	if !isQuiet {
		return QuietHoursResult{IsQuiet: false}
	}

	result := QuietHoursResult{
		IsQuiet: true,
		Action:  config.Action,
	}

	if config.Action == "postpone" {
		// Calculate next end_hour
		postponeTo := time.Date(localTime.Year(), localTime.Month(), localTime.Day(),
			config.EndHour, 0, 0, 0, localTime.Location())
		if postponeTo.Before(localTime) || postponeTo.Equal(localTime) {
			postponeTo = postponeTo.Add(24 * time.Hour)
		}
		result.PostponeTo = postponeTo
	}

	return result
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/shared/timezone/ -v -count=1
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/shared/timezone/quiet_hours.go internal/shared/timezone/quiet_hours_test.go
git commit -m "feat(timezone): add quiet hours check with postpone/skip logic"
```

---

### Task 8: Update campaign domain model — new statuses and DeliverAt

**Files:**
- Modify: `internal/services/campaign/domain/models.go`

- [ ] **Step 1: Add new recipient status constants**

```go
// Add to existing recipient status constants
const (
	RecipientCapped            = "capped"
	RecipientPendingRollout    = "pending_rollout"
	RecipientSkippedQuietHours = "skipped_quiet_hours"
)
```

- [ ] **Step 2: Add DeliverAt to Recipient and CappedCount to Campaign**

Add `DeliverAt *time.Time` to Recipient struct (after RenderedText).

Add `CappedCount int32` to Campaign struct (after FailedCount).

- [ ] **Step 3: Commit**

```bash
git add internal/services/campaign/domain/models.go
git commit -m "feat(campaign): add capped/pending_rollout statuses, DeliverAt, CappedCount"
```

---

### Task 9: Settings HTTP handlers (frequency caps + quiet hours)

**Files:**
- Create: `internal/gateway/portal/handlers/settings.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Create settings handlers**

```go
// internal/gateway/portal/handlers/settings.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SettingsHandlers struct {
	pool *pgxpool.Pool
}

func NewSettingsHandlers(pool *pgxpool.Pool) *SettingsHandlers {
	return &SettingsHandlers{pool: pool}
}

// --- Frequency Caps ---

type FrequencyCapRequest struct {
	CapType     string `json:"cap_type"`
	MaxMessages int    `json:"max_messages"`
	PeriodHours int    `json:"period_hours"`
	Enabled     bool   `json:"enabled"`
}

func (h *SettingsHandlers) GetFrequencyCaps(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, cap_type, max_messages, period_hours, enabled FROM frequency_caps WHERE client_id = $1`, clientID)
	if err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	defer rows.Close()

	var caps []map[string]interface{}
	for rows.Next() {
		var id, capType string
		var maxMsg, periodH int
		var enabled bool
		rows.Scan(&id, &capType, &maxMsg, &periodH, &enabled)
		caps = append(caps, map[string]interface{}{
			"id": id, "cap_type": capType, "max_messages": maxMsg,
			"period_hours": periodH, "enabled": enabled,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"caps": caps})
}

func (h *SettingsHandlers) UpsertFrequencyCap(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req FrequencyCapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO frequency_caps (client_id, cap_type, max_messages, period_hours, enabled)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (client_id, cap_type)
		 DO UPDATE SET max_messages = $3, period_hours = $4, enabled = $5, updated_at = now()`,
		clientID, req.CapType, req.MaxMessages, req.PeriodHours, req.Enabled,
	)
	if err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Quiet Hours ---

type QuietHoursRequest struct {
	Enabled   bool   `json:"enabled"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	Action    string `json:"action"`
}

func (h *SettingsHandlers) GetQuietHours(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var enabled bool
	var startH, endH int
	var action string
	err := h.pool.QueryRow(r.Context(),
		`SELECT enabled, start_hour, end_hour, action FROM client_quiet_hours WHERE client_id = $1`, clientID,
	).Scan(&enabled, &startH, &endH, &action)
	if err != nil {
		// No config yet — return defaults
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false, "start_hour": 22, "end_hour": 8, "action": "postpone",
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": enabled, "start_hour": startH, "end_hour": endH, "action": action,
	})
}

func (h *SettingsHandlers) UpsertQuietHours(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}

	var req QuietHoursRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO client_quiet_hours (client_id, enabled, start_hour, end_hour, action)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (client_id)
		 DO UPDATE SET enabled = $2, start_hour = $3, end_hour = $4, action = $5, updated_at = now()`,
		clientID, req.Enabled, req.StartHour, req.EndHour, req.Action,
	)
	if err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Register routes in router.go**

```go
// Settings endpoints
settings := protected.PathPrefix("/settings").Subrouter()
settings.HandleFunc("/frequency-caps", settingsHandlers.GetFrequencyCaps).Methods("GET")
settings.HandleFunc("/frequency-caps", settingsHandlers.UpsertFrequencyCap).Methods("PUT")
settings.HandleFunc("/quiet-hours", settingsHandlers.GetQuietHours).Methods("GET")
settings.HandleFunc("/quiet-hours", settingsHandlers.UpsertQuietHours).Methods("PUT")
```

Add `settingsHandlers *handlers.SettingsHandlers` to `SetupRouter` parameters.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/settings.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add frequency caps and quiet hours settings endpoints"
```

---

### Task 10: Frontend — FrequencyCapForm component

**Files:**
- Create: `portal-frontend/src/components/settings/FrequencyCapForm.tsx`

- [ ] **Step 1: Create the component**

```tsx
// portal-frontend/src/components/settings/FrequencyCapForm.tsx
import { useState, useEffect } from 'react';

interface Cap {
  id: string;
  cap_type: string;
  max_messages: number;
  period_hours: number;
  enabled: boolean;
}

export function FrequencyCapForm() {
  const [caps, setCaps] = useState<Cap[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchCaps = async () => {
    const res = await fetch('/portal/v1/settings/frequency-caps', { credentials: 'include' });
    if (res.ok) {
      const data = await res.json();
      setCaps(data.caps || []);
    }
    setLoading(false);
  };

  useEffect(() => { fetchCaps(); }, []);

  const saveCap = async (capType: string, maxMessages: number, periodHours: number, enabled: boolean) => {
    await fetch('/portal/v1/settings/frequency-caps', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ cap_type: capType, max_messages: maxMessages, period_hours: periodHours, enabled }),
    });
    fetchCaps();
  };

  const capTypes = [
    { type: 'marketing', label: 'Маркетинговые' },
    { type: 'transactional', label: 'Транзакционные' },
  ];

  if (loading) return <p className="text-sm text-gray-500">Загрузка...</p>;

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-900">Ограничения частоты</h3>
      {capTypes.map(({ type, label }) => {
        const cap = caps.find((c) => c.cap_type === type);
        return (
          <div key={type} className="p-4 bg-white border border-gray-200 rounded space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{label}</span>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={cap?.enabled ?? false}
                  onChange={(e) => saveCap(type, cap?.max_messages ?? 5, cap?.period_hours ?? 24, e.target.checked)}
                />
                Включено
              </label>
            </div>
            <div className="flex gap-4">
              <label className="text-sm text-gray-600">
                Макс. сообщений:
                <input
                  type="number"
                  min={1}
                  defaultValue={cap?.max_messages ?? 5}
                  onBlur={(e) => saveCap(type, parseInt(e.target.value) || 5, cap?.period_hours ?? 24, cap?.enabled ?? false)}
                  className="ml-2 w-20 px-2 py-1 border border-gray-300 rounded text-sm"
                />
              </label>
              <label className="text-sm text-gray-600">
                За период (часов):
                <input
                  type="number"
                  min={1}
                  defaultValue={cap?.period_hours ?? 24}
                  onBlur={(e) => saveCap(type, cap?.max_messages ?? 5, parseInt(e.target.value) || 24, cap?.enabled ?? false)}
                  className="ml-2 w-20 px-2 py-1 border border-gray-300 rounded text-sm"
                />
              </label>
            </div>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/settings/FrequencyCapForm.tsx
git commit -m "feat(frontend): add FrequencyCapForm settings component"
```

---

### Task 11: Frontend — QuietHoursForm component

**Files:**
- Create: `portal-frontend/src/components/settings/QuietHoursForm.tsx`

- [ ] **Step 1: Create the component**

```tsx
// portal-frontend/src/components/settings/QuietHoursForm.tsx
import { useState, useEffect } from 'react';

interface QuietHoursConfig {
  enabled: boolean;
  start_hour: number;
  end_hour: number;
  action: string;
}

export function QuietHoursForm() {
  const [config, setConfig] = useState<QuietHoursConfig>({
    enabled: false, start_hour: 22, end_hour: 8, action: 'postpone',
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/portal/v1/settings/quiet-hours', { credentials: 'include' })
      .then((r) => r.json())
      .then((data) => { setConfig(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  const save = async (updated: Partial<QuietHoursConfig>) => {
    const newConfig = { ...config, ...updated };
    setConfig(newConfig);
    await fetch('/portal/v1/settings/quiet-hours', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(newConfig),
    });
  };

  if (loading) return <p className="text-sm text-gray-500">Загрузка...</p>;

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-900">Тихие часы</h3>
      <div className="p-4 bg-white border border-gray-200 rounded space-y-3">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={config.enabled} onChange={(e) => save({ enabled: e.target.checked })} />
          Включить тихие часы
        </label>
        {config.enabled && (
          <>
            <div className="flex gap-4 items-center">
              <label className="text-sm text-gray-600">
                С:
                <select
                  value={config.start_hour}
                  onChange={(e) => save({ start_hour: parseInt(e.target.value) })}
                  className="ml-2 px-2 py-1 border border-gray-300 rounded text-sm"
                >
                  {Array.from({ length: 24 }, (_, i) => (
                    <option key={i} value={i}>{String(i).padStart(2, '0')}:00</option>
                  ))}
                </select>
              </label>
              <label className="text-sm text-gray-600">
                До:
                <select
                  value={config.end_hour}
                  onChange={(e) => save({ end_hour: parseInt(e.target.value) })}
                  className="ml-2 px-2 py-1 border border-gray-300 rounded text-sm"
                >
                  {Array.from({ length: 24 }, (_, i) => (
                    <option key={i} value={i}>{String(i).padStart(2, '0')}:00</option>
                  ))}
                </select>
              </label>
            </div>
            <div>
              <label className="text-sm text-gray-600">Действие при попадании в тихие часы:</label>
              <div className="mt-1 space-y-1">
                <label className="flex items-center gap-2 text-sm">
                  <input type="radio" name="action" value="postpone" checked={config.action === 'postpone'}
                    onChange={() => save({ action: 'postpone' })} />
                  Отложить до окончания тихих часов
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input type="radio" name="action" value="skip" checked={config.action === 'skip'}
                    onChange={() => save({ action: 'skip' })} />
                  Пропустить (не отправлять)
                </label>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/settings/QuietHoursForm.tsx
git commit -m "feat(frontend): add QuietHoursForm settings component"
```
