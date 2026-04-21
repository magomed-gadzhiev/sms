package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// CommitRetryEntry — строка commit_retry_queue: отложенный CommitCharge после
// transport/timeout ошибки. Worker забирает записи с next_retry_at <= NOW,
// вызывает повторный CommitCharge и на успех удаляет row, иначе обновляет
// attempt_count и next_retry_at через экспоненциальный backoff.
type CommitRetryEntry struct {
	MessageID      uuid.UUID
	ClientID       uuid.UUID
	OperatorID     uuid.UUID
	SenderName     string
	SegmentCount   int
	IdempotencyKey string
	AttemptCount   int
	LastError      string
	NextRetryAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CommitRetryRepository — контракт persistent-очереди. Реализация в
// infrastructure/repository/commit_retry_repository.go.
type CommitRetryRepository interface {
	// Enqueue — INSERT ... ON CONFLICT (message_id) DO NOTHING. Возвращает
	// inserted=true, если строка реально создана. false означает, что запись
	// уже была (idempotent — второй enqueue того же message_id не дубль).
	Enqueue(ctx context.Context, entry *CommitRetryEntry) (bool, error)

	// ClaimBatch — забирает batch готовых к retry записей в транзакции
	// с FOR UPDATE SKIP LOCKED для конкурентных worker-инстансов.
	// Caller обязан ком/роллбечнуть tx.
	ClaimBatch(ctx context.Context, tx *sqlx.Tx, limit int, now time.Time) ([]*CommitRetryEntry, error)

	// UpdateAttempt — для неуспешного retry: UPDATE attempt_count, next_retry_at,
	// last_error. В той же tx что и ClaimBatch (row locked).
	UpdateAttempt(ctx context.Context, tx *sqlx.Tx, messageID uuid.UUID, attemptCount int, nextRetryAt time.Time, lastError string) error

	// Delete — для успешного retry или permanent failure (attempts exhausted).
	// Не требует tx.
	Delete(ctx context.Context, messageID uuid.UUID) error

	// DeleteTx — та же операция, но внутри транзакции worker'а (row уже
	// locked через ClaimBatch FOR UPDATE). Предпочтительна для worker'а,
	// чтобы delete/update coexistовали атомарно с claim.
	DeleteTx(ctx context.Context, tx *sqlx.Tx, messageID uuid.UUID) error

	// BeginTx — хелпер для callers, которым нужна транзакция для ClaimBatch/UpdateAttempt.
	BeginTx(ctx context.Context) (*sqlx.Tx, error)
}
