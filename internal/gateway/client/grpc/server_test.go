package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
)

// ─── Mock clients ───────────────────────────────────────────────────────────

type mockMessagingClient struct {
	mock.Mock
}

func (m *mockMessagingClient) SendMessage(ctx context.Context, in *messagingv1.SendMessageRequest, opts ...grpc.CallOption) (*messagingv1.SendMessageResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.SendMessageResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) SendBatch(ctx context.Context, in *messagingv1.SendBatchRequest, opts ...grpc.CallOption) (*messagingv1.SendBatchResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.SendBatchResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) GetMessageStatus(ctx context.Context, in *messagingv1.GetMessageStatusRequest, opts ...grpc.CallOption) (*messagingv1.GetMessageStatusResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.GetMessageStatusResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) GetMessageHistory(ctx context.Context, in *messagingv1.GetMessageHistoryRequest, opts ...grpc.CallOption) (*messagingv1.GetMessageHistoryResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.GetMessageHistoryResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) ProcessDLR(ctx context.Context, in *messagingv1.ProcessDLRRequest, opts ...grpc.CallOption) (*messagingv1.ProcessDLRResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.ProcessDLRResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) CancelMessage(ctx context.Context, in *messagingv1.CancelMessageRequest, opts ...grpc.CallOption) (*messagingv1.CancelMessageResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.CancelMessageResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessagingClient) ListScheduledMessages(ctx context.Context, in *messagingv1.ListScheduledMessagesRequest, opts ...grpc.CallOption) (*messagingv1.ListScheduledMessagesResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*messagingv1.ListScheduledMessagesResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockBillingClient struct {
	mock.Mock
}

func (m *mockBillingClient) GetBalance(ctx context.Context, in *billingv1.GetBalanceRequest, opts ...grpc.CallOption) (*billingv1.GetBalanceResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.GetBalanceResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) ChargeMessage(ctx context.Context, in *billingv1.ChargeMessageRequest, opts ...grpc.CallOption) (*billingv1.ChargeMessageResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.ChargeMessageResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) ChargeMessageDual(ctx context.Context, in *billingv1.ChargeMessageDualRequest, opts ...grpc.CallOption) (*billingv1.ChargeMessageDualResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.ChargeMessageDualResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) AddCredits(ctx context.Context, in *billingv1.AddCreditsRequest, opts ...grpc.CallOption) (*billingv1.AddCreditsResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.AddCreditsResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) DeductCredits(ctx context.Context, in *billingv1.DeductCreditsRequest, opts ...grpc.CallOption) (*billingv1.DeductCreditsResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.DeductCreditsResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) GetTransactionHistory(ctx context.Context, in *billingv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*billingv1.GetTransactionHistoryResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.GetTransactionHistoryResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) GetPricingRules(ctx context.Context, in *billingv1.GetPricingRulesRequest, opts ...grpc.CallOption) (*billingv1.GetPricingRulesResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.GetPricingRulesResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) CreatePricingRule(ctx context.Context, in *billingv1.CreatePricingRuleRequest, opts ...grpc.CallOption) (*billingv1.CreatePricingRuleResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.CreatePricingRuleResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) TransferBalance(ctx context.Context, in *billingv1.TransferBalanceRequest, opts ...grpc.CallOption) (*billingv1.TransferBalanceResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.TransferBalanceResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) FreezeAccount(ctx context.Context, in *billingv1.FreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.FreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.FreezeAccountResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) UnfreezeAccount(ctx context.Context, in *billingv1.UnfreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.UnfreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.UnfreezeAccountResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) SetCreditLimit(ctx context.Context, in *billingv1.SetCreditLimitRequest, opts ...grpc.CallOption) (*billingv1.SetCreditLimitResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.SetCreditLimitResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) SetLowBalanceThreshold(ctx context.Context, in *billingv1.SetLowBalanceThresholdRequest, opts ...grpc.CallOption) (*billingv1.SetLowBalanceThresholdResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.SetLowBalanceThresholdResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBillingClient) ListBalances(ctx context.Context, in *billingv1.ListBalancesRequest, opts ...grpc.CallOption) (*billingv1.ListBalancesResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*billingv1.ListBalancesResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockAnalyticsClient struct {
	mock.Mock
}

func (m *mockAnalyticsClient) GetStatistics(ctx context.Context, in *analyticsv1.GetStatisticsRequest, opts ...grpc.CallOption) (*analyticsv1.GetStatisticsResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*analyticsv1.GetStatisticsResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockAnalyticsClient) GenerateReport(ctx context.Context, in *analyticsv1.GenerateReportRequest, opts ...grpc.CallOption) (*analyticsv1.GenerateReportResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*analyticsv1.GenerateReportResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockAnalyticsClient) GetRealtimeMetrics(ctx context.Context, in *analyticsv1.GetRealtimeMetricsRequest, opts ...grpc.CallOption) (*analyticsv1.GetRealtimeMetricsResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*analyticsv1.GetRealtimeMetricsResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockAnalyticsClient) GetProviderPerformance(ctx context.Context, in *analyticsv1.GetProviderPerformanceRequest, opts ...grpc.CallOption) (*analyticsv1.GetProviderPerformanceResponse, error) {
	args := m.Called(ctx, in)
	if v := args.Get(0); v != nil {
		return v.(*analyticsv1.GetProviderPerformanceResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// helpers ────────────────────────────────────────────────────────────────────

func ctxWithClient(clientID uuid.UUID) context.Context {
	return context.WithValue(context.Background(), ClientIDKey, clientID)
}

func ctxWithoutClient() context.Context {
	return context.Background()
}

func setupServer() (*Server, *mockMessagingClient, *mockBillingClient, *mockAnalyticsClient) {
	mc := &mockMessagingClient{}
	bc := &mockBillingClient{}
	ac := &mockAnalyticsClient{}
	srv := NewServer(mc, bc, ac)
	return srv, mc, bc, ac
}

// ─── SendMessage ────────────────────────────────────────────────────────────

func TestSendMessage_Success(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &messagingv1.SendMessageResponse{MessageId: "msg-1", Status: "queued"}
	mc.On("SendMessage", ctx, mock.MatchedBy(func(req *messagingv1.SendMessageRequest) bool {
		return req.ClientId == clientID.String() && req.Destination == "+1234567890"
	})).Return(expected, nil)

	resp, err := srv.SendMessage(ctx, &messagingv1.SendMessageRequest{
		Destination: "+1234567890",
		Text:        "Hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "msg-1", resp.MessageId)
	assert.Equal(t, "queued", resp.Status)

	mc.AssertExpectations(t)
}

func TestSendMessage_SetsClientIDFromContext(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	mc.On("SendMessage", ctx, mock.MatchedBy(func(req *messagingv1.SendMessageRequest) bool {
		return req.ClientId == clientID.String()
	})).Return(&messagingv1.SendMessageResponse{}, nil)

	// Request without client_id set -- should be filled from context
	_, err := srv.SendMessage(ctx, &messagingv1.SendMessageRequest{
		Destination: "+1",
		Text:        "Hi",
	})
	require.NoError(t, err)

	mc.AssertExpectations(t)
}

func TestSendMessage_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.SendMessage(ctxWithoutClient(), &messagingv1.SendMessageRequest{
		Destination: "+1",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestSendMessage_ClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.SendMessage(ctx, &messagingv1.SendMessageRequest{
		ClientId:    otherID.String(),
		Destination: "+1",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestSendMessage_MatchingClientIDAllowed(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	mc.On("SendMessage", ctx, mock.Anything).Return(&messagingv1.SendMessageResponse{}, nil)

	// Request with matching client_id -- should succeed
	_, err := srv.SendMessage(ctx, &messagingv1.SendMessageRequest{
		ClientId:    clientID.String(),
		Destination: "+1",
	})
	require.NoError(t, err)

	mc.AssertExpectations(t)
}

func TestSendMessage_ProxyError(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	mc.On("SendMessage", ctx, mock.Anything).Return(nil, errors.New("upstream down"))

	_, err := srv.SendMessage(ctx, &messagingv1.SendMessageRequest{Destination: "+1"})
	require.Error(t, err)

	mc.AssertExpectations(t)
}

// ─── SendBatch ──────────────────────────────────────────────────────────────

func TestSendBatch_Success(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &messagingv1.SendBatchResponse{SuccessCount: 2}
	mc.On("SendBatch", ctx, mock.MatchedBy(func(req *messagingv1.SendBatchRequest) bool {
		return req.ClientId == clientID.String() &&
			len(req.Messages) == 2 &&
			req.Messages[0].ClientId == clientID.String() &&
			req.Messages[1].ClientId == clientID.String()
	})).Return(expected, nil)

	resp, err := srv.SendBatch(ctx, &messagingv1.SendBatchRequest{
		Messages: []*messagingv1.SendMessageRequest{
			{Destination: "+1", Text: "A"},
			{Destination: "+2", Text: "B"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.SuccessCount)

	mc.AssertExpectations(t)
}

func TestSendBatch_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.SendBatch(ctxWithoutClient(), &messagingv1.SendBatchRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestSendBatch_ClientIDMismatchInBatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.SendBatch(ctx, &messagingv1.SendBatchRequest{
		ClientId: clientID.String(),
		Messages: []*messagingv1.SendMessageRequest{
			{ClientId: otherID.String(), Destination: "+1"},
		},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestSendBatch_BatchClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.SendBatch(ctx, &messagingv1.SendBatchRequest{
		ClientId: otherID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

// ─── GetMessageStatus ───────────────────────────────────────────────────────

func TestGetMessageStatus_Success(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &messagingv1.GetMessageStatusResponse{
		MessageId: "msg-1",
		Status:    "delivered",
	}
	mc.On("GetMessageStatus", ctx, mock.MatchedBy(func(req *messagingv1.GetMessageStatusRequest) bool {
		return req.ClientId == clientID.String() && req.MessageId == "msg-1"
	})).Return(expected, nil)

	resp, err := srv.GetMessageStatus(ctx, &messagingv1.GetMessageStatusRequest{
		MessageId: "msg-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "delivered", resp.Status)

	mc.AssertExpectations(t)
}

func TestGetMessageStatus_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.GetMessageStatus(ctxWithoutClient(), &messagingv1.GetMessageStatusRequest{
		MessageId: "msg-1",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetMessageStatus_ClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.GetMessageStatus(ctx, &messagingv1.GetMessageStatusRequest{
		MessageId: "msg-1",
		ClientId:  otherID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

// ─── GetMessageHistory ──────────────────────────────────────────────────────

func TestGetMessageHistory_Success(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &messagingv1.GetMessageHistoryResponse{Total: 42}
	mc.On("GetMessageHistory", ctx, mock.MatchedBy(func(req *messagingv1.GetMessageHistoryRequest) bool {
		return req.ClientId == clientID.String()
	})).Return(expected, nil)

	resp, err := srv.GetMessageHistory(ctx, &messagingv1.GetMessageHistoryRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(42), resp.Total)

	mc.AssertExpectations(t)
}

func TestGetMessageHistory_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.GetMessageHistory(ctxWithoutClient(), &messagingv1.GetMessageHistoryRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetMessageHistory_ClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.GetMessageHistory(ctx, &messagingv1.GetMessageHistoryRequest{
		ClientId: otherID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

// ─── GetBalance ─────────────────────────────────────────────────────────────

func TestGetBalance_Success(t *testing.T) {
	srv, _, bc, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &billingv1.GetBalanceResponse{
		ClientId: clientID.String(),
		Balance:  "100.50",
		Currency: "RUB",
	}
	bc.On("GetBalance", ctx, mock.MatchedBy(func(req *billingv1.GetBalanceRequest) bool {
		return req.ClientId == clientID.String()
	})).Return(expected, nil)

	resp, err := srv.GetBalance(ctx, &billingv1.GetBalanceRequest{})
	require.NoError(t, err)
	assert.Equal(t, "100.50", resp.Balance)
	assert.Equal(t, "RUB", resp.Currency)

	bc.AssertExpectations(t)
}

func TestGetBalance_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.GetBalance(ctxWithoutClient(), &billingv1.GetBalanceRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetBalance_ClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.GetBalance(ctx, &billingv1.GetBalanceRequest{
		ClientId: otherID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGetBalance_ProxyError(t *testing.T) {
	srv, _, bc, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	bc.On("GetBalance", ctx, mock.Anything).Return(nil, errors.New("billing unavailable"))

	_, err := srv.GetBalance(ctx, &billingv1.GetBalanceRequest{})
	require.Error(t, err)

	bc.AssertExpectations(t)
}

// ─── ProcessDLR ─────────────────────────────────────────────────────────────

func TestProcessDLR_Success(t *testing.T) {
	srv, mc, _, _ := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &messagingv1.ProcessDLRResponse{Success: true, UpdatedStatus: "delivered"}
	mc.On("ProcessDLR", ctx, mock.MatchedBy(func(req *messagingv1.ProcessDLRRequest) bool {
		return req.MessageId == "msg-1"
	})).Return(expected, nil)

	resp, err := srv.ProcessDLR(ctx, &messagingv1.ProcessDLRRequest{
		MessageId: "msg-1",
		Stat:      "DELIVRD",
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	mc.AssertExpectations(t)
}

func TestProcessDLR_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.ProcessDLR(ctxWithoutClient(), &messagingv1.ProcessDLRRequest{
		MessageId: "msg-1",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

// ─── GetStatistics ──────────────────────────────────────────────────────────

func TestGetStatistics_Success(t *testing.T) {
	srv, _, _, ac := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	expected := &analyticsv1.GetStatisticsResponse{}
	ac.On("GetStatistics", ctx, mock.MatchedBy(func(req *analyticsv1.GetStatisticsRequest) bool {
		return req.ClientId == clientID.String()
	})).Return(expected, nil)

	resp, err := srv.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	ac.AssertExpectations(t)
}

func TestGetStatistics_Unauthenticated(t *testing.T) {
	srv, _, _, _ := setupServer()

	_, err := srv.GetStatistics(ctxWithoutClient(), &analyticsv1.GetStatisticsRequest{})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetStatistics_ClientIDMismatch(t *testing.T) {
	srv, _, _, _ := setupServer()
	clientID := uuid.New()
	otherID := uuid.New()
	ctx := ctxWithClient(clientID)

	_, err := srv.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{
		ClientId: otherID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGetStatistics_ProxyError(t *testing.T) {
	srv, _, _, ac := setupServer()
	clientID := uuid.New()
	ctx := ctxWithClient(clientID)

	ac.On("GetStatistics", ctx, mock.Anything).Return(nil, errors.New("analytics down"))

	_, err := srv.GetStatistics(ctx, &analyticsv1.GetStatisticsRequest{})
	require.Error(t, err)

	ac.AssertExpectations(t)
}

// ─── interceptor_test.go (inline) ──────────────────────────────────────────

// TestAuthInterceptor_LoadTestMode — после fix A7.3 dummy-инжекция
// сохранена только за LOAD_TEST_MODE env-var (см. interceptor_test.go
// для regression coverage реальной ValidateToken-логики).
func TestAuthInterceptor_LoadTestMode(t *testing.T) {
	t.Setenv("LOAD_TEST_MODE", "true")
	interceptor := AuthInterceptor(nil)

	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		cid, ok := GetClientID(ctx)
		require.True(t, ok)
		assert.Equal(t, uuid.MustParse("c0000000-0000-0000-0000-000000000001"), cid)
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, "ok", resp)
}

func TestGetClientID_Present(t *testing.T) {
	expected := uuid.New()
	ctx := context.WithValue(context.Background(), ClientIDKey, expected)

	got, ok := GetClientID(ctx)
	assert.True(t, ok)
	assert.Equal(t, expected, got)
}

func TestGetClientID_Missing(t *testing.T) {
	_, ok := GetClientID(context.Background())
	assert.False(t, ok)
}

func TestGetClientID_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ClientIDKey, "not-a-uuid")

	_, ok := GetClientID(ctx)
	assert.False(t, ok)
}

// ─── NewServer ──────────────────────────────────────────────────────────────

func TestNewServer(t *testing.T) {
	mc := &mockMessagingClient{}
	bc := &mockBillingClient{}
	ac := &mockAnalyticsClient{}

	srv := NewServer(mc, bc, ac)
	assert.NotNil(t, srv)
}
