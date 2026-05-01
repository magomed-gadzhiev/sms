package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// CountryRepository реализует domain.CountryRepository
type CountryRepository struct {
	db *sqlx.DB
}

// NewCountryRepository создает новый репозиторий стран
func NewCountryRepository(db *sqlx.DB) *CountryRepository {
	return &CountryRepository{
		db: db,
	}
}

// Create создает новую страну
func (r *CountryRepository) Create(ctx context.Context, country *domain.Country) error {
	query := `
		INSERT INTO countries (
			id, name, iso_code, phone_code, currency, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		country.ID,
		country.Name,
		country.ISOCode,
		country.PhoneCode,
		country.Currency,
		country.CreatedAt,
		country.UpdatedAt,
	)

	return err
}

// GetByID получает страну по ID
func (r *CountryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Country, error) {
	var country domain.Country
	query := `
		SELECT id, name, iso_code, phone_code, currency, created_at, updated_at
		FROM countries
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&country.ID,
		&country.Name,
		&country.ISOCode,
		&country.PhoneCode,
		&country.Currency,
		&country.CreatedAt,
		&country.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCountryNotFound
		}
		return nil, err
	}

	return &country, nil
}

// GetByISOCode получает страну по ISO-коду
func (r *CountryRepository) GetByISOCode(ctx context.Context, isoCode string) (*domain.Country, error) {
	var country domain.Country
	query := `
		SELECT id, name, iso_code, phone_code, currency, created_at, updated_at
		FROM countries
		WHERE iso_code = $1
	`

	err := r.db.QueryRowContext(ctx, query, isoCode).Scan(
		&country.ID,
		&country.Name,
		&country.ISOCode,
		&country.PhoneCode,
		&country.Currency,
		&country.CreatedAt,
		&country.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCountryNotFound
		}
		return nil, err
	}

	return &country, nil
}

// Update обновляет страну
func (r *CountryRepository) Update(ctx context.Context, country *domain.Country) error {
	query := `
		UPDATE countries
		SET name = $1, iso_code = $2, phone_code = $3, currency = $4, updated_at = $5
		WHERE id = $6
	`

	result, err := r.db.ExecContext(ctx, query,
		country.Name,
		country.ISOCode,
		country.PhoneCode,
		country.Currency,
		country.UpdatedAt,
		country.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrCountryNotFound
	}

	return nil
}

// List получает список стран с пагинацией
func (r *CountryRepository) List(ctx context.Context, limit, offset int) ([]*domain.Country, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM countries`
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, name, iso_code, phone_code, currency, created_at, updated_at
		FROM countries
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var countries []*domain.Country
	for rows.Next() {
		var country domain.Country
		err := rows.Scan(
			&country.ID,
			&country.Name,
			&country.ISOCode,
			&country.PhoneCode,
			&country.Currency,
			&country.CreatedAt,
			&country.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		countries = append(countries, &country)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return countries, total, nil
}
