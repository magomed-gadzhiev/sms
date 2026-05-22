package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ConnectionPoolService управляет пулом SMPP соединений к провайдерам.
// Реализуется через infrastructure/smpp.PoolAdapter; SMPPConnectionPool-stub,
// который раньше жил здесь, удалён вместе с зависимыми типами в spec 017 D2.
type ConnectionPoolService interface {
	Connect(ctx context.Context, provider *domain.Provider) error
	Disconnect(providerID uuid.UUID) error
	GetConnection(providerID uuid.UUID) (Connection, error)
	HealthCheck(providerID uuid.UUID) (int, int, error) // active, total
	CloseAll() error
}

// Connection представляет интерфейс SMPP соединения.
type Connection interface {
	IsBound() bool
	Close() error
}

// SendMessageParams параметры для отправки сообщения через SenderService.
type SendMessageParams struct {
	Source             string
	Destination        string
	Text               string
	ServiceType        string
	SourceAddrTON      int
	SourceAddrNPI      int
	DestAddrTON        int
	DestAddrNPI        int
	ESMClass           int
	ProtocolID         int
	PriorityFlag       int
	RegisteredDelivery int
	ReplaceIfPresent   int
	DataCoding         int
	ValidityPeriod     *time.Time
}
