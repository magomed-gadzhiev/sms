package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidatePositiveAmount (BUG-59): до фикса TopUp принимал "-100"/"abc"/"0"/"Inf"
// и пробрасывал в payment session, затем callback списывал баланс.
func TestValidatePositiveAmount(t *testing.T) {
	cases := []struct {
		name    string
		amount  string
		wantErr bool
	}{
		{"empty", "", true},
		{"non-numeric", "abc", true},
		{"negative", "-100", true},
		{"zero", "0", true},
		{"plus zero", "+0", true},
		{"infinity", "Inf", true},
		{"plus inf", "+Inf", true},
		{"NaN (big.Float SetString rejects)", "NaN", true},
		{"valid integer", "100", false},
		{"valid decimal", "100.50", false},
		{"valid small", "0.01", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePositiveAmount(tc.amount)
			if tc.wantErr {
				assert.NotNil(t, err, "amount %q should be rejected", tc.amount)
			} else {
				assert.Nil(t, err, "amount %q should be accepted", tc.amount)
			}
		})
	}
}

// TestValidateNonNegativeAmount (BUG-60): threshold допускает 0 (выключение
// уведомлений), но не отрицательные / нечисловые.
func TestValidateNonNegativeAmount(t *testing.T) {
	cases := []struct {
		name    string
		amount  string
		wantErr bool
	}{
		{"non-numeric", "abc", true},
		{"negative", "-100", true},
		{"infinity", "Inf", true},
		{"NaN", "NaN", true},
		{"zero", "0", false},
		{"valid integer", "1000", false},
		{"valid decimal", "100.50", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNonNegativeAmount(tc.amount)
			if tc.wantErr {
				assert.NotNil(t, err, "value %q should be rejected", tc.amount)
			} else {
				assert.Nil(t, err, "value %q should be accepted", tc.amount)
			}
		})
	}
}
