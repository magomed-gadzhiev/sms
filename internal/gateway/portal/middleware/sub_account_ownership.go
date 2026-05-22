package middleware

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// VerifySubAccountOwnership возвращает 404, если subID не суб-аккаунт текущего
// reseller'а. 404 (а не 403) — чтобы не светить наличие чужих client_id.
// Connection / scan errors отделяются и возвращают 500, чтобы не маскировать
// инфраструктурные сбои под "не найдено".
//
// Ранее — duplicate в network_assignments.go и subaccount_network_overrides.go.
// Plan 3 Task 7 (B5): extracted здесь, чтобы будущие endpoint'ы под
// /sub-accounts/* использовали единый источник правды.
func VerifySubAccountOwnership(ctx context.Context, pool *pgxpool.Pool, resellerID, subID uuid.UUID) *shared.AppError {
	var parent uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT parent_client_id FROM clients WHERE id = $1 AND parent_client_id IS NOT NULL`,
		subID,
	).Scan(&parent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return shared.ErrNotFound("суб-аккаунт")
		}
		log.Error().Err(err).Str("sub_id", subID.String()).Msg("VerifySubAccountOwnership query")
		return shared.ErrInternalServer("verify sub-account ownership")
	}
	if parent != resellerID {
		return shared.ErrNotFound("суб-аккаунт")
	}
	return nil
}
