package domain

import "context"

// StatsRepository handles pre-aggregated statistics data.
type StatsRepository interface {
	// GetStatistics returns grouped statistics rows with pagination.
	GetStatistics(ctx context.Context, filter *SharedFilter) ([]StatRow, int, error)

	// GetAnalyticsSummary returns statistics with trends and signals for analytics mode.
	GetAnalyticsSummary(ctx context.Context, filter *SharedFilter) (*AnalyticsResult, error)

	// GetDrillDown returns detailed breakdown for a specific slice.
	GetDrillDown(ctx context.Context, params *DrillDownParams) (*DrillDownResult, error)

	// UpsertHourlyStats inserts or updates hourly aggregated rows.
	UpsertHourlyStats(ctx context.Context, rows []HourlyStatsRow) error
}

// MonitoringRepository handles real-time monitoring data.
type MonitoringRepository interface {
	// GetMonitoringMetrics returns current monitoring table data.
	GetMonitoringMetrics(ctx context.Context, filter *SharedFilter, hideHealthy bool) ([]MonitorRow, error)

	// GetMonitoringChart returns time-series data for monitoring charts.
	GetMonitoringChart(ctx context.Context, filter *SharedFilter, metric string) ([]MetricPoint, error)

	// SaveSnapshot persists Redis snapshots to PostgreSQL.
	SaveSnapshot(ctx context.Context, snapshots []MonitoringSnapshot) error

	// GetCurrentMetrics reads live metrics from Redis for all providers.
	GetCurrentMetrics(ctx context.Context, partnerID int64) ([]ProviderLiveMetrics, error)
}

// ExportRepository manages async export jobs.
type ExportRepository interface {
	CreateJob(ctx context.Context, job *ExportJob) error
	GetJob(ctx context.Context, jobID string) (*ExportJob, error)
	UpdateJob(ctx context.Context, job *ExportJob) error
}

// ViewsRepository manages saved view configurations.
type ViewsRepository interface {
	List(ctx context.Context, partnerID, userID int64) ([]SavedView, error)
	Save(ctx context.Context, view *SavedView) (*SavedView, error)
	Delete(ctx context.Context, id, partnerID, userID int64) error
}
