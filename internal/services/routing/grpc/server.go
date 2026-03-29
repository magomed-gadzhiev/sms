package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
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
	hlrService       *application.HLRService
	smartRouter      *application.SmartRoutingService
	hlrProviderRepo    domain.HLRProviderRepository
	lookupLogRepo      domain.LookupLogRepository
	clientProviderRepo domain.ClientProviderRepository
	clientRouteRepo    domain.ClientRouteRepository
	clientStrategyRepo domain.ClientRoutingStrategyRepository
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

// SetHLRService устанавливает HLR-сервис
func (s *Server) SetHLRService(hlr *application.HLRService) {
	s.hlrService = hlr
}

// SetSmartRouter устанавливает сервис smart routing
func (s *Server) SetSmartRouter(sr *application.SmartRoutingService) {
	s.smartRouter = sr
}

// SetHLRProviderRepo устанавливает репозиторий HLR-провайдеров
func (s *Server) SetHLRProviderRepo(repo domain.HLRProviderRepository) {
	s.hlrProviderRepo = repo
}

// SetLookupLogRepo устанавливает репозиторий логов lookup
func (s *Server) SetLookupLogRepo(repo domain.LookupLogRepository) {
	s.lookupLogRepo = repo
}

// ==================== Существующие методы маршрутизации ====================

// SetClientRoutingDeps устанавливает зависимости для клиентской маршрутизации
func (s *Server) SetClientRoutingDeps(
	cpRepo domain.ClientProviderRepository,
	crRepo domain.ClientRouteRepository,
	csRepo domain.ClientRoutingStrategyRepository,
) {
	s.clientProviderRepo = cpRepo
	s.clientRouteRepo = crRepo
	s.clientStrategyRepo = csRepo
}

// SetHLRDependencies устанавливает зависимости для HLR/MNP операций
func (s *Server) SetHLRDependencies(
	hlrService *application.HLRService,
	hlrProviderRepo domain.HLRProviderRepository,
	lookupLogRepo domain.LookupLogRepository,
	smartRouter *application.SmartRoutingService,
) {
	s.hlrService = hlrService
	s.hlrProviderRepo = hlrProviderRepo
	s.lookupLogRepo = lookupLogRepo
	s.smartRouter = smartRouter
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

// ==================== Управление странами ====================

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

// ==================== Управление операторами ====================

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

// ==================== Управление префиксами операторов ====================

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

// ==================== HLR Number Lookup ====================

// NumberLookup выполняет HLR-запрос для проверки номера
func (s *Server) NumberLookup(ctx context.Context, req *routingv1.NumberLookupRequest) (*routingv1.NumberLookupResponse, error) {
	if s.hlrService == nil {
		return nil, status.Error(codes.Unavailable, "HLR сервис не сконфигурирован")
	}
	if req.Msisdn == "" {
		return nil, status.Error(codes.InvalidArgument, "msisdn is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	result, err := s.hlrService.LookupNumber(
		ctx, req.Msisdn, req.ForceRefresh, clientID,
		req.RequestId, domain.LookupSourceAPILookup, nil,
	)
	if err != nil {
		if err == domain.ErrInvalidMSISDN {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if err == domain.ErrHLRProviderUnavailable || err == domain.ErrHLRLookupFailed {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		log.Error().Err(err).Str("msisdn", req.Msisdn).Msg("ошибка HLR lookup")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if result == nil {
		return nil, status.Error(codes.NotFound, "номер не подходит для HLR lookup (short code)")
	}

	return lookupResultToProto(result), nil
}

// BulkNumberLookup выполняет массовый HLR-запрос для проверки номеров
func (s *Server) BulkNumberLookup(ctx context.Context, req *routingv1.BulkNumberLookupRequest) (*routingv1.BulkNumberLookupResponse, error) {
	if s.hlrService == nil {
		return nil, status.Error(codes.Unavailable, "HLR сервис не сконфигурирован")
	}
	if len(req.Msisdns) == 0 {
		return nil, status.Error(codes.InvalidArgument, "msisdns is required")
	}
	if len(req.Msisdns) > 1000 {
		return nil, status.Error(codes.InvalidArgument, domain.ErrBulkLookupTooLarge.Error())
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	var results []*routingv1.NumberLookupResponse
	var successCount, failedCount int32

	for _, msisdn := range req.Msisdns {
		result, err := s.hlrService.LookupNumber(
			ctx, msisdn, req.ForceRefresh, clientID,
			req.RequestId, domain.LookupSourceAPILookup, nil,
		)
		if err != nil {
			failedCount++
			continue
		}
		if result == nil {
			failedCount++
			continue
		}
		results = append(results, lookupResultToProto(result))
		successCount++
	}

	return &routingv1.BulkNumberLookupResponse{
		Results:      results,
		TotalCount:   int32(len(req.Msisdns)),
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

// ==================== Управление HLR-провайдерами ====================

// CreateHLRProvider создает нового HLR-провайдера
func (s *Server) CreateHLRProvider(ctx context.Context, req *routingv1.CreateHLRProviderRequest) (*routingv1.HLRProviderProto, error) {
	if s.hlrProviderRepo == nil {
		return nil, status.Error(codes.Unavailable, "HLR провайдер репозиторий не сконфигурирован")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.AdapterType == "" {
		return nil, status.Error(codes.InvalidArgument, "adapter_type is required")
	}

	// Парсим конфигурацию из JSON
	var config map[string]interface{}
	if req.ConfigJson != "" {
		if err := json.Unmarshal([]byte(req.ConfigJson), &config); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid config_json format: "+err.Error())
		}
	}

	// Парсим стоимость
	var costPerLookup float64
	if req.CostPerLookup != "" {
		var err error
		costPerLookup, err = strconv.ParseFloat(req.CostPerLookup, 64)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid cost_per_lookup format")
		}
	}

	provider := domain.NewHLRProvider(
		req.Name,
		req.AdapterType,
		config,
		int(req.Priority),
		req.SupportedRegions,
		costPerLookup,
	)

	if err := s.hlrProviderRepo.Create(ctx, provider); err != nil {
		log.Error().Err(err).Msg("ошибка создания HLR-провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return hlrProviderToProto(provider), nil
}

// UpdateHLRProvider обновляет HLR-провайдера
func (s *Server) UpdateHLRProvider(ctx context.Context, req *routingv1.UpdateHLRProviderRequest) (*routingv1.HLRProviderProto, error) {
	if s.hlrProviderRepo == nil {
		return nil, status.Error(codes.Unavailable, "HLR провайдер репозиторий не сконфигурирован")
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	provider, err := s.hlrProviderRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrHLRProviderNotFound {
			return nil, status.Error(codes.NotFound, "HLR провайдер не найден")
		}
		log.Error().Err(err).Msg("ошибка получения HLR-провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if req.Name != "" {
		provider.Name = req.Name
	}
	if req.AdapterType != "" {
		provider.AdapterType = req.AdapterType
	}
	if req.ConfigJson != "" {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(req.ConfigJson), &config); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid config_json format: "+err.Error())
		}
		provider.Config = config
	}
	if req.Priority != 0 {
		provider.Priority = int(req.Priority)
	}
	if len(req.SupportedRegions) > 0 {
		provider.SupportedRegions = req.SupportedRegions
	}
	if req.CostPerLookup != "" {
		cost, err := strconv.ParseFloat(req.CostPerLookup, 64)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid cost_per_lookup format")
		}
		provider.CostPerLookup = cost
	}
	provider.Active = req.Active
	provider.UpdatedAt = time.Now()

	if err := s.hlrProviderRepo.Update(ctx, provider); err != nil {
		log.Error().Err(err).Msg("ошибка обновления HLR-провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return hlrProviderToProto(provider), nil
}

// DeleteHLRProvider удаляет HLR-провайдера (soft delete)
func (s *Server) DeleteHLRProvider(ctx context.Context, req *routingv1.DeleteHLRProviderRequest) (*routingv1.DeleteRouteResponse, error) {
	if s.hlrProviderRepo == nil {
		return nil, status.Error(codes.Unavailable, "HLR провайдер репозиторий не сконфигурирован")
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	if err := s.hlrProviderRepo.Delete(ctx, id); err != nil {
		if err == domain.ErrHLRProviderNotFound {
			return nil, status.Error(codes.NotFound, "HLR провайдер не найден")
		}
		log.Error().Err(err).Msg("ошибка удаления HLR-провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.DeleteRouteResponse{
		Success: true,
	}, nil
}

// GetHLRProvider получает HLR-провайдера по ID
func (s *Server) GetHLRProvider(ctx context.Context, req *routingv1.GetHLRProviderRequest) (*routingv1.HLRProviderProto, error) {
	if s.hlrProviderRepo == nil {
		return nil, status.Error(codes.Unavailable, "HLR провайдер репозиторий не сконфигурирован")
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	provider, err := s.hlrProviderRepo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrHLRProviderNotFound {
			return nil, status.Error(codes.NotFound, "HLR провайдер не найден")
		}
		log.Error().Err(err).Msg("ошибка получения HLR-провайдера")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return hlrProviderToProto(provider), nil
}

// ListHLRProviders получает список HLR-провайдеров
func (s *Server) ListHLRProviders(ctx context.Context, req *routingv1.ListHLRProvidersRequest) (*routingv1.ListHLRProvidersResponse, error) {
	if s.hlrProviderRepo == nil {
		return nil, status.Error(codes.Unavailable, "HLR провайдер репозиторий не сконфигурирован")
	}

	var providers []*domain.HLRProvider
	var err error

	if req.ActiveOnly {
		providers, err = s.hlrProviderRepo.ListActive(ctx)
	} else {
		// ListActive возвращает только активных; для полного списка получаем все через GetByPriority с пустым кодом
		// Используем ListActive как fallback, т.к. нет метода ListAll в интерфейсе
		providers, err = s.hlrProviderRepo.ListActive(ctx)
	}
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка HLR-провайдеров")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoProviders := make([]*routingv1.HLRProviderProto, len(providers))
	for i, provider := range providers {
		protoProviders[i] = hlrProviderToProto(provider)
	}

	return &routingv1.ListHLRProvidersResponse{
		Providers: protoProviders,
	}, nil
}

// ==================== Smart Route Weights ====================

// SetSmartRouteWeights создает или обновляет веса умной маршрутизации
func (s *Server) SetSmartRouteWeights(ctx context.Context, req *routingv1.SetSmartRouteWeightsRequest) (*routingv1.SmartRouteWeightProto, error) {
	if s.smartRouter == nil {
		return nil, status.Error(codes.Unavailable, "smart routing сервис не сконфигурирован")
	}
	if req.OperatorCode == "" {
		return nil, status.Error(codes.InvalidArgument, "operator_code is required")
	}
	if req.CountryCode == "" {
		return nil, status.Error(codes.InvalidArgument, "country_code is required")
	}

	costWeight, err := strconv.ParseFloat(req.CostWeight, 64)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid cost_weight format")
	}

	qualityWeight, err := strconv.ParseFloat(req.QualityWeight, 64)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid quality_weight format")
	}

	weight, err := s.smartRouter.SetWeights(ctx, req.OperatorCode, req.CountryCode, costWeight, qualityWeight)
	if err != nil {
		log.Error().Err(err).Msg("ошибка установки весов smart routing")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return smartRouteWeightToProto(weight), nil
}

// GetSmartRouteWeights получает веса умной маршрутизации
func (s *Server) GetSmartRouteWeights(ctx context.Context, req *routingv1.GetSmartRouteWeightsRequest) (*routingv1.SmartRouteWeightProto, error) {
	if s.smartRouter == nil {
		return nil, status.Error(codes.Unavailable, "smart routing сервис не сконфигурирован")
	}
	if req.OperatorCode == "" {
		return nil, status.Error(codes.InvalidArgument, "operator_code is required")
	}
	if req.CountryCode == "" {
		return nil, status.Error(codes.InvalidArgument, "country_code is required")
	}

	weight, err := s.smartRouter.GetWeights(ctx, req.OperatorCode, req.CountryCode)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения весов smart routing")
		return nil, status.Error(codes.NotFound, "smart route weights not found")
	}

	return smartRouteWeightToProto(weight), nil
}

// ListSmartRouteWeights получает список весов умной маршрутизации
func (s *Server) ListSmartRouteWeights(ctx context.Context, req *routingv1.ListSmartRouteWeightsRequest) (*routingv1.ListSmartRouteWeightsResponse, error) {
	if s.smartRouter == nil {
		return nil, status.Error(codes.Unavailable, "smart routing сервис не сконфигурирован")
	}

	weights, err := s.smartRouter.ListWeights(ctx, req.CountryCode)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка весов smart routing")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoWeights := make([]*routingv1.SmartRouteWeightProto, len(weights))
	for i, weight := range weights {
		protoWeights[i] = smartRouteWeightToProto(weight)
	}

	return &routingv1.ListSmartRouteWeightsResponse{
		Weights: protoWeights,
	}, nil
}

// DeleteSmartRouteWeights удаляет веса умной маршрутизации
func (s *Server) DeleteSmartRouteWeights(ctx context.Context, req *routingv1.DeleteSmartRouteWeightsRequest) (*routingv1.DeleteRouteResponse, error) {
	if s.smartRouter == nil {
		return nil, status.Error(codes.Unavailable, "smart routing сервис не сконфигурирован")
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id format")
	}

	if err := s.smartRouter.DeleteWeights(ctx, id); err != nil {
		log.Error().Err(err).Msg("ошибка удаления весов smart routing")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &routingv1.DeleteRouteResponse{
		Success: true,
	}, nil
}

// ==================== История lookup-запросов ====================

// GetLookupHistory получает историю lookup-запросов клиента
func (s *Server) GetLookupHistory(ctx context.Context, req *routingv1.GetLookupHistoryRequest) (*routingv1.GetLookupHistoryResponse, error) {
	if s.lookupLogRepo == nil {
		return nil, status.Error(codes.Unavailable, "lookup log репозиторий не сконфигурирован")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	// Преобразуем временные фильтры
	var fromTime, toTime *time.Time
	if req.FromDate != nil {
		t := req.FromDate.AsTime()
		fromTime = &t
	}
	if req.ToDate != nil {
		t := req.ToDate.AsTime()
		toTime = &t
	}

	page := int(req.Page)
	if page == 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize == 0 {
		pageSize = 50
	}

	entries, totalCount, err := s.lookupLogRepo.ListByClient(
		ctx, clientID, fromTime, toTime,
		req.MsisdnFilter, req.SourceFilter,
		page, pageSize,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории lookup")
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEntries := make([]*routingv1.LookupLogEntry, len(entries))
	for i, entry := range entries {
		protoEntries[i] = lookupLogEntryToProto(entry)
	}

	return &routingv1.GetLookupHistoryResponse{
		Items:      protoEntries,
		TotalCount: totalCount,
		Page:       int32(page),
		PageSize:   int32(pageSize),
	}, nil
}

// ==================== Маршрутизация с HLR ====================

// RouteMessageWithHLR маршрутизирует сообщение с предварительным HLR lookup
func (s *Server) RouteMessageWithHLR(ctx context.Context, req *routingv1.RouteMessageWithHLRRequest) (*routingv1.RouteMessageWithHLRResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.Destination == "" {
		return nil, status.Error(codes.InvalidArgument, "destination is required")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsed, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsed
	}

	var existingRouteID *uuid.UUID
	if req.ExistingRouteId != "" {
		parsed, err := uuid.Parse(req.ExistingRouteId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid existing_route_id format")
		}
		existingRouteID = &parsed
	}

	var existingProviderID *uuid.UUID
	if req.ExistingProviderId != "" {
		parsed, err := uuid.Parse(req.ExistingProviderId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid existing_provider_id format")
		}
		existingProviderID = &parsed
	}

	result, err := s.routingService.RouteMessageWithHLR(
		ctx, messageID, req.Destination, clientID,
		existingRouteID, existingProviderID,
	)
	if err != nil {
		if err == domain.ErrNumberInvalid {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if err == domain.ErrNoMatchingRoute {
			return nil, status.Error(codes.NotFound, "no matching route found")
		}
		if err == domain.ErrNoProviders {
			return nil, status.Error(codes.NotFound, "no available providers")
		}
		log.Error().Err(err).Msg("ошибка маршрутизации с HLR")
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &routingv1.RouteMessageWithHLRResponse{
		RouteId:      result.RouteID.String(),
		ProviderId:   result.ProviderID.String(),
		HlrUsed:      result.HLRUsed,
		RoutingScore: result.RoutingScore,
	}

	if result.HLRResult != nil {
		resp.HlrResult = lookupResultToProto(result.HLRResult)
	}

	return resp, nil
}

// ==================== Вспомогательные функции преобразования ====================

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

// lookupResultToProto преобразует domain.LookupResult в proto.NumberLookupResponse
func lookupResultToProto(result *domain.LookupResult) *routingv1.NumberLookupResponse {
	return &routingv1.NumberLookupResponse{
		Msisdn:                 result.MSISDN,
		OperatorMccmnc:         result.OperatorMCCMNC,
		OperatorName:           result.OperatorName,
		NumberStatus:           numberStatusToProto(result.NumberStatus),
		CountryCode:            result.CountryCode,
		NumberType:             numberTypeToProto(result.NumberType),
		IsPorted:               result.IsPorted,
		OriginalOperatorMccmnc: result.OriginalOperatorMCCMNC,
		Cached:                 result.Cached,
		QueriedAt:              timestamppb.New(result.QueriedAt),
	}
}

// hlrProviderToProto преобразует domain.HLRProvider в proto.HLRProviderProto
func hlrProviderToProto(provider *domain.HLRProvider) *routingv1.HLRProviderProto {
	// Сериализуем конфигурацию в JSON
	configJSON := ""
	if provider.Config != nil {
		data, err := json.Marshal(provider.Config)
		if err == nil {
			configJSON = string(data)
		}
	}

	proto := &routingv1.HLRProviderProto{
		Id:               provider.ID.String(),
		Name:             provider.Name,
		AdapterType:      provider.AdapterType,
		ConfigJson:       configJSON,
		Priority:         int32(provider.Priority),
		SupportedRegions: provider.SupportedRegions,
		CostPerLookup:    fmt.Sprintf("%.6f", provider.CostPerLookup),
		Status:           string(provider.Status),
		SuccessRate:      fmt.Sprintf("%.2f", provider.SuccessRate),
		Active:           provider.Active,
		CreatedAt:        timestamppb.New(provider.CreatedAt),
		UpdatedAt:        timestamppb.New(provider.UpdatedAt),
	}

	if provider.LastSuccessAt != nil {
		proto.LastSuccessAt = timestamppb.New(*provider.LastSuccessAt)
	}
	if provider.LastFailureAt != nil {
		proto.LastFailureAt = timestamppb.New(*provider.LastFailureAt)
	}

	return proto
}

// smartRouteWeightToProto преобразует domain.SmartRouteWeight в proto.SmartRouteWeightProto
func smartRouteWeightToProto(weight *domain.SmartRouteWeight) *routingv1.SmartRouteWeightProto {
	return &routingv1.SmartRouteWeightProto{
		Id:            weight.ID.String(),
		OperatorCode:  weight.OperatorCode,
		CountryCode:   weight.CountryCode,
		CostWeight:    fmt.Sprintf("%.4f", weight.CostWeight),
		QualityWeight: fmt.Sprintf("%.4f", weight.QualityWeight),
		Active:        weight.Active,
		CreatedAt:     timestamppb.New(weight.CreatedAt),
		UpdatedAt:     timestamppb.New(weight.UpdatedAt),
	}
}

// lookupLogEntryToProto преобразует domain.LookupLogEntry в proto.LookupLogEntry
func lookupLogEntryToProto(entry *domain.LookupLogEntry) *routingv1.LookupLogEntry {
	proto := &routingv1.LookupLogEntry{
		Id:             entry.ID.String(),
		Msisdn:         entry.MSISDN,
		OperatorMccmnc: entry.OperatorMCCMNC,
		OperatorName:   entry.OperatorName,
		NumberStatus:   numberStatusFromString(entry.NumberStatus),
		CountryCode:    entry.CountryCode,
		NumberType:     numberTypeFromString(entry.NumberType),
		IsPorted:       entry.IsPorted,
		Source:         string(entry.Source),
		ClientId:       entry.ClientID.String(),
		Cached:         entry.Cached,
		LatencyMs:      int32(entry.LatencyMs),
		RequestId:      entry.RequestID,
		CreatedAt:      timestamppb.New(entry.CreatedAt),
	}

	if entry.MessageID != nil {
		proto.MessageId = entry.MessageID.String()
	}

	return proto
}

// ==================== Вспомогательные функции маппинга enum ====================

// numberStatusToProto преобразует domain.NumberStatus в proto.NumberStatus
func numberStatusToProto(s domain.NumberStatus) routingv1.NumberStatus {
	switch s {
	case domain.NumberStatusActive:
		return routingv1.NumberStatus_NUMBER_STATUS_ACTIVE
	case domain.NumberStatusAbsent:
		return routingv1.NumberStatus_NUMBER_STATUS_ABSENT
	case domain.NumberStatusInvalid:
		return routingv1.NumberStatus_NUMBER_STATUS_INVALID
	case domain.NumberStatusUnknown:
		return routingv1.NumberStatus_NUMBER_STATUS_UNKNOWN
	default:
		return routingv1.NumberStatus_NUMBER_STATUS_UNSPECIFIED
	}
}

// numberTypeToProto преобразует domain.NumberType в proto.NumberType
func numberTypeToProto(t domain.NumberType) routingv1.NumberType {
	switch t {
	case domain.NumberTypeMobile:
		return routingv1.NumberType_NUMBER_TYPE_MOBILE
	case domain.NumberTypeFixed:
		return routingv1.NumberType_NUMBER_TYPE_FIXED
	case domain.NumberTypeVoip:
		return routingv1.NumberType_NUMBER_TYPE_VOIP
	default:
		return routingv1.NumberType_NUMBER_TYPE_UNSPECIFIED
	}
}

// numberStatusFromString преобразует строку в proto.NumberStatus
func numberStatusFromString(s string) routingv1.NumberStatus {
	return numberStatusToProto(domain.NumberStatus(s))
}

// numberTypeFromString преобразует строку в proto.NumberType
func numberTypeFromString(s string) routingv1.NumberType {
	return numberTypeToProto(domain.NumberType(s))
}

// ==================== Client Provider methods ====================

func (s *Server) AssignProviderToClient(ctx context.Context, req *routingv1.AssignProviderRequest) (*routingv1.ClientProviderProto, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	cp := domain.NewClientProvider(clientID, providerID, domain.ProviderOwnership(req.Ownership))
	cp.SharedPriority = int(req.SharedPriority)

	if err := cp.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.clientProviderRepo.Create(ctx, cp); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return clientProviderToProto(cp), nil
}

func (s *Server) RevokeProviderFromClient(ctx context.Context, req *routingv1.RevokeProviderRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	cp, err := s.clientProviderRepo.GetByClientAndProvider(ctx, clientID, providerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "client provider not found: %v", err)
	}

	if err := s.clientProviderRepo.Delete(ctx, cp.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ListClientProviders(ctx context.Context, req *routingv1.ListClientProvidersRequest) (*routingv1.ListClientProvidersResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	providers, err := s.clientProviderRepo.ListByClient(ctx, clientID, req.ActiveOnly)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list failed: %v", err)
	}

	resp := &routingv1.ListClientProvidersResponse{}
	for _, cp := range providers {
		resp.Providers = append(resp.Providers, clientProviderToProto(cp))
	}
	return resp, nil
}

func (s *Server) UpdateClientProvider(ctx context.Context, req *routingv1.UpdateClientProviderRequest) (*routingv1.ClientProviderProto, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	cp, err := s.clientProviderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	cp.SharedPriority = int(req.SharedPriority)
	cp.ExposeCost = req.ExposeCost
	cp.ExposeProviderName = req.ExposeProviderName
	cp.Active = req.Active

	if err := s.clientProviderRepo.Update(ctx, cp); err != nil {
		return nil, status.Errorf(codes.Internal, "update failed: %v", err)
	}

	return clientProviderToProto(cp), nil
}

func (s *Server) ShareProviderWithChild(ctx context.Context, req *routingv1.ShareProviderRequest) (*routingv1.ClientProviderProto, error) {
	parentID, err := uuid.Parse(req.ParentClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent_client_id: %v", err)
	}
	childID, err := uuid.Parse(req.ChildClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid child_client_id: %v", err)
	}
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	// Verify parent has the provider
	_, err = s.clientProviderRepo.GetByClientAndProvider(ctx, parentID, providerID)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "parent does not have this provider: %v", err)
	}

	cp := domain.NewClientProvider(childID, providerID, domain.OwnershipInherited)
	cp.SourceClientID = &parentID
	cp.ExposeCost = req.ExposeCost
	cp.ExposeProviderName = req.ExposeProviderName

	if err := cp.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.clientProviderRepo.Create(ctx, cp); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return clientProviderToProto(cp), nil
}

func (s *Server) RevokeSharedProvider(ctx context.Context, req *routingv1.RevokeSharedProviderRequest) (*emptypb.Empty, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	if err := s.clientProviderRepo.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ==================== Client Route methods ====================

func (s *Server) CreateClientRoute(ctx context.Context, req *routingv1.CreateClientRouteRequest) (*routingv1.ClientRouteProto, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}
	operatorID, err := uuid.Parse(req.OperatorId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
	}
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider_id: %v", err)
	}

	// Application-level check: provider must be assigned to client
	_, err = s.clientProviderRepo.GetByClientAndProvider(ctx, clientID, providerID)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "provider not assigned to client: %v", err)
	}

	route := domain.NewClientRoute(clientID, operatorID, providerID, int(req.Priority), int(req.Weight))

	if err := s.clientRouteRepo.Create(ctx, route); err != nil {
		return nil, status.Errorf(codes.Internal, "create failed: %v", err)
	}

	return clientRouteToProto(route), nil
}

func (s *Server) UpdateClientRoute(ctx context.Context, req *routingv1.UpdateClientRouteRequest) (*routingv1.ClientRouteProto, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	route, err := s.clientRouteRepo.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	route.Priority = int(req.Priority)
	route.Weight = int(req.Weight)
	route.Active = req.Active

	if err := s.clientRouteRepo.Update(ctx, route); err != nil {
		return nil, status.Errorf(codes.Internal, "update failed: %v", err)
	}

	return clientRouteToProto(route), nil
}

func (s *Server) DeleteClientRoute(ctx context.Context, req *routingv1.DeleteClientRouteRequest) (*emptypb.Empty, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}

	if err := s.clientRouteRepo.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ListClientRoutes(ctx context.Context, req *routingv1.ListClientRoutesRequest) (*routingv1.ListClientRoutesResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	var routes []*domain.ClientRoute
	if req.OperatorId != "" {
		operatorID, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		routes, err = s.clientRouteRepo.ListByClientAndOperator(ctx, clientID, operatorID, false)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list failed: %v", err)
		}
	} else {
		routes, err = s.clientRouteRepo.ListByClient(ctx, clientID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list failed: %v", err)
		}
	}

	resp := &routingv1.ListClientRoutesResponse{}
	for _, r := range routes {
		resp.Routes = append(resp.Routes, clientRouteToProto(r))
	}
	return resp, nil
}

// ==================== Routing Strategy methods ====================

func (s *Server) SetRoutingStrategy(ctx context.Context, req *routingv1.SetRoutingStrategyRequest) (*routingv1.ClientRoutingStrategyProto, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	var operatorID *uuid.UUID
	if req.OperatorId != "" {
		parsed, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		operatorID = &parsed
	}

	strategy := domain.NewClientRoutingStrategy(clientID, operatorID, domain.RoutingStrategy(req.Strategy))
	if err := strategy.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if err := s.clientStrategyRepo.Upsert(ctx, strategy); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert failed: %v", err)
	}

	return clientStrategyToProto(strategy), nil
}

func (s *Server) GetRoutingStrategy(ctx context.Context, req *routingv1.GetRoutingStrategyRequest) (*routingv1.ClientRoutingStrategyProto, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	var operatorID *uuid.UUID
	if req.OperatorId != "" {
		parsed, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		operatorID = &parsed
	}

	strategy, err := s.clientStrategyRepo.Get(ctx, clientID, operatorID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return clientStrategyToProto(strategy), nil
}

func (s *Server) DeleteRoutingStrategy(ctx context.Context, req *routingv1.DeleteRoutingStrategyRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid client_id: %v", err)
	}

	var operatorID *uuid.UUID
	if req.OperatorId != "" {
		parsed, err := uuid.Parse(req.OperatorId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid operator_id: %v", err)
		}
		operatorID = &parsed
	}

	if err := s.clientStrategyRepo.Delete(ctx, clientID, operatorID); err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ==================== Proto conversion helpers ====================

func clientProviderToProto(cp *domain.ClientProvider) *routingv1.ClientProviderProto {
	proto := &routingv1.ClientProviderProto{
		Id:                 cp.ID.String(),
		ClientId:           cp.ClientID.String(),
		ProviderId:         cp.ProviderID.String(),
		Ownership:          string(cp.Ownership),
		SharedPriority:     int32(cp.SharedPriority),
		ExposeCost:         cp.ExposeCost,
		ExposeProviderName: cp.ExposeProviderName,
		Active:             cp.Active,
		CreatedAt:          timestamppb.New(cp.CreatedAt),
		UpdatedAt:          timestamppb.New(cp.UpdatedAt),
	}
	if cp.SourceClientID != nil {
		proto.SourceClientId = cp.SourceClientID.String()
	}
	return proto
}

func clientRouteToProto(r *domain.ClientRoute) *routingv1.ClientRouteProto {
	return &routingv1.ClientRouteProto{
		Id:         r.ID.String(),
		ClientId:   r.ClientID.String(),
		OperatorId: r.OperatorID.String(),
		ProviderId: r.ProviderID.String(),
		Priority:   int32(r.Priority),
		Weight:     int32(r.Weight),
		Active:     r.Active,
		CreatedAt:  timestamppb.New(r.CreatedAt),
		UpdatedAt:  timestamppb.New(r.UpdatedAt),
	}
}

func clientStrategyToProto(s *domain.ClientRoutingStrategy) *routingv1.ClientRoutingStrategyProto {
	proto := &routingv1.ClientRoutingStrategyProto{
		Id:        s.ID.String(),
		ClientId:  s.ClientID.String(),
		Strategy:  string(s.Strategy),
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
	if s.OperatorID != nil {
		proto.OperatorId = s.OperatorID.String()
	}
	return proto
}
