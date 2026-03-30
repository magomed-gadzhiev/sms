package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterBuilder_Empty(t *testing.T) {
	fb := NewFilterBuilder()
	assert.Equal(t, "", fb.WhereClause())
	assert.Empty(t, fb.Args())
	assert.Equal(t, 1, fb.NextParam())
}

func TestFilterBuilder_SingleCondition(t *testing.T) {
	fb := NewFilterBuilder()
	fb.Add("status = $%d", "active")

	assert.Equal(t, " WHERE status = $1", fb.WhereClause())
	assert.Equal(t, []interface{}{"active"}, fb.Args())
	assert.Equal(t, 2, fb.NextParam())
}

func TestFilterBuilder_MultipleConditions(t *testing.T) {
	fb := NewFilterBuilder()
	fb.Add("status = $%d", "active")
	fb.Add("client_id = $%d", "uuid-123")

	assert.Equal(t, " WHERE status = $1 AND client_id = $2", fb.WhereClause())
	assert.Equal(t, []interface{}{"active", "uuid-123"}, fb.Args())
}

func TestFilterBuilder_AddIf(t *testing.T) {
	fb := NewFilterBuilder()
	fb.AddIf(true, "status = $%d", "active")
	fb.AddIf(false, "client_id = $%d", "uuid-123") // skipped

	assert.Equal(t, " WHERE status = $1", fb.WhereClause())
	assert.Len(t, fb.Args(), 1)
}

func TestFilterBuilder_AddLike(t *testing.T) {
	fb := NewFilterBuilder()
	fb.AddLike("name", "test")

	assert.Equal(t, " WHERE name ILIKE $1", fb.WhereClause())
	assert.Equal(t, []interface{}{"%test%"}, fb.Args())
}

func TestFilterBuilder_AddLike_Empty(t *testing.T) {
	fb := NewFilterBuilder()
	fb.AddLike("name", "")

	assert.Equal(t, "", fb.WhereClause())
	assert.Empty(t, fb.Args())
}

func TestFilterBuilder_WithPagination(t *testing.T) {
	fb := NewFilterBuilder()
	fb.Add("status = $%d", "active")

	paginationClause, allArgs := fb.WithPagination(20, 40)

	assert.Equal(t, " LIMIT $2 OFFSET $3", paginationClause)
	assert.Equal(t, []interface{}{"active", int32(20), int32(40)}, allArgs)
}

func TestFilterBuilder_ChainedCalls(t *testing.T) {
	fb := NewFilterBuilder()
	fb.AddIf(true, "status = $%d", "active").
		AddLike("name", "test").
		Add("client_id = $%d", "uuid")

	assert.Equal(t, " WHERE status = $1 AND name ILIKE $2 AND client_id = $3", fb.WhereClause())
	assert.Len(t, fb.Args(), 3)
}
