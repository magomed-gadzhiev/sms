// operator_lookup.go
package application

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// OperatorCodeLookup — абстракция резолва UUID → operators.code.
type OperatorCodeLookup interface {
	Code(ctx context.Context, operatorID uuid.UUID) (string, error)
}

// operatorCodeSource — минимальный интерфейс, нужный кешу.
type operatorCodeSource interface {
	GetCodeByID(ctx context.Context, id uuid.UUID) (string, error)
}

// CachedOperatorLookup — in-memory map без TTL. Операторы — справочник
// (десятки записей), код иммутабелен, инвалидация через рестарт сервиса.
type CachedOperatorLookup struct {
	inner operatorCodeSource
	mu    sync.RWMutex
	cache map[uuid.UUID]string
}

// NewCachedOperatorLookup создаёт lookup с пустым кешем.
func NewCachedOperatorLookup(inner operatorCodeSource) *CachedOperatorLookup {
	return &CachedOperatorLookup{
		inner: inner,
		cache: make(map[uuid.UUID]string),
	}
}

// Code возвращает operators.code для заданного UUID.
// При кеш-хите — возвращает без обращения к БД.
// При ошибке inner — НЕ кеширует результат, следующий вызов повторит запрос.
func (c *CachedOperatorLookup) Code(ctx context.Context, id uuid.UUID) (string, error) {
	c.mu.RLock()
	if code, ok := c.cache[id]; ok {
		c.mu.RUnlock()
		return code, nil
	}
	c.mu.RUnlock()

	code, err := c.inner.GetCodeByID(ctx, id)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.cache[id] = code
	c.mu.Unlock()
	return code, nil
}
