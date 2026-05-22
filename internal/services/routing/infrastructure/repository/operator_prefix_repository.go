package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// OperatorPrefixRepository реализует domain.OperatorPrefixRepository
type OperatorPrefixRepository struct {
	db *sqlx.DB
}

// NewOperatorPrefixRepository создает новый репозиторий префиксов операторов
func NewOperatorPrefixRepository(db *sqlx.DB) *OperatorPrefixRepository {
	return &OperatorPrefixRepository{
		db: db,
	}
}

// Create создает новый префикс оператора
func (r *OperatorPrefixRepository) Create(ctx context.Context, prefix *domain.OperatorPrefix) error {
	query := `
		INSERT INTO operator_prefixes (
			id, operator_id, prefix, priority, created_at
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		prefix.ID,
		prefix.OperatorID,
		prefix.Prefix,
		prefix.Priority,
		prefix.CreatedAt,
	)

	return err
}

// Delete удаляет префикс оператора по ID
func (r *OperatorPrefixRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM operator_prefixes WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrPrefixNotFound
	}

	return nil
}

// ListByOperatorID получает список префиксов для оператора
func (r *OperatorPrefixRepository) ListByOperatorID(ctx context.Context, operatorID uuid.UUID) ([]*domain.OperatorPrefix, error) {
	query := `
		SELECT id, operator_id, prefix, priority, created_at
		FROM operator_prefixes
		WHERE operator_id = $1
		ORDER BY priority DESC, prefix ASC
	`

	rows, err := r.db.QueryContext(ctx, query, operatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefixes []*domain.OperatorPrefix
	for rows.Next() {
		var prefix domain.OperatorPrefix
		err := rows.Scan(
			&prefix.ID,
			&prefix.OperatorID,
			&prefix.Prefix,
			&prefix.Priority,
			&prefix.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, &prefix)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prefixes, nil
}

// FindByNumber находит наиболее подходящий префикс для номера телефона
func (r *OperatorPrefixRepository) FindByNumber(ctx context.Context, phoneNumber string) (*domain.OperatorPrefix, error) {
	var prefix domain.OperatorPrefix
	query := `
		SELECT id, operator_id, prefix, priority, created_at
		FROM operator_prefixes
		WHERE $1 LIKE prefix || '%'
		ORDER BY length(prefix) DESC, priority DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, phoneNumber).Scan(
		&prefix.ID,
		&prefix.OperatorID,
		&prefix.Prefix,
		&prefix.Priority,
		&prefix.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPrefixNotFound
		}
		return nil, err
	}

	return &prefix, nil
}
