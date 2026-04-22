package handlers

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// resolveAggregatorClientID fetches the client_id linked to an aggregator user.
// Scope.ResellerID holds the aggregator's USER ID; parent_client_id in the
// clients table is a FK to clients.id — not users.id. This helper bridges the
// gap by looking up users.client_id before any scope-filtered query.
//
// Returns the uuid.UUID or an error if the user has no client (non-aggregator
// users may have NULL client_id, which is treated as "no scope" by the caller).
func resolveAggregatorClientID(ctx context.Context, db *storage.DB, userID uuid.UUID) (uuid.UUID, error) {
	var clientID uuid.UUID
	err := db.QueryRowContext(ctx, `SELECT client_id FROM users WHERE id = $1`, userID).Scan(&clientID)
	if err != nil {
		return uuid.Nil, err
	}
	if clientID == uuid.Nil {
		return uuid.Nil, errors.New("user has no client_id; cannot apply aggregator scope")
	}
	return clientID, nil
}
