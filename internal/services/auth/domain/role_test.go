package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRole_HasPermission(t *testing.T) {
	t.Run("returns true when role has matching permission", func(t *testing.T) {
		role := &Role{
			ID:   uuid.New(),
			Name: "admin",
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
				{Resource: "messages", Action: "write"},
				{Resource: "users", Action: "read"},
			},
		}
		assert.True(t, role.HasPermission("messages", "write"))
	})

	t.Run("returns false when action does not match", func(t *testing.T) {
		role := &Role{
			ID:   uuid.New(),
			Name: "client",
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		assert.False(t, role.HasPermission("messages", "delete"))
	})

	t.Run("returns false when resource does not match", func(t *testing.T) {
		role := &Role{
			ID:   uuid.New(),
			Name: "client",
			Permissions: []Permission{
				{Resource: "messages", Action: "read"},
			},
		}
		assert.False(t, role.HasPermission("billing", "read"))
	})

	t.Run("returns false when permissions are nil", func(t *testing.T) {
		role := &Role{ID: uuid.New(), Name: "empty", Permissions: nil}
		assert.False(t, role.HasPermission("messages", "read"))
	})

	t.Run("returns false when permissions are empty", func(t *testing.T) {
		role := &Role{ID: uuid.New(), Name: "empty", Permissions: []Permission{}}
		assert.False(t, role.HasPermission("messages", "read"))
	})
}

func TestRoleName_Constants(t *testing.T) {
	t.Run("predefined role names have expected values", func(t *testing.T) {
		assert.Equal(t, RoleName("admin"), RoleAdmin)
		assert.Equal(t, RoleName("client"), RoleClient)
		assert.Equal(t, RoleName("operator"), RoleOperator)
	})
}
