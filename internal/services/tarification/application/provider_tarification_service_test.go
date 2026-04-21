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

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// --- inline mocks for provider repositories ---

type mockProviderTariffPlanRepo struct{ mock.Mock }

func (m *mockProviderTariffPlanRepo) Create(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	return m.Called(ctx, plan).Error(0)
}
func (m *mockProviderTariffPlanRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPlan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTariffPlan), args.Error(1)
}
func (m *mockProviderTariffPlanRepo) GetActiveByProviderAndOperator(ctx context.Context, providerID, operatorID uuid.UUID) (*domain.ProviderTariffPlan, error) {
	args := m.Called(ctx, providerID, operatorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTariffPlan), args.Error(1)
}
func (m *mockProviderTariffPlanRepo) Update(ctx context.Context, plan *domain.ProviderTariffPlan) error {
	return m.Called(ctx, plan).Error(0)
}
func (m *mockProviderTariffPlanRepo) List(ctx context.Context, providerID *uuid.UUID, activeOnly bool, limit, offset int) ([]*domain.ProviderTariffPlan, int, error) {
	args := m.Called(ctx, providerID, activeOnly, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.ProviderTariffPlan), args.Int(1), args.Error(2)
}

type mockProviderTariffPeriodRepo struct{ mock.Mock }

func (m *mockProviderTariffPeriodRepo) Create(ctx context.Context, period *domain.ProviderTariffPeriod) error {
	return m.Called(ctx, period).Error(0)
}
func (m *mockProviderTariffPeriodRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffPeriod, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTariffPeriod), args.Error(1)
}
func (m *mockProviderTariffPeriodRepo) GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*domain.ProviderTariffPeriod, error) {
	args := m.Called(ctx, planID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTariffPeriod), args.Error(1)
}
func (m *mockProviderTariffPeriodRepo) ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.ProviderTariffPeriod, error) {
	args := m.Called(ctx, planID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProviderTariffPeriod), args.Error(1)
}

type mockProviderTariffTierRepo struct{ mock.Mock }

func (m *mockProviderTariffTierRepo) Create(ctx context.Context, tier *domain.ProviderTariffTier) error {
	return m.Called(ctx, tier).Error(0)
}
func (m *mockProviderTariffTierRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderTariffTier, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTariffTier), args.Error(1)
}
func (m *mockProviderTariffTierRepo) Update(ctx context.Context, tier *domain.ProviderTariffTier) error {
	return m.Called(ctx, tier).Error(0)
}
func (m *mockProviderTariffTierRepo) ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*domain.ProviderTariffTier, error) {
	args := m.Called(ctx, periodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProviderTariffTier), args.Error(1)
}

type mockProviderUsageCounterRepo struct{ mock.Mock }

func (m *mockProviderUsageCounterRepo) GetOrCreate(ctx context.Context, planID, periodID uuid.UUID) (*domain.ProviderUsageCounter, error) {
	args := m.Called(ctx, planID, periodID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderUsageCounter), args.Error(1)
}
func (m *mockProviderUsageCounterRepo) IncrementAndGet(ctx context.Context, planID, periodID uuid.UUID, segments int) (*domain.ProviderUsageCounter, error) {
	args := m.Called(ctx, planID, periodID, segments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderUsageCounter), args.Error(1)
}

type mockProviderTarificationLogRepo struct{ mock.Mock }

func (m *mockProviderTarificationLogRepo) Create(ctx context.Context, entry *domain.ProviderTarificationLog) error {
	return m.Called(ctx, entry).Error(0)
}
func (m *mockProviderTarificationLogRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.ProviderTarificationLog, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderTarificationLog), args.Error(1)
}

// --- helpers ---

type providerTestFixtures struct {
	providerID uuid.UUID
	operatorID uuid.UUID
	clientID   uuid.UUID
	messageID  uuid.UUID
	planID     uuid.UUID
	periodID   uuid.UUID
	tierID     uuid.UUID

	planRepo   *mockProviderTariffPlanRepo
	periodRepo *mockProviderTariffPeriodRepo
	tierRepo   *mockProviderTariffTierRepo
	usageRepo  *mockProviderUsageCounterRepo
	logRepo    *mockProviderTarificationLogRepo

	service *application.ProviderTarificationService
}

func newProviderTestFixtures() *providerTestFixtures {
	f := &providerTestFixtures{
		providerID: uuid.New(),
		operatorID: uuid.New(),
		clientID:   uuid.New(),
		messageID:  uuid.New(),
		planID:     uuid.New(),
		periodID:   uuid.New(),
		tierID:     uuid.New(),

		planRepo:   new(mockProviderTariffPlanRepo),
		periodRepo: new(mockProviderTariffPeriodRepo),
		tierRepo:   new(mockProviderTariffTierRepo),
		usageRepo:  new(mockProviderUsageCounterRepo),
		logRepo:    new(mockProviderTarificationLogRepo),
	}

	f.service = application.NewProviderTarificationService(
		f.planRepo,
		f.periodRepo,
		f.tierRepo,
		f.usageRepo,
		f.logRepo,
	)

	return f
}

func TestProviderTarificationService(t *testing.T) {
	t.Run("TarifyProviderCost", func(t *testing.T) {
		t.Run("happy_path_fixed_strategy", func(t *testing.T) {
			f := newProviderTestFixtures()
			ctx := context.Background()

			req := &application.TarifyProviderCostRequest{
				ProviderID:     f.providerID,
				OperatorID:     f.operatorID,
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				SegmentCount:   2,
				IdempotencyKey: "provider-idem-1",
			}

			// 1. Idempotency — no existing record
			f.logRepo.On("GetByIdempotencyKey", ctx, "provider-idem-1").
				Return(nil, nil)

			// 2. Active provider tariff plan (fixed strategy)
			plan := &domain.ProviderTariffPlan{
				ID:         f.planID,
				ProviderID: f.providerID,
				OperatorID: f.operatorID,
				Strategy:   domain.StrategyFixed,
				Active:     true,
			}
			f.planRepo.On("GetActiveByProviderAndOperator", ctx, f.providerID, f.operatorID).
				Return(plan, nil)

			// 3. Active period
			now := time.Now()
			period := &domain.ProviderTariffPeriod{
				ID:                   f.periodID,
				ProviderTariffPlanID: f.planID,
				StartDate:            now.AddDate(0, -1, 0),
				EndDate:              now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(period, nil)

			// 4. Tiers — single fixed tier
			tiers := []*domain.ProviderTariffTier{
				{
					ID:                     f.tierID,
					ProviderTariffPeriodID: f.periodID,
					FromCount:              0,
					PricePerSegment:        "0.030000",
				},
			}
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).Return(tiers, nil)

			// 5. Usage counter
			counter := &domain.ProviderUsageCounter{
				ID:                     uuid.New(),
				ProviderTariffPlanID:   f.planID,
				ProviderTariffPeriodID: f.periodID,
				SegmentCount:           10,
			}
			f.usageRepo.On("GetOrCreate", ctx, f.planID, f.periodID).Return(counter, nil)

			// 6. IncrementAndGet
			f.usageRepo.On("IncrementAndGet", ctx, f.planID, f.periodID, 2).
				Return(&domain.ProviderUsageCounter{SegmentCount: 12}, nil)

			// 7. Log entry creation
			f.logRepo.On("Create", ctx, mock.AnythingOfType("*domain.ProviderTarificationLog")).
				Return(nil)

			err := f.service.TarifyProviderCost(ctx, req)

			require.NoError(t, err)
			f.planRepo.AssertExpectations(t)
			f.periodRepo.AssertExpectations(t)
			f.tierRepo.AssertExpectations(t)
			f.usageRepo.AssertExpectations(t)
			f.logRepo.AssertExpectations(t)
		})

		t.Run("idempotency_already_processed", func(t *testing.T) {
			f := newProviderTestFixtures()
			ctx := context.Background()

			req := &application.TarifyProviderCostRequest{
				ProviderID:     f.providerID,
				OperatorID:     f.operatorID,
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				SegmentCount:   1,
				IdempotencyKey: "provider-idem-existing",
			}

			existing := &domain.ProviderTarificationLog{
				ID:             uuid.New(),
				IdempotencyKey: "provider-idem-existing",
			}
			f.logRepo.On("GetByIdempotencyKey", ctx, "provider-idem-existing").
				Return(existing, nil)

			err := f.service.TarifyProviderCost(ctx, req)

			require.NoError(t, err)
			// No plan/period/tier lookups should happen
			f.planRepo.AssertNotCalled(t, "GetActiveByProviderAndOperator", mock.Anything, mock.Anything, mock.Anything)
			f.logRepo.AssertExpectations(t)
		})

		t.Run("no_active_plan_returns_nil", func(t *testing.T) {
			f := newProviderTestFixtures()
			ctx := context.Background()

			req := &application.TarifyProviderCostRequest{
				ProviderID:     f.providerID,
				OperatorID:     f.operatorID,
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				SegmentCount:   1,
				IdempotencyKey: "provider-idem-noplan",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "provider-idem-noplan").
				Return(nil, nil)

			f.planRepo.On("GetActiveByProviderAndOperator", ctx, f.providerID, f.operatorID).
				Return(nil, domain.ErrNoActiveProviderTariffPlan)

			err := f.service.TarifyProviderCost(ctx, req)

			// Graceful: no plan = no cost tracking, not an error
			require.NoError(t, err)
			f.planRepo.AssertExpectations(t)
			f.logRepo.AssertExpectations(t)
		})

		t.Run("no_tiers_returns_nil", func(t *testing.T) {
			f := newProviderTestFixtures()
			ctx := context.Background()

			req := &application.TarifyProviderCostRequest{
				ProviderID:     f.providerID,
				OperatorID:     f.operatorID,
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				SegmentCount:   1,
				IdempotencyKey: "provider-idem-notiers",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "provider-idem-notiers").
				Return(nil, nil)

			plan := &domain.ProviderTariffPlan{
				ID:         f.planID,
				ProviderID: f.providerID,
				OperatorID: f.operatorID,
				Strategy:   domain.StrategyFixed,
				Active:     true,
			}
			f.planRepo.On("GetActiveByProviderAndOperator", ctx, f.providerID, f.operatorID).
				Return(plan, nil)

			now := time.Now()
			period := &domain.ProviderTariffPeriod{
				ID:                   f.periodID,
				ProviderTariffPlanID: f.planID,
				StartDate:            now.AddDate(0, -1, 0),
				EndDate:              now.AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetActiveByPlanID", ctx, f.planID, mock.AnythingOfType("time.Time")).
				Return(period, nil)

			// Empty tiers
			f.tierRepo.On("ListByPeriodID", ctx, f.periodID).
				Return([]*domain.ProviderTariffTier{}, nil)

			err := f.service.TarifyProviderCost(ctx, req)

			// Graceful: no tiers = skip
			require.NoError(t, err)
			f.planRepo.AssertExpectations(t)
			f.periodRepo.AssertExpectations(t)
			f.tierRepo.AssertExpectations(t)
			f.logRepo.AssertExpectations(t)
		})

		t.Run("idempotency_check_error_returns_error", func(t *testing.T) {
			f := newProviderTestFixtures()
			ctx := context.Background()

			req := &application.TarifyProviderCostRequest{
				ProviderID:     f.providerID,
				OperatorID:     f.operatorID,
				ClientID:       f.clientID,
				MessageID:      f.messageID,
				SegmentCount:   1,
				IdempotencyKey: "provider-idem-err",
			}

			f.logRepo.On("GetByIdempotencyKey", ctx, "provider-idem-err").
				Return(nil, errors.New("db connection error"))

			err := f.service.TarifyProviderCost(ctx, req)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "idempotency check")
			f.logRepo.AssertExpectations(t)
		})
	})
}
