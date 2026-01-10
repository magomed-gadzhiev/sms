package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// KafkaMessage представляет сообщение для Kafka
type KafkaMessage struct {
	ID          string                 `json:"id"`
	MessageID   uuid.UUID             `json:"message_id"`
	Source      string                 `json:"source"`
	Destination string                 `json:"destination"`
	Text        string                 `json:"text"`
	ProviderID  *uuid.UUID            `json:"provider_id,omitempty"`
	RouteID     *uuid.UUID             `json:"route_id,omitempty"`
	ClientID    *uuid.UUID             `json:"client_id,omitempty"`
	Priority    int                    `json:"priority"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToMessage преобразует KafkaMessage в shared.Message
func (km *KafkaMessage) ToMessage() *shared.Message {
	msg := &shared.Message{
		ID:          km.MessageID,
		Source:      km.Source,
		Destination: km.Destination,
		Text:        km.Text,
		ProviderID:  km.ProviderID,
		RouteID:     km.RouteID,
		ClientID:    km.ClientID,
		PriorityFlag: km.Priority,
		RetryCount:  km.RetryCount,
		MaxRetries:  km.MaxRetries,
		CreatedAt:   km.CreatedAt,
		Status:      shared.MessageStatusQueued,
	}

	// Установка значений по умолчанию
	if msg.MaxRetries == 0 {
		msg.MaxRetries = 5
	}

	return msg
}

// FromMessage создает KafkaMessage из shared.Message
func FromMessage(msg *shared.Message) *KafkaMessage {
	return &KafkaMessage{
		ID:          msg.ID.String(),
		MessageID:   msg.ID,
		Source:      msg.Source,
		Destination: msg.Destination,
		Text:        msg.Text,
		ProviderID:  msg.ProviderID,
		RouteID:     msg.RouteID,
		ClientID:    msg.ClientID,
		Priority:    msg.PriorityFlag,
		RetryCount:  msg.RetryCount,
		MaxRetries:  msg.MaxRetries,
		CreatedAt:   msg.CreatedAt,
	}
}

// Serialize сериализует сообщение в JSON
func (km *KafkaMessage) Serialize() ([]byte, error) {
	return json.Marshal(km)
}

// Deserialize десериализует сообщение из JSON
func Deserialize(data []byte) (*KafkaMessage, error) {
	var msg KafkaMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DLRMessage представляет delivery receipt сообщение для Kafka
type DLRMessage struct {
	MessageID       uuid.UUID  `json:"message_id"`
	SMPPMessageID   string     `json:"smpp_message_id"`
	ProviderID      *uuid.UUID `json:"provider_id,omitempty"`
	ReceiptedMessageID string  `json:"receipted_message_id,omitempty"`
	SubmitDate      *time.Time `json:"submit_date,omitempty"`
	DoneDate        *time.Time `json:"done_date,omitempty"`
	Stat            string     `json:"stat"`
	Err             *int       `json:"err,omitempty"`
	Text            string     `json:"text,omitempty"`
	Source          string     `json:"source,omitempty"`
	Destination     string     `json:"destination,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Serialize сериализует DLR сообщение в JSON
func (dlr *DLRMessage) Serialize() ([]byte, error) {
	return json.Marshal(dlr)
}

// DeserializeDLR десериализует DLR сообщение из JSON
func DeserializeDLR(data []byte) (*DLRMessage, error) {
	var dlr DLRMessage
	if err := json.Unmarshal(data, &dlr); err != nil {
		return nil, err
	}
	return &dlr, nil
}

// FailedMessage представляет сообщение об ошибке для Kafka
type FailedMessage struct {
	MessageID    uuid.UUID              `json:"message_id"`
	KafkaMessage *KafkaMessage          `json:"kafka_message,omitempty"`
	Error        string                 `json:"error"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	RetryCount   int                    `json:"retry_count"`
	FailedAt     time.Time              `json:"failed_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Serialize сериализует failed сообщение в JSON
func (fm *FailedMessage) Serialize() ([]byte, error) {
	return json.Marshal(fm)
}

// DeserializeFailed десериализует failed сообщение из JSON
func DeserializeFailed(data []byte) (*FailedMessage, error) {
	var fm FailedMessage
	if err := json.Unmarshal(data, &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}
