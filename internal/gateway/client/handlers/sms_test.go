package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
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

func (m *mockMessagingClient) ListScheduledMessages(ctx context.Context, in *messagingv1.ListScheduledMessagesRequest, opts ...grpc.CallOption) (*messagingv1.ListScheduledMessagesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*messagingv1.ListScheduledMessagesResponse), args.Error(1)
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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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

		// Bug #7 integration: handler wires resolveSenderName into the SendSMS
		// flow when a non-nil senderNameClient is present. The helper-level
		// tests (sms_sender_auth_test.go) only cover resolveSenderName in
		// isolation; these tests prove the handler actually calls it, honours
		// the 403, and forwards the UUID via gRPC metadata to SendMessage.
		t.Run("rejects unregistered sender name with 403 and no SendMessage call", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			snClient := new(mockSenderNameClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, snClient)

			clientID := uuid.New()

			// approved-filter list: empty → sender not in approved set.
			snClient.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
				return in.ClientId == clientID.String() && in.Status == "approved"
			})).Return(&sendernamev1.ListSenderNamesResponse{}, nil).Once()
			// unfiltered list: also empty → truly unknown.
			snClient.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
				return in.ClientId == clientID.String() && in.Status == ""
			})).Return(&sendernamev1.ListSenderNamesResponse{}, nil).Once()

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "GHOST",
				Destination: "+79001234567",
				Text:        "Hello",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code)
			// Critical: SendMessage must NOT be invoked when sender is unregistered.
			msgClient.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything)
			snClient.AssertExpectations(t)
		})

		t.Run("forwards sender_name_id via x-sms-sender-name-id metadata", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			snClient := new(mockSenderNameClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, snClient)

			clientID := uuid.New()
			senderID := uuid.New().String()

			snClient.On("ListSenderNames", mock.Anything, mock.MatchedBy(func(in *sendernamev1.ListSenderNamesRequest) bool {
				return in.ClientId == clientID.String() && in.Status == "approved"
			})).Return(&sendernamev1.ListSenderNamesResponse{
				SenderNames: []*sendernamev1.SenderNameInfo{
					{Id: senderID, Name: "MYBRAND", Status: "approved"},
				},
			}, nil).Once()

			// Capture the ctx passed into SendMessage and assert metadata carries
			// the resolved sender_name_id (Bug #7 audit linkage).
			msgClient.On("SendMessage", mock.MatchedBy(func(ctx context.Context) bool {
				md, ok := metadata.FromOutgoingContext(ctx)
				if !ok {
					return false
				}
				vals := md.Get("x-sms-sender-name-id")
				return len(vals) == 1 && vals[0] == senderID
			}), mock.Anything).Return(&messagingv1.SendMessageResponse{
				MessageId:    "msg-sn-1",
				Status:       "queued",
				CreatedAt:    timestamppb.Now(),
				SegmentCount: 1,
			}, nil)

			body, _ := json.Marshal(SendSMSRequest{
				Source:      "MYBRAND",
				Destination: "+79001234567",
				Text:        "Hello",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SendSMS(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			msgClient.AssertExpectations(t)
			snClient.AssertExpectations(t)
		})
	})

	t.Run("GetStatus", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

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

	t.Run("ListScheduled", func(t *testing.T) {
		t.Run("success returns scheduled messages", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

			clientID := uuid.New()
			scheduledTs := timestamppb.Now()
			createdTs := timestamppb.Now()

			msgClient.On("ListScheduledMessages", mock.Anything, mock.MatchedBy(func(req *messagingv1.ListScheduledMessagesRequest) bool {
				return req.ClientId == clientID.String() && req.Limit == 50 && req.Offset == 10
			})).Return(&messagingv1.ListScheduledMessagesResponse{
				Messages: []*messagingv1.MessageInfo{
					{
						MessageId:   "msg-sched-1",
						Source:      "Sender",
						Destination: "+79001234567",
						Text:        "Scheduled text",
						Status:      "scheduled",
						ExternalId:  "ext-1",
						CreatedAt:   createdTs,
						ScheduledAt: scheduledTs,
					},
				},
				Total:  1,
				Limit:  50,
				Offset: 10,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/scheduled?limit=50&offset=10", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.ListScheduled(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, float64(1), resp["total"])
			assert.Equal(t, float64(50), resp["limit"])
			assert.Equal(t, float64(10), resp["offset"])

			messages, ok := resp["messages"].([]interface{})
			require.True(t, ok)
			require.Len(t, messages, 1)

			msg := messages[0].(map[string]interface{})
			assert.Equal(t, "msg-sched-1", msg["message_id"])
			assert.Equal(t, "Sender", msg["source"])
			assert.Equal(t, "+79001234567", msg["destination"])
			assert.Equal(t, "Scheduled text", msg["text"])
			assert.Equal(t, "scheduled", msg["status"])
			assert.Equal(t, "ext-1", msg["external_id"])
			assert.NotNil(t, msg["scheduled_at"])
			assert.NotEmpty(t, msg["created_at"])

			msgClient.AssertExpectations(t)
		})

		t.Run("returns 401 when no client ID", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/scheduled", nil)

			rr := httptest.NewRecorder()
			handler.ListScheduled(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("uses default limit and offset when not provided", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

			clientID := uuid.New()

			msgClient.On("ListScheduledMessages", mock.Anything, mock.MatchedBy(func(req *messagingv1.ListScheduledMessagesRequest) bool {
				return req.ClientId == clientID.String() && req.Limit == 100 && req.Offset == 0
			})).Return(&messagingv1.ListScheduledMessagesResponse{
				Messages: []*messagingv1.MessageInfo{},
				Total:    0,
				Limit:    100,
				Offset:   0,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/scheduled", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.ListScheduled(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, float64(0), resp["total"])

			msgClient.AssertExpectations(t)
		})

		t.Run("returns error when messaging service fails", func(t *testing.T) {
			msgClient := new(mockMessagingClient)
			tmplClient := new(mockTemplateClient)
			handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

			clientID := uuid.New()

			msgClient.On("ListScheduledMessages", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "service down"))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/scheduled", nil)
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.ListScheduled(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})
}

// --- AC-coverage additions ---
//
// Tests below fill gaps against specs/001-sms-gateway-platform/spec.md and
// specs/001-sms-gateway-platform/contracts/client-api.md:
//   - US1 AC4: invalid destination validation
//   - US2 AC2/AC3: NotFound / PermissionDenied mapping for status lookup
//   - US3 AC1/AC2: SendBatch happy-path & partial-success (previously 0% covered)
//   - US4 AC1/AC2: scheduled_at propagation and CancelSMS edge cases
//   - US7 (templates): template_id metadata forwarding + render-error mapping
//   - GetHistory: parameter parsing and default pagination (previously 0% covered)

func TestSendSMS_ACExtras(t *testing.T) {
	t.Run("returns 400 when destination is empty (US1 AC4)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		body, _ := json.Marshal(SendSMSRequest{
			Source: "MyApp",
			Text:   "Hello",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendSMS(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Defence: handler must reject before calling messaging-service.
		msgClient.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything)
	})

	t.Run("propagates external_id and scheduled_at to messaging proto (US4 AC1)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()
		future := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

		msgClient.On("SendMessage", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendMessageRequest) bool {
			if req.ExternalId != "client-ref-123" {
				return false
			}
			if req.ScheduledAt == nil {
				return false
			}
			return req.ScheduledAt.AsTime().Equal(future)
		})).Return(&messagingv1.SendMessageResponse{
			MessageId:    "msg-sched",
			Status:       "scheduled",
			CreatedAt:    timestamppb.Now(),
			ScheduledAt:  timestamppb.New(future),
			SegmentCount: 1,
		}, nil)

		body, _ := json.Marshal(SendSMSRequest{
			Source:      "MyApp",
			Destination: "+79001234567",
			Text:        "Hello",
			ExternalID:  "client-ref-123",
			ScheduledAt: &future,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendSMS(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "scheduled", resp["status"])
		assert.NotNil(t, resp["scheduled_at"])
		msgClient.AssertExpectations(t)
	})

	t.Run("forwards template_id via x-sms-template-id metadata (US7 audit linkage)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		tmplClient.On("RenderTemplate", mock.Anything, mock.Anything).
			Return(&templatev1.RenderTemplateResponse{RenderedText: "Hi, John"}, nil)

		msgClient.On("SendMessage", mock.MatchedBy(func(ctx context.Context) bool {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				return false
			}
			vals := md.Get("x-sms-template-id")
			return len(vals) == 1 && vals[0] == "tmpl-42"
		}), mock.Anything).Return(&messagingv1.SendMessageResponse{
			MessageId:    "m1",
			Status:       "queued",
			CreatedAt:    timestamppb.Now(),
			SegmentCount: 1,
		}, nil)

		body, _ := json.Marshal(SendSMSRequest{
			Source:      "MyApp",
			Destination: "+79001234567",
			TemplateID:  "tmpl-42",
			Variables:   map[string]string{"name": "John"},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendSMS(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		msgClient.AssertExpectations(t)
		tmplClient.AssertExpectations(t)
	})

	t.Run("rejects scheduled_at in the past with 400 (US4 AC4)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		past := time.Now().Add(-1 * time.Hour).UTC()
		body, _ := json.Marshal(SendSMSRequest{
			Source:      "MyApp",
			Destination: "+79001234567",
			Text:        "Hello",
			ScheduledAt: &past,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendSMS(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Must NOT call messaging-service when scheduled_at is in the past.
		msgClient.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything)
	})

	t.Run("template render failure is forwarded as gRPC error (US7 AC3)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		tmplClient.On("RenderTemplate", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.InvalidArgument, "missing required variable 'name'"))

		body, _ := json.Marshal(SendSMSRequest{
			Source:      "MyApp",
			Destination: "+79001234567",
			TemplateID:  "tmpl-9",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendSMS(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Messaging service must NOT be called when template render fails.
		msgClient.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything)
	})
}

func TestGetStatus_ACExtras(t *testing.T) {
	t.Run("returns 404 when messaging service reports NotFound (US2 AC2)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("GetMessageStatus", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.NotFound, "message not found"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/missing", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "missing"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetStatus(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("exposes sent_at per contract (mapped from proto submitted_at)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		submittedTs := timestamppb.Now()
		msgClient.On("GetMessageStatus", mock.Anything, mock.Anything).Return(&messagingv1.GetMessageStatusResponse{
			MessageId:   "msg-1",
			Status:      "sent",
			CreatedAt:   timestamppb.Now(),
			SubmittedAt: submittedTs,
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/msg-1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-1"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetStatus(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		// Canonical public field from the contract.
		assert.NotNil(t, resp["sent_at"], "response must expose sent_at per contracts/client-api.md")
		// Internal alias retained during transition.
		assert.NotNil(t, resp["submitted_at"])
	})

	t.Run("returns 403 when messaging service reports PermissionDenied (US2 AC3)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("GetMessageStatus", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.PermissionDenied, "not your message"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/other", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "other"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetStatus(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

func TestCancelSMS_ACExtras(t *testing.T) {
	t.Run("returns 401 when client ID is missing", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sms/msg-1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-1"})

		rr := httptest.NewRecorder()
		handler.CancelSMS(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		msgClient.AssertNotCalled(t, "CancelMessage", mock.Anything, mock.Anything)
	})

	t.Run("returns 400 when message ID is empty", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sms/", nil)
		req = mux.SetURLVars(req, map[string]string{"id": ""})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.CancelSMS(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		msgClient.AssertNotCalled(t, "CancelMessage", mock.Anything, mock.Anything)
	})

	t.Run("POST /sms/cancel/{id} returns JSON body per contract", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("CancelMessage", mock.Anything, mock.MatchedBy(func(req *messagingv1.CancelMessageRequest) bool {
			return req.MessageId == "msg-42" && req.ClientId == clientID.String()
		})).Return(&messagingv1.CancelMessageResponse{}, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/cancel/msg-42", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-42"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.CancelScheduled(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "msg-42", resp["message_id"])
		assert.Equal(t, "cancelled", resp["status"])
		msgClient.AssertExpectations(t)
	})

	t.Run("POST /sms/cancel/{id}: 401 when client ID missing", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/cancel/msg-42", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-42"})

		rr := httptest.NewRecorder()
		handler.CancelScheduled(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		msgClient.AssertNotCalled(t, "CancelMessage", mock.Anything, mock.Anything)
	})

	t.Run("POST /sms/cancel/{id}: forwards FailedPrecondition as 400", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("CancelMessage", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.FailedPrecondition, "not in scheduled status"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/cancel/msg-42", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-42"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.CancelScheduled(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("forwards FailedPrecondition as 400 when message is not scheduled (US4 AC2)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("CancelMessage", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.FailedPrecondition, "not in scheduled status"))

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/sms/msg-5", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "msg-5"})
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.CancelSMS(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestSendBatch(t *testing.T) {
	t.Run("success accepts multiple messages and returns per-message results (US3 AC1)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("SendBatch", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendBatchRequest) bool {
			return req.ClientId == clientID.String() && len(req.Messages) == 2
		})).Return(&messagingv1.SendBatchResponse{
			Results: []*messagingv1.SendMessageResponse{
				{MessageId: "m1", Status: "queued", CreatedAt: timestamppb.Now(), SegmentCount: 1},
				{MessageId: "m2", Status: "queued", CreatedAt: timestamppb.Now(), SegmentCount: 1},
			},
			SuccessCount: 2,
			FailedCount:  0,
		}, nil)

		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages: []SendSMSRequest{
				{Source: "App", Destination: "+79001111111", Text: "hi 1"},
				{Source: "App", Destination: "+79002222222", Text: "hi 2"},
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, float64(2), resp["total"])
		assert.Equal(t, float64(2), resp["accepted"])
		assert.Equal(t, float64(0), resp["rejected"])
		results, ok := resp["results"].([]interface{})
		require.True(t, ok)
		assert.Len(t, results, 2)
		msgClient.AssertExpectations(t)
	})

	t.Run("partial success: rejects malformed entries with index, accepts valid (US3 AC2)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		// Out of 5 submitted, only 2 are valid: 1 missing source, 1 missing text,
		// 1 has both text+template_id (mutually exclusive).
		msgClient.On("SendBatch", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendBatchRequest) bool {
			return len(req.Messages) == 2
		})).Return(&messagingv1.SendBatchResponse{
			Results: []*messagingv1.SendMessageResponse{
				{MessageId: "ok-1", Status: "queued", CreatedAt: timestamppb.Now(), SegmentCount: 1},
				{MessageId: "ok-2", Status: "queued", CreatedAt: timestamppb.Now(), SegmentCount: 1},
			},
			SuccessCount: 2,
		}, nil)

		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages: []SendSMSRequest{
				{Source: "App", Destination: "+79001111111", Text: "valid 1"},
				{Source: "", Destination: "+79002222222", Text: "no source"},
				{Source: "App", Destination: "+79003333333"}, // no text, no template
				{Source: "App", Destination: "+79004444444", Text: "x", TemplateID: "t"}, // both
				{Source: "App", Destination: "+79005555555", Text: "valid 2"},
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, float64(5), resp["total"])
		assert.Equal(t, float64(2), resp["accepted"])
		assert.Equal(t, float64(3), resp["rejected"])

		results, ok := resp["results"].([]interface{})
		require.True(t, ok)
		require.Len(t, results, 5)

		// Index 0 (valid) → accepted with message_id.
		r0 := results[0].(map[string]interface{})
		assert.Equal(t, "ok-1", r0["message_id"])
		assert.Nil(t, r0["error"])

		// Indices 1, 2, 3 → rejected with {error, index}.
		for _, i := range []int{1, 2, 3} {
			ri := results[i].(map[string]interface{})
			assert.NotEmpty(t, ri["error"], "results[%d] must carry error", i)
			assert.Equal(t, float64(i), ri["index"], "results[%d] must echo original index", i)
			assert.Nil(t, ri["message_id"])
		}

		// Index 4 (valid) → accepted.
		r4 := results[4].(map[string]interface{})
		assert.Equal(t, "ok-2", r4["message_id"])
		assert.Nil(t, r4["error"])

		msgClient.AssertExpectations(t)
	})

	t.Run("rejects batch larger than MaxBatchSize with 400 batch_too_large (US3 AC3)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgs := make([]SendSMSRequest, MaxBatchSize+1)
		for i := range msgs {
			msgs[i] = SendSMSRequest{Source: "App", Destination: "+79001234567", Text: "x"}
		}
		body, _ := json.Marshal(SendBatchSMSRequest{Messages: msgs})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		msgClient.AssertNotCalled(t, "SendBatch", mock.Anything, mock.Anything)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		errObj, ok := resp["error"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "BATCH_TOO_LARGE", errObj["code"])
	})

	t.Run("rejects batch with scheduled_at in the past (US4 AC4)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		past := time.Now().Add(-1 * time.Hour).UTC()
		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages:    []SendSMSRequest{{Source: "App", Destination: "+79001234567", Text: "hi"}},
			ScheduledAt: &past,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		msgClient.AssertNotCalled(t, "SendBatch", mock.Anything, mock.Anything)
	})

	t.Run("returns 400 on invalid JSON", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader([]byte("not-json")))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 401 when client ID is missing", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages: []SendSMSRequest{{Source: "App", Destination: "+79001111111", Text: "hi"}},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		msgClient.AssertNotCalled(t, "SendBatch", mock.Anything, mock.Anything)
	})

	t.Run("returns 400 when messages list is empty", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		body, _ := json.Marshal(SendBatchSMSRequest{Messages: []SendSMSRequest{}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		msgClient.AssertNotCalled(t, "SendBatch", mock.Anything, mock.Anything)
	})

	t.Run("propagates batch-level scheduled_at to messaging proto", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()
		future := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)

		msgClient.On("SendBatch", mock.Anything, mock.MatchedBy(func(req *messagingv1.SendBatchRequest) bool {
			return req.ScheduledAt != nil && req.ScheduledAt.AsTime().Equal(future)
		})).Return(&messagingv1.SendBatchResponse{
			Results:      []*messagingv1.SendMessageResponse{{MessageId: "m1", Status: "scheduled", CreatedAt: timestamppb.Now(), SegmentCount: 1}},
			SuccessCount: 1,
		}, nil)

		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages:    []SendSMSRequest{{Source: "App", Destination: "+79001111111", Text: "hi"}},
			ScheduledAt: &future,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		msgClient.AssertExpectations(t)
	})

	t.Run("forwards gRPC error from messaging service", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("SendBatch", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.Unavailable, "down"))

		body, _ := json.Marshal(SendBatchSMSRequest{
			Messages: []SendSMSRequest{{Source: "App", Destination: "+79001111111", Text: "hi"}},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.SendBatch(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}

func TestGetHistory(t *testing.T) {
	t.Run("passes filters and pagination to messaging proto", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

		msgClient.On("GetMessageHistory", mock.Anything, mock.MatchedBy(func(req *messagingv1.GetMessageHistoryRequest) bool {
			return req.ClientId == clientID.String() &&
				req.Status == "delivered" &&
				req.Destination == "+79001234567" &&
				req.Limit == 50 && req.Offset == 100 &&
				req.From != nil && req.From.AsTime().Equal(from) &&
				req.To != nil && req.To.AsTime().Equal(to)
		})).Return(&messagingv1.GetMessageHistoryResponse{
			Messages: []*messagingv1.MessageInfo{
				{MessageId: "h1", ClientId: clientID.String(), Source: "App", Destination: "+79001234567", Text: "hi", Status: "delivered", CreatedAt: timestamppb.Now()},
			},
			Total:  1,
			Limit:  50,
			Offset: 100,
		}, nil)

		url := "/api/v1/sms/history?status=delivered&destination=%2B79001234567&from=" +
			from.Format(time.RFC3339) + "&to=" + to.Format(time.RFC3339) +
			"&limit=50&offset=100"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetHistory(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, float64(1), resp["total"])
		assert.Equal(t, float64(50), resp["limit"])
		assert.Equal(t, float64(100), resp["offset"])
		msgs, ok := resp["messages"].([]interface{})
		require.True(t, ok)
		require.Len(t, msgs, 1)
		m := msgs[0].(map[string]interface{})
		assert.Equal(t, "h1", m["message_id"])
		assert.Equal(t, "delivered", m["status"])
		msgClient.AssertExpectations(t)
	})

	t.Run("uses default limit=100 and offset=0 when query params absent", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("GetMessageHistory", mock.Anything, mock.MatchedBy(func(req *messagingv1.GetMessageHistoryRequest) bool {
			return req.Limit == 100 && req.Offset == 0 && req.Status == "" && req.From == nil && req.To == nil
		})).Return(&messagingv1.GetMessageHistoryResponse{Messages: nil, Total: 0, Limit: 100, Offset: 0}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/history", nil)
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetHistory(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		msgClient.AssertExpectations(t)
	})

	t.Run("clamps out-of-range limit to default (>1000 ignored)", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("GetMessageHistory", mock.Anything, mock.MatchedBy(func(req *messagingv1.GetMessageHistoryRequest) bool {
			return req.Limit == 100 // 9999 rejected → fallback to default
		})).Return(&messagingv1.GetMessageHistoryResponse{Limit: 100, Offset: 0}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/history?limit=9999", nil)
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetHistory(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		msgClient.AssertExpectations(t)
	})

	t.Run("returns 401 when client ID is missing", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/history", nil)

		rr := httptest.NewRecorder()
		handler.GetHistory(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		msgClient.AssertNotCalled(t, "GetMessageHistory", mock.Anything, mock.Anything)
	})

	t.Run("forwards gRPC error from messaging service", func(t *testing.T) {
		msgClient := new(mockMessagingClient)
		tmplClient := new(mockTemplateClient)
		handler := NewSMSHandlers(msgClient, tmplClient, nil, nil)
		clientID := uuid.New()

		msgClient.On("GetMessageHistory", mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.Unavailable, "down"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/history", nil)
		req = req.WithContext(contextWithClientID(req.Context(), clientID))

		rr := httptest.NewRecorder()
		handler.GetHistory(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}
