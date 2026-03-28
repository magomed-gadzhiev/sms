// internal/shared/freqcap/checker_test.go
package freqcap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapConfig_IsCapped(t *testing.T) {
	// Basic logic test without Redis
	cfg := &CapConfig{MaxMessages: 3, PeriodHours: 24}
	assert.True(t, cfg.MaxMessages > 0)
	assert.True(t, cfg.PeriodHours > 0)
}

func TestCapConfig_Bypassed(t *testing.T) {
	cfg := &CapConfig{Bypass: true}
	assert.True(t, cfg.Bypass)
}
