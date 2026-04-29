package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateUserEmail (BUG-48): до фикса CreateUser принимал любую строку
// как email, в том числе "not-an-email" → HTTP 201 с битыми данными.
func TestValidateUserEmail(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"empty", "", true},
		{"plain word", "not-an-email", true},
		{"missing @", "userexample.com", true},
		{"missing domain", "user@", true},
		{"with display name (rejected)", "User <user@x.com>", true},
		{"valid simple", "user@example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"valid with subdomain", "u@a.b.c.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUserEmail(tc.email)
			if tc.wantErr {
				assert.NotNil(t, err, "email %q should be rejected", tc.email)
			} else {
				assert.Nil(t, err, "email %q should be accepted", tc.email)
			}
		})
	}
}

// TestValidateUserPassword (BUG-49): до фикса CreateUser принимал пароль
// в 1 символ.
func TestValidateUserPassword(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty", "", true},
		{"1 char", "a", true},
		{"7 chars (boundary -1)", "1234567", true},
		{"8 chars (boundary)", "12345678", false},
		{"long", "P@ssw0rd123!@#", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUserPassword(tc.password)
			if tc.wantErr {
				assert.NotNil(t, err, "password %q should be rejected", tc.password)
			} else {
				assert.Nil(t, err, "password %q should be accepted", tc.password)
			}
		})
	}
}
