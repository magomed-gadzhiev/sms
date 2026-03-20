package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MetricRepository определяет интерфейс для работы с метриками
type MetricRepository interface {
	Create(ctx context.Context, metric *Metric) error
	CreateAggregated(ctx context.Context, metric *AggregatedMetric) error
	GetAggregated(ctx context.Context, filters *AggregateFilters) ([]*AggregatedMetric, error)
	GetStatistics(ctx context.Context, filters *StatisticsFilters) (*Statistics, error)
	GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*ProviderPerformance, error)
	UpdateAggregated(ctx context.Context, metric *AggregatedMetric) error
}

// ReportRepository определяет интерфейс для работы с отчетами
type ReportRepository interface {
	Create(ctx context.Context, report *Report) error
	GetByID(ctx context.Context, reportID uuid.UUID) (*Report, error)
	List(ctx context.Context, filters *ReportFilters) ([]*Report, error)
}

// AggregateFilters содержит фильтры для агрегированных метрик
type AggregateFilters struct {
	Period      string        // day, hour, minute
	PeriodStart time.Time
	PeriodEnd   time.Time
	ClientID    *uuid.UUID
	ProviderID  *uuid.UUID
	Status      *string
}

// StatisticsFilters содержит фильтры для статистики
type StatisticsFilters struct {
	ClientID    *uuid.UUID
	From        time.Time
	To          time.Time
	GroupBy     string      // day, hour, provider, status
	ProviderIDs []uuid.UUID
}

// Statistics представляет статистику
type Statistics struct {
	Groups []*StatisticGroup
	Totals *TotalStats
}

// StatisticGroup представляет группу статистики
type StatisticGroup struct {
	Key   string
	Stats *TotalStats
}

// TotalStats представляет общую статистику
type TotalStats struct {
	TotalSent           int64
	TotalDelivered      int64
	TotalFailed         int64
	TotalPending        int64
	TotalQueued         int64
	TotalSegments       int64
	SuccessRate         int32
	AvgDeliveryTimeMs   int64
}

// ProviderPerformance представляет производительность провайдера
type ProviderPerformance struct {
	ProviderID        uuid.UUID
	TotalSent         int64
	TotalDelivered    int64
	TotalFailed       int64
	SuccessRate       int32
	AvgDeliveryTimeMs int64
	StatusBreakdown   map[string]int64
	Points            []*PerformancePoint
}

// PerformancePoint представляет точку производительности
type PerformancePoint struct {
	Timestamp  time.Time
	Sent       int64
	Delivered  int64
	Failed     int64
	SuccessRate int32
}

// ReportFilters содержит фильтры для списка отчетов
type ReportFilters struct {
	ReportType *ReportType
	ClientID   *uuid.UUID
	ProviderID *uuid.UUID
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}
