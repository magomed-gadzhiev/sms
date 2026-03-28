package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/stretchr/testify/assert"
)

func TestSelectWinnerByDeliveryRate(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 200, DeliveredCount: 180, FailedCount: 20},
		{ID: uuid.New(), Name: "B", SentCount: 200, DeliveredCount: 150, FailedCount: 50},
	}
	winner := selectBestVariant(variants, "delivery_rate")
	assert.Equal(t, variants[0].ID, winner.ID) // A has 90% vs B's 75%
}

func TestSelectWinnerByClickRate(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	variants := []domain.Variant{
		{ID: a, Name: "A", SentCount: 200, DeliveredCount: 180, ClickCount: 10},
		{ID: b, Name: "B", SentCount: 200, DeliveredCount: 150, ClickCount: 20},
	}
	winner := selectBestVariant(variants, "click_rate")
	assert.Equal(t, b, winner.ID) // B has higher click rate
}

func TestSelectWinnerTie(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 200, DeliveredCount: 180},
		{ID: uuid.New(), Name: "B", SentCount: 201, DeliveredCount: 181}, // Almost same rate
	}
	winner := selectBestVariant(variants, "delivery_rate")
	// When tie (<1% diff), pick the one with larger sample
	assert.Equal(t, variants[1].ID, winner.ID)
}

func TestMinSampleSize(t *testing.T) {
	variants := []domain.Variant{
		{ID: uuid.New(), Name: "A", SentCount: 50},
		{ID: uuid.New(), Name: "B", SentCount: 50},
	}
	assert.False(t, hasMinSampleSize(variants))

	variants[0].SentCount = 100
	variants[1].SentCount = 100
	assert.True(t, hasMinSampleSize(variants))
}
