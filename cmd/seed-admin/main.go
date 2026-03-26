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
	defaultPassword = "Admin123!"
	adminRoleID     = "00000000-0000-0000-0000-000000000001"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://smpp:smpp_password@localhost:5432/smpp_db?sslmode=disable"
	}

	username := envOr("ADMIN_USERNAME", defaultUsername)
	email := envOr("ADMIN_EMAIL", defaultEmail)
	password := envOr("ADMIN_PASSWORD", defaultPassword)

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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
