package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// MessageEnrichment contains fields needed to build webhook payload
type MessageEnrichment struct {
	ClientID    *uuid.UUID
	ExternalID  string
	Source      string
	Destination string
	SubmittedAt *time.Time
}

type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) GetEnrichment(ctx context.Context, messageID uuid.UUID) (*MessageEnrichment, error) {
	query := `SELECT client_id, external_id, source, destination, submitted_at
		FROM messages WHERE id = $1 LIMIT 1`

	var clientID *uuid.UUID
	var externalID sql.NullString
	var source, destination string
	var submittedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&clientID, &externalID, &source, &destination, &submittedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // message not found, caller handles this
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message enrichment: %w", err)
	}

	enrichment := &MessageEnrichment{
		ClientID:    clientID,
		Source:      source,
		Destination: destination,
	}
	if externalID.Valid {
		enrichment.ExternalID = externalID.String
	}
	if submittedAt.Valid {
		enrichment.SubmittedAt = &submittedAt.Time
	}
	return enrichment, nil
}
