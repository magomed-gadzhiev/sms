package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUser_IsActive(t *testing.T) {
	t.Run("returns true when user is active", func(t *testing.T) {
		user := &User{ID: uuid.New(), Active: true}
		assert.True(t, user.IsActive())
	})

	t.Run("returns false when user is inactive", func(t *testing.T) {
		user := &User{ID: uuid.New(), Active: false}
		assert.False(t, user.IsActive())
	})
}

func TestUser_HasPermission(t *testing.T) {
	t.Run("returns true when user has matching permission", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
				{Resource: "messages", Action: "write"},
			},
		}
		assert.True(t, user.HasPermission("messages", "read"))
	})

	t.Run("returns false when action does not match", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		assert.False(t, user.HasPermission("messages", "delete"))
	})

	t.Run("returns false when resource does not match", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		assert.False(t, user.HasPermission("users", "read"))
	})

	t.Run("returns false when permissions are nil", func(t *testing.T) {
		user := &User{ID: uuid.New(), Permissions: nil}
		assert.False(t, user.HasPermission("messages", "read"))
	})

	t.Run("returns false when permissions are empty", func(t *testing.T) {
		user := &User{ID: uuid.New(), Permissions: []Permission{}}
		assert.False(t, user.HasPermission("messages", "read"))
	})
}

func TestUser_HasAnyPermission(t *testing.T) {
	t.Run("returns true when at least one permission matches", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		result := user.HasAnyPermission(
			Permission{Resource: "users", Action: "write"},
			Permission{Resource: "messages", Action: "read"},
		)
		assert.True(t, result)
	})

	t.Run("returns false when no permissions match", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		result := user.HasAnyPermission(
			Permission{Resource: "users", Action: "write"},
			Permission{Resource: "billing", Action: "read"},
		)
		assert.False(t, result)
	})

	t.Run("returns false when user permissions are nil", func(t *testing.T) {
		user := &User{ID: uuid.New(), Permissions: nil}
		result := user.HasAnyPermission(
			Permission{Resource: "messages", Action: "read"},
		)
		assert.False(t, result)
	})

	t.Run("returns false when no required permissions provided", func(t *testing.T) {
		user := &User{
			ID: uuid.New(),
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		result := user.HasAnyPermission()
		assert.False(t, result)
	})
}
