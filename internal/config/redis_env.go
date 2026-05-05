package config

import (
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// RedisOptionsFromEnv reads REDIS_URL / REDIS_ADDR / REDIS_HOST+REDIS_PORT and
// REDIS_PASSWORD / REDIS_DB and returns a populated *redis.Options. Resolution
// order for the address: REDIS_URL (full URL form) → REDIS_ADDR (host:port) →
// REDIS_HOST+REDIS_PORT (separate). Defaults to "localhost:6379" if none set.
// Empty/invalid REDIS_DB defaults to 0. REDIS_PASSWORD must be set in any
// environment that exposes Redis to a non-loopback interface (see
// project_redis_hijack_2026_05_04). When REDIS_URL is used, REDIS_PASSWORD
// overrides any password embedded in the URL.
func RedisOptionsFromEnv() *redis.Options {
	password := os.Getenv("REDIS_PASSWORD")

	if rawURL := os.Getenv("REDIS_URL"); rawURL != "" {
		if opts, err := redis.ParseURL(rawURL); err == nil {
			if password != "" {
				opts.Password = password
			}
			return opts
		}
	}

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		host := os.Getenv("REDIS_HOST")
		port := os.Getenv("REDIS_PORT")
		if host != "" || port != "" {
			if host == "" {
				host = "localhost"
			}
			if port == "" {
				port = "6379"
			}
			addr = host + ":" + port
		} else {
			addr = "localhost:6379"
		}
	}

	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			db = parsed
		}
	}
	return &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}
}
