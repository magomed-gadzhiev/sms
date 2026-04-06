package handlers

import (
	"bytes"
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

	clientv1 "github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// --- Mock ClientServiceClient ---

type mockClientServiceClient struct {
	mock.Mock
}

func (m *mockClientServiceClient) CreateClient(ctx context.Context, in *clientv1.CreateClientRequest, opts ...grpc.CallOption) (*clientv1.CreateClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.CreateClientResponse), args.Error(1)
}

func (m *mockClientServiceClient) UpdateClient(ctx context.Context, in *clientv1.UpdateClientRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.UpdateClientResponse), args.Error(1)
}

func (m *mockClientServiceClient) GetClient(ctx context.Context, in *clientv1.GetClientRequest, opts ...grpc.CallOption) (*clientv1.GetClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.GetClientResponse), args.Error(1)
}

func (m *mockClientServiceClient) ListClients(ctx context.Context, in *clientv1.ListClientsRequest, opts ...grpc.CallOption) (*clientv1.ListClientsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.ListClientsResponse), args.Error(1)
}

func (m *mockClientServiceClient) DeleteClient(ctx context.Context, in *clientv1.DeleteClientRequest, opts ...grpc.CallOption) (*clientv1.DeleteClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.DeleteClientResponse), args.Error(1)
}

func (m *mockClientServiceClient) GetClientConfig(ctx context.Context, in *clientv1.GetClientConfigRequest, opts ...grpc.CallOption) (*clientv1.GetClientConfigResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.GetClientConfigResponse), args.Error(1)
}

func (m *mockClientServiceClient) UpdateClientConfig(ctx context.Context, in *clientv1.UpdateClientConfigRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientConfigResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.UpdateClientConfigResponse), args.Error(1)
}

func (m *mockClientServiceClient) UpdateClientRateLimits(ctx context.Context, in *clientv1.UpdateClientRateLimitsRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientRateLimitsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.UpdateClientRateLimitsResponse), args.Error(1)
}

func (m *mockClientServiceClient) CreateSubAccount(ctx context.Context, in *clientv1.CreateSubAccountRequest, opts ...grpc.CallOption) (*clientv1.CreateSubAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.CreateSubAccountResponse), args.Error(1)
}

func (m *mockClientServiceClient) ListSubAccounts(ctx context.Context, in *clientv1.ListSubAccountsRequest, opts ...grpc.CallOption) (*clientv1.ListSubAccountsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.ListSubAccountsResponse), args.Error(1)
}

func (m *mockClientServiceClient) GetSubAccount(ctx context.Context, in *clientv1.GetSubAccountRequest, opts ...grpc.CallOption) (*clientv1.GetSubAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.GetSubAccountResponse), args.Error(1)
}

func (m *mockClientServiceClient) DeleteSubAccount(ctx context.Context, in *clientv1.DeleteSubAccountRequest, opts ...grpc.CallOption) (*clientv1.DeleteSubAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.DeleteSubAccountResponse), args.Error(1)
}

func (m *mockClientServiceClient) UpdateSubAccountLimits(ctx context.Context, in *clientv1.UpdateSubAccountLimitsRequest, opts ...grpc.CallOption) (*clientv1.UpdateSubAccountLimitsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.UpdateSubAccountLimitsResponse), args.Error(1)
}

func (m *mockClientServiceClient) ListPlans(ctx context.Context, in *clientv1.ListPlansRequest, opts ...grpc.CallOption) (*clientv1.ListPlansResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.ListPlansResponse), args.Error(1)
}

func (m *mockClientServiceClient) AssignPlan(ctx context.Context, in *clientv1.AssignPlanRequest, opts ...grpc.CallOption) (*clientv1.AssignPlanResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.AssignPlanResponse), args.Error(1)
}

func (m *mockClientServiceClient) ToggleSandbox(ctx context.Context, in *clientv1.ToggleSandboxRequest, opts ...grpc.CallOption) (*clientv1.ToggleSandboxResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.ToggleSandboxResponse), args.Error(1)
}

// --- helpers ---

func newTariffHandlersWithMocks(cc *mockClientServiceClient) *TariffHandlers {
	return NewTariffHandlers(cc, nil, nil)
}

func withClientID(r *http.Request, clientID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClientIDKey, clientID)
	return r.WithContext(ctx)
}

// --- Tests ---

func TestChangePlan_Success(t *testing.T) {
	clientID := uuid.New()
	planID := uuid.New().String()

	cc := &mockClientServiceClient{}
	cc.On("AssignPlan", mock.Anything, &clientv1.AssignPlanRequest{
		ClientId: clientID.String(),
		PlanId:   planID,
	}).Return(&clientv1.AssignPlanResponse{}, nil)

	h := newTariffHandlersWithMocks(cc)

	body, _ := json.Marshal(changePlanRequest{PlanID: planID})
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/tariffs/change", bytes.NewReader(body))
	req = withClientID(req, clientID)
	w := httptest.NewRecorder()

	h.ChangePlan(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "успешно")
	cc.AssertExpectations(t)
}

func TestChangePlan_EmptyPlanID(t *testing.T) {
	clientID := uuid.New()

	cc := &mockClientServiceClient{}
	h := newTariffHandlersWithMocks(cc)

	body, _ := json.Marshal(changePlanRequest{PlanID: ""})
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/tariffs/change", bytes.NewReader(body))
	req = withClientID(req, clientID)
	w := httptest.NewRecorder()

	h.ChangePlan(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj, ok := resp["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, errObj["message"], "plan_id")
}

func TestChangePlan_InvalidJSON(t *testing.T) {
	clientID := uuid.New()

	cc := &mockClientServiceClient{}
	h := newTariffHandlersWithMocks(cc)

	req := httptest.NewRequest(http.MethodPost, "/portal/v1/tariffs/change", bytes.NewReader([]byte("not json")))
	req = withClientID(req, clientID)
	w := httptest.NewRecorder()

	h.ChangePlan(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangePlan_GRPCError(t *testing.T) {
	clientID := uuid.New()
	planID := uuid.New().String()

	cc := &mockClientServiceClient{}
	cc.On("AssignPlan", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.NotFound, "plan not found"))

	h := newTariffHandlersWithMocks(cc)

	body, _ := json.Marshal(changePlanRequest{PlanID: planID})
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/tariffs/change", bytes.NewReader(body))
	req = withClientID(req, clientID)
	w := httptest.NewRecorder()

	h.ChangePlan(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	cc.AssertExpectations(t)
}

func TestChangePlan_Unauthorized(t *testing.T) {
	cc := &mockClientServiceClient{}
	h := newTariffHandlersWithMocks(cc)

	body, _ := json.Marshal(changePlanRequest{PlanID: "some-plan"})
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/tariffs/change", bytes.NewReader(body))
	// No clientID set in context
	w := httptest.NewRecorder()

	h.ChangePlan(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAvailablePlans_Success(t *testing.T) {
	clientID := uuid.New()

	cc := &mockClientServiceClient{}
	cc.On("ListPlans", mock.Anything, &clientv1.ListPlansRequest{}).
		Return(&clientv1.ListPlansResponse{
			Plans: []*clientv1.SubscriptionPlan{
				{
					Id:                "plan-001",
					Name:              "free",
					DisplayName:       "Free / Trial",
					MonthlyPriceRub:   0,
					MaxSmsPerMonth:    1000,
					MaxSmppConnections: 1,
					MaxUsers:          1,
					Features:          map[string]bool{},
				},
				{
					Id:                "plan-002",
					Name:              "starter",
					DisplayName:       "Starter",
					MonthlyPriceRub:   5000,
					MaxSmsPerMonth:    50000,
					MaxSmppConnections: 1,
					MaxUsers:          1,
					Features:          map[string]bool{"analytics": true},
				},
			},
		}, nil)

	h := newTariffHandlersWithMocks(cc)

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/tariffs/plans", nil)
	req = withClientID(req, clientID)
	w := httptest.NewRecorder()

	h.ListAvailablePlans(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	plans, ok := resp["plans"].([]interface{})
	require.True(t, ok)
	assert.Len(t, plans, 2)

	// Verify plan has id field (critical for BUG-01 fix)
	firstPlan := plans[0].(map[string]interface{})
	assert.Equal(t, "plan-001", firstPlan["id"])
	assert.Equal(t, "free", firstPlan["name"])
	assert.Equal(t, "Free / Trial", firstPlan["display_name"])

	cc.AssertExpectations(t)
}
