package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
		if err == sql.ErrNoRows {
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
		if err == sql.ErrNoRows {
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
