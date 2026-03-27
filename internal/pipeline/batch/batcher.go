package batch

import (
	"context"
	"sync"
	"time"
)

// BatchAccumulator accumulates messages of generic type T, flushing when
// max_size or max_wait is reached. It is safe for concurrent use.
type BatchAccumulator[T any] struct {
	messages []T
	maxSize  int
	maxWait  time.Duration
	flushCh  chan []T
	mu       sync.Mutex
}

// New creates a BatchAccumulator with the given maxSize and maxWait.
// The flush channel is buffered with size 1.
func New[T any](maxSize int, maxWait time.Duration) *BatchAccumulator[T] {
	return &BatchAccumulator[T]{
		messages: make([]T, 0, maxSize),
		maxSize:  maxSize,
		maxWait:  maxWait,
		flushCh:  make(chan []T, 1),
	}
}

// Add appends a message to the batch. If the batch reaches maxSize,
// it is flushed immediately to the flush channel.
func (b *BatchAccumulator[T]) Add(msg T) {
	b.mu.Lock()
	b.messages = append(b.messages, msg)
	if len(b.messages) >= b.maxSize {
		b.flush()
	}
	b.mu.Unlock()
}

// FlushCh returns a read-only channel that consumers use to receive
// completed batches.
func (b *BatchAccumulator[T]) FlushCh() <-chan []T {
	return b.flushCh
}

// Start launches a background goroutine that periodically flushes pending
// messages when maxWait elapses. When ctx is cancelled, any remaining
// messages are flushed and flushCh is closed.
func (b *BatchAccumulator[T]) Start(ctx context.Context) {
	go func() {
		defer close(b.flushCh)
		timer := time.NewTimer(b.maxWait)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				b.mu.Lock()
				if len(b.messages) > 0 {
					b.flushCh <- b.messages
					b.messages = nil
				}
				b.mu.Unlock()
				return
			case <-timer.C:
				b.mu.Lock()
				if len(b.messages) > 0 {
					b.flushCh <- b.messages
					b.messages = make([]T, 0, b.maxSize)
				}
				b.mu.Unlock()
				timer.Reset(b.maxWait)
			}
		}
	}()
}

// flush moves the accumulated messages into the flush channel and
// resets the internal buffer. Must be called with b.mu held.
func (b *BatchAccumulator[T]) flush() {
	b.flushCh <- b.messages
	b.messages = make([]T, 0, b.maxSize)
}
