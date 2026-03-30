package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	t.Run("TarifyMessage", func(t *testing.T) {
		t.Run("fixed_strategy", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   2,
				IdempotencyKey: "idem-fixed-1",
			}

			// 1. Идемпотентность — записи нет
			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-fixed-1").
				Return(nil, nil)

			// 2. Sender name пустой — категория shared (senderRepo не вызывается)

			// 3. Активный тарифный план (fixed)
			plan := &domain.TariffPlan{
				ID:             f.planID,
				OperatorID:     f.operatorID,
				SenderCategory: domain.CategoryShared,
				Strategy:       domain.StrategyFixed,
				Active:         true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).
				Return(plan, nil)

			// 4. Активный период
			now := time.Now()
			period := &domain.TariffPeriod{
				ID:           f.periodID,
				TariffPlanID: f.planID,
				StartDate:    now.AddDate(0, -1, 0),
				EndDate:      now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(period, nil)

			// 5. Пороги — один фиксированный
			tiers := []*domain.TariffTier{
				{
					ID:              f.tierID,
					TariffPeriodID:  f.periodID,
					FromCount:       0,
					PricePerSegment: "0.050000",
				},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).
				Return(tiers, nil)

			// 6. Счётчик использования
			counter := &domain.UsageCounter{
				ID:             uuid.New(),
				ClientID:       f.clientID,
				TariffPlanID:   f.planID,
				TariffPeriodID: f.periodID,
				SegmentCount:   50,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).
				Return(counter, nil)

			// 8. Списание через billing
			f.billing.On("ChargeMessage", ctx, mock.MatchedBy(func(req *billingv1.ChargeMessageRequest) bool {
				return req.ClientId == f.clientID.String() &&
					req.MessageId == f.messageID.String() &&
					req.Amount == "0.100000"
			})).Return(&billingv1.ChargeMessageResponse{
				TransactionId: "tx-1",
				NewBalance:    "99.900000",
				Success:       true,
			}, nil)

			// 9. Инкремент счётчика
			updatedCounter := &domain.UsageCounter{
				ClientID:       f.clientID,
				TariffPlanID:   f.planID,
				TariffPeriodID: f.periodID,
				SegmentCount:   52,
			}
			f.usageRepo.On("IncrementAndGet", ctx, f.clientID, f.planID, f.periodID, 2).
				Return(updatedCounter, nil)

			// 10. Запись лога
			f.logRepo.On("Create", ctx, mock.AnythingOfType("*domain.TarificationLog")).
				Return(nil)

			// 12. Публикация события
			f.eventPub.On("PublishTarificationResult", ctx, mock.AnythingOfType("*domain.TarificationLog")).
				Return(nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.True(t, resp.Approved)
			// 2 сегмента * 0.05 = 0.10
			assert.Equal(t, "0.100000", resp.TotalAmount)
			assert.Equal(t, "fixed", resp.Strategy)
			assert.Equal(t, f.planID.String(), resp.TariffPlanID)
			assert.False(t, resp.ThresholdCrossed)
			assert.Empty(t, resp.RecalcAmount)

			f.logRepo.AssertExpectations(t)
			f.planRepo.AssertExpectations(t)
			f.periodRepo.AssertExpectations(t)
			f.tierRepo.AssertExpectations(t)
			f.usageRepo.AssertExpectations(t)
			f.billing.AssertExpectations(t)
			f.eventPub.AssertExpectations(t)
		})

		t.Run("threshold_strategy", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "MySender",
				SegmentCount:   4,
				IdempotencyKey: "idem-thresh-1",
			}

			// 1. Идемпотентность — записи нет
			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-thresh-1").
				Return(nil, nil)

			// 2. Определение категории — paid_registered
			senderReg := &domain.SenderRegistration{
				ID:         uuid.New(),
				ClientID:   f.clientID,
				OperatorID: f.operatorID,
				SenderName: "MySender",
				Type:       domain.SenderTypePaid,
				Status:     domain.SenderStatusActive,
			}
			f.senderRepo.On("GetActiveByClientOperatorName", ctx, f.clientID, f.operatorID, "MySender").
				Return(senderReg, nil)

			// 3. Активный тарифный план (threshold)
			plan := &domain.TariffPlan{
				ID:             f.planID,
				OperatorID:     f.operatorID,
				SenderCategory: domain.CategoryPaidRegistered,
				Strategy:       domain.StrategyThreshold,
				Active:         true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryPaidRegistered).
				Return(plan, nil)

			// 4. Активный период
			now := time.Now()
			period := &domain.TariffPeriod{
				ID:           f.periodID,
				TariffPlanID: f.planID,
				StartDate:    now.AddDate(0, -1, 0),
				EndDate:      now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(period, nil)

			// 5. Пороги — два уровня: 0-999 = 0.10, 1000+ = 0.07
			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  f.periodID,
					FromCount:       0,
					PricePerSegment: "0.100000",
				},
				{
					ID:              uuid.New(),
					TariffPeriodID:  f.periodID,
					FromCount:       1000,
					PricePerSegment: "0.070000",
				},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).
				Return(tiers, nil)

			// 6. Счётчик — пересекаем порог: currentCount=998, +4 сегмента
			counter := &domain.UsageCounter{
				ID:             uuid.New(),
				ClientID:       f.clientID,
				TariffPlanID:   f.planID,
				TariffPeriodID: f.periodID,
				SegmentCount:   998,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).
				Return(counter, nil)

			// 8. Списание через billing
			// 2 * 0.10 + 2 * 0.07 = 0.20 + 0.14 = 0.34
			f.billing.On("ChargeMessage", ctx, mock.MatchedBy(func(req *billingv1.ChargeMessageRequest) bool {
				return req.ClientId == f.clientID.String() &&
					req.Amount == "0.340000"
			})).Return(&billingv1.ChargeMessageResponse{
				TransactionId: "tx-2",
				NewBalance:    "99.660000",
				Success:       true,
			}, nil)

			// 9. Инкремент счётчика
			f.usageRepo.On("IncrementAndGet", ctx, f.clientID, f.planID, f.periodID, 4).
				Return(&domain.UsageCounter{SegmentCount: 1002}, nil)

			// 10. Запись лога
			f.logRepo.On("Create", ctx, mock.AnythingOfType("*domain.TarificationLog")).
				Return(nil)

			// 12. Публикация события
			f.eventPub.On("PublishTarificationResult", ctx, mock.AnythingOfType("*domain.TarificationLog")).
				Return(nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.True(t, resp.Approved)
			assert.Equal(t, "0.340000", resp.TotalAmount)
			assert.Equal(t, "threshold", resp.Strategy)
			assert.True(t, resp.ThresholdCrossed)

			f.senderRepo.AssertExpectations(t)
			f.planRepo.AssertExpectations(t)
			f.billing.AssertExpectations(t)
		})

		t.Run("idempotency", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   1,
				IdempotencyKey: "idem-dup-1",
			}

			// Уже есть запись с таким ключом
			existingLog := &domain.TarificationLog{
				ID:              uuid.New(),
				ClientID:        f.clientID,
				MessageID:       f.messageID,
				OperatorID:      f.operatorID,
				TariffPlanID:    f.planID,
				TariffPeriodID:  f.periodID,
				SenderCategory:  domain.CategoryShared,
				Strategy:        domain.StrategyFixed,
				SegmentCount:    1,
				PricePerSegment: "0.050000",
				TotalAmount:     "0.050000",
				IdempotencyKey:  "idem-dup-1",
			}
			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-dup-1").
				Return(existingLog, nil)

			// При идемпотентности вызывается GetByID для получения валюты
			existingPlan := &domain.TariffPlan{
				ID:             f.planID,
				OperatorID:     f.operatorID,
				SenderCategory: domain.CategoryShared,
				Strategy:       domain.StrategyFixed,
				Active:         true,
				Currency:       "RUB",
			}
			f.planRepo.On("GetByID", ctx, f.planID).
				Return(existingPlan, nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.True(t, resp.Approved)
			assert.Equal(t, "0.050000", resp.TotalAmount)
			assert.Equal(t, "RUB", resp.Currency)
			assert.Equal(t, "fixed", resp.Strategy)
			assert.Equal(t, f.planID.String(), resp.TariffPlanID)

			// Никакие другие репозитории не должны вызываться при дубликате
			f.planRepo.AssertNotCalled(t, "GetActiveByOperatorAndCategory", mock.Anything, mock.Anything, mock.Anything)
			f.periodRepo.AssertNotCalled(t, "GetActiveByPlanID", mock.Anything, mock.Anything, mock.Anything)
			f.tierRepo.AssertNotCalled(t, "ListByPeriodID", mock.Anything, mock.Anything)
			f.usageRepo.AssertNotCalled(t, "GetOrCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			f.billing.AssertNotCalled(t, "ChargeMessage", mock.Anything, mock.Anything)
			f.logRepo.AssertExpectations(t)
		})

		t.Run("no_active_plan_rejected", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   1,
				IdempotencyKey: "idem-noplan-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-noplan-1").
				Return(nil, nil)

			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).
				Return(nil, domain.ErrNoActiveTariffPlan)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.False(t, resp.Approved)
			assert.Equal(t, domain.ErrNoActiveTariffPlan.Error(), resp.RejectionReason)
		})

		t.Run("insufficient_balance_rejected", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   1,
				IdempotencyKey: "idem-nobal-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-nobal-1").
				Return(nil, nil)

			plan := &domain.TariffPlan{
				ID:             f.planID,
				OperatorID:     f.operatorID,
				SenderCategory: domain.CategoryShared,
				Strategy:       domain.StrategyFixed,
				Active:         true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).
				Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID:           f.periodID,
				TariffPlanID: f.planID,
				StartDate:    now.AddDate(0, -1, 0),
				EndDate:      now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(period, nil)

			tiers := []*domain.TariffTier{
				{
					ID:              uuid.New(),
					TariffPeriodID:  f.periodID,
					FromCount:       0,
					PricePerSegment: "0.050000",
				},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).
				Return(tiers, nil)

			counter := &domain.UsageCounter{
				ID:             uuid.New(),
				ClientID:       f.clientID,
				TariffPlanID:   f.planID,
				TariffPeriodID: f.periodID,
				SegmentCount:   0,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).
				Return(counter, nil)

			f.billing.On("ChargeMessage", ctx, mock.Anything).
				Return(&billingv1.ChargeMessageResponse{
					Success: false,
					Error:   "insufficient balance",
				}, nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.False(t, resp.Approved)
			assert.Equal(t, domain.ErrInsufficientBalance.Error(), resp.RejectionReason)
		})

		t.Run("no_active_period_rejected", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   1,
				IdempotencyKey: "idem-noperiod-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-noperiod-1").
				Return(nil, nil)

			plan := &domain.TariffPlan{
				ID:             f.planID,
				OperatorID:     f.operatorID,
				SenderCategory: domain.CategoryShared,
				Strategy:       domain.StrategyFixed,
				Active:         true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).
				Return(plan, nil)

			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(nil, domain.ErrNoActivePeriod)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.False(t, resp.Approved)
			assert.Equal(t, domain.ErrNoActivePeriod.Error(), resp.RejectionReason)
		})

		t.Run("no_tiers_configured_rejected", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "",
				SegmentCount:   1,
				IdempotencyKey: "idem-notiers-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-notiers-1").Return(nil, nil)

			plan := &domain.TariffPlan{
				ID: f.planID, OperatorID: f.operatorID,
				SenderCategory: domain.CategoryShared, Strategy: domain.StrategyFixed, Active: true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID: f.periodID, TariffPlanID: f.planID,
				StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).Return(period, nil)

			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return([]*domain.TariffTier{}, nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.False(t, resp.Approved)
			assert.Contains(t, resp.RejectionReason, "no tiers configured")
		})

		t.Run("free_registered_sender_category", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				OperatorID:     f.operatorID,
				SenderName:     "FreeSender",
				SegmentCount:   1,
				IdempotencyKey: "idem-free-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-free-1").Return(nil, nil)

			// Free registration
			senderReg := &domain.SenderRegistration{
				ID: uuid.New(), ClientID: f.clientID, OperatorID: f.operatorID,
				SenderName: "FreeSender", Type: domain.SenderTypeFree, Status: domain.SenderStatusActive,
			}
			f.senderRepo.On("GetActiveByClientOperatorName", ctx, f.clientID, f.operatorID, "FreeSender").
				Return(senderReg, nil)

			plan := &domain.TariffPlan{
				ID: f.planID, OperatorID: f.operatorID,
				SenderCategory: domain.CategoryFreeRegistered, Strategy: domain.StrategyFixed, Active: true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryFreeRegistered).
				Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID: f.periodID, TariffPlanID: f.planID,
				StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).Return(period, nil)

			tiers := []*domain.TariffTier{
				{ID: f.tierID, TariffPeriodID: f.periodID, FromCount: 0, PricePerSegment: "0.030000"},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return(tiers, nil)

			counter := &domain.UsageCounter{
				ID: uuid.New(), ClientID: f.clientID,
				TariffPlanID: f.planID, TariffPeriodID: f.periodID, SegmentCount: 0,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).Return(counter, nil)

			f.billing.On("ChargeMessage", ctx, mock.MatchedBy(func(req *billingv1.ChargeMessageRequest) bool {
				return req.Amount == "0.030000"
			})).Return(&billingv1.ChargeMessageResponse{
				TransactionId: "tx-free-1", NewBalance: "99.97", Success: true,
			}, nil)

			f.usageRepo.On("IncrementAndGet", ctx, f.clientID, f.planID, f.periodID, 1).
				Return(&domain.UsageCounter{SegmentCount: 1}, nil)
			f.logRepo.On("Create", ctx, mock.AnythingOfType("*domain.TarificationLog")).Return(nil)
			f.eventPub.On("PublishTarificationResult", ctx, mock.AnythingOfType("*domain.TarificationLog")).Return(nil)

			resp, err := f.service.TarifyMessage(ctx, req)

			require.NoError(t, err)
			assert.True(t, resp.Approved)
			assert.Equal(t, "0.030000", resp.TotalAmount)
		})

		t.Run("idempotency_check_error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID: f.clientID, MessageID: f.messageID, OperatorID: f.operatorID,
				SenderName: "", SegmentCount: 1, IdempotencyKey: "idem-err-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-err-1").
				Return(nil, errors.New("db error"))

			resp, err := f.service.TarifyMessage(ctx, req)

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "idempotency check failed")
		})

		t.Run("tiers_fetch_error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID: f.clientID, MessageID: f.messageID, OperatorID: f.operatorID,
				SenderName: "", SegmentCount: 1, IdempotencyKey: "idem-tierserr-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-tierserr-1").Return(nil, nil)

			plan := &domain.TariffPlan{
				ID: f.planID, OperatorID: f.operatorID,
				SenderCategory: domain.CategoryShared, Strategy: domain.StrategyFixed, Active: true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID: f.periodID, TariffPlanID: f.planID,
				StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).Return(period, nil)
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return(nil, errors.New("db error"))

			resp, err := f.service.TarifyMessage(ctx, req)

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to get tiers")
		})

		t.Run("usage_counter_error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID: f.clientID, MessageID: f.messageID, OperatorID: f.operatorID,
				SenderName: "", SegmentCount: 1, IdempotencyKey: "idem-usage-err-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-usage-err-1").Return(nil, nil)

			plan := &domain.TariffPlan{
				ID: f.planID, OperatorID: f.operatorID,
				SenderCategory: domain.CategoryShared, Strategy: domain.StrategyFixed, Active: true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID: f.periodID, TariffPlanID: f.planID,
				StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).Return(period, nil)

			tiers := []*domain.TariffTier{
				{ID: f.tierID, TariffPeriodID: f.periodID, FromCount: 0, PricePerSegment: "0.050000"},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return(tiers, nil)
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).
				Return(nil, errors.New("db error"))

			resp, err := f.service.TarifyMessage(ctx, req)

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to get usage counter")
		})

		t.Run("billing_charge_error", func(t *testing.T) {
			f := newTestFixtures()
			ctx := context.Background()

			req := &application.TarifyMessageRequest{
				ClientID: f.clientID, MessageID: f.messageID, OperatorID: f.operatorID,
				SenderName: "", SegmentCount: 1, IdempotencyKey: "idem-billing-err-1",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "idem-billing-err-1").Return(nil, nil)

			plan := &domain.TariffPlan{
				ID: f.planID, OperatorID: f.operatorID,
				SenderCategory: domain.CategoryShared, Strategy: domain.StrategyFixed, Active: true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, f.operatorID, domain.CategoryShared).Return(plan, nil)

			now := time.Now()
			period := &domain.TariffPeriod{
				ID: f.periodID, TariffPlanID: f.planID,
				StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).Return(period, nil)

			tiers := []*domain.TariffTier{
				{ID: f.tierID, TariffPeriodID: f.periodID, FromCount: 0, PricePerSegment: "0.050000"},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return(tiers, nil)

			counter := &domain.UsageCounter{
				ID: uuid.New(), ClientID: f.clientID,
				TariffPlanID: f.planID, TariffPeriodID: f.periodID, SegmentCount: 0,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.clientID, f.planID, f.periodID).Return(counter, nil)
			f.billing.On("ChargeMessage", ctx, mock.Anything).
				Return(nil, errors.New("billing unavailable"))

			resp, err := f.service.TarifyMessage(ctx, req)

			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "billing charge failed")
		})
	})

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
