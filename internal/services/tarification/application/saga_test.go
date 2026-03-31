package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
)

// mockBillingClient реализует billingv1.BillingServiceClient для тестов
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

func (m *mockBillingClient) FreezeAccount(ctx context.Context, in *billingv1.FreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.FreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.FreezeAccountResponse), args.Error(1)
}

func (m *mockBillingClient) UnfreezeAccount(ctx context.Context, in *billingv1.UnfreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.UnfreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.UnfreezeAccountResponse), args.Error(1)
}

func (m *mockBillingClient) SetCreditLimit(ctx context.Context, in *billingv1.SetCreditLimitRequest, opts ...grpc.CallOption) (*billingv1.SetCreditLimitResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.SetCreditLimitResponse), args.Error(1)
}

func (m *mockBillingClient) SetLowBalanceThreshold(ctx context.Context, in *billingv1.SetLowBalanceThresholdRequest, opts ...grpc.CallOption) (*billingv1.SetLowBalanceThresholdResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.SetLowBalanceThresholdResponse), args.Error(1)
}

func (m *mockBillingClient) ListBalances(ctx context.Context, in *billingv1.ListBalancesRequest, opts ...grpc.CallOption) (*billingv1.ListBalancesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.ListBalancesResponse), args.Error(1)
}

func TestSagaOrchestrator(t *testing.T) {
	t.Run("Charge", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("ChargeMessage", mock.Anything, &billingv1.ChargeMessageRequest{
				ClientId:    "client-1",
				MessageId:   "msg-1",
				Amount:      "0.500000",
				Currency:    "RUB",
				Description: "test charge",
			}).Return(&billingv1.ChargeMessageResponse{
				TransactionId: "tx-1",
				NewBalance:    "99.500000",
				Success:       true,
			}, nil)

			result, err := saga.Charge(context.Background(), "client-1", "msg-1", "0.500000", "RUB", "test charge", 1)

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "tx-1", result.TransactionID)
			assert.Equal(t, "99.500000", result.NewBalance)
			bc.AssertExpectations(t)
		})

		t.Run("billing_error", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("ChargeMessage", mock.Anything, mock.Anything).
				Return(nil, errors.New("connection refused"))

			result, err := saga.Charge(context.Background(), "client-1", "msg-1", "1.000000", "", "test", 1)

			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "billing charge failed")
			bc.AssertExpectations(t)
		})

		t.Run("insufficient_balance", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("ChargeMessage", mock.Anything, mock.Anything).
				Return(&billingv1.ChargeMessageResponse{
					Success: false,
					Error:   "insufficient balance",
				}, nil)

			result, err := saga.Charge(context.Background(), "client-1", "msg-1", "1000.000000", "", "test", 1)

			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Equal(t, "insufficient balance", result.Error)
			bc.AssertExpectations(t)
		})
	})

	t.Run("Refund", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("AddCredits", mock.Anything, &billingv1.AddCreditsRequest{
				ClientId:    "client-1",
				Amount:      "5.000000",
				Currency:    "RUB",
				Description: "refund",
			}).Return(&billingv1.AddCreditsResponse{
				TransactionId: "tx-refund-1",
				NewBalance:    "105.000000",
				Success:       true,
			}, nil)

			result, err := saga.Refund(context.Background(), "client-1", "5.000000", "RUB", "refund")

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "tx-refund-1", result.TransactionID)
			bc.AssertExpectations(t)
		})
	})

	t.Run("HandleRecalc", func(t *testing.T) {
		t.Run("positive_amount_deducts", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("DeductCredits", mock.Anything, &billingv1.DeductCreditsRequest{
				ClientId:    "client-1",
				Amount:      "2.500000",
				Currency:    "RUB",
				Description: "threshold recalculation charge",
			}).Return(&billingv1.DeductCreditsResponse{
				TransactionId: "tx-recalc-1",
				NewBalance:    "97.500000",
				Success:       true,
			}, nil)

			result, err := saga.HandleRecalc(context.Background(), "client-1", "2.500000", "RUB")

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "tx-recalc-1", result.TransactionID)
			bc.AssertExpectations(t)
		})

		t.Run("negative_amount_refunds", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("AddCredits", mock.Anything, &billingv1.AddCreditsRequest{
				ClientId:    "client-1",
				Amount:      "1.500000",
				Currency:    "RUB",
				Description: "threshold recalculation refund",
			}).Return(&billingv1.AddCreditsResponse{
				TransactionId: "tx-refund-1",
				NewBalance:    "101.500000",
				Success:       true,
			}, nil)

			result, err := saga.HandleRecalc(context.Background(), "client-1", "-1.500000", "RUB")

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "tx-refund-1", result.TransactionID)
			bc.AssertExpectations(t)
		})

		t.Run("zero_amount_noop", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			result, err := saga.HandleRecalc(context.Background(), "client-1", "0.000000", "RUB")

			require.NoError(t, err)
			assert.True(t, result.Success)
			// Billing не должен вызываться
			bc.AssertNotCalled(t, "DeductCredits", mock.Anything, mock.Anything)
			bc.AssertNotCalled(t, "AddCredits", mock.Anything, mock.Anything)
		})

		t.Run("deduct_insufficient_balance_records_debt", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			bc.On("DeductCredits", mock.Anything, mock.Anything).
				Return(&billingv1.DeductCreditsResponse{
					Success: false,
					Error:   "insufficient balance",
				}, nil)

			result, err := saga.HandleRecalc(context.Background(), "client-1", "100.000000", "RUB")

			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Contains(t, result.Error, "debt")
			bc.AssertExpectations(t)
		})

		t.Run("invalid_amount_returns_error", func(t *testing.T) {
			bc := new(mockBillingClient)
			saga := NewSagaOrchestrator(bc)

			result, err := saga.HandleRecalc(context.Background(), "client-1", "not-a-number", "RUB")

			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid recalc amount")
		})
	})
}
