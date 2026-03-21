package monitoring

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	t.Run("registration", func(t *testing.T) {
		// Проверяем, что метрики зарегистрированы
		registry := prometheus.NewRegistry()

		t.Run("HTTP metrics are not nil", func(t *testing.T) {
			assert.NotNil(t, HTTPRequestsTotal)
			assert.NotNil(t, HTTPRequestDuration)
		})

		t.Run("gRPC metrics are not nil", func(t *testing.T) {
			assert.NotNil(t, GRPCRequestsTotal)
			assert.NotNil(t, GRPCRequestDuration)
		})

		t.Run("SMS metrics are not nil", func(t *testing.T) {
			assert.NotNil(t, SMSMessagesReceived)
			assert.NotNil(t, SMSMessagesQueued)
			assert.NotNil(t, SMSMessagesFailed)
		})

		t.Run("metrics can be gathered", func(t *testing.T) {
			_, err := registry.Gather()
			assert.NoError(t, err)
		})
	})
}

func TestHealthChecker(t *testing.T) {
	t.Run("creation", func(t *testing.T) {
		t.Run("initializes with correct fields", func(t *testing.T) {
			checker := NewHealthChecker("test-service", "1.0.0")

			assert.NotNil(t, checker)
			assert.Equal(t, "test-service", checker.serviceName)
			assert.Equal(t, "1.0.0", checker.version)
		})
	})

	t.Run("handlers", func(t *testing.T) {
		t.Run("Handler returns non-nil handler", func(t *testing.T) {
			checker := NewHealthChecker("test-service", "1.0.0")
			handler := checker.Handler()

			assert.NotNil(t, handler)
		})

		t.Run("LivenessHandler returns non-nil handler", func(t *testing.T) {
			checker := NewHealthChecker("test-service", "1.0.0")
			handler := checker.LivenessHandler()

			assert.NotNil(t, handler)
		})

		t.Run("ReadinessHandler returns non-nil handler", func(t *testing.T) {
			checker := NewHealthChecker("test-service", "1.0.0")
			handler := checker.ReadinessHandler()

			assert.NotNil(t, handler)
		})
	})
}
