package http

import (
	"time"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// SendSMSRequest представляет запрос на отправку SMS
type SendSMSRequest struct {
	Source            string     `json:"source"`
	Destination       string     `json:"destination"`
	Text              string     `json:"text"`
	ExternalID        string     `json:"external_id,omitempty"`
	Priority          int        `json:"priority,omitempty"`
	RegisteredDelivery bool      `json:"registered_delivery,omitempty"`
	ValidityPeriod    *time.Time `json:"validity_period,omitempty"`
	ServiceType       string     `json:"service_type,omitempty"`
	SourceAddrTON     int        `json:"source_addr_ton,omitempty"`
	SourceAddrNPI     int        `json:"source_addr_npi,omitempty"`
	DestAddrTON       int        `json:"dest_addr_ton,omitempty"`
	DestAddrNPI       int        `json:"dest_addr_npi,omitempty"`
	DataCoding        int        `json:"data_coding,omitempty"`
}

// Validate валидирует запрос
func (r *SendSMSRequest) Validate() error {
	if r.Source == "" {
		return shared.ErrInvalidInput("Поле source обязательно")
	}
	if r.Destination == "" {
		return shared.ErrInvalidInput("Поле destination обязательно")
	}
	if r.Text == "" {
		return shared.ErrInvalidInput("Поле text обязательно")
	}
	if len(r.Text) > 1600 {
		return shared.ErrInvalidInput("Текст сообщения слишком длинный (максимум 1600 символов)")
	}
	if r.Priority < 0 || r.Priority > 3 {
		return shared.ErrInvalidInput("Приоритет должен быть в диапазоне 0-3")
	}
	return nil
}

// SendSMSResponse представляет ответ на отправку SMS
type SendSMSResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// SendBatchRequest представляет запрос на пакетную отправку SMS
type SendBatchRequest struct {
	Messages []SendSMSRequest `json:"messages"`
}

// SendBatchResponse представляет ответ на пакетную отправку
type SendBatchResponse struct {
	Results      []SendSMSResponse `json:"results"`
	SuccessCount int               `json:"success_count"`
	FailedCount  int               `json:"failed_count"`
}

// GetStatusResponse представляет ответ со статусом сообщения
type GetStatusResponse struct {
	MessageID     string     `json:"message_id"`
	Status        string     `json:"status"`
	StatusMessage string     `json:"status_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	FailedAt      *time.Time `json:"failed_at,omitempty"`
	SMPPMessageID string     `json:"smpp_message_id,omitempty"`
}
