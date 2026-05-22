package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/smpp-server/smpp-server/api/proto/clientv1"
)

// --- Mock ClientServiceClient ---

type mockClientClient struct {
	mock.Mock
}

func (m *mockClientClient) CreateClient(ctx context.Context, in *clientv1.CreateClientRequest, opts ...grpc.CallOption) (*clientv1.CreateClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.CreateClientResponse), args.Error(1)
}

func (m *mockClientClient) UpdateClient(ctx context.Context, in *clientv1.UpdateClientRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clientv1.UpdateClientResponse), args.Error(1)
}

func (m *mockClientClient) GetClient(ctx context.Context, in *clientv1.GetClientRequest, opts ...grpc.CallOption) (*clientv1.GetClientResponse, error) {
	return nil, nil
}
func (m *mockClientClient) ListClients(ctx context.Context, in *clientv1.ListClientsRequest, opts ...grpc.CallOption) (*clientv1.ListClientsResponse, error) {
	return nil, nil
}
func (m *mockClientClient) DeleteClient(ctx context.Context, in *clientv1.DeleteClientRequest, opts ...grpc.CallOption) (*clientv1.DeleteClientResponse, error) {
	return nil, nil
}
func (m *mockClientClient) GetClientConfig(ctx context.Context, in *clientv1.GetClientConfigRequest, opts ...grpc.CallOption) (*clientv1.GetClientConfigResponse, error) {
	return nil, nil
}
func (m *mockClientClient) UpdateClientConfig(ctx context.Context, in *clientv1.UpdateClientConfigRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientConfigResponse, error) {
	return nil, nil
}
func (m *mockClientClient) UpdateClientRateLimits(ctx context.Context, in *clientv1.UpdateClientRateLimitsRequest, opts ...grpc.CallOption) (*clientv1.UpdateClientRateLimitsResponse, error) {
	return nil, nil
}
func (m *mockClientClient) CreateSubAccount(ctx context.Context, in *clientv1.CreateSubAccountRequest, opts ...grpc.CallOption) (*clientv1.CreateSubAccountResponse, error) {
	return nil, nil
}
func (m *mockClientClient) ListSubAccounts(ctx context.Context, in *clientv1.ListSubAccountsRequest, opts ...grpc.CallOption) (*clientv1.ListSubAccountsResponse, error) {
	return nil, nil
}
func (m *mockClientClient) GetSubAccount(ctx context.Context, in *clientv1.GetSubAccountRequest, opts ...grpc.CallOption) (*clientv1.GetSubAccountResponse, error) {
	return nil, nil
}
func (m *mockClientClient) DeleteSubAccount(ctx context.Context, in *clientv1.DeleteSubAccountRequest, opts ...grpc.CallOption) (*clientv1.DeleteSubAccountResponse, error) {
	return nil, nil
}
func (m *mockClientClient) UpdateSubAccountLimits(ctx context.Context, in *clientv1.UpdateSubAccountLimitsRequest, opts ...grpc.CallOption) (*clientv1.UpdateSubAccountLimitsResponse, error) {
	return nil, nil
}
func (m *mockClientClient) ListPlans(ctx context.Context, in *clientv1.ListPlansRequest, opts ...grpc.CallOption) (*clientv1.ListPlansResponse, error) {
	return nil, nil
}
func (m *mockClientClient) AssignPlan(ctx context.Context, in *clientv1.AssignPlanRequest, opts ...grpc.CallOption) (*clientv1.AssignPlanResponse, error) {
	return nil, nil
}
func (m *mockClientClient) ToggleSandbox(ctx context.Context, in *clientv1.ToggleSandboxRequest, opts ...grpc.CallOption) (*clientv1.ToggleSandboxResponse, error) {
	return nil, nil
}
func (m *mockClientClient) IncrementMonthlySMSUsage(ctx context.Context, in *clientv1.IncrementMonthlySMSUsageRequest, opts ...grpc.CallOption) (*clientv1.IncrementMonthlySMSUsageResponse, error) {
	return nil, nil
}

// --- Tests ---

// callUpdateClient выполняет PATCH /admin/v1/clients/:id с указанным body
// и возвращает captured grpcReq для проверки.
func callUpdateClient(t *testing.T, client *mockClientClient, body []byte, clientID string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewClientHandlers(client)

	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/clients/"+clientID, bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": clientID})
	rec := httptest.NewRecorder()

	h.UpdateClient(rec, req)
	return rec
}

// TestAdminUpdateClient_NilFieldsPreserveAsNil — regression-guard для D.1 / housekeeping
// gap: handler должен пробрасывать optional bool/int32 поля как pointer-style,
// nil → nil (не уплощать в zero-value). Без этого PATCH без active молча
// дезактивирует клиента (housekeeping #2), без is_reseller — сбрасывает флаг.
func TestAdminUpdateClient_NilFieldsPreserveAsNil(t *testing.T) {
	client := new(mockClientClient)

	var capturedReq *clientv1.UpdateClientRequest
	client.On("UpdateClient", mock.Anything, mock.MatchedBy(func(req *clientv1.UpdateClientRequest) bool {
		capturedReq = req
		return true
	})).Return(&clientv1.UpdateClientResponse{Success: true}, nil)

	body := []byte(`{"name":"NewName"}`)
	rec := callUpdateClient(t, client, body, "client-123")

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedReq)
	assert.Nil(t, capturedReq.Active, "Active без передачи в JSON должен остаться nil")
	assert.Nil(t, capturedReq.IsReseller, "IsReseller без передачи должен остаться nil")
	assert.Nil(t, capturedReq.MaxSubAccounts, "MaxSubAccounts без передачи должен остаться nil")
	assert.Equal(t, "NewName", capturedReq.Name)
	assert.Equal(t, "client-123", capturedReq.ClientId)
}

// TestAdminUpdateClient_OptionalFieldsExplicitFalse — после housekeeping #2
// (active как optional bool) PATCH с active=false должен передать &false,
// чтобы gRPC server явно дезактивировал клиента.
func TestAdminUpdateClient_OptionalFieldsExplicitFalse(t *testing.T) {
	client := new(mockClientClient)

	var capturedReq *clientv1.UpdateClientRequest
	client.On("UpdateClient", mock.Anything, mock.MatchedBy(func(req *clientv1.UpdateClientRequest) bool {
		capturedReq = req
		return true
	})).Return(&clientv1.UpdateClientResponse{Success: true}, nil)

	body := []byte(`{"active":false,"is_reseller":false,"max_sub_accounts":0}`)
	rec := callUpdateClient(t, client, body, "client-456")

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedReq)
	require.NotNil(t, capturedReq.Active)
	assert.False(t, *capturedReq.Active, "Active=false должен пробрасываться явно (не nil)")
	require.NotNil(t, capturedReq.IsReseller)
	assert.False(t, *capturedReq.IsReseller)
	require.NotNil(t, capturedReq.MaxSubAccounts)
	assert.Equal(t, int32(0), *capturedReq.MaxSubAccounts)
}

// TestAdminUpdateClient_OptionalFieldsTrue — non-nil &true / &10 проброс.
func TestAdminUpdateClient_OptionalFieldsTrue(t *testing.T) {
	client := new(mockClientClient)

	var capturedReq *clientv1.UpdateClientRequest
	client.On("UpdateClient", mock.Anything, mock.MatchedBy(func(req *clientv1.UpdateClientRequest) bool {
		capturedReq = req
		return true
	})).Return(&clientv1.UpdateClientResponse{Success: true}, nil)

	body := []byte(`{"active":true,"is_reseller":true,"max_sub_accounts":10}`)
	rec := callUpdateClient(t, client, body, "client-789")

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedReq.Active)
	assert.True(t, *capturedReq.Active)
	require.NotNil(t, capturedReq.IsReseller)
	assert.True(t, *capturedReq.IsReseller)
	require.NotNil(t, capturedReq.MaxSubAccounts)
	assert.Equal(t, int32(10), *capturedReq.MaxSubAccounts)
}

// TestAdminUpdateClient_InvalidJSON_400 — malformed body даёт 400.
func TestAdminUpdateClient_InvalidJSON_400(t *testing.T) {
	client := new(mockClientClient)

	body := []byte(`not-json`)
	rec := callUpdateClient(t, client, body, "client-x")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	client.AssertNotCalled(t, "UpdateClient", mock.Anything, mock.Anything)
}

// TestAdminUpdateClient_GRPCNotFound_404 — gRPC NotFound маппится в HTTP 404
// через respondGRPCError.
func TestAdminUpdateClient_GRPCNotFound_404(t *testing.T) {
	client := new(mockClientClient)
	client.On("UpdateClient", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.NotFound, "client not found"))

	body := []byte(`{"name":"X"}`)
	rec := callUpdateClient(t, client, body, "missing-id")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestAdminUpdateClient_GRPCInvalidArgument_400 — gRPC InvalidArgument → 400.
// Покрывает D.1+housekeeping #3 (chk_reseller_is_top_level + reseller-без-слотов).
func TestAdminUpdateClient_GRPCInvalidArgument_400(t *testing.T) {
	client := new(mockClientClient)
	client.On("UpdateClient", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.InvalidArgument, "invalid client data"))

	body := []byte(`{"is_reseller":true,"max_sub_accounts":0}`)
	rec := callUpdateClient(t, client, body, "client-bad-state")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- CreateClient sanity test ---

// TestAdminCreateClient_HandlerValidationRejectsResellerWithoutSlots — handler
// short-circuit'ит business rule валидацию ДО вызова gRPC (D.1 paranoid дублирование).
func TestAdminCreateClient_HandlerValidationRejectsResellerWithoutSlots(t *testing.T) {
	client := new(mockClientClient)

	body := []byte(`{"name":"X","email":"x@example.com","is_reseller":true,"max_sub_accounts":0}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/clients", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h := NewClientHandlers(client)
	h.CreateClient(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	client.AssertNotCalled(t, "CreateClient", mock.Anything, mock.Anything)

	// Body должен содержать сообщение про реселлер.
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, fmt.Sprint(resp), "реселлер")
}
