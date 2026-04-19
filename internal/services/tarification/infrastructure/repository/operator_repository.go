// operator_repository.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// OperatorRepository — тонкий reader для operators + countries JOIN.
// operators и countries — shared-reference-data; tarification читает их
// напрямую, чтобы не вводить gRPC-зависимость для read-only lookup'а.
type OperatorRepository struct {
	db *sqlx.DB
}

// NewOperatorRepository создаёт репозиторий с готовым sqlx-пулом.
func NewOperatorRepository(db *sqlx.DB) *OperatorRepository {
	return &OperatorRepository{db: db}
}

const getOperatorMetaSQL = `
  SELECT o.code, COALESCE(c.currency, '')
  FROM operators o
  LEFT JOIN countries c ON c.id = o.country_id
  WHERE o.id = $1
`

// GetMetaByID возвращает Code + Currency для заданного UUID.
// LEFT JOIN + COALESCE гарантирует, что оператор без country или с
// country.currency=NULL вернётся с Currency="" — невыход в ошибку, чтобы
// вызывающий мог отдельно решить fallback-стратегию.
// Возвращает domain.ErrOperatorNotFound, если оператор вовсе не найден.
func (r *OperatorRepository) GetMetaByID(ctx context.Context, id uuid.UUID) (application.OperatorMeta, error) {
	var m application.OperatorMeta
	err := r.db.QueryRowxContext(ctx, getOperatorMetaSQL, id).Scan(&m.Code, &m.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		return application.OperatorMeta{}, domain.ErrOperatorNotFound
	}
	return m, err
}
