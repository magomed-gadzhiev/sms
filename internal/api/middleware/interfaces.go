package middleware

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// ClientRepository интерфейс для работы с клиентами
type ClientRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*shared.Client, error)
}
