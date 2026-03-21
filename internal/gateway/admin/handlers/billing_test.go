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
	"google.golang.org/protobuf/types/known/timestamppb"

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
	return nil, nil
}

func (m *mockBillingClient) AddCredits(ctx context.Context, in *billingv1.AddCreditsRequest, opts ...grpc.CallOption) (*billingv1.AddCreditsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.AddCreditsResponse), args.Error(1)
}

func (m *mockBillingClient) DeductCredits(ctx context.Context, in *billingv1.DeductCreditsRequest, opts ...grpc.CallOption) (*billingv1.DeductCreditsResponse, error) {
	return nil, nil
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
	return nil, nil
}

var _ billingv1.BillingServiceClient = (*mockBillingClient)(nil)

// --- Tests ---

func TestBillingHandlers(t *testing.T) {
	t.Run("GetBalance", func(t *testing.T) {
		t.Run("success with path variable", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("GetBalance", mock.Anything, mock.MatchedBy(func(req *billingv1.GetBalanceRequest) bool {
				return req.ClientId == "client-abc"
			})).Return(&billingv1.GetBalanceResponse{
				ClientId:  "client-abc",
				Balance:   "500.00",
				Currency:  "USD",
				UpdatedAt: timestamppb.Now(),
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/clients/client-abc/balance", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "client-abc"})

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp BalanceResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "client-abc", resp.ClientID)
			assert.Equal(t, "500.00", resp.Balance)
			assert.Equal(t, "USD", resp.Currency)

			client.AssertExpectations(t)
		})

		t.Run("success with query parameter fallback", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("GetBalance", mock.Anything, mock.MatchedBy(func(req *billingv1.GetBalanceRequest) bool {
				return req.ClientId == "client-xyz"
			})).Return(&billingv1.GetBalanceResponse{
				ClientId:  "client-xyz",
				Balance:   "200.00",
				Currency:  "EUR",
				UpdatedAt: timestamppb.Now(),
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/balance?client_id=client-xyz", nil)
			req = mux.SetURLVars(req, map[string]string{})

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 when no client_id", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/balance", nil)
			req = mux.SetURLVars(req, map[string]string{})

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when billing service fails", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("GetBalance", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.NotFound, "client not found"))

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/clients/unknown/balance", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "unknown"})

			rr := httptest.NewRecorder()
			handler.GetBalance(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	})

	t.Run("AddCredits", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("AddCredits", mock.Anything, mock.MatchedBy(func(req *billingv1.AddCreditsRequest) bool {
				return req.ClientId == "client-abc" &&
					req.Amount == "100.00" &&
					req.Currency == "USD"
			})).Return(&billingv1.AddCreditsResponse{
				TransactionId: "tx-123",
				NewBalance:    "600.00",
				Success:       true,
			}, nil)

			body, _ := json.Marshal(AddCreditsRequest{
				Amount:      "100.00",
				Currency:    "USD",
				Description: "Manual top-up",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/billing/clients/client-abc/credits", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "client-abc"})

			rr := httptest.NewRecorder()
			handler.AddCredits(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp AddCreditsResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "tx-123", resp.TransactionID)
			assert.Equal(t, "600.00", resp.NewBalance)
			assert.True(t, resp.Success)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 when amount is empty", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			body, _ := json.Marshal(AddCreditsRequest{
				Amount:   "",
				Currency: "USD",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/billing/clients/client-abc/credits", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "client-abc"})

			rr := httptest.NewRecorder()
			handler.AddCredits(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("CreatePricingRule", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("CreatePricingRule", mock.Anything, mock.MatchedBy(func(req *billingv1.CreatePricingRuleRequest) bool {
				return req.DestinationPattern == "^\\+7" && req.PricePerMessage == "0.05"
			})).Return(&billingv1.CreatePricingRuleResponse{
				RuleId:    "rule-new-1",
				CreatedAt: timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(CreatePricingRuleRequest{
				DestinationPattern: "^\\+7",
				PricePerMessage:    "0.05",
				Currency:           "USD",
				Priority:           10,
				Active:             true,
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/billing/pricing-rules", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreatePricingRule(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp CreatePricingRuleResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "rule-new-1", resp.RuleID)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 when destination_pattern is empty", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			body, _ := json.Marshal(CreatePricingRuleRequest{
				DestinationPattern: "",
				PricePerMessage:    "0.05",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/billing/pricing-rules", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreatePricingRule(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when price_per_message is empty", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			body, _ := json.Marshal(CreatePricingRuleRequest{
				DestinationPattern: "^\\+7",
				PricePerMessage:    "",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/billing/pricing-rules", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreatePricingRule(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("GetTransactionHistory", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			client.On("GetTransactionHistory", mock.Anything, mock.MatchedBy(func(req *billingv1.GetTransactionHistoryRequest) bool {
				return req.ClientId == "client-abc"
			})).Return(&billingv1.GetTransactionHistoryResponse{
				Transactions: []*billingv1.Transaction{
					{
						TransactionId: "tx-1",
						ClientId:      "client-abc",
						Type:          "credit",
						Amount:        "100.00",
						Currency:      "USD",
						BalanceBefore: "500.00",
						BalanceAfter:  "600.00",
						CreatedAt:     timestamppb.Now(),
					},
				},
				Total: 1,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/transactions?client_id=client-abc", nil)

			rr := httptest.NewRecorder()
			handler.GetTransactionHistory(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp TransactionHistoryResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Len(t, resp.Transactions, 1)
			assert.Equal(t, "tx-1", resp.Transactions[0].TransactionID)
			assert.Equal(t, 1, resp.Total)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 when client_id is missing", func(t *testing.T) {
			client := new(mockBillingClient)
			handler := NewBillingHandlers(client)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/billing/transactions", nil)

			rr := httptest.NewRecorder()
			handler.GetTransactionHistory(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})
}
