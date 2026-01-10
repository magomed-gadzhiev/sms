package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config представляет общую конфигурацию приложения
type Config struct {
	Service   ServiceConfig   `mapstructure:"service"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	SMSP      SMSPConfig      `mapstructure:"smpp"`
	API       APIConfig       `mapstructure:"api"`
	Worker    WorkerConfig    `mapstructure:"worker"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
}

// ServiceConfig представляет конфигурацию сервиса
type ServiceConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"` // development, staging, production
}

// DatabaseConfig представляет конфигурацию PostgreSQL
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// RedisConfig представляет конфигурацию Redis
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// KafkaConfig представляет конфигурацию Kafka
type KafkaConfig struct {
	Brokers           []string      `mapstructure:"brokers"`
	TopicOutgoing     string        `mapstructure:"topic_outgoing"`
	TopicDLR          string        `mapstructure:"topic_dlr"`
	TopicFailed       string        `mapstructure:"topic_failed"`
	ConsumerGroup     string        `mapstructure:"consumer_group"`
	SessionTimeout    time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	MaxRetries        int           `mapstructure:"max_retries"`
	RetryBackoff      time.Duration `mapstructure:"retry_backoff"`
}

// SMSPConfig представляет конфигурацию SMPP сервера
type SMSPConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	EnquireLinkPeriod time.Duration `mapstructure:"enquire_link_period"`
	MaxConnections    int           `mapstructure:"max_connections"`
	RateLimitPerSec   int           `mapstructure:"rate_limit_per_sec"`
}

// APIConfig представляет конфигурацию API Gateway
type APIConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
	GRPC GRPCConfig `mapstructure:"grpc"`
	Auth AuthConfig `mapstructure:"auth"`
}

// HTTPConfig представляет конфигурацию HTTP сервера
type HTTPConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// GRPCConfig представляет конфигурацию gRPC сервера
type GRPCConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	MaxRecv int    `mapstructure:"max_recv"` // максимальный размер сообщения в байтах
	MaxSend int    `mapstructure:"max_send"` // максимальный размер сообщения в байтах
}

// AuthConfig представляет конфигурацию аутентификации
type AuthConfig struct {
	APIKeyHeader string        `mapstructure:"api_key_header"`
	JWTSecret    string        `mapstructure:"jwt_secret"`
	TokenExpiry  time.Duration `mapstructure:"token_expiry"`
}

// WorkerConfig представляет конфигурацию Worker сервиса
type WorkerConfig struct {
	Concurrency        int           `mapstructure:"concurrency"`         // количество горутин для обработки
	MaxRetries         int           `mapstructure:"max_retries"`        // максимальное количество попыток
	RetryBackoffBase   time.Duration `mapstructure:"retry_backoff_base"` // базовая задержка для retry
	RetryBackoffMax    time.Duration `mapstructure:"retry_backoff_max"`  // максимальная задержка для retry
	BatchSize          int           `mapstructure:"batch_size"`         // размер батча для обработки
	BatchTimeout       time.Duration `mapstructure:"batch_timeout"`     // таймаут для батча
	HealthCheckPeriod  time.Duration `mapstructure:"health_check_period"` // период проверки здоровья соединений
}

// MonitoringConfig представляет конфигурацию мониторинга
type MonitoringConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Prometheus   PrometheusConfig `mapstructure:"prometheus"`
	MetricsPort  int           `mapstructure:"metrics_port"`
}

// PrometheusConfig представляет конфигурацию Prometheus
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// Load загружает конфигурацию из файла и переменных окружения
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Установка значений по умолчанию
	setDefaults(v)

	// Настройка чтения из файла
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/smpp-server")
	}

	// Чтение переменных окружения с префиксом SMPP_
	v.SetEnvPrefix("SMPP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Чтение переменных окружения без префикса (для совместимости с docker-compose)
	// Database
	if host := os.Getenv("POSTGRES_HOST"); host != "" {
		v.Set("database.host", host)
	}
	if port := os.Getenv("POSTGRES_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("database.port", p)
		}
	}
	if user := os.Getenv("POSTGRES_USER"); user != "" {
		v.Set("database.user", user)
	}
	if password := os.Getenv("POSTGRES_PASSWORD"); password != "" {
		v.Set("database.password", password)
	}
	if db := os.Getenv("POSTGRES_DB"); db != "" {
		v.Set("database.database", db)
	}

	// Redis
	if host := os.Getenv("REDIS_HOST"); host != "" {
		v.Set("redis.host", host)
	}
	if port := os.Getenv("REDIS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("redis.port", p)
		}
	}

	// Kafka
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		v.Set("kafka.brokers", strings.Split(brokers, ","))
	}
	if group := os.Getenv("KAFKA_CONSUMER_GROUP"); group != "" {
		v.Set("kafka.consumer_group", group)
	}

	// Service
	if name := os.Getenv("SERVICE_NAME"); name != "" {
		v.Set("service.name", name)
	}

	// HTTP Port
	if port := os.Getenv("HTTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("api.http.port", p)
		}
	}

	// gRPC Port
	if port := os.Getenv("GRPC_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			v.Set("api.grpc.port", p)
		}
	}

	// Чтение конфигурационного файла (если существует)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("ошибка чтения конфигурации: %w", err)
		}
		// Файл не найден - используем только переменные окружения и значения по умолчанию
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации: %w", err)
	}

	// Валидация конфигурации
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("ошибка валидации конфигурации: %w", err)
	}

	return &config, nil
}

// setDefaults устанавливает значения по умолчанию
func setDefaults(v *viper.Viper) {
	// Service
	v.SetDefault("service.name", "smpp-server")
	v.SetDefault("service.version", "1.0.0")
	v.SetDefault("service.env", "development")

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "smpp")
	v.SetDefault("database.password", "")
	v.SetDefault("database.database", "smpp_db")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "10m")

	// Redis
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")

	// Kafka
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.topic_outgoing", "sms.outgoing")
	v.SetDefault("kafka.topic_dlr", "sms.dlr")
	v.SetDefault("kafka.topic_failed", "sms.failed")
	v.SetDefault("kafka.consumer_group", "smpp-worker")
	v.SetDefault("kafka.session_timeout", "30s")
	v.SetDefault("kafka.heartbeat_interval", "10s")
	v.SetDefault("kafka.max_retries", 3)
	v.SetDefault("kafka.retry_backoff", "1s")

	// SMPP
	v.SetDefault("smpp.host", "0.0.0.0")
	v.SetDefault("smpp.port", 2775)
	v.SetDefault("smpp.read_timeout", "30s")
	v.SetDefault("smpp.write_timeout", "30s")
	v.SetDefault("smpp.enquire_link_period", "60s")
	v.SetDefault("smpp.max_connections", 1000)
	v.SetDefault("smpp.rate_limit_per_sec", 100)

	// API HTTP
	v.SetDefault("api.http.host", "0.0.0.0")
	v.SetDefault("api.http.port", 8080)
	v.SetDefault("api.http.read_timeout", "10s")
	v.SetDefault("api.http.write_timeout", "10s")
	v.SetDefault("api.http.idle_timeout", "120s")

	// API gRPC
	v.SetDefault("api.grpc.host", "0.0.0.0")
	v.SetDefault("api.grpc.port", 9090)
	v.SetDefault("api.grpc.max_recv", 4194304) // 4MB
	v.SetDefault("api.grpc.max_send", 4194304) // 4MB

	// Auth
	v.SetDefault("api.auth.api_key_header", "X-API-Key")
	v.SetDefault("api.auth.jwt_secret", "")
	v.SetDefault("api.auth.token_expiry", "24h")

	// Worker
	v.SetDefault("worker.concurrency", 10)
	v.SetDefault("worker.max_retries", 5)
	v.SetDefault("worker.retry_backoff_base", "1s")
	v.SetDefault("worker.retry_backoff_max", "60s")
	v.SetDefault("worker.batch_size", 100)
	v.SetDefault("worker.batch_timeout", "5s")
	v.SetDefault("worker.health_check_period", "30s")

	// Monitoring
	v.SetDefault("monitoring.enabled", true)
	v.SetDefault("monitoring.prometheus.enabled", true)
	v.SetDefault("monitoring.prometheus.path", "/metrics")
	v.SetDefault("monitoring.metrics_port", 2112)
}

// validate валидирует конфигурацию
func validate(cfg *Config) error {
	if cfg.Database.Host == "" {
		return fmt.Errorf("database.host не может быть пустым")
	}
	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return fmt.Errorf("database.port должен быть в диапазоне 1-65535")
	}
	if cfg.Database.Database == "" {
		return fmt.Errorf("database.database не может быть пустым")
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers не может быть пустым")
	}
	if cfg.Kafka.TopicOutgoing == "" {
		return fmt.Errorf("kafka.topic_outgoing не может быть пустым")
	}
	return nil
}

// GetDSN возвращает строку подключения к PostgreSQL
func (c *DatabaseConfig) GetDSN() string {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
	return dsn
}

// GetAddr возвращает адрес для HTTP сервера
func (c *HTTPConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetAddr возвращает адрес для gRPC сервера
func (c *GRPCConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetAddr возвращает адрес для SMPP сервера
func (c *SMSPConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetAddr возвращает адрес для Redis
func (c *RedisConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
