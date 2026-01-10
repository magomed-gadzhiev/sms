package shared

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	// RequestIDKey ключ для хранения request ID в контексте
	RequestIDKey contextKey = "request_id"
	// UserIDKey ключ для хранения user ID в контексте
	UserIDKey contextKey = "user_id"
	// ClientIDKey ключ для хранения client ID в контексте
	ClientIDKey contextKey = "client_id"
	// ServiceNameKey ключ для хранения имени сервиса в контексте
	ServiceNameKey contextKey = "service_name"
)

// WithRequestID добавляет request ID в контекст
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID извлекает request ID из контекста
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithUserID добавляет user ID в контекст
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserID извлекает user ID из контекста
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := ctx.Value(UserIDKey).(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}

// WithClientID добавляет client ID в контекст
func WithClientID(ctx context.Context, clientID uuid.UUID) context.Context {
	return context.WithValue(ctx, ClientIDKey, clientID)
}

// GetClientID извлекает client ID из контекста
func GetClientID(ctx context.Context) (uuid.UUID, bool) {
	if id, ok := ctx.Value(ClientIDKey).(uuid.UUID); ok {
		return id, true
	}
	return uuid.Nil, false
}

// WithServiceName добавляет имя сервиса в контекст
func WithServiceName(ctx context.Context, serviceName string) context.Context {
	return context.WithValue(ctx, ServiceNameKey, serviceName)
}

// GetServiceName извлекает имя сервиса из контекста
func GetServiceName(ctx context.Context) string {
	if name, ok := ctx.Value(ServiceNameKey).(string); ok {
		return name
	}
	return ""
}

// NewRequestContext создает новый контекст для запроса с request ID
func NewRequestContext(ctx context.Context) context.Context {
	requestID := uuid.New().String()
	return WithRequestID(ctx, requestID)
}

// WithTimeout создает контекст с таймаутом
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// WithDeadline создает контекст с дедлайном
func WithDeadline(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, deadline)
}

// IsCancelled проверяет, был ли контекст отменен
func IsCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
