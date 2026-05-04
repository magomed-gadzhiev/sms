package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResellerProviderSet — шаблон каталога провайдеров для агрегатора.
type ResellerProviderSet struct {
	ID         uuid.UUID
	ResellerID uuid.UUID
	Name       string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ResellerProviderSetRepository предоставляет CRUD-операции над reseller_provider_sets.
type ResellerProviderSetRepository struct {
	pool *pgxpool.Pool
}

// NewResellerProviderSetRepository создаёт репозиторий для работы с provider-set'ами агрегатора.
func NewResellerProviderSetRepository(pool *pgxpool.Pool) *ResellerProviderSetRepository {
	return &ResellerProviderSetRepository{pool: pool}
}

// Create создаёт новый provider-set для агрегатора.
func (r *ResellerProviderSetRepository) Create(ctx context.Context, resellerID uuid.UUID, name string, isDefault bool) (*ResellerProviderSet, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO reseller_provider_sets (reseller_id, name, is_default)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		resellerID, name, isDefault)
	out := &ResellerProviderSet{ResellerID: resellerID, Name: name, IsDefault: isDefault}
	if err := row.Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return out, nil
}

// GetByID возвращает provider-set по первичному ключу. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerProviderSetRepository) GetByID(ctx context.Context, id uuid.UUID) (*ResellerProviderSet, error) {
	var s ResellerProviderSet
	err := r.pool.QueryRow(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_provider_sets WHERE id = $1`, id,
	).Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return &s, err
}

// ListByReseller возвращает все provider-set'ы заданного агрегатора, отсортированные по дате создания (DESC).
func (r *ResellerProviderSetRepository) ListByReseller(ctx context.Context, resellerID uuid.UUID) ([]ResellerProviderSet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, reseller_id, name, is_default, created_at, updated_at
		 FROM reseller_provider_sets WHERE reseller_id = $1
		 ORDER BY created_at DESC`, resellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResellerProviderSet
	for rows.Next() {
		var s ResellerProviderSet
		if err := rows.Scan(&s.ID, &s.ResellerID, &s.Name, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Update обновляет name и is_default записи. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerProviderSetRepository) Update(ctx context.Context, id uuid.UUID, name string, isDefault bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reseller_provider_sets SET name=$1, is_default=$2, updated_at=now() WHERE id=$3`,
		name, isDefault, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет provider-set по ID. Возвращает ErrNotFound, если запись отсутствует.
func (r *ResellerProviderSetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reseller_provider_sets WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
