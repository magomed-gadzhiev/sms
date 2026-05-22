package batch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testMaxWait = 100 * time.Millisecond
	testDeadline = time.Second
)

// drainWithDeadline reads all batches from ch until it is closed or the
// deadline expires, then returns the collected items.
func drainWithDeadline(t *testing.T, ch <-chan []int, deadline time.Duration) []int {
	t.Helper()
	var result []int
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	for {
		select {
		case batch, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, batch...)
		case <-timer.C:
			t.Fatal("drainWithDeadline: timed out waiting for channel to close")
			return result
		}
	}
}

// TestFlushOnMaxSize verifies that adding maxSize items causes an immediate flush.
func TestFlushOnMaxSize(t *testing.T) {
	t.Parallel()

	const maxSize = 5
	b := New[int](maxSize, 10*time.Second) // long maxWait so timer never fires
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Start(ctx)

	for i := 0; i < maxSize; i++ {
		b.Add(i)
	}

	select {
	case batch := <-b.FlushCh():
		assert.Len(t, batch, maxSize, "batch should contain exactly maxSize items")
		for i := 0; i < maxSize; i++ {
			assert.Equal(t, i, batch[i])
		}
	case <-time.After(testDeadline):
		t.Fatal("timed out waiting for flush on max size")
	}
}

// TestFlushOnMaxWait verifies that a partially-filled batch is flushed after maxWait.
func TestFlushOnMaxWait(t *testing.T) {
	t.Parallel()

	b := New[int](100, testMaxWait)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Start(ctx)

	b.Add(42)

	select {
	case batch := <-b.FlushCh():
		require.Len(t, batch, 1)
		assert.Equal(t, 42, batch[0])
	case <-time.After(testDeadline):
		t.Fatal("timed out waiting for flush on max wait")
	}
}

// TestFlushOnContextCancel verifies that cancelling the context flushes remaining items
// and closes the channel.
func TestFlushOnContextCancel(t *testing.T) {
	t.Parallel()

	const count = 3
	b := New[int](100, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	for i := 0; i < count; i++ {
		b.Add(i)
	}

	cancel()

	var got []int
	timer := time.NewTimer(testDeadline)
	defer timer.Stop()
loop:
	for {
		select {
		case batch, ok := <-b.FlushCh():
			if !ok {
				break loop
			}
			got = append(got, batch...)
		case <-timer.C:
			t.Fatal("timed out waiting for channel to close after context cancel")
		}
	}

	assert.Len(t, got, count, "all added items should have been flushed")
}

// TestEmptyOnCancel verifies that cancelling without adding any items closes the
// channel without sending a batch.
func TestEmptyOnCancel(t *testing.T) {
	t.Parallel()

	b := New[int](10, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	cancel()

	select {
	case batch, ok := <-b.FlushCh():
		if ok {
			t.Fatalf("expected channel to close without data, got batch: %v", batch)
		}
		// closed with no data — correct
	case <-time.After(testDeadline):
		t.Fatal("timed out waiting for channel to close after empty cancel")
	}
}

// TestConcurrentAdd verifies that many goroutines can call Add concurrently
// without races and that all items are eventually flushed.
func TestConcurrentAdd(t *testing.T) {
	t.Parallel()

	const (
		goroutines   = 20
		perGoroutine = 10
		total        = goroutines * perGoroutine
	)

	// Use a small maxSize so flushes happen via Add, and a short maxWait so
	// the timer also fires. A consumer goroutine drains flushCh continuously
	// to prevent the channel from blocking Add (flushCh has buffer 1).
	b := New[int](5, testMaxWait)
	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	var (
		collMu sync.Mutex
		collected []int
	)

	// Consumer: drain flushCh until it's closed.
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for batch := range b.FlushCh() {
			collMu.Lock()
			collected = append(collected, batch...)
			collMu.Unlock()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				b.Add(g*perGoroutine + i)
			}
		}()
	}
	wg.Wait()

	// Cancel to flush any remaining items and close the channel.
	cancel()

	select {
	case <-consumerDone:
	case <-time.After(testDeadline):
		t.Fatal("timed out waiting for consumer to finish after context cancel")
	}

	collMu.Lock()
	got := len(collected)
	collMu.Unlock()

	assert.Equal(t, total, got, "all concurrently added items should be flushed")
}

// TestMultipleBatches verifies that adding 2×maxSize items produces two separate batches.
func TestMultipleBatches(t *testing.T) {
	t.Parallel()

	const maxSize = 4
	b := New[int](maxSize, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Start(ctx)

	// Collect batches concurrently so that flushCh never blocks Add.
	type result struct {
		batch []int
	}
	batchCh := make(chan result, 4)
	go func() {
		for batch := range b.FlushCh() {
			batchCh <- result{batch}
		}
		close(batchCh)
	}()

	for i := 0; i < 2*maxSize; i++ {
		b.Add(i)
	}

	received := 0
	timer := time.NewTimer(testDeadline)
	defer timer.Stop()
	for received < 2 {
		select {
		case r, ok := <-batchCh:
			require.True(t, ok, "batch channel closed prematurely")
			assert.Len(t, r.batch, maxSize, "each batch should have maxSize items")
			received++
		case <-timer.C:
			t.Fatalf("timed out after receiving only %d of 2 expected batches", received)
		}
	}
}
