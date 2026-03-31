package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ChannelRepository определяет интерфейс репозитория каналов доставки
type ChannelRepository interface {
	List(ctx context.Context) ([]*ChannelConfig, error)
	Get(ctx context.Context, id uuid.UUID) (*ChannelConfig, error)
	GetByType(ctx context.Context, ct ChannelType) (*ChannelConfig, error)
	Create(ctx context.Context, ch *ChannelConfig) error
	Update(ctx context.Context, ch *ChannelConfig) error
	Toggle(ctx context.Context, id uuid.UUID, active bool) error
}

// StrategyRepository определяет интерфейс репозитория стратегий доставки
type StrategyRepository interface {
	List(ctx context.Context, activeOnly bool) ([]*DeliveryStrategy, error)
	Get(ctx context.Context, id uuid.UUID) (*DeliveryStrategy, error)
	Create(ctx context.Context, s *DeliveryStrategy) error
	Update(ctx context.Context, s *DeliveryStrategy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DeliveryRepository определяет интерфейс репозитория доставок
type DeliveryRepository interface {
	Create(ctx context.Context, d *Delivery) error
	Get(ctx context.Context, id uuid.UUID) (*Delivery, error)
	GetByClientID(ctx context.Context, id, clientID uuid.UUID) (*Delivery, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status DeliveryStatus, deliveredVia string) error
	UpdateStep(ctx context.Context, id uuid.UUID, step int) error
	UpdateCost(ctx context.Context, id uuid.UUID, totalCost float64) error
	List(ctx context.Context, filter DeliveryFilter) ([]*Delivery, int, error)
	HasActiveByStrategy(ctx context.Context, strategyID uuid.UUID) (bool, error)
	Stats(ctx context.Context, filter StatsFilter) (*DeliveryStats, error)
}

// DeliveryFilter представляет фильтр для списка доставок
type DeliveryFilter struct {
	ClientID   uuid.UUID
	StrategyID *uuid.UUID
	Status     string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
}

// StatsFilter представляет фильтр для статистики доставок
type StatsFilter struct {
	ClientID   uuid.UUID
	StrategyID *uuid.UUID
	DateFrom   time.Time
	DateTo     time.Time
}

// DeliveryStats представляет агрегированную статистику доставок
type DeliveryStats struct {
	Total          int64
	DeliveredCount int64
	FailedCount    int64
	DeliveryRate   float64
	ChannelStats   []ChannelStat
	AvgCost        float64
	TotalCost      float64
	Currency       string
}

// ChannelStat представляет статистику по каналу доставки
type ChannelStat struct {
	ChannelType    string
	AttemptCount   int64
	DeliveredCount int64
	DeliveryRate   float64
	AvgLatencyMs   float64
	AvgCost        float64
}

// AttemptRepository определяет интерфейс репозитория попыток доставки
type AttemptRepository interface {
	Create(ctx context.Context, a *DeliveryAttempt) error
	Get(ctx context.Context, id uuid.UUID) (*DeliveryAttempt, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status AttemptStatus, providerRef string, errMsg string, resultAt *time.Time) error
	UpdateCost(ctx context.Context, id uuid.UUID, cost float64) error
	ListByDelivery(ctx context.Context, deliveryID uuid.UUID) ([]*DeliveryAttempt, error)
	FindPendingTimedOut(ctx context.Context) ([]*DeliveryAttempt, error)
}

// OperatorSupportRepository определяет интерфейс репозитория поддержки каналов операторами
type OperatorSupportRepository interface {
	List(ctx context.Context, operatorID *uuid.UUID) ([]*OperatorChannelSupport, error)
	Upsert(ctx context.Context, ocs *OperatorChannelSupport) error
	GetSupport(ctx context.Context, operatorID uuid.UUID, channelType ChannelType) (*OperatorChannelSupport, error)
}
