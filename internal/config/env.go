package config

import "os"

// EnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func EnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
