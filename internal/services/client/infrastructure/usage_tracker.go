package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type UsageTracker struct {
	redis *redis.Client
}

func NewUsageTracker(redisClient *redis.Client) *UsageTracker {
	return &UsageTracker{redis: redisClient}
}

// IncrementSMSCount increments the monthly SMS counter for a client.
// Returns the new total count.
func (t *UsageTracker) IncrementSMSCount(ctx context.Context, clientID uuid.UUID, count int) (int64, error) {
	key := t.monthlyKey(clientID)
	pipe := t.redis.Pipeline()
	incrCmd := pipe.IncrBy(ctx, key, int64(count))
	pipe.ExpireNX(ctx, key, t.timeUntilEndOfMonth()+24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("increment sms count: %w", err)
	}
	return incrCmd.Val(), nil
}

// GetMonthlySMSCount returns the current monthly SMS count for a client.
func (t *UsageTracker) GetMonthlySMSCount(ctx context.Context, clientID uuid.UUID) (int, error) {
	key := t.monthlyKey(clientID)
	val, err := t.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get monthly sms count: %w", err)
	}
	count, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse sms count: %w", err)
	}
	return count, nil
}

func (t *UsageTracker) monthlyKey(clientID uuid.UUID) string {
	now := time.Now()
	return fmt.Sprintf("usage:%s:sms:%d-%02d", clientID.String(), now.Year(), now.Month())
}

func (t *UsageTracker) timeUntilEndOfMonth() time.Duration {
	now := time.Now()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	return endOfMonth.Sub(now)
}
