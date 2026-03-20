package domain

import (
	"time"

	"github.com/google/uuid"
)

type PrepaidFee struct {
	ID             uuid.UUID
	TariffPlanID   uuid.UUID
	TariffPeriodID uuid.UUID
	Amount         string
	Currency       string
	Charged        bool
	ChargedAt      *time.Time
	CreatedAt      time.Time
}

func NewPrepaidFee(tariffPlanID, tariffPeriodID uuid.UUID, amount, currency string) *PrepaidFee {
	return &PrepaidFee{
		ID:             uuid.New(),
		TariffPlanID:   tariffPlanID,
		TariffPeriodID: tariffPeriodID,
		Amount:         amount,
		Currency:       currency,
		Charged:        false,
		CreatedAt:      time.Now(),
	}
}

func (p *PrepaidFee) Validate() error {
	if p.TariffPlanID == uuid.Nil || p.TariffPeriodID == uuid.Nil {
		return ErrPrepaidFeeNotFound
	}
	if p.Amount == "" || p.Currency == "" {
		return ErrPrepaidFeeNotFound
	}
	return nil
}
