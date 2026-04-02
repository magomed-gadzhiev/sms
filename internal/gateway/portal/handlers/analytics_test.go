package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

type analyticsTestClient struct {
	getStatisticsFn func(ctx context.Context, req *analyticsv1.GetStatisticsRequest) (*analyticsv1.GetStatisticsResponse, error)
}

func (m *analyticsTestClient) GetStatistics(ctx context.Context, req *analyticsv1.GetStatisticsRequest, _ ...grpc.CallOption) (*analyticsv1.GetStatisticsResponse, error) {
	if m.getStatisticsFn != nil {
		return m.getStatisticsFn(ctx, req)
	}
	return &analyticsv1.GetStatisticsResponse{}, nil
}

func (m *analyticsTestClient) GenerateReport(context.Context, *analyticsv1.GenerateReportRequest, ...grpc.CallOption) (*analyticsv1.GenerateReportResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *analyticsTestClient) GetRealtimeMetrics(context.Context, *analyticsv1.GetRealtimeMetricsRequest, ...grpc.CallOption) (*analyticsv1.GetRealtimeMetricsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *analyticsTestClient) GetProviderPerformance(context.Context, *analyticsv1.GetProviderPerformanceRequest, ...grpc.CallOption) (*analyticsv1.GetProviderPerformanceResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func withAnalyticsClientID(r *http.Request, clientID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClientIDKey, clientID)
	return r.WithContext(ctx)
}

func TestAnalyticsHandlers_GetAnalytics(t *testing.T) {
	t.Run("returns 503 when analytics service is unavailable", func(t *testing.T) {
		h := NewAnalyticsHandlers(nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("returns 400 for invalid group_by", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d&group_by=provider", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 when only one custom date is provided", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?date_from=2026-03-01", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 when date_to is earlier than date_from", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?date_from=2026-03-31&date_to=2026-03-01", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for invalid compare flag", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d&compare=not-bool", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 400 for too large custom range", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?date_from=2024-01-01&date_to=2026-03-31", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns analytics payload for valid request", func(t *testing.T) {
		clientID := uuid.New()
		h := NewAnalyticsHandlers(&analyticsTestClient{
			getStatisticsFn: func(_ context.Context, req *analyticsv1.GetStatisticsRequest) (*analyticsv1.GetStatisticsResponse, error) {
				assert.Equal(t, clientID.String(), req.ClientId)
				assert.Equal(t, "day", req.GroupBy)
				return &analyticsv1.GetStatisticsResponse{
					Totals: &analyticsv1.TotalStats{
						TotalSent:      100,
						TotalDelivered: 90,
						TotalFailed:    10,
						SuccessRate:    90,
					},
					Groups: []*analyticsv1.StatisticGroup{
						{
							Key: "2026-03-31",
							Stats: &analyticsv1.TotalStats{
								TotalSent:      100,
								TotalDelivered: 90,
								TotalFailed:    10,
								SuccessRate:    90,
							},
						},
					},
				}, nil
			},
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d&group_by=day", nil)
		req = withAnalyticsClientID(req, clientID)
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Contains(t, resp, "summary")
		require.Contains(t, resp, "timeline")

		summary := resp["summary"].(map[string]interface{})
		assert.Equal(t, float64(100), summary["total_sent"])
		assert.Equal(t, float64(90), summary["total_delivered"])
		assert.Equal(t, float64(10), summary["total_failed"])
		assert.Equal(t, float64(0), summary["total_expired"])

		timeline := resp["timeline"].([]interface{})
		require.Len(t, timeline, 1)
		entry := timeline[0].(map[string]interface{})
		assert.Equal(t, "2026-03-31", entry["period"])
		assert.Equal(t, float64(100), entry["sent"])
		assert.Equal(t, float64(90), entry["delivered"])
		assert.Equal(t, float64(10), entry["failed"])
	})

	t.Run("returns 400 for invalid include_cost flag", func(t *testing.T) {
		h := NewAnalyticsHandlers(&analyticsTestClient{}, nil)
		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d&include_cost=invalid", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("supports compare=true without failing", func(t *testing.T) {
		calls := 0
		h := NewAnalyticsHandlers(&analyticsTestClient{
			getStatisticsFn: func(_ context.Context, _ *analyticsv1.GetStatisticsRequest) (*analyticsv1.GetStatisticsResponse, error) {
				calls++
				return &analyticsv1.GetStatisticsResponse{
					Totals: &analyticsv1.TotalStats{},
					Groups: []*analyticsv1.StatisticGroup{
						{Key: timestamppb.Now().AsTime().Format("2006-01-02"), Stats: &analyticsv1.TotalStats{}},
					},
				}, nil
			},
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/portal/v1/analytics?period=7d&compare=true", nil)
		req = withAnalyticsClientID(req, uuid.New())
		w := httptest.NewRecorder()

		h.GetAnalytics(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 2, calls)
	})
}
