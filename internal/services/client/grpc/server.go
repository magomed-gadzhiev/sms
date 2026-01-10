package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/services/client/application"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	clientrepo "github.com/smpp-server/smpp-server/internal/services/client/infrastructure/repository"
)

// Server реализует gRPC сервис для управления клиентами
type Server struct {
	clientv1.UnimplementedClientServiceServer
	clientService *application.ClientService
}

// NewServer создает новый gRPC сервер для Client Service
func NewServer(clientService *application.ClientService) *Server {
	return &Server{
		clientService: clientService,
	}
}

// CreateClient создает нового клиента
func (s *Server) CreateClient(ctx context.Context, req *clientv1.CreateClientRequest) (*clientv1.CreateClientResponse, error) {
	// Валидация
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Преобразуем метаданные
	var metadata map[string]string
	if req.Metadata != nil {
		metadata = req.Metadata
	}

	// Создаем клиента
	client, err := s.clientService.CreateClient(
		ctx,
		req.Name,
		req.Email,
		req.ContactPerson,
		req.Phone,
		req.Active,
		metadata,
	)
	if err != nil {
		if err == application.ErrInvalidClientData {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		log.Error().Err(err).Msg("ошибка создания клиента")
		return nil, status.Error(codes.Internal, "failed to create client")
	}

	return &clientv1.CreateClientResponse{
		ClientId:  client.ID.String(),
		CreatedAt: timestamppb.New(client.CreatedAt),
	}, nil
}

// UpdateClient обновляет клиента
func (s *Server) UpdateClient(ctx context.Context, req *clientv1.UpdateClientRequest) (*clientv1.UpdateClientResponse, error) {
	// Валидация
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	// Подготавливаем параметры для обновления
	var name, email, contactPerson, phone *string
	var active *bool
	var metadata map[string]string

	if req.Name != "" {
		name = &req.Name
	}
	if req.Email != "" {
		email = &req.Email
	}
	if req.ContactPerson != "" {
		contactPerson = &req.ContactPerson
	}
	if req.Phone != "" {
		phone = &req.Phone
	}
	if req.Metadata != nil {
		metadata = req.Metadata
	}
	// active всегда передаем, если указан (нужно различать false от не указанного)
	// В proto3 это сложнее, но мы будем использовать значение по умолчанию false
	// и проверять через has_active или отдельное поле
	active = &req.Active

	// Обновляем клиента
	_, err = s.clientService.UpdateClient(
		ctx,
		clientID,
		name, email, contactPerson, phone,
		active,
		metadata,
	)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка обновления клиента")
		return nil, status.Error(codes.Internal, "failed to update client")
	}

	return &clientv1.UpdateClientResponse{
		Success: true,
	}, nil
}

// GetClient получает информацию о клиенте
func (s *Server) GetClient(ctx context.Context, req *clientv1.GetClientRequest) (*clientv1.GetClientResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	client, err := s.clientService.GetClient(ctx, clientID)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка получения клиента")
		return nil, status.Error(codes.Internal, "failed to get client")
	}

	return &clientv1.GetClientResponse{
		Client: s.domainClientToProto(client),
	}, nil
}

// ListClients получает список клиентов
func (s *Server) ListClients(ctx context.Context, req *clientv1.ListClientsRequest) (*clientv1.ListClientsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 100 // значение по умолчанию
	}

	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	clients, total, err := s.clientService.ListClients(
		ctx,
		req.ActiveOnly,
		req.Search,
		limit,
		offset,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка клиентов")
		return nil, status.Error(codes.Internal, "failed to list clients")
	}

	protoClients := make([]*clientv1.ClientInfo, len(clients))
	for i, client := range clients {
		protoClients[i] = s.domainClientToProto(client)
	}

	return &clientv1.ListClientsResponse{
		Clients: protoClients,
		Total:   int32(total),
	}, nil
}

// DeleteClient удаляет клиента
func (s *Server) DeleteClient(ctx context.Context, req *clientv1.DeleteClientRequest) (*clientv1.DeleteClientResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	err = s.clientService.DeleteClient(ctx, clientID)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка удаления клиента")
		return nil, status.Error(codes.Internal, "failed to delete client")
	}

	return &clientv1.DeleteClientResponse{
		Success: true,
	}, nil
}

// GetClientConfig получает конфигурацию клиента
func (s *Server) GetClientConfig(ctx context.Context, req *clientv1.GetClientConfigRequest) (*clientv1.GetClientConfigResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	config, err := s.clientService.GetClientConfig(ctx, clientID)
	if err != nil {
		if err == application.ErrConfigNotFound {
			return nil, status.Error(codes.NotFound, "client config not found")
		}
		log.Error().Err(err).Msg("ошибка получения конфигурации клиента")
		return nil, status.Error(codes.Internal, "failed to get client config")
	}

	return &clientv1.GetClientConfigResponse{
		Config: s.domainConfigToProto(clientID, config),
	}, nil
}

// UpdateClientConfig обновляет конфигурацию клиента
func (s *Server) UpdateClientConfig(ctx context.Context, req *clientv1.UpdateClientConfigRequest) (*clientv1.UpdateClientConfigResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}

	config := s.protoToDomainConfig(clientID, req.Config)

	err = s.clientService.UpdateClientConfig(ctx, clientID, config)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка обновления конфигурации клиента")
		return nil, status.Error(codes.Internal, "failed to update client config")
	}

	return &clientv1.UpdateClientConfigResponse{
		Success: true,
	}, nil
}

// UpdateClientRateLimits обновляет rate limits клиента
func (s *Server) UpdateClientRateLimits(ctx context.Context, req *clientv1.UpdateClientRateLimitsRequest) (*clientv1.UpdateClientRateLimitsResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if req.RateLimits == nil {
		return nil, status.Error(codes.InvalidArgument, "rate_limits is required")
	}

	limits := &domain.RateLimits{
		PerSecond: int(req.RateLimits.MessagesPerSecond),
		PerMinute: int(req.RateLimits.MessagesPerMinute),
		PerHour:   int(req.RateLimits.MessagesPerHour),
		PerDay:    int(req.RateLimits.MessagesPerDay),
	}

	err = s.clientService.UpdateClientRateLimits(ctx, clientID, limits)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		if err == application.ErrConfigNotFound {
			return nil, status.Error(codes.NotFound, "client config not found")
		}
		log.Error().Err(err).Msg("ошибка обновления rate limits")
		return nil, status.Error(codes.Internal, "failed to update rate limits")
	}

	return &clientv1.UpdateClientRateLimitsResponse{
		Success: true,
	}, nil
}

// domainClientToProto преобразует domain.Client в proto ClientInfo
func (s *Server) domainClientToProto(client *domain.Client) *clientv1.ClientInfo {
	if client == nil {
		return nil
	}

	info := &clientv1.ClientInfo{
		ClientId:     client.ID.String(),
		Name:         client.Name,
		Email:        client.Email,
		ContactPerson: client.ContactPerson,
		Phone:        client.Phone,
		Active:       client.Active,
		Metadata:     client.GetMetadata(),
		CreatedAt:    timestamppb.New(client.CreatedAt),
		UpdatedAt:    timestamppb.New(client.UpdatedAt),
	}

	// Добавляем rate limits из конфигурации
	if client.Config != nil {
		info.RateLimits = &clientv1.RateLimits{
			MessagesPerSecond: int32(client.Config.RateLimitPerSecond),
			MessagesPerMinute: int32(client.Config.RateLimitPerMinute),
			MessagesPerHour:   int32(client.Config.RateLimitPerHour),
			MessagesPerDay:    int32(client.Config.RateLimitPerDay),
		}
	}

	return info
}

// domainConfigToProto преобразует domain.ClientConfig в proto ClientConfig
func (s *Server) domainConfigToProto(clientID uuid.UUID, config *domain.ClientConfig) *clientv1.ClientConfig {
	if config == nil {
		return nil
	}

	return &clientv1.ClientConfig{
		ClientId:            clientID.String(),
		RateLimits: &clientv1.RateLimits{
			MessagesPerSecond: int32(config.RateLimitPerSecond),
			MessagesPerMinute: int32(config.RateLimitPerMinute),
			MessagesPerHour:   int32(config.RateLimitPerHour),
			MessagesPerDay:    int32(config.RateLimitPerDay),
		},
		AllowedSources:      config.AllowedSources,
		BlockedDestinations: config.BlockedDestinations,
		Settings:            config.GetSettings(),
		UpdatedAt:           timestamppb.New(config.UpdatedAt),
	}
}

// protoToDomainConfig преобразует proto ClientConfig в domain.ClientConfig
func (s *Server) protoToDomainConfig(clientID uuid.UUID, protoConfig *clientv1.ClientConfig) *domain.ClientConfig {
	config := &domain.ClientConfig{
		ClientID: clientID,
		AllowedSources: protoConfig.AllowedSources,
		BlockedDestinations: protoConfig.BlockedDestinations,
	}

	if protoConfig.RateLimits != nil {
		config.RateLimitPerSecond = int(protoConfig.RateLimits.MessagesPerSecond)
		config.RateLimitPerMinute = int(protoConfig.RateLimits.MessagesPerMinute)
		config.RateLimitPerHour = int(protoConfig.RateLimits.MessagesPerHour)
		config.RateLimitPerDay = int(protoConfig.RateLimits.MessagesPerDay)
	}

	if protoConfig.Settings != nil {
		config.SetSettings(protoConfig.Settings)
	}

	return config
}
