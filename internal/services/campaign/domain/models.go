package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors
var (
	ErrCampaignNotFound      = errors.New("campaign not found")
	ErrVariantNotFound       = errors.New("variant not found")
	ErrInvalidCampaignStatus = errors.New("invalid campaign status")
	ErrCampaignNotDraft      = errors.New("campaign is not in draft status")
	ErrCampaignNotRunning    = errors.New("campaign is not running")
	ErrCampaignNotPaused     = errors.New("campaign is not paused")
	ErrVariantPercentageSum  = errors.New("variant percentages must sum to 100")
	ErrTooFewVariants        = errors.New("at least 2 variants required for A/B testing")
	ErrTooManyVariants       = errors.New("at most 5 variants allowed for A/B testing")
	ErrWinnerAlreadySelected = errors.New("winner has already been selected")
	ErrNoFailedRecipients    = errors.New("no failed recipients to retry")
)

// Campaign status constants
const (
	StatusDraft         = "draft"
	StatusScheduled     = "scheduled"
	StatusMaterializing = "materializing"
	StatusRunning       = "running"
	StatusPaused        = "paused"
	StatusCompleted     = "completed"
	StatusCancelled     = "cancelled"
)

// Recipient status constants
const (
	RecipientPending   = "pending"
	RecipientSent      = "sent"
	RecipientDelivered = "delivered"
	RecipientFailed    = "failed"
	RecipientRetry     = "retry"
	RecipientCancelled = "cancelled"
)

// Campaign represents a broadcast campaign
type Campaign struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Status          string
	ContactListID   uuid.UUID
	TemplateID      *uuid.UUID
	Source          string
	SegmentRules    string
	SegmentTags     []string
	SendRate        int32
	ScheduledAt     *time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	RetryConfig     *RetryConfig
	TotalRecipients int32
	SentCount       int32
	DeliveredCount  int32
	FailedCount     int32
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Loaded relations (not always populated)
	Variants []Variant
	ABConfig *ABConfig
}

// Variant represents an A/B test variant for a campaign
type Variant struct {
	ID             uuid.UUID
	CampaignID     uuid.UUID
	Name           string
	TemplateID     *uuid.UUID
	Percentage     int32
	IsWinner       bool
	IsControl      bool
	SentCount        int32
	DeliveredCount   int32
	FailedCount      int32
	ClickCount       int32
	UniqueClickCount int32
}

// ABConfig represents A/B testing configuration for a campaign
type ABConfig struct {
	CampaignID        uuid.UUID
	Metric            string
	TestDurationHours int32
	AutoSelectWinner  bool
	WinnerVariantID   *uuid.UUID
	WinnerSelectedAt  *time.Time
	// Test-then-send fields
	Strategy         string     // "full_split" | "test_then_send"
	TestPercentage   int32      // 1-100
	TestPhase        string     // "none" | "testing" | "waiting_winner" | "rollout" | "completed"
	WinningMetric    string     // "delivery_rate" | "click_rate" | "unique_click_rate"
	TestStartedAt    *time.Time
	RolloutStartedAt *time.Time
}

// RetryConfig represents retry configuration stored as JSONB
type RetryConfig struct {
	Enabled               bool   `json:"enabled"`
	DelayHours            int32  `json:"delay_hours"`
	MaxRetries            int32  `json:"max_retries"`
	AlternativeTemplateID string `json:"alternative_template_id,omitempty"`
}

// Recipient represents a campaign recipient
type Recipient struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	ContactID   uuid.UUID
	Phone       string
	VariantID   *uuid.UUID
	Status      string
	MessageID   *uuid.UUID
	RetryCount  int32
	LastRetryAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RetryLogEntry represents an entry in the campaign retry log
type RetryLogEntry struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	RecipientID uuid.UUID
	RetryNumber int32
	ProviderID  *uuid.UUID
	Status      string
	ErrorCode   string
	CreatedAt   time.Time
}

// StatsSnapshot represents a point-in-time stats snapshot
type StatsSnapshot struct {
	CampaignID        uuid.UUID
	VariantID         uuid.UUID
	SnapshotAt        time.Time
	Sent              int32
	Delivered         int32
	Failed            int32
	Pending           int32
	AvgDeliveryTimeMs int32
	Cost              float64
}

// HeatmapCell represents a cell in the delivery heatmap (day_of_week x hour)
type HeatmapCell struct {
	DayOfWeek      int32
	Hour           int32
	DeliveredCount int32
	DeliveryRate   float64
}

// TimelinePoint represents a single point in the campaign timeline
type TimelinePoint struct {
	Timestamp time.Time
	Value     int32
}

// VariantComparison holds comparison data for a single variant
type VariantComparison struct {
	VariantID         uuid.UUID
	VariantName       string
	AudienceSize      int32
	Sent              int32
	Delivered         int32
	Failed            int32
	DeliveryRate      float64
	AvgDeliveryTimeMs int32
	Cost              float64
}

// TimeSlot represents a recommended send time slot
type TimeSlot struct {
	DayOfWeek         int32
	Hour              int32
	DeliveryRate      float64
	AvgDeliveryTimeMs int32
	Score             float64
}

// Test phase constants
const (
	TestPhaseNone          = "none"
	TestPhaseTesting       = "testing"
	TestPhaseWaitingWinner = "waiting_winner"
	TestPhaseRollout       = "rollout"
	TestPhaseCompleted     = "completed"
)

// Strategy constants
const (
	StrategyFullSplit    = "full_split"
	StrategyTestThenSend = "test_then_send"
)
