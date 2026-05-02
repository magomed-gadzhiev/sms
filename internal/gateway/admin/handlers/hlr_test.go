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

// --- Mock RoutingServiceClient for HLR ---

type mockRoutingClientForHLR struct {
	mock.Mock
}

// Routing methods (not used by HLR handlers, stubs only)
func (m *mockRoutingClientForHLR) GetRoute(ctx context.Context, in *routingv1.GetRouteRequest, opts ...grpc.CallOption) (*routingv1.GetRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) SelectProvider(ctx context.Context, in *routingv1.SelectProviderRequest, opts ...grpc.CallOption) (*routingv1.SelectProviderResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) CreateRoute(ctx context.Context, in *routingv1.CreateRouteRequest, opts ...grpc.CallOption) (*routingv1.CreateRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) UpdateRoute(ctx context.Context, in *routingv1.UpdateRouteRequest, opts ...grpc.CallOption) (*routingv1.UpdateRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) DeleteRoute(ctx context.Context, in *routingv1.DeleteRouteRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListRoutes(ctx context.Context, in *routingv1.ListRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListRoutesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) CreateCountry(ctx context.Context, in *routingv1.CreateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) GetCountry(ctx context.Context, in *routingv1.GetCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListCountries(ctx context.Context, in *routingv1.ListCountriesRequest, opts ...grpc.CallOption) (*routingv1.ListCountriesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) UpdateCountry(ctx context.Context, in *routingv1.UpdateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) CreateOperator(ctx context.Context, in *routingv1.CreateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) GetOperator(ctx context.Context, in *routingv1.GetOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListOperators(ctx context.Context, in *routingv1.ListOperatorsRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorsResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) UpdateOperator(ctx context.Context, in *routingv1.UpdateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) CreateOperatorPrefix(ctx context.Context, in *routingv1.CreateOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.OperatorPrefix, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListOperatorPrefixes(ctx context.Context, in *routingv1.ListOperatorPrefixesRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorPrefixesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) DeleteOperatorPrefix(ctx context.Context, in *routingv1.DeleteOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.DeleteOperatorPrefixResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ResolveOperator(ctx context.Context, in *routingv1.ResolveOperatorRequest, opts ...grpc.CallOption) (*routingv1.ResolveOperatorResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) NumberLookup(ctx context.Context, in *routingv1.NumberLookupRequest, opts ...grpc.CallOption) (*routingv1.NumberLookupResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) BulkNumberLookup(ctx context.Context, in *routingv1.BulkNumberLookupRequest, opts ...grpc.CallOption) (*routingv1.BulkNumberLookupResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) GetLookupHistory(ctx context.Context, in *routingv1.GetLookupHistoryRequest, opts ...grpc.CallOption) (*routingv1.GetLookupHistoryResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) RouteMessageWithHLR(ctx context.Context, in *routingv1.RouteMessageWithHLRRequest, opts ...grpc.CallOption) (*routingv1.RouteMessageWithHLRResponse, error) {
	return nil, nil
}

// HLR-specific methods (used by HLRHandlers)
func (m *mockRoutingClientForHLR) CreateHLRProvider(ctx context.Context, in *routingv1.CreateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.HLRProviderProto), args.Error(1)
}

func (m *mockRoutingClientForHLR) UpdateHLRProvider(ctx context.Context, in *routingv1.UpdateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.HLRProviderProto), args.Error(1)
}

func (m *mockRoutingClientForHLR) DeleteHLRProvider(ctx context.Context, in *routingv1.DeleteHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.DeleteRouteResponse), args.Error(1)
}

func (m *mockRoutingClientForHLR) GetHLRProvider(ctx context.Context, in *routingv1.GetHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.HLRProviderProto), args.Error(1)
}

func (m *mockRoutingClientForHLR) ListHLRProviders(ctx context.Context, in *routingv1.ListHLRProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListHLRProvidersResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.ListHLRProvidersResponse), args.Error(1)
}

// Client-provider assignment stubs
func (m *mockRoutingClientForHLR) AssignProviderToClient(ctx context.Context, in *routingv1.AssignProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) RevokeProviderFromClient(ctx context.Context, in *routingv1.RevokeProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListClientProviders(ctx context.Context, in *routingv1.ListClientProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListClientProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) UpdateClientProvider(ctx context.Context, in *routingv1.UpdateClientProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ShareProviderWithChild(ctx context.Context, in *routingv1.ShareProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) RevokeSharedProvider(ctx context.Context, in *routingv1.RevokeSharedProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

// Client route stubs
func (m *mockRoutingClientForHLR) CreateClientRoute(ctx context.Context, in *routingv1.CreateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) UpdateClientRoute(ctx context.Context, in *routingv1.UpdateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) DeleteClientRoute(ctx context.Context, in *routingv1.DeleteClientRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) ListClientRoutes(ctx context.Context, in *routingv1.ListClientRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListClientRoutesResponse, error) {
	return nil, nil
}

// Routing strategy stubs
func (m *mockRoutingClientForHLR) SetRoutingStrategy(ctx context.Context, in *routingv1.SetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) GetRoutingStrategy(ctx context.Context, in *routingv1.GetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForHLR) DeleteRoutingStrategy(ctx context.Context, in *routingv1.DeleteRoutingStrategyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

var _ routingv1.RoutingServiceClient = (*mockRoutingClientForHLR)(nil)

// --- Tests ---

func TestHLRHandlers(t *testing.T) {
	t.Run("ListProviders", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			client.On("ListHLRProviders", mock.Anything, mock.MatchedBy(func(req *routingv1.ListHLRProvidersRequest) bool {
				return req.ActiveOnly == true
			})).Return(&routingv1.ListHLRProvidersResponse{
				Providers: []*routingv1.HLRProviderProto{
					{
						Id:          "hlr-1",
						Name:        "Infobip HLR",
						AdapterType: "infobip",
						Priority:    1,
						Status:      "healthy",
						Active:      true,
						CreatedAt:   timestamppb.Now(),
					},
					{
						Id:          "hlr-2",
						Name:        "TMT HLR",
						AdapterType: "tmt",
						Priority:    2,
						Status:      "healthy",
						Active:      true,
						CreatedAt:   timestamppb.Now(),
					},
				},
			}, nil)

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/hlr/providers?active_only=true", nil)

			rr := httptest.NewRecorder()
			handler.ListProviders(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			providers := resp["providers"].([]interface{})
			assert.Len(t, providers, 2)
			assert.Equal(t, float64(2), resp["total"])

			first := providers[0].(map[string]interface{})
			assert.Equal(t, "hlr-1", first["id"])
			assert.Equal(t, "Infobip HLR", first["name"])

			client.AssertExpectations(t)
		})

		t.Run("returns error when service fails", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			client.On("ListHLRProviders", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Unavailable, "service down"))

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/hlr/providers", nil)

			rr := httptest.NewRecorder()
			handler.ListProviders(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	})

	t.Run("CreateProvider", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			client.On("CreateHLRProvider", mock.Anything, mock.MatchedBy(func(req *routingv1.CreateHLRProviderRequest) bool {
				return req.Name == "New HLR" && req.AdapterType == "infobip"
			})).Return(&routingv1.HLRProviderProto{
				Id:          "hlr-new-1",
				Name:        "New HLR",
				AdapterType: "infobip",
				Priority:    1,
				Active:      true,
				CreatedAt:   timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(CreateHLRProviderRequest{
				Name:        "New HLR",
				AdapterType: "infobip",
				Priority:    1,
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/hlr/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "hlr-new-1", resp["id"])
			assert.Equal(t, "New HLR", resp["name"])

			client.AssertExpectations(t)
		})

		t.Run("returns 400 when name is empty", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			body, _ := json.Marshal(CreateHLRProviderRequest{
				Name:        "",
				AdapterType: "infobip",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/hlr/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 when adapter_type is empty", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			body, _ := json.Marshal(CreateHLRProviderRequest{
				Name:        "My HLR",
				AdapterType: "",
			})

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/hlr/providers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 400 for invalid JSON", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			req := httptest.NewRequest(http.MethodPost, "/admin/v1/hlr/providers", bytes.NewReader([]byte("bad")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.CreateProvider(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})

	t.Run("DeleteProvider", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			client := new(mockRoutingClientForHLR)
			handler := NewHLRHandlers(client)

			client.On("DeleteHLRProvider", mock.Anything, mock.MatchedBy(func(req *routingv1.DeleteHLRProviderRequest) bool {
				return req.Id == "hlr-to-delete"
			})).Return(&routingv1.DeleteRouteResponse{
				Success: true,
			}, nil)

			req := httptest.NewRequest(http.MethodDelete, "/admin/v1/hlr/providers/hlr-to-delete", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "hlr-to-delete"})

			rr := httptest.NewRecorder()
			handler.DeleteProvider(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, true, resp["success"])

			client.AssertExpectations(t)
		})
	})

}
