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
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
)

// --- Mock RoutingServiceClient ---

type mockRoutingClient struct {
	mock.Mock
}

func (m *mockRoutingClient) GetRoute(ctx context.Context, in *routingv1.GetRouteRequest, opts ...grpc.CallOption) (*routingv1.GetRouteResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.GetRouteResponse), args.Error(1)
}

func (m *mockRoutingClient) SelectProvider(ctx context.Context, in *routingv1.SelectProviderRequest, opts ...grpc.CallOption) (*routingv1.SelectProviderResponse, error) {
	return nil, nil
}

func (m *mockRoutingClient) CreateRoute(ctx context.Context, in *routingv1.CreateRouteRequest, opts ...grpc.CallOption) (*routingv1.CreateRouteResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.CreateRouteResponse), args.Error(1)
}

func (m *mockRoutingClient) UpdateRoute(ctx context.Context, in *routingv1.UpdateRouteRequest, opts ...grpc.CallOption) (*routingv1.UpdateRouteResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.UpdateRouteResponse), args.Error(1)
}

func (m *mockRoutingClient) DeleteRoute(ctx context.Context, in *routingv1.DeleteRouteRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.DeleteRouteResponse), args.Error(1)
}

func (m *mockRoutingClient) ListRoutes(ctx context.Context, in *routingv1.ListRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListRoutesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.ListRoutesResponse), args.Error(1)
}

func (m *mockRoutingClient) CreateCountry(ctx context.Context, in *routingv1.CreateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClient) GetCountry(ctx context.Context, in *routingv1.GetCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListCountries(ctx context.Context, in *routingv1.ListCountriesRequest, opts ...grpc.CallOption) (*routingv1.ListCountriesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) UpdateCountry(ctx context.Context, in *routingv1.UpdateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClient) CreateOperator(ctx context.Context, in *routingv1.CreateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClient) GetOperator(ctx context.Context, in *routingv1.GetOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListOperators(ctx context.Context, in *routingv1.ListOperatorsRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorsResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) UpdateOperator(ctx context.Context, in *routingv1.UpdateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClient) CreateOperatorPrefix(ctx context.Context, in *routingv1.CreateOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.OperatorPrefix, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListOperatorPrefixes(ctx context.Context, in *routingv1.ListOperatorPrefixesRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorPrefixesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) DeleteOperatorPrefix(ctx context.Context, in *routingv1.DeleteOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.DeleteOperatorPrefixResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) ResolveOperator(ctx context.Context, in *routingv1.ResolveOperatorRequest, opts ...grpc.CallOption) (*routingv1.ResolveOperatorResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) NumberLookup(ctx context.Context, in *routingv1.NumberLookupRequest, opts ...grpc.CallOption) (*routingv1.NumberLookupResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) BulkNumberLookup(ctx context.Context, in *routingv1.BulkNumberLookupRequest, opts ...grpc.CallOption) (*routingv1.BulkNumberLookupResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) CreateHLRProvider(ctx context.Context, in *routingv1.CreateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) UpdateHLRProvider(ctx context.Context, in *routingv1.UpdateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) DeleteHLRProvider(ctx context.Context, in *routingv1.DeleteHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) GetHLRProvider(ctx context.Context, in *routingv1.GetHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListHLRProviders(ctx context.Context, in *routingv1.ListHLRProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListHLRProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) GetLookupHistory(ctx context.Context, in *routingv1.GetLookupHistoryRequest, opts ...grpc.CallOption) (*routingv1.GetLookupHistoryResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) RouteMessageWithHLR(ctx context.Context, in *routingv1.RouteMessageWithHLRRequest, opts ...grpc.CallOption) (*routingv1.RouteMessageWithHLRResponse, error) {
	return nil, nil
}

// Client-provider assignment stubs
func (m *mockRoutingClient) AssignProviderToClient(ctx context.Context, in *routingv1.AssignProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) RevokeProviderFromClient(ctx context.Context, in *routingv1.RevokeProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListClientProviders(ctx context.Context, in *routingv1.ListClientProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListClientProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClient) UpdateClientProvider(ctx context.Context, in *routingv1.UpdateClientProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) ShareProviderWithChild(ctx context.Context, in *routingv1.ShareProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) RevokeSharedProvider(ctx context.Context, in *routingv1.RevokeSharedProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

// Client route stubs
func (m *mockRoutingClient) CreateClientRoute(ctx context.Context, in *routingv1.CreateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) UpdateClientRoute(ctx context.Context, in *routingv1.UpdateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) DeleteClientRoute(ctx context.Context, in *routingv1.DeleteClientRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClient) ListClientRoutes(ctx context.Context, in *routingv1.ListClientRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListClientRoutesResponse, error) {
	return nil, nil
}

// Routing strategy stubs
func (m *mockRoutingClient) SetRoutingStrategy(ctx context.Context, in *routingv1.SetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) GetRoutingStrategy(ctx context.Context, in *routingv1.GetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClient) DeleteRoutingStrategy(ctx context.Context, in *routingv1.DeleteRoutingStrategyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

var _ routingv1.RoutingServiceClient = (*mockRoutingClient)(nil)

// --- Tests ---

func TestRoutingHandlers(t *testing.T) {
	t.Run("CreateRoute", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			client.On("CreateRoute", mock.Anything, mock.MatchedBy(func(req *routingv1.CreateRouteRequest) bool {
				return req.Name == "Default Route" &&
					req.Pattern == "^\\+7" &&
					req.Priority == 10 &&
					len(req.ProviderIds) == 2
			})).Return(&routingv1.CreateRouteResponse{
				RouteId:   "route-new-1",
				CreatedAt: timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(CreateRouteRequest{
				Name:                "Default Route",
				Pattern:             "^\\+7",
				Priority:            10,
				ProviderIDs:         []string{"prov-1", "prov-2"},
				LoadBalanceStrategy: "round_robin",
				FailoverEnabled:     true,
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/routes", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateRoute(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp CreateRouteResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "route-new-1", resp.RouteID)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/routes", bytes.NewReader([]byte("bad")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateRoute(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when validation fails - missing name", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			body, _ := json.Marshal(CreateRouteRequest{
				Pattern:     "^\\+7",
				ProviderIDs: []string{"prov-1"},
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/routes", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateRoute(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when validation fails - empty provider_ids", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			body, _ := json.Marshal(CreateRouteRequest{
				Name:        "Route",
				Pattern:     "^\\+7",
				ProviderIDs: []string{},
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/routes", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateRoute(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("UpdateRoute", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			client.On("UpdateRoute", mock.Anything, mock.MatchedBy(func(req *routingv1.UpdateRouteRequest) bool {
				return req.RouteId == "route-1" && req.Name == "Updated Route"
			})).Return(&routingv1.UpdateRouteResponse{
				Success: true,
			}, nil)

			name := "Updated Route"
			body, _ := json.Marshal(UpdateRouteRequest{
				Name: &name,
			})

			req := httptest.NewRequest(http.MethodPut, "/admin/v1/routes/route-1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "route-1"})

			rr := httptest.NewRecorder()
			handler.UpdateRoute(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp UpdateRouteResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.True(t, resp.Success)

			client.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			req := httptest.NewRequest(http.MethodPut, "/admin/v1/routes/route-1", bytes.NewReader([]byte("bad")))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "route-1"})

			rr := httptest.NewRecorder()
			handler.UpdateRoute(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when gRPC fails", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			client.On("UpdateRoute", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.NotFound, "route not found"))

			name := "Updated"
			body, _ := json.Marshal(UpdateRouteRequest{
				Name: &name,
			})

			req := httptest.NewRequest(http.MethodPut, "/admin/v1/routes/nonexistent", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

			rr := httptest.NewRecorder()
			handler.UpdateRoute(rr, req)

			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	})

	t.Run("ListRoutes", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			client.On("ListRoutes", mock.Anything, mock.MatchedBy(func(req *routingv1.ListRoutesRequest) bool {
				return req.ActiveOnly == false && req.Limit == 50 && req.Offset == 0
			})).Return(&routingv1.ListRoutesResponse{
				Routes: []*routingv1.RouteInfo{
					{
						RouteId:     "route-1",
						Name:        "Route One",
						Pattern:     "^\\+7",
						Priority:    10,
						ProviderIds: []string{"prov-1"},
						Active:      true,
						CreatedAt:   timestamppb.Now(),
						UpdatedAt:   timestamppb.Now(),
					},
				},
				Total: 1,
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/routes", nil)

			rr := httptest.NewRecorder()
			handler.ListRoutes(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp ListRoutesResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Len(t, resp.Routes, 1)
			assert.Equal(t, "route-1", resp.Routes[0].RouteID)
			assert.Equal(t, 1, resp.Total)

			client.AssertExpectations(t)
		})
	})

	t.Run("DeleteRoute", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClient)
			handler := NewRoutingHandlers(client)

			client.On("DeleteRoute", mock.Anything, mock.MatchedBy(func(req *routingv1.DeleteRouteRequest) bool {
				return req.RouteId == "route-to-delete"
			})).Return(&routingv1.DeleteRouteResponse{
				Success: true,
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/admin/v1/routes/route-to-delete", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "route-to-delete"})

			rr := httptest.NewRecorder()
			handler.DeleteRoute(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp DeleteRouteResponse
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.True(t, resp.Success)

			client.AssertExpectations(t)
		})
	})
}
