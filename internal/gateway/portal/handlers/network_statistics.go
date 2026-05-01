package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	networkanalyticsv1 "github.com/smpp-server/smpp-server/api/proto/networkanalyticsv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

const networkStatsTimeout = 10 * time.Second

// NetworkStatisticsHandlers handles HTTP requests for network analytics endpoints.
type NetworkStatisticsHandlers struct {
	client networkanalyticsv1.NetworkAnalyticsServiceClient
	pool   *pgxpool.Pool
}

// NewNetworkStatisticsHandlers creates a new NetworkStatisticsHandlers.
func NewNetworkStatisticsHandlers(client networkanalyticsv1.NetworkAnalyticsServiceClient, pool *pgxpool.Pool) *NetworkStatisticsHandlers {
	return &NetworkStatisticsHandlers{client: client, pool: pool}
}

// exportJobIDFromRequest extracts job_id from mux vars or URL path as fallback.
// Path pattern: .../export/{job_id}/status or .../export/{job_id}/download
func exportJobIDFromRequest(r *http.Request) string {
	if id := mux.Vars(r)["job_id"]; id != "" {
		return id
	}
	parts := strings.Split(r.URL.Path, "/")
	for i, p := range parts {
		if p == "export" && i+1 < len(parts) {
			next := parts[i+1]
			if next != "" && next != "status" && next != "download" {
				return next
			}
		}
	}
	return ""
}

func (h *NetworkStatisticsHandlers) checkClient(w http.ResponseWriter) bool {
	if h.client == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"kpis": []interface{}{},
			"rows": []interface{}{},
			"pagination": map[string]int{"page": 1, "page_size": 25, "total_rows": 0, "total_pages": 0},
		})
		return false
	}
	return true
}

// parseSharedFilter reads all filter query params from the request.
func parseSharedFilter(r *http.Request) *networkanalyticsv1.SharedFilter {
	q := r.URL.Query()

	var dateFrom, dateTo int64
	if v := q.Get("date_from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			dateFrom = n
		}
	}
	if v := q.Get("date_to"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			dateTo = n
		}
	}

	var page, pageSize int32
	if v := q.Get("page"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			page = int32(n)
		}
	}
	if v := q.Get("page_size"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			pageSize = int32(n)
		}
	}

	international := false
	if v := q.Get("international"); v != "" {
		international, _ = strconv.ParseBool(v)
	}

	return &networkanalyticsv1.SharedFilter{
		PeriodPreset: q.Get("period_preset"),
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		GroupBy:      q.Get("group_by"),
		Login:        q.Get("login"),
		ServiceType:  q.Get("service_type"),
		Operator:     q.Get("operator"),
		Channel:      q.Get("channel"),
		SenderName:   q.Get("sender_name"),
		SenderPaid:   q.Get("sender_paid"),
		International: international,
		TrafficType:  q.Get("traffic_type"),
		Status:       q.Get("status"),
		PriceRange:   q.Get("price_range"),
		Method:       q.Get("method"),
		Provider:     q.Get("provider"),
		Country:      q.Get("country"),
		Manager:      q.Get("manager"),
		ErrorCode:    q.Get("error_code"),
		Page:         page,
		PageSize:     pageSize,
		SortBy:       q.Get("sort_by"),
		SortDir:      q.Get("sort_dir"),
	}
}

// writeJSON sets Content-Type and encodes data as JSON.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("network_statistics: failed to encode JSON response")
	}
}

// resolvePartnerID returns the EFFECTIVE partner_id for the authenticated
// client: for sub-accounts (parent_client_id IS NOT NULL) we collapse to the
// parent's partner_id so reseller analytics include the entire account tree.
// Direct clients (parent_client_id IS NULL) use their own partner_id. The
// aggregation query in network_analytics service uses the same hierarchy
// resolution — both sides must stay in sync.
//
// The `?partner_id=` query parameter is IGNORED — historically it was trusted
// (TC-AGG-5 / B.1 leak); now scoping is derived from the session client_id only.
//
// Returns (partner_id, true) on success. On failure writes the response and
// returns (0, false): caller must return immediately.
func (h *NetworkStatisticsHandlers) resolvePartnerID(ctx context.Context, w http.ResponseWriter, clientID uuid.UUID) (int64, bool) {
	if h.pool == nil {
		respondError(w, shared.ErrInternalServer("База недоступна"))
		return 0, false
	}
	var pid int64
	err := h.pool.QueryRow(ctx, `
		SELECT COALESCE(p.partner_id, c.partner_id)
		FROM clients c
		LEFT JOIN clients p ON p.id = c.parent_client_id
		WHERE c.id = $1
	`, clientID).Scan(&pid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, shared.ErrUnauthorized("Клиент не найден"))
			return 0, false
		}
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("network_statistics: resolve partner_id failed")
		respondError(w, shared.ErrInternalServer("Не удалось разрешить partner_id"))
		return 0, false
	}
	return pid, true
}

// userIDFromContext returns the authenticated user's numeric id.
// UUID is hashed to a stable int64 via its most-significant 64 bits.
func userIDFromContext(r *http.Request) int64 {
	userUUID, ok := middleware.GetUserID(r.Context())
	if !ok {
		return 0
	}
	// Use the first 8 bytes of the UUID as int64.
	b := userUUID
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

// GetStatistics handles GET /network/statistics
func (h *NetworkStatisticsHandlers) GetStatistics(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.GetStatistics(ctx, &networkanalyticsv1.StatisticsRequest{
		PartnerId: partnerID,
		Filter:    parseSharedFilter(r),
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: GetStatistics failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// GetAnalytics handles GET /network/analytics
func (h *NetworkStatisticsHandlers) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.GetAnalyticsSummary(ctx, &networkanalyticsv1.AnalyticsRequest{
		PartnerId: partnerID,
		Filter:    parseSharedFilter(r),
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: GetAnalyticsSummary failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// GetMonitoring handles GET /network/monitoring
func (h *NetworkStatisticsHandlers) GetMonitoring(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	hideHealthy := false
	if v := r.URL.Query().Get("hide_healthy"); v != "" {
		hideHealthy, _ = strconv.ParseBool(v)
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.GetMonitoringMetrics(ctx, &networkanalyticsv1.MonitoringRequest{
		PartnerId:   partnerID,
		Filter:      parseSharedFilter(r),
		HideHealthy: hideHealthy,
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: GetMonitoringMetrics failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// GetDrillDown handles GET /network/drilldown
func (h *NetworkStatisticsHandlers) GetDrillDown(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	q := r.URL.Query()

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.GetDrillDown(ctx, &networkanalyticsv1.DrillDownRequest{
		PartnerId:   partnerID,
		Filter:      parseSharedFilter(r),
		SliceType:   q.Get("slice_type"),
		SliceValue:  q.Get("slice_value"),
		DetailView:  q.Get("detail_view"),
		ParentType:  q.Get("parent_type"),
		ParentValue: q.Get("parent_value"),
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: GetDrillDown failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// startExportBody is the expected JSON body for StartExport.
type startExportBody struct {
	Filter *networkanalyticsv1.SharedFilter `json:"filter"`
	Mode   string                           `json:"mode"`
	Format string                           `json:"format"`
}

// StartExport handles POST /network/export
func (h *NetworkStatisticsHandlers) StartExport(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	var body startExportBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.StartExport(ctx, &networkanalyticsv1.ExportRequest{
		PartnerId: partnerID,
		Filter:    body.Filter,
		Mode:      body.Mode,
		Format:    body.Format,
		UserId:    userIDFromContext(r),
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: StartExport failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// GetExportStatus handles GET /network/export/{job_id}
func (h *NetworkStatisticsHandlers) GetExportStatus(w http.ResponseWriter, r *http.Request) {
	// gorilla/mux subrouter routing bug: download URL sometimes matches this handler
	if strings.HasSuffix(r.URL.Path, "/download") {
		h.DownloadExport(w, r)
		return
	}

	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	jobID := exportJobIDFromRequest(r)
	if jobID == "" {
		respondError(w, shared.ErrInvalidInput("job_id обязателен"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	resp, err := h.client.GetExportStatus(ctx, &networkanalyticsv1.ExportStatusRequest{
		JobId: jobID,
	})
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("network_statistics: GetExportStatus failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// DownloadExport handles GET /network/export/{job_id}/download — streams the exported file.
func (h *NetworkStatisticsHandlers) DownloadExport(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	jobID := exportJobIDFromRequest(r)
	if jobID == "" {
		respondError(w, shared.ErrInvalidInput("job_id обязателен"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	resp, err := h.client.GetExportStatus(ctx, &networkanalyticsv1.ExportStatusRequest{
		JobId: jobID,
	})
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("network_statistics: DownloadExport status check failed")
		respondGRPCError(w, err)
		return
	}

	if resp.Status != "done" || resp.DownloadUrl == "" {
		respondError(w, shared.ErrInvalidInput("Экспорт ещё не готов"))
		return
	}

	filePath := resp.DownloadUrl // DownloadUrl contains the server-side file path
	f, err := os.Open(filePath)
	if err != nil {
		log.Error().Err(err).Str("path", filePath).Msg("network_statistics: cannot open export file")
		respondError(w, shared.ErrInternalServer("Файл экспорта не найден"))
		return
	}
	defer f.Close()

	ext := filepath.Ext(filePath)
	switch ext {
	case ".csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	case ".xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="export-%s%s"`, jobID, ext))
	http.ServeContent(w, r, filepath.Base(filePath), time.Now(), f)
}

// ListViews handles GET /network/views
func (h *NetworkStatisticsHandlers) ListViews(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.ListSavedViews(ctx, &networkanalyticsv1.ListViewsRequest{
		PartnerId: partnerID,
		UserId:    userIDFromContext(r),
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: ListSavedViews failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// saveViewBody is the expected JSON body for SaveView.
type saveViewBody struct {
	View *networkanalyticsv1.SavedView `json:"view"`
}

// SaveView handles POST /network/views
func (h *NetworkStatisticsHandlers) SaveView(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	var body saveViewBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if body.View == nil {
		respondError(w, shared.ErrInvalidInput("Поле view обязательно"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	resp, err := h.client.SaveView(ctx, &networkanalyticsv1.SaveViewRequest{
		PartnerId: partnerID,
		UserId:    userIDFromContext(r),
		View:      body.View,
	})
	if err != nil {
		log.Error().Err(err).Msg("network_statistics: SaveView failed")
		respondGRPCError(w, err)
		return
	}

	writeJSON(w, resp)
}

// DeleteView handles DELETE /network/views/{id}
func (h *NetworkStatisticsHandlers) DeleteView(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	if !h.checkClient(w) {
		return
	}

	idStr := mux.Vars(r)["id"]
	if idStr == "" {
		respondError(w, shared.ErrInvalidInput("id обязателен"))
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат id"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), networkStatsTimeout)
	defer cancel()

	partnerID, ok := h.resolvePartnerID(ctx, w, clientID)
	if !ok {
		return
	}

	_, err = h.client.DeleteView(ctx, &networkanalyticsv1.DeleteViewRequest{
		Id:        id,
		PartnerId: partnerID,
		UserId:    userIDFromContext(r),
	})
	if err != nil {
		log.Error().Err(err).Int64("id", id).Msg("network_statistics: DeleteView failed")
		respondGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
