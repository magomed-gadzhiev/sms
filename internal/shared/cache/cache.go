package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Cache представляет Redis кэш
type Cache struct {
	client *redis.Client
	logger zerolog.Logger
}

// Config представляет конфигурацию Redis
type Config struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// GetAddr возвращает адрес Redis
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// WaitForCache ожидает готовности Redis
func WaitForCache(cfg *Config, maxAttempts int, backoff time.Duration) error {
	logger := log.With().Str("component", "cache_wait").Logger()

	client := redis.NewClient(&redis.Options{
		Addr:        cfg.GetAddr(),
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: 5 * time.Second,
	})
	defer client.Close()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info().
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Msg("проверка доступности Redis")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.Ping(ctx).Err()
		cancel()

		if err == nil {
			logger.Info().Msg("Redis доступен")
			return nil
		}

		if attempt < maxAttempts {
			logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Redis недоступен, повторная попытка")
			time.Sleep(backoff)
		} else {
			return fmt.Errorf("Redis недоступен после %d попыток: %w", maxAttempts, err)
		}
	}

	return fmt.Errorf("Redis недоступен после %d попыток", maxAttempts)
}

// NewCache создает новый экземпляр кэша
func NewCache(cfg *Config) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.GetAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Проверка соединения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger := log.With().Str("component", "cache").Logger()

	return &Cache{
		client: client,
		logger: logger,
	}, nil
}

// Close закрывает соединение с Redis
func (c *Cache) Close() error {
	return c.client.Close()
}

// Get получает значение по ключу
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Set устанавливает значение по ключу
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Delete удаляет ключ
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists проверяет существование ключа
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetNX устанавливает значение только если ключ не существует
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiration).Result()
}

// Increment увеличивает значение по ключу
func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// IncrementBy увеличивает значение по ключу на указанную величину
func (c *Cache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

// Expire устанавливает срок действия ключа
func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// Ping проверяет соединение с Redis
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client возвращает прямой доступ к Redis клиенту (для продвинутых операций)
func (c *Cache) Client() *redis.Client {
	return c.client
}

// ErrNotFound возвращается когда ключ не найден
var ErrNotFound = fmt.Errorf("key not found")
