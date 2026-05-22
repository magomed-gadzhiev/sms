package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// DB представляет соединение с базой данных
type DB struct {
	*sql.DB
	logger zerolog.Logger
}

// Config представляет конфигурацию базы данных
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// WaitForDatabase ожидает готовности базы данных
func WaitForDatabase(dsn string, maxAttempts int, backoff time.Duration) error {
	logger := log.With().Str("component", "db_wait").Logger()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info().
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Msg("проверка доступности базы данных")

		db, err := sql.Open("pgx", dsn)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = db.PingContext(ctx)
			cancel()
			db.Close()

			if err == nil {
				logger.Info().Msg("база данных доступна")
				return nil
			}
		}

		if attempt < maxAttempts {
			logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("база данных недоступна, повторная попытка")
			time.Sleep(backoff)
		} else {
			return fmt.Errorf("база данных недоступна после %d попыток: %w", maxAttempts, err)
		}
	}

	return fmt.Errorf("база данных недоступна после %d попыток", maxAttempts)
}

// NewDB создает новое соединение с базой данных
func NewDB(dsn string) (*DB, error) {
	return NewDBWithConfig(Config{
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
	})
}

// NewDBWithConfig создает новое соединение с базой данных с настройками пула
func NewDBWithConfig(cfg Config) (*DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка пула подключений
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger := log.With().Str("component", "database").Logger()

	return &DB{
		DB:     db,
		logger: logger,
	}, nil
}

// Close закрывает соединение с базой данных
func (db *DB) Close() error {
	return db.DB.Close()
}

// Ping проверяет соединение с базой данных
func (db *DB) Ping(ctx context.Context) error {
	return db.DB.PingContext(ctx)
}

// Stats возвращает статистику пула соединений
func (db *DB) Stats() sql.DBStats {
	return db.DB.Stats()
}
