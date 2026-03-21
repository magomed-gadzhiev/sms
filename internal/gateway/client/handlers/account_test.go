package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
)

// --- Mock BillingServiceClient ---

type mockBillingClient struct {
	mock.Mock
}

func (m *mockBillingClient) GetBalance(ctx context.Context, in *billingv1.GetBalanceRequest, opts ...grpc.CallOption) (*billingv1.GetBalanceResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetBalanceResponse), args.Error(1)
}

func (m *mockBillingClient) ChargeMessage(ctx context.Context, in *billingv1.ChargeMessageRequest, opts ...grpc.CallOption) (*billingv1.ChargeMessageResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.ChargeMessageResponse), args.Error(1)
}

func (m *mockBillingClient) AddCredits(ctx context.Context, in *billingv1.AddCreditsRequest, opts ...grpc.CallOption) (*billingv1.AddCreditsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.AddCreditsResponse), args.Error(1)
}

func (m *mockBillingClient) DeductCredits(ctx context.Context, in *billingv1.DeductCreditsRequest, opts ...grpc.CallOption) (*billingv1.DeductCreditsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.DeductCreditsResponse), args.Error(1)
}

func (m *mockBillingClient) GetTransactionHistory(ctx context.Context, in *billingv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*billingv1.GetTransactionHistoryResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetTransactionHistoryResponse), args.Error(1)
}

func (m *mockBillingClient) GetPricingRules(ctx context.Context, in *billingv1.GetPricingRulesRequest, opts ...grpc.CallOption) (*billingv1.GetPricingRulesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetPricingRulesResponse), args.Error(1)
}

func (m *mockBillingClient) CreatePricingRule(ctx context.Context, in *billingv1.CreatePricingRuleRequest, opts ...grpc.CallOption) (*billingv1.CreatePricingRuleResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.CreatePricingRuleResponse), args.Error(1)
}

func (m *mockBillingClient) TransferBalance(ctx context.Context, in *billingv1.TransferBalanceRequest, opts ...grpc.CallOption) (*billingv1.TransferBalanceResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.TransferBalanceResponse), args.Error(1)
}

// --- Mock AnalyticsServiceClient ---

type mockAnalyticsClient struct {
	mock.Mock
}

func (m *mockAnalyticsClient) GetStatistics(ctx context.Context, in *analyticsv1.GetStatisticsRequest, opts ...grpc.CallOption) (*analyticsv1.GetStatisticsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*analyticsv1.GetStatisticsResponse), args.Error(1)
}

func (m *mockAnalyticsClient) GenerateReport(ctx context.Context, in *analyticsv1.GenerateReportRequest, opts ...grpc.CallOption) (*analyticsv1.GenerateReportResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*analyticsv1.GenerateReportResponse), args.Error(1)
}

func (m *mockAnalyticsClient) GetRealtimeMetrics(ctx context.Context, in *analyticsv1.GetRealtimeMetricsRequest, opts ...grpc.CallOption) (*analyticsv1.GetRealtimeMetricsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*analyticsv1.GetRealtimeMetricsResponse), args.Error(1)
}

func (m *mockAnalyticsClient) GetProviderPerformance(ctx context.Context, in *analyticsv1.GetProviderPerformanceRequest, opts ...grpc.CallOption) (*analyticsv1.GetProviderPerformanceResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*analyticsv1.GetProviderPerformanceResponse), args.Error(1)
}

// --- Tests ---

func TestAccountHandlers(t *testing.T) {
	t.Run("GetBalance", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			billingClient := new(mockBillingClient)
			analyticsClient := new(mockAnalyticsClient)
			handler := NewAccountHandlers(billingClient, analyticsClient)

			clientID := uuid.New()

			billingClient.On("GetBalance", mock.Anything, mock.MatchedBy(func(req *billingv1.GetBalanceRequest) bool {
				return req.ClientId == clientID.String()
			})).Return(&billingv1.GetBalanceResponse{
				ClientId:  clientID.String(),
				Balance:   "150.50",
				Currency:  "USD",
				UpdatedAt: timestamppb.Now(),
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, clientID.String(), resp["client_id"])
			assert.Equal(t, "150.50", resp["balance"])
			assert.Equal(t, "USD", resp["currency"])
			assert.NotNil(t, resp["updated_at"])

			billingClient.AssertExpectations(t)
		})

		t.Run("returns 401 when client ID is missing", func(t *testing.T) {
			billingClient := new(mockBillingClient)
			analyticsClient := new(mockAnalyticsClient)
			handler := NewAccountHandlers(billingClient, analyticsClient)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns error when billing service fails", func(t *testing.T) {
			billingClient := new(mockBillingClient)
			analyticsClient := new(mockAnalyticsClient)
			handler := NewAccountHandlers(billingClient, analyticsClient)

			clientID := uuid.New()

			billingClient.On("GetBalance", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "billing service down"))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("GetStats", func(t *testing.T) {
		t.Run("success with defaults", func(t *testing.T) {
			billingClient := new(mockBillingClient)
			analyticsClient := new(mockAnalyticsClient)
			handler := NewAccountHandlers(billingClient, analyticsClient)

			clientID := uuid.New()

			analyticsClient.On("GetStatistics", mock.Anything, mock.MatchedBy(func(req *analyticsv1.GetStatisticsRequest) bool {
				return req.ClientId == clientID.String() && req.GroupBy == "day"
			})).Return(&analyticsv1.GetStatisticsResponse{
				Groups: []*analyticsv1.StatisticGroup{},
				Totals: &analyticsv1.TotalStats{
					TotalSent:      100,
					TotalDelivered: 95,
					TotalFailed:    5,
					SuccessRate:    95.0,
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/stats", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.GetStats(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, clientID.String(), resp["client_id"])
			assert.Equal(t, "day", resp["group_by"])
			assert.NotNil(t, resp["totals"])

			analyticsClient.AssertExpectations(t)
		})

		t.Run("returns 401 when client ID is missing", func(t *testing.T) {
			billingClient := new(mockBillingClient)
			analyticsClient := new(mockAnalyticsClient)
			handler := NewAccountHandlers(billingClient, analyticsClient)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/stats", nil)

			rr := httptest.NewRecorder()
			handler.GetStats(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	})
}
