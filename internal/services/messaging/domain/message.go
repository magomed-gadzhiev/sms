package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// Message представляет доменную модель сообщения
type Message struct {
	ID                 uuid.UUID
	MessageID          string
	ExternalID         string
	Source             string
	Destination        string
	Text               string
	Encoding           shared.MessageEncoding
	DataCoding         int
	ESMClass           int
	ProtocolID         int
	PriorityFlag       int
	ReplaceIfPresent   int
	RegisteredDelivery int
	ValidityPeriod     *time.Time
	ServiceType        string
	SourceAddrTON      int
	SourceAddrNPI      int
	DestAddrTON        int
	DestAddrNPI        int
	Status             shared.MessageStatus
	StatusMessage      string
	ProviderID         *uuid.UUID
	RouteID            *uuid.UUID
	ClientID           *uuid.UUID
	RetryCount         int
	MaxRetries         int
	NextRetryAt        *time.Time
	SMPPMessageID      string
	SubmittedAt        *time.Time
	DeliveredAt        *time.Time
	FailedAt           *time.Time
	ScheduledAt        *time.Time
	ExpiredAt          *time.Time
	SegmentCount       int
	TemplateID         *uuid.UUID
	SenderNameID       *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewMessage создает новое сообщение с начальным статусом
func NewMessage(
	clientID uuid.UUID,
	source, destination, text string,
) *Message {
	now := time.Now()

	return &Message{
		ID:                 uuid.New(),
		Source:             source,
		Destination:        destination,
		Text:               text,
		Status:             shared.MessageStatusPending,
		ClientID:           &clientID,
		MaxRetries:         5,
		RegisteredDelivery: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// MarkAsQueued помечает сообщение как добавленное в очередь
func (m *Message) MarkAsQueued() {
	m.Status = shared.MessageStatusQueued
	m.UpdatedAt = time.Now()
}

// MarkAsSent помечает сообщение как отправленное
func (m *Message) MarkAsSent(smppMessageID string) {
	m.Status = shared.MessageStatusSent
	m.SMPPMessageID = smppMessageID
	now := time.Now()
	m.SubmittedAt = &now
	m.UpdatedAt = now
}

// MarkAsDelivered помечает сообщение как доставленное
func (m *Message) MarkAsDelivered() {
	m.Status = shared.MessageStatusDelivered
	now := time.Now()
	m.DeliveredAt = &now
	m.UpdatedAt = now
}

// MarkAsFailed помечает сообщение как неудачное
func (m *Message) MarkAsFailed(reason string) {
	m.Status = shared.MessageStatusFailed
	m.StatusMessage = reason
	now := time.Now()
	m.FailedAt = &now
	m.UpdatedAt = now
}

// MarkAsExpired помечает сообщение как истекшее (DLR timeout)
func (m *Message) MarkAsExpired() {
	m.Status = shared.MessageStatusExpired
	now := time.Now()
	m.ExpiredAt = &now
	m.UpdatedAt = now
}

// MarkAsRejected помечает сообщение как отклоненное
func (m *Message) MarkAsRejected(reason string) {
	m.Status = shared.MessageStatusRejected
	m.StatusMessage = reason
	now := time.Now()
	m.FailedAt = &now
	m.UpdatedAt = now
}

// MarkAsScheduled marks the message as scheduled for future delivery
func (m *Message) MarkAsScheduled(scheduledAt time.Time) {
	m.Status = shared.MessageStatusScheduled
	m.ScheduledAt = &scheduledAt
	m.UpdatedAt = time.Now()
}

// MarkAsCancelled marks a scheduled message as cancelled
func (m *Message) MarkAsCancelled() {
	m.Status = shared.MessageStatusCancelled
	m.UpdatedAt = time.Now()
}

// IncrementRetry увеличивает счетчик попыток
func (m *Message) IncrementRetry(nextRetryAt time.Time) {
	m.RetryCount++
	m.NextRetryAt = &nextRetryAt
	m.UpdatedAt = time.Now()
}

// CanRetry проверяет, можно ли повторить отправку
func (m *Message) CanRetry() bool {
	return m.RetryCount < m.MaxRetries &&
		m.Status == shared.MessageStatusFailed &&
		(m.NextRetryAt == nil || m.NextRetryAt.Before(time.Now()) || m.NextRetryAt.Equal(time.Now()))
}

// ToShared преобразует доменную модель в shared.Message
func (m *Message) ToShared() *shared.Message {
	return &shared.Message{
		ID:                 m.ID,
		MessageID:          shared.NullString(m.MessageID),
		ExternalID:         shared.NullString(m.ExternalID),
		Source:             m.Source,
		Destination:        m.Destination,
		Text:               m.Text,
		Encoding:           m.Encoding,
		DataCoding:         m.DataCoding,
		ESMClass:           m.ESMClass,
		ProtocolID:         m.ProtocolID,
		PriorityFlag:       m.PriorityFlag,
		ReplaceIfPresent:   m.ReplaceIfPresent,
		RegisteredDelivery: m.RegisteredDelivery,
		ValidityPeriod:     m.ValidityPeriod,
		ServiceType:        m.ServiceType,
		SourceAddrTON:      m.SourceAddrTON,
		SourceAddrNPI:      m.SourceAddrNPI,
		DestAddrTON:        m.DestAddrTON,
		DestAddrNPI:        m.DestAddrNPI,
		Status:             m.Status,
		StatusMessage:      shared.NullString(m.StatusMessage),
		ProviderID:         m.ProviderID,
		RouteID:            m.RouteID,
		ClientID:           m.ClientID,
		RetryCount:         m.RetryCount,
		MaxRetries:         m.MaxRetries,
		NextRetryAt:        m.NextRetryAt,
		SMPPMessageID:      shared.NullString(m.SMPPMessageID),
		SubmittedAt:        m.SubmittedAt,
		DeliveredAt:        m.DeliveredAt,
		FailedAt:           m.FailedAt,
		ScheduledAt:        m.ScheduledAt,
		ExpiredAt:          m.ExpiredAt,
		SegmentCount:       m.SegmentCount,
		TemplateID:         m.TemplateID,
		SenderNameID:       m.SenderNameID,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

// FromShared создает доменную модель из shared.Message
func MessageFromShared(msg *shared.Message) *Message {
	return &Message{
		ID:                 msg.ID,
		MessageID:          string(msg.MessageID),
		ExternalID:         string(msg.ExternalID),
		Source:             msg.Source,
		Destination:        msg.Destination,
		Text:               msg.Text,
		Encoding:           msg.Encoding,
		DataCoding:         msg.DataCoding,
		ESMClass:           msg.ESMClass,
		ProtocolID:         msg.ProtocolID,
		PriorityFlag:       msg.PriorityFlag,
		ReplaceIfPresent:   msg.ReplaceIfPresent,
		RegisteredDelivery: msg.RegisteredDelivery,
		ValidityPeriod:     msg.ValidityPeriod,
		ServiceType:        msg.ServiceType,
		SourceAddrTON:      msg.SourceAddrTON,
		SourceAddrNPI:      msg.SourceAddrNPI,
		DestAddrTON:        msg.DestAddrTON,
		DestAddrNPI:        msg.DestAddrNPI,
		Status:             msg.Status,
		StatusMessage:      string(msg.StatusMessage),
		ProviderID:         msg.ProviderID,
		RouteID:            msg.RouteID,
		ClientID:           msg.ClientID,
		RetryCount:         msg.RetryCount,
		MaxRetries:         msg.MaxRetries,
		NextRetryAt:        msg.NextRetryAt,
		SMPPMessageID:      string(msg.SMPPMessageID),
		SubmittedAt:        msg.SubmittedAt,
		DeliveredAt:        msg.DeliveredAt,
		FailedAt:           msg.FailedAt,
		ScheduledAt:        msg.ScheduledAt,
		ExpiredAt:          msg.ExpiredAt,
		SegmentCount:       msg.SegmentCount,
		TemplateID:         msg.TemplateID,
		SenderNameID:       msg.SenderNameID,
		CreatedAt:          msg.CreatedAt,
		UpdatedAt:          msg.UpdatedAt,
	}
}
