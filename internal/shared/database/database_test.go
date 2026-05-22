package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Config tests ---

func TestConfig_DefaultValues(t *testing.T) {
	cfg := Config{}
	assert.Empty(t, cfg.DSN)
	assert.Zero(t, cfg.MaxOpenConns)
	assert.Zero(t, cfg.MaxIdleConns)
	assert.Zero(t, cfg.ConnMaxLifetime)
	assert.Zero(t, cfg.ConnMaxIdleTime)
}

func TestConfig_FieldAssignment(t *testing.T) {
	cfg := Config{
		DSN:             "postgres://user:pass@localhost:5432/testdb",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
	}
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", cfg.DSN)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.ConnMaxLifetime)
	assert.Equal(t, 10*time.Minute, cfg.ConnMaxIdleTime)
}

// --- parsePartitionSuffix tests ---

func TestParsePartitionSuffix_Valid(t *testing.T) {
	tests := []struct {
		name        string
		childName   string
		parentTable string
		wantYear    int
		wantMonth   time.Month
		wantOK      bool
	}{
		{
			name:        "messages partition jan 2025",
			childName:   "messages_2025_01",
			parentTable: "messages",
			wantYear:    2025,
			wantMonth:   time.January,
			wantOK:      true,
		},
		{
			name:        "messages partition dec 2024",
			childName:   "messages_2024_12",
			parentTable: "messages",
			wantYear:    2024,
			wantMonth:   time.December,
			wantOK:      true,
		},
		{
			name:        "audit_log partition mar 2025",
			childName:   "audit_log_2025_03",
			parentTable: "audit_log",
			wantYear:    2025,
			wantMonth:   time.March,
			wantOK:      true,
		},
		{
			name:        "messages partition jun 2026",
			childName:   "messages_2026_06",
			parentTable: "messages",
			wantYear:    2026,
			wantMonth:   time.June,
			wantOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parsePartitionSuffix(tt.childName, tt.parentTable)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantYear, result.Year())
				assert.Equal(t, tt.wantMonth, result.Month())
				assert.Equal(t, 1, result.Day())
			}
		})
	}
}

func TestParsePartitionSuffix_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		childName   string
		parentTable string
	}{
		{
			name:        "wrong prefix",
			childName:   "other_table_2025_01",
			parentTable: "messages",
		},
		{
			name:        "no date suffix",
			childName:   "messages_default",
			parentTable: "messages",
		},
		{
			name:        "invalid month",
			childName:   "messages_2025_13",
			parentTable: "messages",
		},
		{
			name:        "partial date",
			childName:   "messages_2025",
			parentTable: "messages",
		},
		{
			name:        "empty child name",
			childName:   "",
			parentTable: "messages",
		},
		{
			name:        "just prefix no suffix",
			childName:   "messages_",
			parentTable: "messages",
		},
		{
			name:        "date with dashes instead of underscores",
			childName:   "messages_2025-01",
			parentTable: "messages",
		},
		{
			name:        "extra segments in date",
			childName:   "messages_2025_01_15",
			parentTable: "messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parsePartitionSuffix(tt.childName, tt.parentTable)
			assert.False(t, ok)
		})
	}
}

func TestParsePartitionSuffix_ParentWithUnderscore(t *testing.T) {
	// Table name itself has underscores: "audit_log"
	result, ok := parsePartitionSuffix("audit_log_2025_06", "audit_log")
	require.True(t, ok)
	assert.Equal(t, 2025, result.Year())
	assert.Equal(t, time.June, result.Month())
}

func TestNewPartitionPurger(t *testing.T) {
	purger := NewPartitionPurger()
	require.NotNil(t, purger)
}
