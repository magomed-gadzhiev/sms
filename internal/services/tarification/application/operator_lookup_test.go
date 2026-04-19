// operator_lookup_test.go
package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

type stubOpRepo struct {
	calls int64
	metas map[uuid.UUID]domain.OperatorMeta
	err   error
}

func (s *stubOpRepo) GetMetaByID(_ context.Context, id uuid.UUID) (domain.OperatorMeta, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.err != nil {
		return domain.OperatorMeta{}, s.err
	}
	m, ok := s.metas[id]
	if !ok {
		return domain.OperatorMeta{}, errors.New("not found")
	}
	return m, nil
}

func TestCachedOperatorLookup_HitAfterFirstCall(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]domain.OperatorMeta{id: {Code: "mts-ru", Currency: "RUB"}}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 10; i++ {
		meta, err := l.Meta(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "mts-ru", meta.Code)
		require.Equal(t, "RUB", meta.Currency)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_MissForNewID(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]domain.OperatorMeta{
		id1: {Code: "a", Currency: "RUB"},
		id2: {Code: "b", Currency: "KZT"},
	}}
	l := NewCachedOperatorLookup(repo)

	_, _ = l.Meta(context.Background(), id1)
	_, _ = l.Meta(context.Background(), id2)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ErrorNotCached(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{err: errors.New("db down")}
	l := NewCachedOperatorLookup(repo)

	_, err := l.Meta(context.Background(), id)
	require.Error(t, err)

	repo.err = nil
	repo.metas = map[uuid.UUID]domain.OperatorMeta{id: {Code: "x", Currency: "RUB"}}
	meta, err := l.Meta(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "x", meta.Code)
	require.Equal(t, "RUB", meta.Currency)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_EmptyCurrencyCached(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]domain.OperatorMeta{id: {Code: "orphan", Currency: ""}}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 5; i++ {
		meta, err := l.Meta(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "orphan", meta.Code)
		require.Equal(t, "", meta.Currency)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ConcurrentNoRace(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{metas: map[uuid.UUID]domain.OperatorMeta{id: {Code: "mts-ru", Currency: "RUB"}}}
	l := NewCachedOperatorLookup(repo)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.Meta(context.Background(), id)
		}()
	}
	wg.Wait()
}
