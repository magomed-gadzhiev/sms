package domain

import (
	"time"

	"github.com/google/uuid"
)

type TarificationLog struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	MessageID      uuid.UUID
	OperatorID     uuid.UUID
	SenderCategory SenderCategory
	Strategy       TarificationStrategy
	// TariffPlanID/TariffPeriodID nullable: заполнены для legacy пути,
	// nil для unified (см. 2026-04-19-tarification-log-nullable-plan-design.md).
	TariffPlanID   *uuid.UUID
	TariffPeriodID *uuid.UUID
	// SourceRuleID заполнен для unified пути; nil для legacy. Ссылка на
	// price_rules.id, без FK (правила могут удаляться).
	SourceRuleID    *uuid.UUID
	SegmentCount    int
	PricePerSegment string
	TotalAmount     string
	RecalcAmount    *string
	IdempotencyKey  string
	CreatedAt       time.Time
}

// NewTarificationLog — конструктор для legacy-пути (plan + period обязательны).
func NewTarificationLog(
	clientID, messageID, operatorID, tariffPlanID, tariffPeriodID uuid.UUID,
	senderCategory SenderCategory,
	strategy TarificationStrategy,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog {
	planID := tariffPlanID
	periodID := tariffPeriodID
	return &TarificationLog{
		ID:              uuid.New(),
		ClientID:        clientID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SenderCategory:  senderCategory,
		Strategy:        strategy,
		TariffPlanID:    &planID,
		TariffPeriodID:  &periodID,
		SegmentCount:    segmentCount,
		PricePerSegment: pricePerSegment,
		TotalAmount:     totalAmount,
		IdempotencyKey:  idempotencyKey,
		CreatedAt:       time.Now(),
	}
}

// NewUnifiedTarificationLog — конструктор для unified hot-path. Plan/period
// отсутствуют, вместо них source_rule_id указывает на price_rules.id.
// Strategy фиксирована как StrategyUnified.
func NewUnifiedTarificationLog(
	clientID, messageID, operatorID, sourceRuleID uuid.UUID,
	senderCategory SenderCategory,
	segmentCount int,
	pricePerSegment, totalAmount string,
	idempotencyKey string,
) *TarificationLog {
	ruleID := sourceRuleID
	return &TarificationLog{
		ID:              uuid.New(),
		ClientID:        clientID,
		MessageID:       messageID,
		OperatorID:      operatorID,
		SenderCategory:  senderCategory,
		Strategy:        StrategyUnified,
		SourceRuleID:    &ruleID,
		SegmentCount:    segmentCount,
		PricePerSegment: pricePerSegment,
		TotalAmount:     totalAmount,
		IdempotencyKey:  idempotencyKey,
		CreatedAt:       time.Now(),
	}
}
