package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ChannelConfig представляет конфигурацию канала доставки
type ChannelConfig struct {
	ID          uuid.UUID
	ChannelType ChannelType
	Name        string
	Description string
	Config      map[string]interface{}
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewChannelConfig создает новую конфигурацию канала
func NewChannelConfig(channelType ChannelType, name, description string, config map[string]interface{}) *ChannelConfig {
	now := time.Now()
	return &ChannelConfig{
		ID:          uuid.New(),
		ChannelType: channelType,
		Name:        name,
		Description: description,
		Config:      config,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate проверяет корректность конфигурации канала
func (cc *ChannelConfig) Validate() error {
	if !cc.ChannelType.IsValid() {
		return fmt.Errorf("invalid channel type: %s", cc.ChannelType)
	}
	if cc.Name == "" {
		return fmt.Errorf("channel name is required")
	}
	return nil
}

// Activate активирует канал
func (cc *ChannelConfig) Activate() {
	cc.Active = true
	cc.UpdatedAt = time.Now()
}

// Deactivate деактивирует канал
func (cc *ChannelConfig) Deactivate() {
	cc.Active = false
	cc.UpdatedAt = time.Now()
}
