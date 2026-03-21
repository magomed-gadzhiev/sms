package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/internal/services/analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// --- Mocks ---

type mockMetricRepo struct {
	mock.Mock
}

func (m *mockMetricRepo) Create(ctx context.Context, metric *domain.Metric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *mockMetricRepo) CreateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *mockMetricRepo) GetAggregated(ctx context.Context, filters *domain.AggregateFilters) ([]*domain.AggregatedMetric, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AggregatedMetric), args.Error(1)
}

func (m *mockMetricRepo) GetStatistics(ctx context.Context, filters *domain.StatisticsFilters) (*domain.Statistics, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Statistics), args.Error(1)
}

func (m *mockMetricRepo) GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*domain.ProviderPerformance, error) {
	args := m.Called(ctx, providerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderPerformance), args.Error(1)
}

func (m *mockMetricRepo) UpdateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

type mockReportRepo struct {
	mock.Mock
}

func (m *mockReportRepo) Create(ctx context.Context, report *domain.Report) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *mockReportRepo) GetByID(ctx context.Context, reportID uuid.UUID) (*domain.Report, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Report), args.Error(1)
}

func (m *mockReportRepo) List(ctx context.Context, filters *domain.ReportFilters) ([]*domain.Report, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Report), args.Error(1)
}

// --- Helper ---

func newTestAnalyticsServer(metricRepo *mockMetricRepo, reportRepo *mockReportRepo) *Server {
	analyticsService := application.NewAnalyticsService(metricRepo)
	reportService := application.NewReportService(reportRepo, metricRepo)
	realtimeService := application.NewRealtimeService()
	return NewServer(analyticsService, reportService, realtimeService)
}

// --- Tests ---

func TestAnalyticsServer_GetStatistics(t *testing.T) {
	t.Run("returns statistics successfully", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		stats := &domain.Statistics{
			Groups: []*domain.StatisticGroup{
				{
					Key: "2025-01-15",
					Stats: &domain.TotalStats{
						TotalSent:      100,
						TotalDelivered: 90,
						TotalFailed:    10,
						SuccessRate:    90,
					},
				},
			},
			Totals: &domain.TotalStats{
				TotalSent:      100,
				TotalDelivered: 90,
				TotalFailed:    10,
				SuccessRate:    90,
			},
		}

		metricRepo.On("GetStatistics", mock.Anything, mock.AnythingOfType("*domain.StatisticsFilters")).Return(stats, nil)

		now := time.Now()
		resp, err := srv.GetStatistics(context.Background(), &analyticsv1.GetStatisticsRequest{
			From:    timestamppb.New(now.AddDate(0, 0, -30)),
			To:      timestamppb.New(now),
			GroupBy: "day",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Totals)
		assert.Equal(t, int64(100), resp.Totals.TotalSent)
		assert.Equal(t, int64(90), resp.Totals.TotalDelivered)
		assert.Equal(t, int64(10), resp.Totals.TotalFailed)
		assert.Equal(t, int32(90), resp.Totals.SuccessRate)
		assert.Len(t, resp.Groups, 1)
		assert.Equal(t, "2025-01-15", resp.Groups[0].Key)
	})

	t.Run("with client_id filter", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		stats := &domain.Statistics{
			Groups: []*domain.StatisticGroup{},
			Totals: &domain.TotalStats{
				TotalSent: 50,
			},
		}

		metricRepo.On("GetStatistics", mock.Anything, mock.AnythingOfType("*domain.StatisticsFilters")).Return(stats, nil)

		clientID := uuid.New()
		resp, err := srv.GetStatistics(context.Background(), &analyticsv1.GetStatisticsRequest{
			ClientId: clientID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int64(50), resp.Totals.TotalSent)
	})

	t.Run("invalid client_id returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GetStatistics(context.Background(), &analyticsv1.GetStatisticsRequest{
			ClientId: "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("defaults applied when from/to not provided", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		stats := &domain.Statistics{
			Groups: []*domain.StatisticGroup{},
			Totals: &domain.TotalStats{},
		}

		metricRepo.On("GetStatistics", mock.Anything, mock.AnythingOfType("*domain.StatisticsFilters")).Return(stats, nil)

		resp, err := srv.GetStatistics(context.Background(), &analyticsv1.GetStatisticsRequest{})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Totals)
	})
}

func TestAnalyticsServer_GetRealtimeMetrics(t *testing.T) {
	t.Run("returns metrics from realtime service", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GetRealtimeMetrics(context.Background(), &analyticsv1.GetRealtimeMetricsRequest{})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Timestamp)
		// Realtime service returns default empty metrics
		assert.Equal(t, int64(0), resp.MessagesPerSecond)
	})
}

func TestAnalyticsServer_GenerateReport(t *testing.T) {
	t.Run("missing report_type returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GenerateReport(context.Background(), &analyticsv1.GenerateReportRequest{
			ReportType: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("missing from/to timestamps returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GenerateReport(context.Background(), &analyticsv1.GenerateReportRequest{
			ReportType: "daily",
			From:       nil,
			To:         nil,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("valid request generates report", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		now := time.Now()
		stats := &domain.Statistics{
			Groups: []*domain.StatisticGroup{},
			Totals: &domain.TotalStats{TotalSent: 10},
		}

		metricRepo.On("GetStatistics", mock.Anything, mock.AnythingOfType("*domain.StatisticsFilters")).Return(stats, nil)
		reportRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Report")).Return(nil)

		resp, err := srv.GenerateReport(context.Background(), &analyticsv1.GenerateReportRequest{
			ReportType: "daily",
			Format:     "json",
			From:       timestamppb.New(now.AddDate(0, 0, -7)),
			To:         timestamppb.New(now),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.ReportId)
		assert.Equal(t, "json", resp.Format)
		assert.NotNil(t, resp.GeneratedAt)
	})

	t.Run("invalid provider_id returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		now := time.Now()
		resp, err := srv.GenerateReport(context.Background(), &analyticsv1.GenerateReportRequest{
			ReportType: "provider",
			From:       timestamppb.New(now.AddDate(0, 0, -7)),
			To:         timestamppb.New(now),
			ProviderId: "not-a-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestAnalyticsServer_GetProviderPerformance(t *testing.T) {
	t.Run("empty provider_id returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GetProviderPerformance(context.Background(), &analyticsv1.GetProviderPerformanceRequest{
			ProviderId: "",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("invalid provider_id returns InvalidArgument", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		resp, err := srv.GetProviderPerformance(context.Background(), &analyticsv1.GetProviderPerformanceRequest{
			ProviderId: "not-valid-uuid",
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("valid provider returns performance data", func(t *testing.T) {
		metricRepo := new(mockMetricRepo)
		reportRepo := new(mockReportRepo)
		srv := newTestAnalyticsServer(metricRepo, reportRepo)

		providerID := uuid.New()
		perf := &domain.ProviderPerformance{
			ProviderID:      providerID,
			TotalSent:       1000,
			TotalDelivered:  950,
			TotalFailed:     50,
			SuccessRate:     95,
			StatusBreakdown: map[string]int64{"delivered": 950, "failed": 50},
			Points:          []*domain.PerformancePoint{},
		}

		metricRepo.On("GetProviderPerformance", mock.Anything, providerID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(perf, nil)

		resp, err := srv.GetProviderPerformance(context.Background(), &analyticsv1.GetProviderPerformanceRequest{
			ProviderId: providerID.String(),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, providerID.String(), resp.ProviderId)
		assert.Equal(t, int64(1000), resp.TotalSent)
		assert.Equal(t, int64(950), resp.TotalDelivered)
		assert.Equal(t, int32(95), resp.SuccessRate)
	})
}
