package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResellerRouteSet — набор маршрутов агрегатора.
type ResellerRouteSet struct {
	ID         uuid.UUID
	ResellerID uuid.UUID
	Name       string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ResellerRouteSetRepository предоставляет CRUD-операции над reseller_route_sets.
type ResellerRouteSetRepository struct {
	pool *pgxpool.Pool
}

// NewResellerRouteSetRepository создаёт репозиторий для работы с route-set'ами агрегатора.
func NewResellerRouteSetRepository(pool *pgxpool.Pool) *ResellerRouteSetRepository {
	return &ResellerRouteSetRepository{pool: pool}
}

// Create создаёт новый route-set для агрегатора.
func (r *ResellerRouteSetRepository) Create(ctx context.Context, resellerID uuid.UUID, name string, isDefault bool) (*ResellerRouteSet, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO reseller_route_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, name, isDefault)
	out := &ResellerRouteSet{ResellerID: resellerID, Name: name, IsDefault: isDefault}
	if err := row.Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return out, nil
}

// GetByID возвращает route-set по первичному ключу. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerRouteSetRepository) GetByID(ctx context.Context, id uuid.UUID) (*ResellerRouteSet, error) {
	var s ResellerRouteSet
	err := r.pool.QueryRow(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_route_sets WHERE id = $1`, id,
	).Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

// ListByReseller возвращает все route-set'ы заданного агрегатора, отсортированные по дате создания (DESC).
func (r *ResellerRouteSetRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]ResellerRouteSet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_route_sets WHERE reseller_id = $1
		 ORDER BY created_at DESC`, resellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResellerRouteSet
	for rows.Next() {
		var s ResellerRouteSet
		if err := rows.Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Update обновляет name и is_default записи. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerRouteSetRepository) Update(ctx context.Context, id uuid.UUID, name string, isDefault bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reseller_route_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		name, isDefault, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет route-set по ID. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerRouteSetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_route_sets WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
