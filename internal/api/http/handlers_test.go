package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func TestHandler_SendSMS(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()

	tests := []struct {
		name           string
		request        SendSMSRequest
		setupMocks     func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository)
		expectedStatus int
		validateResponse func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful send",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				producer := &testutil.MockProducer{
					PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
						return nil
					},
				}
				messageRepo := &testutil.MockMessageRepository{
					CreateFunc: func(ctx context.Context, msg *shared.Message) error {
						msg.ID = messageID
						return nil
					},
					UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error {
						return nil
					},
				}
				clientRepo := &testutil.MockClientRepository{}
				return producer, messageRepo, clientRepo
			},
			expectedStatus: http.StatusAccepted,
			validateResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result SendSMSResponse
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.MessageID)
				assert.Equal(t, "queued", result.Status)
			},
		},
		{
			name: "invalid request - missing source",
			request: SendSMSRequest{
				Destination: "79001234567",
				Text:        "Test message",
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "database error",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					CreateFunc: func(ctx context.Context, msg *shared.Message) error {
						return shared.ErrDatabase("Database error", nil)
					},
				}
				return &testutil.MockProducer{}, messageRepo, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "kafka error",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				producer := &testutil.MockProducer{
					PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
						return shared.ErrKafkaProducer(nil)
					},
				}
				messageRepo := &testutil.MockMessageRepository{
					CreateFunc: func(ctx context.Context, msg *shared.Message) error {
						msg.ID = messageID
						return nil
					},
				}
				return producer, messageRepo, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			producer, messageRepo, clientRepo := tt.setupMocks()
			
			// Создаем handler с моками
			handler := NewHandler(
				producer,
				messageRepo,
				clientRepo,
				nil,
			)

			// Создаем запрос
			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/sms/send", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), middleware.ClientIDKey, clientID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.SendSMS(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestHandler_SendBatchSMS(t *testing.T) {
	clientID := uuid.New()
	messageID1 := uuid.New()
	messageID2 := uuid.New()

	tests := []struct {
		name           string
		request        SendBatchRequest
		setupMocks     func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository)
		expectedStatus int
		validateResponse func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful batch",
			request: SendBatchRequest{
				Messages: []SendSMSRequest{
					{Source: "12345", Destination: "79001234567", Text: "Message 1"},
					{Source: "12345", Destination: "79001234568", Text: "Message 2"},
				},
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				callCount := 0
				producer := &testutil.MockProducer{
					PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
						return nil
					},
				}
				messageRepo := &testutil.MockMessageRepository{
					CreateFunc: func(ctx context.Context, msg *shared.Message) error {
						if callCount == 0 {
							msg.ID = messageID1
						} else {
							msg.ID = messageID2
						}
						callCount++
						return nil
					},
					UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error {
						return nil
					},
				}
				return producer, messageRepo, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusAccepted,
			validateResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result SendBatchResponse
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, 2, result.SuccessCount)
				assert.Equal(t, 0, result.FailedCount)
				assert.Len(t, result.Results, 2)
			},
		},
		{
			name: "partial failures",
			request: SendBatchRequest{
				Messages: []SendSMSRequest{
					{Source: "12345", Destination: "79001234567", Text: "Message 1"},
					{Source: "", Destination: "79001234568", Text: "Message 2"}, // invalid
					{Source: "12345", Destination: "79001234569", Text: "Message 3"},
				},
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				producer := &testutil.MockProducer{
					PublishOutgoingFunc: func(ctx context.Context, msg *queue.KafkaMessage) error {
						return nil
					},
				}
				messageRepo := &testutil.MockMessageRepository{
					CreateFunc: func(ctx context.Context, msg *shared.Message) error {
						if msg.Destination == "79001234567" {
							msg.ID = messageID1
						} else {
							msg.ID = messageID2
						}
						return nil
					},
					UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error {
						return nil
					},
				}
				return producer, messageRepo, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusAccepted,
			validateResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result SendBatchResponse
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, 2, result.SuccessCount)
				assert.Equal(t, 1, result.FailedCount)
			},
		},
		{
			name: "empty messages list",
			request: SendBatchRequest{
				Messages: []SendSMSRequest{},
			},
			setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
				return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			producer, messageRepo, clientRepo := tt.setupMocks()
			
			handler := NewHandler(
				producer,
				messageRepo,
				clientRepo,
				nil,
			)

			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/v1/sms/send/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), middleware.ClientIDKey, clientID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.SendBatchSMS(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestHandler_GetStatus(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()
	otherClientID := uuid.New()

	tests := []struct {
		name           string
		messageID      string
		setupMocks     func() (*testutil.MockMessageRepository, *testutil.MockClientRepository)
		contextClientID uuid.UUID
		expectedStatus int
		validateResponse func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:      "successful get status",
			messageID: messageID.String(),
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
						if id == messageID {
							return &shared.Message{
								ID:          messageID,
								Source:      "12345",
								Destination: "79001234567",
								Text:        "Test",
								Status:      shared.MessageStatusSent,
								ClientID:    &clientID,
								CreatedAt:   time.Now(),
							}, nil
						}
						return nil, storage.ErrNotFound
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusOK,
			validateResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result GetStatusResponse
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, messageID.String(), result.MessageID)
				assert.Equal(t, "sent", result.Status)
			},
		},
		{
			name:      "message not found",
			messageID: messageID.String(),
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
						return nil, storage.ErrNotFound
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusNotFound,
		},
		{
			name:      "access denied - different client",
			messageID: messageID.String(),
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
						return &shared.Message{
							ID:       messageID,
							ClientID: &otherClientID,
						}, nil
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusForbidden,
		},
		{
			name:      "invalid message ID format",
			messageID: "invalid-uuid",
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				return &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusBadRequest,
		},
		{
			name:      "missing message ID",
			messageID: "",
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				return &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageRepo, clientRepo := tt.setupMocks()
			
			handler := NewHandler(
				nil,
				messageRepo,
				clientRepo,
				nil,
			)

			req := httptest.NewRequest("GET", "/api/v1/sms/status?id="+tt.messageID, nil)
			ctx := context.WithValue(req.Context(), middleware.ClientIDKey, tt.contextClientID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.GetStatus(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestHandler_GetHistory(t *testing.T) {
	clientID := uuid.New()

	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func() (*testutil.MockMessageRepository, *testutil.MockClientRepository)
		contextClientID uuid.UUID
		expectedStatus int
		validateResponse func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:        "successful get history",
			queryParams: "limit=10&offset=0",
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByClientIDFunc: func(ctx context.Context, id uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
						return []*shared.Message{
							{ID: uuid.New(), Source: "12345", Destination: "79001234567", Status: shared.MessageStatusSent},
							{ID: uuid.New(), Source: "12345", Destination: "79001234568", Status: shared.MessageStatusQueued},
						}, nil
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusOK,
			validateResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result map[string]interface{}
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Contains(t, result, "messages")
				assert.Equal(t, float64(10), result["limit"])
			},
		},
		{
			name:        "with status filter",
			queryParams: "limit=10&offset=0&status=sent",
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByClientIDFunc: func(ctx context.Context, id uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
						if status != nil && *status == shared.MessageStatusSent {
							return []*shared.Message{
								{ID: uuid.New(), Status: shared.MessageStatusSent},
							}, nil
						}
						return []*shared.Message{}, nil
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusOK,
		},
		{
			name:        "invalid limit",
			queryParams: "limit=invalid&offset=0",
			setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
				messageRepo := &testutil.MockMessageRepository{
					GetByClientIDFunc: func(ctx context.Context, id uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
						// Должен использоваться дефолтный limit=100
						assert.Equal(t, 100, limit)
						return []*shared.Message{}, nil
					},
				}
				return messageRepo, &testutil.MockClientRepository{}
			},
			contextClientID: clientID,
			expectedStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageRepo, clientRepo := tt.setupMocks()
			
			handler := NewHandler(
				nil,
				messageRepo,
				clientRepo,
				nil,
			)

			req := httptest.NewRequest("GET", "/api/v1/sms/history?"+tt.queryParams, nil)
			ctx := context.WithValue(req.Context(), middleware.ClientIDKey, tt.contextClientID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.GetHistory(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestHandler_Health(t *testing.T) {
	t.Run("without health checker", func(t *testing.T) {
		handler := NewHandler(nil, nil, nil, nil)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "ok", result["status"])
	})

	t.Run("with health checker", func(t *testing.T) {
		healthChecker := monitoring.NewHealthChecker("test-service", "1.0.0")
		handler := NewHandler(nil, nil, nil, healthChecker)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSendSMSRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request SendSMSRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
			},
			wantErr: false,
		},
		{
			name: "missing source",
			request: SendSMSRequest{
				Destination: "79001234567",
				Text:        "Test message",
			},
			wantErr: true,
		},
		{
			name: "missing destination",
			request: SendSMSRequest{
				Source: "12345",
				Text:   "Test message",
			},
			wantErr: true,
		},
		{
			name: "missing text",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
			},
			wantErr: true,
		},
		{
			name: "text too long",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        string(make([]byte, 1601)),
			},
			wantErr: true,
		},
		{
			name: "invalid priority - negative",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				Priority:    -1,
			},
			wantErr: true,
		},
		{
			name: "invalid priority - too high",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				Priority:    4,
			},
			wantErr: true,
		},
		{
			name: "valid priority",
			request: SendSMSRequest{
				Source:      "12345",
				Destination: "79001234567",
				Text:        "Test message",
				Priority:    2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected shared.MessageEncoding
	}{
		{
			name:     "GSM7 - ASCII only",
			text:     "Hello World",
			expected: shared.MessageEncodingGSM7,
		},
		{
			name:     "UCS2 - with unicode",
			text:     "Привет",
			expected: shared.MessageEncodingUCS2,
		},
		{
			name:     "GSM7 - numbers and basic symbols",
			text:     "1234567890@#$%",
			expected: shared.MessageEncodingGSM7,
		},
		{
			name:     "UCS2 - mixed",
			text:     "Hello Привет",
			expected: shared.MessageEncodingUCS2,
		},
		{
			name:     "empty string",
			text:     "",
			expected: shared.MessageEncodingGSM7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectEncoding(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}
