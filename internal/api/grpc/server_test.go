package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/api/proto/smsv1"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/metadata"
)

func TestServer(t *testing.T) {
	t.Run("SendSMS", func(t *testing.T) {
		clientID := uuid.New()
		messageID := uuid.New()

		tests := []struct {
			name           string
			request        *smsv1.SendSMSRequest
			setupMocks     func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository)
			contextClientID uuid.UUID
			expectedCode   codes.Code
			validateResponse func(t *testing.T, resp *smsv1.SendSMSResponse)
		}{
			{
				name: "successful send",
				request: &smsv1.SendSMSRequest{
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
					return producer, messageRepo, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.OK,
				validateResponse: func(t *testing.T, resp *smsv1.SendSMSResponse) {
					assert.NotEmpty(t, resp.MessageId)
					assert.Equal(t, "queued", resp.Status)
				},
			},
			{
				name: "missing source",
				request: &smsv1.SendSMSRequest{
					Destination: "79001234567",
					Text:        "Test message",
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "missing destination",
				request: &smsv1.SendSMSRequest{
					Source: "12345",
					Text:   "Test message",
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "missing text",
				request: &smsv1.SendSMSRequest{
					Source:      "12345",
					Destination: "79001234567",
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "text too long",
				request: &smsv1.SendSMSRequest{
					Source:      "12345",
					Destination: "79001234567",
					Text:        string(make([]byte, 1601)),
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "invalid priority",
				request: &smsv1.SendSMSRequest{
					Source:      "12345",
					Destination: "79001234567",
					Text:        "Test message",
					Priority:    5,
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "client not found in context",
				request: &smsv1.SendSMSRequest{
					Source:      "12345",
					Destination: "79001234567",
					Text:        "Test message",
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: uuid.Nil,
				expectedCode:    codes.Unauthenticated,
			},
			{
				name: "database error",
				request: &smsv1.SendSMSRequest{
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
				contextClientID: clientID,
				expectedCode:    codes.Internal,
			},
			{
				name: "kafka error",
				request: &smsv1.SendSMSRequest{
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
				contextClientID: clientID,
				expectedCode:    codes.Internal,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				producer, messageRepo, clientRepo := tt.setupMocks()

				server := NewServer(
					producer,
					messageRepo,
					clientRepo,
				)

				ctx := context.Background()
				if tt.contextClientID != uuid.Nil {
					ctx = context.WithValue(ctx, middleware.ClientIDKey, tt.contextClientID)
				}

				resp, err := server.SendSMS(ctx, tt.request)

				if tt.expectedCode == codes.OK {
					require.NoError(t, err)
					if tt.validateResponse != nil {
						tt.validateResponse(t, resp)
					}
				} else {
					require.Error(t, err)
					st, ok := status.FromError(err)
					require.True(t, ok)
					assert.Equal(t, tt.expectedCode, st.Code())
				}
			})
		}
	})

	t.Run("SendBatchSMS", func(t *testing.T) {
		clientID := uuid.New()
		messageID1 := uuid.New()
		messageID2 := uuid.New()

		tests := []struct {
			name           string
			request        *smsv1.SendBatchRequest
			setupMocks     func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository)
			contextClientID uuid.UUID
			expectedCode   codes.Code
			validateResponse func(t *testing.T, resp *smsv1.SendBatchResponse)
		}{
			{
				name: "successful batch",
				request: &smsv1.SendBatchRequest{
					Messages: []*smsv1.SendSMSRequest{
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
				contextClientID: clientID,
				expectedCode:    codes.OK,
				validateResponse: func(t *testing.T, resp *smsv1.SendBatchResponse) {
					assert.Equal(t, int32(2), resp.SuccessCount)
					assert.Equal(t, int32(0), resp.FailedCount)
					assert.Len(t, resp.Results, 2)
				},
			},
			{
				name: "empty messages list",
				request: &smsv1.SendBatchRequest{
					Messages: []*smsv1.SendSMSRequest{},
				},
				setupMocks: func() (*testutil.MockProducer, *testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockProducer{}, &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "partial failures",
				request: &smsv1.SendBatchRequest{
					Messages: []*smsv1.SendSMSRequest{
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
				contextClientID: clientID,
				expectedCode:    codes.OK,
				validateResponse: func(t *testing.T, resp *smsv1.SendBatchResponse) {
					assert.Equal(t, int32(2), resp.SuccessCount)
					assert.Equal(t, int32(1), resp.FailedCount)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				producer, messageRepo, clientRepo := tt.setupMocks()

				server := NewServer(
					producer,
					messageRepo,
					clientRepo,
				)

				ctx := context.Background()
				if tt.contextClientID != uuid.Nil {
					ctx = context.WithValue(ctx, middleware.ClientIDKey, tt.contextClientID)
				}

				resp, err := server.SendBatchSMS(ctx, tt.request)

				if tt.expectedCode == codes.OK {
					require.NoError(t, err)
					if tt.validateResponse != nil {
						tt.validateResponse(t, resp)
					}
				} else {
					require.Error(t, err)
					st, ok := status.FromError(err)
					require.True(t, ok)
					assert.Equal(t, tt.expectedCode, st.Code())
				}
			})
		}
	})

	t.Run("GetStatus", func(t *testing.T) {
		clientID := uuid.New()
		otherClientID := uuid.New()
		messageID := uuid.New()

		tests := []struct {
			name           string
			request        *smsv1.GetStatusRequest
			setupMocks     func() (*testutil.MockMessageRepository, *testutil.MockClientRepository)
			contextClientID uuid.UUID
			expectedCode   codes.Code
			validateResponse func(t *testing.T, resp *smsv1.GetStatusResponse)
		}{
			{
				name: "successful get status",
				request: &smsv1.GetStatusRequest{
					MessageId: messageID.String(),
				},
				setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
					messageRepo := &testutil.MockMessageRepository{
						GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
							if id == messageID {
								now := time.Now()
								return &shared.Message{
									ID:          messageID,
									Source:      "12345",
									Destination: "79001234567",
									Text:        "Test",
									Status:      shared.MessageStatusSent,
									ClientID:    &clientID,
									CreatedAt:   now,
									SubmittedAt: &now,
								}, nil
							}
							return nil, storage.ErrNotFound
						},
					}
					return messageRepo, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.OK,
				validateResponse: func(t *testing.T, resp *smsv1.GetStatusResponse) {
					assert.Equal(t, messageID.String(), resp.MessageId)
					assert.Equal(t, "sent", resp.Status)
					assert.NotNil(t, resp.SubmittedAt)
				},
			},
			{
				name: "message not found",
				request: &smsv1.GetStatusRequest{
					MessageId: messageID.String(),
				},
				setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
					messageRepo := &testutil.MockMessageRepository{
						GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
							return nil, storage.ErrNotFound
						},
					}
					return messageRepo, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.NotFound,
			},
			{
				name: "access denied - different client",
				request: &smsv1.GetStatusRequest{
					MessageId: messageID.String(),
				},
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
				expectedCode:    codes.PermissionDenied,
			},
			{
				name: "missing message ID",
				request: &smsv1.GetStatusRequest{
					MessageId: "",
				},
				setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
			{
				name: "invalid message ID format",
				request: &smsv1.GetStatusRequest{
					MessageId: "invalid-uuid",
				},
				setupMocks: func() (*testutil.MockMessageRepository, *testutil.MockClientRepository) {
					return &testutil.MockMessageRepository{}, &testutil.MockClientRepository{}
				},
				contextClientID: clientID,
				expectedCode:    codes.InvalidArgument,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				messageRepo, clientRepo := tt.setupMocks()

				server := NewServer(
					nil,
					messageRepo,
					clientRepo,
				)

				ctx := context.Background()
				if tt.contextClientID != uuid.Nil {
					ctx = context.WithValue(ctx, middleware.ClientIDKey, tt.contextClientID)
				}

				resp, err := server.GetStatus(ctx, tt.request)

				if tt.expectedCode == codes.OK {
					require.NoError(t, err)
					if tt.validateResponse != nil {
						tt.validateResponse(t, resp)
					}
				} else {
					require.Error(t, err)
					st, ok := status.FromError(err)
					require.True(t, ok)
					assert.Equal(t, tt.expectedCode, st.Code())
				}
			})
		}
	})

	t.Run("StreamDLR", func(t *testing.T) {
		t.Run("returns unimplemented", func(t *testing.T) {
			server := NewServer(nil, nil, nil)

			req := &smsv1.StreamDLRRequest{}
			stream := &mockDLRStream{}

			err := server.StreamDLR(req, stream)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.Unimplemented, st.Code())
		})
	})
}

// Mock для DLR stream
type mockDLRStream struct{}

func (m *mockDLRStream) Send(*smsv1.DLRUpdate) error { return nil }
func (m *mockDLRStream) Context() context.Context      { return context.Background() }
func (m *mockDLRStream) SendMsg(interface{}) error     { return nil }
func (m *mockDLRStream) RecvMsg(interface{}) error     { return nil }
func (m *mockDLRStream) SetHeader(metadata.MD) error    { return nil }
func (m *mockDLRStream) SendHeader(metadata.MD) error   { return nil }
func (m *mockDLRStream) SetTrailer(metadata.MD)        {}
