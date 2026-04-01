package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// ProviderRepository реализует domain.ProviderRepository
// Пока используем локальную БД, в будущем можно заменить на gRPC вызов к Provider Service
type ProviderRepository struct {
	repo *storage.ProviderRepository
	db   *sqlx.DB
}

// NewProviderRepository создает новый репозиторий провайдеров
func NewProviderRepository(db *sqlx.DB) *ProviderRepository {
	storageDB := &storage.DB{DB: db.DB}
	storageRepo := storage.NewProviderRepository(storageDB)
	return &ProviderRepository{
		repo: storageRepo,
		db:   db,
	}
}

// GetByID получает информацию о провайдере по ID
func (r *ProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProviderInfo, error) {
	provider, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.ProviderInfo{
		ID:              provider.ID,
		Name:            provider.Name,
		Active:          provider.Active,
		Priority:        provider.Priority,
		ThroughputPerSec: provider.ThroughputPerSec,
	}, nil
}

// GetAllActive получает все активные провайдеры
func (r *ProviderRepository) GetAllActive(ctx context.Context) ([]*domain.ProviderInfo, error) {
	providers, err := r.repo.GetAllActive(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]*domain.ProviderInfo, len(providers))
	for i, provider := range providers {
		infos[i] = &domain.ProviderInfo{
			ID:              provider.ID,
			Name:            provider.Name,
			Active:          provider.Active,
			Priority:        provider.Priority,
			ThroughputPerSec: provider.ThroughputPerSec,
		}
	}

	return infos, nil
}

// GetHealth получает информацию о здоровье провайдера
// Пока возвращаем простую информацию, в будущем можно получать через Provider Service
func (r *ProviderRepository) GetHealth(ctx context.Context, id uuid.UUID) (*domain.ProviderHealth, error) {
	provider, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Пока возвращаем базовую информацию
	// В реальной реализации нужно получать эту информацию из Provider Service
	health := &domain.ProviderHealth{
		Status: "healthy",
		ActiveConnections: 0,
		TotalConnections:  provider.MaxConnections,
		SuccessRate:       100, // По умолчанию 100%
	}

	if !provider.Active {
		health.Status = "unhealthy"
	}

	return health, nil
}

// GetHealthBatch получает информацию о здоровье нескольких провайдеров за один запрос.
func (r *ProviderRepository) GetHealthBatch(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*domain.ProviderHealth, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*domain.ProviderHealth{}, nil
	}

	// Build IN list for the query.
	query, args, err := sqlx.In(
		`SELECT id, active, max_connections FROM providers WHERE id IN (?)`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("GetHealthBatch: build query: %w", err)
	}
	query = r.db.Rebind(query)

	type row struct {
		ID             uuid.UUID `db:"id"`
		Active         bool      `db:"active"`
		MaxConnections int       `db:"max_connections"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("GetHealthBatch: query: %w", err)
	}

	result := make(map[uuid.UUID]*domain.ProviderHealth, len(rows))
	for _, r := range rows {
		health := &domain.ProviderHealth{
			Status:           "healthy",
			ActiveConnections: 0,
			TotalConnections:  r.MaxConnections,
			SuccessRate:       100,
		}
		if !r.Active {
			health.Status = "unhealthy"
		}
		result[r.ID] = health
	}
	return result, nil
}