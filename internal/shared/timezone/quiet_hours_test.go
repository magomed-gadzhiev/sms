// internal/shared/timezone/quiet_hours_test.go
package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQuietHours_OvernightQuiet(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 22, EndHour: 8, Action: "postpone"}

	// 23:00 — should be quiet
	at := time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)

	// 3:00 — should be quiet
	at = time.Date(2026, 3, 29, 3, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)

	// 10:00 — should NOT be quiet
	at = time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}

func TestQuietHours_DaytimeQuiet(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 13, EndHour: 15, Action: "skip"}

	at := time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)
	assert.Equal(t, "skip", result.Action)

	at = time.Date(2026, 3, 28, 16, 0, 0, 0, time.UTC)
	result = CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}

func TestQuietHours_PostponeCalculation(t *testing.T) {
	qh := QuietHoursConfig{Enabled: true, StartHour: 22, EndHour: 8, Action: "postpone"}

	at := time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.True(t, result.IsQuiet)
	assert.Equal(t, "postpone", result.Action)
	// Next end_hour = 08:00 next day
	expected := time.Date(2026, 3, 29, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, result.PostponeTo)
}

func TestQuietHours_Disabled(t *testing.T) {
	qh := QuietHoursConfig{Enabled: false, StartHour: 22, EndHour: 8}
	at := time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	result := CheckQuietHours(at, qh)
	assert.False(t, result.IsQuiet)
}
