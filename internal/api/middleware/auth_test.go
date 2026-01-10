package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/testutil"
)

func TestAuthMiddleware(t *testing.T) {
	clientID := uuid.New()
	apiKey := "test-api-key"
	activeClient := &shared.Client{
		ID:     clientID,
		APIKey: apiKey,
		Active: true,
	}
	inactiveClient := &shared.Client{
		ID:     clientID,
		APIKey: apiKey,
		Active: false,
	}

	tests := []struct {
		name           string
		request        func() *http.Request
		setupMocks     func() *testutil.MockClientRepository
		expectedStatus int
		checkContext   func(t *testing.T, ctx context.Context)
	}{
		{
			name: "successful auth with X-API-Key header",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
				req.Header.Set("X-API-Key", apiKey)
				return req
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{
					GetByAPIKeyFunc: func(ctx context.Context, key string) (*shared.Client, error) {
						if key == apiKey {
							return activeClient, nil
						}
						return nil, storage.ErrNotFound
					},
				}
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, ctx context.Context) {
				id, ok := GetClientID(ctx)
				assert.True(t, ok)
				assert.Equal(t, clientID, id)
			},
		},
		{
			name: "successful auth with Bearer token",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
				req.Header.Set("Authorization", "Bearer "+apiKey)
				return req
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{
					GetByAPIKeyFunc: func(ctx context.Context, key string) (*shared.Client, error) {
						if key == apiKey {
							return activeClient, nil
						}
						return nil, storage.ErrNotFound
					},
				}
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, ctx context.Context) {
				id, ok := GetClientID(ctx)
				assert.True(t, ok)
				assert.Equal(t, clientID, id)
			},
		},
		{
			name: "missing API key",
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/api/v1/sms/status", nil)
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid API key",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
				req.Header.Set("X-API-Key", "invalid-key")
				return req
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{
					GetByAPIKeyFunc: func(ctx context.Context, key string) (*shared.Client, error) {
						return nil, storage.ErrNotFound
					},
				}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "inactive client",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
				req.Header.Set("X-API-Key", apiKey)
				return req
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{
					GetByAPIKeyFunc: func(ctx context.Context, key string) (*shared.Client, error) {
						if key == apiKey {
							return inactiveClient, nil
						}
						return nil, storage.ErrNotFound
					},
				}
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "skip health endpoint",
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/health", nil)
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "skip metrics endpoint",
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/metrics", nil)
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "database error",
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/sms/status", nil)
				req.Header.Set("X-API-Key", apiKey)
				return req
			},
			setupMocks: func() *testutil.MockClientRepository {
				return &testutil.MockClientRepository{
					GetByAPIKeyFunc: func(ctx context.Context, key string) (*shared.Client, error) {
						return nil, shared.ErrDatabase("Database error", nil)
					},
				}
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientRepo := tt.setupMocks()
			cfg := &config.AuthConfig{
				APIKeyHeader: "X-API-Key",
			}

			middleware := AuthMiddleware(clientRepo, cfg)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.checkContext != nil {
					tt.checkContext(t, r.Context())
				}
				w.WriteHeader(http.StatusOK)
			}))

			req := tt.request()
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestGetClientID(t *testing.T) {
	clientID := uuid.New()

	ctx := context.WithValue(context.Background(), ClientIDKey, clientID)

	id, ok := GetClientID(ctx)
	assert.True(t, ok)
	assert.Equal(t, clientID, id)

	// Тест без client ID
	ctx2 := context.Background()
	id2, ok2 := GetClientID(ctx2)
	assert.False(t, ok2)
	assert.Equal(t, uuid.Nil, id2)
}

func TestGetClient(t *testing.T) {
	client := &shared.Client{
		ID:     uuid.New(),
		APIKey: "test-key",
		Active: true,
	}

	ctx := context.WithValue(context.Background(), ClientKey, client)

	retrieved, ok := GetClient(ctx)
	require.True(t, ok)
	assert.Equal(t, client.ID, retrieved.ID)
	assert.Equal(t, client.APIKey, retrieved.APIKey)

	// Тест без client
	ctx2 := context.Background()
	_, ok2 := GetClient(ctx2)
	assert.False(t, ok2)
}
