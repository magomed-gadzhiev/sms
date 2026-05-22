package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
)

// --- Mock RoutingServiceClient (client gateway) ---

type mockRoutingClientForLookup struct {
	mock.Mock
}

func (m *mockRoutingClientForLookup) GetRoute(ctx context.Context, in *routingv1.GetRouteRequest, opts ...grpc.CallOption) (*routingv1.GetRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) SelectProvider(ctx context.Context, in *routingv1.SelectProviderRequest, opts ...grpc.CallOption) (*routingv1.SelectProviderResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateRoute(ctx context.Context, in *routingv1.CreateRouteRequest, opts ...grpc.CallOption) (*routingv1.CreateRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateRoute(ctx context.Context, in *routingv1.UpdateRouteRequest, opts ...grpc.CallOption) (*routingv1.UpdateRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteRoute(ctx context.Context, in *routingv1.DeleteRouteRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListRoutes(ctx context.Context, in *routingv1.ListRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListRoutesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateCountry(ctx context.Context, in *routingv1.CreateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) GetCountry(ctx context.Context, in *routingv1.GetCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListCountries(ctx context.Context, in *routingv1.ListCountriesRequest, opts ...grpc.CallOption) (*routingv1.ListCountriesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateCountry(ctx context.Context, in *routingv1.UpdateCountryRequest, opts ...grpc.CallOption) (*routingv1.Country, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateOperator(ctx context.Context, in *routingv1.CreateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) GetOperator(ctx context.Context, in *routingv1.GetOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListOperators(ctx context.Context, in *routingv1.ListOperatorsRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorsResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateOperator(ctx context.Context, in *routingv1.UpdateOperatorRequest, opts ...grpc.CallOption) (*routingv1.Operator, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateOperatorPrefix(ctx context.Context, in *routingv1.CreateOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.OperatorPrefix, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListOperatorPrefixes(ctx context.Context, in *routingv1.ListOperatorPrefixesRequest, opts ...grpc.CallOption) (*routingv1.ListOperatorPrefixesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteOperatorPrefix(ctx context.Context, in *routingv1.DeleteOperatorPrefixRequest, opts ...grpc.CallOption) (*routingv1.DeleteOperatorPrefixResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ResolveOperator(ctx context.Context, in *routingv1.ResolveOperatorRequest, opts ...grpc.CallOption) (*routingv1.ResolveOperatorResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) NumberLookup(ctx context.Context, in *routingv1.NumberLookupRequest, opts ...grpc.CallOption) (*routingv1.NumberLookupResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.NumberLookupResponse), args.Error(1)
}
func (m *mockRoutingClientForLookup) BulkNumberLookup(ctx context.Context, in *routingv1.BulkNumberLookupRequest, opts ...grpc.CallOption) (*routingv1.BulkNumberLookupResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.BulkNumberLookupResponse), args.Error(1)
}
func (m *mockRoutingClientForLookup) CreateHLRProvider(ctx context.Context, in *routingv1.CreateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateHLRProvider(ctx context.Context, in *routingv1.UpdateHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteHLRProvider(ctx context.Context, in *routingv1.DeleteHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.DeleteRouteResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) GetHLRProvider(ctx context.Context, in *routingv1.GetHLRProviderRequest, opts ...grpc.CallOption) (*routingv1.HLRProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListHLRProviders(ctx context.Context, in *routingv1.ListHLRProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListHLRProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) GetLookupHistory(ctx context.Context, in *routingv1.GetLookupHistoryRequest, opts ...grpc.CallOption) (*routingv1.GetLookupHistoryResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*routingv1.GetLookupHistoryResponse), args.Error(1)
}
func (m *mockRoutingClientForLookup) RouteMessageWithHLR(ctx context.Context, in *routingv1.RouteMessageWithHLRRequest, opts ...grpc.CallOption) (*routingv1.RouteMessageWithHLRResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) AssignProviderToClient(ctx context.Context, in *routingv1.AssignProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) RevokeProviderFromClient(ctx context.Context, in *routingv1.RevokeProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListClientProviders(ctx context.Context, in *routingv1.ListClientProvidersRequest, opts ...grpc.CallOption) (*routingv1.ListClientProvidersResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateClientProvider(ctx context.Context, in *routingv1.UpdateClientProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ShareProviderWithChild(ctx context.Context, in *routingv1.ShareProviderRequest, opts ...grpc.CallOption) (*routingv1.ClientProviderProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) RevokeSharedProvider(ctx context.Context, in *routingv1.RevokeSharedProviderRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) CreateClientRoute(ctx context.Context, in *routingv1.CreateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) UpdateClientRoute(ctx context.Context, in *routingv1.UpdateClientRouteRequest, opts ...grpc.CallOption) (*routingv1.ClientRouteProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteClientRoute(ctx context.Context, in *routingv1.DeleteClientRouteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) ListClientRoutes(ctx context.Context, in *routingv1.ListClientRoutesRequest, opts ...grpc.CallOption) (*routingv1.ListClientRoutesResponse, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) SetRoutingStrategy(ctx context.Context, in *routingv1.SetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) GetRoutingStrategy(ctx context.Context, in *routingv1.GetRoutingStrategyRequest, opts ...grpc.CallOption) (*routingv1.ClientRoutingStrategyProto, error) {
	return nil, nil
}
func (m *mockRoutingClientForLookup) DeleteRoutingStrategy(ctx context.Context, in *routingv1.DeleteRoutingStrategyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}

// --- Tests ---

func TestLookupHandlers(t *testing.T) {
	t.Run("SingleLookup", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			clientID := uuid.New()

			routingClient.On("NumberLookup", mock.Anything, mock.MatchedBy(func(req *routingv1.NumberLookupRequest) bool {
				return req.Msisdn == "+79001234567" && req.ClientId == clientID.String()
			})).Return(&routingv1.NumberLookupResponse{
				Msisdn:         "+79001234567",
				OperatorMccmnc: "25001",
				OperatorName:   "MTS",
				NumberStatus:   routingv1.NumberStatus_NUMBER_STATUS_ACTIVE,
				CountryCode:    "RU",
				NumberType:     routingv1.NumberType_NUMBER_TYPE_MOBILE,
				IsPorted:       false,
				Cached:         false,
				QueriedAt:      timestamppb.Now(),
			}, nil)

			body, _ := json.Marshal(SingleLookupRequest{
				MSISDN: "+79001234567",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SingleLookup(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, "+79001234567", resp["msisdn"])
			assert.Equal(t, "25001", resp["operator_mccmnc"])
			assert.Equal(t, "MTS", resp["operator_name"])
			assert.Equal(t, "NUMBER_STATUS_ACTIVE", resp["number_status"])
			assert.Equal(t, "RU", resp["country_code"])
			assert.Equal(t, false, resp["is_ported"])

			routingClient.AssertExpectations(t)
		})

		t.Run("returns 400 for invalid body", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader([]byte("invalid")))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.SingleLookup(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns 401 when no client ID", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			body, _ := json.Marshal(SingleLookupRequest{
				MSISDN: "+79001234567",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.SingleLookup(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})

		t.Run("returns 400 when msisdn is empty", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			clientID := uuid.New()

			body, _ := json.Marshal(SingleLookupRequest{
				MSISDN: "",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SingleLookup(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})

		t.Run("returns error when routing service fails", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			clientID := uuid.New()

			routingClient.On("NumberLookup", mock.Anything, mock.Anything).
				Return(nil, status.Error(codes.Internal, "internal error"))

			body, _ := json.Marshal(SingleLookupRequest{
				MSISDN: "+79001234567",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.SingleLookup(rr, req)

			assert.Equal(t, http.StatusInternalServerError, rr.Code)
		})
	})

	t.Run("BulkLookup", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			clientID := uuid.New()

			routingClient.On("BulkNumberLookup", mock.Anything, mock.MatchedBy(func(req *routingv1.BulkNumberLookupRequest) bool {
				return len(req.Msisdns) == 2 && req.ClientId == clientID.String()
			})).Return(&routingv1.BulkNumberLookupResponse{
				Results: []*routingv1.NumberLookupResponse{
					{
						Msisdn:       "+79001234567",
						NumberStatus: routingv1.NumberStatus_NUMBER_STATUS_ACTIVE,
						QueriedAt:    timestamppb.Now(),
					},
					{
						Msisdn:       "+79009876543",
						NumberStatus: routingv1.NumberStatus_NUMBER_STATUS_ABSENT,
						QueriedAt:    timestamppb.Now(),
					},
				},
				TotalCount:   2,
				SuccessCount: 2,
				FailedCount:  0,
			}, nil)

			body, _ := json.Marshal(BulkLookupRequest{
				MSISDNs: []string{"+79001234567", "+79009876543"},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup/bulk", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.BulkLookup(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(rr.Body.Bytes(), &resp)
			require.NoError(t, err)
			results := resp["results"].([]interface{})
			assert.Len(t, results, 2)
			assert.Equal(t, float64(2), resp["total_count"])

			routingClient.AssertExpectations(t)
		})

		t.Run("returns 400 when msisdns list is empty", func(t *testing.T) {
			routingClient := new(mockRoutingClientForLookup)
			handler := NewLookupHandlers(routingClient)

			clientID := uuid.New()

			body, _ := json.Marshal(BulkLookupRequest{
				MSISDNs: []string{},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup/bulk", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(contextWithClientID(req.Context(), clientID))

			rr := httptest.NewRecorder()
			handler.BulkLookup(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	})
}
