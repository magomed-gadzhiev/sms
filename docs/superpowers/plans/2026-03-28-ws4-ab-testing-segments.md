# WS4: A/B Test-Then-Send + Smart Segments — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend A/B testing with test-then-send strategy (test on sample, auto-pick winner, rollout to rest) and add cross-list saved segments with visual rule builder.

**Architecture:** A/B test-then-send extends existing campaign_ab_config with strategy/test_percentage/test_phase fields. Winner selection runs via cron-like goroutine in campaign-service. Smart Segments adds saved_segments table with JSONB rules, translated to parameterized SQL queries for contact filtering. New segment-service logic lives in contact-service. Frontend gets ABStatusPanel and SegmentBuilder components.

**Tech Stack:** Go 1.24.0, pgx/v5, existing campaign-service/contact-service architecture, React 19 + TypeScript

**Dependencies:** WS2 (click metrics for A/B winning_metric=click_rate), WS3 (capped status for recipients)

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `migrations/000051_ab_test_then_send.up.sql` | ALTER campaign_ab_config, campaign_variants, campaign_stats_snapshots |
| Create | `migrations/000051_ab_test_then_send.down.sql` | Rollback |
| Create | `migrations/000052_smart_segments.up.sql` | saved_segments table, ALTER campaigns ADD segment_id |
| Create | `migrations/000052_smart_segments.down.sql` | Rollback |
| Modify | `internal/services/campaign/domain/models.go` | Extend ABConfig with strategy/test fields, add click counts to Variant |
| Modify | `internal/services/campaign/application/campaign_service.go` | Add test-then-send lifecycle methods |
| Create | `internal/services/campaign/application/winner_selector.go` | Auto winner selection goroutine |
| Create | `internal/services/campaign/application/winner_selector_test.go` | Tests |
| Modify | `api/proto/campaign/campaign.proto` | Add GetABStatus, new AB fields |
| Modify | `internal/services/campaign/grpc/server.go` | Add GetABStatus handler |
| Modify | `internal/gateway/portal/handlers/campaigns.go` | Add GetABStatus HTTP handler |
| Modify | `internal/gateway/portal/router/router.go` | Register new campaign routes |
| Create | `internal/services/contact/domain/segment.go` | Segment domain models (extend existing) |
| Create | `internal/services/contact/application/segment_service.go` | Segment CRUD + SQL builder |
| Create | `internal/services/contact/application/segment_service_test.go` | Tests |
| Create | `internal/services/contact/infrastructure/repository/segment_repo.go` | Segment repository |
| Create | `api/proto/contact/segment.proto` | Segment gRPC definition |
| Create | `internal/services/contact/grpc/segment_server.go` | Segment gRPC server |
| Create | `internal/gateway/portal/handlers/segments.go` | Segment HTTP handlers |
| Modify | `internal/gateway/portal/router/router.go` | Register segment routes |
| Create | `portal-frontend/src/pages/segments/SegmentsPage.tsx` | Segment list page |
| Create | `portal-frontend/src/pages/segments/SegmentDetailPage.tsx` | Segment detail/edit |
| Create | `portal-frontend/src/components/segments/SegmentBuilder.tsx` | Visual rule builder |
| Create | `portal-frontend/src/components/campaigns/ABStatusPanel.tsx` | A/B test status panel |
| Modify | `portal-frontend/src/App.tsx` | Add segment routes |
| Modify | `portal-frontend/src/components/layout/UserLayout.tsx` | Add Segments nav item |

---

### Task 1: Database migration — A/B test-then-send extensions

**Files:**
- Create: `migrations/000051_ab_test_then_send.up.sql`
- Create: `migrations/000051_ab_test_then_send.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000051_ab_test_then_send.up.sql

-- Extend campaign_ab_config for test-then-send
ALTER TABLE campaign_ab_config ADD COLUMN strategy VARCHAR(20) NOT NULL DEFAULT 'full_split';
ALTER TABLE campaign_ab_config ADD COLUMN test_percentage INT NOT NULL DEFAULT 100;
ALTER TABLE campaign_ab_config ADD COLUMN test_phase VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE campaign_ab_config ADD COLUMN winning_metric VARCHAR(20) NOT NULL DEFAULT 'delivery_rate';
ALTER TABLE campaign_ab_config ADD COLUMN test_started_at TIMESTAMPTZ;
ALTER TABLE campaign_ab_config ADD COLUMN rollout_started_at TIMESTAMPTZ;

-- Drop existing metric CHECK constraint and replace
ALTER TABLE campaign_ab_config DROP CONSTRAINT IF EXISTS campaign_ab_config_metric_check;
ALTER TABLE campaign_ab_config ADD CONSTRAINT campaign_ab_config_metric_check
    CHECK (metric IN ('delivery_rate', 'click_rate', 'unique_click_rate'));

-- Add click counts to variants
ALTER TABLE campaign_variants ADD COLUMN click_count INT NOT NULL DEFAULT 0;
ALTER TABLE campaign_variants ADD COLUMN unique_click_count INT NOT NULL DEFAULT 0;

-- Add click columns to stats snapshots
ALTER TABLE campaign_stats_snapshots ADD COLUMN clicks INT NOT NULL DEFAULT 0;
ALTER TABLE campaign_stats_snapshots ADD COLUMN unique_clicks INT NOT NULL DEFAULT 0;
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000051_ab_test_then_send.down.sql
ALTER TABLE campaign_stats_snapshots DROP COLUMN IF EXISTS unique_clicks;
ALTER TABLE campaign_stats_snapshots DROP COLUMN IF EXISTS clicks;
ALTER TABLE campaign_variants DROP COLUMN IF EXISTS unique_click_count;
ALTER TABLE campaign_variants DROP COLUMN IF EXISTS click_count;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS rollout_started_at;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_started_at;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS winning_metric;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_phase;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS test_percentage;
ALTER TABLE campaign_ab_config DROP COLUMN IF EXISTS strategy;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000051_ab_test_then_send.up.sql migrations/000051_ab_test_then_send.down.sql
git commit -m "migration: extend AB config for test-then-send strategy"
```

---

### Task 2: Database migration — smart segments

**Files:**
- Create: `migrations/000052_smart_segments.up.sql`
- Create: `migrations/000052_smart_segments.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000052_smart_segments.up.sql

CREATE TABLE saved_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    contact_list_ids UUID[] NOT NULL,
    rules JSONB NOT NULL DEFAULT '{}',
    tag_rules JSONB,
    estimated_count INT NOT NULL DEFAULT 0,
    estimated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_segments_client ON saved_segments(client_id);

-- Add segment_id to campaigns
ALTER TABLE campaigns ADD COLUMN segment_id UUID REFERENCES saved_segments(id);
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000052_smart_segments.down.sql
ALTER TABLE campaigns DROP COLUMN IF EXISTS segment_id;
DROP TABLE IF EXISTS saved_segments;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000052_smart_segments.up.sql migrations/000052_smart_segments.down.sql
git commit -m "migration: add saved_segments table and campaigns.segment_id"
```

---

### Task 3: Extend campaign domain models for A/B test-then-send

**Files:**
- Modify: `internal/services/campaign/domain/models.go`

- [ ] **Step 1: Extend ABConfig struct**

Replace the existing `ABConfig` struct with:

```go
type ABConfig struct {
	CampaignID        uuid.UUID
	Metric            string
	TestDurationHours int32
	AutoSelectWinner  bool
	WinnerVariantID   *uuid.UUID
	WinnerSelectedAt  *time.Time
	// Test-then-send fields
	Strategy          string     // "full_split" | "test_then_send"
	TestPercentage    int32      // 1-100
	TestPhase         string     // "none" | "testing" | "waiting_winner" | "rollout" | "completed"
	WinningMetric     string     // "delivery_rate" | "click_rate" | "unique_click_rate"
	TestStartedAt     *time.Time
	RolloutStartedAt  *time.Time
}
```

- [ ] **Step 2: Add click counts to Variant struct**

Add these fields to the `Variant` struct after `FailedCount`:

```go
ClickCount       int32
UniqueClickCount int32
```

- [ ] **Step 3: Add test phase constants**

```go
const (
	TestPhaseNone           = "none"
	TestPhaseTesting        = "testing"
	TestPhaseWaitingWinner  = "waiting_winner"
	TestPhaseRollout        = "rollout"
	TestPhaseCompleted      = "completed"
)

const (
	StrategyFullSplit    = "full_split"
	StrategyTestThenSend = "test_then_send"
)
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/campaign/domain/models.go
git commit -m "feat(campaign): extend ABConfig for test-then-send, add click counts to Variant"
```

---

### Task 4: Winner selector — automatic winner selection

**Files:**
- Create: `internal/services/campaign/application/winner_selector.go`
- Create: `internal/services/campaign/application/winner_selector_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/services/campaign/application/winner_selector_test.go
package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/stretchr/testify/assert"
)

func TestSelectWinnerByDeliveryRate(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 200, DeliveredCount: 180, FailedCount: 20},
		{ID: uuid.New(), Name: "B", SentCount: 200, DeliveredCount: 150, FailedCount: 50},
	}
	winner := selectBestVariant(variants, "delivery_rate")
	assert.Equal(t, variants[0].ID, winner.ID) // A has 90% vs B's 75%
}

func TestSelectWinnerByClickRate(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	variants := []domain.Variant{
		{ID: a, Name: "A", SentCount: 200, DeliveredCount: 180, ClickCount: 10},
		{ID: b, Name: "B", SentCount: 200, DeliveredCount: 150, ClickCount: 20},
	}
	winner := selectBestVariant(variants, "click_rate")
	assert.Equal(t, b, winner.ID) // B has higher click rate
}

func TestSelectWinnerTie(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 200, DeliveredCount: 180},
		{ID: uuid.New(), Name: "B", SentCount: 201, DeliveredCount: 181}, // Almost same rate
	}
	winner := selectBestVariant(variants, "delivery_rate")
	// When tie (<1% diff), pick the one with larger sample
	assert.Equal(t, variants[1].ID, winner.ID)
}

func TestMinSampleSize(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 50},
		{ID: uuid.New(), Name: "B", SentCount: 50},
	}
	assert.False(t, hasMinSampleSize(variants))

	variants[0].SentCount = 100
	variants[1].SentCount = 100
	assert.True(t, hasMinSampleSize(variants))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/campaign/application/ -v -run TestSelect -count=1
```

Expected: FAIL

- [ ] **Step 3: Write the winner selector**

```go
// internal/services/campaign/application/winner_selector.go
package application

import (
	"math"

	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

const minSamplePerVariant = 100
const tieThreshold = 0.01 // 1%

// selectBestVariant picks the best variant by the given metric.
func selectBestVariant(variants []domain.Variant, metric string) domain.Variant {
	if len(variants) == 0 {
		return domain.Variant{}
	}

	best := variants[0]
	bestScore := variantScore(best, metric)

	for _, v := range variants[1:] {
		score := variantScore(v, metric)
		diff := math.Abs(score - bestScore)

		if diff < tieThreshold {
			// Tie — pick larger sample
			if v.SentCount > best.SentCount {
				best = v
				bestScore = score
			}
		} else if score > bestScore {
			best = v
			bestScore = score
		}
	}
	return best
}

func variantScore(v domain.Variant, metric string) float64 {
	if v.SentCount == 0 {
		return 0
	}
	switch metric {
	case "delivery_rate":
		return float64(v.DeliveredCount) / float64(v.SentCount)
	case "click_rate":
		return float64(v.ClickCount) / float64(v.SentCount)
	case "unique_click_rate":
		return float64(v.UniqueClickCount) / float64(v.SentCount)
	default:
		return float64(v.DeliveredCount) / float64(v.SentCount)
	}
}

// hasMinSampleSize checks if all variants have enough sent messages.
func hasMinSampleSize(variants []domain.Variant) bool {
	for _, v := range variants {
		if v.SentCount < minSamplePerVariant {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/campaign/application/ -v -run TestSelect -count=1
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/campaign/application/winner_selector.go internal/services/campaign/application/winner_selector_test.go
git commit -m "feat(campaign): add automatic winner selection for test-then-send A/B"
```

---

### Task 5: Segment domain models

**Files:**
- Create: `internal/services/contact/domain/segment.go` (or extend if it already exists)

- [ ] **Step 1: Write segment models**

Check if `internal/services/contact/domain/segment.go` already exists. If yes, extend it. If not, create:

```go
// internal/services/contact/domain/segment.go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSegmentNotFound = errors.New("segment not found")
)

// SavedSegment represents a cross-list saved segment with filter rules.
type SavedSegment struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	Name           string
	Description    string
	ContactListIDs []uuid.UUID
	Rules          SegmentRules
	TagRules       *TagRules
	EstimatedCount int32
	EstimatedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SegmentRules represents the top-level rule group with nested conditions.
type SegmentRules struct {
	Operator   string           `json:"operator"` // "AND" | "OR"
	Conditions []SegmentRuleNode `json:"conditions"`
}

// SegmentRuleNode is either a leaf condition or a nested group.
type SegmentRuleNode struct {
	// Leaf condition fields
	Field string      `json:"field,omitempty"`
	Op    string      `json:"op,omitempty"`
	Value interface{} `json:"value,omitempty"`

	// Nested group fields
	Operator   string           `json:"operator,omitempty"`
	Conditions []SegmentRuleNode `json:"conditions,omitempty"`
}

// TagRules for tag-based filtering.
type TagRules struct {
	Op    string   `json:"op"`    // "contains_any" | "contains_all"
	Tags  []string `json:"tags"`
}

// IsGroup returns true if this node is a nested group (has Operator and Conditions).
func (n SegmentRuleNode) IsGroup() bool {
	return n.Operator != "" && len(n.Conditions) > 0
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/contact/domain/segment.go
git commit -m "feat(contact): add SavedSegment domain models with nested rules"
```

---

### Task 6: Segment repository

**Files:**
- Create: `internal/services/contact/infrastructure/repository/segment_repo.go`

- [ ] **Step 1: Write the repository**

```go
// internal/services/contact/infrastructure/repository/segment_repo.go
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type SegmentRepository struct {
	pool *pgxpool.Pool
}

func NewSegmentRepository(pool *pgxpool.Pool) *SegmentRepository {
	return &SegmentRepository{pool: pool}
}

func (r *SegmentRepository) Create(ctx context.Context, seg *domain.SavedSegment) error {
	seg.ID = uuid.New()
	rulesJSON, err := json.Marshal(seg.Rules)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}
	var tagJSON []byte
	if seg.TagRules != nil {
		tagJSON, _ = json.Marshal(seg.TagRules)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO saved_segments (id, client_id, name, description, contact_list_ids, rules, tag_rules)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		seg.ID, seg.ClientID, seg.Name, seg.Description, seg.ContactListIDs, rulesJSON, tagJSON,
	)
	return err
}

func (r *SegmentRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.SavedSegment, error) {
	var seg domain.SavedSegment
	var rulesJSON, tagJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, client_id, name, description, contact_list_ids, rules, tag_rules, estimated_count, estimated_at, created_at, updated_at
		 FROM saved_segments WHERE id = $1 AND client_id = $2`, id, clientID,
	).Scan(&seg.ID, &seg.ClientID, &seg.Name, &seg.Description, &seg.ContactListIDs,
		&rulesJSON, &tagJSON, &seg.EstimatedCount, &seg.EstimatedAt, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, domain.ErrSegmentNotFound
	}
	json.Unmarshal(rulesJSON, &seg.Rules)
	if tagJSON != nil {
		seg.TagRules = &domain.TagRules{}
		json.Unmarshal(tagJSON, seg.TagRules)
	}
	return &seg, nil
}

func (r *SegmentRepository) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.SavedSegment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, contact_list_ids, estimated_count, estimated_at, created_at
		 FROM saved_segments WHERE client_id = $1 ORDER BY created_at DESC`, clientID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []*domain.SavedSegment
	for rows.Next() {
		var seg domain.SavedSegment
		seg.ClientID = clientID
		if err := rows.Scan(&seg.ID, &seg.Name, &seg.Description, &seg.ContactListIDs,
			&seg.EstimatedCount, &seg.EstimatedAt, &seg.CreatedAt); err != nil {
			return nil, err
		}
		segments = append(segments, &seg)
	}
	return segments, nil
}

func (r *SegmentRepository) Update(ctx context.Context, seg *domain.SavedSegment) error {
	rulesJSON, err := json.Marshal(seg.Rules)
	if err != nil {
		return err
	}
	var tagJSON []byte
	if seg.TagRules != nil {
		tagJSON, _ = json.Marshal(seg.TagRules)
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE saved_segments SET name=$3, description=$4, contact_list_ids=$5, rules=$6, tag_rules=$7, updated_at=now()
		 WHERE id=$1 AND client_id=$2`,
		seg.ID, seg.ClientID, seg.Name, seg.Description, seg.ContactListIDs, rulesJSON, tagJSON,
	)
	return err
}

func (r *SegmentRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM saved_segments WHERE id = $1 AND client_id = $2`, id, clientID)
	return err
}

func (r *SegmentRepository) UpdateEstimate(ctx context.Context, id uuid.UUID, count int32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE saved_segments SET estimated_count = $2, estimated_at = now() WHERE id = $1`, id, count,
	)
	return err
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/contact/infrastructure/repository/segment_repo.go
git commit -m "feat(contact): add segment repository CRUD"
```

---

### Task 7: Segment service with SQL rule builder

**Files:**
- Create: `internal/services/contact/application/segment_service.go`
- Create: `internal/services/contact/application/segment_service_test.go`

- [ ] **Step 1: Write failing tests for rule builder**

```go
// internal/services/contact/application/segment_service_test.go
package application

import (
	"testing"

	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWhereClause_SimpleEq(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "eq", Value: "Москва"},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "attributes->>'city'")
	assert.Contains(t, args, "Москва")
}

func TestBuildWhereClause_NestedGroup(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "eq", Value: "Москва"},
			{
				Operator: "OR",
				Conditions: []domain.SegmentRuleNode{
					{Field: "attributes.age", Op: "gte", Value: float64(25)},
					{Field: "attributes.spending", Op: "gte", Value: float64(10000)},
				},
			},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "AND")
	assert.Contains(t, clause, "OR")
	assert.Len(t, args, 3)
}

func TestBuildWhereClause_InOperator(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "in", Value: []interface{}{"Москва", "Санкт-Петербург"}},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "IN")
	assert.True(t, len(args) >= 2)
}

func TestBuildWhereClause_IsEmpty(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.email", Op: "is_empty"},
		},
	}
	clause, _, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "IS NULL")
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/contact/application/ -v -run TestBuild -count=1
```

Expected: FAIL

- [ ] **Step 3: Write the segment service with SQL builder**

```go
// internal/services/contact/application/segment_service.go
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

type SegmentService struct {
	segmentRepo *repository.SegmentRepository
	pool        *pgxpool.Pool
	logger      zerolog.Logger
}

func NewSegmentService(segmentRepo *repository.SegmentRepository, pool *pgxpool.Pool) *SegmentService {
	return &SegmentService{
		segmentRepo: segmentRepo,
		pool:        pool,
		logger:      log.With().Str("component", "segment-service").Logger(),
	}
}

func (s *SegmentService) Create(ctx context.Context, seg *domain.SavedSegment) error {
	return s.segmentRepo.Create(ctx, seg)
}

func (s *SegmentService) Get(ctx context.Context, id, clientID uuid.UUID) (*domain.SavedSegment, error) {
	return s.segmentRepo.GetByID(ctx, id, clientID)
}

func (s *SegmentService) List(ctx context.Context, clientID uuid.UUID) ([]*domain.SavedSegment, error) {
	return s.segmentRepo.ListByClient(ctx, clientID)
}

func (s *SegmentService) Update(ctx context.Context, seg *domain.SavedSegment) error {
	return s.segmentRepo.Update(ctx, seg)
}

func (s *SegmentService) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	return s.segmentRepo.Delete(ctx, id, clientID)
}

// EstimateCount runs the segment query and returns the count of matching contacts.
func (s *SegmentService) EstimateCount(ctx context.Context, seg *domain.SavedSegment) (int32, error) {
	whereClause, args, err := BuildWhereClause(seg.Rules, 2) // $1 is contact_list_ids
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(
		`SELECT COUNT(DISTINCT phone) FROM contacts WHERE contact_list_id = ANY($1::uuid[]) AND (%s)`,
		whereClause,
	)
	allArgs := append([]interface{}{seg.ContactListIDs}, args...)

	var count int32
	err = s.pool.QueryRow(ctx, query, allArgs...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("estimate count: %w", err)
	}

	s.segmentRepo.UpdateEstimate(ctx, seg.ID, count)
	return count, nil
}

// BuildWhereClause translates segment rules to a parameterized SQL WHERE clause.
// startParam is the next available $N parameter index.
func BuildWhereClause(rules domain.SegmentRules, startParam int) (string, []interface{}, error) {
	if len(rules.Conditions) == 0 {
		return "TRUE", nil, nil
	}
	return buildGroup(rules.Operator, rules.Conditions, startParam)
}

func buildGroup(operator string, conditions []domain.SegmentRuleNode, paramIdx int) (string, []interface{}, error) {
	if operator != "AND" && operator != "OR" {
		operator = "AND"
	}

	var parts []string
	var allArgs []interface{}

	for _, cond := range conditions {
		if cond.IsGroup() {
			clause, args, err := buildGroup(cond.Operator, cond.Conditions, paramIdx)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, "("+clause+")")
			allArgs = append(allArgs, args...)
			paramIdx += len(args)
		} else {
			clause, args, err := buildCondition(cond, paramIdx)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, clause)
			allArgs = append(allArgs, args...)
			paramIdx += len(args)
		}
	}

	return strings.Join(parts, " "+operator+" "), allArgs, nil
}

func buildCondition(cond domain.SegmentRuleNode, paramIdx int) (string, []interface{}, error) {
	field := cond.Field
	// Extract JSONB path: "attributes.city" → "attributes->>'city'"
	attrName := strings.TrimPrefix(field, "attributes.")
	col := fmt.Sprintf("attributes->>'%s'", attrName)

	switch cond.Op {
	case "eq":
		return fmt.Sprintf("%s = $%d", col, paramIdx), []interface{}{fmt.Sprintf("%v", cond.Value)}, nil
	case "neq":
		return fmt.Sprintf("(%s IS NULL OR %s != $%d)", col, col, paramIdx), []interface{}{fmt.Sprintf("%v", cond.Value)}, nil
	case "gt":
		return fmt.Sprintf("(%s)::numeric > $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "gte":
		return fmt.Sprintf("(%s)::numeric >= $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "lt":
		return fmt.Sprintf("(%s)::numeric < $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "lte":
		return fmt.Sprintf("(%s)::numeric <= $%d", col, paramIdx), []interface{}{cond.Value}, nil
	case "contains":
		return fmt.Sprintf("%s ILIKE $%d", col, paramIdx), []interface{}{fmt.Sprintf("%%%v%%", cond.Value)}, nil
	case "not_contains":
		return fmt.Sprintf("(%s IS NULL OR %s NOT ILIKE $%d)", col, col, paramIdx), []interface{}{fmt.Sprintf("%%%v%%", cond.Value)}, nil
	case "in":
		values, ok := cond.Value.([]interface{})
		if !ok {
			return "", nil, fmt.Errorf("in operator requires array value")
		}
		placeholders := make([]string, len(values))
		args := make([]interface{}, len(values))
		for i, v := range values {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx+i)
			args[i] = fmt.Sprintf("%v", v)
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")), args, nil
	case "is_empty":
		return fmt.Sprintf("(%s IS NULL OR %s = '')", col, col), nil, nil
	case "is_not_empty":
		return fmt.Sprintf("(%s IS NOT NULL AND %s != '')", col, col), nil, nil
	case "older_than_days":
		days := fmt.Sprintf("%v", cond.Value)
		return fmt.Sprintf("(%s)::date < (CURRENT_DATE - INTERVAL '%s days')", col, days), nil, nil
	case "newer_than_days":
		days := fmt.Sprintf("%v", cond.Value)
		return fmt.Sprintf("(%s)::date > (CURRENT_DATE - INTERVAL '%s days')", col, days), nil, nil
	default:
		return "", nil, fmt.Errorf("unknown operator: %s", cond.Op)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/magomed/projects/sms && go test ./internal/services/contact/application/ -v -run TestBuild -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/contact/application/segment_service.go internal/services/contact/application/segment_service_test.go
git commit -m "feat(contact): add segment service with SQL rule builder"
```

---

### Task 8: Segment HTTP handlers

**Files:**
- Create: `internal/gateway/portal/handlers/segments.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Step 1: Create segment handlers**

```go
// internal/gateway/portal/handlers/segments.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type SegmentHandlers struct {
	service *application.SegmentService
}

func NewSegmentHandlers(pool *pgxpool.Pool) *SegmentHandlers {
	repo := repository.NewSegmentRepository(pool)
	svc := application.NewSegmentService(repo, pool)
	return &SegmentHandlers{service: svc}
}

func (h *SegmentHandlers) CreateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	var req struct {
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		ContactListIDs []string            `json:"contact_list_ids"`
		Rules          domain.SegmentRules `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	listIDs := make([]uuid.UUID, len(req.ContactListIDs))
	for i, s := range req.ContactListIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный contact_list_id"))
			return
		}
		listIDs[i] = id
	}

	seg := &domain.SavedSegment{
		ClientID:       clientID,
		Name:           req.Name,
		Description:    req.Description,
		ContactListIDs: listIDs,
		Rules:          req.Rules,
	}
	if err := h.service.Create(r.Context(), seg); err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusCreated, seg)
}

func (h *SegmentHandlers) ListSegments(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	segments, err := h.service.List(r.Context(), clientID)
	if err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"segments": segments})
}

func (h *SegmentHandlers) GetSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID"))
		return
	}
	seg, err := h.service.Get(r.Context(), id, clientID)
	if err != nil {
		respondError(w, shared.ErrNotFound("Сегмент не найден"))
		return
	}
	respondJSON(w, http.StatusOK, seg)
}

func (h *SegmentHandlers) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID"))
		return
	}

	var req struct {
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		ContactListIDs []string            `json:"contact_list_ids"`
		Rules          domain.SegmentRules `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат"))
		return
	}

	listIDs := make([]uuid.UUID, len(req.ContactListIDs))
	for i, s := range req.ContactListIDs {
		listIDs[i], _ = uuid.Parse(s)
	}

	seg := &domain.SavedSegment{
		ID: id, ClientID: clientID, Name: req.Name, Description: req.Description,
		ContactListIDs: listIDs, Rules: req.Rules,
	}
	if err := h.service.Update(r.Context(), seg); err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, seg)
}

func (h *SegmentHandlers) DeleteSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	h.service.Delete(r.Context(), id, clientID)
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *SegmentHandlers) EstimateSegment(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не аутентифицирован"))
		return
	}
	id, _ := uuid.Parse(mux.Vars(r)["id"])
	seg, err := h.service.Get(r.Context(), id, clientID)
	if err != nil {
		respondError(w, shared.ErrNotFound("Сегмент не найден"))
		return
	}
	count, err := h.service.EstimateCount(r.Context(), seg)
	if err != nil {
		respondError(w, shared.ErrInternal(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]int32{"estimated_count": count})
}
```

- [ ] **Step 2: Register segment routes in router.go**

```go
// Segments
segments := protected.PathPrefix("/segments").Subrouter()
segments.HandleFunc("", segmentHandlers.CreateSegment).Methods("POST")
segments.HandleFunc("", segmentHandlers.ListSegments).Methods("GET")
segments.HandleFunc("/{id}", segmentHandlers.GetSegment).Methods("GET")
segments.HandleFunc("/{id}", segmentHandlers.UpdateSegment).Methods("PUT")
segments.HandleFunc("/{id}", segmentHandlers.DeleteSegment).Methods("DELETE")
segments.HandleFunc("/{id}/estimate", segmentHandlers.EstimateSegment).Methods("POST")
```

Add `segmentHandlers *handlers.SegmentHandlers` to `SetupRouter` parameters.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/segments.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add segment CRUD and estimate endpoints"
```

---

### Task 9: Frontend — Segments pages and SegmentBuilder

**Files:**
- Create: `portal-frontend/src/pages/segments/SegmentsPage.tsx`
- Create: `portal-frontend/src/pages/segments/SegmentDetailPage.tsx`
- Create: `portal-frontend/src/components/segments/SegmentBuilder.tsx`

- [ ] **Step 1: Create SegmentsPage**

```tsx
// portal-frontend/src/pages/segments/SegmentsPage.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';

interface Segment {
  id: string;
  name: string;
  description: string;
  estimated_count: number;
  contact_list_ids: string[];
  created_at: string;
}

export function SegmentsPage() {
  const [segments, setSegments] = useState<Segment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/portal/v1/segments', { credentials: 'include' })
      .then((r) => r.json())
      .then((data) => { setSegments(data.segments || []); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  return (
    <div>
      <PageHeader title="Сегменты" description="Сохранённые сегменты для кампаний" />
      <div className="mb-4">
        <Link to="/segments/new" className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90">
          Создать сегмент
        </Link>
      </div>
      {loading ? (
        <p className="text-sm text-gray-500">Загрузка...</p>
      ) : segments.length === 0 ? (
        <p className="text-sm text-gray-500">Нет сегментов</p>
      ) : (
        <div className="space-y-2">
          {segments.map((s) => (
            <Link key={s.id} to={`/segments/${s.id}`}
              className="block p-4 bg-white border border-gray-200 rounded hover:border-gray-300">
              <div className="flex justify-between items-center">
                <div>
                  <p className="font-medium text-sm">{s.name}</p>
                  {s.description && <p className="text-xs text-gray-500 mt-0.5">{s.description}</p>}
                </div>
                <span className="text-sm text-gray-600">{s.estimated_count} контактов</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Create SegmentBuilder**

```tsx
// portal-frontend/src/components/segments/SegmentBuilder.tsx
import { useState } from 'react';

interface Condition {
  field: string;
  op: string;
  value: string;
}

interface RuleGroup {
  operator: 'AND' | 'OR';
  conditions: Condition[];
}

interface SegmentBuilderProps {
  rules: RuleGroup;
  onChange: (rules: RuleGroup) => void;
}

const OPERATORS = [
  { value: 'eq', label: '=' },
  { value: 'neq', label: '!=' },
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'contains', label: 'содержит' },
  { value: 'in', label: 'в списке' },
  { value: 'is_empty', label: 'пусто' },
  { value: 'is_not_empty', label: 'не пусто' },
  { value: 'older_than_days', label: 'старше (дней)' },
  { value: 'newer_than_days', label: 'новее (дней)' },
];

export function SegmentBuilder({ rules, onChange }: SegmentBuilderProps) {
  const addCondition = () => {
    onChange({
      ...rules,
      conditions: [...rules.conditions, { field: '', op: 'eq', value: '' }],
    });
  };

  const updateCondition = (index: number, updates: Partial<Condition>) => {
    const newConditions = [...rules.conditions];
    newConditions[index] = { ...newConditions[index], ...updates };
    onChange({ ...rules, conditions: newConditions });
  };

  const removeCondition = (index: number) => {
    onChange({
      ...rules,
      conditions: rules.conditions.filter((_, i) => i !== index),
    });
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-sm text-gray-600">Совпадение:</span>
        <select
          value={rules.operator}
          onChange={(e) => onChange({ ...rules, operator: e.target.value as 'AND' | 'OR' })}
          className="px-2 py-1 border border-gray-300 rounded text-sm"
        >
          <option value="AND">Все условия (AND)</option>
          <option value="OR">Любое условие (OR)</option>
        </select>
      </div>

      {rules.conditions.map((cond, i) => (
        <div key={i} className="flex items-center gap-2 p-2 bg-gray-50 rounded">
          <input
            value={cond.field}
            onChange={(e) => updateCondition(i, { field: e.target.value })}
            placeholder="attributes.city"
            className="flex-1 px-2 py-1 border border-gray-300 rounded text-sm"
          />
          <select
            value={cond.op}
            onChange={(e) => updateCondition(i, { op: e.target.value })}
            className="px-2 py-1 border border-gray-300 rounded text-sm"
          >
            {OPERATORS.map((op) => (
              <option key={op.value} value={op.value}>{op.label}</option>
            ))}
          </select>
          {!['is_empty', 'is_not_empty'].includes(cond.op) && (
            <input
              value={cond.value}
              onChange={(e) => updateCondition(i, { value: e.target.value })}
              placeholder="значение"
              className="flex-1 px-2 py-1 border border-gray-300 rounded text-sm"
            />
          )}
          <button onClick={() => removeCondition(i)} className="text-red-500 hover:text-red-700 text-sm px-1">
            ✕
          </button>
        </div>
      ))}

      <button onClick={addCondition} className="text-sm text-primary hover:underline">
        + Добавить условие
      </button>
    </div>
  );
}
```

- [ ] **Step 3: Create SegmentDetailPage**

```tsx
// portal-frontend/src/pages/segments/SegmentDetailPage.tsx
import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';
import { SegmentBuilder } from '../../components/segments/SegmentBuilder';

export function SegmentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isNew = id === 'new';

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [contactListIds, setContactListIds] = useState('');
  const [rules, setRules] = useState<{ operator: 'AND' | 'OR'; conditions: any[] }>({
    operator: 'AND', conditions: [],
  });
  const [estimatedCount, setEstimatedCount] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!isNew && id) {
      fetch(`/portal/v1/segments/${id}`, { credentials: 'include' })
        .then((r) => r.json())
        .then((data) => {
          setName(data.name || '');
          setDescription(data.description || '');
          setContactListIds((data.contact_list_ids || []).join(', '));
          if (data.rules) setRules(data.rules);
          setEstimatedCount(data.estimated_count);
        });
    }
  }, [id, isNew]);

  const save = async () => {
    setSaving(true);
    const body = {
      name,
      description,
      contact_list_ids: contactListIds.split(',').map((s: string) => s.trim()).filter(Boolean),
      rules,
    };
    const url = isNew ? '/portal/v1/segments' : `/portal/v1/segments/${id}`;
    const method = isNew ? 'POST' : 'PUT';
    const res = await fetch(url, {
      method, headers: { 'Content-Type': 'application/json' },
      credentials: 'include', body: JSON.stringify(body),
    });
    setSaving(false);
    if (res.ok) {
      const data = await res.json();
      if (isNew) navigate(`/segments/${data.id}`);
    }
  };

  const estimate = async () => {
    if (!id || isNew) return;
    const res = await fetch(`/portal/v1/segments/${id}/estimate`, {
      method: 'POST', credentials: 'include',
    });
    if (res.ok) {
      const data = await res.json();
      setEstimatedCount(data.estimated_count);
    }
  };

  return (
    <div>
      <PageHeader title={isNew ? 'Новый сегмент' : name} />
      <div className="max-w-2xl space-y-4">
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Название"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />
        <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Описание"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />
        <input value={contactListIds} onChange={(e) => setContactListIds(e.target.value)}
          placeholder="ID списков контактов (через запятую)"
          className="w-full px-3 py-2 border border-gray-300 rounded text-sm" />

        <div className="p-4 border border-gray-200 rounded">
          <h3 className="text-sm font-semibold mb-3">Правила фильтрации</h3>
          <SegmentBuilder rules={rules} onChange={setRules} />
        </div>

        {estimatedCount !== null && (
          <p className="text-sm text-gray-600">Оценка: {estimatedCount} контактов</p>
        )}

        <div className="flex gap-2">
          <button onClick={save} disabled={saving}
            className="px-4 py-2 bg-primary text-white rounded text-sm hover:bg-primary/90 disabled:opacity-50">
            {saving ? 'Сохранение...' : 'Сохранить'}
          </button>
          {!isNew && (
            <button onClick={estimate}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded text-sm hover:bg-gray-200">
              Пересчитать
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/segments/ portal-frontend/src/components/segments/
git commit -m "feat(frontend): add Segments pages and SegmentBuilder component"
```

---

### Task 10: Frontend — ABStatusPanel component

**Files:**
- Create: `portal-frontend/src/components/campaigns/ABStatusPanel.tsx`

- [ ] **Step 1: Create the component**

```tsx
// portal-frontend/src/components/campaigns/ABStatusPanel.tsx
import { useState, useEffect } from 'react';

interface ABStatus {
  test_phase: string;
  strategy: string;
  test_percentage: number;
  winning_metric: string;
  auto_select_winner: boolean;
  time_remaining_seconds: number;
  variants: {
    id: string;
    name: string;
    sent_count: number;
    delivered_count: number;
    failed_count: number;
    click_count: number;
    delivery_rate: number;
    click_rate: number;
    is_winner: boolean;
  }[];
}

interface ABStatusPanelProps {
  campaignId: string;
}

export function ABStatusPanel({ campaignId }: ABStatusPanelProps) {
  const [status, setStatus] = useState<ABStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStatus = async () => {
    const res = await fetch(`/portal/v1/campaigns/${campaignId}/ab-status`, { credentials: 'include' });
    if (res.ok) setStatus(await res.json());
    setLoading(false);
  };

  useEffect(() => { fetchStatus(); const t = setInterval(fetchStatus, 30000); return () => clearInterval(t); }, [campaignId]);

  const selectWinner = async (variantId: string) => {
    await fetch(`/portal/v1/campaigns/${campaignId}/select-winner`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ variant_id: variantId }),
    });
    fetchStatus();
  };

  if (loading || !status) return null;
  if (status.strategy !== 'test_then_send') return null;

  const phaseLabels: Record<string, string> = {
    none: 'Не запущен',
    testing: 'Тестирование',
    waiting_winner: 'Ожидание выбора победителя',
    rollout: 'Рассылка остальным',
    completed: 'Завершён',
  };

  const phaseColors: Record<string, string> = {
    testing: 'text-blue-600 bg-blue-50',
    waiting_winner: 'text-amber-600 bg-amber-50',
    rollout: 'text-green-600 bg-green-50',
    completed: 'text-gray-600 bg-gray-50',
  };

  return (
    <div className="p-4 border border-gray-200 rounded space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">A/B Тест: Test-Then-Send</h3>
        <span className={`text-xs px-2 py-0.5 rounded ${phaseColors[status.test_phase] || 'text-gray-500'}`}>
          {phaseLabels[status.test_phase] || status.test_phase}
        </span>
      </div>

      <p className="text-xs text-gray-500">
        Тест: {status.test_percentage}% аудитории / Метрика: {status.winning_metric}
      </p>

      {status.time_remaining_seconds > 0 && (
        <p className="text-xs text-gray-500">
          Осталось: {Math.ceil(status.time_remaining_seconds / 60)} мин.
        </p>
      )}

      <div className="space-y-2">
        {status.variants?.map((v) => (
          <div key={v.id} className={`p-3 rounded border ${v.is_winner ? 'border-green-300 bg-green-50' : 'border-gray-200'}`}>
            <div className="flex justify-between items-center">
              <span className="text-sm font-medium">{v.name} {v.is_winner && '(Победитель)'}</span>
              {status.test_phase === 'waiting_winner' && !v.is_winner && (
                <button onClick={() => selectWinner(v.id)}
                  className="text-xs px-2 py-1 bg-primary text-white rounded hover:bg-primary/90">
                  Выбрать
                </button>
              )}
            </div>
            <div className="mt-1 flex gap-4 text-xs text-gray-600">
              <span>Отправлено: {v.sent_count}</span>
              <span>Доставлено: {(v.delivery_rate * 100).toFixed(1)}%</span>
              <span>Клики: {v.click_count}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/campaigns/ABStatusPanel.tsx
git commit -m "feat(frontend): add ABStatusPanel for test-then-send campaigns"
```

---

### Task 11: Update App.tsx and sidebar for Segments

**Files:**
- Modify: `portal-frontend/src/App.tsx`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add routes to App.tsx**

Add imports:

```tsx
import { SegmentsPage } from './pages/segments/SegmentsPage';
import { SegmentDetailPage } from './pages/segments/SegmentDetailPage';
```

Add routes inside `<Route element={<RequireAuth />}>`:

```tsx
<Route path="/segments" element={<SegmentsPage />} />
<Route path="/segments/new" element={<SegmentDetailPage />} />
<Route path="/segments/:id" element={<SegmentDetailPage />} />
```

- [ ] **Step 2: Add Segments to sidebar navigation**

In `portal-frontend/src/components/layout/UserLayout.tsx`, add to `USER_NAV` array between Контакты and Рассылки:

```tsx
{ path: '/segments', label: 'Сегменты' },
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(frontend): add Segments navigation and routes"
```
