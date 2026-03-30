package config

import "time"

// Таймауты по умолчанию
const (
	DefaultHealthCheckTimeout      = 5 * time.Second
	DefaultGracefulShutdownTimeout = 5 * time.Second
	DefaultPartitionPurgeInterval  = 60 * time.Second
)

// CORS
const (
	DefaultCORSMaxAge = "3600"
)

// Пагинация
const (
	DefaultPageSize    int32 = 20
	DefaultMaxPageSize int32 = 100
	DefaultMinPage     int32 = 1
)

// Rate limiting
const (
	DefaultRateLimit     = 100
	DefaultBufferSize    = 1000
	DefaultSMSSegmentLen = 160
)
