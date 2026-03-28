package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCode_Length(t *testing.T) {
	code := GenerateCode(6)
	assert.Len(t, code, 6)
}

func TestGenerateCode_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code := GenerateCode(8)
		assert.False(t, seen[code], "duplicate code generated: %s", code)
		seen[code] = true
	}
}

func TestGenerateCode_ValidChars(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := GenerateCode(8)
		for _, c := range code {
			assert.True(t,
				(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'),
				"invalid character: %c", c)
		}
	}
}
