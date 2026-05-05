package handlers

import (
	"context"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// userIDFromCtx — общий хелпер для AuditEvent.UserID. Возвращает nil для
// unauthenticated/system-контекста (uuid.Nil → nil pointer = SQL NULL).
func userIDFromCtx(ctx context.Context) *uuid.UUID {
	id, _ := middleware.GetUserID(ctx)
	if id == uuid.Nil {
		return nil
	}
	return &id
}
