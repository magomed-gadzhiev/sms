package network

import (
	"context"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// ConflictValidator проверяет провайдер-инвариант (spec §6.1):
// каждый provider_id в правилах route-set должен быть в provider-set.
type ConflictValidator struct {
	psItems *storage.ResellerProviderSetItemsRepository
	rsItems *storage.ResellerRouteSetItemsRepository
}

func NewConflictValidator(ps *storage.ResellerProviderSetItemsRepository, rs *storage.ResellerRouteSetItemsRepository) *ConflictValidator {
	return &ConflictValidator{psItems: ps, rsItems: rs}
}

// missingProviders — in-memory set diff: rsIDs \ psIDs.
func missingProviders(psIDs, rsIDs []uuid.UUID) []uuid.UUID {
	psSet := make(map[uuid.UUID]struct{}, len(psIDs))
	for _, id := range psIDs {
		psSet[id] = struct{}{}
	}
	var missing []uuid.UUID
	for _, id := range rsIDs {
		if _, ok := psSet[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

// MissingProviders — provider_id'ы, упомянутые в route-set, но отсутствующие в provider-set.
// Если provider-set пустой — все провайдеры route-set считаются missing.
func (v *ConflictValidator) MissingProviders(ctx context.Context, providerSetID, routeSetID uuid.UUID) ([]uuid.UUID, error) {
	psIDs, err := v.psItems.ListProvidersInSet(ctx, providerSetID)
	if err != nil {
		return nil, err
	}
	rsIDs, err := v.rsItems.ProvidersInSet(ctx, routeSetID)
	if err != nil {
		return nil, err
	}
	return missingProviders(psIDs, rsIDs), nil
}

// MissingProvidersByIDs — то же, но обрабатывает nil-указатели.
// nil route-set → пустой результат (нечего проверять).
// nil provider-set + не-nil route-set → все провайдеры route-set'а — missing.
func (v *ConflictValidator) MissingProvidersByIDs(ctx context.Context, providerSetID, routeSetID *uuid.UUID) ([]uuid.UUID, error) {
	if routeSetID == nil {
		return nil, nil
	}
	rsIDs, err := v.rsItems.ProvidersInSet(ctx, *routeSetID)
	if err != nil {
		return nil, err
	}
	if providerSetID == nil {
		return rsIDs, nil
	}
	psIDs, err := v.psItems.ListProvidersInSet(ctx, *providerSetID)
	if err != nil {
		return nil, err
	}
	return missingProviders(psIDs, rsIDs), nil
}
