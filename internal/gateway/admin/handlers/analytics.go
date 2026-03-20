package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// AnalyticsHandlers обрабатывает HTTP запросы для аналитики
type AnalyticsHandlers struct {
	analyticsClient analyticsv1.AnalyticsServiceClient
}

// NewAnalyticsHandlers создает новый экземпляр AnalyticsHandlers
func NewAnalyticsHandlers(analyticsClient analyticsv1.AnalyticsServiceClient) *AnalyticsHandlers {
	return &AnalyticsHandlers{
		analyticsClient: analyticsClient,
	}
}

// GetStatistics обрабатывает GET /admin/v1/analytics/stats
func (h *AnalyticsHandlers) GetStatistics(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	groupBy := r.URL.Query().Get("group_by")
	providerIDs := r.URL.Query()["provider_ids"]

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра from (ожидается RFC3339)"))
			return
		}
		from = &t
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра to (ожидается RFC3339)"))
			return
		}
		to = &t
	}

	grpcReq := &analyticsv1.GetStatisticsRequest{
		ClientId:    clientID,
		GroupBy:     groupBy,
		ProviderIds: providerIDs,
	}
	if from != nil {
		grpcReq.From = timestamppb.New(*from)
	}
	if to != nil {
		grpcReq.To = timestamppb.New(*to)
	}

	resp, err := h.analyticsClient.GetStatistics(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения статистики")
		respondGRPCError(w, err)
		return
	}

	groups := make([]StatisticGroup, len(resp.Groups))
	for i, g := range resp.Groups {
		groups[i] = statisticGroupToResponse(g)
	}

	respondJSON(w, http.StatusOK, GetStatisticsResponse{
		Groups: groups,
		Totals: totalStatsToResponse(resp.Totals),
	})
}

// GenerateReport обрабатывает POST /admin/v1/analytics/reports
func (h *AnalyticsHandlers) GenerateReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, err.(*shared.AppError))
		return
	}

	grpcReq := &analyticsv1.GenerateReportRequest{
		ReportType: req.ReportType,
		ClientId:   req.ClientID,
		ProviderId: req.ProviderID,
		Format:     req.Format,
	}
	if req.From != nil {
		grpcReq.From = timestamppb.New(*req.From)
	}
	if req.To != nil {
		grpcReq.To = timestamppb.New(*req.To)
	}

	resp, err := h.analyticsClient.GenerateReport(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка генерации отчета")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, GenerateReportResponse{
		ReportID:    resp.ReportId,
		Format:      resp.Format,
		Data:        resp.Data,
		GeneratedAt: resp.GeneratedAt.AsTime(),
	})
}

// GetRealtimeMetrics обрабатывает GET /admin/v1/analytics/metrics/realtime
func (h *AnalyticsHandlers) GetRealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	resp, err := h.analyticsClient.GetRealtimeMetrics(r.Context(), &analyticsv1.GetRealtimeMetricsRequest{})
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения метрик в реальном времени")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, RealtimeMetrics{
		MessagesPerSecond:      resp.MessagesPerSecond,
		TotalMessagesQueued:    resp.TotalMessagesQueued,
		TotalMessagesProcessing: resp.TotalMessagesProcessing,
		ActiveProviders:        resp.ActiveProviders,
		ActiveConnections:      resp.ActiveConnections,
		ProviderMetrics:        resp.ProviderMetrics,
		Timestamp:              resp.Timestamp.AsTime(),
	})
}

// GetProviderPerformance обрабатывает GET /admin/v1/analytics/providers/:id/performance
func (h *AnalyticsHandlers) GetProviderPerformance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]
	if providerID == "" {
		// Попробуем из query параметра для обратной совместимости
		providerID = r.URL.Query().Get("provider_id")
	}
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if providerID == "" {
		respondError(w, shared.ErrInvalidInput("provider_id обязателен"))
		return
	}

	var from, to *time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра from"))
			return
		}
		from = &t
	}
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный формат параметра to"))
			return
		}
		to = &t
	}

	grpcReq := &analyticsv1.GetProviderPerformanceRequest{
		ProviderId: providerID,
	}
	if from != nil {
		grpcReq.From = timestamppb.New(*from)
	}
	if to != nil {
		grpcReq.To = timestamppb.New(*to)
	}

	resp, err := h.analyticsClient.GetProviderPerformance(r.Context(), grpcReq)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения производительности провайдера")
		respondGRPCError(w, err)
		return
	}

	points := make([]PerformancePoint, len(resp.Points))
	for i, p := range resp.Points {
		points[i] = PerformancePoint{
			Timestamp:  p.Timestamp.AsTime(),
			Sent:       p.Sent,
			Delivered:  p.Delivered,
			Failed:     p.Failed,
			SuccessRate: int(p.SuccessRate),
		}
	}

	respondJSON(w, http.StatusOK, ProviderPerformance{
		ProviderID:      resp.ProviderId,
		TotalSent:       resp.TotalSent,
		TotalDelivered:  resp.TotalDelivered,
		TotalFailed:     resp.TotalFailed,
		SuccessRate:     int(resp.SuccessRate),
		AvgDeliveryTimeMs: resp.AvgDeliveryTimeMs,
		StatusBreakdown: resp.StatusBreakdown,
		Points:          points,
	})
}

// Типы запросов и ответов

type GetStatisticsResponse struct {
	Groups []StatisticGroup `json:"groups"`
	Totals TotalStats       `json:"totals"`
}

type StatisticGroup struct {
	Key   string     `json:"key"`
	Stats TotalStats `json:"stats"`
}

type TotalStats struct {
	TotalSent         int64 `json:"total_sent"`
	TotalDelivered    int64 `json:"total_delivered"`
	TotalFailed       int64 `json:"total_failed"`
	TotalPending      int64 `json:"total_pending"`
	TotalQueued       int64 `json:"total_queued"`
	SuccessRate       int   `json:"success_rate"`
	AvgDeliveryTimeMs int64 `json:"avg_delivery_time_ms"`
}

type GenerateReportRequest struct {
	ReportType string     `json:"report_type"`
	From       *time.Time `json:"from,omitempty"`
	To         *time.Time `json:"to,omitempty"`
	ClientID   string     `json:"client_id,omitempty"`
	ProviderID string     `json:"provider_id,omitempty"`
	Format     string     `json:"format"`
}

func (r *GenerateReportRequest) Validate() error {
	if r.ReportType == "" {
		return shared.ErrInvalidInput("report_type обязателен")
	}
	if r.Format == "" {
		r.Format = "json"
	}
	return nil
}

type GenerateReportResponse struct {
	ReportID    string    `json:"report_id"`
	Format      string    `json:"format"`
	Data        []byte    `json:"data"`
	GeneratedAt time.Time `json:"generated_at"`
}

type RealtimeMetrics struct {
	MessagesPerSecond       int64            `json:"messages_per_second"`
	TotalMessagesQueued     int64            `json:"total_messages_queued"`
	TotalMessagesProcessing int64            `json:"total_messages_processing"`
	ActiveProviders         int64            `json:"active_providers"`
	ActiveConnections       int64            `json:"active_connections"`
	ProviderMetrics         map[string]int64 `json:"provider_metrics"`
	Timestamp               time.Time        `json:"timestamp"`
}

type ProviderPerformance struct {
	ProviderID        string            `json:"provider_id"`
	TotalSent         int64             `json:"total_sent"`
	TotalDelivered    int64             `json:"total_delivered"`
	TotalFailed       int64             `json:"total_failed"`
	SuccessRate       int               `json:"success_rate"`
	AvgDeliveryTimeMs int64             `json:"avg_delivery_time_ms"`
	StatusBreakdown   map[string]int64  `json:"status_breakdown"`
	Points            []PerformancePoint `json:"points"`
}

type PerformancePoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Sent        int64     `json:"sent"`
	Delivered   int64     `json:"delivered"`
	Failed      int64     `json:"failed"`
	SuccessRate int       `json:"success_rate"`
}

func statisticGroupToResponse(g *analyticsv1.StatisticGroup) StatisticGroup {
	return StatisticGroup{
		Key:   g.Key,
		Stats: totalStatsToResponse(g.Stats),
	}
}

func totalStatsToResponse(ts *analyticsv1.TotalStats) TotalStats {
	return TotalStats{
		TotalSent:         ts.TotalSent,
		TotalDelivered:    ts.TotalDelivered,
		TotalFailed:       ts.TotalFailed,
		TotalPending:      ts.TotalPending,
		TotalQueued:       ts.TotalQueued,
		SuccessRate:       int(ts.SuccessRate),
		AvgDeliveryTimeMs: ts.AvgDeliveryTimeMs,
	}
}
