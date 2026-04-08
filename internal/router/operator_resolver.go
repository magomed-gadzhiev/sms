package router

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
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
	logger            zerolog.Logger
}

func NewOperatorResolver(repo OperatorPrefixRepository, defaultOperatorID uuid.UUID) *OperatorResolver {
	return &OperatorResolver{
		repo:              repo,
		defaultOperatorID: defaultOperatorID,
		refreshTTL:        5 * time.Minute,
		logger:            log.With().Str("component", "operator_resolver").Logger(),
	}
}

// Resolve возвращает operator_id для номера.
func (r *OperatorResolver) Resolve(ctx context.Context, number string) uuid.UUID {
	normalized := normalizePhoneNumber(number)
	if normalized == "" {
		r.logger.Debug().
			Str("number", number).
			Str("operator_id", r.defaultOperatorID.String()).
			Msg("empty number, using default operator")
		return r.defaultOperatorID
	}

	r.mu.RLock()
	stale := time.Since(r.lastRefresh) > r.refreshTTL
	r.mu.RUnlock()

	if stale {
		r.refresh(ctx)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.prefixes {
		if len(normalized) >= len(p.Prefix) && normalized[:len(p.Prefix)] == p.Prefix {
			r.logger.Debug().
				Str("number", normalized).
				Str("prefix", p.Prefix).
				Str("operator_id", p.OperatorID.String()).
				Msg("operator resolved by prefix")
			return p.OperatorID
		}
	}

	r.logger.Debug().
		Str("number", normalized).
		Str("operator_id", r.defaultOperatorID.String()).
		Msg("no prefix match, using default operator")
	return r.defaultOperatorID
}

func normalizePhoneNumber(number string) string {
	return strings.TrimPrefix(strings.TrimSpace(number), "+")
}

func (r *OperatorResolver) refresh(ctx context.Context) {
	prefixes, err := r.repo.GetAllActive(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to load operator prefixes")
		return
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i].Prefix) > len(prefixes[j].Prefix)
	})
	r.mu.Lock()
	r.prefixes = prefixes
	r.lastRefresh = time.Now()
	r.mu.Unlock()
	r.logger.Info().Int("prefix_count", len(prefixes)).Msg("operator prefixes reloaded")
}
