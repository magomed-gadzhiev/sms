package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderSetItem — одна запись в reseller_provider_set_items.
type ProviderSetItem struct {
	ID                 uuid.UUID
	SetID              uuid.UUID
	ProviderID         uuid.UUID
	Priority           int
	ExposeCost         bool
	ExposeProviderName bool
}

// ProviderSetItemInput — входные данные для добавления/замены item'а.
type ProviderSetItemInput struct {
	ProviderID         uuid.UUID
	Priority           int
	ExposeCost         bool
	ExposeProviderName bool
}

// ResellerProviderSetItemsRepository управляет содержимым provider-set'ов агрегатора.
type ResellerProviderSetItemsRepository struct {
	pool *pgxpool.Pool
}

// NewResellerProviderSetItemsRepository конструирует репозиторий.
func NewResellerProviderSetItemsRepository(pool *pgxpool.Pool) *ResellerProviderSetItemsRepository {
	return &ResellerProviderSetItemsRepository{pool: pool}
}

// ListBySet возвращает все items set'а, отсортированные по убыванию priority.
func (r *ResellerProviderSetItemsRepository) ListBySet(ctx context.Context, setID uuid.UUID) ([]ProviderSetItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, set_id, provider_id, priority, expose_cost, expose_provider_name
		 FROM reseller_provider_set_items WHERE set_id = $1 ORDER BY priority DESC, provider_id`, setID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProviderSetItem
	for rows.Next() {
		var i ProviderSetItem
		if err := rows.Scan(&i.ID, &i.SetID, &i.ProviderID, &i.Priority, &i.ExposeCost, &i.ExposeProviderName); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ReplaceItems атомарно заменяет все items set'а (DELETE + INSERT в одной транзакции).
// Pre-validation вызывается на уровне сервиса/handler'а ДО вызова этого метода.
func (r *ResellerProviderSetItemsRepository) ReplaceItems(ctx context.Context, setID uuid.UUID, items []ProviderSetItemInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM reseller_provider_set_items WHERE set_id = $1`, setID); err != nil {
		return err
	}
	for _, it := range items {
		_, err := tx.Exec(ctx,
			`INSERT INTO reseller_provider_set_items (set_id, provider_id, priority, expose_cost, expose_provider_name)
			 VALUES ($1, $2, $3, $4, $5)`,
			setID, it.ProviderID, it.Priority, it.ExposeCost, it.ExposeProviderName)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListProvidersInSet возвращает список provider_id в set'е.
// Используется service-уровнем при валидации конфликтов provider-set ↔ route-set.
func (r *ResellerProviderSetItemsRepository) ListProvidersInSet(ctx context.Context, setID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT provider_id FROM reseller_provider_set_items WHERE set_id = $1`, setID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
