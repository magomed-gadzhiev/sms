package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/internal/services/analytics/application"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
)

// Server реализует gRPC сервис для аналитики
type Server struct {
	analyticsv1.UnimplementedAnalyticsServiceServer
	analyticsService *application.AnalyticsService
	reportService    *application.ReportService
	realtimeService  *application.RealtimeService
}

// NewServer создает новый gRPC сервер для Analytics Service
func NewServer(
	analyticsService *application.AnalyticsService,
	reportService *application.ReportService,
	realtimeService *application.RealtimeService,
) *Server {
	return &Server{
		analyticsService: analyticsService,
		reportService:    reportService,
		realtimeService:  realtimeService,
	}
}

// GetStatistics получает статистику по заданным фильтрам
func (s *Server) GetStatistics(ctx context.Context, req *analyticsv1.GetStatisticsRequest) (*analyticsv1.GetStatisticsResponse, error) {
	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	var from, to time.Time
	if req.From != nil {
		from = req.From.AsTime()
	} else {
		from = time.Now().AddDate(0, 0, -30) // По умолчанию последние 30 дней
	}

	if req.To != nil {
		to = req.To.AsTime()
	} else {
		to = time.Now()
	}

	groupBy := req.GroupBy
	if groupBy == "" {
		groupBy = "day"
	}

	filters := &domain.StatisticsFilters{
		ClientID: clientID,
		From:     from,
		To:       to,
		GroupBy:  groupBy,
	}

	if len(req.ProviderIds) > 0 {
		filters.ProviderIDs = make([]uuid.UUID, len(req.ProviderIds))
		for i, pid := range req.ProviderIds {
			providerID, err := uuid.Parse(pid)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid provider_id format")
			}
			filters.ProviderIDs[i] = providerID
		}
	}

	stats, err := s.analyticsService.GetStatistics(ctx, filters)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения статистики")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Преобразуем в proto формат
	groups := make([]*analyticsv1.StatisticGroup, len(stats.Groups))
	for i, group := range stats.Groups {
		groups[i] = &analyticsv1.StatisticGroup{
			Key: group.Key,
			Stats: &analyticsv1.TotalStats{
				TotalSent:           group.Stats.TotalSent,
				TotalDelivered:      group.Stats.TotalDelivered,
				TotalFailed:         group.Stats.TotalFailed,
				TotalPending:        group.Stats.TotalPending,
				TotalQueued:         group.Stats.TotalQueued,
				SuccessRate:         group.Stats.SuccessRate,
				AvgDeliveryTimeMs:   group.Stats.AvgDeliveryTimeMs,
			},
		}
	}

	return &analyticsv1.GetStatisticsResponse{
		Groups: groups,
		Totals: &analyticsv1.TotalStats{
			TotalSent:           stats.Totals.TotalSent,
			TotalDelivered:      stats.Totals.TotalDelivered,
			TotalFailed:         stats.Totals.TotalFailed,
			TotalPending:        stats.Totals.TotalPending,
			TotalQueued:         stats.Totals.TotalQueued,
			SuccessRate:         stats.Totals.SuccessRate,
			AvgDeliveryTimeMs:   stats.Totals.AvgDeliveryTimeMs,
		},
	}, nil
}

// GenerateReport генерирует отчет указанного типа
func (s *Server) GenerateReport(ctx context.Context, req *analyticsv1.GenerateReportRequest) (*analyticsv1.GenerateReportResponse, error) {
	if req.ReportType == "" {
		return nil, status.Error(codes.InvalidArgument, "report_type is required")
	}

	if req.From == nil || req.To == nil {
		return nil, status.Error(codes.InvalidArgument, "from and to timestamps are required")
	}

	reportType := domain.ReportType(req.ReportType)
	format := domain.ReportFormat(req.Format)
	if format == "" {
		format = domain.ReportFormatJSON
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		id, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &id
	}

	var providerID *uuid.UUID
	if req.ProviderId != "" {
		id, err := uuid.Parse(req.ProviderId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid provider_id format")
		}
		providerID = &id
	}

	from := req.From.AsTime()
	to := req.To.AsTime()

	report, err := s.reportService.GenerateReport(ctx, reportType, format, from, to, clientID, providerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка генерации отчета")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &analyticsv1.GenerateReportResponse{
		ReportId:    report.ID.String(),
		Format:      string(report.Format),
		Data:        report.Data,
		GeneratedAt: timestamppb.New(report.GeneratedAt),
	}, nil
}

// GetRealtimeMetrics получает метрики в реальном времени
func (s *Server) GetRealtimeMetrics(ctx context.Context, req *analyticsv1.GetRealtimeMetricsRequest) (*analyticsv1.GetRealtimeMetricsResponse, error) {
	metrics, err := s.realtimeService.GetRealtimeMetrics(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения real-time метрик")
		return nil, status.Error(codes.Internal, err.Error())
	}

	providerMetrics := make(map[string]int64)
	for k, v := range metrics.ProviderMetrics {
		providerMetrics[k] = v
	}

	return &analyticsv1.GetRealtimeMetricsResponse{
		MessagesPerSecond:       metrics.MessagesPerSecond,
		TotalMessagesQueued:     metrics.TotalMessagesQueued,
		TotalMessagesProcessing: metrics.TotalMessagesProcessing,
		ActiveProviders:         metrics.ActiveProviders,
		ActiveConnections:       metrics.ActiveConnections,
		ProviderMetrics:         providerMetrics,
		Timestamp:               timestamppb.New(metrics.Timestamp),
	}, nil
}

// GetProviderPerformance получает статистику производительности провайдера
func (s *Server) GetProviderPerformance(ctx context.Context, req *analyticsv1.GetProviderPerformanceRequest) (*analyticsv1.GetProviderPerformanceResponse, error) {
	if req.ProviderId == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_id is required")
	}

	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id format")
	}

	var from, to time.Time
	if req.From != nil {
		from = req.From.AsTime()
	} else {
		from = time.Now().AddDate(0, 0, -7) // По умолчанию последние 7 дней
	}

	if req.To != nil {
		to = req.To.AsTime()
	} else {
		to = time.Now()
	}

	perf, err := s.analyticsService.GetProviderPerformance(ctx, providerID, from, to)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения производительности провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Преобразуем в proto формат
	points := make([]*analyticsv1.PerformancePoint, len(perf.Points))
	for i, point := range perf.Points {
		points[i] = &analyticsv1.PerformancePoint{
			Timestamp:  timestamppb.New(point.Timestamp),
			Sent:       point.Sent,
			Delivered:  point.Delivered,
			Failed:     point.Failed,
			SuccessRate: point.SuccessRate,
		}
	}

	statusBreakdown := make(map[string]int64)
	for k, v := range perf.StatusBreakdown {
		statusBreakdown[k] = v
	}

	return &analyticsv1.GetProviderPerformanceResponse{
		ProviderId:        perf.ProviderID.String(),
		TotalSent:         perf.TotalSent,
		TotalDelivered:    perf.TotalDelivered,
		TotalFailed:       perf.TotalFailed,
		SuccessRate:       perf.SuccessRate,
		AvgDeliveryTimeMs: perf.AvgDeliveryTimeMs,
		StatusBreakdown:   statusBreakdown,
		Points:            points,
	}, nil
}
