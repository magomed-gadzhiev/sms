package application

import (
	"context"
)

// PrepaidThresholdStrategy реализует стратегию предоплаты с порогами
// Абонентская плата списывается отдельно в начале периода
// Сообщения тарифицируются по порогам (как ThresholdStrategy)
type PrepaidThresholdStrategy struct {
	threshold *ThresholdStrategy
}

// NewPrepaidThresholdStrategy создает стратегию предоплаты с порогами
func NewPrepaidThresholdStrategy() *PrepaidThresholdStrategy {
	return &PrepaidThresholdStrategy{
		threshold: NewThresholdStrategy(),
	}
}

// Calculate вычисляет стоимость по порогам (предоплата обрабатывается отдельно)
func (s *PrepaidThresholdStrategy) Calculate(ctx context.Context, params CalculationParams) (*CalculationResult, error) {
	return s.threshold.Calculate(ctx, params)
}
