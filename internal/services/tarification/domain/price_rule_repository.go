package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ResolveInput — запрос на поиск применимого правила.
// AggregatorID заполняется PriceResolver'ом из AggregatorResolver до вызова
// FindApplicable (его знает только application-слой, а не вызывающий сервис).
type ResolveInput struct {
	SubaccountID   uuid.UUID
	AggregatorID   uuid.UUID
	Country        string
	Operator       string
	SenderCategory string
	TrafficType    string
	Now            time.Time
}

// PriceRuleRepository — CRUD + применимый lookup по price_rules.
type PriceRuleRepository interface {
	// FindApplicable реализует алгоритм owner-first + specificity bitmap.
	// Возвращает (nil, ErrNoApplicableRule), если не найдено ни одного правила
	// (это означает нарушение инварианта — catch-all на платформе должен быть
	// всегда).
	FindApplicable(ctx context.Context, in ResolveInput) (*PriceRule, error)

	Create(ctx context.Context, r *PriceRule) error
	Update(ctx context.Context, r *PriceRule) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*PriceRule, error)

	// HasPlatformCatchAll — startup-инвариант чек.
	// Если возвращает false — tarification-service не должен подниматься.
	HasPlatformCatchAll(ctx context.Context) (bool, error)
}

// ResolvedRulesRepository — горячий кеш разрешённых правил.
type ResolvedRulesRepository interface {
	Get(ctx context.Context, subaccountID uuid.UUID, country, operator, senderCategory, trafficType string, effectiveDate time.Time) (*ResolvedRule, error)
	Upsert(ctx context.Context, r *ResolvedRule) error
	// DeleteAffected удаляет строки resolved_rules, затронутые изменением правила.
	// Критерии: ownerType+ownerID (scope инвалидации) + dims (nullable = любое значение).
	// Возвращает число удалённых строк.
	DeleteAffected(ctx context.Context, ownerType PriceOwnerType, ownerID *uuid.UUID, country, operator, senderCategory, trafficType *string) (int64, error)
}

// PriceRulesVersionRepository — монотонная глобальная версия price_rules.
type PriceRulesVersionRepository interface {
	GetVersion(ctx context.Context) (int64, error)
}

// SubaccountUsageCounterRepository — счётчик потребления per-subaccount-per-period.
type SubaccountUsageCounterRepository interface {
	// Increment — атомарный UPSERT. Возвращает segments_used ПОСЛЕ инкремента.
	Increment(ctx context.Context, subaccountID uuid.UUID, periodKey string, segments int64, amountCharged string) (int64, error)
	Get(ctx context.Context, subaccountID uuid.UUID, periodKey string) (*SubaccountUsageCounter, error)
}

// InvalidationEvent — событие из resolved_rules_invalidation_outbox.
// Генерируется триггером price_rules_after_change() при любом изменении price_rules.
type InvalidationEvent struct {
	ID        int64
	RuleID    uuid.UUID
	OwnerType PriceOwnerType
	OwnerID   *uuid.UUID
	Country   *string
	Operator  *string
	SenderCat *string
	Traffic   *string
	CreatedAt time.Time
}

type InvalidationOutboxRepository interface {
	ListUnprocessed(ctx context.Context, limit int) ([]InvalidationEvent, error)
	MarkProcessed(ctx context.Context, ids []int64) error
}
