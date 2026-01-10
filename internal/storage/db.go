package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/rs/zerolog/log"
)

// DB представляет соединение с базой данных
type DB struct {
	*sql.DB
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
	return NewDBWithConfig(dsn, 25, 5, 5*time.Minute, 10*time.Minute)
}

// NewDBWithConfig создает новое соединение с базой данных с настройками пула
func NewDBWithConfig(dsn string, maxOpenConns, maxIdleConns int, connMaxLifetime, connMaxIdleTime time.Duration) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка пула подключений
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// Close закрывает соединение с базой данных
func (db *DB) Close() error {
	return db.DB.Close()
}
