package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProviderUsageCounter struct {
	ID                     uuid.UUID
	ProviderTariffPlanID   uuid.UUID
	ProviderTariffPeriodID uuid.UUID
	SegmentCount           int
	UpdatedAt              time.Time
}

func NewProviderUsageCounter(planID, periodID uuid.UUID) *ProviderUsageCounter {
	return &ProviderUsageCounter{
		ID:                     uuid.New(),
		ProviderTariffPlanID:   planID,
		ProviderTariffPeriodID: periodID,
		SegmentCount:           0,
		UpdatedAt:              time.Now(),
	}
}
