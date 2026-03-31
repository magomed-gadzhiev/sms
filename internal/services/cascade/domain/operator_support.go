package domain

import (
	"time"

	"github.com/google/uuid"
)

// OperatorChannelSupport представляет поддержку канала доставки оператором
type OperatorChannelSupport struct {
	ID          uuid.UUID
	OperatorID  uuid.UUID
	ChannelType ChannelType
	Supported   bool
	Notes       string
	UpdatedAt   time.Time
	UpdatedBy   *uuid.UUID
}

// NewOperatorChannelSupport создает новую запись поддержки канала оператором
func NewOperatorChannelSupport(operatorID uuid.UUID, channelType ChannelType, supported bool, notes string, updatedBy *uuid.UUID) *OperatorChannelSupport {
	return &OperatorChannelSupport{
		ID:          uuid.New(),
		OperatorID:  operatorID,
		ChannelType: channelType,
		Supported:   supported,
		Notes:       notes,
		UpdatedAt:   time.Now(),
		UpdatedBy:   updatedBy,
	}
}
