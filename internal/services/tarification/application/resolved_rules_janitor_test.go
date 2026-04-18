package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type fakeOutbox struct {
	events    []domain.InvalidationEvent
	processed []int64
}

func (f *fakeOutbox) ListUnprocessed(_ context.Context, limit int) ([]domain.InvalidationEvent, error) {
	if len(f.events) == 0 {
		return nil, nil
	}
	n := limit
	if n > len(f.events) {
		n = len(f.events)
	}
	out := f.events[:n]
	f.events = f.events[n:]
	return out, nil
}

func (f *fakeOutbox) MarkProcessed(_ context.Context, ids []int64) error {
	f.processed = append(f.processed, ids...)
	return nil
}

type recordingResolvedRepo struct {
	deleteCalls []string
	failOn      int64 // fail if event_id matches
}

func (r *recordingResolvedRepo) Get(context.Context, uuid.UUID, string, string, string, string, time.Time) (*domain.ResolvedRule, error) {
	return nil, nil
}
func (r *recordingResolvedRepo) Upsert(context.Context, *domain.ResolvedRule) error { return nil }
func (r *recordingResolvedRepo) DeleteAffected(_ context.Context, ot domain.PriceOwnerType, oid *uuid.UUID, _, _, _, _ *string) (int64, error) {
	key := string(ot)
	if oid != nil {
		key += "/" + oid.String()
	}
	r.deleteCalls = append(r.deleteCalls, key)
	return 1, nil
}

type failingResolvedRepo struct {
	failCount int
}

func (r *failingResolvedRepo) Get(context.Context, uuid.UUID, string, string, string, string, time.Time) (*domain.ResolvedRule, error) {
	return nil, nil
}
func (r *failingResolvedRepo) Upsert(context.Context, *domain.ResolvedRule) error { return nil }
func (r *failingResolvedRepo) DeleteAffected(context.Context, domain.PriceOwnerType, *uuid.UUID, *string, *string, *string, *string) (int64, error) {
	r.failCount++
	return 0, errors.New("boom")
}

func TestJanitor_ProcessesAllEvents(t *testing.T) {
	subID := uuid.New()
	outbox := &fakeOutbox{events: []domain.InvalidationEvent{
		{ID: 1, OwnerType: domain.OwnerSubaccount, OwnerID: &subID, RuleID: uuid.New()},
		{ID: 2, OwnerType: domain.OwnerPlatform, RuleID: uuid.New()},
	}}
	resolved := &recordingResolvedRepo{}
	j := NewResolvedRulesJanitor(outbox, resolved, 10, zerolog.Nop())

	n, err := j.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, []int64{1, 2}, outbox.processed)
	require.Len(t, resolved.deleteCalls, 2)
}

func TestJanitor_EmptyOutbox_NoOp(t *testing.T) {
	outbox := &fakeOutbox{}
	resolved := &recordingResolvedRepo{}
	j := NewResolvedRulesJanitor(outbox, resolved, 10, zerolog.Nop())

	n, err := j.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, outbox.processed)
}

func TestJanitor_DeleteFailure_EventNotMarkedProcessed(t *testing.T) {
	outbox := &fakeOutbox{events: []domain.InvalidationEvent{
		{ID: 1, OwnerType: domain.OwnerPlatform, RuleID: uuid.New()},
	}}
	resolved := &failingResolvedRepo{}
	j := NewResolvedRulesJanitor(outbox, resolved, 10, zerolog.Nop())

	n, err := j.RunOnce(context.Background())
	require.NoError(t, err) // batch-level success even if individual event failed
	require.Zero(t, n)
	require.Empty(t, outbox.processed, "failed event must stay unprocessed")
}
