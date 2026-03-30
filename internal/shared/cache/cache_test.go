package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_GetAddr(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "standard redis address",
			host:     "localhost",
			port:     6379,
			expected: "localhost:6379",
		},
		{
			name:     "custom host and port",
			host:     "redis.example.com",
			port:     6380,
			expected: "redis.example.com:6380",
		},
		{
			name:     "ip address",
			host:     "10.0.0.1",
			port:     16379,
			expected: "10.0.0.1:16379",
		},
		{
			name:     "empty host",
			host:     "",
			port:     6379,
			expected: ":6379",
		},
		{
			name:     "zero port",
			host:     "localhost",
			port:     0,
			expected: "localhost:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Host: tt.host,
				Port: tt.port,
			}
			assert.Equal(t, tt.expected, cfg.GetAddr())
		})
	}
}

func TestErrNotFound(t *testing.T) {
	require.NotNil(t, ErrNotFound)
	assert.Equal(t, "key not found", ErrNotFound.Error())
}

func TestErrNotFound_IsDistinctError(t *testing.T) {
	otherErr := fmt.Errorf("some other error")
	assert.NotEqual(t, ErrNotFound, otherErr)
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := &Config{}
	assert.Empty(t, cfg.Host)
	assert.Zero(t, cfg.Port)
	assert.Empty(t, cfg.Password)
	assert.Zero(t, cfg.DB)
	assert.Zero(t, cfg.PoolSize)
	assert.Zero(t, cfg.MinIdleConns)
	assert.Zero(t, cfg.DialTimeout)
	assert.Zero(t, cfg.ReadTimeout)
	assert.Zero(t, cfg.WriteTimeout)
}

func TestConfig_GetAddr_Format(t *testing.T) {
	cfg := &Config{
		Host: "myhost",
		Port: 1234,
	}
	addr := cfg.GetAddr()
	assert.Equal(t, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), addr)
}
