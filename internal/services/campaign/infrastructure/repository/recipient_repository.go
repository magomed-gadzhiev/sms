package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

// RecipientRepository handles CRUD operations for campaign recipients.
type RecipientRepository struct {
	db *sqlx.DB
}

// NewRecipientRepository creates a new RecipientRepository.
func NewRecipientRepository(db *sqlx.DB) *RecipientRepository {
	return &RecipientRepository{db: db}
}

type recipientRow struct {
	ID          uuid.UUID      `db:"id"`
	CampaignID  uuid.UUID      `db:"campaign_id"`
	ContactID   uuid.UUID      `db:"contact_id"`
	Phone       string         `db:"phone"`
	VariantID   sql.NullString `db:"variant_id"`
	Status      string         `db:"status"`
	MessageID   sql.NullString `db:"message_id"`
	RetryCount  int32          `db:"retry_count"`
	LastRetryAt sql.NullTime   `db:"last_retry_at"`
	CreatedAt   sql.NullTime   `db:"created_at"`
	UpdatedAt   sql.NullTime   `db:"updated_at"`
}

func (r *recipientRow) toDomain() *domain.Recipient {
	rec := &domain.Recipient{
		ID:         r.ID,
		CampaignID: r.CampaignID,
		ContactID:  r.ContactID,
		Phone:      r.Phone,
		Status:     r.Status,
		RetryCount: r.RetryCount,
	}
	if r.VariantID.Valid {
		id, err := uuid.Parse(r.VariantID.String)
		if err == nil {
			rec.VariantID = &id
		}
	}
	if r.MessageID.Valid {
		id, err := uuid.Parse(r.MessageID.String)
		if err == nil {
			rec.MessageID = &id
		}
	}
	if r.LastRetryAt.Valid {
		t := r.LastRetryAt.Time
		rec.LastRetryAt = &t
	}
	if r.CreatedAt.Valid {
		rec.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		rec.UpdatedAt = r.UpdatedAt.Time
	}
	return rec
}

// BulkInsert inserts a batch of recipients using multi-value INSERT.
func (r *RecipientRepository) BulkInsert(ctx context.Context, recipients []domain.Recipient) error {
	if len(recipients) == 0 {
		return nil
	}

	// Build multi-value INSERT in batches of 500
	const batchSize = 500
	for i := 0; i < len(recipients); i += batchSize {
		end := i + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}
		batch := recipients[i:end]

		var sb strings.Builder
		sb.WriteString(`INSERT INTO campaign_recipients (id, campaign_id, contact_id, phone, variant_id, status) VALUES `)

		args := make([]interface{}, 0, len(batch)*6)
		for j, rec := range batch {
			if j > 0 {
				sb.WriteString(", ")
			}
			base := j * 6
			sb.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
				base+1, base+2, base+3, base+4, base+5, base+6))

			var variantID sql.NullString
			if rec.VariantID != nil {
				variantID = sql.NullString{String: rec.VariantID.String(), Valid: true}
			}

			args = append(args, rec.ID, rec.CampaignID, rec.ContactID, rec.Phone, variantID, rec.Status)
		}

		_, err := r.db.ExecContext(ctx, sb.String(), args...)
		if err != nil {
			return fmt.Errorf("failed to bulk insert recipients: %w", err)
		}
	}
	return nil
}

// GetPendingBatch retrieves a batch of pending recipients for sending.
func (r *RecipientRepository) GetPendingBatch(ctx context.Context, campaignID uuid.UUID, limit int) ([]*domain.Recipient, error) {
	query := `SELECT id, campaign_id, contact_id, phone, variant_id, status, message_id,
		retry_count, last_retry_at, created_at, updated_at
		FROM campaign_recipients
		WHERE campaign_id = $1 AND status = 'pending'
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := r.db.QueryxContext(ctx, query, campaignID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending recipients: %w", err)
	}
	defer rows.Close()

	var recipients []*domain.Recipient
	for rows.Next() {
		var row recipientRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan recipient: %w", err)
		}
		recipients = append(recipients, row.toDomain())
	}
	return recipients, nil
}

// UpdateStatus updates a recipient's status.
func (r *RecipientRepository) UpdateStatus(ctx context.Context, recipientID uuid.UUID, status string, messageID *uuid.UUID) error {
	var msgID sql.NullString
	if messageID != nil {
		msgID = sql.NullString{String: messageID.String(), Valid: true}
	}

	query := `UPDATE campaign_recipients SET status = $1, message_id = COALESCE($2, message_id), updated_at = now()
		WHERE id = $3`

	_, err := r.db.ExecContext(ctx, query, status, msgID, recipientID)
	if err != nil {
		return fmt.Errorf("failed to update recipient status: %w", err)
	}
	return nil
}

// UpdateStatusByMessageID updates a recipient's status by message_id (used for delivery status callbacks).
func (r *RecipientRepository) UpdateStatusByMessageID(ctx context.Context, messageID uuid.UUID, status string) error {
	query := `UPDATE campaign_recipients SET status = $1, updated_at = now()
		WHERE message_id = $2`

	_, err := r.db.ExecContext(ctx, query, status, messageID.String())
	if err != nil {
		return fmt.Errorf("failed to update recipient status by message_id: %w", err)
	}
	return nil
}

// CancelPending cancels all pending recipients for a campaign.
func (r *RecipientRepository) CancelPending(ctx context.Context, campaignID uuid.UUID) (int64, error) {
	query := `UPDATE campaign_recipients SET status = 'cancelled', updated_at = now()
		WHERE campaign_id = $1 AND status IN ('pending', 'retry')`

	result, err := r.db.ExecContext(ctx, query, campaignID)
	if err != nil {
		return 0, fmt.Errorf("failed to cancel pending recipients: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// GetFailedForRetry retrieves failed recipients that are eligible for retry.
func (r *RecipientRepository) GetFailedForRetry(ctx context.Context, campaignID uuid.UUID, maxRetries int32) ([]*domain.Recipient, error) {
	query := `SELECT id, campaign_id, contact_id, phone, variant_id, status, message_id,
		retry_count, last_retry_at, created_at, updated_at
		FROM campaign_recipients
		WHERE campaign_id = $1 AND status = 'failed' AND retry_count < $2
		ORDER BY created_at ASC`

	rows, err := r.db.QueryxContext(ctx, query, campaignID, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed recipients for retry: %w", err)
	}
	defer rows.Close()

	var recipients []*domain.Recipient
	for rows.Next() {
		var row recipientRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan recipient: %w", err)
		}
		recipients = append(recipients, row.toDomain())
	}
	return recipients, nil
}

// IncrementRetry increments a recipient's retry count and sets status to retry.
func (r *RecipientRepository) IncrementRetry(ctx context.Context, recipientID uuid.UUID) error {
	now := time.Now()
	query := `UPDATE campaign_recipients SET retry_count = retry_count + 1,
		last_retry_at = $1, status = 'retry', updated_at = now()
		WHERE id = $2`

	_, err := r.db.ExecContext(ctx, query, now, recipientID)
	if err != nil {
		return fmt.Errorf("failed to increment retry: %w", err)
	}
	return nil
}

// CountByStatus returns counts grouped by status for a campaign.
func (r *RecipientRepository) CountByStatus(ctx context.Context, campaignID uuid.UUID) (map[string]int32, error) {
	query := `SELECT status, COUNT(*) as cnt FROM campaign_recipients
		WHERE campaign_id = $1 GROUP BY status`

	rows, err := r.db.QueryxContext(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to count recipients by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int32)
	for rows.Next() {
		var status string
		var cnt int32
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		counts[status] = cnt
	}
	return counts, nil
}

// CountTotal returns the total number of recipients for a campaign.
func (r *RecipientRepository) CountTotal(ctx context.Context, campaignID uuid.UUID) (int32, error) {
	query := `SELECT COUNT(*) FROM campaign_recipients WHERE campaign_id = $1`

	var total int32
	if err := r.db.QueryRowContext(ctx, query, campaignID).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count total recipients: %w", err)
	}
	return total, nil
}

// InsertRetryLog inserts a retry log entry.
func (r *RecipientRepository) InsertRetryLog(ctx context.Context, entry *domain.RetryLogEntry) error {
	query := `INSERT INTO campaign_retry_log (id, campaign_id, recipient_id, retry_number, provider_id, status, error_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var providerID sql.NullString
	if entry.ProviderID != nil {
		providerID = sql.NullString{String: entry.ProviderID.String(), Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		entry.ID, entry.CampaignID, entry.RecipientID, entry.RetryNumber,
		providerID, entry.Status, entry.ErrorCode,
	)
	if err != nil {
		return fmt.Errorf("failed to insert retry log: %w", err)
	}
	return nil
}
