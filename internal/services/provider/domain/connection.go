package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Connection представляет SMPP соединение к провайдеру
type Connection struct {
	ID          string
	ProviderID  uuid.UUID
	Bound       bool
	LastUsed    time.Time
	SequenceNum uint32
	CreatedAt   time.Time
	mu          sync.RWMutex
}

// NewConnection создает новое соединение
func NewConnection(providerID uuid.UUID) *Connection {
	return &Connection{
		ID:          uuid.New().String(),
		ProviderID:  providerID,
		Bound:       false,
		LastUsed:    time.Now(),
		SequenceNum: 0,
		CreatedAt:   time.Now(),
	}
}

// IsBound проверяет, привязано ли соединение
func (c *Connection) IsBound() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Bound
}

// SetBound устанавливает статус привязки
func (c *Connection) SetBound(bound bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Bound = bound
}

// UpdateLastUsed обновляет время последнего использования
func (c *Connection) UpdateLastUsed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastUsed = time.Now()
}

// IncrementSequence увеличивает sequence number
func (c *Connection) IncrementSequence() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SequenceNum++
	return c.SequenceNum
}
