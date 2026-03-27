package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ProviderTariffPlanRepository interface {
	Create(ctx context.Context, plan *ProviderTariffPlan) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffPlan, error)
	GetActiveByProviderAndOperator(ctx context.Context, providerID, operatorID uuid.UUID) (*ProviderTariffPlan, error)
	Update(ctx context.Context, plan *ProviderTariffPlan) error
	List(ctx context.Context, providerID *uuid.UUID, activeOnly bool, limit, offset int) ([]*ProviderTariffPlan, int, error)
}

type ProviderTariffPeriodRepository interface {
	Create(ctx context.Context, period *ProviderTariffPeriod) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffPeriod, error)
	GetActiveByPlanID(ctx context.Context, planID uuid.UUID, now time.Time) (*ProviderTariffPeriod, error)
	ListByPlanID(ctx context.Context, planID uuid.UUID) ([]*ProviderTariffPeriod, error)
}

type ProviderTariffTierRepository interface {
	Create(ctx context.Context, tier *ProviderTariffTier) error
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTariffTier, error)
	Update(ctx context.Context, tier *ProviderTariffTier) error
	ListByPeriodID(ctx context.Context, periodID uuid.UUID) ([]*ProviderTariffTier, error)
}

type ProviderUsageCounterRepository interface {
	GetOrCreate(ctx context.Context, planID, periodID uuid.UUID) (*ProviderUsageCounter, error)
	IncrementAndGet(ctx context.Context, planID, periodID uuid.UUID, segments int) (*ProviderUsageCounter, error)
}

type ProviderTarificationLogRepository interface {
	Create(ctx context.Context, log *ProviderTarificationLog) error
	GetByIdempotencyKey(ctx context.Context, key string) (*ProviderTarificationLog, error)
}

// MarginReportEntry represents a single row in the margin report.
type MarginReportEntry struct {
	OperatorID   uuid.UUID
	OperatorName string
	ProviderID   uuid.UUID
	ProviderName string
	Segments     int
	Revenue      string
	Cost         string
	Margin       string
}

type MarginReportRepository interface {
	GetMarginReport(ctx context.Context, clientID uuid.UUID, from, to time.Time) ([]*MarginReportEntry, error)
}
