package network

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// RetryPendingOnce читает SRA-rows с last_materialize_error_at IS NOT NULL
// и materialize_retry_count < 100 (cap) и пробует применить материализацию
// повторно через ApplyAssignmentMaterializers. Idempotent: при per-row сбое
// retry-state остаётся записанным (last_materialize_error_at обновится на now()),
// что приведёт к повторной попытке на следующем тике; при success —
// retry-state очищается helper'ом.
//
// Возвращает error только при невозможности прочитать pending-rows
// (ошибка соединения с БД и т.п.); per-row сбои логируются, но не возвращаются.
//
// LIMIT 100 — мягкий потолок на объём одного тика. Если pending-очередь
// длиннее, остаток обработается на следующих тиках (ORDER BY error_at ASC →
// самые старые в первую очередь). Cap=100 по retry_count: один тик = ~1 минута,
// row висящая 100 минут — реальный broken state; такие rows excluded из SELECT
// и сигнализируются через SRARetryGiveUpGauge для ops.
//
// Plan 4 Task 6: закрывает gap из Plan 3 Task 4 — committed-but-unmaterialized
// SRA остаётся неприменённым до перезапуска worker'а.
// Plan 6 Task 2: добавлен WHERE materialize_retry_count < 100 и give-up gauge.
func RetryPendingOnce(ctx context.Context, pool *pgxpool.Pool, pm ProviderApplier, rm RouteApplier) error {
	rows, err := pool.Query(ctx, `
		SELECT client_id, provider_set_id, route_set_id
		  FROM subaccount_routing_assignment
		 WHERE last_materialize_error_at IS NOT NULL
		   AND materialize_retry_count < 100
		 ORDER BY last_materialize_error_at ASC
		 LIMIT 100`)
	if err != nil {
		return err
	}
	type pending struct {
		clientID    uuid.UUID
		providerSet *uuid.UUID
		routeSet    *uuid.UUID
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.clientID, &p.providerSet, &p.routeSet); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var giveUpCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM subaccount_routing_assignment
		  WHERE last_materialize_error_at IS NOT NULL
		    AND materialize_retry_count >= 100`).Scan(&giveUpCount); err != nil {
		log.Warn().Err(err).Msg("SRA give-up count query failed")
	} else {
		SRARetryGiveUpGauge.Set(float64(giveUpCount))
	}

	SRAPendingRetryGauge.Set(float64(len(batch)))

	// Plan 5 Task 4 (B4): backlog signal. Batch == LIMIT means there may be more
	// pending rows than this tick can drain; ops needs a counter, not just a
	// gauge capped at 100, to see "always at limit" trend.
	if len(batch) >= 100 {
		SRARetryBacklogOverflowTotal.Inc()
		log.Warn().
			Int("batch_size", len(batch)).
			Msg("SRA retry backlog at LIMIT — overflow possible, increase tick frequency or investigate stuck rows")
	}

	// Plan 5 Task 4 (B2): per-row defer/recover. Per-row scope means a panic
	// inside a materializer (e.g. nil-deref in pgx scan) doesn't kill the loop —
	// remaining rows are still processed. Per-row defer/recover: panic в
	// materializer'е считается как failure (counted as
	// MaterializeFailureTotal{operation=retry_panic,source=retry}); per-row
	// scope значит loop продолжает обработку остальных rows.
	for _, p := range batch {
		func(p pending) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Str("client_id", p.clientID.String()).
						Msg("SRA retry per-row panic — counted as failure, loop continues")
					MaterializeFailureTotal.WithLabelValues("retry_panic", "retry").Inc()
				}
			}()
			warnings := ApplyAssignmentMaterializers(ctx, pool, pm, rm, p.clientID, p.providerSet, p.routeSet, "retry")
			if len(warnings) == 0 {
				log.Info().
					Str("client_id", p.clientID.String()).
					Msg("SRA retry succeeded — materialization applied, retry-state cleared")
			} else {
				log.Warn().
					Str("client_id", p.clientID.String()).
					Int("warnings", len(warnings)).
					Msg("SRA retry still failing — retry-state preserved for next tick")
			}
		}(p)
	}
	return nil
}

// RunRetryLoop запускает RetryPendingOnce каждые `interval` до отмены ctx.
// При ctx.Done — graceful exit. Запускается в cmd/pipeline-worker (maintenance
// loops) для фонового retry SRA-rows с зафиксированной ошибкой материализации.
func RunRetryLoop(ctx context.Context, pool *pgxpool.Pool, pm ProviderApplier, rm RouteApplier, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	log.Info().Dur("interval", interval).Msg("SRA retry loop started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("SRA retry loop stopped")
			return
		case <-t.C:
			if err := RetryPendingOnce(ctx, pool, pm, rm); err != nil {
				log.Error().Err(err).Msg("SRA retry loop iteration failed")
			}
		}
	}
}
