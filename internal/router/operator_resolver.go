package router

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// OperatorPrefixRepository загружает префиксы из БД.
type OperatorPrefixRepository interface {
	GetAllActive(ctx context.Context) ([]shared.OperatorPrefix, error)
}

// OperatorResolver определяет operator_id по номеру телефона.
type OperatorResolver struct {
	repo              OperatorPrefixRepository
	defaultOperatorID uuid.UUID
	prefixes          []shared.OperatorPrefix
	mu                sync.RWMutex
	refreshTTL        time.Duration
	lastRefresh       time.Time
}

func NewOperatorResolver(repo OperatorPrefixRepository, defaultOperatorID uuid.UUID) *OperatorResolver {
	return &OperatorResolver{
		repo:              repo,
		defaultOperatorID: defaultOperatorID,
		refreshTTL:        5 * time.Minute,
	}
}

// Resolve возвращает operator_id для номера.
func (r *OperatorResolver) Resolve(ctx context.Context, number string) uuid.UUID {
	r.mu.RLock()
	stale := time.Since(r.lastRefresh) > r.refreshTTL
	r.mu.RUnlock()

	if stale {
		r.refresh(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.prefixes {
		if len(number) >= len(p.Prefix) && number[:len(p.Prefix)] == p.Prefix {
			return p.OperatorID
		}
	}
	return r.defaultOperatorID
}

func (r *OperatorResolver) refresh(ctx context.Context) {
	prefixes, err := r.repo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("OperatorResolver: ошибка загрузки префиксов")
		return
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i].Prefix) > len(prefixes[j].Prefix)
	})
	r.mu.Lock()
	r.prefixes = prefixes
	r.lastRefresh = time.Now()
	r.mu.Unlock()
}
