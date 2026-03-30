package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
)

// --- Mock MessagingServiceClient ---

type mockMessagingClient struct {
	mock.Mock
}

func (m *mockMessagingClient) SendMessage(ctx context.Context, in *messagingv1.SendMessageRequest, opts ...grpc.CallOption) (*messagingv1.SendMessageResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.SendMessageResponse), args.Error(1)
}

func (m *mockMessagingClient) SendBatch(ctx context.Context, in *messagingv1.SendBatchRequest, opts ...grpc.CallOption) (*messagingv1.SendBatchResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.SendBatchResponse), args.Error(1)
}

func (m *mockMessagingClient) GetMessageStatus(ctx context.Context, in *messagingv1.GetMessageStatusRequest, opts ...grpc.CallOption) (*messagingv1.GetMessageStatusResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.GetMessageStatusResponse), args.Error(1)
}

func (m *mockMessagingClient) GetMessageHistory(ctx context.Context, in *messagingv1.GetMessageHistoryRequest, opts ...grpc.CallOption) (*messagingv1.GetMessageHistoryResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.GetMessageHistoryResponse), args.Error(1)
}

func (m *mockMessagingClient) ProcessDLR(ctx context.Context, in *messagingv1.ProcessDLRRequest, opts ...grpc.CallOption) (*messagingv1.ProcessDLRResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.ProcessDLRResponse), args.Error(1)
}

func (m *mockMessagingClient) CancelMessage(ctx context.Context, in *messagingv1.CancelMessageRequest, opts ...grpc.CallOption) (*messagingv1.CancelMessageResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.CancelMessageResponse), args.Error(1)
}

// --- Mock TemplateServiceClient ---

type mockTemplateClient struct {
	mock.Mock
}

func (m *mockTemplateClient) CreateTemplate(ctx context.Context, in *templatev1.CreateTemplateRequest, opts ...grpc.CallOption) (*templatev1.CreateTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.CreateTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) UpdateTemplate(ctx context.Context, in *templatev1.UpdateTemplateRequest, opts ...grpc.CallOption) (*templatev1.UpdateTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.UpdateTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) DeleteTemplate(ctx context.Context, in *templatev1.DeleteTemplateRequest, opts ...grpc.CallOption) (*templatev1.DeleteTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.DeleteTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) GetTemplate(ctx context.Context, in *templatev1.GetTemplateRequest, opts ...grpc.CallOption) (*templatev1.GetTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.GetTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) ListTemplates(ctx context.Context, in *templatev1.ListTemplatesRequest, opts ...grpc.CallOption) (*templatev1.ListTemplatesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.ListTemplatesResponse), args.Error(1)
}

func (m *mockTemplateClient) ApproveTemplate(ctx context.Context, in *templatev1.ApproveTemplateRequest, opts ...grpc.CallOption) (*templatev1.ApproveTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.ApproveTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) RejectTemplate(ctx context.Context, in *templatev1.RejectTemplateRequest, opts ...grpc.CallOption) (*templatev1.RejectTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.RejectTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) RenderTemplate(ctx context.Context, in *templatev1.RenderTemplateRequest, opts ...grpc.CallOption) (*templatev1.RenderTemplateResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.RenderTemplateResponse), args.Error(1)
}

func (m *mockTemplateClient) GetTemplateAuditLog(ctx context.Context, in *templatev1.GetTemplateAuditLogRequest, opts ...grpc.CallOption) (*templatev1.GetTemplateAuditLogResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.GetTemplateAuditLogResponse), args.Error(1)
}

func (m *mockTemplateClient) AssignReviewer(ctx context.Context, in *templatev1.AssignReviewerRequest, opts ...grpc.CallOption) (*templatev1.AssignReviewerResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.AssignReviewerResponse), args.Error(1)
}

func (m *mockTemplateClient) RequestRevision(ctx context.Context, in *templatev1.RequestRevisionRequest, opts ...grpc.CallOption) (*templatev1.RequestRevisionResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.RequestRevisionResponse), args.Error(1)
}

func (m *mockTemplateClient) SubmitForReview(ctx context.Context, in *templatev1.SubmitForReviewRequest, opts ...grpc.CallOption) (*templatev1.SubmitForReviewResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*templatev1.SubmitForReviewResponse), args.Error(1)
}

// --- Helpers ---

func contextWithClientID(ctx context.Context, clientID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, middleware.ClientIDKey, clientID)
	ctx = context.WithValue(ctx, middleware.UserIDKey, clientID)
	return ctx
}

// --- Tests ---

func TestSMSHandlers(t *testing.T) {
	t.Run("SendSMS", func(t *testing.T) {
		t.Run("success with direct text", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			msgClient.On("SendMessage", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendMessageRequest) bool {
				return req.ClientId == clientID.String() &&
					req.Source == "MyApp" &&
					req.Destination == "+79001234567" &&
					req.Text == "Hello World"
			})).Return(&messagingv1.SendMessageResponse{
				MessageId:    "msg-123",
				Status:       "queued",
				CreatedAt:    timestamppb.Now(),
				SegmentCount: 1,
			}, nil)

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
				Text:        "Hello World",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "msg-123", resp["message_id"])
			assert.Equal(t, "queued", resp["status"])
			assert.NotNil(t, resp["created_at"])
			assert.Equal(t, float64(1), resp["segment_count"])

			msgClient.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader([]byte("invalid json")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.NotNil(t, resp["error"])
		})

		t.Run("returns 401 when client ID is missing from context", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
				Text:        "Hello World",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			// No client ID in context

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns 400 when source is empty", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "",
				Destination: "+79001234567",
				Text:        "Hello World",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when both text and template_id provided", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
				Text:        "Hello World",
				TemplateID:  "tmpl-1",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when neither text nor template_id provided", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("success with template_id", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			tmplClient.On("RenderTemplate", mock.Anything, mock.MatchedBy(func(req *templatev1.RenderTemplateRequest) bool {
				return req.TemplateId == "tmpl-1" && req.ClientId == clientID.String()
			})).Return(&templatev1.RenderTemplateResponse{
				RenderedText: "Hello, John!",
			}, nil)

			msgClient.On("SendMessage", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendMessageRequest) bool {
				return req.Text == "Hello, John!"
			})).Return(&messagingv1.SendMessageResponse{
				MessageId:    "msg-456",
				Status:       "queued",
				CreatedAt:    timestamppb.Now(),
				SegmentCount: 1,
			}, nil)

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
				TemplateID:  "tmpl-1",
				Variables:   map[string]string{"name": "John"},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "msg-456", resp["message_id"])

			tmplClient.AssertExpectations(t)
			msgClient.AssertExpectations(t)
		})

		t.Run("returns error when messaging service fails", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			msgClient.On("SendMessage", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "service down"))

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MyApp",
				Destination: "+79001234567",
				Text:        "Hello",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("GetStatus", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			msgClient.On("GetMessageStatus", mock.Anything, mock.MatchedBy(func(req *messagingv1.GetMessageStatusRequest) bool {
				return req.MessageId == "msg-123" && req.ClientId == clientID.String()
			})).Return(&messagingv1.GetMessageStatusResponse{
				MessageId:     "msg-123",
				Status:        "delivered",
				StatusMessage: "Delivered",
				CreatedAt:     timestamppb.Now(),
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/msg-123", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "msg-123"})
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.GetStatus(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "msg-123", resp["message_id"])
			assert.Equal(t, "delivered", resp["status"])
		})

		t.Run("returns 401 when no client ID", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/msg-123", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "msg-123"})

			rr := httptest.NewRecorder()
			handler.GetStatus(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	})

	t.Run("CancelSMS", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil)

			clientID := uuid.New()

			msgClient.On("CancelMessage", mock.Anything, mock.MatchedBy(func(req *messagingv1.CancelMessageRequest) bool {
				return req.MessageId == "msg-789" && req.ClientId == clientID.String()
			})).Return(&messagingv1.CancelMessageResponse{}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/sms/msg-789", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "msg-789"})
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.CancelSMS(rr, req)

			assert.Equal(t, http.StatusNoContent, rr.Code)
			msgClient.AssertExpectations(t)
		})
	})
}
