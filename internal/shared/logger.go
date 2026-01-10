package shared

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger инициализирует глобальный логгер с настройками
func InitLogger(env string) {
	// Настройка формата вывода в зависимости от окружения
	if env == "development" || env == "dev" {
		// В development режиме используем консольный формат с цветами
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		// В production используем JSON формат
		log.Logger = zerolog.New(os.Stderr).With().
			Timestamp().
			Logger()
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Установка временной зоны UTC
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

// Logger возвращает экземпляр логгера с дополнительными полями
func Logger() zerolog.Logger {
	return log.Logger
}

// WithService добавляет поле service к логгеру
func WithService(service string) zerolog.Logger {
	return log.Logger.With().Str("service", service).Logger()
}

// WithError добавляет поле error к логгеру
func WithError(err error) zerolog.Logger {
	return log.Logger.With().Err(err).Logger()
}

// WithFields добавляет произвольные поля к логгеру
func WithFields(fields map[string]interface{}) zerolog.Logger {
	ctx := log.Logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return ctx.Logger()
}
