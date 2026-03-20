package domain

import (
	"time"

	"github.com/google/uuid"
)

// Metric представляет доменную модель метрики
type Metric struct {
	ID           uuid.UUID
	Type         MetricType
	ClientID     *uuid.UUID
	ProviderID   *uuid.UUID
	MessageID    *uuid.UUID
	Status       string
	Value        int64
	SegmentCount int
	Timestamp    time.Time
	Metadata     map[string]interface{}
	CreatedAt    time.Time
}

// MetricType представляет тип метрики
type MetricType string

const (
	MetricTypeMessageCreated   MetricType = "message.created"
	MetricTypeMessageSent      MetricType = "message.sent"
	MetricTypeMessageDelivered MetricType = "message.delivered"
	MetricTypeMessageFailed    MetricType = "message.failed"
)

// NewMetric создает новую метрику
func NewMetric(
	metricType MetricType,
	status string,
	value int64,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
	messageID *uuid.UUID,
) *Metric {
	now := time.Now()
	return &Metric{
		ID:         uuid.New(),
		Type:       metricType,
		Status:     status,
		Value:      value,
		ClientID:   clientID,
		ProviderID: providerID,
		MessageID:  messageID,
		Timestamp:  now,
		CreatedAt:  now,
		Metadata:   make(map[string]interface{}),
	}
}

// AggregatedMetric представляет агрегированную метрику
type AggregatedMetric struct {
	ID          uuid.UUID
	Period      string          // day, hour, minute
	PeriodStart time.Time
	PeriodEnd   time.Time
	ClientID    *uuid.UUID
	ProviderID  *uuid.UUID
	Status      string
	Count       int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewAggregatedMetric создает новую агрегированную метрику
func NewAggregatedMetric(
	period string,
	periodStart time.Time,
	periodEnd time.Time,
	clientID *uuid.UUID,
	providerID *uuid.UUID,
	status string,
	count int64,
) *AggregatedMetric {
	now := time.Now()
	return &AggregatedMetric{
		ID:          uuid.New(),
		Period:      period,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		ClientID:    clientID,
		ProviderID:  providerID,
		Status:      status,
		Count:       count,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
