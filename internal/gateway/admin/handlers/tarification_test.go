package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/tarificationv1"
)

// --- Mock TarificationServiceClient ---

type mockTarificationClient struct {
	mock.Mock
}

func (m *mockTarificationClient) TarifyMessage(ctx context.Context, in *tarificationv1.TarifyMessageRequest, opts ...grpc.CallOption) (*tarificationv1.TarifyMessageResponse, error) {
	return nil, nil
}

func (m *mockTarificationClient) CreateSenderRegistration(ctx context.Context, in *tarificationv1.CreateSenderRegistrationRequest, opts ...grpc.CallOption) (*tarificationv1.SenderRegistration, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.SenderRegistration), args.Error(1)
}

func (m *mockTarificationClient) ListSenderRegistrations(ctx context.Context, in *tarificationv1.ListSenderRegistrationsRequest, opts ...grpc.CallOption) (*tarificationv1.ListSenderRegistrationsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.ListSenderRegistrationsResponse), args.Error(1)
}

func (m *mockTarificationClient) UpdateSenderRegistration(ctx context.Context, in *tarificationv1.UpdateSenderRegistrationRequest, opts ...grpc.CallOption) (*tarificationv1.SenderRegistration, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.SenderRegistration), args.Error(1)
}

func (m *mockTarificationClient) CreateTariffPlan(ctx context.Context, in *tarificationv1.CreateTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.TariffPlan, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.TariffPlan), args.Error(1)
}

func (m *mockTarificationClient) GetTariffPlan(ctx context.Context, in *tarificationv1.GetTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.TariffPlan, error) {
	return nil, nil
}

func (m *mockTarificationClient) ListTariffPlans(ctx context.Context, in *tarificationv1.ListTariffPlansRequest, opts ...grpc.CallOption) (*tarificationv1.ListTariffPlansResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.ListTariffPlansResponse), args.Error(1)
}

func (m *mockTarificationClient) UpdateTariffPlan(ctx context.Context, in *tarificationv1.UpdateTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.TariffPlan, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.TariffPlan), args.Error(1)
}

func (m *mockTarificationClient) CreateTariffPeriod(ctx context.Context, in *tarificationv1.CreateTariffPeriodRequest, opts ...grpc.CallOption) (*tarificationv1.TariffPeriod, error) {
	return nil, nil
}

func (m *mockTarificationClient) CreateTariffTier(ctx context.Context, in *tarificationv1.CreateTariffTierRequest, opts ...grpc.CallOption) (*tarificationv1.TariffTier, error) {
	return nil, nil
}

func (m *mockTarificationClient) UpdateTariffTier(ctx context.Context, in *tarificationv1.UpdateTariffTierRequest, opts ...grpc.CallOption) (*tarificationv1.TariffTier, error) {
	return nil, nil
}

func (m *mockTarificationClient) CreatePricingPeriod(ctx context.Context, in *tarificationv1.CreatePricingPeriodRequest, opts ...grpc.CallOption) (*tarificationv1.PricingPeriod, error) {
	return nil, nil
}

func (m *mockTarificationClient) CreatePrepaidFee(ctx context.Context, in *tarificationv1.CreatePrepaidFeeRequest, opts ...grpc.CallOption) (*tarificationv1.PrepaidFee, error) {
	return nil, nil
}

func (m *mockTarificationClient) GetUsageCounter(ctx context.Context, in *tarificationv1.GetUsageCounterRequest, opts ...grpc.CallOption) (*tarificationv1.UsageCounter, error) {
	return nil, nil
}

func (m *mockTarificationClient) ListUsageCounters(ctx context.Context, in *tarificationv1.ListUsageCountersRequest, opts ...grpc.CallOption) (*tarificationv1.ListUsageCountersResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.ListUsageCountersResponse), args.Error(1)
}

func (m *mockTarificationClient) TarifyLookup(ctx context.Context, in *tarificationv1.TarifyLookupRequest, opts ...grpc.CallOption) (*tarificationv1.TarifyLookupResponse, error) {
	return nil, nil
}

// Provider tariff stubs
func (m *mockTarificationClient) CreateProviderTariffPlan(ctx context.Context, in *tarificationv1.CreateProviderTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffPlanProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) GetProviderTariffPlan(ctx context.Context, in *tarificationv1.GetProviderTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffPlanProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) ListProviderTariffPlans(ctx context.Context, in *tarificationv1.ListProviderTariffPlansRequest, opts ...grpc.CallOption) (*tarificationv1.ListProviderTariffPlansResponse, error) {
	return nil, nil
}
func (m *mockTarificationClient) UpdateProviderTariffPlan(ctx context.Context, in *tarificationv1.UpdateProviderTariffPlanRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffPlanProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) CreateProviderTariffPeriod(ctx context.Context, in *tarificationv1.CreateProviderTariffPeriodRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffPeriodProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) CreateProviderTariffTier(ctx context.Context, in *tarificationv1.CreateProviderTariffTierRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffTierProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) UpdateProviderTariffTier(ctx context.Context, in *tarificationv1.UpdateProviderTariffTierRequest, opts ...grpc.CallOption) (*tarificationv1.ProviderTariffTierProto, error) {
	return nil, nil
}
func (m *mockTarificationClient) GetMarginReport(ctx context.Context, in *tarificationv1.MarginReportRequest, opts ...grpc.CallOption) (*tarificationv1.MarginReportResponse, error) {
	return nil, nil
}

func (m *mockTarificationClient) CreateSenderBillingRecord(ctx context.Context, in *tarificationv1.CreateSenderBillingRecordRequest, opts ...grpc.CallOption) (*tarificationv1.CreateSenderBillingRecordResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.CreateSenderBillingRecordResponse), args.Error(1)
}

func (m *mockTarificationClient) ListSenderBillingRecords(ctx context.Context, in *tarificationv1.ListSenderBillingRecordsRequest, opts ...grpc.CallOption) (*tarificationv1.ListSenderBillingRecordsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tarificationv1.ListSenderBillingRecordsResponse), args.Error(1)
}

var _ tarificationv1.TarificationServiceClient = (*mockTarificationClient)(nil)

// --- Tests ---

func TestTarificationHandler(t *testing.T) {
	t.Run("CreateTariffPlan", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("CreateTariffPlan", mock.Anything, mock.MatchedBy(func(req *tarificationv1.CreateTariffPlanRequest) bool {
				return req.OperatorId == "op-1" &&
					req.SenderCategory == "marketing" &&
					req.Strategy == "tiered"
			})).Return(&tarificationv1.TariffPlan{
				Id:             "plan-new-1",
				OperatorId:     "op-1",
				SenderCategory: "marketing",
				Strategy:       "tiered",
				Active:         true,
			}, nil)

			body, _ := json.Marshal(map[string]string{
				"operator_id":     "op-1",
				"sender_category": "marketing",
				"strategy":        "tiered",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/tarification/tariff-plans", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateTariffPlan(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "plan-new-1", resp["tariff_plan_id"])

			client.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/tarification/tariff-plans", bytes.NewReader([]byte("bad")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateTariffPlan(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when gRPC fails", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("CreateTariffPlan", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))

			body, _ := json.Marshal(map[string]string{
				"operator_id":     "op-1",
				"sender_category": "marketing",
				"strategy":        "tiered",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/tarification/tariff-plans", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateTariffPlan(rr, req)

			assert.Equal(t, http.StatusInternalServerError, rr.Code)
		})
	})

	t.Run("ListTariffPlans", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("ListTariffPlans", mock.Anything, mock.MatchedBy(func(req *tarificationv1.ListTariffPlansRequest) bool {
				return req.OperatorId == "op-1" && req.ActiveOnly == true
			})).Return(&tarificationv1.ListTariffPlansResponse{
				Plans: []*tarificationv1.TariffPlan{
					{
						Id:             "plan-1",
						OperatorId:     "op-1",
						SenderCategory: "transactional",
						Strategy:       "flat",
						Active:         true,
					},
				},
				Total: 1,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/tarification/tariff-plans?operator_id=op-1&active_only=true", nil)

			rr := httptest.NewRecorder()
			handler.ListTariffPlans(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			plans := resp["tariff_plans"].([]interface{})
			assert.Len(t, plans, 1)

			client.AssertExpectations(t)
		})

		t.Run("returns error when service fails", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("ListTariffPlans", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "service unavailable"))

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/tarification/tariff-plans", nil)

			rr := httptest.NewRecorder()
			handler.ListTariffPlans(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("UpdateTariffPlan", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("UpdateTariffPlan", mock.Anything, mock.MatchedBy(func(req *tarificationv1.UpdateTariffPlanRequest) bool {
				return req.Id == "plan-1" && req.Active == false
			})).Return(&tarificationv1.TariffPlan{
				Id:     "plan-1",
				Active: false,
			}, nil)

			body, _ := json.Marshal(map[string]interface{}{
				"active": false,
			})

			req := httptest.NewRequest(http.MethodPut, "/admin/v1/tarification/tariff-plans/plan-1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "plan-1"})

			rr := httptest.NewRecorder()
			handler.UpdateTariffPlan(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			client.AssertExpectations(t)
		})
	})

	t.Run("ListUsageCounters", func(t *testing.T) {
		t.Run("returns 400 when client_id is missing", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/tarification/usage", nil)

			rr := httptest.NewRecorder()
			handler.ListUsageCounters(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("success", func(t *testing.T) {
			client := new(mockTarificationClient)
			handler := NewTarificationHandler(client, nil)

			client.On("ListUsageCounters", mock.Anything, mock.MatchedBy(func(req *tarificationv1.ListUsageCountersRequest) bool {
				return req.ClientId == "client-1"
			})).Return(&tarificationv1.ListUsageCountersResponse{}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/tarification/usage?client_id=client-1", nil)

			rr := httptest.NewRecorder()
			handler.ListUsageCounters(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			client.AssertExpectations(t)
		})
	})
}
