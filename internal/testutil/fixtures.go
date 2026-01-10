package testutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NewTestMessage создает тестовое сообщение
func NewTestMessage() *shared.Message {
	now := time.Now()
	clientID := uuid.New()
	return &shared.Message{
		ID:                uuid.New(),
		MessageID:         "test-message-id",
		Source:            "12345",
		Destination:       "79001234567",
		Text:              "Test message",
		Encoding:          shared.MessageEncodingGSM7,
		DataCoding:        0,
		ESMClass:          0,
		ProtocolID:        0,
		PriorityFlag:      0,
		ReplaceIfPresent:  0,
		RegisteredDelivery: 0,
		ServiceType:       "",
		SourceAddrTON:     0,
		SourceAddrNPI:     0,
		DestAddrTON:       0,
		DestAddrNPI:       0,
		Status:            shared.MessageStatusPending,
		StatusMessage:     "",
		ProviderID:        nil,
		RouteID:           nil,
		ClientID:          &clientID,
		RetryCount:        0,
		MaxRetries:        5,
		NextRetryAt:       nil,
		SMPPMessageID:    "",
		SubmittedAt:       nil,
		DeliveredAt:       nil,
		FailedAt:          nil,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// NewTestClient создает тестового клиента
func NewTestClient() *shared.Client {
	return &shared.Client{
		ID:                   uuid.New(),
		Name:                 "Test Client",
		APIKey:               "test-api-key",
		Secret:               "test-secret",
		Active:               true,
		RateLimitPerSecond:  10,
		RateLimitPerMinute:  100,
		RateLimitPerHour:    1000,
		AllowedSourceAddresses: shared.StringArray{"12345", "67890"},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}

// NewTestProvider создает тестового провайдера
func NewTestProvider() *shared.Provider {
	return &shared.Provider{
		ID:               uuid.New(),
		Name:             "Test Provider",
		Host:             "localhost",
		Port:             2775,
		SystemID:         "test-system",
		Password:         "test-password",
		SystemType:       "",
		BindType:         "transceiver",
		BindTON:          0,
		BindNPI:          0,
		AddrTON:          0,
		AddrNPI:          0,
		AddressRange:     "",
		MaxConnections:   10,
		Active:           true,
		Priority:         1,
		ThroughputPerSec: 100,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// NewTestRoute создает тестовый маршрут
func NewTestRoute(providerID uuid.UUID) *shared.Route {
	return &shared.Route{
		ID:                uuid.New(),
		Name:              "Test Route",
		Pattern:           "7900",
		PatternType:       "prefix",
		ProviderID:        providerID,
		Priority:          1,
		Active:            true,
		FailoverProviderID: nil,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// NewTestDLRReceipt создает тестовый DLR receipt
func NewTestDLRReceipt(messageID uuid.UUID) *shared.DLRReceipt {
	now := time.Now()
	return &shared.DLRReceipt{
		ID:                  uuid.New(),
		MessageID:           messageID,
		SMPPMessageID:       "smpp-msg-id-123",
		ProviderID:          nil,
		ReceiptedMessageID:  "receipt-id-123",
		SubmitDate:          &now,
		DoneDate:            &now,
		Stat:                "DELIVRD",
		Err:                 nil,
		Text:                "",
		Source:              "79001234567",
		Destination:         "12345",
		CreatedAt:           now,
	}
}
