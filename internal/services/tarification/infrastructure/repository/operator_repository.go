// operator_repository.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// OperatorRepository — тонкий reader для operators.code.
// operators — shared-reference-data (routing-service тоже её использует);
// tarification обращается напрямую, чтобы не вводить gRPC-зависимость для
// одного read-only lookup'а.
type OperatorRepository struct {
	db *sqlx.DB
}

// NewOperatorRepository создаёт репозиторий с готовым sqlx-пулом.
func NewOperatorRepository(db *sqlx.DB) *OperatorRepository {
	return &OperatorRepository{db: db}
}

// GetCodeByID возвращает operators.code для заданного UUID.
// Возвращает domain.ErrOperatorNotFound, если строка не найдена.
func (r *OperatorRepository) GetCodeByID(ctx context.Context, id uuid.UUID) (string, error) {
	var code string
	err := r.db.GetContext(ctx, &code, `SELECT code FROM operators WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrOperatorNotFound
	}
	return code, err
}
