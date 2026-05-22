package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetric_AllFieldsPopulated(t *testing.T) {
	clientID := uuid.New()
	providerID := uuid.New()
	messageID := uuid.New()

	before := time.Now()
	m := NewMetric(MetricTypeMessageSent, "sent", 42, &clientID, &providerID, &messageID)
	after := time.Now()

	require.NotNil(t, m)
	assert.NotEqual(t, uuid.Nil, m.ID)
	assert.Equal(t, MetricTypeMessageSent, m.Type)
	assert.Equal(t, "sent", m.Status)
	assert.Equal(t, int64(42), m.Value)
	assert.Equal(t, &clientID, m.ClientID)
	assert.Equal(t, &providerID, m.ProviderID)
	assert.Equal(t, &messageID, m.MessageID)
	assert.False(t, m.Timestamp.Before(before))
	assert.False(t, m.Timestamp.After(after))
	assert.False(t, m.CreatedAt.Before(before))
	assert.False(t, m.CreatedAt.After(after))
	assert.NotNil(t, m.Metadata)
	assert.Empty(t, m.Metadata)
	assert.Equal(t, 0, m.SegmentCount)
}

func TestNewMetric_NilOptionalFields(t *testing.T) {
	m := NewMetric(MetricTypeMessageCreated, "created", 1, nil, nil, nil)

	require.NotNil(t, m)
	assert.Nil(t, m.ClientID)
	assert.Nil(t, m.ProviderID)
	assert.Nil(t, m.MessageID)
}

func TestNewMetric_ZeroValue(t *testing.T) {
	m := NewMetric(MetricTypeMessageFailed, "failed", 0, nil, nil, nil)

	require.NotNil(t, m)
	assert.Equal(t, int64(0), m.Value)
}

func TestNewMetric_NegativeValue(t *testing.T) {
	m := NewMetric(MetricTypeMessageFailed, "failed", -1, nil, nil, nil)

	require.NotNil(t, m)
	assert.Equal(t, int64(-1), m.Value)
}

func TestNewMetric_EmptyStatus(t *testing.T) {
	m := NewMetric(MetricTypeMessageCreated, "", 1, nil, nil, nil)

	require.NotNil(t, m)
	assert.Equal(t, "", m.Status)
}

func TestNewMetric_UniqueIDs(t *testing.T) {
	m1 := NewMetric(MetricTypeMessageSent, "sent", 1, nil, nil, nil)
	m2 := NewMetric(MetricTypeMessageSent, "sent", 1, nil, nil, nil)

	assert.NotEqual(t, m1.ID, m2.ID)
}

func TestNewMetric_MetadataIsWritable(t *testing.T) {
	m := NewMetric(MetricTypeMessageDelivered, "delivered", 1, nil, nil, nil)
	m.Metadata["key"] = "value"

	assert.Equal(t, "value", m.Metadata["key"])
}

func TestMetricTypeConstants(t *testing.T) {
	assert.Equal(t, MetricType("message.created"), MetricTypeMessageCreated)
	assert.Equal(t, MetricType("message.sent"), MetricTypeMessageSent)
	assert.Equal(t, MetricType("message.delivered"), MetricTypeMessageDelivered)
	assert.Equal(t, MetricType("message.failed"), MetricTypeMessageFailed)
}

func TestNewAggregatedMetric_AllFieldsPopulated(t *testing.T) {
	clientID := uuid.New()
	providerID := uuid.New()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	before := time.Now()
	am := NewAggregatedMetric("day", start, end, &clientID, &providerID, "delivered", 100)
	after := time.Now()

	require.NotNil(t, am)
	assert.NotEqual(t, uuid.Nil, am.ID)
	assert.Equal(t, "day", am.Period)
	assert.Equal(t, start, am.PeriodStart)
	assert.Equal(t, end, am.PeriodEnd)
	assert.Equal(t, &clientID, am.ClientID)
	assert.Equal(t, &providerID, am.ProviderID)
	assert.Equal(t, "delivered", am.Status)
	assert.Equal(t, int64(100), am.Count)
	assert.False(t, am.CreatedAt.Before(before))
	assert.False(t, am.CreatedAt.After(after))
	assert.False(t, am.UpdatedAt.Before(before))
	assert.False(t, am.UpdatedAt.After(after))
}

func TestNewAggregatedMetric_NilOptionalFields(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)

	am := NewAggregatedMetric("hour", start, end, nil, nil, "sent", 50)

	require.NotNil(t, am)
	assert.Nil(t, am.ClientID)
	assert.Nil(t, am.ProviderID)
}

func TestNewAggregatedMetric_ZeroCount(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Hour)

	am := NewAggregatedMetric("hour", start, end, nil, nil, "sent", 0)

	require.NotNil(t, am)
	assert.Equal(t, int64(0), am.Count)
}

func TestNewAggregatedMetric_UniqueIDs(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Hour)

	am1 := NewAggregatedMetric("hour", start, end, nil, nil, "sent", 1)
	am2 := NewAggregatedMetric("hour", start, end, nil, nil, "sent", 1)

	assert.NotEqual(t, am1.ID, am2.ID)
}

func TestNewAggregatedMetric_EmptyPeriod(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Hour)

	am := NewAggregatedMetric("", start, end, nil, nil, "sent", 1)

	require.NotNil(t, am)
	assert.Equal(t, "", am.Period)
}
