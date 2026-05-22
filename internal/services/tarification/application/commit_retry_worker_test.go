package application

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// ─────────────────────────────────────────────────────────────────────────────
// backoffFor — pure function, проверяем инварианты.
// ─────────────────────────────────────────────────────────────────────────────

func TestBackoffFor(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Second},            // защитная ветка — нулевой attempt
		{attempt: 1, want: time.Second},            // 1s
		{attempt: 2, want: 2 * time.Second},        // 2s
		{attempt: 3, want: 4 * time.Second},        // 4s
		{attempt: 6, want: 32 * time.Second},       // 32s
		{attempt: 10, want: 512 * time.Second},     // 1<<9 = 512s
		{attempt: 12, want: 2048 * time.Second},    // 1<<11 = 2048s < 3600s
		{attempt: 13, want: time.Hour},             // 1<<12 = 4096s > cap → 1h
		{attempt: 20, want: time.Hour},             // cap
		{attempt: 100, want: time.Hour},            // cap после overflow protection
		{attempt: -1, want: time.Second},           // защита от отрицательных
	}
	for _, tc := range cases {
		got := backoffFor(tc.attempt)
		assert.Equal(t, tc.want, got, "backoffFor(%d)", tc.attempt)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// fakeCommitChargeInvoker — управляет ответами CommitCharge по message_id.
// ─────────────────────────────────────────────────────────────────────────────

type fakeCommitChargeInvoker struct {
	mu      sync.Mutex
	resp    map[uuid.UUID]*CommitChargeResult
	err     map[uuid.UUID]error
	calls   map[uuid.UUID]int
	callsMu sync.Mutex
}

func newFakeInvoker() *fakeCommitChargeInvoker {
	return &fakeCommitChargeInvoker{
		resp:  make(map[uuid.UUID]*CommitChargeResult),
		err:   make(map[uuid.UUID]error),
		calls: make(map[uuid.UUID]int),
	}
}

func (f *fakeCommitChargeInvoker) setResp(id uuid.UUID, r *CommitChargeResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resp[id] = r
}

func (f *fakeCommitChargeInvoker) setErr(id uuid.UUID, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err[id] = err
}

func (f *fakeCommitChargeInvoker) CommitCharge(_ context.Context, req *CommitChargeRequest) (*CommitChargeResult, error) {
	f.callsMu.Lock()
	f.calls[req.MessageID]++
	f.callsMu.Unlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.err[req.MessageID]; ok {
		return nil, err
	}
	if r, ok := f.resp[req.MessageID]; ok {
		return r, nil
	}
	return nil, errors.New("no fake response configured for " + req.MessageID.String())
}

func (f *fakeCommitChargeInvoker) callCount(id uuid.UUID) int {
	f.callsMu.Lock()
	defer f.callsMu.Unlock()
	return f.calls[id]
}

// ─────────────────────────────────────────────────────────────────────────────
// fakeRetryRepo — in-memory domain.CommitRetryRepository без Postgres.
// ClaimBatch не делает реального FOR UPDATE; просто забирает entries с
// NextRetryAt <= now и помечает их "claimed" пока fakeTx не завершён.
// ─────────────────────────────────────────────────────────────────────────────

type fakeRetryRepo struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*domain.CommitRetryEntry
}

func newFakeRepo() *fakeRetryRepo {
	return &fakeRetryRepo{entries: make(map[uuid.UUID]*domain.CommitRetryEntry)}
}

func (r *fakeRetryRepo) Enqueue(_ context.Context, e *domain.CommitRetryEntry) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[e.MessageID]; ok {
		return false, nil
	}
	cp := *e
	r.entries[e.MessageID] = &cp
	return true, nil
}

func (r *fakeRetryRepo) ClaimBatch(_ context.Context, _ *sqlx.Tx, limit int, now time.Time) ([]*domain.CommitRetryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.CommitRetryEntry
	for _, e := range r.entries {
		if !e.NextRetryAt.After(now) {
			cp := *e
			out = append(out, &cp)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *fakeRetryRepo) UpdateAttempt(_ context.Context, _ *sqlx.Tx, id uuid.UUID, attempt int, nextAt time.Time, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return errors.New("not found")
	}
	e.AttemptCount = attempt
	e.NextRetryAt = nextAt
	e.LastError = lastErr
	return nil
}

func (r *fakeRetryRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
	return nil
}

func (r *fakeRetryRepo) DeleteTx(_ context.Context, _ *sqlx.Tx, id uuid.UUID) error {
	return r.Delete(context.Background(), id)
}

// fakeTx — test-only domain.Tx. Worker вызывает Rollback/Commit; fake отмечает флаг.
type fakeTx struct {
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Commit() error   { t.committed = true; return nil }
func (t *fakeTx) Rollback() error { t.rolledBack = true; return nil }

func (r *fakeRetryRepo) BeginTx(_ context.Context) (*sqlx.Tx, domain.Tx, error) {
	// Worker использует *sqlx.Tx только как opaque pass-through в Claim/Update/Delete.
	// Наш fake не обращается к tx содержимому — nil ok.
	return nil, &fakeTx{}, nil
}

func (r *fakeRetryRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

func (r *fakeRetryRepo) get(id uuid.UUID) *domain.CommitRetryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[id]; ok {
		cp := *e
		return &cp
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RunOnce outcomes.
// ─────────────────────────────────────────────────────────────────────────────

func newWorker(repo domain.CommitRetryRepository, inv commitChargeInvoker) *CommitRetryWorker {
	return NewCommitRetryWorker(repo, inv, 50, 10, zerolog.New(io.Discard))
}

func seed(repo *fakeRetryRepo, entry *domain.CommitRetryEntry) {
	_, _ = repo.Enqueue(context.Background(), entry)
}

func makeEntry(id uuid.UUID, attempt int, nextAt time.Time) *domain.CommitRetryEntry {
	return &domain.CommitRetryEntry{
		MessageID:      id,
		ClientID:       uuid.New(),
		OperatorID:     uuid.New(),
		SenderName:     "S",
		SegmentCount:   1,
		IdempotencyKey: id.String(),
		AttemptCount:   attempt,
		NextRetryAt:    nextAt,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

func TestRunOnce_SuccessDeletesEntry(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	seed(repo, makeEntry(id, 0, time.Now().UTC().Add(-time.Second)))
	inv.setResp(id, &CommitChargeResult{Committed: true})

	w := newWorker(repo, inv)
	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, repo.count(), "success → entry deleted")
}

func TestRunOnce_AlreadyCommittedDeletesEntry(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	seed(repo, makeEntry(id, 2, time.Now().UTC().Add(-time.Second)))
	inv.setResp(id, &CommitChargeResult{AlreadyCommitted: true})

	w := newWorker(repo, inv)
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, repo.count(), "already_committed → entry deleted")
}

func TestRunOnce_TerminalRejectionDeletesEntry(t *testing.T) {
	cases := []struct {
		name string
		resp *CommitChargeResult
	}{
		{"quota_missing", &CommitChargeResult{QuotaMissing: true}},
		{"sub_insufficient", &CommitChargeResult{SubInsufficient: true}},
		{"agg_insufficient", &CommitChargeResult{AggInsufficient: true}},
		{"no_tariff", &CommitChargeResult{NoTariff: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			inv := newFakeInvoker()
			id := uuid.New()

			seed(repo, makeEntry(id, 1, time.Now().UTC().Add(-time.Second)))
			inv.setResp(id, tc.resp)

			w := newWorker(repo, inv)
			_, err := w.RunOnce(context.Background())
			require.NoError(t, err)
			assert.Equal(t, 0, repo.count(), "terminal rejection → entry deleted")
		})
	}
}

func TestRunOnce_TransportErrorIncrementsAttempt(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	seed(repo, makeEntry(id, 2, time.Now().UTC().Add(-time.Second)))
	inv.setErr(id, errors.New("billing timeout"))

	w := newWorker(repo, inv)
	before := time.Now().UTC()
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, repo.count(), "transient → entry retained")

	got := repo.get(id)
	require.NotNil(t, got)
	assert.Equal(t, 3, got.AttemptCount, "attempt_count++")
	// После attempt=3 backoff=4s; next_retry_at ≈ before + 4s (±10s для CI slack).
	expected := before.Add(4 * time.Second)
	assert.WithinDuration(t, expected, got.NextRetryAt, 10*time.Second)
	assert.Equal(t, "billing timeout", got.LastError)
}

func TestRunOnce_ExhaustedDeletesAndMetric(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	// maxAttempts=10 → при attempt=9, транзиентный исход приведёт к attempt=10=max → delete.
	seed(repo, makeEntry(id, 9, time.Now().UTC().Add(-time.Second)))
	inv.setErr(id, errors.New("still down"))

	w := newWorker(repo, inv)
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, repo.count(), "exhausted → entry deleted")
}

func TestRunOnce_NotReadyEntriesSkipped(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()

	// ready сейчас
	readyID := uuid.New()
	seed(repo, makeEntry(readyID, 0, time.Now().UTC().Add(-time.Second)))
	inv.setResp(readyID, &CommitChargeResult{Committed: true})

	// готов через 1 час
	futureID := uuid.New()
	seed(repo, makeEntry(futureID, 0, time.Now().UTC().Add(time.Hour)))
	inv.setResp(futureID, &CommitChargeResult{Committed: true})

	w := newWorker(repo, inv)
	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "обработан только ready entry")
	assert.Equal(t, 1, repo.count(), "future entry остался в очереди")
	assert.NotNil(t, repo.get(futureID))
	assert.Nil(t, repo.get(readyID))

	assert.Equal(t, 1, inv.callCount(readyID))
	assert.Equal(t, 0, inv.callCount(futureID))
}

func TestRunOnce_EmptyQueueNoOp(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()

	w := newWorker(repo, inv)
	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestRunOnce_MixedBatch(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()

	// 1. success
	successID := uuid.New()
	seed(repo, makeEntry(successID, 0, time.Now().UTC().Add(-time.Second)))
	inv.setResp(successID, &CommitChargeResult{Committed: true})

	// 2. terminal
	termID := uuid.New()
	seed(repo, makeEntry(termID, 0, time.Now().UTC().Add(-time.Second)))
	inv.setResp(termID, &CommitChargeResult{QuotaMissing: true})

	// 3. transient, не exhausted
	transID := uuid.New()
	seed(repo, makeEntry(transID, 3, time.Now().UTC().Add(-time.Second)))
	inv.setErr(transID, errors.New("5xx"))

	// 4. exhausted
	exhaustID := uuid.New()
	seed(repo, makeEntry(exhaustID, 9, time.Now().UTC().Add(-time.Second)))
	inv.setErr(exhaustID, errors.New("5xx"))

	w := newWorker(repo, inv)
	n, err := w.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	assert.Nil(t, repo.get(successID), "success deleted")
	assert.Nil(t, repo.get(termID), "terminal deleted")
	assert.Nil(t, repo.get(exhaustID), "exhausted deleted")

	remain := repo.get(transID)
	require.NotNil(t, remain)
	assert.Equal(t, 4, remain.AttemptCount)
}

func TestRunOnce_NilResponseTreatedAsTransient(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	seed(repo, makeEntry(id, 1, time.Now().UTC().Add(-time.Second)))
	inv.setResp(id, nil) // CommitCharge вернул (nil, nil)

	w := newWorker(repo, inv)
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)

	got := repo.get(id)
	require.NotNil(t, got, "nil response → transient, entry retained")
	assert.Equal(t, 2, got.AttemptCount)
}

func TestRunOnce_UnknownResponseTreatedAsTransient(t *testing.T) {
	repo := newFakeRepo()
	inv := newFakeInvoker()
	id := uuid.New()

	seed(repo, makeEntry(id, 0, time.Now().UTC().Add(-time.Second)))
	inv.setResp(id, &CommitChargeResult{}) // все флаги false

	w := newWorker(repo, inv)
	_, err := w.RunOnce(context.Background())
	require.NoError(t, err)

	got := repo.get(id)
	require.NotNil(t, got, "unknown outcome → transient")
	assert.Equal(t, 1, got.AttemptCount)
}
