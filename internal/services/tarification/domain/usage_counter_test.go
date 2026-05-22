package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewUsageCounter(t *testing.T) {
	t.Run("creates counter with zero segment count", func(t *testing.T) {
		clientID := uuid.New()
		planID := uuid.New()
		periodID := uuid.New()

		counter := NewUsageCounter(clientID, planID, periodID)

		assert.NotEqual(t, uuid.Nil, counter.ID)
		assert.Equal(t, clientID, counter.ClientID)
		assert.Equal(t, planID, counter.TariffPlanID)
		assert.Equal(t, periodID, counter.TariffPeriodID)
		assert.Equal(t, 0, counter.SegmentCount)
		assert.False(t, counter.UpdatedAt.IsZero())
	})

	// Note: UsageCounter has no public methods beyond the constructor.
	// Segment count manipulation is handled at the repository/service layer.
}
