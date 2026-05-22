// Package canary реализует allowlist клиентов на cutover-period (Plan 7 Task 12).
//
// Активируется через CANARY_CLIENT_IDS env-var (запятой-разделённый список UUID).
// Empty/unset = canary off (все клиенты разрешены).
//
// Запрос rejected → caller возвращает 503/ESME_RTHROTTLED. После 24h
// observation operator unset'ит env-var + restart gateways.
package canary

import (
	"os"
	"strings"
)

// EnvVar — название переменной окружения с allowlist'ом.
const EnvVar = "CANARY_CLIENT_IDS"

// IsAllowed проверяет, разрешён ли client_id (UUID-string) в текущем canary-mode.
//
//   - Empty/unset CANARY_CLIENT_IDS → canary off, любой client разрешён → return true.
//   - Non-empty CANARY_CLIENT_IDS → разрешены ТОЛЬКО client'ы в списке (с trim
//     whitespace вокруг каждого UUID).
//
// IsAllowed reads env var ON EACH CALL — ops может toggle без redeploy
// (изменение env-var не подхватится без restart container'а, но семантика
// "при следующем restart" сохранена).
func IsAllowed(clientID string) bool {
	allowlist := strings.TrimSpace(os.Getenv(EnvVar))
	if allowlist == "" {
		return true // canary off
	}
	for _, id := range strings.Split(allowlist, ",") {
		if strings.TrimSpace(id) == clientID {
			return true
		}
	}
	return false
}
