package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// CommitIdempotencyGuard — INSERT ... ON CONFLICT DO NOTHING в crow PK=message_id
// таблицу commit_idempotency_guard. Ставит метку «этот message_id обрабатывается
// прямо сейчас» в той же tx что и остальные shifts/balances dual-charge.
//
// Причина отдельной таблицы: aggregator_margin_log partitioned by created_at,
// unique (idempotency_key, created_at) композитный — Postgres не разрешает
// solo-unique на partitioned table. Guard — non-partitioned, PK=message_id.
type CommitIdempotencyGuard interface {
	// ClaimTx пытается insert'нуть message_id в guard. Возвращает inserted=true
	// если это первое обращение (ранее не было). false — уже было (ALREADY_COMMITTED).
	// Выполняется внутри переданной tx — conflict откатывается при Rollback.
	ClaimTx(ctx context.Context, tx *sqlx.Tx, messageID uuid.UUID) (bool, error)
}
