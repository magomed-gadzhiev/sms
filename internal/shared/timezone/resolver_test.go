// internal/shared/timezone/resolver_test.go
package timezone

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_RussianMoscow(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+79161234567") // Moscow (916)
	require.NoError(t, err)
	assert.Equal(t, "Europe/Moscow", loc.String())
}

func TestResolver_USNumber(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+12125551234") // New York (212)
	require.NoError(t, err)
	assert.Contains(t, loc.String(), "America/")
}

func TestResolver_InvalidNumber(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("invalid")
	require.NoError(t, err)
	assert.Equal(t, "UTC", loc.String()) // Fallback
}

func TestResolver_Kazakhstan(t *testing.T) {
	r := NewPhoneTimezoneResolver()
	loc, err := r.GetTimezone("+77011234567")
	require.NoError(t, err)
	assert.NotEmpty(t, loc.String())
}
