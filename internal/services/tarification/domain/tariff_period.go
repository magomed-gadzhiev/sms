package domain

import (
	"time"

	"github.com/google/uuid"
)

type TariffPeriod struct {
	ID           uuid.UUID
	TariffPlanID uuid.UUID
	StartDate    time.Time
	EndDate      *time.Time
	CreatedAt    time.Time
}

func NewTariffPeriod(tariffPlanID uuid.UUID, startDate time.Time, endDate *time.Time) *TariffPeriod {
	return &TariffPeriod{
		ID:           uuid.New(),
		TariffPlanID: tariffPlanID,
		StartDate:    startDate,
		EndDate:      endDate,
		CreatedAt:    time.Now(),
	}
}

func (t *TariffPeriod) Validate() error {
	if t.TariffPlanID == uuid.Nil {
		return ErrTariffPeriodNotFound
	}
	if t.EndDate != nil && !t.EndDate.After(t.StartDate) {
		return ErrTariffPeriodInvalidDates
	}
	return nil
}

func (t *TariffPeriod) IsActive(now time.Time) bool {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := time.Date(t.StartDate.Year(), t.StartDate.Month(), t.StartDate.Day(), 0, 0, 0, 0, t.StartDate.Location())
	if t.EndDate == nil {
		return !today.Before(start)
	}
	end := time.Date(t.EndDate.Year(), t.EndDate.Month(), t.EndDate.Day(), 0, 0, 0, 0, t.EndDate.Location())
	return !today.Before(start) && !today.After(end)
}
