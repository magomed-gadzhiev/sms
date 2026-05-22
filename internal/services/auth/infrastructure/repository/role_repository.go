package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var ErrRoleNotFound = errors.New("role not found")

// RoleRepository предоставляет методы для работы с ролями
type RoleRepository struct {
	db *sqlx.DB
}

// NewRoleRepository создает новый репозиторий ролей
func NewRoleRepository(db *database.DB) *RoleRepository {
	return &RoleRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// GetByID получает роль по ID
func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM roles WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	return &role, nil
}

// GetByName получает роль по имени
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM roles WHERE name = $1
	`

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	return &role, nil
}

// GetByIDWithPermissions получает роль по ID вместе с правами
func (r *RoleRepository) GetByIDWithPermissions(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	role, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Загружаем права
	permissions, err := r.getPermissionsByRoleID(ctx, id)
	if err != nil {
		return nil, err
	}
	role.Permissions = permissions

	return role, nil
}

// List возвращает список ролей с количеством пользователей и пагинацией
func (r *RoleRepository) List(ctx context.Context, limit, offset int32) ([]*domain.Role, int32, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Общее количество ролей
	var total int32
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Получаем роли с подсчётом пользователей
	query := `
		SELECT r.id, r.name, r.description, r.created_at, r.updated_at,
			COALESCE(uc.cnt, 0) AS user_count
		FROM roles r
		LEFT JOIN (SELECT role_id, COUNT(*) AS cnt FROM users GROUP BY role_id) uc ON uc.role_id = r.id
		ORDER BY r.name
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		var role domain.Role
		err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt,
			&role.UserCount,
		)
		if err != nil {
			return nil, 0, err
		}

		// Загружаем права для каждой роли
		perms, err := r.getPermissionsByRoleID(ctx, role.ID)
		if err != nil {
			log.Warn().Err(err).Str("role_id", role.ID.String()).Msg("не удалось загрузить права роли")
		}
		role.Permissions = perms

		roles = append(roles, &role)
	}

	return roles, total, nil
}

// Create создает новую роль с привязанными правами
func (r *RoleRepository) Create(ctx context.Context, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	now := time.Now()
	role := &domain.Role{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Вставляем роль
	_, err = tx.ExecContext(ctx,
		`INSERT INTO roles (id, name, description, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		role.ID, role.Name, role.Description, role.CreatedAt, role.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Привязываем права
	for _, permID := range permissionIDs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
			role.ID, permID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Загружаем права
	perms, err := r.getPermissionsByRoleID(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	role.Permissions = perms

	return role, nil
}

// Update обновляет роль и заменяет привязанные права
func (r *RoleRepository) Update(ctx context.Context, roleID uuid.UUID, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()

	// Обновляем роль
	result, err := tx.ExecContext(ctx,
		`UPDATE roles SET name = $2, description = $3, updated_at = $4 WHERE id = $1`,
		roleID, name, description, now,
	)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrRoleNotFound
	}

	// Удаляем старые права
	_, err = tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}

	// Привязываем новые права
	for _, permID := range permissionIDs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
			roleID, permID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Возвращаем обновлённую роль
	role, err := r.GetByIDWithPermissions(ctx, roleID)
	if err != nil {
		return nil, err
	}

	return role, nil
}

// Delete удаляет роль и её привязки к правам
func (r *RoleRepository) Delete(ctx context.Context, roleID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Удаляем привязки прав
	_, err = tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return err
	}

	// Удаляем роль
	result, err := tx.ExecContext(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRoleNotFound
	}

	return tx.Commit()
}

// ListAllPermissions возвращает все доступные права
func (r *RoleRepository) ListAllPermissions(ctx context.Context) ([]domain.Permission, error) {
	var permissions []domain.Permission
	query := `
		SELECT id, resource, action, description, created_at
		FROM permissions
		ORDER BY resource, action
	`

	err := r.db.SelectContext(ctx, &permissions, query)
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

// GetUserCount возвращает количество пользователей с указанной ролью
func (r *RoleRepository) GetUserCount(ctx context.Context, roleID uuid.UUID) (int32, error) {
	var count int32
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role_id = $1`, roleID).Scan(&count)
	return count, err
}

// getPermissionsByRoleID получает права роли по ID роли
func (r *RoleRepository) getPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	var permissions []domain.Permission
	query := `
		SELECT p.id, p.resource, p.action, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`

	err := r.db.SelectContext(ctx, &permissions, query, roleID)
	if err != nil {
		return nil, err
	}

	return permissions, nil
}
