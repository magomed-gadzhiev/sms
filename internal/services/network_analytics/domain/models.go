package domain

import "time"

// Health status constants
const (
	HealthOK      = "ok"
	HealthWarning = "warning"
	HealthDanger  = "danger"
)

// Health threshold constants
const (
	DLRRateWarning     = 0.90
	DLRRateDanger      = 0.80
	LatencyP95WarnMs   = 10000
	LatencyP95DangerMs = 30000
	ErrorRateWarning   = 0.05
	ErrorRateDanger    = 0.15
	PendingCountWarn   = 100
	PendingCountDanger = 500
	TimeoutRateWarning = 0.03
	TimeoutRateDanger  = 0.10
)

// Allowed groupings and their max period in hours
var GroupByMaxPeriodHours = map[string]int{
	"5min":  24,
	"15min": 24 * 7,
	"hour":  24 * 30,
	"day":   24 * 366,
}

// SharedFilter is the unified filter for all three modes.
type SharedFilter struct {
	PartnerID    int64
	PeriodPreset string
	DateFrom     time.Time
	DateTo       time.Time
	GroupBy      string

	// Quick filters
	Login       string
	ServiceType string
	Operator    string
	Channel     string

	// Additional filters
	SenderName   string
	SenderPaid   string
	International bool
	TrafficType  string
	Status       string
	PriceRange   string
	Method       string
	Provider     string
	Country      string
	Manager      string
	ErrorCode    string

	// Pagination & sorting
	Page     int
	PageSize int
	SortBy   string
	SortDir  string // "asc" or "desc"
}

// DefaultPageSize is used when PageSize is not specified.
const DefaultPageSize = 25
const MaxPageSize = 100

// Normalize sets defaults and clamps values.
func (f *SharedFilter) Normalize() {
	if f.PageSize <= 0 {
		f.PageSize = DefaultPageSize
	}
	if f.PageSize > MaxPageSize {
		f.PageSize = MaxPageSize
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.SortDir != "asc" {
		f.SortDir = "desc"
	}
}

// KPI represents a single KPI card value.
type KPI struct {
	Name   string
	Value  float64
	Delta  float64 // comparison with previous period
	Status string  // ok | warning | danger
}

// StatRow represents one row in the statistics/analytics table.
type StatRow struct {
	Slice     string
	Total     int64
	Sent      int64
	Delivered int64
	Failed    int64
	Pending   int64
	Timeout   int64
	Error     int64
	DLRRate   float64
	Revenue   float64
	Cost      float64
	Profit    float64
	Margin    float64
	Health    string
	Alerts    map[string]string
}

// MonitorRow represents one row in the monitoring table.
type MonitorRow struct {
	Slice          string
	Throughput     float64
	Sent           int64
	Delivered      int64
	Pending        int64
	Timeout        int64
	Error          int64
	DLRLatencyP50  float64
	DLRLatencyP95  float64
	DLRRate        float64
	TopError       string
	Health         string
}

// Trend represents a time-series for one metric.
type Trend struct {
	Metric string
	Points []MetricPoint
}

// MetricPoint is a single data point in a trend.
type MetricPoint struct {
	Timestamp int64
	Value     float64
}

// Signal is a short actionable insight shown in analytics mode.
type Signal struct {
	Severity  string // danger | warning | success
	Text      string
	LinkType  string
	LinkValue string
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page       int
	PageSize   int
	TotalRows  int
	TotalPages int
}

// StatisticsResult is the response for GetStatistics.
type StatisticsResult struct {
	KPIs       []KPI
	Rows       []StatRow
	Pagination Pagination
}

// AnalyticsResult is the response for GetAnalyticsSummary.
type AnalyticsResult struct {
	KPIs         []KPI
	PreviousKPIs []KPI
	Trends       []Trend
	Signals      []Signal
	Rows         []StatRow
	Pagination   Pagination
}

// MonitoringResult is the response for GetMonitoringMetrics.
type MonitoringResult struct {
	KPIs       []KPI
	Rows       []MonitorRow
	Chart      []MetricPoint
	Pagination Pagination
}

// DrillDownParams specifies what to drill down into.
type DrillDownParams struct {
	PartnerID   int64
	Filter      *SharedFilter
	SliceType   string // provider | operator | channel | login | country
	SliceValue  string
	DetailView  string // operators | statuses | errors | timeline | money
	ParentType  string // for nested drill-down
	ParentValue string
}

// DrillDownResult is the response for GetDrillDown.
type DrillDownResult struct {
	Summary []KPI
	Rows    []StatRow
	Trends  []Trend
	Health  string
}

// ExportJob tracks an async export operation.
type ExportJob struct {
	ID          string
	PartnerID   int64
	UserID      int64
	Mode        string
	Filters     string // JSON
	Format      string // csv | xlsx
	Status      string // pending | processing | done | failed
	FilePath    string
	RowCount    int
	Error       string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// SavedView stores a user's saved filter/column configuration.
type SavedView struct {
	ID         int64
	PartnerID  int64
	UserID     *int64 // nil = global template
	Name       string
	IsDefault  bool
	Mode       string
	Filters    string // JSON
	GroupBy    string
	SortBy     string
	SortDir    string
	Columns    []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// HourlyStatsRow is the raw aggregated data for upsert into network_stats_hourly.
type HourlyStatsRow struct {
	PartnerID     int64
	Hour          time.Time
	ProviderID    int64
	Operator      string
	Country       string
	Channel       string
	Login         string
	SenderName    string
	TrafficType   string
	Method        string
	Total         int
	Sent          int
	Delivered     int
	Failed        int
	Pending       int
	Timeout       int
	Error         int
	Revenue       float64
	Cost          float64
	DLRLatencySum int64
	DLRLatencyCnt int
	DLRLatencyP50 int
	DLRLatencyP95 int
	ThroughputMax float64
}

// MonitoringSnapshot is a point-in-time capture of provider metrics.
type MonitoringSnapshot struct {
	PartnerID     int64
	ProviderID    int64
	Timestamp     time.Time
	Throughput    float64
	QueueDepth    int
	ActiveConns   int
	ErrorCount    int
	TimeoutCount  int
	PendingCount  int
	DLRLatencyP50 int
	DLRLatencyP95 int
	HealthStatus  string
}

// ProviderLiveMetrics is the current state from Redis.
type ProviderLiveMetrics struct {
	ProviderID    int64
	ProviderName  string
	Throughput    float64
	QueueDepth    int
	Pending       int
	ErrorCount    int
	TimeoutCount  int
	LatencyP50    int
	LatencyP95    int
	HealthStatus  string
	ActiveConns   int
}

// ComputeHealth derives health status from metrics.
func ComputeHealth(dlrRate float64, latencyP95Ms int, errorRate float64, pendingCount int) string {
	if dlrRate < DLRRateDanger || latencyP95Ms > LatencyP95DangerMs || errorRate > ErrorRateDanger || pendingCount > PendingCountDanger {
		return HealthDanger
	}
	if dlrRate < DLRRateWarning || latencyP95Ms > LatencyP95WarnMs || errorRate > ErrorRateWarning || pendingCount > PendingCountWarn {
		return HealthWarning
	}
	return HealthOK
}
