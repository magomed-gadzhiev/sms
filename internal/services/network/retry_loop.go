package network

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// RetryPendingOnce читает все SRA-rows с last_materialize_error_at IS NOT NULL
// и пробует применить материализацию повторно через ApplyAssignmentMaterializers.
// Idempotent: при per-row сбое retry-state остаётся записанным
// (last_materialize_error_at обновится на now()), что приведёт к повторной
// попытке на следующем тике; при success — retry-state очищается helper'ом.
//
// Возвращает error только при невозможности прочитать pending-rows
// (ошибка соединения с БД и т.п.); per-row сбои логируются, но не возвращаются.
//
// LIMIT 100 — мягкий потолок на объём одного тика. Если pending-очередь
// длиннее, остаток обработается на следующих тиках (ORDER BY error_at ASC →
// самые старые в первую очередь).
//
// Plan 4 Task 6: закрывает gap из Plan 3 Task 4 — committed-but-unmaterialized
// SRA остаётся неприменённым до перезапуска worker'а.
func RetryPendingOnce(ctx context.Context, pool *pgxpool.Pool, pm ProviderApplier, rm RouteApplier) error {
	rows, err := pool.Query(ctx, `
		SELECT client_id, provider_set_id, route_set_id
		  FROM subaccount_routing_assignment
		 WHERE last_materialize_error_at IS NOT NULL
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

	SRAPendingRetryGauge.Set(float64(len(batch)))

	for _, p := range batch {
		warnings := ApplyAssignmentMaterializers(ctx, pool, pm, rm, p.clientID, p.providerSet, p.routeSet)
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
	}
	return nil
}

// RunRetryLoop запускает RetryPendingOnce каждые `interval` до отмены ctx.
// При ctx.Done — graceful exit. Используется в cmd/worker для фонового retry
// SRA-rows с зафиксированной ошибкой материализации.
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
