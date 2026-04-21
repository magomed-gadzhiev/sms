package application

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// CommitRetryWorker периодически забирает из commit_retry_queue записи
// с next_retry_at <= NOW и повторно вызывает CommitCharge.
//
// Жизненный цикл записи в очереди:
//  1. TarificationService.CommitCharge при transport-level ошибке enqueue-ит
//     запись с attempt_count=0, next_retry_at=NOW+backoff(0).
//  2. Worker claim-ит пакет FOR UPDATE SKIP LOCKED, для каждой записи вызывает
//     CommitCharge заново (тот же код, что и в hot-path).
//  3. Если CommitCharge вернул Committed или AlreadyCommitted — запись
//     удаляется (success). Метрика: outcome=success.
//  4. Если terminal business error (QuotaMissing/SubInsufficient/AggInsufficient/
//     NoTariff) — запись удаляется (retry не поможет). Метрика: outcome=terminal.
//     Лог уровня warn — финансы не сошлись, но система не виновата.
//  5. Если снова transport error — UPDATE attempt_count++, next_retry_at через
//     экспоненциальный backoff. Если attempt_count >= MaxAttempts — DELETE,
//     инкремент commit_retry_exhausted_total, лог уровня error (требует
//     ручного разбора — сообщение ушло, деньги не списаны).
type CommitRetryWorker struct {
	repo        domain.CommitRetryRepository
	service     commitChargeInvoker
	batchSize   int
	maxAttempts int
	logger      zerolog.Logger
}

// commitChargeInvoker изолирует TarificationService для тестирования worker'а
// без подтягивания всего сервиса. В prod — TarificationService сам реализует
// этот интерфейс через CommitCharge.
type commitChargeInvoker interface {
	CommitCharge(ctx context.Context, req *CommitChargeRequest) (*CommitChargeResult, error)
}

// NewCommitRetryWorker. maxAttempts=10 по умолчанию (~17 минут с экспоненциальным
// backoff от 1s до 1h с cap'ом на 1h — примерно сутки до exhausted).
func NewCommitRetryWorker(
	repo domain.CommitRetryRepository,
	service commitChargeInvoker,
	batchSize int,
	maxAttempts int,
	logger zerolog.Logger,
) *CommitRetryWorker {
	if batchSize <= 0 {
		batchSize = 50
	}
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	return &CommitRetryWorker{
		repo:        repo,
		service:     service,
		batchSize:   batchSize,
		maxAttempts: maxAttempts,
		logger:      logger,
	}
}

// Run запускает цикл с интервалом interval. Останавливается при ctx.Done.
func (w *CommitRetryWorker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	w.logger.Info().Dur("interval", interval).Int("batch_size", w.batchSize).Int("max_attempts", w.maxAttempts).Msg("commit retry worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("commit retry worker stopped")
			return
		case <-tick.C:
			if n, err := w.RunOnce(ctx); err != nil {
				w.logger.Error().Err(err).Msg("commit retry run_once failed")
			} else if n > 0 {
				w.logger.Debug().Int("processed", n).Msg("commit retry batch processed")
			}
		}
	}
}

// RunOnce обрабатывает один батч. Возвращает число обработанных записей.
// Каждая запись — в своей транзакции (tx для claim+update), чтобы ошибка
// в одной не держала lock на остальных.
func (w *CommitRetryWorker) RunOnce(ctx context.Context) (int, error) {
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	// Даже при успехе — commit в конце; при early-return rollback освобождает lock'и.
	defer func() { _ = tx.Rollback() }()

	entries, err := w.repo.ClaimBatch(ctx, tx, w.batchSize, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("claim batch: %w", err)
	}
	if len(entries) == 0 {
		return 0, nil
	}

	// Все operations — внутри tx (claim, update, delete). row locked FOR UPDATE,
	// Commit освобождает lock'и одним атомом. Если Commit упадёт — ни один
	// processed outcome не применится (вариант отличный от прежнего split'а,
	// где DELETE был вне tx и при его сбое метрика success переоценивалась).
	for _, e := range entries {
		outcome, lastErr := w.processEntry(ctx, e)
		switch outcome {
		case "success", "terminal":
			if err := w.repo.DeleteTx(ctx, tx, e.MessageID); err != nil {
				w.logger.Warn().Err(err).Str("message_id", e.MessageID.String()).Msg("delete entry failed")
			}
		case "transient":
			nextAttempt := e.AttemptCount + 1
			if nextAttempt >= w.maxAttempts {
				if err := w.repo.DeleteTx(ctx, tx, e.MessageID); err != nil {
					w.logger.Warn().Err(err).Str("message_id", e.MessageID.String()).Msg("delete entry failed")
				}
				commitRetryExhaustedTotal.Inc()
				w.logger.Error().
					Str("message_id", e.MessageID.String()).
					Str("client_id", e.ClientID.String()).
					Int("attempts", nextAttempt).
					Str("last_error", lastErr).
					Msg("commit retry exhausted — message sent but not charged, manual reconciliation required")
				commitRetryProcessedTotal.WithLabelValues(outcome).Inc()
				continue
			}
			nextAt := time.Now().UTC().Add(backoffFor(nextAttempt))
			if err := w.repo.UpdateAttempt(ctx, tx, e.MessageID, nextAttempt, nextAt, lastErr); err != nil {
				w.logger.Warn().Err(err).Str("message_id", e.MessageID.String()).Msg("update attempt failed")
			}
		}
		commitRetryProcessedTotal.WithLabelValues(outcome).Inc()
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return len(entries), nil
}

// processEntry возвращает ("success"|"terminal"|"transient", lastErrString).
func (w *CommitRetryWorker) processEntry(ctx context.Context, e *domain.CommitRetryEntry) (string, string) {
	// Тайм-аут на один retry — не держим lock долго.
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := w.service.CommitCharge(callCtx, &CommitChargeRequest{
		MessageID:      e.MessageID,
		ClientID:       e.ClientID,
		OperatorID:     e.OperatorID,
		SenderName:     e.SenderName,
		SegmentCount:   e.SegmentCount,
		IdempotencyKey: e.IdempotencyKey,
	})
	if err != nil {
		// Transport/timeout — ретраим.
		return "transient", err.Error()
	}
	if resp == nil {
		return "transient", "nil response"
	}
	switch {
	case resp.Committed, resp.AlreadyCommitted:
		return "success", ""
	case resp.QuotaMissing, resp.SubInsufficient, resp.AggInsufficient, resp.NoTariff:
		// Business error — retry не изменит результат до ручного вмешательства
		// (квота, баланс). Удаляем из очереди, остаётся лог для оператора.
		w.logger.Warn().
			Str("message_id", e.MessageID.String()).
			Str("client_id", e.ClientID.String()).
			Bool("quota_missing", resp.QuotaMissing).
			Bool("sub_insufficient", resp.SubInsufficient).
			Bool("agg_insufficient", resp.AggInsufficient).
			Bool("no_tariff", resp.NoTariff).
			Msg("commit retry terminal — message sent but charge rejected, manual reconciliation required")
		return "terminal", "business rejection"
	default:
		// Consistent неизвестный негатив — расцениваем как transient.
		return "transient", "unknown non-committed response"
	}
}

// backoffFor — экспоненциальный backoff с cap'ом 1h.
// attempt=1 → 1s, attempt=2 → 2s, ..., attempt=12 → 1h (дальше cap).
func backoffFor(attempt int) time.Duration {
	const cap = time.Hour
	if attempt <= 0 {
		return time.Second
	}
	// Shift безопасен до attempt ~30; после clamp'им через cap. Но уже при
	// attempt=12 получаем 4096s > 3600s cap.
	if attempt > 30 {
		return cap
	}
	d := time.Second << uint(attempt-1)
	if d > cap || d <= 0 {
		return cap
	}
	return d
}
