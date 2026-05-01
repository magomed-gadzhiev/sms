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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/authv1"
	portalMiddleware "github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// contextWithUserID creates a context with user ID set (portal middleware style).
func contextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, portalMiddleware.UserIDKey, userID)
}

// --- Tests ---

func TestAPIKeyHandlers(t *testing.T) {
	t.Run("ListAPIKeys", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			authClient.On("ListAPIKeys", mock.Anything, mock.MatchedBy(func(req *authv1.ListAPIKeysRequest) bool {
				return req.UserId == userID.String()
			})).Return(&authv1.ListAPIKeysResponse{
				Keys: []*authv1.APIKeyInfo{
					{
						Id:        "key-1",
						Name:      "Production Key",
						Prefix:    "sk_prod_",
						Active:    true,
						Scopes:    []string{"sms:send", "sms:read"},
						CreatedAt: timestamppb.Now(),
					},
					{
						Id:        "key-2",
						Name:      "Test Key",
						Prefix:    "sk_test_",
						Active:    false,
						CreatedAt: timestamppb.Now(),
					},
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/portal/v1/api-keys", nil)
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.ListAPIKeys(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			keys := resp["keys"].([]interface{})
			assert.Len(t, keys, 2)

			firstKey := keys[0].(map[string]interface{})
			assert.Equal(t, "key-1", firstKey["id"])
			assert.Equal(t, "Production Key", firstKey["name"])
			assert.Equal(t, true, firstKey["active"])

			authClient.AssertExpectations(t)
		})

		t.Run("returns 401 when user ID is missing", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			req := httptest.NewRequest(http.MethodGet, "/portal/v1/api-keys", nil)

			rr := httptest.NewRecorder()
			handler.ListAPIKeys(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns error when auth service fails", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			authClient.On("ListAPIKeys", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "auth service unavailable"))

			req := httptest.NewRequest(http.MethodGet, "/portal/v1/api-keys", nil)
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.ListAPIKeys(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("CreateAPIKey", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			authClient.On("CreateAPIKey", mock.Anything, mock.MatchedBy(func(req *authv1.CreateAPIKeyRequest) bool {
				return req.UserId == userID.String() && req.Name == "My Key"
			})).Return(&authv1.CreateAPIKeyResponse{
				ApiKey:    "sk_live_abc123xyz789",
				ApiKeyId:  "key-new-1",
				CreatedAt: timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(createAPIKeyRequest{
				Name:   "My Key",
				Scopes: []string{"sms:send"},
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/api-keys", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.CreateAPIKey(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "sk_live_abc123xyz789", resp["api_key"])
			assert.Equal(t, "key-new-1", resp["api_key_id"])

			authClient.AssertExpectations(t)
		})

		t.Run("returns 400 when name is empty", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			body, _ := json.Marshal(createAPIKeyRequest{
				Name: "",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/api-keys", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.CreateAPIKey(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 401 when user ID is missing", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			body, _ := json.Marshal(createAPIKeyRequest{
				Name: "Key",
			})

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/api-keys", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateAPIKey(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			req := httptest.NewRequest(http.MethodPost, "/portal/v1/api-keys", bytes.NewReader([]byte("bad json")))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.CreateAPIKey(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("RevokeAPIKey", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			authClient.On("RevokeAPIKey", mock.Anything, mock.MatchedBy(func(req *authv1.RevokeAPIKeyRequest) bool {
				return req.ApiKeyId == "key-to-revoke" && req.UserId == userID.String()
			})).Return(&authv1.RevokeAPIKeyResponse{
				Success: true,
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/portal/v1/api-keys/key-to-revoke", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "key-to-revoke"})
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.RevokeAPIKey(rr, req)

			assert.Equal(t, http.StatusNoContent, rr.Code)

			authClient.AssertExpectations(t)
		})

		t.Run("returns 401 when user ID is missing", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			req := httptest.NewRequest(http.MethodDelete, "/portal/v1/api-keys/key-1", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "key-1"})

			rr := httptest.NewRecorder()
			handler.RevokeAPIKey(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns error when auth service fails", func(t *testing.T) {
			authClient := new(mockAuthClient)
			handler := &APIKeyHandlers{
				authClient:     authClient,
				auditPublisher: nil,
			}

			userID := uuid.New()

			authClient.On("RevokeAPIKey", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.NotFound, "key not found"))

			req := httptest.NewRequest(http.MethodDelete, "/portal/v1/api-keys/nonexistent", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
			req = req.WithContext(contextWithUserID(req.Context(), userID))

			rr := httptest.NewRecorder()
			handler.RevokeAPIKey(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	})
}
