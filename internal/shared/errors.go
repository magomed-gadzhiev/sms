package shared

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode представляет код ошибки
type ErrorCode string

const (
	// Общие ошибки
	ErrCodeInternal      ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized  ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden     ErrorCode = "FORBIDDEN"
	ErrCodeConflict      ErrorCode = "CONFLICT"
	ErrCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"

	// Ошибки базы данных
	ErrCodeDatabase      ErrorCode = "DATABASE_ERROR"
	ErrCodeDatabaseConn  ErrorCode = "DATABASE_CONNECTION_ERROR"
	ErrCodeDatabaseQuery ErrorCode = "DATABASE_QUERY_ERROR"

	// Ошибки Kafka
	ErrCodeKafkaProducer ErrorCode = "KAFKA_PRODUCER_ERROR"
	ErrCodeKafkaConsumer ErrorCode = "KAFKA_CONSUMER_ERROR"

	// Ошибки SMPP
	ErrCodeSMPPConnection ErrorCode = "SMPP_CONNECTION_ERROR"
	ErrCodeSMPPBind       ErrorCode = "SMPP_BIND_ERROR"
	ErrCodeSMPPSubmit     ErrorCode = "SMPP_SUBMIT_ERROR"
	ErrCodeSMPPTimeout    ErrorCode = "SMPP_TIMEOUT_ERROR"

	// Ошибки SMSC
	ErrCodeSMSCUnavailable ErrorCode = "SMSC_UNAVAILABLE"
	ErrCodeSMSCRejected    ErrorCode = "SMSC_REJECTED"
	ErrCodeSMSCRateLimit    ErrorCode = "SMSC_RATE_LIMIT"

	// Ошибки маршрутизации
	ErrCodeNoRoute      ErrorCode = "NO_ROUTE"
	ErrCodeRouteNotFound ErrorCode = "ROUTE_NOT_FOUND"
	
	// Ошибки сервисов
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// AppError представляет типизированную ошибку приложения
type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
	Err        error     `json:"-"`
}

// Error реализует интерфейс error
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap возвращает вложенную ошибку
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError создает новую ошибку приложения
func NewAppError(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// WithDetails добавляет детали к ошибке
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithError добавляет вложенную ошибку
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	if e.Details == "" && err != nil {
		e.Details = err.Error()
	}
	return e
}

// Предопределенные ошибки

// ErrInternalServer создает ошибку внутренней ошибки сервера
func ErrInternalServer(message string) *AppError {
	return NewAppError(ErrCodeInternal, message, http.StatusInternalServerError)
}

// ErrInvalidInput создает ошибку невалидного ввода
func ErrInvalidInput(message string) *AppError {
	return NewAppError(ErrCodeInvalidInput, message, http.StatusBadRequest)
}

// ErrNotFound создает ошибку "не найдено".
// Если переданное сообщение уже содержит «не найден/не найдена/не найдено/not found»,
// возвращаем его без модификаций. Иначе добавляем суффикс «не найден» —
// это обратносовместимо с вызовами вида ErrNotFound("шаблон").
func ErrNotFound(resource string) *AppError {
	lower := strings.ToLower(resource)
	if strings.Contains(lower, "не найден") || strings.Contains(lower, "не найдена") ||
		strings.Contains(lower, "не найдено") || strings.Contains(lower, "not found") {
		return NewAppError(ErrCodeNotFound, resource, http.StatusNotFound)
	}
	return NewAppError(ErrCodeNotFound, fmt.Sprintf("%s не найден", resource), http.StatusNotFound)
}

// ErrUnauthorized создает ошибку неавторизованного доступа
func ErrUnauthorized(message string) *AppError {
	if message == "" {
		message = "Требуется аутентификация"
	}
	return NewAppError(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

// ErrForbidden создает ошибку запрещенного доступа
func ErrForbidden(message string) *AppError {
	if message == "" {
		message = "Доступ запрещен"
	}
	return NewAppError(ErrCodeForbidden, message, http.StatusForbidden)
}

// ErrConflict создает ошибку конфликта
func ErrConflict(message string) *AppError {
	return NewAppError(ErrCodeConflict, message, http.StatusConflict)
}

// ErrTooManyRequests создает ошибку превышения лимита запросов
func ErrTooManyRequests(message string) *AppError {
	if message == "" {
		message = "Превышен лимит запросов"
	}
	return NewAppError(ErrCodeTooManyRequests, message, http.StatusTooManyRequests)
}

// ErrDatabase создает ошибку базы данных
func ErrDatabase(message string, err error) *AppError {
	return NewAppError(ErrCodeDatabase, message, http.StatusInternalServerError).WithError(err)
}

// ErrDatabaseConnection создает ошибку подключения к базе данных
func ErrDatabaseConnection(err error) *AppError {
	return NewAppError(ErrCodeDatabaseConn, "Ошибка подключения к базе данных", http.StatusInternalServerError).WithError(err)
}

// ErrDatabaseQuery создает ошибку выполнения запроса к базе данных
func ErrDatabaseQuery(err error) *AppError {
	return NewAppError(ErrCodeDatabaseQuery, "Ошибка выполнения запроса к базе данных", http.StatusInternalServerError).WithError(err)
}

// ErrKafkaProducer создает ошибку Kafka producer
func ErrKafkaProducer(err error) *AppError {
	return NewAppError(ErrCodeKafkaProducer, "Ошибка публикации сообщения в Kafka", http.StatusInternalServerError).WithError(err)
}

// ErrKafkaConsumer создает ошибку Kafka consumer
func ErrKafkaConsumer(err error) *AppError {
	return NewAppError(ErrCodeKafkaConsumer, "Ошибка чтения сообщения из Kafka", http.StatusInternalServerError).WithError(err)
}

// ErrSMPPConnection создает ошибку SMPP соединения
func ErrSMPPConnection(err error) *AppError {
	return NewAppError(ErrCodeSMPPConnection, "Ошибка SMPP соединения", http.StatusInternalServerError).WithError(err)
}

// ErrSMPPBind создает ошибку SMPP bind
func ErrSMPPBind(err error) *AppError {
	return NewAppError(ErrCodeSMPPBind, "Ошибка SMPP bind", http.StatusInternalServerError).WithError(err)
}

// ErrSMPPSubmit создает ошибку SMPP submit
func ErrSMPPSubmit(err error) *AppError {
	return NewAppError(ErrCodeSMPPSubmit, "Ошибка SMPP submit", http.StatusInternalServerError).WithError(err)
}

// ErrSMPPTimeout создает ошибку SMPP таймаута
func ErrSMPPTimeout() *AppError {
	return NewAppError(ErrCodeSMPPTimeout, "SMPP таймаут", http.StatusRequestTimeout)
}

// ErrSMSCUnavailable создает ошибку недоступности SMSC
func ErrSMSCUnavailable(provider string) *AppError {
	return NewAppError(ErrCodeSMSCUnavailable, fmt.Sprintf("SMSC провайдер %s недоступен", provider), http.StatusServiceUnavailable)
}

// ErrSMSCRejected создает ошибку отклонения SMSC
func ErrSMSCRejected(reason string) *AppError {
	return NewAppError(ErrCodeSMSCRejected, fmt.Sprintf("SMSC отклонил сообщение: %s", reason), http.StatusBadRequest)
}

// ErrSMSCRateLimit создает ошибку превышения лимита SMSC
func ErrSMSCRateLimit(provider string) *AppError {
	return NewAppError(ErrCodeSMSCRateLimit, fmt.Sprintf("Превышен лимит для провайдера %s", provider), http.StatusTooManyRequests)
}

// ErrNoRoute создает ошибку отсутствия маршрута
func ErrNoRoute(destination string) *AppError {
	return NewAppError(ErrCodeNoRoute, fmt.Sprintf("Маршрут для %s не найден", destination), http.StatusNotFound)
}

// ErrRouteNotFound создает ошибку отсутствия правила маршрутизации
func ErrRouteNotFound(routeID string) *AppError {
	return NewAppError(ErrCodeRouteNotFound, fmt.Sprintf("Правило маршрутизации %s не найдено", routeID), http.StatusNotFound)
}

// ErrServiceUnavailable создает ошибку недоступности сервиса
func ErrServiceUnavailable(message string) *AppError {
	if message == "" {
		message = "Сервис временно недоступен"
	}
	return NewAppError(ErrCodeServiceUnavailable, message, http.StatusServiceUnavailable)
}
