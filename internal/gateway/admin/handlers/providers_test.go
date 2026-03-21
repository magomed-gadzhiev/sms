package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/providerv1"
)

// --- Mock ProviderServiceClient ---

type mockProviderClient struct {
	mock.Mock
}

func (m *mockProviderClient) CreateProvider(ctx context.Context, in *providerv1.CreateProviderRequest, opts ...grpc.CallOption) (*providerv1.CreateProviderResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.CreateProviderResponse), args.Error(1)
}

func (m *mockProviderClient) UpdateProvider(ctx context.Context, in *providerv1.UpdateProviderRequest, opts ...grpc.CallOption) (*providerv1.UpdateProviderResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.UpdateProviderResponse), args.Error(1)
}

func (m *mockProviderClient) GetProvider(ctx context.Context, in *providerv1.GetProviderRequest, opts ...grpc.CallOption) (*providerv1.GetProviderResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.GetProviderResponse), args.Error(1)
}

func (m *mockProviderClient) ListProviders(ctx context.Context, in *providerv1.ListProvidersRequest, opts ...grpc.CallOption) (*providerv1.ListProvidersResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.ListProvidersResponse), args.Error(1)
}

func (m *mockProviderClient) DeleteProvider(ctx context.Context, in *providerv1.DeleteProviderRequest, opts ...grpc.CallOption) (*providerv1.DeleteProviderRequest, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.DeleteProviderRequest), args.Error(1)
}

func (m *mockProviderClient) GetProviderHealth(ctx context.Context, in *providerv1.GetProviderHealthRequest, opts ...grpc.CallOption) (*providerv1.GetProviderHealthResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.GetProviderHealthResponse), args.Error(1)
}

func (m *mockProviderClient) SendToProvider(ctx context.Context, in *providerv1.SendToProviderRequest, opts ...grpc.CallOption) (*providerv1.SendToProviderResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providerv1.SendToProviderResponse), args.Error(1)
}

var _ providerv1.ProviderServiceClient = (*mockProviderClient)(nil)

// --- Tests ---

func TestProviderHandlers(t *testing.T) {
	t.Run("CreateProvider", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("CreateProvider", mock.Anything, mock.MatchedBy(func(req *providerv1.CreateProviderRequest) bool {
				return req.Name == "SMS Provider 1" &&
					req.Host == "smsc.example.com" &&
					req.Port == 2775 &&
					req.SystemId == "sys1" &&
					req.Password == "pass123"
			})).Return(&providerv1.CreateProviderResponse{
				ProviderId: "prov-123",
				CreatedAt:  timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(CreateProviderRequest{
				Name:           "SMS Provider 1",
				Host:           "smsc.example.com",
				Port:           2775,
				SystemID:       "sys1",
				Password:       "pass123",
				MaxConnections: 10,
				Active:         true,
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp CreateProviderResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "prov-123", resp.ProviderID)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/providers", bytes.NewReader([]byte("bad")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when validation fails - missing name", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			body, _ := json.Marshal(CreateProviderRequest{
				Host:     "smsc.example.com",
				Port:     2775,
				SystemID: "sys1",
				Password: "pass",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when validation fails - invalid port", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			body, _ := json.Marshal(CreateProviderRequest{
				Name:     "Test",
				Host:     "smsc.example.com",
				Port:     0,
				SystemID: "sys1",
				Password: "pass",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when gRPC service fails", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("CreateProvider", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.AlreadyExists, "provider already exists"))

			body, _ := json.Marshal(CreateProviderRequest{
				Name:     "Provider",
				Host:     "smsc.example.com",
				Port:     2775,
				SystemID: "sys1",
				Password: "pass",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusConflict, rr.Code)
		})
	})

	t.Run("ListProviders", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("ListProviders", mock.Anything, mock.MatchedBy(func(req *providerv1.ListProvidersRequest) bool {
				return req.ActiveOnly == true && req.Limit == 50 && req.Offset == 0
			})).Return(&providerv1.ListProvidersResponse{
				Providers: []*providerv1.ProviderInfo{
					{
						ProviderId: "prov-1",
						Name:       "Provider One",
						Host:       "host1.example.com",
						Port:       2775,
						Active:     true,
						CreatedAt:  timestamppb.Now(),
						UpdatedAt:  timestamppb.Now(),
					},
				},
				Total: 1,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/providers?active_only=true", nil)

			rr := httptest.NewRecorder()
			handler.ListProviders(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp ListProvidersResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Len(t, resp.Providers, 1)
			assert.Equal(t, "prov-1", resp.Providers[0].ProviderID)
			assert.Equal(t, 1, resp.Total)

			client.AssertExpectations(t)
		})

		t.Run("returns error when service fails", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("ListProviders", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "unavailable"))

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/providers", nil)

			rr := httptest.NewRecorder()
			handler.ListProviders(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("DeleteProvider", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("DeleteProvider", mock.Anything, mock.MatchedBy(func(req *providerv1.DeleteProviderRequest) bool {
				return req.ProviderId == "prov-to-delete"
			})).Return(&providerv1.DeleteProviderRequest{
				ProviderId: "prov-to-delete",
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/admin/v1/providers/prov-to-delete", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "prov-to-delete"})

			rr := httptest.NewRecorder()
			handler.DeleteProvider(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, true, resp["success"])

			client.AssertExpectations(t)
		})

		t.Run("returns error when service fails", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("DeleteProvider", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.NotFound, "provider not found"))

			req := httptest.NewRequest(http.MethodDelete, "/admin/v1/providers/nonexistent", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

			rr := httptest.NewRecorder()
			handler.DeleteProvider(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	})

	t.Run("GetProvider", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockProviderClient)
			handler := NewProviderHandlers(client)

			client.On("GetProvider", mock.Anything, mock.MatchedBy(func(req *providerv1.GetProviderRequest) bool {
				return req.ProviderId == "prov-123"
			})).Return(&providerv1.GetProviderResponse{
				Provider: &providerv1.ProviderInfo{
					ProviderId: "prov-123",
					Name:       "Test Provider",
					Host:       "host.example.com",
					Port:       2775,
					Active:     true,
					CreatedAt:  timestamppb.Now(),
					UpdatedAt:  timestamppb.Now(),
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/providers/prov-123", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "prov-123"})

			rr := httptest.NewRecorder()
			handler.GetProvider(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp ProviderInfo
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "prov-123", resp.ProviderID)
			assert.Equal(t, "Test Provider", resp.Name)

			client.AssertExpectations(t)
		})
	})
}
