// rollout.go
package application

import (
	"hash/fnv"

	"github.com/google/uuid"
)

// Rollout — детерминистичный gate для staged rollout unified-пути.
// Percentage ∈ [0..100]. hash(subaccount_id) % 100 < percentage.
// Один и тот же subaccount всегда получает одинаковый ответ при том же
// percentage — клиенты не «скачут» между unified и legacy в пределах релиза.
type Rollout struct {
	percentage int
}

func NewRollout(percentage int) *Rollout {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	return &Rollout{percentage: percentage}
}

func (r *Rollout) Enabled(subaccountID uuid.UUID) bool {
	if r.percentage <= 0 {
		return false
	}
	if r.percentage >= 100 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write(subaccountID[:])
	return int(h.Sum32()%100) < r.percentage
}

func (r *Rollout) Percentage() int { return r.percentage }
