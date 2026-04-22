//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/smpp-server/smpp-server/internal/services/template/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestDB returns a *sqlx.DB connected to the integration test database.
// Set TEST_DATABASE_URL to enable; otherwise the calling test is skipped.
func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}
	db, err := sqlx.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedTemplateID inserts a minimal template row and returns its UUID.
func seedTemplateID(t *testing.T, _ *sqlx.DB) uuid.UUID {
	t.Helper()
	t.Skip("seedTemplateID helper not implemented — wire fixture to enable")
	return uuid.UUID{}
}

// seedSenderNameID inserts a minimal sender_name row and returns its UUID.
func seedSenderNameID(t *testing.T, _ *sqlx.DB) uuid.UUID {
	t.Helper()
	t.Skip("seedSenderNameID helper not implemented — wire fixture to enable")
	return uuid.UUID{}
}

// seedOperatorID inserts a minimal operator row and returns its UUID.
func seedOperatorID(t *testing.T, _ *sqlx.DB) uuid.UUID {
	t.Helper()
	t.Skip("seedOperatorID helper not implemented — wire fixture to enable")
	return uuid.UUID{}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOperatorBindingRepo_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewOperatorBindingRepo(db)
	ctx := context.Background()

	templateID := seedTemplateID(t, db)
	senderNameID := seedSenderNameID(t, db)
	operatorID := seedOperatorID(t, db)

	b := domain.NewOperatorTemplateBinding(templateID, senderNameID, operatorID)
	require.NoError(t, repo.Create(ctx, b))

	got, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)

	assert.Equal(t, b.ID, got.ID)
	assert.Equal(t, b.TemplateID, got.TemplateID)
	assert.Equal(t, b.SenderNameID, got.SenderNameID)
	assert.Equal(t, b.OperatorID, got.OperatorID)
	assert.Equal(t, domain.OperatorBindingStatusPending, got.Status)
	assert.Empty(t, got.RejectionReason)
	assert.Nil(t, got.ReviewedBy)
	assert.Nil(t, got.ReviewedAt)
}

func TestOperatorBindingRepo_DuplicateFails(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewOperatorBindingRepo(db)
	ctx := context.Background()

	templateID := seedTemplateID(t, db)
	senderNameID := seedSenderNameID(t, db)
	operatorID := seedOperatorID(t, db)

	b1 := domain.NewOperatorTemplateBinding(templateID, senderNameID, operatorID)
	require.NoError(t, repo.Create(ctx, b1))

	// Same (template_id, operator_id) — unique constraint must fire.
	b2 := domain.NewOperatorTemplateBinding(templateID, senderNameID, operatorID)
	err := repo.Create(ctx, b2)
	assert.ErrorIs(t, err, domain.ErrDuplicateOperatorBinding)
}

func TestOperatorBindingRepo_ListPendingByOperator(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewOperatorBindingRepo(db)
	ctx := context.Background()

	operatorID := seedOperatorID(t, db)

	// Two pending bindings for this operator (different templates).
	tmpl1 := seedTemplateID(t, db)
	tmpl2 := seedTemplateID(t, db)
	sn := seedSenderNameID(t, db)

	b1 := domain.NewOperatorTemplateBinding(tmpl1, sn, operatorID)
	b1.CreatedAt = time.Now().UTC().Add(-2 * time.Second)
	b1.UpdatedAt = b1.CreatedAt
	require.NoError(t, repo.Create(ctx, b1))

	b2 := domain.NewOperatorTemplateBinding(tmpl2, sn, operatorID)
	require.NoError(t, repo.Create(ctx, b2))

	// One binding for a different operator — must NOT appear in the list.
	otherOp := seedOperatorID(t, db)
	tmpl3 := seedTemplateID(t, db)
	bOther := domain.NewOperatorTemplateBinding(tmpl3, sn, otherOp)
	require.NoError(t, repo.Create(ctx, bOther))

	list, err := repo.ListPendingByOperator(ctx, operatorID, 50, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Must be ordered by created_at ASC.
	assert.Equal(t, b1.ID, list[0].ID)
	assert.Equal(t, b2.ID, list[1].ID)
}

func TestOperatorBindingRepo_UpdateStatus(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewOperatorBindingRepo(db)
	ctx := context.Background()

	templateID := seedTemplateID(t, db)
	senderNameID := seedSenderNameID(t, db)
	operatorID := seedOperatorID(t, db)

	b := domain.NewOperatorTemplateBinding(templateID, senderNameID, operatorID)
	require.NoError(t, repo.Create(ctx, b))

	reviewerID := uuid.New()
	require.NoError(t, repo.UpdateStatus(ctx, b.ID, domain.OperatorBindingStatusApproved, "", reviewerID))

	got, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.OperatorBindingStatusApproved, got.Status)
	require.NotNil(t, got.ReviewedBy)
	assert.Equal(t, reviewerID, *got.ReviewedBy)
	assert.NotNil(t, got.ReviewedAt)
	assert.Empty(t, got.RejectionReason)
}

func TestOperatorBindingRepo_UpdateStatus_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewOperatorBindingRepo(db)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, uuid.New(), domain.OperatorBindingStatusApproved, "", uuid.New())
	assert.ErrorIs(t, err, domain.ErrOperatorBindingNotFound)
}
