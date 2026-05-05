package config

import (
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// RedisOptionsFromEnv reads REDIS_ADDR / REDIS_PASSWORD / REDIS_DB and returns
// a populated *redis.Options. Empty REDIS_ADDR defaults to "localhost:6379".
// Empty/invalid REDIS_DB defaults to 0. REDIS_PASSWORD must be set in any
// environment that exposes Redis to a non-loopback interface (see
// project_redis_hijack_2026_05_04).
func RedisOptionsFromEnv() *redis.Options {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			db = parsed
		}
	}
	return &redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	}
}
