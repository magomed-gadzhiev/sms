package main

import "testing"

func TestValidateAdminPassword(t *testing.T) {
	tests := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "Short1!", true},
		{"exactly 15", "123456789012345", true},
		{"exactly 16", "1234567890123456", false},
		{"long random", "aB3$xYz9!qWe7&rT", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAdminPassword(tc.pw)
			if tc.wantErr && err == nil {
				t.Errorf("validateAdminPassword(%q) = nil, want error", tc.pw)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateAdminPassword(%q) = %v, want nil", tc.pw, err)
			}
		})
	}
}
