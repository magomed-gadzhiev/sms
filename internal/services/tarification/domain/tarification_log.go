package domain

import (
	"time"

	"github.com/google/uuid"
)

type TarificationLog struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	MessageID       uuid.UUID
	OperatorID      uuid.UUID
	SenderCategory  SenderCategory
	Strategy        TarificationStrategy
	TariffPlanID    uuid.UUID
	TariffPeriodID  uuid.UUID
	SegmentCount    int
	PricePerSegment string
	TotalAmount     string
	RecalcAmount    *string
	IdempotencyKey  string
	CreatedAt       time.Time
}

func NewTarificationLog(
	clientID, messageID, operatorID, tariffPlanID, tariffPeriodID uuid.UUID,
	senderCategory SenderCategory,
	strategy TarificationStrategy,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog {
	return &TarificationLog{
		ID:              uuid.New(),
		ClientID:        clientID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SenderCategory:  senderCategory,
		Strategy:        strategy,
		TariffPlanID:    tariffPlanID,
		TariffPeriodID:  tariffPeriodID,
		SegmentCount:    segmentCount,
		PricePerSegment: pricePerSegment,
		TotalAmount:     totalAmount,
		IdempotencyKey:  idempotencyKey,
		CreatedAt:       time.Now(),
	}
}
