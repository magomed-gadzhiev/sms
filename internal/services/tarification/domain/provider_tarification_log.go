package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProviderTarificationLog struct {
	ID                     uuid.UUID
	ProviderID             uuid.UUID
	OperatorID             uuid.UUID
	ClientID               uuid.UUID
	MessageID              uuid.UUID
	SegmentCount           int
	PricePerSegment        string
	TotalCost              string
	Strategy               TarificationStrategy
	ProviderTariffPlanID   uuid.UUID
	ProviderTariffPeriodID uuid.UUID
	IdempotencyKey         string
	CreatedAt              time.Time
}

func NewProviderTarificationLog(
	providerID, operatorID, clientID, messageID, planID, periodID uuid.UUID,
	strategy TarificationStrategy,
	segmentCount int,
	pricePerSegment, totalCost string,
	idempotencyKey string,
) *ProviderTarificationLog {
	return &ProviderTarificationLog{
		ID:                     uuid.New(),
		ProviderID:             providerID,
		OperatorID:             operatorID,
		ClientID:               clientID,
		MessageID:              messageID,
		SegmentCount:           segmentCount,
		PricePerSegment:        pricePerSegment,
		TotalCost:              totalCost,
		Strategy:               strategy,
		ProviderTariffPlanID:   planID,
		ProviderTariffPeriodID: periodID,
		IdempotencyKey:         idempotencyKey,
		CreatedAt:              time.Now(),
	}
}
