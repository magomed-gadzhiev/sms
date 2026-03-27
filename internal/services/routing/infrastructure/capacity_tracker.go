package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type CapacityTracker struct {
	rdb *redis.Client
}

func NewCapacityTracker(rdb *redis.Client) *CapacityTracker {
	return &CapacityTracker{rdb: rdb}
}

// CheckAndIncrement atomically checks if provider has capacity and increments counters.
// Returns true if capacity is available, false if any limit exceeded.
func (t *CapacityTracker) CheckAndIncrement(ctx context.Context, providerID uuid.UUID, tpsLimit, dailyQuota, monthlyQuota int) (bool, error) {
	pid := providerID.String()

	// Check TPS with sliding window (1 second TTL)
	if tpsLimit > 0 {
		tpsKey := fmt.Sprintf("provider:%s:tps_current", pid)
		val, err := t.rdb.Incr(ctx, tpsKey).Result()
		if err != nil {
			return false, fmt.Errorf("tps incr: %w", err)
		}
		if val == 1 {
			t.rdb.Expire(ctx, tpsKey, time.Second)
		}
		if int(val) > tpsLimit {
			t.rdb.Decr(ctx, tpsKey)
			return false, nil
		}
	}

	// Check daily quota
	if dailyQuota > 0 {
		dailyKey := fmt.Sprintf("provider:%s:daily_count", pid)
		val, err := t.rdb.Incr(ctx, dailyKey).Result()
		if err != nil {
			return false, fmt.Errorf("daily incr: %w", err)
		}
		if val == 1 {
			now := time.Now().UTC()
			midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			t.rdb.ExpireAt(ctx, dailyKey, midnight)
		}
		if int(val) > dailyQuota {
			t.rdb.Decr(ctx, dailyKey)
			if tpsLimit > 0 {
				t.rdb.Decr(ctx, fmt.Sprintf("provider:%s:tps_current", pid))
			}
			return false, nil
		}
	}

	// Check monthly quota
	if monthlyQuota > 0 {
		monthlyKey := fmt.Sprintf("provider:%s:monthly_count", pid)
		val, err := t.rdb.Incr(ctx, monthlyKey).Result()
		if err != nil {
			return false, fmt.Errorf("monthly incr: %w", err)
		}
		if val == 1 {
			now := time.Now().UTC()
			nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			t.rdb.ExpireAt(ctx, monthlyKey, nextMonth)
		}
		if int(val) > monthlyQuota {
			t.rdb.Decr(ctx, monthlyKey)
			if dailyQuota > 0 {
				t.rdb.Decr(ctx, fmt.Sprintf("provider:%s:daily_count", pid))
			}
			if tpsLimit > 0 {
				t.rdb.Decr(ctx, fmt.Sprintf("provider:%s:tps_current", pid))
			}
			return false, nil
		}
	}

	return true, nil
}

// GetCurrentUsage returns current TPS, daily, and monthly usage for a provider.
func (t *CapacityTracker) GetCurrentUsage(ctx context.Context, providerID uuid.UUID) (tps, daily, monthly int64, err error) {
	pid := providerID.String()
	pipe := t.rdb.Pipeline()

	tpsCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:tps_current", pid))
	dailyCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:daily_count", pid))
	monthlyCmd := pipe.Get(ctx, fmt.Sprintf("provider:%s:monthly_count", pid))

	_, _ = pipe.Exec(ctx)

	tps, _ = tpsCmd.Int64()
	daily, _ = dailyCmd.Int64()
	monthly, _ = monthlyCmd.Int64()

	return tps, daily, monthly, nil
}
