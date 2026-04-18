package domain

import (
	"time"

	"github.com/google/uuid"
)

type PriceOwnerType string

const (
	OwnerPlatform   PriceOwnerType = "platform"
	OwnerAggregator PriceOwnerType = "aggregator"
	OwnerSubaccount PriceOwnerType = "subaccount"
)

type PriceModelType string

const (
	ModelFixed            PriceModelType = "fixed"
	ModelTiered           PriceModelType = "tiered"
	ModelPrepaidThreshold PriceModelType = "prepaid_threshold"
)

// PriceRule — единое правило ценообразования. Применяется на 3 уровнях владения
// (platform/aggregator/subaccount) с наследованием и точечными override.
// Измерения (Country/Operator/SenderCategory/TrafficType) — NULL = wildcard (любое значение).
// Lookup использует owner-first + specificity bitmap (traffic_type=8, sender_category=4,
// operator=2, country=1). См. docs/superpowers/specs/2026-04-18-unified-pricing-model-design.md
type PriceRule struct {
	ID             uuid.UUID
	OwnerType      PriceOwnerType
	OwnerID        *uuid.UUID
	Country        *string
	Operator       *string
	SenderCategory *string
	TrafficType    *string
	ValidFrom      time.Time
	ValidTo        *time.Time
	PriceModel     PriceModelType
	// PriceValue хранится как строка (NUMERIC в БД). Конвертация — в application-слое
	// при расчёте. Это позволяет избежать потерь точности в domain.
	PriceValue *string
	// TiersJSON — raw JSONB для моделей tiered/prepaid_threshold. Парсится в
	// application.TiersCache с LRU-кешем.
	TiersJSON []byte
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy *uuid.UUID
}

func (p *PriceRule) Validate() error {
	if p.OwnerType == OwnerPlatform && p.OwnerID != nil {
		return ErrOwnerIDMismatch
	}
	if p.OwnerType != OwnerPlatform && p.OwnerID == nil {
		return ErrOwnerIDMismatch
	}
	if p.PriceModel == ModelFixed {
		if p.PriceValue == nil || len(p.TiersJSON) > 0 {
			return ErrPriceSpecMismatch
		}
	} else {
		if len(p.TiersJSON) == 0 || p.PriceValue != nil {
			return ErrPriceSpecMismatch
		}
	}
	if p.ValidTo != nil && !p.ValidTo.After(p.ValidFrom) {
		return ErrInvalidPeriod
	}
	return nil
}

// SpecificityBitmap — битмап специфичности по измерениям.
// Порядок приоритета: traffic_type > sender_category > operator > country.
// Более высокое значение побеждает в lookup при одинаковом owner_rank.
func (p *PriceRule) SpecificityBitmap() int {
	bm := 0
	if p.Country != nil && *p.Country != "" {
		bm |= 1
	}
	if p.Operator != nil && *p.Operator != "" {
		bm |= 2
	}
	if p.SenderCategory != nil && *p.SenderCategory != "" {
		bm |= 4
	}
	if p.TrafficType != nil && *p.TrafficType != "" {
		bm |= 8
	}
	return bm
}
