package domain

import "github.com/google/uuid"

type TariffTier struct {
	ID              uuid.UUID
	TariffPeriodID  uuid.UUID
	FromCount       int
	PricePerSegment string
}

func NewTariffTier(tariffPeriodID uuid.UUID, fromCount int, pricePerSegment string) *TariffTier {
	return &TariffTier{
		ID:              uuid.New(),
		TariffPeriodID:  tariffPeriodID,
		FromCount:       fromCount,
		PricePerSegment: pricePerSegment,
	}
}

func (t *TariffTier) Validate() error {
	if t.TariffPeriodID == uuid.Nil {
		return ErrTariffTierNotFound
	}
	if t.FromCount < 0 {
		return ErrTariffTierInvalidPrice
	}
	if t.PricePerSegment == "" {
		return ErrTariffTierInvalidPrice
	}
	return nil
}
