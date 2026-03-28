package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/smpp-server/smpp-server/internal/services/template/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newService(tmplRepo application.TemplateRepository, auditRepo application.AuditRepository) *application.TemplateService {
	return application.NewTemplateService(tmplRepo, auditRepo)
}

// ─── CreateTemplate ───────────────────────────────────────────────────────────

func TestCreateTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	name := "Welcome"
	body := "Hello {{name}}, your code is {{code}}."

	created := &domain.Template{
		ID:        uuid.New(),
		ClientID:  clientID,
		Name:      name,
		Body:      body,
		Variables: []string{"name", "code"},
		Status:    domain.StatusDraft,
	}

	tmplRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Template")).Return(created, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	result, err := svc.CreateTemplate(context.Background(), clientID, name, body)
	require.NoError(t, err)
	assert.Equal(t, created, result)
	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestCreateTemplate_EmptyName(t *testing.T) {
	svc := newService(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := svc.CreateTemplate(context.Background(), uuid.New(), "", "body text")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidTemplateName))
}

func TestCreateTemplate_EmptyBody(t *testing.T) {
	svc := newService(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := svc.CreateTemplate(context.Background(), uuid.New(), "name", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidTemplateBody))
}

func TestCreateTemplate_BodyTooLong(t *testing.T) {
	svc := newService(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	body := strings.Repeat("a", domain.MaxBodyLength+1)
	_, err := svc.CreateTemplate(context.Background(), uuid.New(), "name", body)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidTemplateBody))
}

// ─── RenderTemplate ───────────────────────────────────────────────────────────

func TestRenderTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	tmplID := uuid.New()
	tmpl := &domain.Template{
		ID:        tmplID,
		ClientID:  clientID,
		Name:      "Promo",
		Body:      "Hi {{name}}, enjoy {{discount}}% off!",
		Variables: []string{"name", "discount"},
		Status:    domain.StatusApproved,
	}

	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(tmpl, nil)

	rendered, tplName, err := svc.RenderTemplate(context.Background(), tmplID, clientID, map[string]string{
		"name":     "Alice",
		"discount": "20",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hi Alice, enjoy 20% off!", rendered)
	assert.Equal(t, "Promo", tplName)
	tmplRepo.AssertExpectations(t)
}

func TestRenderTemplate_NotApproved(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	clientID := uuid.New()
	tmplID := uuid.New()
	tmpl := &domain.Template{
		ID:       tmplID,
		ClientID: clientID,
		Status:   domain.StatusDraft,
	}

	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(tmpl, nil)

	_, _, err := svc.RenderTemplate(context.Background(), tmplID, clientID, map[string]string{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotApproved))
}

func TestRenderTemplate_MissingVariables(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	clientID := uuid.New()
	tmplID := uuid.New()
	tmpl := &domain.Template{
		ID:        tmplID,
		ClientID:  clientID,
		Body:      "Hello {{name}} and {{city}}",
		Variables: []string{"name", "city"},
		Status:    domain.StatusApproved,
	}

	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(tmpl, nil)

	_, _, err := svc.RenderTemplate(context.Background(), tmplID, clientID, map[string]string{"name": "Bob"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrMissingVariables))
}

func TestRenderTemplate_VariableValueTooLong(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	clientID := uuid.New()
	tmplID := uuid.New()
	tmpl := &domain.Template{
		ID:        tmplID,
		ClientID:  clientID,
		Body:      "Hello {{name}}",
		Variables: []string{"name"},
		Status:    domain.StatusApproved,
	}

	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(tmpl, nil)

	longValue := strings.Repeat("x", domain.MaxVariableValueLength+1)
	_, _, err := svc.RenderTemplate(context.Background(), tmplID, clientID, map[string]string{"name": longValue})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrVariableValueTooLong))
}

// ─── ApproveTemplate ──────────────────────────────────────────────────────────

func TestApproveTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	tmplID := uuid.New()
	actorID := uuid.New()
	draft := &domain.Template{ID: tmplID, Status: domain.StatusDraft}
	approved := &domain.Template{ID: tmplID, Status: domain.StatusApproved}

	tmplRepo.On("GetByIDAdmin", mock.Anything, tmplID).Return(draft, nil)
	tmplRepo.On("UpdateStatus", mock.Anything, tmplID, domain.StatusApproved, "").Return(approved, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	result, err := svc.ApproveTemplate(context.Background(), tmplID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusApproved, result.Status)
	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestApproveTemplate_NonDraft(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	tmplID := uuid.New()
	rejected := &domain.Template{ID: tmplID, Status: domain.StatusRejected}

	tmplRepo.On("GetByIDAdmin", mock.Anything, tmplID).Return(rejected, nil)

	_, err := svc.ApproveTemplate(context.Background(), tmplID, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidStatus))
}

// ─── RejectTemplate ───────────────────────────────────────────────────────────

func TestRejectTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	tmplID := uuid.New()
	draft := &domain.Template{ID: tmplID, Status: domain.StatusDraft}
	rejected := &domain.Template{ID: tmplID, Status: domain.StatusRejected, RejectionReason: "spam"}

	tmplRepo.On("GetByIDAdmin", mock.Anything, tmplID).Return(draft, nil)
	tmplRepo.On("UpdateStatus", mock.Anything, tmplID, domain.StatusRejected, "spam").Return(rejected, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	result, err := svc.RejectTemplate(context.Background(), tmplID, nil, "spam")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, result.Status)
	tmplRepo.AssertExpectations(t)
}

func TestRejectTemplate_NonDraft(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	tmplID := uuid.New()
	approved := &domain.Template{ID: tmplID, Status: domain.StatusApproved}

	tmplRepo.On("GetByIDAdmin", mock.Anything, tmplID).Return(approved, nil)

	_, err := svc.RejectTemplate(context.Background(), tmplID, nil, "reason")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidStatus))
}

// ─── ListTemplates ────────────────────────────────────────────────────────────

func TestListTemplates_DefaultsForInvalidParams(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	clientID := uuid.New()
	// Expect limit clamped to 100, offset clamped to 0
	tmplRepo.On("ListByClientID", mock.Anything, clientID, "", 100, 0).
		Return([]*domain.Template{}, 0, nil)

	// limit=-1 and offset=-5 should be corrected to 100 and 0
	templates, total, err := svc.ListTemplates(context.Background(), clientID, "", -1, -5)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, templates)
	tmplRepo.AssertExpectations(t)
}

func TestListTemplates_LimitTooLarge(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	svc := newService(tmplRepo, new(mocks.MockAuditRepository))

	clientID := uuid.New()
	tmplRepo.On("ListByClientID", mock.Anything, clientID, "", 100, 0).
		Return([]*domain.Template{}, 0, nil)

	_, _, err := svc.ListTemplates(context.Background(), clientID, "", 9999, 0)
	require.NoError(t, err)
	tmplRepo.AssertExpectations(t)
}

// ─── UpdateTemplate ───────────────────────────────────────────────────────────

func TestUpdateTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	tmplID := uuid.New()
	existing := &domain.Template{
		ID: tmplID, ClientID: clientID, Name: "Old", Body: "Old body", Status: domain.StatusApproved,
	}
	newName := "New Name"
	updated := &domain.Template{
		ID: tmplID, ClientID: clientID, Name: newName, Body: "Old body", Status: domain.StatusApproved,
	}

	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(existing, nil)
	tmplRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Template")).Return(updated, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	result, err := svc.UpdateTemplate(context.Background(), tmplID, clientID, &newName, nil)
	require.NoError(t, err)
	assert.Equal(t, newName, result.Name)
	// Status should remain unchanged when only name changes
	assert.Equal(t, domain.StatusApproved, result.Status)
	tmplRepo.AssertExpectations(t)
}

func TestUpdateTemplate_BodyChangeResetsToDraft(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	tmplID := uuid.New()
	existing := &domain.Template{
		ID: tmplID, ClientID: clientID, Name: "T", Body: "Old body", Status: domain.StatusApproved,
	}
	newBody := "New body with {{var}}"

	// The service sets status to draft on the existing object before calling Update.
	tmplRepo.On("GetByID", mock.Anything, tmplID, clientID).Return(existing, nil)
	tmplRepo.On("Update", mock.Anything, mock.MatchedBy(func(t *domain.Template) bool {
		return t.Status == domain.StatusDraft && t.Body == newBody
	})).Return(&domain.Template{
		ID: tmplID, ClientID: clientID, Name: "T", Body: newBody, Status: domain.StatusDraft,
	}, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	result, err := svc.UpdateTemplate(context.Background(), tmplID, clientID, nil, &newBody)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDraft, result.Status)
	tmplRepo.AssertExpectations(t)
}

// ─── DeleteTemplate ───────────────────────────────────────────────────────────

func TestDeleteTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	tmplID := uuid.New()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)
	tmplRepo.On("Delete", mock.Anything, tmplID, clientID).Return(nil)

	err := svc.DeleteTemplate(context.Background(), tmplID, clientID)
	require.NoError(t, err)
	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(tmplRepo, auditRepo)

	clientID := uuid.New()
	tmplID := uuid.New()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)
	tmplRepo.On("Delete", mock.Anything, tmplID, clientID).Return(domain.ErrTemplateNotFound)

	err := svc.DeleteTemplate(context.Background(), tmplID, clientID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotFound))
}

// ─── GetAuditLog ──────────────────────────────────────────────────────────────

func TestGetAuditLog_DefaultsForInvalidParams(t *testing.T) {
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(new(mocks.MockTemplateRepository), auditRepo)

	tmplID := uuid.New()
	// limit=0 → 100, offset=-1 → 0
	auditRepo.On("ListByTemplateID", mock.Anything, tmplID, 100, 0).
		Return([]*domain.AuditEntry{}, 0, nil)

	entries, total, err := svc.GetAuditLog(context.Background(), tmplID, 0, -1)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, entries)
	auditRepo.AssertExpectations(t)
}

func TestGetAuditLog_LimitTooLarge(t *testing.T) {
	auditRepo := new(mocks.MockAuditRepository)
	svc := newService(new(mocks.MockTemplateRepository), auditRepo)

	tmplID := uuid.New()
	auditRepo.On("ListByTemplateID", mock.Anything, tmplID, 100, 0).
		Return([]*domain.AuditEntry{}, 0, nil)

	_, _, err := svc.GetAuditLog(context.Background(), tmplID, 5000, 0)
	require.NoError(t, err)
	auditRepo.AssertExpectations(t)
}
