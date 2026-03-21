package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
	"github.com/smpp-server/smpp-server/internal/services/analytics/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReportService(t *testing.T) {
	t.Run("GenerateReport", func(t *testing.T) {
		t.Run("creates_json_report", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
			clientID := uuid.New()

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      200,
					TotalDelivered: 180,
					TotalFailed:    20,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.MatchedBy(func(f *domain.StatisticsFilters) bool {
				return f.From == from && f.To == to && f.ClientID != nil && *f.ClientID == clientID
			})).Return(expectedStats, nil)

			reportRepo.On("Create", ctx, mock.MatchedBy(func(r *domain.Report) bool {
				return r.Type == domain.ReportTypeMonthly &&
					r.Format == domain.ReportFormatJSON &&
					r.PeriodStart == from &&
					r.PeriodEnd == to &&
					r.ClientID != nil && *r.ClientID == clientID &&
					len(r.Data) > 0
			})).Return(nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeMonthly, domain.ReportFormatJSON, from, to, &clientID, nil)

			require.NoError(t, err)
			assert.NotNil(t, report)
			assert.Equal(t, domain.ReportTypeMonthly, report.Type)
			assert.Equal(t, domain.ReportFormatJSON, report.Format)
			assert.NotEmpty(t, report.Data)
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})

		t.Run("creates_csv_report", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      50,
					TotalDelivered: 45,
					TotalFailed:    5,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(expectedStats, nil)
			reportRepo.On("Create", ctx, mock.AnythingOfType("*domain.Report")).Return(nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeDaily, domain.ReportFormatCSV, from, to, nil, nil)

			require.NoError(t, err)
			assert.NotNil(t, report)
			assert.Equal(t, domain.ReportFormatCSV, report.Format)
			assert.Contains(t, string(report.Data), "Period Start")
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})

		t.Run("with_provider_filter", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
			providerID := uuid.New()

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      100,
					TotalDelivered: 95,
					TotalFailed:    5,
					SuccessRate:    95,
				},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.MatchedBy(func(f *domain.StatisticsFilters) bool {
				return len(f.ProviderIDs) == 1 && f.ProviderIDs[0] == providerID
			})).Return(expectedStats, nil)

			reportRepo.On("Create", ctx, mock.AnythingOfType("*domain.Report")).Return(nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeProvider, domain.ReportFormatJSON, from, to, nil, &providerID)

			require.NoError(t, err)
			assert.NotNil(t, report)
			assert.Equal(t, domain.ReportTypeProvider, report.Type)
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})

		t.Run("with_PDF_format_falls_back_to_JSON", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      100,
					TotalDelivered: 90,
					TotalFailed:    10,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(expectedStats, nil)
			reportRepo.On("Create", ctx, mock.MatchedBy(func(r *domain.Report) bool {
				return r.Format == domain.ReportFormatPDF && len(r.Data) > 0
			})).Return(nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeMonthly, domain.ReportFormatPDF, from, to, nil, nil)

			require.NoError(t, err)
			assert.NotNil(t, report)
			assert.Equal(t, domain.ReportFormatPDF, report.Format)
			// PDF falls back to JSON data, so data should be valid JSON
			assert.NotEmpty(t, report.Data)
			assert.Contains(t, string(report.Data), "statistics")
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})

		t.Run("with_unsupported_format_returns_error", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      100,
					TotalDelivered: 90,
					TotalFailed:    10,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(expectedStats, nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeDaily, domain.ReportFormat("xml"), from, to, nil, nil)

			require.Error(t, err)
			assert.Nil(t, report)
			assert.Contains(t, err.Error(), "unsupported report format")
			metricRepo.AssertExpectations(t)
		})

		t.Run("csv_report_with_multiple_groups", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{
					TotalSent:      300,
					TotalDelivered: 270,
					TotalFailed:    30,
					SuccessRate:    90,
				},
				Groups: []*domain.StatisticGroup{
					{
						Key: "2024-01-01",
						Stats: &domain.TotalStats{
							TotalSent:      100,
							TotalDelivered: 90,
							TotalFailed:    10,
						},
					},
					{
						Key: "2024-01-02",
						Stats: &domain.TotalStats{
							TotalSent:      100,
							TotalDelivered: 95,
							TotalFailed:    5,
						},
					},
					{
						Key: "2024-01-03",
						Stats: &domain.TotalStats{
							TotalSent:      100,
							TotalDelivered: 85,
							TotalFailed:    15,
						},
					},
				},
			}

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(expectedStats, nil)
			reportRepo.On("Create", ctx, mock.AnythingOfType("*domain.Report")).Return(nil)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeDaily, domain.ReportFormatCSV, from, to, nil, nil)

			require.NoError(t, err)
			assert.NotNil(t, report)
			csvData := string(report.Data)
			assert.Contains(t, csvData, "Period Start")
			assert.Contains(t, csvData, "Group,Total Sent,Total Delivered,Total Failed")
			assert.Contains(t, csvData, "2024-01-01")
			assert.Contains(t, csvData, "2024-01-02")
			assert.Contains(t, csvData, "2024-01-03")
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_statistics_failure", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(nil, assert.AnError)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeDaily, domain.ReportFormatJSON, from, to, nil, nil)

			require.Error(t, err)
			assert.Nil(t, report)
			assert.Contains(t, err.Error(), "failed to get statistics")
			metricRepo.AssertExpectations(t)
		})

		t.Run("returns_error_on_save_failure", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

			expectedStats := &domain.Statistics{
				Totals: &domain.TotalStats{TotalSent: 10},
				Groups: []*domain.StatisticGroup{},
			}

			metricRepo.On("GetStatistics", ctx, mock.AnythingOfType("*domain.StatisticsFilters")).Return(expectedStats, nil)
			reportRepo.On("Create", ctx, mock.AnythingOfType("*domain.Report")).Return(assert.AnError)

			report, err := svc.GenerateReport(ctx, domain.ReportTypeDaily, domain.ReportFormatJSON, from, to, nil, nil)

			require.Error(t, err)
			assert.Nil(t, report)
			assert.Contains(t, err.Error(), "failed to save report")
			metricRepo.AssertExpectations(t)
			reportRepo.AssertExpectations(t)
		})
	})

	t.Run("ListReports", func(t *testing.T) {
		t.Run("returns_report_list", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			clientID := uuid.New()
			reportType := domain.ReportTypeDaily
			filters := &domain.ReportFilters{
				ReportType: &reportType,
				ClientID:   &clientID,
				Limit:      10,
				Offset:     0,
			}

			expectedReports := []*domain.Report{
				{
					ID:          uuid.New(),
					Type:        domain.ReportTypeDaily,
					Format:      domain.ReportFormatJSON,
					PeriodStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					PeriodEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					ClientID:    &clientID,
					Data:        []byte(`{"test": true}`),
				},
				{
					ID:          uuid.New(),
					Type:        domain.ReportTypeDaily,
					Format:      domain.ReportFormatCSV,
					PeriodStart: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					PeriodEnd:   time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
					ClientID:    &clientID,
					Data:        []byte("csv data"),
				},
			}

			reportRepo.On("List", ctx, filters).Return(expectedReports, nil)

			reports, err := svc.ListReports(ctx, filters)

			require.NoError(t, err)
			assert.Len(t, reports, 2)
			assert.Equal(t, expectedReports, reports)
			assert.Equal(t, domain.ReportTypeDaily, reports[0].Type)
			assert.Equal(t, domain.ReportFormatCSV, reports[1].Format)
			reportRepo.AssertExpectations(t)
		})

		t.Run("repository_error", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			filters := &domain.ReportFilters{
				Limit: 10,
			}

			reportRepo.On("List", ctx, filters).Return(nil, assert.AnError)

			reports, err := svc.ListReports(ctx, filters)

			require.Error(t, err)
			assert.Nil(t, reports)
			reportRepo.AssertExpectations(t)
		})
	})

	t.Run("GetReport", func(t *testing.T) {
		t.Run("returns_report", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			reportID := uuid.New()

			expectedReport := &domain.Report{
				ID:          reportID,
				Type:        domain.ReportTypeDaily,
				Format:      domain.ReportFormatJSON,
				PeriodStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
				Data:        []byte(`{"test": true}`),
			}

			reportRepo.On("GetByID", ctx, reportID).Return(expectedReport, nil)

			report, err := svc.GetReport(ctx, reportID)

			require.NoError(t, err)
			assert.Equal(t, expectedReport, report)
			assert.Equal(t, reportID, report.ID)
			assert.Equal(t, domain.ReportTypeDaily, report.Type)
			reportRepo.AssertExpectations(t)
		})

		t.Run("returns_error_when_not_found", func(t *testing.T) {
			reportRepo := new(mocks.MockReportRepository)
			metricRepo := new(mocks.MockMetricRepository)
			svc := NewReportService(reportRepo, metricRepo)

			ctx := context.Background()
			reportID := uuid.New()

			reportRepo.On("GetByID", ctx, reportID).Return(nil, assert.AnError)

			report, err := svc.GetReport(ctx, reportID)

			require.Error(t, err)
			assert.Nil(t, report)
			reportRepo.AssertExpectations(t)
		})
	})
}
