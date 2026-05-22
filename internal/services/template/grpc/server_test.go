package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/services/template/application"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	grpcserver "github.com/smpp-server/smpp-server/internal/services/template/grpc"
	"github.com/smpp-server/smpp-server/internal/services/template/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// helpers

func newTemplateServer(tmplRepo application.TemplateRepository, auditRepo application.AuditRepository) *grpcserver.Server {
	svc := application.NewTemplateService(tmplRepo, auditRepo)
	return grpcserver.NewServer(svc)
}

func validClientID() string { return uuid.New().String() }
func validID() string       { return uuid.New().String() }

func makeTemplate(id, clientID uuid.UUID, status string) *domain.Template {
	return &domain.Template{
		ID:        id,
		ClientID:  clientID,
		Name:      "Test Template",
		Body:      "Hello {{name}}",
		Variables: []string{"name"},
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ─── CreateTemplate ───────────────────────────────────────────────────────────

func TestCreateTemplate_MissingClientID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: "",
		Name:     "name",
		Body:     "body",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "client_id")
}

func TestCreateTemplate_MissingName(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: validClientID(),
		Name:     "",
		Body:     "body",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "name")
}

func TestCreateTemplate_MissingBody(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: validClientID(),
		Name:     "name",
		Body:     "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "body")
}

func TestCreateTemplate_InvalidClientIDFormat(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: "not-a-uuid",
		Name:     "name",
		Body:     "body",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "client_id")
}

func TestCreateTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	clientID := uuid.New()
	id := uuid.New()
	tmpl := makeTemplate(id, clientID, domain.StatusDraft)

	tmplRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Template")).Return(tmpl, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	resp, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: clientID.String(),
		Name:     tmpl.Name,
		Body:     tmpl.Body,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Template)
	assert.Equal(t, id.String(), resp.Template.Id)
	assert.Equal(t, clientID.String(), resp.Template.ClientId)
	assert.Equal(t, domain.StatusDraft, resp.Template.Status)

	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestCreateTemplate_DomainError_MapsToAlreadyExists(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	tmplRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Template")).
		Return(nil, domain.ErrDuplicateTemplateName)

	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: validClientID(),
		Name:     "dupe",
		Body:     "body",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

// ─── GetTemplate ──────────────────────────────────────────────────────────────

func TestGetTemplate_MissingID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.GetTemplate(context.Background(), &templatev1.GetTemplateRequest{
		Id:       "",
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetTemplate_NotFound(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	tmplRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(nil, domain.ErrTemplateNotFound)

	_, err := srv.GetTemplate(context.Background(), &templatev1.GetTemplateRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())

	tmplRepo.AssertExpectations(t)
}

func TestGetTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	clientID := uuid.New()
	id := uuid.New()
	tmpl := makeTemplate(id, clientID, domain.StatusApproved)

	tmplRepo.On("GetByID", mock.Anything, id, clientID).Return(tmpl, nil)

	resp, err := srv.GetTemplate(context.Background(), &templatev1.GetTemplateRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Template)
	assert.Equal(t, id.String(), resp.Template.Id)
	assert.Equal(t, domain.StatusApproved, resp.Template.Status)

	tmplRepo.AssertExpectations(t)
}

// ─── UpdateTemplate ───────────────────────────────────────────────────────────

func TestUpdateTemplate_MissingID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.UpdateTemplate(context.Background(), &templatev1.UpdateTemplateRequest{
		Id:       "",
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUpdateTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	clientID := uuid.New()
	id := uuid.New()
	existing := makeTemplate(id, clientID, domain.StatusDraft)
	newName := "Updated Name"
	updated := makeTemplate(id, clientID, domain.StatusDraft)
	updated.Name = newName

	tmplRepo.On("GetByID", mock.Anything, id, clientID).Return(existing, nil)
	tmplRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Template")).Return(updated, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	resp, err := srv.UpdateTemplate(context.Background(), &templatev1.UpdateTemplateRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
		Name:     &newName,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Template)
	assert.Equal(t, newName, resp.Template.Name)

	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestUpdateTemplate_NotFound(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	tmplRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(nil, domain.ErrTemplateNotFound)

	_, err := srv.UpdateTemplate(context.Background(), &templatev1.UpdateTemplateRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ─── DeleteTemplate ───────────────────────────────────────────────────────────

func TestDeleteTemplate_MissingClientID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.DeleteTemplate(context.Background(), &templatev1.DeleteTemplateRequest{
		Id:       validID(),
		ClientId: "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestDeleteTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	clientID := uuid.New()
	id := uuid.New()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)
	tmplRepo.On("Delete", mock.Anything, id, clientID).Return(nil)

	resp, err := srv.DeleteTemplate(context.Background(), &templatev1.DeleteTemplateRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)
	tmplRepo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(domain.ErrTemplateNotFound)

	_, err := srv.DeleteTemplate(context.Background(), &templatev1.DeleteTemplateRequest{
		Id:       validID(),
		ClientId: validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ─── ApproveTemplate ─────────────────────────────────────────────────────────

func TestApproveTemplate_MissingID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.ApproveTemplate(context.Background(), &templatev1.ApproveTemplateRequest{Id: ""})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestApproveTemplate_InvalidIDFormat(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.ApproveTemplate(context.Background(), &templatev1.ApproveTemplateRequest{Id: "bad-uuid"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestApproveTemplate_InvalidActorIDFormat(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.ApproveTemplate(context.Background(), &templatev1.ApproveTemplateRequest{
		Id:      validID(),
		ActorId: "bad-actor",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestApproveTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	pending := makeTemplate(id, clientID, domain.StatusPending)
	approved := makeTemplate(id, clientID, domain.StatusApproved)

	tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(pending, nil)
	tmplRepo.On("UpdateStatus", mock.Anything, id, domain.StatusApproved, "").Return(approved, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	resp, err := srv.ApproveTemplate(context.Background(), &templatev1.ApproveTemplateRequest{
		Id: id.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Template)
	assert.Equal(t, domain.StatusApproved, resp.Template.Status)

	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestApproveTemplate_AlreadyApproved_FailedPrecondition(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	alreadyApproved := makeTemplate(id, clientID, domain.StatusApproved)

	tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(alreadyApproved, nil)

	_, err := srv.ApproveTemplate(context.Background(), &templatev1.ApproveTemplateRequest{
		Id: id.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ─── RejectTemplate ───────────────────────────────────────────────────────────

func TestRejectTemplate_MissingID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.RejectTemplate(context.Background(), &templatev1.RejectTemplateRequest{Id: ""})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestRejectTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	pending := makeTemplate(id, clientID, domain.StatusPending)
	rejected := makeTemplate(id, clientID, domain.StatusRejected)
	rejected.RejectionReason = "policy violation"

	tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(pending, nil)
	tmplRepo.On("UpdateStatus", mock.Anything, id, domain.StatusRejected, "policy violation").Return(rejected, nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditEntry")).Return(nil)

	resp, err := srv.RejectTemplate(context.Background(), &templatev1.RejectTemplateRequest{
		Id:     id.String(),
		Reason: "policy violation",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Template)
	assert.Equal(t, domain.StatusRejected, resp.Template.Status)
	assert.Equal(t, "policy violation", resp.Template.RejectionReason)

	tmplRepo.AssertExpectations(t)
	auditRepo.AssertExpectations(t)
}

func TestRejectTemplate_AlreadyRejected_FailedPrecondition(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	alreadyRejected := makeTemplate(id, clientID, domain.StatusRejected)

	tmplRepo.On("GetByIDAdmin", mock.Anything, id).Return(alreadyRejected, nil)

	_, err := srv.RejectTemplate(context.Background(), &templatev1.RejectTemplateRequest{
		Id:     id.String(),
		Reason: "reason",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// ─── RenderTemplate ───────────────────────────────────────────────────────────

func TestRenderTemplate_MissingTemplateID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.RenderTemplate(context.Background(), &templatev1.RenderTemplateRequest{
		TemplateId: "",
		ClientId:   validClientID(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "template_id")
}

func TestRenderTemplate_MissingClientID(t *testing.T) {
	srv := newTemplateServer(new(mocks.MockTemplateRepository), new(mocks.MockAuditRepository))

	_, err := srv.RenderTemplate(context.Background(), &templatev1.RenderTemplateRequest{
		TemplateId: validID(),
		ClientId:   "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "client_id")
}

func TestRenderTemplate_TemplateNotApproved(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	draft := makeTemplate(id, clientID, domain.StatusDraft)

	tmplRepo.On("GetByID", mock.Anything, id, clientID).Return(draft, nil)

	_, err := srv.RenderTemplate(context.Background(), &templatev1.RenderTemplateRequest{
		TemplateId: id.String(),
		ClientId:   clientID.String(),
		Variables:  map[string]string{"name": "Alice"},
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestRenderTemplate_Success(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	tmpl := makeTemplate(id, clientID, domain.StatusApproved)
	// Body: "Hello {{name}}"
	tmplRepo.On("GetByID", mock.Anything, id, clientID).Return(tmpl, nil)

	resp, err := srv.RenderTemplate(context.Background(), &templatev1.RenderTemplateRequest{
		TemplateId: id.String(),
		ClientId:   clientID.String(),
		Variables:  map[string]string{"name": "Alice"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello Alice", resp.RenderedText)
	assert.Equal(t, tmpl.Name, resp.TemplateName)

	tmplRepo.AssertExpectations(t)
}

func TestRenderTemplate_MissingVariables(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	tmpl := makeTemplate(id, clientID, domain.StatusApproved)

	tmplRepo.On("GetByID", mock.Anything, id, clientID).Return(tmpl, nil)

	_, err := srv.RenderTemplate(context.Background(), &templatev1.RenderTemplateRequest{
		TemplateId: id.String(),
		ClientId:   clientID.String(),
		Variables:  map[string]string{}, // missing "name"
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ─── error mapping ────────────────────────────────────────────────────────────

func TestErrorMapping_InvalidTemplateName(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	// Name validation happens inside TemplateService before repo is called
	_, err := srv.CreateTemplate(context.Background(), &templatev1.CreateTemplateRequest{
		ClientId: validClientID(),
		Name:     "", // triggers ErrInvalidTemplateName via service
		Body:     "body",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	// Empty name is caught by server validation before service: codes.InvalidArgument
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestErrorMapping_InternalError(t *testing.T) {
	tmplRepo := new(mocks.MockTemplateRepository)
	auditRepo := new(mocks.MockAuditRepository)
	srv := newTemplateServer(tmplRepo, auditRepo)

	id := uuid.New()
	clientID := uuid.New()
	tmplRepo.On("GetByID", mock.Anything, id, clientID).
		Return(nil, assert.AnError)

	_, err := srv.GetTemplate(context.Background(), &templatev1.GetTemplateRequest{
		Id:       id.String(),
		ClientId: clientID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}
