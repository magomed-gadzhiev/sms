package pipeline

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RoutedMessage — результат работы router stage.
// Расширяет исходное KafkaMessage routing metadata (primary + fallback provider).
// Topic: sms.routed, partition key: provider_id.
type RoutedMessage struct {
	SchemaVersion      int                    `json:"schema_version"`
	MessageID          uuid.UUID              `json:"message_id"`
	TraceID            string                 `json:"trace_id,omitempty"`
	Source             string                 `json:"source"`
	Destination        string                 `json:"destination"`
	Text               string                 `json:"text"`
	ClientID           *uuid.UUID             `json:"client_id,omitempty"`
	OperatorID         *uuid.UUID             `json:"operator_id,omitempty"`
	ProviderID         uuid.UUID              `json:"provider_id"`
	FallbackProviderID *uuid.UUID             `json:"fallback_provider_id,omitempty"`
	RouteID            *uuid.UUID             `json:"route_id,omitempty"`
	Priority           int                    `json:"priority"`
	RetryCount         int                    `json:"retry_count"`
	MaxRetries         int                    `json:"max_retries"`
	RoutedAt           time.Time              `json:"routed_at"`
	CreatedAt          time.Time              `json:"created_at"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

func (m *RoutedMessage) Serialize() ([]byte, error)   { return json.Marshal(m) }
func DeserializeRoutedMessage(data []byte) (*RoutedMessage, error) {
	var msg RoutedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// SentMessage — результат работы sender stage.
// Фиксирует результат SMPP-отправки.
// Topic: sms.sent, partition key: provider_id.
type SentMessage struct {
	SchemaVersion int        `json:"schema_version"`
	MessageID     uuid.UUID  `json:"message_id"`
	TraceID       string     `json:"trace_id,omitempty"`
	ProviderID    uuid.UUID  `json:"provider_id"`
	OperatorID    *uuid.UUID `json:"operator_id,omitempty"`
	RouteID       *uuid.UUID `json:"route_id,omitempty"`
	Channel       string     `json:"channel,omitempty"`
	SMPPMessageID string     `json:"smpp_message_id"`
	Status        string     `json:"status"` // sent, failed, retry
	ErrorCode     *int       `json:"error_code,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	SentAt        time.Time  `json:"sent_at"`
	SegmentsCount int        `json:"segments_count"`
	ConnectionID  string     `json:"connection_id"`
}

func (m *SentMessage) Serialize() ([]byte, error) { return json.Marshal(m) }
func DeserializeSentMessage(data []byte) (*SentMessage, error) {
	var msg SentMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// StatusUpdate — объединённый статус для записи в БД.
// Status writer обрабатывает сообщения из sms.sent и sms.dlr.
// Topic: sms.status, partition key: message_id.
type StatusUpdate struct {
	SchemaVersion int        `json:"schema_version"`
	MessageID     uuid.UUID  `json:"message_id"`
	TraceID       string     `json:"trace_id,omitempty"`
	Status        string     `json:"status"`
	SMPPMessageID string     `json:"smpp_message_id,omitempty"`
	ProviderID    *uuid.UUID `json:"provider_id,omitempty"`
	ErrorCode     *int       `json:"error_code,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	DLRStat       string     `json:"dlr_stat,omitempty"`
	SubmitDate    *time.Time `json:"submit_date,omitempty"`
	DoneDate      *time.Time `json:"done_date,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (m *StatusUpdate) Serialize() ([]byte, error) { return json.Marshal(m) }
func DeserializeStatusUpdate(data []byte) (*StatusUpdate, error) {
	var msg StatusUpdate
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
