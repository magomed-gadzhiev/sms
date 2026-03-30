package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/shared/database"
)

// BaseRepository содержит общую логику для всех репозиториев
type BaseRepository struct {
	DB *sqlx.DB
}

// NewBaseRepository создает BaseRepository из database.DB
func NewBaseRepository(db *database.DB) BaseRepository {
	return BaseRepository{
		DB: sqlx.NewDb(db.DB, "pgx"),
	}
}

// NewBaseRepositoryFromSqlx создает BaseRepository из *sqlx.DB
func NewBaseRepositoryFromSqlx(db *sqlx.DB) BaseRepository {
	return BaseRepository{DB: db}
}

// GetByID выполняет SELECT одной строки по ID с маппингом через StructScan.
// Возвращает notFoundErr если строка не найдена.
func GetByID[T any](repo *BaseRepository, ctx context.Context, query string, id interface{}, notFoundErr error) (*T, error) {
	var row T
	err := repo.DB.QueryRowxContext(ctx, query, id).StructScan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFoundErr
		}
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &row, nil
}

// ExecReturning выполняет INSERT/UPDATE ... RETURNING с маппингом результата.
func ExecReturning[T any](repo *BaseRepository, ctx context.Context, query string, args ...interface{}) (*T, error) {
	var row T
	err := repo.DB.QueryRowxContext(ctx, query, args...).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("exec returning failed: %w", err)
	}
	return &row, nil
}

// SelectAll выполняет SELECT нескольких строк с маппингом через StructScan.
func SelectAll[T any](repo *BaseRepository, ctx context.Context, query string, args ...interface{}) ([]T, error) {
	var rows []T
	err := repo.DB.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select failed: %w", err)
	}
	return rows, nil
}

// ExecAffecting выполняет UPDATE/DELETE и проверяет что затронута хотя бы одна строка.
// Возвращает notFoundErr если RowsAffected == 0.
func ExecAffecting(repo *BaseRepository, ctx context.Context, query string, notFoundErr error, args ...interface{}) error {
	result, err := repo.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return notFoundErr
	}
	return nil
}

// Count выполняет COUNT(*) запрос
func Count(repo *BaseRepository, ctx context.Context, query string, args ...interface{}) (int64, error) {
	var count int64
	err := repo.DB.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count failed: %w", err)
	}
	return count, nil
}

// FilterBuilder помогает строить динамические WHERE-условия
type FilterBuilder struct {
	conditions []string
	args       []interface{}
	paramIdx   int
}

// NewFilterBuilder создает новый FilterBuilder
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{paramIdx: 1}
}

// Add добавляет условие с параметром (e.g. "status = $%d")
func (fb *FilterBuilder) Add(condition string, value interface{}) *FilterBuilder {
	fb.conditions = append(fb.conditions, fmt.Sprintf(condition, fb.paramIdx))
	fb.args = append(fb.args, value)
	fb.paramIdx++
	return fb
}

// AddIf добавляет условие только если predicate == true
func (fb *FilterBuilder) AddIf(predicate bool, condition string, value interface{}) *FilterBuilder {
	if predicate {
		return fb.Add(condition, value)
	}
	return fb
}

// AddLike добавляет ILIKE условие для текстового поиска
func (fb *FilterBuilder) AddLike(column string, value string) *FilterBuilder {
	if value == "" {
		return fb
	}
	return fb.Add(column+" ILIKE $%d", "%"+value+"%")
}

// WhereClause возвращает строку WHERE (или пустую строку)
func (fb *FilterBuilder) WhereClause() string {
	if len(fb.conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(fb.conditions, " AND ")
}

// Args возвращает все значения параметров
func (fb *FilterBuilder) Args() []interface{} {
	return fb.args
}

// NextParam возвращает текущий индекс следующего параметра
func (fb *FilterBuilder) NextParam() int {
	return fb.paramIdx
}

// WithPagination добавляет LIMIT и OFFSET к запросу и аргументам
func (fb *FilterBuilder) WithPagination(limit, offset int32) (string, []interface{}) {
	clause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", fb.paramIdx, fb.paramIdx+1)
	fb.paramIdx += 2
	args := append(fb.args, limit, offset)
	return clause, args
}

// IsNotFound проверяет, является ли ошибка sql.ErrNoRows
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
