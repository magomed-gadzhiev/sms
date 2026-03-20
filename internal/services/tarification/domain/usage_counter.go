package domain

import (
	"time"

	"github.com/google/uuid"
)

type UsageCounter struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	TariffPlanID   uuid.UUID
	TariffPeriodID uuid.UUID
	SegmentCount   int
	UpdatedAt      time.Time
}

func NewUsageCounter(clientID, tariffPlanID, tariffPeriodID uuid.UUID) *UsageCounter {
	return &UsageCounter{
		ID:             uuid.New(),
		ClientID:       clientID,
		TariffPlanID:   tariffPlanID,
		TariffPeriodID: tariffPeriodID,
		SegmentCount:   0,
		UpdatedAt:      time.Now(),
	}
}
