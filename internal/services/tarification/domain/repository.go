package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SenderRegistrationRepository interface {
	Create(ctx context.Context, reg *SenderRegistration) error
	GetByID(ctx context.Context, id uuid.UUID) (*SenderRegistration, error)
	GetByClientAndOperator(ctx context.Context, clientID, operatorID uuid.UUID) ([]*SenderRegistration, error)
	GetActiveByClientOperatorName(ctx context.Context, clientID, operatorID uuid.UUID, senderName string) (*SenderRegistration, error)
	Update(ctx context.Context, reg *SenderRegistration) error
	List(ctx context.Context, clientID, operatorID *uuid.UUID, limit, offset int) ([]*SenderRegistration, int, error)
}

type TariffPlanRepository interface {
	Create(ctx context.Context, plan *TariffPlan) error
	GetByID(ctx context.Context, id uuid.UUID) (*TariffPlan, error)
	GetActiveByOperatorAndCategory(ctx context.Context, operatorID uuid.UUID, category SenderCategory) (*TariffPlan, error)
	Update(ctx context.Context, plan *TariffPlan) error
	List(ctx context.Context, operatorID *uuid.UUID, activeOnly bool, limit, offset int) ([]*TariffPlan, int, error)
}

type TariffPeriodRepository interface {
	Create(ctx context.Context, period *TariffPeriod) error
	GetByID(ctx context.Context, id uuid.UUID) (*TariffPeriod, error)
	GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*TariffPeriod, error)
	ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*TariffPeriod, error)
	HasActivePeriod(ctx context.Context, planID uuid.UUID, now time.Time) (bool, error)
}

type TariffTierRepository interface {
	Create(ctx context.Context, tier *TariffTier) error
	GetByID(ctx context.Context, id uuid.UUID) (*TariffTier, error)
	Update(ctx context.Context, tier *TariffTier) error
	ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*TariffTier, error)
}

type PricingPeriodRepository interface {
	Create(ctx context.Context, period *PricingPeriod) error
	GetByID(ctx context.Context, id uuid.UUID) (*PricingPeriod, error)
	GetActiveByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID, now time.Time) (*PricingPeriod, error)
	ListByTariffPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) ([]*PricingPeriod, error)
}

type PrepaidFeeRepository interface {
	Create(ctx context.Context, fee *PrepaidFee) error
	GetByID(ctx context.Context, id uuid.UUID) (*PrepaidFee, error)
	GetByPeriodID(ctx context.Context, tariffPeriodID uuid.UUID) (*PrepaidFee, error)
	GetUncharged(ctx context.Context, now time.Time) ([]*PrepaidFee, error)
	MarkCharged(ctx context.Context, id uuid.UUID) error
}

type UsageCounterRepository interface {
	GetOrCreate(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID) (*UsageCounter, error)
	IncrementAndGet(ctx context.Context, clientID, tariffPlanID, tariffPeriodID uuid.UUID, segments int) (*UsageCounter, error)
	GetByClient(ctx context.Context, clientID uuid.UUID, tariffPlanID *uuid.UUID, limit, offset int) ([]*UsageCounter, int, error)
}

type TarificationLogRepository interface {
	Create(ctx context.Context, log *TarificationLog) error
	GetByIdempotencyKey(ctx context.Context, key string) (*TarificationLog, error)
	GetByMessageID(ctx context.Context, messageID uuid.UUID) (*TarificationLog, error)
}
