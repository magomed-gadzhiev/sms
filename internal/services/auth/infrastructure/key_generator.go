package infrastructure

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateAPIKey генерирует новый API ключ
// Формат: sk_live_<random_base64>
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	key := base64.URLEncoding.EncodeToString(bytes)
	return fmt.Sprintf("sk_live_%s", key), nil
}

// GetKeyPrefix возвращает префикс ключа для отображения
func GetKeyPrefix(key string) string {
	if len(key) < 16 {
		return key[:len(key)]
	}
	return key[:16]
}
