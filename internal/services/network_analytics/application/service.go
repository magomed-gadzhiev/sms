package application

import (
	"context"
	"fmt"
	"time"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// NetworkAnalyticsService is the application-level service for network analytics.
type NetworkAnalyticsService struct {
	statsRepo      domain.StatsRepository
	monitoringRepo domain.MonitoringRepository
	exportRepo     domain.ExportRepository
	viewsRepo      domain.ViewsRepository
}

// NewNetworkAnalyticsService creates a new NetworkAnalyticsService.
func NewNetworkAnalyticsService(
	statsRepo domain.StatsRepository,
	monitoringRepo domain.MonitoringRepository,
	exportRepo domain.ExportRepository,
	viewsRepo domain.ViewsRepository,
) *NetworkAnalyticsService {
	return &NetworkAnalyticsService{
		statsRepo:      statsRepo,
		monitoringRepo: monitoringRepo,
		exportRepo:     exportRepo,
		viewsRepo:      viewsRepo,
	}
}

// validateFilter checks that the requested period is within the allowed maximum for the chosen grouping.
// D-12 fix: returns *domain.ValidationError for user-input violations so the gRPC layer
// maps them to codes.InvalidArgument (HTTP 400), not codes.Internal (HTTP 500).
func validateFilter(filter *domain.SharedFilter) error {
	if filter.GroupBy == "" {
		return nil
	}
	// Dimensional (non-time) groupings have no period limit
	if domain.GroupByDimensional[filter.GroupBy] {
		return nil
	}
	maxHours, ok := domain.GroupByMaxPeriodHours[filter.GroupBy]
	if !ok {
		return domain.NewValidationError(fmt.Sprintf(
			"Недопустимое значение group_by=%q. Разрешены: 5min, 15min, hour, day, month, year, provider, operator, channel, login, country",
			filter.GroupBy,
		))
	}
	periodHours := filter.DateTo.Sub(filter.DateFrom).Hours()
	if periodHours > float64(maxHours) {
		return domain.NewValidationError(fmt.Sprintf(
			"Группировка %q допустима только для периода до %d часов (запрошено: %.0f часов). Выберите меньший период или укажите другую группировку.",
			filter.GroupBy, maxHours, periodHours,
		))
	}
	return nil
}

// buildStatKPIs computes KPI cards from the aggregate of all returned rows.
func buildStatKPIs(rows []domain.StatRow) []domain.KPI {
	var totalMsgs, delivered, failed, pending, timeout int64
	var revenue, cost float64

	for _, r := range rows {
		totalMsgs += r.Total
		delivered += r.Delivered
		failed += r.Failed
		pending += r.Pending
		timeout += r.Timeout
		revenue += r.Revenue
		cost += r.Cost
	}

	dlrRate := float64(0)
	if totalMsgs > 0 {
		dlrRate = float64(delivered) / float64(totalMsgs)
	}
	errorRate := float64(0)
	if totalMsgs > 0 {
		errorRate = float64(failed+timeout) / float64(totalMsgs)
	}
	profit := revenue - cost

	dlrStatus := domain.HealthOK
	if dlrRate < domain.DLRRateDanger {
		dlrStatus = domain.HealthDanger
	} else if dlrRate < domain.DLRRateWarning {
		dlrStatus = domain.HealthWarning
	}

	errStatus := domain.HealthOK
	if errorRate > domain.ErrorRateDanger {
		errStatus = domain.HealthDanger
	} else if errorRate > domain.ErrorRateWarning {
		errStatus = domain.HealthWarning
	}

	return []domain.KPI{
		{Name: "Всего", Value: float64(totalMsgs), Status: domain.HealthOK},
		{Name: "Доставляемость", Value: dlrRate, Status: dlrStatus},
		{Name: "Ошибки", Value: errorRate, Status: errStatus},
		{Name: "Выручка", Value: revenue, Status: domain.HealthOK},
		{Name: "Себестоимость", Value: cost, Status: domain.HealthOK},
		{Name: "Прибыль", Value: profit, Status: domain.HealthOK},
		{Name: "Pending", Value: float64(pending), Status: domain.HealthOK},
	}
}

// buildMonitoringKPIs computes aggregate KPI cards from monitoring rows.
func buildMonitoringKPIs(rows []domain.MonitorRow) []domain.KPI {
	if len(rows) == 0 {
		return []domain.KPI{}
	}

	var totalSent, totalDelivered, totalPending, totalTimeout, totalError int64
	var totalThroughput float64
	dangerCount, warningCount := 0, 0

	for _, r := range rows {
		totalSent += r.Sent
		totalDelivered += r.Delivered
		totalPending += r.Pending
		totalTimeout += r.Timeout
		totalError += r.Error
		totalThroughput += r.Throughput
		if r.Health == domain.HealthDanger {
			dangerCount++
		} else if r.Health == domain.HealthWarning {
			warningCount++
		}
	}

	overallHealth := domain.HealthOK
	if dangerCount > 0 {
		overallHealth = domain.HealthDanger
	} else if warningCount > 0 {
		overallHealth = domain.HealthWarning
	}

	dlrRate := float64(0)
	if totalSent > 0 {
		dlrRate = float64(totalDelivered) / float64(totalSent)
	}

	return []domain.KPI{
		{Name: "throughput", Value: totalThroughput, Status: overallHealth},
		{Name: "dlr_rate", Value: dlrRate, Status: overallHealth},
		{Name: "pending", Value: float64(totalPending), Status: overallHealth},
		{Name: "errors", Value: float64(totalError), Status: overallHealth},
		{Name: "timeouts", Value: float64(totalTimeout), Status: overallHealth},
		{Name: "unhealthy_providers", Value: float64(dangerCount + warningCount), Status: overallHealth},
	}
}

// GetStatistics validates the filter, fetches grouped rows and returns paginated results with KPIs.
func (s *NetworkAnalyticsService) GetStatistics(ctx context.Context, filter *domain.SharedFilter) (*domain.StatisticsResult, error) {
	if err := validateFilter(filter); err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}
	filter.Normalize()

	rows, total, err := s.statsRepo.GetStatistics(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("GetStatistics: %w", err)
	}

	totalPages := 0
	if filter.PageSize > 0 && total > 0 {
		totalPages = (total + filter.PageSize - 1) / filter.PageSize
	}

	return &domain.StatisticsResult{
		KPIs: buildStatKPIs(rows),
		Rows: rows,
		Pagination: domain.Pagination{
			Page:       filter.Page,
			PageSize:   filter.PageSize,
			TotalRows:  total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetAnalyticsSummary validates the filter and returns analytics summary with trends and signals.
func (s *NetworkAnalyticsService) GetAnalyticsSummary(ctx context.Context, filter *domain.SharedFilter) (*domain.AnalyticsResult, error) {
	if err := validateFilter(filter); err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}
	filter.Normalize()

	result, err := s.statsRepo.GetAnalyticsSummary(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("GetAnalyticsSummary: %w", err)
	}
	return result, nil
}

// GetMonitoringMetrics fetches monitoring rows and chart data, then builds monitoring KPIs.
func (s *NetworkAnalyticsService) GetMonitoringMetrics(ctx context.Context, filter *domain.SharedFilter, hideHealthy bool) (*domain.MonitoringResult, error) {
	filter.Normalize()

	rows, err := s.monitoringRepo.GetMonitoringMetrics(ctx, filter, hideHealthy)
	if err != nil {
		return nil, fmt.Errorf("GetMonitoringMetrics: %w", err)
	}

	chart, err := s.monitoringRepo.GetMonitoringChart(ctx, filter, "throughput")
	if err != nil {
		// Chart is best-effort; log but don't fail the whole request.
		chart = []domain.MetricPoint{}
	}

	totalPages := 0
	total := len(rows)
	if filter.PageSize > 0 && total > 0 {
		totalPages = (total + filter.PageSize - 1) / filter.PageSize
	}

	return &domain.MonitoringResult{
		KPIs:  buildMonitoringKPIs(rows),
		Rows:  rows,
		Chart: chart,
		Pagination: domain.Pagination{
			Page:       filter.Page,
			PageSize:   filter.PageSize,
			TotalRows:  total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetDrillDown returns a detailed breakdown for a specific dimension slice.
func (s *NetworkAnalyticsService) GetDrillDown(ctx context.Context, params *domain.DrillDownParams) (*domain.DrillDownResult, error) {
	if params.Filter != nil {
		if err := validateFilter(params.Filter); err != nil {
			return nil, fmt.Errorf("invalid filter: %w", err)
		}
		params.Filter.Normalize()
	}

	result, err := s.statsRepo.GetDrillDown(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("GetDrillDown: %w", err)
	}
	return result, nil
}

// StartExport creates an export job and returns its ID.
// Actual background processing is handled separately by an export worker.
func (s *NetworkAnalyticsService) StartExport(ctx context.Context, job *domain.ExportJob) (string, error) {
	if job.Status == "" {
		job.Status = "pending"
	}
	job.CreatedAt = time.Now().UTC()

	if err := s.exportRepo.CreateJob(ctx, job); err != nil {
		return "", fmt.Errorf("StartExport: %w", err)
	}
	return job.ID, nil
}

// GetExportStatus retrieves the current state of an export job.
func (s *NetworkAnalyticsService) GetExportStatus(ctx context.Context, jobID string) (*domain.ExportJob, error) {
	job, err := s.exportRepo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("GetExportStatus: %w", err)
	}
	return job, nil
}

// ListViews returns saved view configurations visible to the given partner/user combination.
func (s *NetworkAnalyticsService) ListViews(ctx context.Context, partnerID, userID int64) ([]domain.SavedView, error) {
	views, err := s.viewsRepo.List(ctx, partnerID, userID)
	if err != nil {
		return nil, fmt.Errorf("ListViews: %w", err)
	}
	return views, nil
}

// SaveView persists a saved view configuration.
func (s *NetworkAnalyticsService) SaveView(ctx context.Context, view *domain.SavedView) (*domain.SavedView, error) {
	saved, err := s.viewsRepo.Save(ctx, view)
	if err != nil {
		return nil, fmt.Errorf("SaveView: %w", err)
	}
	return saved, nil
}

// DeleteView removes a saved view by ID, scoped to partner/user.
func (s *NetworkAnalyticsService) DeleteView(ctx context.Context, id, partnerID, userID int64) error {
	if err := s.viewsRepo.Delete(ctx, id, partnerID, userID); err != nil {
		return fmt.Errorf("DeleteView: %w", err)
	}
	return nil
}
