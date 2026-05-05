package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisOptionsFromEnv_Defaults(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Empty(t, opts.Password)
	assert.Equal(t, 0, opts.DB)
}

func TestRedisOptionsFromEnv_FromVars(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis.internal:6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	t.Setenv("REDIS_DB", "3")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "redis.internal:6380", opts.Addr)
	assert.Equal(t, "s3cret", opts.Password)
	assert.Equal(t, 3, opts.DB)
}

func TestRedisOptionsFromEnv_InvalidDB_Defaults(t *testing.T) {
	t.Setenv("REDIS_DB", "not-a-number")
	opts := RedisOptionsFromEnv()
	assert.Equal(t, 0, opts.DB)
}
