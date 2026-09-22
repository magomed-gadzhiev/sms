package application

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// TieredSpec — распакованная структура tiered JSONB.
type TieredSpec struct {
	Period string       `json:"period"`
	Tiers  []TieredTier `json:"tiers"`
}

type TieredTier struct {
	// UpTo = null означает бесконечный тир (последний в массиве).
	UpTo  *int64  `json:"up_to"`
	Price float64 `json:"price"`
}

// PrepaidThresholdSpec — распакованная структура prepaid_threshold JSONB.
type PrepaidThresholdSpec struct {
	Period           string  `json:"period"`
	PrepaidAmount    float64 `json:"prepaid_amount"`
	IncludedSegments int64   `json:"included_segments"`
	OveragePrice     float64 `json:"overage_price"`
}

type cacheKey struct {
	RuleID  uuid.UUID
	Version int64
}

// TiersCache — in-process кеш распарсенных tiers_json. Парсинг JSONB стоит
// микросекунды на сообщение — под высокой нагрузкой это значимо. Кеш снимает
// стоимость до первого вызова per (rule, version).
//
// Инвалидация: ключ включает version. При инкременте price_rules_version
// предыдущий ключ становится недоступен, место освобождается (либо по cap,
// либо естественным GC неиспользуемых entries).
//
// Phase 1 ограничение: нет LRU-eviction. Ожидается несколько сотен rules ×
// несколько версий = тысячи entries = <10MB памяти. Proper LRU — Pillar D.
type TiersCache struct {
	mu      sync.RWMutex
	tiered  map[cacheKey]*TieredSpec
	prepaid map[cacheKey]*PrepaidThresholdSpec
	maxSize int
}

// NewTiersCache создаёт кеш. maxSize — soft limit: при превышении очищаются ВСЕ
// entries (bulk clear). Это грубо, но безопасно: после clear первые несколько
// сообщений парсят заново, дальше всё кешируется. Phase D добавит LRU.
// maxSize <= 0 заменяется на 1024.
func NewTiersCache(maxSize int) *TiersCache {
	if maxSize <= 0 {
		maxSize = 1024
	}
	return &TiersCache{
		tiered:  make(map[cacheKey]*TieredSpec),
		prepaid: make(map[cacheKey]*PrepaidThresholdSpec),
		maxSize: maxSize,
	}
}

func (c *TiersCache) GetOrParseTiered(ruleID uuid.UUID, version int64, raw []byte) (*TieredSpec, error) {
	k := cacheKey{RuleID: ruleID, Version: version}

	c.mu.RLock()
	if v, ok := c.tiered[k]; ok {
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	var spec TieredSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse tiered spec: %w", err)
	}

	c.mu.Lock()
	if len(c.tiered) >= c.maxSize {
		c.tiered = make(map[cacheKey]*TieredSpec)
	}
	c.tiered[k] = &spec
	c.mu.Unlock()
	return &spec, nil
}

func (c *TiersCache) GetOrParsePrepaid(ruleID uuid.UUID, version int64, raw []byte) (*PrepaidThresholdSpec, error) {
	k := cacheKey{RuleID: ruleID, Version: version}

	c.mu.RLock()
	if v, ok := c.prepaid[k]; ok {
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	var spec PrepaidThresholdSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse prepaid spec: %w", err)
	}

	c.mu.Lock()
	if len(c.prepaid) >= c.maxSize {
		c.prepaid = make(map[cacheKey]*PrepaidThresholdSpec)
	}
	c.prepaid[k] = &spec
	c.mu.Unlock()
	return &spec, nil
}
