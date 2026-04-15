package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AggregatorTariff тариф агрегатора для субаккаунта
type AggregatorTariff struct {
	ID              uuid.UUID
	AggregatorID    uuid.UUID
	SubAccountID    *uuid.UUID // nil = default для всех субаккаунтов агрегатора
	OperatorID      uuid.UUID
	SenderCategory  SenderCategory
	PricePerSegment string
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AggregatorMarginLog запись о марже агрегатора по сообщению
type AggregatorMarginLog struct {
	ID              uuid.UUID
	AggregatorID    uuid.UUID
	SubAccountID    uuid.UUID
	MessageID       uuid.UUID
	OperatorID      uuid.UUID
	SegmentCount    int
	SubAccountPrice string // цена субаккаунта (тариф агрегатора)
	AggregatorPrice string // цена агрегатора (платформенный тариф)
	SubAccountTotal string // итого с субаккаунта
	AggregatorTotal string // итого с агрегатора
	Margin          string // маржа = SubAccountTotal - AggregatorTotal
	IdempotencyKey  string
	CreatedAt       time.Time
}

// AggregatorTariffRepository репозиторий тарифов агрегатора
type AggregatorTariffRepository interface {
	// GetForSubAccount ищет тариф с каскадом: сначала специфичный для субаккаунта, потом дефолтный
	GetForSubAccount(ctx context.Context, aggregatorID, subAccountID, operatorID uuid.UUID, category SenderCategory) (*AggregatorTariff, error)
}

// AggregatorMarginLogRepository репозиторий логов маржи агрегатора
type AggregatorMarginLogRepository interface {
	Create(ctx context.Context, entry *AggregatorMarginLog) error
}

// ClientAccountInfo базовая информация о клиенте для определения типа аккаунта
type ClientAccountInfo struct {
	ID           uuid.UUID
	AccountType  string
	ParentClientID *uuid.UUID
}

// ClientRepository минимальный интерфейс для получения информации о клиенте
type ClientInfoRepository interface {
	GetAccountInfo(ctx context.Context, clientID uuid.UUID) (*ClientAccountInfo, error)
}
