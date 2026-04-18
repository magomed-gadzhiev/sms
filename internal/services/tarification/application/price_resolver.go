package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// AggregatorResolver — минимальный интерфейс для получения aggregator_id по
// subaccount_id. В проде реализуется через Redis-cache + fallback на DB.
type AggregatorResolver interface {
	AggregatorFor(ctx context.Context, subaccountID uuid.UUID) (uuid.UUID, error)
}

// PriceResolver — горячий путь: ищет применимое правило, используя
// денормализованный resolved_rules кеш (стадия 1) с fallback на полный lookup
// по price_rules (стадия 2) при cache miss или устаревшей версии.
type PriceResolver struct {
	ruleRepo     domain.PriceRuleRepository
	resolvedRepo domain.ResolvedRulesRepository
	versionRepo  domain.PriceRulesVersionRepository
	aggResolver  AggregatorResolver
}

func NewPriceResolver(
	ruleRepo domain.PriceRuleRepository,
	resolvedRepo domain.ResolvedRulesRepository,
	versionRepo domain.PriceRulesVersionRepository,
	aggResolver AggregatorResolver,
) *PriceResolver {
	return &PriceResolver{
		ruleRepo:     ruleRepo,
		resolvedRepo: resolvedRepo,
		versionRepo:  versionRepo,
		aggResolver:  aggResolver,
	}
}

func (r *PriceResolver) Resolve(ctx context.Context, in domain.ResolveInput) (*domain.ResolvedRule, error) {
	aggID, err := r.aggResolver.AggregatorFor(ctx, in.SubaccountID)
	if err != nil {
		return nil, fmt.Errorf("resolve aggregator: %w", err)
	}
	in.AggregatorID = aggID

	effectiveDate := in.Now.UTC().Truncate(24 * time.Hour)

	// Стадия 1 — cache hit?
	cached, err := r.resolvedRepo.Get(ctx, in.SubaccountID,
		in.Country, in.Operator, in.SenderCategory, in.TrafficType, effectiveDate)
	if err != nil {
		return nil, fmt.Errorf("get resolved: %w", err)
	}
	currentVersion, err := r.versionRepo.GetVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	if cached != nil && cached.RulesVersion == currentVersion {
		return cached, nil
	}

	// Стадия 2 — lookup + materialize.
	// ВАЖНО: захватываем версию ДО lookup, а не на момент write. Если price_rules
	// изменились между SELECT и INSERT — запишем старую версию, janitor удалит
	// эту строку при обработке outbox-события. Альтернатива (писать текущую
	// версию) создаёт баг: следующий запрос увидит совпадение версий и
	// использует устаревшие данные.
	versionAtSelect := currentVersion
	rule, err := r.ruleRepo.FindApplicable(ctx, in)
	if err != nil {
		return nil, err
	}

	rr := &domain.ResolvedRule{
		SubaccountID:   in.SubaccountID,
		Country:        in.Country,
		Operator:       in.Operator,
		SenderCategory: in.SenderCategory,
		TrafficType:    in.TrafficType,
		EffectiveDate:  effectiveDate,
		PriceModel:     rule.PriceModel,
		PriceValue:     rule.PriceValue,
		TiersJSON:      rule.TiersJSON,
		SourceRuleID:   rule.ID,
		SourceLevel:    rule.OwnerType,
		AggregatorID:   aggID,
		RulesVersion:   versionAtSelect,
	}
	if err := r.resolvedRepo.Upsert(ctx, rr); err != nil {
		return nil, fmt.Errorf("upsert resolved: %w", err)
	}
	return rr, nil
}
