package application

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ResolvedRulesJanitor потребляет события из resolved_rules_invalidation_outbox
// (публикуются триггером price_rules_after_change) и удаляет затронутые строки
// resolved_rules. Стратегия — ленивая: не пересчитываем, а удаляем; следующий
// запрос в горячем пути реструктурирует нужные строки по запросу.
type ResolvedRulesJanitor struct {
	outbox       domain.InvalidationOutboxRepository
	resolvedRepo domain.ResolvedRulesRepository
	batchSize    int
	logger       zerolog.Logger
}

func NewResolvedRulesJanitor(
	outbox domain.InvalidationOutboxRepository,
	resolvedRepo domain.ResolvedRulesRepository,
	batchSize int,
	logger zerolog.Logger,
) *ResolvedRulesJanitor {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &ResolvedRulesJanitor{
		outbox:       outbox,
		resolvedRepo: resolvedRepo,
		batchSize:    batchSize,
		logger:       logger,
	}
}

// RunOnce обрабатывает один батч событий. Возвращает число обработанных.
// Используется в цикле через Run или вызывается напрямую из тестов.
func (j *ResolvedRulesJanitor) RunOnce(ctx context.Context) (int, error) {
	events, err := j.outbox.ListUnprocessed(ctx, j.batchSize)
	if err != nil {
		return 0, fmt.Errorf("list unprocessed: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}
	var processed []int64
	for _, ev := range events {
		deleted, err := j.resolvedRepo.DeleteAffected(ctx,
			ev.OwnerType, ev.OwnerID, ev.Country, ev.Operator, ev.SenderCat, ev.Traffic,
		)
		if err != nil {
			// один неудачный event не должен блокировать батч; логируем и идём дальше.
			// Необработанный event останется в outbox — следующий прогон попытается снова.
			j.logger.Warn().
				Err(err).
				Int64("event_id", ev.ID).
				Str("owner_type", string(ev.OwnerType)).
				Msg("delete_affected failed")
			continue
		}
		j.logger.Debug().
			Int64("event_id", ev.ID).
			Int64("deleted", deleted).
			Str("owner_type", string(ev.OwnerType)).
			Msg("resolved_rules invalidated")
		processed = append(processed, ev.ID)
	}
	if len(processed) > 0 {
		if err := j.outbox.MarkProcessed(ctx, processed); err != nil {
			return len(processed), fmt.Errorf("mark processed: %w", err)
		}
	}
	return len(processed), nil
}

// Run запускает loop с фиксированным интервалом. Остановка по ctx.Done.
func (j *ResolvedRulesJanitor) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			j.logger.Info().Msg("resolved_rules_janitor stopped")
			return
		case <-ticker.C:
			if _, err := j.RunOnce(ctx); err != nil {
				j.logger.Warn().Err(err).Msg("janitor iteration failed")
			}
		}
	}
}
