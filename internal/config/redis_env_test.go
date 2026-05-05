package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisOptionsFromEnv_Defaults(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Empty(t, opts.Password)
	assert.Equal(t, 0, opts.DB)
}

func TestRedisOptionsFromEnv_FromVars(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_ADDR", "redis.internal:6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	t.Setenv("REDIS_DB", "3")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "redis.internal:6380", opts.Addr)
	assert.Equal(t, "s3cret", opts.Password)
	assert.Equal(t, 3, opts.DB)
}

func TestRedisOptionsFromEnv_InvalidDB_Defaults(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_DB", "not-a-number")
	opts := RedisOptionsFromEnv()
	assert.Equal(t, 0, opts.DB)
}

func TestRedisOptionsFromEnv_FromHostPort(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_HOST", "redis-internal")
	t.Setenv("REDIS_PORT", "6390")
	t.Setenv("REDIS_PASSWORD", "p")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "redis-internal:6390", opts.Addr)
	assert.Equal(t, "p", opts.Password)
}

func TestRedisOptionsFromEnv_FromURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://:embedded@redis:6379/2")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "redis:6379", opts.Addr)
	assert.Equal(t, "embedded", opts.Password)
	assert.Equal(t, 2, opts.DB)
}

func TestRedisOptionsFromEnv_URL_PasswordOverride(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://:embedded@redis:6379/0")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "override")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "override", opts.Password)
}

func TestRedisOptionsFromEnv_URLAddrPriority(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://urlhost:6380/1")
	t.Setenv("REDIS_ADDR", "ignored:9999")
	t.Setenv("REDIS_HOST", "ignored-host")
	t.Setenv("REDIS_PORT", "1")
	t.Setenv("REDIS_PASSWORD", "")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "urlhost:6380", opts.Addr)
	assert.Equal(t, 1, opts.DB)
}
