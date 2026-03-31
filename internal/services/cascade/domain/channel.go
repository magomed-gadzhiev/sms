package domain

import (
	"context"
	"fmt"
)

// ChannelType представляет тип канала доставки
type ChannelType string

const (
	ChannelSMS         ChannelType = "sms"
	ChannelFlashCall   ChannelType = "flash_call"
	ChannelReverseCall ChannelType = "reverse_call"
	ChannelMessenger      ChannelType = "messenger"
	ChannelMaxMessenger   ChannelType = "max_messenger"
)

// validChannelTypes содержит все допустимые типы каналов
var validChannelTypes = map[ChannelType]bool{
	ChannelSMS:         true,
	ChannelFlashCall:   true,
	ChannelReverseCall: true,
	ChannelMessenger:      true,
	ChannelMaxMessenger:   true,
}

// IsValid проверяет, является ли тип канала допустимым
func (ct ChannelType) IsValid() bool {
	return validChannelTypes[ct]
}

// String возвращает строковое представление типа канала
func (ct ChannelType) String() string {
	return string(ct)
}

// ChannelTypeFromString парсит строку в ChannelType
func ChannelTypeFromString(s string) (ChannelType, error) {
	ct := ChannelType(s)
	if !ct.IsValid() {
		return "", fmt.Errorf("unknown channel type: %s", s)
	}
	return ct, nil
}

// Channel определяет интерфейс канала доставки
type Channel interface {
	Type() ChannelType
	Send(ctx context.Context, attempt *DeliveryAttempt, cfg *ChannelConfig) error
}
