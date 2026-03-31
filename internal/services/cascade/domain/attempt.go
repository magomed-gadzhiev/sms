package domain

import (
	"time"

	"github.com/google/uuid"
)

// AttemptStatus представляет статус попытки доставки
type AttemptStatus string

const (
	AttemptPending       AttemptStatus = "pending"
	AttemptSent          AttemptStatus = "sent"
	AttemptDelivered     AttemptStatus = "delivered"
	AttemptFailed        AttemptStatus = "failed"
	AttemptTimeout       AttemptStatus = "timeout"
	AttemptSkipped       AttemptStatus = "skipped"
	AttemptLateDuplicate AttemptStatus = "late_duplicate"
)

// terminalAttemptStatuses содержит финальные статусы попыток
var terminalAttemptStatuses = map[AttemptStatus]bool{
	AttemptDelivered:     true,
	AttemptFailed:        true,
	AttemptTimeout:       true,
	AttemptSkipped:       true,
	AttemptLateDuplicate: true,
}

// DeliveryAttempt представляет попытку доставки через конкретный канал
type DeliveryAttempt struct {
	ID           uuid.UUID
	DeliveryID   uuid.UUID
	ChannelID    uuid.UUID
	ChannelType  string
	StepOrder    int
	Status       AttemptStatus
	ProviderRef  string
	Cost         float64
	Currency     string
	ErrorMessage string
	SentAt       *time.Time
	ResultAt     *time.Time
	CreatedAt    time.Time
}

// NewDeliveryAttempt создает новую попытку доставки
func NewDeliveryAttempt(deliveryID, channelID uuid.UUID, channelType string, stepOrder int, currency string) *DeliveryAttempt {
	return &DeliveryAttempt{
		ID:          uuid.New(),
		DeliveryID:  deliveryID,
		ChannelID:   channelID,
		ChannelType: channelType,
		StepOrder:   stepOrder,
		Status:      AttemptPending,
		Currency:    currency,
		CreatedAt:   time.Now(),
	}
}

// MarkSent переводит попытку в статус sent
func (a *DeliveryAttempt) MarkSent(providerRef string) error {
	if a.Status != AttemptPending {
		return ErrInvalidTransition
	}
	a.Status = AttemptSent
	a.ProviderRef = providerRef
	now := time.Now()
	a.SentAt = &now
	return nil
}

// MarkDelivered переводит попытку в статус delivered
func (a *DeliveryAttempt) MarkDelivered(resultAt time.Time) error {
	if a.Status != AttemptSent {
		return ErrInvalidTransition
	}
	a.Status = AttemptDelivered
	a.ResultAt = &resultAt
	return nil
}

// MarkFailed переводит попытку в статус failed
func (a *DeliveryAttempt) MarkFailed(errMsg string, resultAt time.Time) error {
	if a.Status != AttemptPending && a.Status != AttemptSent {
		return ErrInvalidTransition
	}
	a.Status = AttemptFailed
	a.ErrorMessage = errMsg
	a.ResultAt = &resultAt
	return nil
}

// MarkTimeout переводит попытку в статус timeout
func (a *DeliveryAttempt) MarkTimeout(resultAt time.Time) error {
	if a.Status != AttemptPending && a.Status != AttemptSent {
		return ErrInvalidTransition
	}
	a.Status = AttemptTimeout
	a.ResultAt = &resultAt
	return nil
}

// MarkSkipped переводит попытку в статус skipped
func (a *DeliveryAttempt) MarkSkipped() error {
	if a.Status != AttemptPending {
		return ErrInvalidTransition
	}
	a.Status = AttemptSkipped
	now := time.Now()
	a.ResultAt = &now
	return nil
}

// MarkLateDuplicate помечает попытку как поздний дубликат
func (a *DeliveryAttempt) MarkLateDuplicate() error {
	if !a.IsFinal() {
		return ErrInvalidTransition
	}
	a.Status = AttemptLateDuplicate
	now := time.Now()
	a.ResultAt = &now
	return nil
}

// IsFinal проверяет, является ли статус попытки финальным
func (a *DeliveryAttempt) IsFinal() bool {
	return terminalAttemptStatuses[a.Status]
}
