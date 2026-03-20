package domain

import "context"

type EventPublisher interface {
	PublishTarificationResult(ctx context.Context, log *TarificationLog) error
	PublishRecalcEvent(ctx context.Context, clientID string, tariffPlanID, tariffPeriodID string, oldPrice, newPrice string, affectedSegments int, recalcAmount string, recalcType string) error
	PublishPrepaidEvent(ctx context.Context, clientID string, tariffPlanID, tariffPeriodID string, amount, currency string) error
	Close() error
}
