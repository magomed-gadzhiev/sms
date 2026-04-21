package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
)

// mockTemplateClient — тестовая заглушка templatev1.TemplateServiceClient.
// Реально используем только GetTemplate и GetTemplateAuditLog (см. ownership
// fix F-C1); остальные методы возвращают nil/не вызываются в этих тестах.
type mockTemplateClient struct {
	mock.Mock
}

func (m *mockTemplateClient) CreateTemplate(ctx context.Context, in *templatev1.CreateTemplateRequest, opts ...grpc.CallOption) (*templatev1.CreateTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) UpdateTemplate(ctx context.Context, in *templatev1.UpdateTemplateRequest, opts ...grpc.CallOption) (*templatev1.UpdateTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) DeleteTemplate(ctx context.Context, in *templatev1.DeleteTemplateRequest, opts ...grpc.CallOption) (*templatev1.DeleteTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) GetTemplate(ctx context.Context, in *templatev1.GetTemplateRequest, opts ...grpc.CallOption) (*templatev1.GetTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.GetTemplateResponse), args.Error(1)
}
func (m *mockTemplateClient) ListTemplates(ctx context.Context, in *templatev1.ListTemplatesRequest, opts ...grpc.CallOption) (*templatev1.ListTemplatesResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) ApproveTemplate(ctx context.Context, in *templatev1.ApproveTemplateRequest, opts ...grpc.CallOption) (*templatev1.ApproveTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) RejectTemplate(ctx context.Context, in *templatev1.RejectTemplateRequest, opts ...grpc.CallOption) (*templatev1.RejectTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) RenderTemplate(ctx context.Context, in *templatev1.RenderTemplateRequest, opts ...grpc.CallOption) (*templatev1.RenderTemplateResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) GetTemplateAuditLog(ctx context.Context, in *templatev1.GetTemplateAuditLogRequest, opts ...grpc.CallOption) (*templatev1.GetTemplateAuditLogResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.GetTemplateAuditLogResponse), args.Error(1)
}
func (m *mockTemplateClient) AssignReviewer(ctx context.Context, in *templatev1.AssignReviewerRequest, opts ...grpc.CallOption) (*templatev1.AssignReviewerResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) RequestRevision(ctx context.Context, in *templatev1.RequestRevisionRequest, opts ...grpc.CallOption) (*templatev1.RequestRevisionResponse, error) {
	return nil, nil
}
func (m *mockTemplateClient) SubmitForReview(ctx context.Context, in *templatev1.SubmitForReviewRequest, opts ...grpc.CallOption) (*templatev1.SubmitForReviewResponse, error) {
	return nil, nil
}

func requestWithClientID(clientID uuid.UUID, templateID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID+"/audit", nil)
	req = mux.SetURLVars(req, map[string]string{"id": templateID})
	ctx := context.WithValue(req.Context(), middleware.ClientIDKey, clientID)
	return req.WithContext(ctx)
}

// ─── F-C1 regression: ownership check через GetTemplate перед audit fetch ───

func TestGetTemplateAudit_OwnershipCheck_AllowsOwnTemplate(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	templateID := uuid.New().String()
	client := &mockTemplateClient{}
	handler := NewTemplateHandlers(client)

	// Ownership check passes — template принадлежит клиенту.
	client.On("GetTemplate", mock.Anything, &templatev1.GetTemplateRequest{
		Id:       templateID,
		ClientId: clientID.String(),
	}).Return(&templatev1.GetTemplateResponse{
		Template: &templatev1.Template{Id: templateID, ClientId: clientID.String()},
	}, nil)
	// После ownership check — обычный audit fetch.
	client.On("GetTemplateAuditLog", mock.Anything, &templatev1.GetTemplateAuditLogRequest{
		TemplateId: templateID,
		Limit:      100,
		Offset:     0,
	}).Return(&templatev1.GetTemplateAuditLogResponse{
		Entries: []*templatev1.AuditEntry{},
		Total:   0,
	}, nil)

	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, requestWithClientID(clientID, templateID))

	assert.Equal(t, http.StatusOK, rr.Code)
	client.AssertExpectations(t)
}

func TestGetTemplateAudit_OwnershipCheck_BlocksForeignTemplate(t *testing.T) {
	t.Parallel()

	clientID := uuid.New() // не владелец template
	templateID := uuid.New().String()
	client := &mockTemplateClient{}
	handler := NewTemplateHandlers(client)

	// Template service возвращает NotFound (template принадлежит другому клиенту —
	// семантика template service: "not your template == not found").
	client.On("GetTemplate", mock.Anything, &templatev1.GetTemplateRequest{
		Id:       templateID,
		ClientId: clientID.String(),
	}).Return(nil, status.Error(codes.NotFound, "template not found"))

	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, requestWithClientID(clientID, templateID))

	// GetTemplateAuditLog НЕ должен быть вызван.
	require.Equal(t, http.StatusNotFound, rr.Code, "должны вернуть 404 для чужого template")
	client.AssertNotCalled(t, "GetTemplateAuditLog", mock.Anything, mock.Anything)
	client.AssertExpectations(t)
}

func TestGetTemplateAudit_Unauthorized(t *testing.T) {
	t.Parallel()

	templateID := uuid.New().String()
	client := &mockTemplateClient{}
	handler := NewTemplateHandlers(client)

	// Нет ClientID в контексте.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID+"/audit", nil)
	req = mux.SetURLVars(req, map[string]string{"id": templateID})
	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	client.AssertNotCalled(t, "GetTemplate", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "GetTemplateAuditLog", mock.Anything, mock.Anything)
}
