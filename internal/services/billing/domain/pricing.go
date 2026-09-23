package domain

import (
	"time"

	"github.com/google/uuid"
)

// PricingRule представляет доменную модель правила тарификации
type PricingRule struct {
	ID                 uuid.UUID
	ClientID           *uuid.UUID // NULL для глобальных правил
	DestinationPattern string     // regex паттерн
	PricePerMessage    string     // Цена за сообщение
	Currency           string
	Priority           int
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewPricingRule создает новое правило тарификации
func NewPricingRule(
	clientID *uuid.UUID,
	destinationPattern, pricePerMessage, currency string,
	priority int,
) *PricingRule {
	now := time.Now()
	return &PricingRule{
		ID:                 uuid.New(),
		ClientID:           clientID,
		DestinationPattern: destinationPattern,
		PricePerMessage:    pricePerMessage,
		Currency:           currency,
		Priority:           priority,
		Active:             true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// Validate валидирует правило тарификации
func (p *PricingRule) Validate() error {
	if p.DestinationPattern == "" {
		return ErrPricingRulePatternRequired
	}
	if p.PricePerMessage == "" {
		return ErrPricingRulePriceRequired
	}
	if p.Currency == "" {
		return ErrPricingRuleCurrencyRequired
	}
	if p.Priority < 0 {
		return ErrPricingRulePriorityInvalid
	}
	return nil
}

// IsGlobal проверяет, является ли правило глобальным
func (p *PricingRule) IsGlobal() bool {
	return p.ClientID == nil
}

// Activate активирует правило
func (p *PricingRule) Activate() {
	p.Active = true
	p.UpdatedAt = time.Now()
}

// Deactivate деактивирует правило
func (p *PricingRule) Deactivate() {
	p.Active = false
	p.UpdatedAt = time.Now()
}
