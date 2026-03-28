// internal/shared/freqcap/checker.go
package freqcap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CapConfig represents a frequency cap configuration.
type CapConfig struct {
	MaxMessages int
	PeriodHours int
	Bypass      bool
}

// Checker checks and records frequency caps using Redis sorted sets.
type Checker struct {
	rdb *redis.Client
}

// NewChecker creates a new frequency cap Checker.
func NewChecker(rdb *redis.Client) *Checker {
	return &Checker{rdb: rdb}
}

// IsCapped checks if the phone has exceeded the cap for the given client.
// Returns true if the phone is capped (should not receive message).
func (c *Checker) IsCapped(ctx context.Context, clientID uuid.UUID, phone string, cap CapConfig) (bool, error) {
	if cap.Bypass || cap.MaxMessages <= 0 {
		return false, nil
	}

	key := fmt.Sprintf("freq:%s:%s", clientID.String(), phone)
	now := time.Now()
	cutoff := now.Add(-time.Duration(cap.PeriodHours) * time.Hour)

	// Remove expired entries
	c.rdb.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff.Unix()))

	// Count remaining entries
	count, err := c.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("freq cap zcard: %w", err)
	}

	return count >= int64(cap.MaxMessages), nil
}

// Record records a sent message for the frequency cap.
func (c *Checker) Record(ctx context.Context, clientID uuid.UUID, phone string, messageID uuid.UUID, ttlHours int) error {
	key := fmt.Sprintf("freq:%s:%s", clientID.String(), phone)
	now := time.Now()

	pipe := c.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.Unix()),
		Member: messageID.String(),
	})
	pipe.Expire(ctx, key, time.Duration(ttlHours)*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}
