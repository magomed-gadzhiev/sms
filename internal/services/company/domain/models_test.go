package domain

import "testing"

func TestValidateINN(t *testing.T) {
	tests := []struct {
		inn  string
		want bool
	}{
		{"", true},               // пустой — OK (оферта)
		{"7707083893", true},     // Сбербанк, 10 цифр, валидный
		{"7707083890", false},    // неверная контрольная сумма
		{"12345", false},         // слишком короткий
		{"abcdefghij", false},    // не цифры
		{"7704217370", true},     // валидный 10-значный ИНН (Яндекс)
		{"500100732259", true},   // валидный 12-значный ИНН (ИП)
		{"500100732250", false},  // неверный 12-значный
	}
	for _, tt := range tests {
		got := ValidateINN(tt.inn)
		if got != tt.want {
			t.Errorf("ValidateINN(%q) = %v, want %v", tt.inn, got, tt.want)
		}
	}
}
