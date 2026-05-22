package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// DLRReceipt представляет delivery receipt в доменной модели
type DLRReceipt struct {
	ID                  uuid.UUID
	MessageID           uuid.UUID
	SMPPMessageID       string
	ProviderID          *uuid.UUID
	ReceiptedMessageID  string
	SubmitDate          *time.Time
	DoneDate            *time.Time
	Stat                string
	Err                 *int
	Text                string
	Source              string
	Destination         string
	CreatedAt           time.Time
}

// NewDLRReceipt создает новый DLR receipt
func NewDLRReceipt(
	messageID uuid.UUID,
	smppMessageID string,
	stat string,
) *DLRReceipt {
	return &DLRReceipt{
		ID:            uuid.New(),
		MessageID:     messageID,
		SMPPMessageID: smppMessageID,
		Stat:          stat,
		CreatedAt:     time.Now(),
	}
}

// IsDelivered проверяет, доставлено ли сообщение
func (d *DLRReceipt) IsDelivered() bool {
	return d.Stat == "DELIVRD"
}

// IsExpired проверяет, истекло ли сообщение
func (d *DLRReceipt) IsExpired() bool {
	return d.Stat == "EXPIRED"
}

// IsFailed проверяет, не удалось ли доставить сообщение
func (d *DLRReceipt) IsFailed() bool {
	return d.Stat == "REJECTD" || d.Stat == "UNDELIV" || d.Stat == "ACCEPTD" && d.Err != nil && *d.Err != 0
}

// GetMessageStatus определяет статус сообщения на основе DLR
func (d *DLRReceipt) GetMessageStatus() string {
	switch d.Stat {
	case "DELIVRD":
		return "delivered"
	case "EXPIRED":
		return "expired"
	case "REJECTD", "UNDELIV":
		return "failed"
	default:
		// Для других статусов возвращаем текущий статус или pending
		if d.Err != nil && *d.Err != 0 {
			return "failed"
		}
		return "pending"
	}
}

// ToShared преобразует доменную модель в shared.DLRReceipt
func (d *DLRReceipt) ToShared() *shared.DLRReceipt {
	return &shared.DLRReceipt{
		ID:                  d.ID,
		MessageID:           d.MessageID,
		SMPPMessageID:       d.SMPPMessageID,
		ProviderID:          d.ProviderID,
		ReceiptedMessageID:  d.ReceiptedMessageID,
		SubmitDate:          d.SubmitDate,
		DoneDate:            d.DoneDate,
		Stat:                d.Stat,
		Err:                 d.Err,
		Text:                d.Text,
		Source:              d.Source,
		Destination:         d.Destination,
		CreatedAt:           d.CreatedAt,
	}
}
