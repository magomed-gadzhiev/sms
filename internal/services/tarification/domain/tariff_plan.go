package domain

import (
	"time"

	"github.com/google/uuid"
)

type TarificationStrategy string

const (
	StrategyFixed            TarificationStrategy = "fixed"
	StrategyThreshold        TarificationStrategy = "threshold"
	StrategyThresholdRecalc  TarificationStrategy = "threshold_recalc"
	StrategyPrepaidThreshold TarificationStrategy = "prepaid_threshold"
	StrategyUnified          TarificationStrategy = "unified"
)

type TariffPlan struct {
	ID             uuid.UUID
	OperatorID     uuid.UUID
	SenderCategory SenderCategory
	Strategy       TarificationStrategy
	Active         bool
	Currency       string // ISO 4217, resolved from operator's country
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewTariffPlan(operatorID uuid.UUID, category SenderCategory, strategy TarificationStrategy) *TariffPlan {
	now := time.Now()
	return &TariffPlan{
		ID:             uuid.New(),
		OperatorID:     operatorID,
		SenderCategory: category,
		Strategy:       strategy,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (t *TariffPlan) Validate() error {
	if t.OperatorID == uuid.Nil {
		return ErrTariffPlanNotFound
	}
	switch t.SenderCategory {
	case CategoryShared, CategoryPaidRegistered, CategoryFreeRegistered:
	default:
		return ErrTariffPlanInvalidCategory
	}
	switch t.Strategy {
	case StrategyFixed, StrategyThreshold, StrategyThresholdRecalc, StrategyPrepaidThreshold:
	default:
		return ErrTariffPlanInvalidStrategy
	}
	return nil
}
