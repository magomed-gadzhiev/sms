package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputePeriodKey_Month(t *testing.T) {
	at := time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC)
	require.Equal(t, "2026-04", ComputePeriodKey(PeriodCalendarMonth, at))
}

func TestComputePeriodKey_Day(t *testing.T) {
	at := time.Date(2026, 4, 18, 14, 30, 0, 0, time.UTC)
	require.Equal(t, "2026-04-18", ComputePeriodKey(PeriodCalendarDay, at))
}

func TestComputePeriodKey_UTC_BoundaryBehaviour(t *testing.T) {
	// 01 марта 02:30 по MSK = 28 февраля 23:30 UTC → period_key='2026-02'
	msk := time.FixedZone("MSK", 3*3600)
	at := time.Date(2026, 3, 1, 2, 30, 0, 0, msk)
	require.Equal(t, "2026-02", ComputePeriodKey(PeriodCalendarMonth, at))
}

func TestComputePeriodKey_Empty_DefaultsToMonth(t *testing.T) {
	at := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	require.Equal(t, "2026-04", ComputePeriodKey("", at))
}

func TestComputePeriodKey_Unknown_IsStable(t *testing.T) {
	at := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	got := ComputePeriodKey(PeriodType("quarterly"), at)
	require.Contains(t, got, "unknown-quarterly")
	require.Contains(t, got, "2026-04")
}
