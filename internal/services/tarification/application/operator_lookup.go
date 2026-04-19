// operator_lookup.go
package application

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// OperatorMeta — метаданные оператора, необходимые tarification hot path.
// Currency источник — countries.currency по operator.country_id.
type OperatorMeta struct {
	Code     string
	Currency string
}

// OperatorMetaLookup — абстракция резолва UUID → OperatorMeta.
type OperatorMetaLookup interface {
	Meta(ctx context.Context, operatorID uuid.UUID) (OperatorMeta, error)
}

// operatorMetaSource — минимальный интерфейс, нужный кешу.
type operatorMetaSource interface {
	GetMetaByID(ctx context.Context, id uuid.UUID) (OperatorMeta, error)
}

// CachedOperatorLookup — in-memory map без TTL. Операторы — справочник
// (десятки записей), код/страна иммутабельны в пределах инстанса, инвалидация
// через рестарт сервиса.
type CachedOperatorLookup struct {
	inner operatorMetaSource
	mu    sync.RWMutex
	cache map[uuid.UUID]OperatorMeta
}

// NewCachedOperatorLookup создаёт lookup с пустым кешем.
func NewCachedOperatorLookup(inner operatorMetaSource) *CachedOperatorLookup {
	return &CachedOperatorLookup{
		inner: inner,
		cache: make(map[uuid.UUID]OperatorMeta),
	}
}

// Meta возвращает OperatorMeta для заданного UUID.
// При кеш-хите — возвращает без обращения к БД.
// При ошибке inner — НЕ кеширует результат, следующий вызов повторит запрос.
func (c *CachedOperatorLookup) Meta(ctx context.Context, id uuid.UUID) (OperatorMeta, error) {
	c.mu.RLock()
	if meta, ok := c.cache[id]; ok {
		c.mu.RUnlock()
		return meta, nil
	}
	c.mu.RUnlock()

	meta, err := c.inner.GetMetaByID(ctx, id)
	if err != nil {
		return OperatorMeta{}, err
	}
	c.mu.Lock()
	c.cache[id] = meta
	c.mu.Unlock()
	return meta, nil
}
