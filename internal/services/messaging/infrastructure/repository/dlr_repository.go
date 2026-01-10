package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/messaging/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// DLRRepository реализует domain.DLRRepository
type DLRRepository struct {
	db *sqlx.DB
}

// NewDLRRepository создает новый репозиторий DLR receipts
func NewDLRRepository(db *sqlx.DB) *DLRRepository {
	return &DLRRepository{
		db: db,
	}
}

// Create создает новый DLR receipt
func (r *DLRRepository) Create(ctx context.Context, dlr *domain.DLRReceipt) error {
	// Получаем created_at сообщения для правильной ссылки на партицию
	var messageCreatedAt string
	err := r.db.GetContext(ctx, &messageCreatedAt,
		"SELECT created_at FROM messages WHERE id = $1", dlr.MessageID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO dlr_receipts (
			id, message_id, message_created_at, smpp_message_id, provider_id,
			receipted_message_id, submit_date, done_date, stat, err, text,
			source, destination, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		dlr.ID, dlr.MessageID, messageCreatedAt, dlr.SMPPMessageID, dlr.ProviderID,
		dlr.ReceiptedMessageID, dlr.SubmitDate, dlr.DoneDate, dlr.Stat, dlr.Err,
		dlr.Text, dlr.Source, dlr.Destination, dlr.CreatedAt,
	)

	return err
}

// GetByMessageID получает DLR receipts по message_id
func (r *DLRRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) ([]*domain.DLRReceipt, error) {
	query := `
		SELECT * FROM dlr_receipts
		WHERE message_id = $1
		ORDER BY created_at DESC
	`

	var sharedReceipts []shared.DLRReceipt
	err := r.db.SelectContext(ctx, &sharedReceipts, query, messageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []*domain.DLRReceipt{}, nil
		}
		return nil, err
	}

	receipts := make([]*domain.DLRReceipt, len(sharedReceipts))
	for i, sr := range sharedReceipts {
		receipts[i] = &domain.DLRReceipt{
			ID:                 sr.ID,
			MessageID:          sr.MessageID,
			SMPPMessageID:      sr.SMPPMessageID,
			ProviderID:         sr.ProviderID,
			ReceiptedMessageID: sr.ReceiptedMessageID,
			SubmitDate:         sr.SubmitDate,
			DoneDate:           sr.DoneDate,
			Stat:               sr.Stat,
			Err:                sr.Err,
			Text:               sr.Text,
			Source:             sr.Source,
			Destination:        sr.Destination,
			CreatedAt:          sr.CreatedAt,
		}
	}

	return receipts, nil
}

// GetBySMPPMessageID получает DLR receipt по SMPP message_id
func (r *DLRRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*domain.DLRReceipt, error) {
	query := `
		SELECT * FROM dlr_receipts
		WHERE smpp_message_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var sharedReceipt shared.DLRReceipt
	err := r.db.GetContext(ctx, &sharedReceipt, query, smppMessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &domain.DLRReceipt{
		ID:                 sharedReceipt.ID,
		MessageID:          sharedReceipt.MessageID,
		SMPPMessageID:      sharedReceipt.SMPPMessageID,
		ProviderID:         sharedReceipt.ProviderID,
		ReceiptedMessageID: sharedReceipt.ReceiptedMessageID,
		SubmitDate:         sharedReceipt.SubmitDate,
		DoneDate:           sharedReceipt.DoneDate,
		Stat:               sharedReceipt.Stat,
		Err:                sharedReceipt.Err,
		Text:               sharedReceipt.Text,
		Source:             sharedReceipt.Source,
		Destination:        sharedReceipt.Destination,
		CreatedAt:          sharedReceipt.CreatedAt,
	}, nil
}
