package shared

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *AppError
		wantMsg string
	}{
		{
			name: "error without details",
			err: &AppError{
				Code:    ErrCodeInvalidInput,
				Message: "Invalid input",
			},
			wantMsg: "INVALID_INPUT: Invalid input",
		},
		{
			name: "error with details",
			err: &AppError{
				Code:    ErrCodeInvalidInput,
				Message: "Invalid input",
				Details: "Field 'email' is required",
			},
			wantMsg: "INVALID_INPUT: Invalid input (Field 'email' is required)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	innerErr := &AppError{
		Code:    ErrCodeDatabase,
		Message: "Database error",
	}

	err := &AppError{
		Code:    ErrCodeInternal,
		Message: "Internal error",
		Err:     innerErr,
	}

	assert.Equal(t, innerErr, err.Unwrap())
}

func TestNewAppError(t *testing.T) {
	err := NewAppError(ErrCodeNotFound, "Resource not found", http.StatusNotFound)

	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Equal(t, "Resource not found", err.Message)
	assert.Equal(t, http.StatusNotFound, err.HTTPStatus)
}

func TestAppError_WithDetails(t *testing.T) {
	err := NewAppError(ErrCodeInvalidInput, "Invalid input", http.StatusBadRequest)
	err = err.WithDetails("Field validation failed")

	assert.Equal(t, "Field validation failed", err.Details)
}

func TestAppError_WithError(t *testing.T) {
	innerErr := &AppError{
		Code:    ErrCodeDatabase,
		Message: "Database connection failed",
	}

	err := NewAppError(ErrCodeInternal, "Internal error", http.StatusInternalServerError)
	err = err.WithError(innerErr)

	assert.Equal(t, innerErr, err.Unwrap())
	assert.Contains(t, err.Details, "Database connection failed")
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		errFunc    func(string) *AppError
		wantCode   ErrorCode
		wantStatus int
	}{
		{
			name:       "ErrInternalServer",
			errFunc:    ErrInternalServer,
			wantCode:   ErrCodeInternal,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "ErrInvalidInput",
			errFunc:    ErrInvalidInput,
			wantCode:   ErrCodeInvalidInput,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ErrNotFound",
			errFunc:    func(msg string) *AppError { return ErrNotFound(msg) },
			wantCode:   ErrCodeNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "ErrUnauthorized",
			errFunc:    ErrUnauthorized,
			wantCode:   ErrCodeUnauthorized,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "ErrForbidden",
			errFunc:    ErrForbidden,
			wantCode:   ErrCodeForbidden,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "ErrConflict",
			errFunc:    ErrConflict,
			wantCode:   ErrCodeConflict,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "ErrTooManyRequests",
			errFunc:    ErrTooManyRequests,
			wantCode:   ErrCodeTooManyRequests,
			wantStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errFunc("test message")
			assert.Equal(t, tt.wantCode, err.Code)
			assert.Equal(t, tt.wantStatus, err.HTTPStatus)
		})
	}
}

func TestDatabaseErrors(t *testing.T) {
	t.Run("ErrDatabase", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeDatabase, Message: "Connection failed"}
		err := ErrDatabase("Database error", innerErr)

		assert.Equal(t, ErrCodeDatabase, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
		assert.Equal(t, innerErr, err.Unwrap())
	})

	t.Run("ErrDatabaseConnection", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeDatabase, Message: "Connection failed"}
		err := ErrDatabaseConnection(innerErr)

		assert.Equal(t, ErrCodeDatabaseConn, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})

	t.Run("ErrDatabaseQuery", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeDatabase, Message: "Query failed"}
		err := ErrDatabaseQuery(innerErr)

		assert.Equal(t, ErrCodeDatabaseQuery, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})
}

func TestKafkaErrors(t *testing.T) {
	t.Run("ErrKafkaProducer", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeKafkaProducer, Message: "Producer error"}
		err := ErrKafkaProducer(innerErr)

		assert.Equal(t, ErrCodeKafkaProducer, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})

	t.Run("ErrKafkaConsumer", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeKafkaConsumer, Message: "Consumer error"}
		err := ErrKafkaConsumer(innerErr)

		assert.Equal(t, ErrCodeKafkaConsumer, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})
}

func TestSMPPErrors(t *testing.T) {
	t.Run("ErrSMPPConnection", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeSMPPConnection, Message: "Connection failed"}
		err := ErrSMPPConnection(innerErr)

		assert.Equal(t, ErrCodeSMPPConnection, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})

	t.Run("ErrSMPPBind", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeSMPPBind, Message: "Bind failed"}
		err := ErrSMPPBind(innerErr)

		assert.Equal(t, ErrCodeSMPPBind, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})

	t.Run("ErrSMPPSubmit", func(t *testing.T) {
		innerErr := &AppError{Code: ErrCodeSMPPSubmit, Message: "Submit failed"}
		err := ErrSMPPSubmit(innerErr)

		assert.Equal(t, ErrCodeSMPPSubmit, err.Code)
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus)
	})

	t.Run("ErrSMPPTimeout", func(t *testing.T) {
		err := ErrSMPPTimeout()

		assert.Equal(t, ErrCodeSMPPTimeout, err.Code)
		assert.Equal(t, http.StatusRequestTimeout, err.HTTPStatus)
	})
}

func TestSMSCErrors(t *testing.T) {
	t.Run("ErrSMSCUnavailable", func(t *testing.T) {
		err := ErrSMSCUnavailable("provider1")

		assert.Equal(t, ErrCodeSMSCUnavailable, err.Code)
		assert.Contains(t, err.Message, "provider1")
		assert.Equal(t, http.StatusServiceUnavailable, err.HTTPStatus)
	})

	t.Run("ErrSMSCRejected", func(t *testing.T) {
		err := ErrSMSCRejected("Invalid destination")

		assert.Equal(t, ErrCodeSMSCRejected, err.Code)
		assert.Contains(t, err.Message, "Invalid destination")
		assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
	})

	t.Run("ErrSMSCRateLimit", func(t *testing.T) {
		err := ErrSMSCRateLimit("provider1")

		assert.Equal(t, ErrCodeSMSCRateLimit, err.Code)
		assert.Contains(t, err.Message, "provider1")
		assert.Equal(t, http.StatusTooManyRequests, err.HTTPStatus)
	})
}

func TestRouteErrors(t *testing.T) {
	t.Run("ErrNoRoute", func(t *testing.T) {
		err := ErrNoRoute("79001234567")

		assert.Equal(t, ErrCodeNoRoute, err.Code)
		assert.Contains(t, err.Message, "79001234567")
		assert.Equal(t, http.StatusNotFound, err.HTTPStatus)
	})

	t.Run("ErrRouteNotFound", func(t *testing.T) {
		err := ErrRouteNotFound("route-id-123")

		assert.Equal(t, ErrCodeRouteNotFound, err.Code)
		assert.Contains(t, err.Message, "route-id-123")
		assert.Equal(t, http.StatusNotFound, err.HTTPStatus)
	})
}
