package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg, err := Load("")

		require.NoError(t, err)
		assert.NotNil(t, cfg)

		// Проверяем значения по умолчанию
		assert.Equal(t, "smpp-server", cfg.Service.Name)
		assert.Equal(t, "1.0.0", cfg.Service.Version)
		assert.Equal(t, "development", cfg.Service.Env)

		assert.Equal(t, "localhost", cfg.Database.Host)
		assert.Equal(t, 5432, cfg.Database.Port)
		assert.Equal(t, "smpp", cfg.Database.User)
		assert.Equal(t, "smpp_db", cfg.Database.Database)

		assert.Equal(t, "localhost", cfg.Redis.Host)
		assert.Equal(t, 6379, cfg.Redis.Port)

		assert.Equal(t, []string{"localhost:9092"}, cfg.Kafka.Brokers)
		assert.Equal(t, "sms.outgoing", cfg.Kafka.TopicOutgoing)
		assert.Equal(t, "sms.dlr", cfg.Kafka.TopicDLR)
		assert.Equal(t, "sms.failed", cfg.Kafka.TopicFailed)

		assert.Equal(t, "0.0.0.0", cfg.SMSP.Host)
		assert.Equal(t, 2775, cfg.SMSP.Port)

		assert.Equal(t, "0.0.0.0", cfg.API.HTTP.Host)
		assert.Equal(t, 8080, cfg.API.HTTP.Port)

		assert.Equal(t, "0.0.0.0", cfg.API.GRPC.Host)
		assert.Equal(t, 9090, cfg.API.GRPC.Port)
	})

	t.Run("from environment variables", func(t *testing.T) {
		// Устанавливаем переменные окружения
		os.Setenv("SMPP_DATABASE_HOST", "test-host")
		os.Setenv("SMPP_DATABASE_PORT", "5433")
		os.Setenv("SMPP_DATABASE_DATABASE", "test_db")
		defer func() {
			os.Unsetenv("SMPP_DATABASE_HOST")
			os.Unsetenv("SMPP_DATABASE_PORT")
			os.Unsetenv("SMPP_DATABASE_DATABASE")
		}()

		cfg, err := Load("")

		require.NoError(t, err)
		assert.Equal(t, "test-host", cfg.Database.Host)
		assert.Equal(t, 5433, cfg.Database.Port)
		assert.Equal(t, "test_db", cfg.Database.Database)
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "test_db",
			},
			Kafka: KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				TopicOutgoing: "sms.outgoing",
			},
		}

		err := validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("invalid database config", func(t *testing.T) {
		tests := []struct {
			name string
			cfg  *Config
		}{
			{
				name: "empty database host",
				cfg: &Config{
					Database: DatabaseConfig{
						Host:     "",
						Port:     5432,
						Database: "test_db",
					},
					Kafka: KafkaConfig{
						Brokers:       []string{"localhost:9092"},
						TopicOutgoing: "sms.outgoing",
					},
				},
			},
			{
				name: "invalid database port",
				cfg: &Config{
					Database: DatabaseConfig{
						Host:     "localhost",
						Port:     0,
						Database: "test_db",
					},
					Kafka: KafkaConfig{
						Brokers:       []string{"localhost:9092"},
						TopicOutgoing: "sms.outgoing",
					},
				},
			},
			{
				name: "empty database name",
				cfg: &Config{
					Database: DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						Database: "",
					},
					Kafka: KafkaConfig{
						Brokers:       []string{"localhost:9092"},
						TopicOutgoing: "sms.outgoing",
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validate(tt.cfg)
				assert.Error(t, err)
			})
		}
	})

	t.Run("invalid kafka config", func(t *testing.T) {
		tests := []struct {
			name string
			cfg  *Config
		}{
			{
				name: "empty kafka brokers",
				cfg: &Config{
					Database: DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						Database: "test_db",
					},
					Kafka: KafkaConfig{
						Brokers:       []string{},
						TopicOutgoing: "sms.outgoing",
					},
				},
			},
			{
				name: "empty kafka topic",
				cfg: &Config{
					Database: DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						Database: "test_db",
					},
					Kafka: KafkaConfig{
						Brokers:       []string{"localhost:9092"},
						TopicOutgoing: "",
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validate(tt.cfg)
				assert.Error(t, err)
			})
		}
	})
}

func TestGetDSN(t *testing.T) {
	t.Run("generates correct DSN string", func(t *testing.T) {
		cfg := DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			Database: "testdb",
			SSLMode:  "disable",
		}

		dsn := cfg.GetDSN()
		assert.Contains(t, dsn, "host=localhost")
		assert.Contains(t, dsn, "port=5432")
		assert.Contains(t, dsn, "user=testuser")
		assert.Contains(t, dsn, "password=testpass")
		assert.Contains(t, dsn, "dbname=testdb")
		assert.Contains(t, dsn, "sslmode=disable")
	})
}

func TestGetAddr(t *testing.T) {
	t.Run("HTTPConfig", func(t *testing.T) {
		cfg := HTTPConfig{
			Host: "0.0.0.0",
			Port: 8080,
		}

		addr := cfg.GetAddr()
		assert.Equal(t, "0.0.0.0:8080", addr)
	})

	t.Run("GRPCConfig", func(t *testing.T) {
		cfg := GRPCConfig{
			Host: "0.0.0.0",
			Port: 9090,
		}

		addr := cfg.GetAddr()
		assert.Equal(t, "0.0.0.0:9090", addr)
	})

	t.Run("SMSPConfig", func(t *testing.T) {
		cfg := SMSPConfig{
			Host: "0.0.0.0",
			Port: 2775,
		}

		addr := cfg.GetAddr()
		assert.Equal(t, "0.0.0.0:2775", addr)
	})

	t.Run("RedisConfig", func(t *testing.T) {
		cfg := RedisConfig{
			Host: "localhost",
			Port: 6379,
		}

		addr := cfg.GetAddr()
		assert.Equal(t, "localhost:6379", addr)
	})
}

func TestSetDefaults(t *testing.T) {
	t.Run("sets correct default values", func(t *testing.T) {
		v := viper.New()
		setDefaults(v)

		// Проверяем некоторые значения по умолчанию
		assert.Equal(t, "smpp-server", v.GetString("service.name"))
		assert.Equal(t, "1.0.0", v.GetString("service.version"))
		assert.Equal(t, "development", v.GetString("service.env"))
		assert.Equal(t, "localhost", v.GetString("database.host"))
		assert.Equal(t, 5432, v.GetInt("database.port"))
		assert.Equal(t, "localhost:9092", v.GetStringSlice("kafka.brokers")[0])
	})
}
