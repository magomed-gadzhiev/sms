package repository

import (
	"context"
	"regexp"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// RouteRepository реализует domain.RouteRepository
type RouteRepository struct {
	repo *storage.RouteRepository
	db   *sqlx.DB
}

// NewRouteRepository создает новый репозиторий маршрутов
func NewRouteRepository(db *sqlx.DB) *RouteRepository {
	// Создаем storage.DB обертку из sql.DB
	storageDB := &storage.DB{DB: db.DB}
	storageRepo := storage.NewRouteRepository(storageDB)
	return &RouteRepository{
		repo: storageRepo,
		db:   db,
	}
}

// Create создает новый маршрут
func (r *RouteRepository) Create(ctx context.Context, route *domain.Route) error {
	// Сохраняем основной провайдер и failover провайдера
	// Используем первый провайдер как основной, второй как failover
	var providerID uuid.UUID
	var failoverProviderID *uuid.UUID

	if len(route.ProviderIDs) > 0 {
		providerID = route.ProviderIDs[0]
		if route.FailoverEnabled && len(route.ProviderIDs) > 1 {
			failoverID := route.ProviderIDs[1]
			failoverProviderID = &failoverID
		}
	}

	query := `
		INSERT INTO routes (id, name, pattern, pattern_type, provider_id, priority, active, failover_provider_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		route.ID,
		route.Name,
		route.Pattern,
		string(route.PatternType),
		providerID,
		route.Priority,
		route.Active,
		failoverProviderID,
		route.CreatedAt,
		route.UpdatedAt,
	)
	return err
}

// GetByID получает маршрут по ID
func (r *RouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	sharedRoute, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domain.RouteFromShared(sharedRoute), nil
}

// Update обновляет маршрут
func (r *RouteRepository) Update(ctx context.Context, route *domain.Route) error {
	var providerID uuid.UUID
	var failoverProviderID *uuid.UUID

	if len(route.ProviderIDs) > 0 {
		providerID = route.ProviderIDs[0]
		if route.FailoverEnabled && len(route.ProviderIDs) > 1 {
			failoverID := route.ProviderIDs[1]
			failoverProviderID = &failoverID
		}
	}

	query := `
		UPDATE routes
		SET name = $2, pattern = $3, pattern_type = $4, provider_id = $5, priority = $6, 
		    active = $7, failover_provider_id = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		route.ID,
		route.Name,
		route.Pattern,
		string(route.PatternType),
		providerID,
		route.Priority,
		route.Active,
		failoverProviderID,
		route.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrRouteNotFound
	}

	return nil
}

// Delete удаляет маршрут
func (r *RouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM routes WHERE id = $1`
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrRouteNotFound
	}

	return nil
}

// List получает список маршрутов
func (r *RouteRepository) List(ctx context.Context, activeOnly bool, limit, offset int) ([]*domain.Route, int, error) {
	var query string
	var args []interface{}

	if activeOnly {
		query = `SELECT * FROM routes WHERE active = true ORDER BY priority DESC, name ASC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	} else {
		query = `SELECT * FROM routes ORDER BY priority DESC, name ASC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	var sharedRoutes []*shared.Route
	err := r.db.SelectContext(ctx, &sharedRoutes, query, args...)
	if err != nil {
		return nil, 0, err
	}

	routes := make([]*domain.Route, len(sharedRoutes))
	for i, sr := range sharedRoutes {
		routes[i] = domain.RouteFromShared(sr)
	}

	// Получаем общее количество
	var countQuery string
	if activeOnly {
		countQuery = `SELECT COUNT(*) FROM routes WHERE active = true`
	} else {
		countQuery = `SELECT COUNT(*) FROM routes`
	}

	var total int
	err = r.db.GetContext(ctx, &total, countQuery)
	if err != nil {
		return nil, 0, err
	}

	return routes, total, nil
}

// GetActiveByDestination получает активные маршруты для номера назначения
func (r *RouteRepository) GetActiveByDestination(ctx context.Context, destination string) ([]*domain.Route, error) {
	sharedRoutes, err := r.repo.GetActiveByDestination(ctx, destination)
	if err != nil {
		return nil, err
	}

	routes := make([]*domain.Route, len(sharedRoutes))
	for i, sr := range sharedRoutes {
		routes[i] = domain.RouteFromShared(sr)
		// Дополнительная проверка regex паттерна (storage может пропустить)
		if sr.PatternType == "regex" {
			matched, err := regexp.MatchString(sr.Pattern, destination)
			if err != nil || !matched {
				continue
			}
		}
	}

	// Фильтруем только те, которые действительно совпадают
	matchedRoutes := make([]*domain.Route, 0)
	for _, route := range routes {
		if route.Matches(destination) {
			matchedRoutes = append(matchedRoutes, route)
		}
	}

	return matchedRoutes, nil
}