// Package network provides shared logic for sub-account routing assignments.
package network

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ProviderApplier и RouteApplier — minimal interfaces для подмены в тестах.
// Совпадают сигнатурой с network.ProviderSetMaterializer / RouteSetMaterializer.
type ProviderApplier interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, providerSetID *uuid.UUID) error
}

type RouteApplier interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, routeSetID *uuid.UUID) error
}

// ApplyAssignmentMaterializers вызывает обоих materializer'ов и обновляет
// retry-state в subaccount_routing_assignment. Контракт:
//   - оба success → сбросить last_materialize_error_at/text/retry_count, warnings = nil.
//   - любой fail → записать last_materialize_error_at = now(), incr retry_count, вернуть warnings.
//
// Helper заменяет 4 дубликата из network_assignments.go (PutOne provider/route и
// Bulk provider/route), накопившиеся в Plan 3 Task 4.
//
// Параметр `source` (Plan 5 Task 2 / A5) — "initial" для вызовов из portal
// handlers (PutOne / Bulk), "retry" для вызовов из retry_loop. Разводит
// MaterializeFailureTotal так, чтобы ops-алёрт по rate(...{source="initial"}[5m])
// не получал шум от per-tick re-attempts по уже стоящему stuck-row.
func ApplyAssignmentMaterializers(
	ctx context.Context,
	pool *pgxpool.Pool,
	pm ProviderApplier,
	rm RouteApplier,
	clientID uuid.UUID,
	providerSetID *uuid.UUID,
	routeSetID *uuid.UUID,
	source string,
) []map[string]string {
	warnings := []map[string]string{}
	if err := pm.ApplyToClient(ctx, clientID, providerSetID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("provider materialize partial-failure")
		MaterializeFailureTotal.WithLabelValues("provider", source).Inc()
		warnings = append(warnings, map[string]string{"step": "provider_materialize", "error": err.Error()})
	}
	if err := rm.ApplyToClient(ctx, clientID, routeSetID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("route materialize partial-failure")
		MaterializeFailureTotal.WithLabelValues("route", source).Inc()
		warnings = append(warnings, map[string]string{"step": "route_materialize", "error": err.Error()})
	}

	if len(warnings) == 0 {
		if _, err := pool.Exec(ctx, `
			UPDATE subaccount_routing_assignment
			   SET last_materialize_error_at = NULL,
			       last_materialize_error_text = NULL,
			       materialize_retry_count = 0
			 WHERE client_id = $1`, clientID); err != nil {
			log.Warn().Err(err).Str("client_id", clientID.String()).Msg("clear retry-state failed")
		}
		return nil
	}

	parts := make([]string, 0, len(warnings))
	for _, w := range warnings {
		parts = append(parts, w["step"]+": "+w["error"])
	}
	errText := strings.Join(parts, "; ")
	if _, err := pool.Exec(ctx, `
		UPDATE subaccount_routing_assignment
		   SET last_materialize_error_at = now(),
		       last_materialize_error_text = $2,
		       materialize_retry_count = materialize_retry_count + 1
		 WHERE client_id = $1`, clientID, errText); err != nil {
		log.Warn().Err(err).Str("client_id", clientID.String()).Msg("record retry-state failed")
	}
	return warnings
}
