package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPermission_String(t *testing.T) {
	t.Run("returns resource:action format", func(t *testing.T) {
		perm := &Permission{
			ID:       uuid.New(),
			Resource: "messages",
			Action:   "send",
		}
		assert.Equal(t, "messages:send", perm.String())
	})
}

func TestPermission_Equals(t *testing.T) {
	t.Run("returns true for same resource and action", func(t *testing.T) {
		p1 := &Permission{Resource: "messages", Action: "read"}
		p2 := &Permission{Resource: "messages", Action: "read"}
		assert.True(t, p1.Equals(p2))
	})

	t.Run("returns false for different action", func(t *testing.T) {
		p1 := &Permission{Resource: "messages", Action: "read"}
		p2 := &Permission{Resource: "messages", Action: "write"}
		assert.False(t, p1.Equals(p2))
	})

	t.Run("returns false for different resource", func(t *testing.T) {
		p1 := &Permission{Resource: "messages", Action: "read"}
		p2 := &Permission{Resource: "users", Action: "read"}
		assert.False(t, p1.Equals(p2))
	})

	t.Run("ignores ID when comparing", func(t *testing.T) {
		p1 := &Permission{ID: uuid.New(), Resource: "messages", Action: "read"}
		p2 := &Permission{ID: uuid.New(), Resource: "messages", Action: "read"}
		assert.True(t, p1.Equals(p2))
	})
}
