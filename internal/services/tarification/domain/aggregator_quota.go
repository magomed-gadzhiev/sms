package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AggregatorQuota represents an infrastructure usage quota for an aggregator.
// All sub-accounts share this pool counter.
type AggregatorQuota struct {
	ID             uuid.UUID
	AggregatorID   uuid.UUID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	SegmentLimit   int64
	SegmentsUsed   int64
	OverageRate    string // NUMERIC as string for precision
	Currency       string
	AutoRenew      bool
	Notified80Pct  bool
	Notified100Pct bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsExhausted returns true if the quota has been fully consumed.
func (q *AggregatorQuota) IsExhausted() bool {
	return q.SegmentsUsed >= q.SegmentLimit
}

// UtilizationPercent returns current usage as a percentage (0-100+).
func (q *AggregatorQuota) UtilizationPercent() float64 {
	if q.SegmentLimit == 0 {
		return 100
	}
	return float64(q.SegmentsUsed) / float64(q.SegmentLimit) * 100
}

// OverageSegments returns how many segments are over the limit.
func (q *AggregatorQuota) OverageSegments() int64 {
	if q.SegmentsUsed <= q.SegmentLimit {
		return 0
	}
	return q.SegmentsUsed - q.SegmentLimit
}

// IncrementResult is returned by the atomic increment operation.
type IncrementResult struct {
	SegmentsUsed   int64
	SegmentLimit   int64
	OverageRate    string
	WasWithinQuota bool // true if segments_used was below limit BEFORE this increment
	OverageCount   int  // how many of the incremented segments are overage
}

// AggregatorQuotaRepository provides persistence for aggregator quotas.
type AggregatorQuotaRepository interface {
	Create(ctx context.Context, quota *AggregatorQuota) error
	GetByID(ctx context.Context, id uuid.UUID) (*AggregatorQuota, error)
	GetActive(ctx context.Context, aggregatorID uuid.UUID, now time.Time) (*AggregatorQuota, error)
	Update(ctx context.Context, quota *AggregatorQuota) error
	IncrementUsage(ctx context.Context, quotaID uuid.UUID, segments int) (*IncrementResult, error)
	ListByAggregator(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*AggregatorQuota, int, error)
	SetNotified80(ctx context.Context, quotaID uuid.UUID) error
	SetNotified100(ctx context.Context, quotaID uuid.UUID) error
	ListAutoRenewable(ctx context.Context, beforeDate time.Time) ([]*AggregatorQuota, error)
}
