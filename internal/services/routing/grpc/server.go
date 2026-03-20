package grpc

import (
	"context"
	"time"

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
	routingService   *application.RoutingService
	countryRepo      domain.CountryRepository
	operatorRepo     domain.OperatorRepository
	prefixRepo       domain.OperatorPrefixRepository
	operatorResolver *application.OperatorResolver
}

// NewServer создает новый gRPC сервер для Routing Service
func NewServer(
	routingService *application.RoutingService,
	countryRepo domain.CountryRepository,
	operatorRepo domain.OperatorRepository,
	prefixRepo domain.OperatorPrefixRepository,
	operatorResolver *application.OperatorResolver,
) *Server {
	return &Server{
		routingService:   routingService,
		countryRepo:      countryRepo,
		operatorRepo:     operatorRepo,
		prefixRepo:       prefixRepo,
		operatorResolver: operatorResolver,
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

// CreateCountry создает новую страну
func (s *Server) CreateCountry(ctx context.Context, req *routingv1.CreateCountryRequest) (*routingv1.Country, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.IsoCode == "" {
		return nil, status.Error(codes.InvalidArgument, "iso_code is required")
	}
	if req.PhoneCode == "" {
		return nil, status.Error(codes.InvalidArgument, "phone_code is required")
	}
	if req.Currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}

	// Проверяем уникальность ISO-кода
	existing, err := s.countryRepo.GetByISOCode(ctx, req.IsoCode)
	if err == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "country with this ISO code already exists")
	}

	country := domain.NewCountry(req.Name, req.IsoCode, req.PhoneCode, req.Currency)
	if err := s.countryRepo.Create(ctx, country); err != nil {
		log.Error().Err(err).Msg("ошибка создания страны")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return countryToProto(country), nil
}

// GetCountry получает страну по ID
func (s *Server) GetCountry(ctx context.Context, req *routingv1.GetCountryRequest) (*routingv1.Country, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	country, err := s.countryRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrCountryNotFound {
			return nil, status.Error(codes.NotFound, "country not found")
		}
		log.Error().Err(err).Msg("ошибка получения страны")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return countryToProto(country), nil
}

// ListCountries получает список стран
func (s *Server) ListCountries(ctx context.Context, req *routingv1.ListCountriesRequest) (*routingv1.ListCountriesResponse, error) {
	limit := 100
	if req.Limit > 0 {
		limit = int(req.Limit)
	}
	offset := int(req.Offset)

	countries, total, err := s.countryRepo.List(ctx, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка стран")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoCountries := make([]*routingv1.Country, len(countries))
	for i, country := range countries {
		protoCountries[i] = countryToProto(country)
	}

	return &routingv1.ListCountriesResponse{
		Countries: protoCountries,
		Total:     int32(total),
	}, nil
}

// UpdateCountry обновляет страну
func (s *Server) UpdateCountry(ctx context.Context, req *routingv1.UpdateCountryRequest) (*routingv1.Country, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	country, err := s.countryRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrCountryNotFound {
			return nil, status.Error(codes.NotFound, "country not found")
		}
		log.Error().Err(err).Msg("ошибка получения страны")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if req.Name != "" {
		country.Name = req.Name
	}
	if req.IsoCode != "" {
		country.ISOCode = req.IsoCode
	}
	if req.PhoneCode != "" {
		country.PhoneCode = req.PhoneCode
	}
	if req.Currency != "" {
		country.Currency = req.Currency
	}
	country.UpdatedAt = time.Now()

	if err := s.countryRepo.Update(ctx, country); err != nil {
		log.Error().Err(err).Msg("ошибка обновления страны")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return countryToProto(country), nil
}

// CreateOperator создает нового оператора
func (s *Server) CreateOperator(ctx context.Context, req *routingv1.CreateOperatorRequest) (*routingv1.Operator, error) {
	if req.CountryId == "" {
		return nil, status.Error(codes.InvalidArgument, "country_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	countryID, err := uuid.Parse(req.CountryId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid country_id format")
	}

	// Проверяем существование страны
	_, err = s.countryRepo.GetByID(ctx, countryID)
	if err != nil {
		if err == domain.ErrCountryNotFound {
			return nil, status.Error(codes.NotFound, "country not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Проверяем уникальность кода
	existing, err := s.operatorRepo.GetByCode(ctx, req.Code)
	if err == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "operator with this code already exists")
	}

	operator := domain.NewOperator(countryID, req.Name, req.Code, req.SupportsPaidSender, req.SupportsFreeSender)
	if err := s.operatorRepo.Create(ctx, operator); err != nil {
		log.Error().Err(err).Msg("ошибка создания оператора")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return operatorToProto(operator), nil
}

// GetOperator получает оператора по ID
func (s *Server) GetOperator(ctx context.Context, req *routingv1.GetOperatorRequest) (*routingv1.Operator, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	operator, err := s.operatorRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrOperatorNotFound {
			return nil, status.Error(codes.NotFound, "operator not found")
		}
		log.Error().Err(err).Msg("ошибка получения оператора")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return operatorToProto(operator), nil
}

// ListOperators получает список операторов
func (s *Server) ListOperators(ctx context.Context, req *routingv1.ListOperatorsRequest) (*routingv1.ListOperatorsResponse, error) {
	limit := 100
	if req.Limit > 0 {
		limit = int(req.Limit)
	}
	offset := int(req.Offset)

	var countryID *uuid.UUID
	if req.CountryId != "" {
		parsed, err := uuid.Parse(req.CountryId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid country_id format")
		}
		countryID = &parsed
	}

	operators, total, err := s.operatorRepo.List(ctx, countryID, req.ActiveOnly, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка операторов")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoOperators := make([]*routingv1.Operator, len(operators))
	for i, operator := range operators {
		protoOperators[i] = operatorToProto(operator)
	}

	return &routingv1.ListOperatorsResponse{
		Operators: protoOperators,
		Total:     int32(total),
	}, nil
}

// UpdateOperator обновляет оператора
func (s *Server) UpdateOperator(ctx context.Context, req *routingv1.UpdateOperatorRequest) (*routingv1.Operator, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	operator, err := s.operatorRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrOperatorNotFound {
			return nil, status.Error(codes.NotFound, "operator not found")
		}
		log.Error().Err(err).Msg("ошибка получения оператора")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if req.Name != "" {
		operator.Name = req.Name
	}
	if req.Code != "" {
		operator.Code = req.Code
	}
	operator.SupportsPaidSender = req.SupportsPaidSender
	operator.SupportsFreeSender = req.SupportsFreeSender
	operator.Active = req.Active
	operator.UpdatedAt = time.Now()

	if err := s.operatorRepo.Update(ctx, operator); err != nil {
		log.Error().Err(err).Msg("ошибка обновления оператора")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return operatorToProto(operator), nil
}

// CreateOperatorPrefix создает новый префикс оператора
func (s *Server) CreateOperatorPrefix(ctx context.Context, req *routingv1.CreateOperatorPrefixRequest) (*routingv1.OperatorPrefix, error) {
	if req.OperatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "operator_id is required")
	}
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operator_id format")
	}

	// Проверяем существование оператора
	_, err = s.operatorRepo.GetByID(ctx, operatorID)
	if err != nil {
		if err == domain.ErrOperatorNotFound {
			return nil, status.Error(codes.NotFound, "operator not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	prefix := domain.NewOperatorPrefix(operatorID, req.Prefix, int(req.Priority))
	if err := s.prefixRepo.Create(ctx, prefix); err != nil {
		log.Error().Err(err).Msg("ошибка создания префикса")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return operatorPrefixToProto(prefix), nil
}

// ListOperatorPrefixes получает список префиксов оператора
func (s *Server) ListOperatorPrefixes(ctx context.Context, req *routingv1.ListOperatorPrefixesRequest) (*routingv1.ListOperatorPrefixesResponse, error) {
	if req.OperatorId == "" {
		return nil, status.Error(codes.InvalidArgument, "operator_id is required")
	}

	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operator_id format")
	}

	prefixes, err := s.prefixRepo.ListByOperatorID(ctx, operatorID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка префиксов")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoPrefixes := make([]*routingv1.OperatorPrefix, len(prefixes))
	for i, prefix := range prefixes {
		protoPrefixes[i] = operatorPrefixToProto(prefix)
	}

	return &routingv1.ListOperatorPrefixesResponse{
		Prefixes: protoPrefixes,
	}, nil
}

// DeleteOperatorPrefix удаляет префикс оператора
func (s *Server) DeleteOperatorPrefix(ctx context.Context, req *routingv1.DeleteOperatorPrefixRequest) (*routingv1.DeleteOperatorPrefixResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	if err := s.prefixRepo.Delete(ctx, id); err != nil {
		if err == domain.ErrPrefixNotFound {
			return nil, status.Error(codes.NotFound, "prefix not found")
		}
		log.Error().Err(err).Msg("ошибка удаления префикса")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.DeleteOperatorPrefixResponse{
		Success: true,
	}, nil
}

// ResolveOperator определяет оператора по номеру телефона
func (s *Server) ResolveOperator(ctx context.Context, req *routingv1.ResolveOperatorRequest) (*routingv1.ResolveOperatorResponse, error) {
	if req.PhoneNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "phone_number is required")
	}

	result, err := s.operatorResolver.ResolveByNumber(ctx, req.PhoneNumber)
	if err != nil {
		if err == domain.ErrPrefixNotFound {
			return nil, status.Error(codes.NotFound, "operator not found for this number")
		}
		log.Error().Err(err).Str("phone_number", req.PhoneNumber).Msg("ошибка определения оператора")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.ResolveOperatorResponse{
		OperatorId:   result.OperatorID.String(),
		CountryId:    result.CountryID.String(),
		OperatorCode: result.OperatorCode,
		CountryCode:  result.CountryCode,
		Currency:     result.Currency,
		ResolvedBy:   result.ResolvedBy,
	}, nil
}

// routeToProto преобразует domain.Route в proto.RouteInfo
func routeToProto(route *domain.Route) *routingv1.RouteInfo {
	providerIDs := make([]string, len(route.ProviderIDs))
	for i, id := range route.ProviderIDs {
		providerIDs[i] = id.String()
	}

	return &routingv1.RouteInfo{
		RouteId:             route.ID.String(),
		Name:                route.Name,
		Pattern:             route.Pattern,
		Priority:            int32(route.Priority),
		ProviderIds:         providerIDs,
		LoadBalanceStrategy: string(route.LoadBalanceStrategy),
		FailoverEnabled:     route.FailoverEnabled,
		Active:              route.Active,
		Metadata:            route.Metadata,
		CreatedAt:           timestamppb.New(route.CreatedAt),
		UpdatedAt:           timestamppb.New(route.UpdatedAt),
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
		ID:                  routeID,
		Name:                proto.Name,
		Pattern:             proto.Pattern,
		PatternType:         domain.PatternTypeFromString("regex"),
		ProviderIDs:         providerIDs,
		Priority:            int(proto.Priority),
		Active:              proto.Active,
		FailoverEnabled:     proto.FailoverEnabled,
		LoadBalanceStrategy: domain.LoadBalanceStrategyFromString(proto.LoadBalanceStrategy),
		Metadata:            proto.Metadata,
	}
}

// countryToProto преобразует domain.Country в proto.Country
func countryToProto(country *domain.Country) *routingv1.Country {
	return &routingv1.Country{
		Id:        country.ID.String(),
		Name:      country.Name,
		IsoCode:   country.ISOCode,
		PhoneCode: country.PhoneCode,
		Currency:  country.Currency,
		CreatedAt: timestamppb.New(country.CreatedAt),
		UpdatedAt: timestamppb.New(country.UpdatedAt),
	}
}

// operatorToProto преобразует domain.Operator в proto.Operator
func operatorToProto(operator *domain.Operator) *routingv1.Operator {
	return &routingv1.Operator{
		Id:                 operator.ID.String(),
		CountryId:          operator.CountryID.String(),
		Name:               operator.Name,
		Code:               operator.Code,
		SupportsPaidSender: operator.SupportsPaidSender,
		SupportsFreeSender: operator.SupportsFreeSender,
		Active:             operator.Active,
		CreatedAt:          timestamppb.New(operator.CreatedAt),
		UpdatedAt:          timestamppb.New(operator.UpdatedAt),
	}
}

// operatorPrefixToProto преобразует domain.OperatorPrefix в proto.OperatorPrefix
func operatorPrefixToProto(prefix *domain.OperatorPrefix) *routingv1.OperatorPrefix {
	return &routingv1.OperatorPrefix{
		Id:         prefix.ID.String(),
		OperatorId: prefix.OperatorID.String(),
		Prefix:     prefix.Prefix,
		Priority:   int32(prefix.Priority),
		CreatedAt:  timestamppb.New(prefix.CreatedAt),
	}
}
