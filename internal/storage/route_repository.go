package storage

import (
	"context"
	"database/sql"
	"regexp"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RouteRepository предоставляет методы для работы с маршрутами
type RouteRepository struct {
	db *sqlx.DB
}

// NewRouteRepository создает новый репозиторий маршрутов
func NewRouteRepository(db *DB) *RouteRepository {
	return &RouteRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// GetByID получает маршрут по ID
func (r *RouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
	var route shared.Route
	query := `
		SELECT * FROM routes WHERE id = $1
	`

	err := r.db.GetContext(ctx, &route, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &route, nil
}

// GetActiveByDestination получает активные маршруты для номера назначения, отсортированные по приоритету
func (r *RouteRepository) GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error) {
	var routes []*shared.Route
	query := `
		SELECT * FROM routes
		WHERE active = true
		ORDER BY priority DESC
	`

	allRoutes, err := r.getAllRoutes(ctx, query)
	if err != nil {
		return nil, err
	}

	// Фильтруем маршруты по паттерну
	for _, route := range allRoutes {
		if r.matchesPattern(route, destination) {
			routes = append(routes, route)
		}
	}

	return routes, nil
}

// GetAllActive получает все активные маршруты, отсортированные по приоритету
func (r *RouteRepository) GetAllActive(ctx context.Context) ([]*shared.Route, error) {
	query := `
		SELECT * FROM routes
		WHERE active = true
		ORDER BY priority DESC, name ASC
	`
	return r.getAllRoutes(ctx, query)
}

// getAllRoutes получает все маршруты по запросу
func (r *RouteRepository) getAllRoutes(ctx context.Context, query string) ([]*shared.Route, error) {
	var routes []*shared.Route
	err := r.db.SelectContext(ctx, &routes, query)
	if err != nil {
		return nil, err
	}
	return routes, nil
}

// matchesPattern проверяет, соответствует ли номер паттерну маршрута
func (r *RouteRepository) matchesPattern(route *shared.Route, destination string) bool {
	switch route.PatternType {
	case "prefix":
		// Проверяем префикс
		if len(destination) >= len(route.Pattern) {
			return destination[:len(route.Pattern)] == route.Pattern
		}
		return false
	case "exact":
		// Точное совпадение
		return destination == route.Pattern
	case "regex":
		// Проверяем regex паттерн
		matched, err := regexp.MatchString(route.Pattern, destination)
		if err != nil {
			// Некорректный regex - логируем и возвращаем false
			return false
		}
		return matched
	default:
		return false
	}
}
