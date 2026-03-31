package kafka

import (
	"encoding/json"
	"fmt"
)

const defaultSchemaVersion = "1"

// CascadeStartEvent — событие начала каскадной доставки
type CascadeStartEvent struct {
	SchemaVersion string `json:"schema_version"`
	DeliveryID    string `json:"delivery_id"`
	ClientID      string `json:"client_id"`
	StrategyID    string `json:"strategy_id"`
	Recipient     string `json:"recipient"`
	Text          string `json:"text"`
	SenderName    string `json:"sender_name"`
	RequestID     string `json:"request_id"`
}

// NewCascadeStartEvent создаёт CascadeStartEvent с дефолтной версией схемы
func NewCascadeStartEvent(deliveryID, clientID, strategyID, recipient, text, senderName, requestID string) CascadeStartEvent {
	return CascadeStartEvent{
		SchemaVersion: defaultSchemaVersion,
		DeliveryID:    deliveryID,
		ClientID:      clientID,
		StrategyID:    strategyID,
		Recipient:     recipient,
		Text:          text,
		SenderName:    senderName,
		RequestID:     requestID,
	}
}

// Serialize сериализует событие в JSON
func (e *CascadeStartEvent) Serialize() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации CascadeStartEvent: %w", err)
	}
	return data, nil
}

// Deserialize десериализует событие из JSON
func (e *CascadeStartEvent) Deserialize(data []byte) error {
	if err := json.Unmarshal(data, e); err != nil {
		return fmt.Errorf("ошибка десериализации CascadeStartEvent: %w", err)
	}
	return nil
}

// CascadeAttemptSendCommand — команда на отправку попытки доставки через канал
type CascadeAttemptSendCommand struct {
	SchemaVersion string                 `json:"schema_version"`
	AttemptID     string                 `json:"attempt_id"`
	DeliveryID    string                 `json:"delivery_id"`
	ChannelType   string                 `json:"channel_type"`
	ChannelConfig map[string]interface{} `json:"channel_config"`
	Recipient     string                 `json:"recipient"`
	Text          string                 `json:"text"`
	TimeoutS      int                    `json:"timeout_s"`
	RequestID     string                 `json:"request_id"`
}

// NewCascadeAttemptSendCommand создаёт CascadeAttemptSendCommand с дефолтной версией схемы
func NewCascadeAttemptSendCommand(attemptID, deliveryID, channelType string, channelConfig map[string]interface{}, recipient, text string, timeoutS int, requestID string) CascadeAttemptSendCommand {
	return CascadeAttemptSendCommand{
		SchemaVersion: defaultSchemaVersion,
		AttemptID:     attemptID,
		DeliveryID:    deliveryID,
		ChannelType:   channelType,
		ChannelConfig: channelConfig,
		Recipient:     recipient,
		Text:          text,
		TimeoutS:      timeoutS,
		RequestID:     requestID,
	}
}

// Serialize сериализует команду в JSON
func (e *CascadeAttemptSendCommand) Serialize() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации CascadeAttemptSendCommand: %w", err)
	}
	return data, nil
}

// Deserialize десериализует команду из JSON
func (e *CascadeAttemptSendCommand) Deserialize(data []byte) error {
	if err := json.Unmarshal(data, e); err != nil {
		return fmt.Errorf("ошибка десериализации CascadeAttemptSendCommand: %w", err)
	}
	return nil
}

// CascadeAttemptResultEvent — событие результата попытки доставки
type CascadeAttemptResultEvent struct {
	SchemaVersion string `json:"schema_version"`
	AttemptID     string `json:"attempt_id"`
	DeliveryID    string `json:"delivery_id"`
	ChannelType   string `json:"channel_type"`
	Status        string `json:"status"` // delivered | failed | timeout
	ProviderRef   string `json:"provider_ref"`
	ErrorMessage  string `json:"error_message"`
	ResultAt      string `json:"result_at"` // ISO 8601
}

// NewCascadeAttemptResultEvent создаёт CascadeAttemptResultEvent с дефолтной версией схемы
func NewCascadeAttemptResultEvent(attemptID, deliveryID, channelType, status, providerRef, errorMessage, resultAt string) CascadeAttemptResultEvent {
	return CascadeAttemptResultEvent{
		SchemaVersion: defaultSchemaVersion,
		AttemptID:     attemptID,
		DeliveryID:    deliveryID,
		ChannelType:   channelType,
		Status:        status,
		ProviderRef:   providerRef,
		ErrorMessage:  errorMessage,
		ResultAt:      resultAt,
	}
}

// Serialize сериализует событие в JSON
func (e *CascadeAttemptResultEvent) Serialize() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации CascadeAttemptResultEvent: %w", err)
	}
	return data, nil
}

// Deserialize десериализует событие из JSON
func (e *CascadeAttemptResultEvent) Deserialize(data []byte) error {
	if err := json.Unmarshal(data, e); err != nil {
		return fmt.Errorf("ошибка десериализации CascadeAttemptResultEvent: %w", err)
	}
	return nil
}

// CascadeBillingCommand — команда на биллинг попытки доставки
type CascadeBillingCommand struct {
	SchemaVersion  string `json:"schema_version"`
	DeliveryID     string `json:"delivery_id"`
	AttemptID      string `json:"attempt_id"`
	ClientID       string `json:"client_id"`
	ChannelType    string `json:"channel_type"`
	Billable       bool   `json:"billable"`
	IdempotencyKey string `json:"idempotency_key"`
	RequestID      string `json:"request_id"`
}

// NewCascadeBillingCommand создаёт CascadeBillingCommand с дефолтной версией схемы
func NewCascadeBillingCommand(deliveryID, attemptID, clientID, channelType string, billable bool, idempotencyKey, requestID string) CascadeBillingCommand {
	return CascadeBillingCommand{
		SchemaVersion:  defaultSchemaVersion,
		DeliveryID:     deliveryID,
		AttemptID:      attemptID,
		ClientID:       clientID,
		ChannelType:    channelType,
		Billable:       billable,
		IdempotencyKey: idempotencyKey,
		RequestID:      requestID,
	}
}

// Serialize сериализует команду в JSON
func (e *CascadeBillingCommand) Serialize() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации CascadeBillingCommand: %w", err)
	}
	return data, nil
}

// Deserialize десериализует команду из JSON
func (e *CascadeBillingCommand) Deserialize(data []byte) error {
	if err := json.Unmarshal(data, e); err != nil {
		return fmt.Errorf("ошибка десериализации CascadeBillingCommand: %w", err)
	}
	return nil
}
