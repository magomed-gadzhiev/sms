package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/services/routing/application"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// Server реализует gRPC сервис для маршрутизации
type Server struct {
	routingv1.UnimplementedRoutingServiceServer
	routingService *application.RoutingService
}

// NewServer создает новый gRPC сервер для Routing Service
func NewServer(routingService *application.RoutingService) *Server {
	return &Server{
		routingService: routingService,
	}
}

// GetRoute получает маршрут для сообщения
func (s *Server) GetRoute(ctx context.Context, req *routingv1.GetRouteRequest) (*routingv1.GetRouteResponse, error) {
	if req.Destination == "" {
		return nil, status.Error(codes.InvalidArgument, "destination is required")
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsed, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsed
	}

	route, err := s.routingService.GetRoute(ctx, req.Destination, clientID)
	if err != nil {
		if err == domain.ErrNoMatchingRoute {
			return nil, status.Error(codes.NotFound, "no matching route found")
		}
		log.Error().Err(err).Msg("ошибка получения маршрута")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.GetRouteResponse{
		Route: routeToProto(route),
	}, nil
}

// SelectProvider выбирает оптимального провайдера для маршрута
func (s *Server) SelectProvider(ctx context.Context, req *routingv1.SelectProviderRequest) (*routingv1.SelectProviderResponse, error) {
	if req.Route == nil {
		return nil, status.Error(codes.InvalidArgument, "route is required")
	}

	route := protoToRoute(req.Route)
	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsed, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsed
	}

	providerID, err := s.routingService.SelectProvider(ctx, route, clientID)
	if err != nil {
		if err == domain.ErrNoProviders {
			return nil, status.Error(codes.NotFound, "no available providers")
		}
		log.Error().Err(err).Msg("ошибка выбора провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.SelectProviderResponse{
		ProviderId:      providerID.String(),
		Strategy:        string(route.LoadBalanceStrategy),
		BackupProviders: []string{}, // TODO: добавить поддержку backup провайдеров
	}, nil
}

// CreateRoute создает новое правило маршрутизации
func (s *Server) CreateRoute(ctx context.Context, req *routingv1.CreateRouteRequest) (*routingv1.CreateRouteResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Pattern == "" {
		return nil, status.Error(codes.InvalidArgument, "pattern is required")
	}
	if len(req.ProviderIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one provider_id is required")
	}

	providerIDs := make([]uuid.UUID, len(req.ProviderIds))
	for i, idStr := range req.ProviderIds {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid provider_id format: "+idStr)
		}
		providerIDs[i] = id
	}

	patternType := domain.PatternTypeFromString("regex") // По умолчанию regex
	if req.Pattern != "" && req.Pattern[0] == '+' {
		patternType = domain.PatternTypePrefix
	}

	strategy := domain.LoadBalanceStrategyFromString(req.LoadBalanceStrategy)
	if strategy == "" {
		strategy = domain.LoadBalanceRoundRobin
	}

	route, err := s.routingService.CreateRoute(
		ctx,
		req.Name,
		req.Pattern,
		patternType,
		providerIDs,
		int(req.Priority),
		strategy,
		req.FailoverEnabled,
		req.Metadata,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания маршрута")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.CreateRouteResponse{
		RouteId:   route.ID.String(),
		CreatedAt: timestamppb.New(route.CreatedAt),
	}, nil
}

// UpdateRoute обновляет правило маршрутизации
func (s *Server) UpdateRoute(ctx context.Context, req *routingv1.UpdateRouteRequest) (*routingv1.UpdateRouteResponse, error) {
	if req.RouteId == "" {
		return nil, status.Error(codes.InvalidArgument, "route_id is required")
	}

	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid route_id format")
	}

	updates := &application.RouteUpdate{}

	if req.Name != "" {
		updates.Name = &req.Name
	}
	if req.Pattern != "" {
		updates.Pattern = &req.Pattern
		patternType := domain.PatternTypeFromString("regex")
		if req.Pattern[0] == '+' {
			patternType = domain.PatternTypePrefix
		}
		updates.PatternType = &patternType
	}
	if req.Priority != 0 {
		priority := int(req.Priority)
		updates.Priority = &priority
	}
	if len(req.ProviderIds) > 0 {
		providerIDs := make([]uuid.UUID, len(req.ProviderIds))
		for i, idStr := range req.ProviderIds {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid provider_id format: "+idStr)
			}
			providerIDs[i] = id
		}
		updates.ProviderIDs = providerIDs
	}
	if req.LoadBalanceStrategy != "" {
		strategy := domain.LoadBalanceStrategyFromString(req.LoadBalanceStrategy)
		updates.Strategy = &strategy
	}
	if req.FailoverEnabled {
		updates.FailoverEnabled = &req.FailoverEnabled
	}
	if req.Active {
		updates.Active = &req.Active
	}
	if req.Metadata != nil {
		updates.Metadata = req.Metadata
	}

	err = s.routingService.UpdateRoute(ctx, routeID, updates)
	if err != nil {
		if err == domain.ErrRouteNotFound {
			return nil, status.Error(codes.NotFound, "route not found")
		}
		log.Error().Err(err).Msg("ошибка обновления маршрута")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.UpdateRouteResponse{
		Success: true,
	}, nil
}

// DeleteRoute удаляет правило маршрутизации
func (s *Server) DeleteRoute(ctx context.Context, req *routingv1.DeleteRouteRequest) (*routingv1.DeleteRouteResponse, error) {
	if req.RouteId == "" {
		return nil, status.Error(codes.InvalidArgument, "route_id is required")
	}

	routeID, err := uuid.Parse(req.RouteId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid route_id format")
	}

	err = s.routingService.DeleteRoute(ctx, routeID)
	if err != nil {
		if err == domain.ErrRouteNotFound {
			return nil, status.Error(codes.NotFound, "route not found")
		}
		log.Error().Err(err).Msg("ошибка удаления маршрута")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.DeleteRouteResponse{
		Success: true,
	}, nil
}

// ListRoutes получает список правил маршрутизации
func (s *Server) ListRoutes(ctx context.Context, req *routingv1.ListRoutesRequest) (*routingv1.ListRoutesResponse, error) {
	limit := 100
	if req.Limit > 0 {
		limit = int(req.Limit)
	}
	offset := int(req.Offset)

	routes, total, err := s.routingService.ListRoutes(ctx, req.ActiveOnly, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка маршрутов")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoRoutes := make([]*routingv1.RouteInfo, len(routes))
	for i, route := range routes {
		protoRoutes[i] = routeToProto(route)
	}

	return &routingv1.ListRoutesResponse{
		Routes: protoRoutes,
		Total:  int32(total),
	}, nil
}

// routeToProto преобразует domain.Route в proto.RouteInfo
func routeToProto(route *domain.Route) *routingv1.RouteInfo {
	providerIDs := make([]string, len(route.ProviderIDs))
	for i, id := range route.ProviderIDs {
		providerIDs[i] = id.String()
	}

	return &routingv1.RouteInfo{
		RouteId:            route.ID.String(),
		Name:               route.Name,
		Pattern:            route.Pattern,
		Priority:           int32(route.Priority),
		ProviderIds:        providerIDs,
		LoadBalanceStrategy: string(route.LoadBalanceStrategy),
		FailoverEnabled:    route.FailoverEnabled,
		Active:             route.Active,
		Metadata:           route.Metadata,
		CreatedAt:          timestamppb.New(route.CreatedAt),
		UpdatedAt:          timestamppb.New(route.UpdatedAt),
	}
}

// protoToRoute преобразует proto.RouteInfo в domain.Route
func protoToRoute(proto *routingv1.RouteInfo) *domain.Route {
	routeID, _ := uuid.Parse(proto.RouteId)
	providerIDs := make([]uuid.UUID, len(proto.ProviderIds))
	for i, idStr := range proto.ProviderIds {
		id, _ := uuid.Parse(idStr)
		providerIDs[i] = id
	}

	return &domain.Route{
		ID:                routeID,
		Name:              proto.Name,
		Pattern:           proto.Pattern,
		PatternType:       domain.PatternTypeFromString("regex"),
		ProviderIDs:       providerIDs,
		Priority:          int(proto.Priority),
		Active:            proto.Active,
		FailoverEnabled:   proto.FailoverEnabled,
		LoadBalanceStrategy: domain.LoadBalanceStrategyFromString(proto.LoadBalanceStrategy),
		Metadata:          proto.Metadata,
	}
}