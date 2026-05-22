package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type ProviderTariffPlan struct {
	ID         uuid.UUID
	ProviderID uuid.UUID
	OperatorID uuid.UUID
	Strategy   TarificationStrategy
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ProviderTariffPeriod struct {
	ID                   uuid.UUID
	ProviderTariffPlanID uuid.UUID
	StartDate            time.Time
	EndDate              time.Time
	CreatedAt            time.Time
}

type ProviderTariffTier struct {
	ID                     uuid.UUID
	ProviderTariffPeriodID uuid.UUID
	FromCount              int
	PricePerSegment        string
}

var (
	ErrProviderTariffPlanNotFound      = errors.New("provider tariff plan not found")
	ErrProviderTariffPeriodNotFound    = errors.New("provider tariff period not found")
	ErrNoActiveProviderTariffPlan      = errors.New("no active provider tariff plan")
	ErrNoActiveProviderTariffPeriod    = errors.New("no active provider tariff period")
)

func NewProviderTariffPlan(providerID, operatorID uuid.UUID, strategy TarificationStrategy) *ProviderTariffPlan {
	now := time.Now()
	return &ProviderTariffPlan{
		ID:         uuid.New(),
		ProviderID: providerID,
		OperatorID: operatorID,
		Strategy:   strategy,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (p *ProviderTariffPlan) Validate() error {
	if p.ProviderID == uuid.Nil {
		return ErrProviderTariffPlanNotFound
	}
	if p.OperatorID == uuid.Nil {
		return errors.New("operator_id is required")
	}
	switch p.Strategy {
	case StrategyFixed, StrategyThreshold, StrategyThresholdRecalc, StrategyPrepaidThreshold:
		return nil
	default:
		return ErrTariffPlanInvalidStrategy
	}
}
