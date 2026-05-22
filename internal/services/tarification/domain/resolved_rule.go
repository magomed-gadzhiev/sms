package domain

import (
	"time"

	"github.com/google/uuid"
)

// ResolvedRule — материализованный выбор правила для конкретного
// (subaccount_id, измерения, effective_date). Читается на горячем пути по PK.
// Источник — price_rules; при изменении источника строка удаляется janitor'ом
// по событию из resolved_rules_invalidation_outbox.
type ResolvedRule struct {
	SubaccountID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	// EffectiveDate хранится как DATE (UTC midnight). Вычисляется из
	// in.Now.UTC().Truncate(24*time.Hour) при resolve.
	EffectiveDate time.Time

	PriceModel   PriceModelType
	PriceValue   *string
	TiersJSON    []byte
	SourceRuleID uuid.UUID
	SourceLevel  PriceOwnerType
	// AggregatorID денормализован сюда для быстрой инвалидации по агрегатору
	// (одно DELETE WHERE aggregator_id=X, без JOIN с clients).
	AggregatorID uuid.UUID

	ResolvedAt   time.Time
	RulesVersion int64
}
