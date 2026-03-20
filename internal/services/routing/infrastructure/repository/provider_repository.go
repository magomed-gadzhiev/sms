package repository

import (
	"context"

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