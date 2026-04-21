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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
)

// mockTemplateClient определён в sms_test.go (в том же test package) — переиспользуем.

func newAuditRequest(clientID uuid.UUID, templateID string) *http.Request {
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

	client.On("GetTemplate", mock.Anything, &templatev1.GetTemplateRequest{
		Id:       templateID,
		ClientId: clientID.String(),
	}).Return(&templatev1.GetTemplateResponse{
		Template: &templatev1.TemplateInfo{Id: templateID, ClientId: clientID.String()},
	}, nil)
	client.On("GetTemplateAuditLog", mock.Anything, &templatev1.GetTemplateAuditLogRequest{
		TemplateId: templateID,
		Limit:      100,
		Offset:     0,
	}).Return(&templatev1.GetTemplateAuditLogResponse{
		Entries: []*templatev1.AuditEntry{},
		Total:   0,
	}, nil)

	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, newAuditRequest(clientID, templateID))

	assert.Equal(t, http.StatusOK, rr.Code)
	client.AssertExpectations(t)
}

func TestGetTemplateAudit_OwnershipCheck_BlocksForeignTemplate(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	templateID := uuid.New().String()
	client := &mockTemplateClient{}
	handler := NewTemplateHandlers(client)

	client.On("GetTemplate", mock.Anything, &templatev1.GetTemplateRequest{
		Id:       templateID,
		ClientId: clientID.String(),
	}).Return(nil, status.Error(codes.NotFound, "template not found"))

	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, newAuditRequest(clientID, templateID))

	require.Equal(t, http.StatusNotFound, rr.Code, "должны вернуть 404 для чужого template")
	client.AssertNotCalled(t, "GetTemplateAuditLog", mock.Anything, mock.Anything)
	client.AssertExpectations(t)
}

func TestGetTemplateAudit_Unauthorized(t *testing.T) {
	t.Parallel()

	templateID := uuid.New().String()
	client := &mockTemplateClient{}
	handler := NewTemplateHandlers(client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/"+templateID+"/audit", nil)
	req = mux.SetURLVars(req, map[string]string{"id": templateID})
	rr := httptest.NewRecorder()
	handler.GetTemplateAudit(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	client.AssertNotCalled(t, "GetTemplate", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "GetTemplateAuditLog", mock.Anything, mock.Anything)
}
