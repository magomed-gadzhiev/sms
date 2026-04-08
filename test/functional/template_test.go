//go:build functional

package functional_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	templateRepo "github.com/smpp-server/smpp-server/internal/services/template/infrastructure/repository"
)

func TestTemplateChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-template', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM template_audit_log WHERE template_id IN (SELECT id FROM templates WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM templates WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	tplRepo := templateRepo.NewTemplateRepository(db)
	auditRepo := templateRepo.NewAuditRepository(db)
	svc := application.NewTemplateService(tplRepo, auditRepo)

	t.Run("CreateTemplate", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Welcome", "Hello {{name}}, welcome to {{company}}!", nil, "")
		require.NoError(t, err)
		require.NotNil(t, tmpl)

		assert.Equal(t, "Welcome", tmpl.Name)
		assert.Equal(t, "Hello {{name}}, welcome to {{company}}!", tmpl.Body)
		assert.Equal(t, domain.StatusDraft, tmpl.Status)
		assert.ElementsMatch(t, []string{"name", "company"}, tmpl.Variables)
		assert.Equal(t, clientID, tmpl.ClientID)
	})

	t.Run("CreateTemplateInvalidNameFails", func(t *testing.T) {
		_, err := svc.CreateTemplate(ctx, clientID, "", "body", nil, "")
		require.Error(t, err)
	})

	t.Run("CreateTemplateEmptyBodyFails", func(t *testing.T) {
		_, err := svc.CreateTemplate(ctx, clientID, "NoBody", "", nil, "")
		require.Error(t, err)
	})

	t.Run("GetTemplate", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Getter", "Hello {{user}}", nil, "")
		require.NoError(t, err)

		fetched, err := svc.GetTemplate(ctx, tmpl.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, tmpl.ID, fetched.ID)
		assert.Equal(t, "Getter", fetched.Name)
	})

	t.Run("GetTemplateNotFoundForOtherClient", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Owned", "Body", nil, "")
		require.NoError(t, err)

		_, err = svc.GetTemplate(ctx, tmpl.ID, uuid.New())
		assert.ErrorIs(t, err, domain.ErrTemplateNotFound)
	})

	t.Run("ListTemplates", func(t *testing.T) {
		_, err := svc.CreateTemplate(ctx, clientID, "ListA", "Body A", nil, "")
		require.NoError(t, err)
		_, err = svc.CreateTemplate(ctx, clientID, "ListB", "Body B", nil, "")
		require.NoError(t, err)

		templates, total, err := svc.ListTemplates(ctx, clientID, "", 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(templates), 2)
	})

	t.Run("ListTemplatesFilterByStatus", func(t *testing.T) {
		templates, total, err := svc.ListTemplates(ctx, clientID, domain.StatusDraft, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		for _, tmpl := range templates {
			assert.Equal(t, domain.StatusDraft, tmpl.Status)
		}
	})

	t.Run("UpdateTemplate", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Original", "Old body", nil, "")
		require.NoError(t, err)

		newName := "Renamed"
		newBody := "New body with {{var}}"
		updated, err := svc.UpdateTemplate(ctx, tmpl.ID, clientID, &newName, &newBody, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", updated.Name)
		assert.Equal(t, "New body with {{var}}", updated.Body)
		assert.ElementsMatch(t, []string{"var"}, updated.Variables)
		// Body change resets status to draft.
		assert.Equal(t, domain.StatusDraft, updated.Status)
	})

	t.Run("DeleteTemplate", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "To Delete", "Body", nil, "")
		require.NoError(t, err)

		err = svc.DeleteTemplate(ctx, tmpl.ID, clientID)
		require.NoError(t, err)

		_, err = svc.GetTemplate(ctx, tmpl.ID, clientID)
		assert.ErrorIs(t, err, domain.ErrTemplateNotFound)
	})
}

func TestTemplateApprovalWorkflow(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	reviewerID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-approval', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM template_audit_log WHERE template_id IN (SELECT id FROM templates WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM templates WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	tplRepo := templateRepo.NewTemplateRepository(db)
	auditRepo := templateRepo.NewAuditRepository(db)
	svc := application.NewTemplateService(tplRepo, auditRepo)

	t.Run("SubmitForReviewApproveAndRender", func(t *testing.T) {
		// Create template.
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Approval Flow", "Hi {{name}}, code: {{code}}", nil, "")
		require.NoError(t, err)
		assert.Equal(t, domain.StatusDraft, tmpl.Status)

		// Submit for review.
		submitted, err := svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusPending, submitted.Status)

		// Cannot submit again from pending.
		_, err = svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.Error(t, err)

		// Approve the template.
		approved, err := svc.ApproveTemplate(ctx, tmpl.ID, &reviewerID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusApproved, approved.Status)

		// Render the approved template.
		rendered, name, err := svc.RenderTemplate(ctx, tmpl.ID, clientID, map[string]string{
			"name": "Alice",
			"code": "1234",
		})
		require.NoError(t, err)
		assert.Equal(t, "Hi Alice, code: 1234", rendered)
		assert.Equal(t, "Approval Flow", name)
	})

	t.Run("RenderUnapprovedTemplateFails", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Draft Only", "Hello {{user}}", nil, "")
		require.NoError(t, err)

		_, _, err = svc.RenderTemplate(ctx, tmpl.ID, clientID, map[string]string{"user": "Bob"})
		assert.ErrorIs(t, err, domain.ErrTemplateNotApproved)
	})

	t.Run("RenderWithMissingVariablesFails", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Missing Vars", "Hello {{a}} {{b}}", nil, "")
		require.NoError(t, err)

		// Submit and approve.
		_, err = svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.NoError(t, err)
		_, err = svc.ApproveTemplate(ctx, tmpl.ID, &reviewerID)
		require.NoError(t, err)

		// Provide only one of two required variables.
		_, _, err = svc.RenderTemplate(ctx, tmpl.ID, clientID, map[string]string{"a": "val"})
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrMissingVariables)
	})

	t.Run("RejectTemplate", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "To Reject", "Bad content", nil, "")
		require.NoError(t, err)

		_, err = svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.NoError(t, err)

		rejected, err := svc.RejectTemplate(ctx, tmpl.ID, &reviewerID, "Contains prohibited content")
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRejected, rejected.Status)
		assert.Equal(t, "Contains prohibited content", rejected.RejectionReason)
	})

	t.Run("RequestRevisionAndResubmit", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Revise Me", "Draft body", nil, "")
		require.NoError(t, err)

		_, err = svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.NoError(t, err)

		// Request revision.
		revised, err := svc.RequestRevision(ctx, tmpl.ID, reviewerID, "Please fix wording")
		require.NoError(t, err)
		assert.Equal(t, domain.StatusRevisionRequested, revised.Status)

		// Can resubmit from revision_requested.
		resubmitted, err := svc.SubmitForReview(ctx, tmpl.ID, clientID)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusPending, resubmitted.Status)
	})

	t.Run("ApproveNonPendingFails", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Not Pending", "Body", nil, "")
		require.NoError(t, err)

		// Try to approve a draft template.
		_, err = svc.ApproveTemplate(ctx, tmpl.ID, &reviewerID)
		require.Error(t, err)
	})
}

func TestTemplateAuditLog(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)
	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-audit', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM template_audit_log WHERE template_id IN (SELECT id FROM templates WHERE client_id = $1)`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM templates WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	tplRepo := templateRepo.NewTemplateRepository(db)
	auditRepo := templateRepo.NewAuditRepository(db)
	svc := application.NewTemplateService(tplRepo, auditRepo)

	t.Run("AuditLogRecordsActions", func(t *testing.T) {
		tmpl, err := svc.CreateTemplate(ctx, clientID, "Audited", "Hello {{user}}", nil, "")
		require.NoError(t, err)

		// Update the template body.
		newBody := "Hi {{user}}, welcome!"
		_, err = svc.UpdateTemplate(ctx, tmpl.ID, clientID, nil, &newBody, nil, nil)
		require.NoError(t, err)

		// Check audit log entries.
		entries, total, err := svc.GetAuditLog(ctx, tmpl.ID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2, "should have at least 'created' and 'updated' entries")
		assert.GreaterOrEqual(t, len(entries), 2)

		// Verify the first entry is 'created'.
		actions := make([]string, len(entries))
		for i, e := range entries {
			actions[i] = e.Action
		}
		assert.Contains(t, actions, "created")
		assert.Contains(t, actions, "updated")
	})
}
