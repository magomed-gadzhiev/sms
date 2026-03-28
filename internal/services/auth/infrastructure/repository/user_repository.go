package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/smpp-server/smpp-server/internal/shared/database"
)

var ErrUserNotFound = errors.New("user not found")

// UserRepository предоставляет методы для работы с пользователями
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository создает новый репозиторий пользователей
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{
		db: sqlx.NewDb(db.DB, "pgx"),
	}
}

// Create создает нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			id, username, email, password_hash, role_id, active, client_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Username, user.Email, user.PasswordHash,
		user.RoleID, user.Active, user.ClientID, user.CreatedAt, user.UpdatedAt,
	)

	if err != nil {
		log.Error().Err(err).Msg("ошибка создания пользователя")
		return err
	}

	return nil
}

// GetByID получает пользователя по ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	query := `
		SELECT id, username, email, password_hash, role_id, active, client_id, created_at, updated_at
		FROM users WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.RoleID, &user.Active, &user.ClientID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByUsername получает пользователя по имени пользователя
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	query := `
		SELECT id, username, email, password_hash, role_id, active, client_id, created_at, updated_at
		FROM users WHERE username = $1
	`

	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.RoleID, &user.Active, &user.ClientID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByEmail получает пользователя по email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	query := `
		SELECT id, username, email, password_hash, role_id, active, client_id, created_at, updated_at
		FROM users WHERE email = $1
	`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.RoleID, &user.Active, &user.ClientID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByIDWithRole получает пользователя по ID вместе с ролью и правами
func (r *UserRepository) GetByIDWithRole(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Загружаем роль
	role, err := r.getRoleByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	user.Role = role

	// Загружаем права роли
	permissions, err := r.getPermissionsByRoleID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	user.Permissions = permissions

	return user, nil
}

// Update обновляет пользователя
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users SET
			username = $2, email = $3, password_hash = $4,
			role_id = $5, active = $6, updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		user.ID, user.Username, user.Email, user.PasswordHash,
		user.RoleID, user.Active, user.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// getRoleByID получает роль по ID
func (r *UserRepository) getRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
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
			return nil, errors.New("role not found")
		}
		return nil, err
	}

	return &role, nil
}

// List возвращает список пользователей с фильтрацией и пагинацией
func (r *UserRepository) List(ctx context.Context, search string, roleID string, activeOnly bool, limit, offset int32) ([]*domain.User, int32, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Строим WHERE условия
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, "(u.username ILIKE $"+itoa(argIdx)+" OR u.email ILIKE $"+itoa(argIdx)+")")
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if roleID != "" {
		conditions = append(conditions, "u.role_id = $"+itoa(argIdx))
		args = append(args, roleID)
		argIdx++
	}

	if activeOnly {
		conditions = append(conditions, "u.active = true")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Подсчет общего количества
	countQuery := "SELECT COUNT(*) FROM users u " + whereClause
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Получаем пользователей с ролями
	query := `
		SELECT u.id, u.username, u.email, u.password_hash, u.role_id, u.active, u.client_id, u.created_at, u.updated_at,
			r.id, r.name, r.description, r.created_at, r.updated_at
		FROM users u
		LEFT JOIN roles r ON u.role_id = r.id
		` + whereClause + `
		ORDER BY u.created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var user domain.User
		var role domain.Role
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash,
			&user.RoleID, &user.Active, &user.ClientID, &user.CreatedAt, &user.UpdatedAt,
			&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		user.Role = &role
		users = append(users, &user)
	}

	return users, total, nil
}

// Deactivate деактивирует пользователя
func (r *UserRepository) Deactivate(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET active = false, updated_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ResetTOTP удаляет TOTP секрет пользователя
func (r *UserRepository) ResetTOTP(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM totp_secrets WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// itoa конвертирует int в строку для построения SQL запросов
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// getPermissionsByRoleID получает права роли по ID роли
func (r *UserRepository) getPermissionsByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
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
