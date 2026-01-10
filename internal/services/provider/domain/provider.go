package domain

import (
	"time"

	"github.com/google/uuid"
)

// Provider представляет доменную модель SMSC провайдера
type Provider struct {
	ID               uuid.UUID
	Name             string
	Host             string
	Port             int
	SystemID         string
	Password         string
	SystemType       string
	BindType         BindType
	BindTON          int
	BindNPI          int
	AddrTON          int
	AddrNPI          int
	AddressRange     string
	MaxConnections   int
	WindowSize       int
	Active           bool
	Priority         int
	ThroughputPerSec int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// BindType представляет тип SMPP bind операции
type BindType string

const (
	BindTypeTransceiver BindType = "transceiver"
	BindTypeTransmitter BindType = "transmitter"
	BindTypeReceiver    BindType = "receiver"
)

// Validate валидирует провайдера
func (p *Provider) Validate() error {
	if p.Name == "" {
		return ErrProviderNameRequired
	}
	if p.Host == "" {
		return ErrProviderHostRequired
	}
	if p.Port <= 0 || p.Port > 65535 {
		return ErrProviderPortInvalid
	}
	if p.SystemID == "" {
		return ErrProviderSystemIDRequired
	}
	if p.Password == "" {
		return ErrProviderPasswordRequired
	}
	if p.MaxConnections < 0 {
		return ErrProviderMaxConnectionsInvalid
	}
	if !p.IsValidBindType() {
		return ErrProviderBindTypeInvalid
	}
	return nil
}

// IsValidBindType проверяет валидность типа bind
func (p *Provider) IsValidBindType() bool {
	return p.BindType == BindTypeTransceiver ||
		p.BindType == BindTypeTransmitter ||
		p.BindType == BindTypeReceiver
}

// IsActive проверяет, активен ли провайдер
func (p *Provider) IsActive() bool {
	return p.Active
}
