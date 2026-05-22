package domain

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryStatus представляет статус доставки
type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryInProgress DeliveryStatus = "in_progress"
	DeliveryDelivered  DeliveryStatus = "delivered"
	DeliveryFailed     DeliveryStatus = "failed"
	DeliveryCancelled  DeliveryStatus = "cancelled"
)

// IsTerminal проверяет, является ли статус финальным
func (ds DeliveryStatus) IsTerminal() bool {
	return ds == DeliveryDelivered || ds == DeliveryFailed || ds == DeliveryCancelled
}

// Delivery представляет доставку сообщения через каскад каналов
type Delivery struct {
	ID           uuid.UUID
	ClientID     uuid.UUID
	MessageID    *uuid.UUID
	StrategyID   uuid.UUID
	Recipient    string
	Text         string
	SenderName   string
	Status       DeliveryStatus
	CurrentStep  int
	DeliveredVia string
	TotalCost    float64
	Currency     string
	RequestID    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Attempts     []DeliveryAttempt
}

// NewDelivery создает новую доставку
func NewDelivery(clientID, strategyID uuid.UUID, recipient, text, senderName, currency, requestID string, messageID *uuid.UUID) *Delivery {
	now := time.Now()
	return &Delivery{
		ID:          uuid.New(),
		ClientID:    clientID,
		MessageID:   messageID,
		StrategyID:  strategyID,
		Recipient:   recipient,
		Text:        text,
		SenderName:  senderName,
		Status:      DeliveryPending,
		CurrentStep: 0,
		TotalCost:   0,
		Currency:    currency,
		RequestID:   requestID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Attempts:    make([]DeliveryAttempt, 0),
	}
}

// Start переводит доставку в статус in_progress
func (d *Delivery) Start() error {
	if d.Status != DeliveryPending {
		return ErrInvalidTransition
	}
	d.Status = DeliveryInProgress
	d.UpdatedAt = time.Now()
	return nil
}

// MarkDelivered переводит доставку в статус delivered
func (d *Delivery) MarkDelivered(channelType string) error {
	if d.Status != DeliveryInProgress {
		return ErrInvalidTransition
	}
	d.Status = DeliveryDelivered
	d.DeliveredVia = channelType
	d.UpdatedAt = time.Now()
	return nil
}

// MarkFailed переводит доставку в статус failed
func (d *Delivery) MarkFailed() error {
	if d.Status != DeliveryInProgress {
		return ErrInvalidTransition
	}
	d.Status = DeliveryFailed
	d.UpdatedAt = time.Now()
	return nil
}

// MarkCancelled переводит доставку в статус cancelled
func (d *Delivery) MarkCancelled() error {
	if d.Status != DeliveryInProgress {
		return ErrInvalidTransition
	}
	d.Status = DeliveryCancelled
	d.UpdatedAt = time.Now()
	return nil
}

// AdvanceStep увеличивает текущий шаг
func (d *Delivery) AdvanceStep() {
	d.CurrentStep++
	d.UpdatedAt = time.Now()
}

// AddCost добавляет стоимость к общей сумме
func (d *Delivery) AddCost(amount float64) {
	d.TotalCost += amount
	d.UpdatedAt = time.Now()
}
