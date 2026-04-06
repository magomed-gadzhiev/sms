package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/clientv1"
	"github.com/smpp-server/smpp-server/internal/services/client/application"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
)

// Server реализует gRPC сервис для управления клиентами
type Server struct {
	clientv1.UnimplementedClientServiceServer
	clientService     *application.ClientService
	subAccountService *application.SubAccountService
}

// NewServer создает новый gRPC сервер для Client Service
func NewServer(clientService *application.ClientService, subAccountService *application.SubAccountService) *Server {
	return &Server{
		clientService:     clientService,
		subAccountService: subAccountService,
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
		req.IsSandbox,
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
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

// CreateSubAccount создает суб-аккаунт для реселлера
func (s *Server) CreateSubAccount(ctx context.Context, req *clientv1.CreateSubAccountRequest) (*clientv1.CreateSubAccountResponse, error) {
	if req.ParentClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	parentClientID, err := parseClientID(req.ParentClientId)
	if err != nil {
		return nil, err
	}

	subAccount, err := s.subAccountService.CreateSubAccount(
		ctx,
		parentClientID,
		req.Name,
		req.Email,
		req.ContactPerson,
		int(req.DailyLimit),
		int(req.MonthlyLimit),
	)
	if err != nil {
		switch err {
		case application.ErrClientNotFound:
			return nil, status.Error(codes.NotFound, "parent client not found")
		case application.ErrNotReseller:
			return nil, status.Error(codes.PermissionDenied, "client is not a reseller")
		case application.ErrMaxSubAccounts:
			return nil, status.Error(codes.ResourceExhausted, "maximum number of sub-accounts reached")
		case application.ErrInvalidClientData:
			return nil, status.Error(codes.InvalidArgument, "invalid sub-account data")
		}
		log.Error().Err(err).Msg("ошибка создания суб-аккаунта")
		return nil, status.Error(codes.Internal, "failed to create sub-account")
	}

	return &clientv1.CreateSubAccountResponse{
		SubAccount: s.domainClientToSubAccount(subAccount),
	}, nil
}

// ListSubAccounts получает список суб-аккаунтов реселлера
func (s *Server) ListSubAccounts(ctx context.Context, req *clientv1.ListSubAccountsRequest) (*clientv1.ListSubAccountsResponse, error) {
	parentClientID, err := parseClientID(req.ParentClientId)
	if err != nil {
		return nil, err
	}

	subAccounts, err := s.subAccountService.ListSubAccounts(ctx, parentClientID)
	if err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "parent client not found")
		}
		if err == application.ErrNotReseller {
			return nil, status.Error(codes.PermissionDenied, "client is not a reseller")
		}
		log.Error().Err(err).Msg("ошибка получения списка суб-аккаунтов")
		return nil, status.Error(codes.Internal, "failed to list sub-accounts")
	}

	// Получаем информацию о лимитах
	parent, _ := s.clientService.GetClient(ctx, parentClientID)
	var maxSubAccounts int32
	if parent != nil {
		maxSubAccounts = int32(parent.MaxSubAccounts)
	}

	protoSubAccounts := make([]*clientv1.SubAccount, len(subAccounts))
	for i, sa := range subAccounts {
		protoSubAccounts[i] = s.domainClientToSubAccount(sa)
	}

	return &clientv1.ListSubAccountsResponse{
		SubAccounts:    protoSubAccounts,
		MaxSubAccounts: maxSubAccounts,
		CurrentCount:   int32(len(subAccounts)),
	}, nil
}

// GetSubAccount получает информацию о суб-аккаунте
func (s *Server) GetSubAccount(ctx context.Context, req *clientv1.GetSubAccountRequest) (*clientv1.GetSubAccountResponse, error) {
	if req.SubAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "sub_account_id is required")
	}
	if req.ParentClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_client_id is required")
	}

	subAccountID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id format")
	}

	parentClientID, err := parseClientID(req.ParentClientId)
	if err != nil {
		return nil, err
	}

	subAccount, err := s.subAccountService.GetSubAccount(ctx, subAccountID, parentClientID)
	if err != nil {
		if err == application.ErrSubAccountNotFound {
			return nil, status.Error(codes.NotFound, "sub-account not found")
		}
		log.Error().Err(err).Msg("ошибка получения суб-аккаунта")
		return nil, status.Error(codes.Internal, "failed to get sub-account")
	}

	return &clientv1.GetSubAccountResponse{
		SubAccount: s.domainClientToSubAccount(subAccount),
	}, nil
}

// DeleteSubAccount удаляет суб-аккаунт
func (s *Server) DeleteSubAccount(ctx context.Context, req *clientv1.DeleteSubAccountRequest) (*clientv1.DeleteSubAccountResponse, error) {
	if req.SubAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "sub_account_id is required")
	}
	if req.ParentClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_client_id is required")
	}

	subAccountID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id format")
	}

	parentClientID, err := parseClientID(req.ParentClientId)
	if err != nil {
		return nil, err
	}

	err = s.subAccountService.DeleteSubAccount(ctx, subAccountID, parentClientID)
	if err != nil {
		if err == application.ErrSubAccountNotFound {
			return nil, status.Error(codes.NotFound, "sub-account not found")
		}
		log.Error().Err(err).Msg("ошибка удаления суб-аккаунта")
		return nil, status.Error(codes.Internal, "failed to delete sub-account")
	}

	return &clientv1.DeleteSubAccountResponse{
		ReturnedBalance: "0",
	}, nil
}

// UpdateSubAccountLimits обновляет лимиты суб-аккаунта
func (s *Server) UpdateSubAccountLimits(ctx context.Context, req *clientv1.UpdateSubAccountLimitsRequest) (*clientv1.UpdateSubAccountLimitsResponse, error) {
	if req.SubAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "sub_account_id is required")
	}
	if req.ParentClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_client_id is required")
	}

	subAccountID, err := uuid.Parse(req.SubAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sub_account_id format")
	}

	parentClientID, err := parseClientID(req.ParentClientId)
	if err != nil {
		return nil, err
	}

	subAccount, err := s.subAccountService.UpdateSubAccountLimits(
		ctx,
		subAccountID,
		parentClientID,
		int(req.DailyLimit),
		int(req.MonthlyLimit),
	)
	if err != nil {
		if err == application.ErrSubAccountNotFound {
			return nil, status.Error(codes.NotFound, "sub-account not found")
		}
		log.Error().Err(err).Msg("ошибка обновления лимитов суб-аккаунта")
		return nil, status.Error(codes.Internal, "failed to update sub-account limits")
	}

	return &clientv1.UpdateSubAccountLimitsResponse{
		SubAccount: s.domainClientToSubAccount(subAccount),
	}, nil
}

// ToggleSandbox включает или отключает sandbox-режим для клиента
func (s *Server) ToggleSandbox(ctx context.Context, req *clientv1.ToggleSandboxRequest) (*clientv1.ToggleSandboxResponse, error) {
	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.clientService.ToggleSandbox(ctx, clientID, req.Enable); err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка изменения sandbox-режима")
		return nil, status.Error(codes.Internal, "failed to toggle sandbox")
	}

	return &clientv1.ToggleSandboxResponse{
		IsSandbox: req.Enable,
	}, nil
}

// AssignPlan назначает тарифный план клиенту
func (s *Server) AssignPlan(ctx context.Context, req *clientv1.AssignPlanRequest) (*clientv1.AssignPlanResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.PlanId == "" {
		return nil, status.Error(codes.InvalidArgument, "plan_id is required")
	}

	clientID, err := parseClientID(req.ClientId)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid plan_id format")
	}

	if err := s.clientService.AssignPlan(ctx, clientID, planID); err != nil {
		if err == application.ErrClientNotFound {
			return nil, status.Error(codes.NotFound, "client not found")
		}
		log.Error().Err(err).Msg("ошибка назначения тарифного плана")
		return nil, status.Error(codes.Internal, "failed to assign plan")
	}

	return &clientv1.AssignPlanResponse{Success: true}, nil
}

// ListPlans возвращает список активных тарифных планов
func (s *Server) ListPlans(ctx context.Context, req *clientv1.ListPlansRequest) (*clientv1.ListPlansResponse, error) {
	plans, err := s.clientService.ListPlans(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка планов")
		return nil, status.Error(codes.Internal, "failed to list plans")
	}

	protoPlans := make([]*clientv1.SubscriptionPlan, 0, len(plans))
	for _, p := range plans {
		protoPlans = append(protoPlans, &clientv1.SubscriptionPlan{
			Id:                 p.ID.String(),
			Name:               p.Name,
			DisplayName:        p.DisplayName,
			MonthlyPriceRub:    p.MonthlyPriceRub,
			MaxSmsPerMonth:     int32(p.MaxSMSPerMonth),
			MaxSmppConnections: int32(p.MaxSMPPConnections),
			MaxUsers:           int32(p.MaxUsers),
			RateLimits: &clientv1.RateLimits{
				MessagesPerSecond: int32(p.RateLimitPerSecond),
				MessagesPerMinute: int32(p.RateLimitPerMinute),
				MessagesPerHour:   int32(p.RateLimitPerHour),
				MessagesPerDay:    int32(p.RateLimitPerDay),
			},
			Features: map[string]bool{
				"analytics":     p.Features.Analytics,
				"webhooks":      p.Features.Webhooks,
				"hlr":           p.Features.HLR,
				"smart_routing": p.Features.SmartRouting,
				"sub_accounts":  p.Features.SubAccounts,
				"white_label":   p.Features.WhiteLabel,
			},
			Active: p.Active,
		})
	}

	return &clientv1.ListPlansResponse{Plans: protoPlans}, nil
}

// domainClientToSubAccount преобразует domain.Client в proto SubAccount
func (s *Server) domainClientToSubAccount(client *domain.Client) *clientv1.SubAccount {
	if client == nil {
		return nil
	}

	sa := &clientv1.SubAccount{
		Id:            client.ID.String(),
		Name:          client.Name,
		Email:         client.Email,
		ContactPerson: client.ContactPerson,
		Active:        client.Active,
		CreatedAt:     timestamppb.New(client.CreatedAt),
	}

	if client.Config != nil {
		sa.DailyLimit = int32(client.Config.RateLimitPerDay)
		settings := client.Config.GetSettings()
		if ml, ok := settings["monthly_limit"]; ok {
			if v, err := parseIntFromString(ml); err == nil {
				sa.MonthlyLimit = int32(v)
			}
		}
	}

	return sa
}

// parseIntFromString парсит int из строки
func parseIntFromString(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// domainClientToProto преобразует domain.Client в proto ClientInfo
func (s *Server) domainClientToProto(client *domain.Client) *clientv1.ClientInfo {
	if client == nil {
		return nil
	}

	info := &clientv1.ClientInfo{
		ClientId:        client.ID.String(),
		Name:            client.Name,
		Email:           client.Email,
		ContactPerson:   client.ContactPerson,
		Phone:           client.Phone,
		Active:          client.Active,
		Metadata:        client.GetMetadata(),
		CreatedAt:       timestamppb.New(client.CreatedAt),
		UpdatedAt:       timestamppb.New(client.UpdatedAt),
		IsReseller:      client.IsReseller,
		MaxSubAccounts:  int32(client.MaxSubAccounts),
		IsSandbox:       client.IsSandbox,
		MonthlySmsCount: int32(client.MonthlySMSCount),
	}

	if client.PlanID != nil {
		info.PlanId = client.PlanID.String()
	}

	if client.Plan != nil {
		info.Plan = &clientv1.SubscriptionPlan{
			Id:                 client.Plan.ID.String(),
			Name:               client.Plan.Name,
			DisplayName:        client.Plan.DisplayName,
			MonthlyPriceRub:    client.Plan.MonthlyPriceRub,
			MaxSmsPerMonth:     int32(client.Plan.MaxSMSPerMonth),
			MaxSmppConnections: int32(client.Plan.MaxSMPPConnections),
			MaxUsers:           int32(client.Plan.MaxUsers),
			RateLimits: &clientv1.RateLimits{
				MessagesPerSecond: int32(client.Plan.RateLimitPerSecond),
				MessagesPerMinute: int32(client.Plan.RateLimitPerMinute),
				MessagesPerHour:   int32(client.Plan.RateLimitPerHour),
				MessagesPerDay:    int32(client.Plan.RateLimitPerDay),
			},
			Features: map[string]bool{
				"analytics":     client.Plan.Features.Analytics,
				"webhooks":      client.Plan.Features.Webhooks,
				"hlr":           client.Plan.Features.HLR,
				"smart_routing": client.Plan.Features.SmartRouting,
				"sub_accounts":  client.Plan.Features.SubAccounts,
				"white_label":   client.Plan.Features.WhiteLabel,
			},
			Active: client.Plan.Active,
		}
	}

	if client.ParentClientID != nil {
		info.ParentClientId = client.ParentClientID.String()
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
