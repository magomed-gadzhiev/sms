package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	cpv1 "github.com/smpp-server/smpp-server/api/proto/clientproviderv1"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// ClientProviderServer реализует gRPC сервис для client-провайдеров
type ClientProviderServer struct {
	cpv1.UnimplementedClientProviderServiceServer
	clientProviderService *application.ClientProviderService
	testConnectionService *application.TestConnectionService
}

// NewClientProviderServer создаёт новый gRPC сервер для client-провайдеров
func NewClientProviderServer(
	clientProviderService *application.ClientProviderService,
) *ClientProviderServer {
	return &ClientProviderServer{
		clientProviderService: clientProviderService,
		testConnectionService: application.NewTestConnectionService(),
	}
}

func (s *ClientProviderServer) CreateClientProvider(ctx context.Context, req *cpv1.CreateClientProviderRequest) (*cpv1.ClientProvider, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный client_id")
	}

	p, err := s.clientProviderService.Create(ctx, clientID, &domain.Provider{
		Name: req.Name, Description: req.Description, Tags: req.Tags,
		Host: req.Host, Port: int(req.Port),
		SystemID: req.SystemId, Password: req.Password,
		BindType:       cpProtoBindTypeToDomain(req.BindType),
		WindowSize:     int(req.WindowSize),
		MaxConnections: int(req.MaxConnections),
		TPSLimit:       int(req.TpsLimit),
		RoutingRules:   cpProtoRulesToDomain(req.RoutingRules),
	})
	if err != nil {
		if errors.Is(err, domain.ErrProviderLimitExceeded) {
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return domainToClientProviderProto(p), nil
}

func (s *ClientProviderServer) ListClientProviders(ctx context.Context, req *cpv1.ListClientProvidersRequest) (*cpv1.ListClientProvidersResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный client_id")
	}
	providers, err := s.clientProviderService.List(ctx, clientID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	result := make([]*cpv1.ClientProvider, len(providers))
	for i, p := range providers {
		result[i] = domainToClientProviderProto(p)
	}
	return &cpv1.ListClientProvidersResponse{Providers: result}, nil
}

func (s *ClientProviderServer) GetClientProvider(ctx context.Context, req *cpv1.GetClientProviderRequest) (*cpv1.ClientProvider, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный id")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный client_id")
	}
	p, err := s.clientProviderService.Get(ctx, id, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "провайдер не найден")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return domainToClientProviderProto(p), nil
}

func (s *ClientProviderServer) UpdateClientProvider(ctx context.Context, req *cpv1.UpdateClientProviderRequest) (*cpv1.ClientProvider, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный id")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный client_id")
	}
	updates := &domain.Provider{
		Name: req.Name, Description: req.Description, Tags: req.Tags,
		Host: req.Host, Port: int(req.Port),
		SystemID: req.SystemId, Password: req.Password,
		BindType:       cpProtoBindTypeToDomain(req.BindType),
		WindowSize:     int(req.WindowSize),
		MaxConnections: int(req.MaxConnections),
		TPSLimit:       int(req.TpsLimit),
		RoutingRules:   cpProtoRulesToDomain(req.RoutingRules),
	}
	p, err := s.clientProviderService.Update(ctx, id, clientID, updates)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "провайдер не найден")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return domainToClientProviderProto(p), nil
}

func (s *ClientProviderServer) DeleteClientProvider(ctx context.Context, req *cpv1.DeleteClientProviderRequest) (*emptypb.Empty, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный id")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный client_id")
	}
	if err := s.clientProviderService.Delete(ctx, id, clientID); err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "провайдер не найден")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *ClientProviderServer) TestClientProviderConnection(ctx context.Context, req *cpv1.TestConnectionRequest) (*cpv1.TestConnectionResponse, error) {
	bindType := "transceiver"
	switch req.BindType {
	case 1:
		bindType = "transmitter"
	case 2:
		bindType = "receiver"
	}
	result := s.testConnectionService.Test(ctx, application.TestConnectionConfig{
		Host: req.Host, Port: int(req.Port),
		SystemID: req.SystemId, Password: req.Password,
		BindType: bindType,
	})
	return &cpv1.TestConnectionResponse{
		Success: result.Success, LatencyMs: result.LatencyMs,
		Log: result.Log, Error: result.Error,
	}, nil
}

// ---- helpers ----

func cpProtoBindTypeToDomain(t int32) domain.BindType {
	switch t {
	case 1:
		return domain.BindTypeTransmitter
	case 2:
		return domain.BindTypeReceiver
	default:
		return domain.BindTypeTransceiver
	}
}

func cpProtoRulesToDomain(rules []*cpv1.RoutingRule) []domain.RoutingRule {
	result := make([]domain.RoutingRule, len(rules))
	for i, r := range rules {
		result[i] = domain.RoutingRule{Pattern: r.Pattern, Priority: int(r.Priority)}
	}
	return result
}

func domainToClientProviderProto(p *domain.Provider) *cpv1.ClientProvider {
	cp := &cpv1.ClientProvider{
		Id: p.ID.String(), Name: p.Name, Description: p.Description,
		Tags: p.Tags, Host: p.Host, Port: int32(p.Port),
		SystemId: p.SystemID, WindowSize: int32(p.WindowSize),
		MaxConnections: int32(p.MaxConnections), TpsLimit: int32(p.TPSLimit),
		Active:    p.Active,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
	if p.ClientID != nil {
		cp.ClientId = p.ClientID.String()
	}
	cp.RoutingRules = make([]*cpv1.RoutingRule, len(p.RoutingRules))
	for i, r := range p.RoutingRules {
		cp.RoutingRules[i] = &cpv1.RoutingRule{Pattern: r.Pattern, Priority: int32(r.Priority)}
	}
	switch p.BindType {
	case domain.BindTypeTransmitter:
		cp.BindType = 1
	case domain.BindTypeReceiver:
		cp.BindType = 2
	default:
		cp.BindType = 0
	}
	return cp
}
