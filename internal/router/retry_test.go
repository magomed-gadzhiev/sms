package router

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func TestRetryManager(t *testing.T) {
	t.Run("ShouldRetry", func(t *testing.T) {
		cfg := &config.WorkerConfig{
			RetryBackoffBase: time.Second,
			RetryBackoffMax:  time.Minute,
		}
		messageRepo := &testutil.MockMessageRepository{}

		rm := NewRetryManager(cfg, messageRepo)

		tests := []struct {
			name      string
			msg       *shared.Message
			wantRetry bool
		}{
			{
				name: "should retry - failed status, retry count less than max",
				msg: &shared.Message{
					Status:     shared.MessageStatusFailed,
					RetryCount: 2,
					MaxRetries: 5,
				},
				wantRetry: true,
			},
			{
				name: "should not retry - retry count equals max",
				msg: &shared.Message{
					Status:     shared.MessageStatusFailed,
					RetryCount: 5,
					MaxRetries: 5,
				},
				wantRetry: false,
			},
			{
				name: "should not retry - not failed status",
				msg: &shared.Message{
					Status:     shared.MessageStatusSent,
					RetryCount: 0,
					MaxRetries: 5,
				},
				wantRetry: false,
			},
			{
				name: "should not retry - retry count exceeds max",
				msg: &shared.Message{
					Status:     shared.MessageStatusFailed,
					RetryCount: 6,
					MaxRetries: 5,
				},
				wantRetry: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := rm.ShouldRetry(tt.msg)
				assert.Equal(t, tt.wantRetry, result)
			})
		}
	})

	t.Run("CalculateNextRetry", func(t *testing.T) {
		cfg := &config.WorkerConfig{
			RetryBackoffBase: time.Second,
			RetryBackoffMax:  time.Minute,
		}
		messageRepo := &testutil.MockMessageRepository{}

		rm := NewRetryManager(cfg, messageRepo)

		tests := []struct {
			name       string
			retryCount int
			checkFunc  func(time.Time)
		}{
			{
				name:       "first retry - 1 second",
				retryCount: 0,
				checkFunc: func(nextRetry time.Time) {
					expected := time.Now().Add(time.Second)
					diff := nextRetry.Sub(expected)
					assert.True(t, diff < time.Second && diff > -time.Second, "next retry should be approximately 1 second from now")
				},
			},
			{
				name:       "second retry - 2 seconds",
				retryCount: 1,
				checkFunc: func(nextRetry time.Time) {
					expected := time.Now().Add(2 * time.Second)
					diff := nextRetry.Sub(expected)
					assert.True(t, diff < time.Second && diff > -time.Second, "next retry should be approximately 2 seconds from now")
				},
			},
			{
				name:       "third retry - 4 seconds",
				retryCount: 2,
				checkFunc: func(nextRetry time.Time) {
					expected := time.Now().Add(4 * time.Second)
					diff := nextRetry.Sub(expected)
					assert.True(t, diff < time.Second && diff > -time.Second, "next retry should be approximately 4 seconds from now")
				},
			},
			{
				name:       "max retry delay capped",
				retryCount: 10,
				checkFunc: func(nextRetry time.Time) {
					expected := time.Now().Add(time.Minute)
					diff := nextRetry.Sub(expected)
					assert.True(t, diff < time.Second && diff > -time.Second, "next retry should be capped at max delay")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				nextRetry := rm.CalculateNextRetry(tt.retryCount)
				tt.checkFunc(nextRetry)
			})
		}
	})

	t.Run("IsPermanentError", func(t *testing.T) {
		cfg := &config.WorkerConfig{
			RetryBackoffBase: time.Second,
			RetryBackoffMax:  time.Minute,
		}
		messageRepo := &testutil.MockMessageRepository{}

		rm := NewRetryManager(cfg, messageRepo)

		tests := []struct {
			name          string
			err           error
			wantPermanent bool
		}{
			{
				name:          "permanent error - ESME_RINVDSTADR",
				err:           &shared.AppError{Message: "ESME_RINVDSTADR: Invalid destination address"},
				wantPermanent: true,
			},
			{
				name:          "permanent error - ESME_RINVSRCADR",
				err:           &shared.AppError{Message: "ESME_RINVSRCADR: Invalid source address"},
				wantPermanent: true,
			},
			{
				name:          "permanent error - ESME_RINVPASWD",
				err:           &shared.AppError{Message: "ESME_RINVPASWD: Invalid password"},
				wantPermanent: true,
			},
			{
				name:          "permanent error - ESME_RINVSYSID",
				err:           &shared.AppError{Message: "ESME_RINVSYSID: Invalid system ID"},
				wantPermanent: true,
			},
			{
				name:          "permanent error - ESME_RALYBND",
				err:           &shared.AppError{Message: "ESME_RALYBND: Already bound"},
				wantPermanent: true,
			},
			{
				name:          "temporary error - connection timeout",
				err:           &shared.AppError{Message: "Connection timeout"},
				wantPermanent: false,
			},
			{
				name:          "nil error",
				err:           nil,
				wantPermanent: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var err error
				if tt.err != nil {
					err = tt.err
				}
				result := rm.IsPermanentError(err)
				assert.Equal(t, tt.wantPermanent, result)
			})
		}
	})

	t.Run("ScheduleRetry", func(t *testing.T) {
		t.Run("schedules retry with correct parameters", func(t *testing.T) {
			messageID := uuid.New()
			msg := testutil.NewTestMessage()
			msg.ID = messageID
			msg.RetryCount = 2

			cfg := &config.WorkerConfig{
				RetryBackoffBase: time.Second,
				RetryBackoffMax:  time.Minute,
			}

			var updatedID uuid.UUID
			var updatedNextRetryAt time.Time

			messageRepo := &testutil.MockMessageRepository{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
					if id == messageID {
						return msg, nil
					}
					return nil, storage.ErrNotFound
				},
				IncrementRetryCountFunc: func(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error {
					updatedID = id
					updatedNextRetryAt = nextRetryAt
					return nil
				},
			}

			rm := NewRetryManager(cfg, messageRepo)

			err := rm.ScheduleRetry(context.Background(), messageID, 2)

			require.NoError(t, err)
			assert.Equal(t, messageID, updatedID)
			assert.True(t, updatedNextRetryAt.After(time.Now()))
		})
	})

	t.Run("MarkAsFailed", func(t *testing.T) {
		t.Run("marks message as failed with error message", func(t *testing.T) {
			messageID := uuid.New()
			msg := testutil.NewTestMessage()
			msg.ID = messageID

			cfg := &config.WorkerConfig{
				RetryBackoffBase: time.Second,
				RetryBackoffMax:  time.Minute,
			}

			var updatedMsg *shared.Message

			messageRepo := &testutil.MockMessageRepository{
				GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
					if id == messageID {
						return msg, nil
					}
					return nil, storage.ErrNotFound
				},
				UpdateFunc: func(ctx context.Context, m *shared.Message) error {
					updatedMsg = m
					return nil
				},
			}

			rm := NewRetryManager(cfg, messageRepo)

			err := rm.MarkAsFailed(context.Background(), messageID, "test error message")

			require.NoError(t, err)
			assert.NotNil(t, updatedMsg)
			assert.Equal(t, shared.MessageStatusFailed, updatedMsg.Status)
			assert.Equal(t, shared.NullString("test error message"), updatedMsg.StatusMessage)
			assert.NotNil(t, updatedMsg.FailedAt)
		})
	})
}
