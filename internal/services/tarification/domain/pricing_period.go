package domain

import (
	"time"

	"github.com/google/uuid"
)

type PricingPeriod struct {
	ID             uuid.UUID
	TariffPeriodID uuid.UUID
	StartDate      time.Time
	EndDate        time.Time
	CreatedAt      time.Time
}

func NewPricingPeriod(tariffPeriodID uuid.UUID, startDate, endDate time.Time) *PricingPeriod {
	return &PricingPeriod{
		ID:             uuid.New(),
		TariffPeriodID: tariffPeriodID,
		StartDate:      startDate,
		EndDate:        endDate,
		CreatedAt:      time.Now(),
	}
}

func (p *PricingPeriod) Validate() error {
	if p.TariffPeriodID == uuid.Nil {
		return ErrPricingPeriodNotFound
	}
	if !p.EndDate.After(p.StartDate) {
		return ErrTariffPeriodInvalidDates
	}
	return nil
}
