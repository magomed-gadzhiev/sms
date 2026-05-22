package network

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderSetMaterializer материализует provider-set агрегатора в client_providers суб-аккаунта.
// При применении: записи ownership='inherited' удаляются и пересоздаются из items;
// записи ownership='private' (override) не затрагиваются.
type ProviderSetMaterializer struct {
	pool *pgxpool.Pool
}

// NewProviderSetMaterializer конструирует сервис.
func NewProviderSetMaterializer(pool *pgxpool.Pool) *ProviderSetMaterializer {
	return &ProviderSetMaterializer{pool: pool}
}

// ApplyToClient материализует provider-set в client_providers под транзакцией.
// providerSetID == nil → только стереть inherited-записи (без вставки новых).
// Записи ownership='private' (override) не затрагиваются в обоих случаях.
func (m *ProviderSetMaterializer) ApplyToClient(ctx context.Context, clientID uuid.UUID, providerSetID *uuid.UUID) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Шаг 1: удалить все inherited-записи для клиента.
	if _, err := tx.Exec(ctx,
		`DELETE FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		clientID,
	); err != nil {
		return err
	}

	// Шаг 2: вставить новые inherited-записи из items, если set указан.
	// NOT EXISTS исключает provider_id, у которых уже есть private override.
	// ON CONFLICT DO NOTHING: при существующей записи с ownership='platform' для того же
	// (client_id, provider_id) — inherited не вставляется. Platform-записи редки в reseller-сценарии
	// (только для самостоятельных клиентов вне иерархии), но если они есть — выигрывают.
	// private-записи отфильтрованы NOT EXISTS выше.
	if providerSetID != nil {
		_, err := tx.Exec(ctx, `
			INSERT INTO client_providers
				(client_id, provider_id, ownership, source_client_id, shared_priority,
				 expose_cost, expose_provider_name, active)
			SELECT $1, items.provider_id, 'inherited', c.parent_client_id,
			       items.priority, items.expose_cost, items.expose_provider_name, true
			FROM reseller_provider_set_items items
			JOIN clients c ON c.id = $1
			WHERE items.set_id = $2
			  AND NOT EXISTS (
			    SELECT 1 FROM client_providers cp
			    WHERE cp.client_id = $1
			      AND cp.provider_id = items.provider_id
			      AND cp.ownership = 'private'
			  )
			ON CONFLICT (client_id, provider_id) DO NOTHING
		`, clientID, *providerSetID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ApplyToAllSubscribers применяет provider-set ко всем суб-аккаунтам, подписанным на него.
// Использует **per-client транзакции** — не атомарна для всей пачки.
// Partial failure: при ошибке у клиента N итерация прерывается, клиенты [0..N-1] уже COMMIT'нуты,
// клиент N и [N+1..end] не материализованы. Caller отвечает за retry / откат на уровне выше.
// Этот выбор сделан осознанно — единая транзакция на 1000+ суб-аккаунтов даёт долгий lock и риск
// OOM на батче. См. spec §4.7 «Атомарная транзакция для bulk на 100+ — долгий запрос».
func (m *ProviderSetMaterializer) ApplyToAllSubscribers(ctx context.Context, providerSetID uuid.UUID) error {
	rows, err := m.pool.Query(ctx,
		`SELECT client_id FROM subaccount_routing_assignment WHERE provider_set_id = $1`,
		providerSetID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var clientIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		clientIDs = append(clientIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, cid := range clientIDs {
		if err := m.ApplyToClient(ctx, cid, &providerSetID); err != nil {
			return err
		}
	}
	return nil
}
