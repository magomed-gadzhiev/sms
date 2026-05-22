package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	billingv1 "github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/services/tarification/mocks"
)

// testBillingClient реализует billingv1.BillingServiceClient для тестов tarification_service
type testBillingClient struct {
	mock.Mock
}

func (m *testBillingClient) GetBalance(ctx context.Context, in *billingv1.GetBalanceRequest, opts ...grpc.CallOption) (*billingv1.GetBalanceResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetBalanceResponse), args.Error(1)
}

func (m *testBillingClient) ChargeMessage(ctx context.Context, in *billingv1.ChargeMessageRequest, opts ...grpc.CallOption) (*billingv1.ChargeMessageResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.ChargeMessageResponse), args.Error(1)
}

func (m *testBillingClient) ChargeMessageDual(ctx context.Context, in *billingv1.ChargeMessageDualRequest, opts ...grpc.CallOption) (*billingv1.ChargeMessageDualResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.ChargeMessageDualResponse), args.Error(1)
}

func (m *testBillingClient) AddCredits(ctx context.Context, in *billingv1.AddCreditsRequest, opts ...grpc.CallOption) (*billingv1.AddCreditsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.AddCreditsResponse), args.Error(1)
}

func (m *testBillingClient) DeductCredits(ctx context.Context, in *billingv1.DeductCreditsRequest, opts ...grpc.CallOption) (*billingv1.DeductCreditsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.DeductCreditsResponse), args.Error(1)
}

func (m *testBillingClient) GetTransactionHistory(ctx context.Context, in *billingv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*billingv1.GetTransactionHistoryResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetTransactionHistoryResponse), args.Error(1)
}

func (m *testBillingClient) GetPricingRules(ctx context.Context, in *billingv1.GetPricingRulesRequest, opts ...grpc.CallOption) (*billingv1.GetPricingRulesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.GetPricingRulesResponse), args.Error(1)
}

func (m *testBillingClient) CreatePricingRule(ctx context.Context, in *billingv1.CreatePricingRuleRequest, opts ...grpc.CallOption) (*billingv1.CreatePricingRuleResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.CreatePricingRuleResponse), args.Error(1)
}

func (m *testBillingClient) TransferBalance(ctx context.Context, in *billingv1.TransferBalanceRequest, opts ...grpc.CallOption) (*billingv1.TransferBalanceResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.TransferBalanceResponse), args.Error(1)
}

func (m *testBillingClient) FreezeAccount(ctx context.Context, in *billingv1.FreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.FreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.FreezeAccountResponse), args.Error(1)
}

func (m *testBillingClient) UnfreezeAccount(ctx context.Context, in *billingv1.UnfreezeAccountRequest, opts ...grpc.CallOption) (*billingv1.UnfreezeAccountResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.UnfreezeAccountResponse), args.Error(1)
}

func (m *testBillingClient) SetCreditLimit(ctx context.Context, in *billingv1.SetCreditLimitRequest, opts ...grpc.CallOption) (*billingv1.SetCreditLimitResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.SetCreditLimitResponse), args.Error(1)
}

func (m *testBillingClient) SetLowBalanceThreshold(ctx context.Context, in *billingv1.SetLowBalanceThresholdRequest, opts ...grpc.CallOption) (*billingv1.SetLowBalanceThresholdResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.SetLowBalanceThresholdResponse), args.Error(1)
}

func (m *testBillingClient) ListBalances(ctx context.Context, in *billingv1.ListBalancesRequest, opts ...grpc.CallOption) (*billingv1.ListBalancesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billingv1.ListBalancesResponse), args.Error(1)
}

// testFixtures содержит общие данные для тестов TarificationService
type testFixtures struct {
	clientID   uuid.UUID
	messageID  uuid.UUID
	operatorID uuid.UUID
	planID     uuid.UUID
	periodID   uuid.UUID
	tierID     uuid.UUID

	senderRepo  *mocks.MockSenderRegistrationRepository
	planRepo    *mocks.MockTariffPlanRepository
	periodRepo  *mocks.MockTariffPeriodRepository
	tierRepo    *mocks.MockTariffTierRepository
	usageRepo   *mocks.MockUsageCounterRepository
	logRepo     *mocks.MockTarificationLogRepository
	prepaidRepo *mocks.MockPrepaidFeeRepository
	billing     *testBillingClient
	eventPub    *mocks.MockEventPublisher

	service *application.TarificationService
}

func newTestFixtures() *testFixtures {
	f := &testFixtures{
		clientID:   uuid.New(),
		messageID:  uuid.New(),
		operatorID: uuid.New(),
		planID:     uuid.New(),
		periodID:   uuid.New(),
		tierID:     uuid.New(),

		senderRepo:  new(mocks.MockSenderRegistrationRepository),
		planRepo:    new(mocks.MockTariffPlanRepository),
		periodRepo:  new(mocks.MockTariffPeriodRepository),
		tierRepo:    new(mocks.MockTariffTierRepository),
		usageRepo:   new(mocks.MockUsageCounterRepository),
		logRepo:     new(mocks.MockTarificationLogRepository),
		prepaidRepo: new(mocks.MockPrepaidFeeRepository),
		billing:     new(testBillingClient),
		eventPub:    new(mocks.MockEventPublisher),
	}

	f.service = application.NewTarificationService(
		f.senderRepo,
		f.planRepo,
		f.periodRepo,
		f.tierRepo,
		f.usageRepo,
		f.logRepo,
		f.prepaidRepo,
		f.billing,
		f.eventPub,
	)

	return f
}

func TestTarificationService(t *testing.T) {

	t.Run("GetUsageCounter", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			expected := &domain.UsageCounter{
				ID: uuid.New(), ClientID: f.clientID,
				TariffPlanID: f.planID, TariffPeriodID: f.periodID, SegmentCount: 100,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).Return(expected, nil)

			counter, err := f.service.GetUsageCounter(ctx, f.clientID, f.planID, f.periodID)

			require.NoError(t, err)
			assert.Equal(t, 100, counter.SegmentCount)
		})

		t.Run("error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).
				Return(nil, errors.New("db error"))

			counter, err := f.service.GetUsageCounter(ctx, f.clientID, f.planID, f.periodID)

			assert.Nil(t, counter)
			assert.Error(t, err)
		})
	})

	t.Run("TarifyMessage_idempotency_replay", func(t *testing.T) {
		// Legacy replay: existing log с TariffPlanID → currency резолвится через planRepo.
		// Должна вернуться TarifyMessageResponse с полями из existing log, без обращения
		// к tier/period/usage/billing/eventPub.
		f := newTestFixtures()
		ctx := context.Background()
		idemKey := "idem-replay-1"
		planID := f.planID
		existing := &domain.TarificationLog{
			ID:             uuid.New(),
			ClientID:       f.clientID,
			MessageID:      f.messageID,
			OperatorID:     f.operatorID,
			Strategy:       domain.StrategyFixed,
			TariffPlanID:   &planID,
			SegmentCount:   2,
			TotalAmount:    "3.50",
			IdempotencyKey: idemKey,
		}
		plan := &domain.TariffPlan{
			ID:       planID,
			Currency: "RUB",
			Strategy: domain.StrategyFixed,
		}

		f.logRepo.On("GetByIdempotencyKey", ctx, idemKey).Return(existing, nil)
		f.planRepo.On("GetByID", ctx, planID).Return(plan, nil)

		resp, err := f.service.TarifyMessage(ctx, &application.TarifyMessageRequest{
			ClientID:       f.clientID,
			MessageID:      f.messageID,
			OperatorID:     f.operatorID,
			SegmentCount:   2,
			IdempotencyKey: idemKey,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Approved)
		assert.Equal(t, "3.50", resp.TotalAmount)
		assert.Equal(t, "RUB", resp.Currency)
		assert.Equal(t, string(domain.StrategyFixed), resp.Strategy)
		assert.Equal(t, planID.String(), resp.TariffPlanID)

		f.logRepo.AssertExpectations(t)
		f.planRepo.AssertExpectations(t)
		// Прочие зависимости НЕ должны вызываться на replay-ветке.
		f.tierRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		f.periodRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		f.usageRepo.AssertNotCalled(t, "GetOrCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		f.billing.AssertNotCalled(t, "ChargeMessage", mock.Anything, mock.Anything)
		f.billing.AssertNotCalled(t, "ChargeMessageDual", mock.Anything, mock.Anything)
		f.eventPub.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
	})

	t.Run("ListUsageCounters", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			counters := []*domain.UsageCounter{
				{ID: uuid.New(), ClientID: f.clientID, SegmentCount: 50},
				{ID: uuid.New(), ClientID: f.clientID, SegmentCount: 100},
			}
			f.usageRepo.On("GetByClient", ctx, f.clientID, (*uuid.UUID)(nil), 10, 0).
				Return(counters, 2, nil)

			result, total, err := f.service.ListUsageCounters(ctx, f.clientID, nil, 10, 0)

			require.NoError(t, err)
			assert.Equal(t, 2, total)
			assert.Len(t, result, 2)
		})

		t.Run("with_plan_filter", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			counters := []*domain.UsageCounter{
				{ID: uuid.New(), ClientID: f.clientID, TariffPlanID: f.planID, SegmentCount: 50},
			}
			f.usageRepo.On("GetByClient", ctx, f.clientID, &f.planID, 10, 0).
				Return(counters, 1, nil)

			result, total, err := f.service.ListUsageCounters(ctx, f.clientID, &f.planID, 10, 0)

			require.NoError(t, err)
			assert.Equal(t, 1, total)
			assert.Len(t, result, 1)
		})

		t.Run("error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			f.usageRepo.On("GetByClient", ctx, f.clientID, (*uuid.UUID)(nil), 10, 0).
				Return(nil, 0, errors.New("db error"))

			result, total, err := f.service.ListUsageCounters(ctx, f.clientID, nil, 10, 0)

			assert.Nil(t, result)
			assert.Equal(t, 0, total)
			assert.Error(t, err)
		})
	})
}
