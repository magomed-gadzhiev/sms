package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	authinfra "github.com/smpp-server/smpp-server/internal/services/auth/infrastructure"
)

const (
	defaultUsername = "admin"
	defaultEmail    = "admin@example.com"
	adminRoleID     = "00000000-0000-0000-0000-000000000001"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://smpp:change-me@localhost:5432/smpp_db?sslmode=disable"
	}

	username := envOr("ADMIN_USERNAME", defaultUsername)
	email := envOr("ADMIN_EMAIL", defaultEmail)
	password := os.Getenv("ADMIN_PASSWORD")
	if err := validateAdminPassword(password); err != nil {
		log.Fatalf("seed-admin: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Check if user already exists
	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)", username, email).Scan(&exists)
	if err != nil {
		log.Fatalf("failed to check existing user: %v", err)
	}
	if exists {
		fmt.Printf("User '%s' (%s) already exists, skipping.\n", username, email)
		return
	}

	hash, err := authinfra.HashPassword(password)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	roleID, _ := uuid.Parse(adminRoleID)
	now := time.Now()

	_, err = conn.Exec(ctx,
		`INSERT INTO users (id, username, email, password_hash, role_id, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		uuid.New(), username, email, hash, roleID, true, now, now,
	)
	if err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}

	fmt.Printf("Admin user created successfully:\n  Username: %s\n  Email:    %s\n  Password: %s\n", username, email, password)
}

// validateAdminPassword проверяет, что пароль задан и достаточно длинный.
// Возвращает ошибку с описанием проблемы — main печатает и exit'ит.
//
// Минимум 16 символов: не bcrypt-стойкость, а защита от очевидных слабых
// паролей (example local password, password, 12345678). Для bcrypt cost=10 16 случайных
// символов = ~96 бит энтропии, достаточно.
func validateAdminPassword(pw string) error {
	if pw == "" {
		return fmt.Errorf("ADMIN_PASSWORD env var is required (no default in production)")
	}
	if len(pw) < 16 {
		return fmt.Errorf("ADMIN_PASSWORD must be at least 16 characters (got %d)", len(pw))
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
