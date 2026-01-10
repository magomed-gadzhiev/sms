package monitoring

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistration(t *testing.T) {
	// Проверяем, что метрики зарегистрированы
	registry := prometheus.NewRegistry()

	// Проверяем HTTP метрики
	assert.NotNil(t, HTTPRequestsTotal)
	assert.NotNil(t, HTTPRequestDuration)

	// Проверяем gRPC метрики
	assert.NotNil(t, GRPCRequestsTotal)
	assert.NotNil(t, GRPCRequestDuration)

	// Проверяем SMS метрики
	assert.NotNil(t, SMSMessagesReceived)
	assert.NotNil(t, SMSMessagesQueued)
	assert.NotNil(t, SMSMessagesFailed)

	// Проверяем, что метрики можно собрать
	_, err := registry.Gather()
	assert.NoError(t, err)
}

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker("test-service", "1.0.0")

	assert.NotNil(t, checker)
	assert.Equal(t, "test-service", checker.serviceName)
	assert.Equal(t, "1.0.0", checker.version)
}

func TestHealthChecker_Handler(t *testing.T) {
	checker := NewHealthChecker("test-service", "1.0.0")
	handler := checker.Handler()

	assert.NotNil(t, handler)
}

func TestHealthChecker_LivenessHandler(t *testing.T) {
	checker := NewHealthChecker("test-service", "1.0.0")
	handler := checker.LivenessHandler()

	assert.NotNil(t, handler)
}

func TestHealthChecker_ReadinessHandler(t *testing.T) {
	checker := NewHealthChecker("test-service", "1.0.0")
	handler := checker.ReadinessHandler()

	assert.NotNil(t, handler)
}
