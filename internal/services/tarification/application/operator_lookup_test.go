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
)

type stubOpRepo struct {
	calls int64
	codes map[uuid.UUID]string
	err   error
}

func (s *stubOpRepo) GetCodeByID(_ context.Context, id uuid.UUID) (string, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.err != nil {
		return "", s.err
	}
	c, ok := s.codes[id]
	if !ok {
		return "", errors.New("not found")
	}
	return c, nil
}

func TestCachedOperatorLookup_HitAfterFirstCall(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id: "mts-ru"}}
	l := NewCachedOperatorLookup(repo)

	for i := 0; i < 10; i++ {
		code, err := l.Code(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, "mts-ru", code)
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_MissForNewID(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id1: "a", id2: "b"}}
	l := NewCachedOperatorLookup(repo)

	_, _ = l.Code(context.Background(), id1)
	_, _ = l.Code(context.Background(), id2)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ErrorNotCached(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{err: errors.New("db down")}
	l := NewCachedOperatorLookup(repo)

	_, err := l.Code(context.Background(), id)
	require.Error(t, err)

	repo.err = nil
	repo.codes = map[uuid.UUID]string{id: "x"}
	code, err := l.Code(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "x", code)
	require.Equal(t, int64(2), atomic.LoadInt64(&repo.calls))
}

func TestCachedOperatorLookup_ConcurrentNoRace(t *testing.T) {
	id := uuid.New()
	repo := &stubOpRepo{codes: map[uuid.UUID]string{id: "mts-ru"}}
	l := NewCachedOperatorLookup(repo)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = l.Code(context.Background(), id)
		}()
	}
	wg.Wait()
	// не проверяем ровно 1 call: возможны concurrent cold-miss.
	// Но gate: race detector должен пройти (go test -race).
}
