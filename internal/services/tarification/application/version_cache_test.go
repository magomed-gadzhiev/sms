// version_cache_test.go
package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type countingVersionRepo struct {
	calls int64
	v     int64
	err   error
	delay time.Duration
}

func (r *countingVersionRepo) GetVersion(ctx context.Context) (int64, error) {
	atomic.AddInt64(&r.calls, 1)
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.v, r.err
}

func TestVersionCache_HitWithinTTL(t *testing.T) {
	inner := &countingVersionRepo{v: 7}
	c := NewVersionCache(inner, 50*time.Millisecond)

	v1, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), v1)

	v2, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), v2)

	require.Equal(t, int64(1), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_ExpiredRefetches(t *testing.T) {
	inner := &countingVersionRepo{v: 7}
	c := NewVersionCache(inner, 10*time.Millisecond)

	_, _ = c.GetVersion(context.Background())
	time.Sleep(20 * time.Millisecond)
	inner.v = 8
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(8), v)
	require.Equal(t, int64(2), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_SingleflightCoalesces(t *testing.T) {
	inner := &countingVersionRepo{v: 9, delay: 20 * time.Millisecond}
	c := NewVersionCache(inner, 100*time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.GetVersion(context.Background())
			require.NoError(t, err)
			require.Equal(t, int64(9), v)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(1), atomic.LoadInt64(&inner.calls))
}

func TestVersionCache_ErrorNotCached(t *testing.T) {
	inner := &countingVersionRepo{err: errors.New("db down")}
	c := NewVersionCache(inner, 50*time.Millisecond)

	_, err := c.GetVersion(context.Background())
	require.Error(t, err)

	inner.err = nil
	inner.v = 3
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), v)
	require.Equal(t, int64(2), atomic.LoadInt64(&inner.calls))
}
