package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidSyntax(t *testing.T) {
	v := NewValidator()
	result := v.Validate("Привет, {{ name }}!")
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidator_InvalidSyntax(t *testing.T) {
	v := NewValidator()
	result := v.Validate("{% if true %}unclosed if")
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidator_ExtractVariables(t *testing.T) {
	v := NewValidator()
	vars := v.ExtractVariables("{{ name }} {% if vip %}{{ discount }}{% endif %}")
	assert.Contains(t, vars, "name")
	assert.Contains(t, vars, "vip")
	assert.Contains(t, vars, "discount")
}

func TestValidator_EstimateLength(t *testing.T) {
	v := NewValidator()
	est := v.EstimateLength("Привет, {{ name }}!", map[string][]interface{}{
		"name": {"Иван", "Александр Константинович"},
	})
	require.NotNil(t, est)
	assert.True(t, est.MinLen > 0)
	assert.True(t, est.MaxLen >= est.MinLen)
	assert.True(t, est.Segments >= 1)
}

func TestValidator_LengthWarning(t *testing.T) {
	v := NewValidator()
	// Build a template that produces > 160 chars
	longPrefix := make([]rune, 155)
	for i := range longPrefix {
		longPrefix[i] = 'А'
	}
	est := v.EstimateLength(string(longPrefix)+"{{ name }}", map[string][]interface{}{
		"name": {"Александр Константинович"},
	})
	assert.True(t, est.Segments > 1, "expected >1 segments, got %d (maxLen=%d)", est.Segments, est.MaxLen)
	assert.True(t, est.Warning != "", "expected warning to be non-empty")
}
