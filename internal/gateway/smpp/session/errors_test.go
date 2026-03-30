package session

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorsAreSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrSessionNotFound", ErrSessionNotFound, "сессия не найдена"},
		{"ErrSessionNotBound", ErrSessionNotBound, "сессия не привязана"},
		{"ErrInvalidBindState", ErrInvalidBindState, "неверное состояние для bind операции"},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, "превышен rate limit"},
		{"ErrConnectionClosed", ErrConnectionClosed, "соединение закрыто"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.err)
			assert.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

func TestErrorsIsComparison(t *testing.T) {
	wrapped := errors.New("outer: " + ErrRateLimitExceeded.Error())
	assert.NotErrorIs(t, wrapped, ErrRateLimitExceeded)

	assert.ErrorIs(t, ErrSessionNotFound, ErrSessionNotFound)
	assert.ErrorIs(t, ErrRateLimitExceeded, ErrRateLimitExceeded)
	assert.ErrorIs(t, ErrConnectionClosed, ErrConnectionClosed)
}
