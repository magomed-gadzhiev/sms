package grpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	providerv1 "github.com/smpp-server/smpp-server/api/proto/providerv1"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
)

// Server реализует gRPC сервис для работы с провайдерами
type Server struct {
	providerv1.UnimplementedProviderServiceServer
	providerService     *application.ProviderService
	connectionPoolService application.ConnectionPoolService
	senderService       *application.SenderService
}

// NewServer создает новый gRPC сервер для Provider Service
func NewServer(
	providerService *application.ProviderService,
	connectionPoolService application.ConnectionPoolService,
	senderService *application.SenderService,
) *Server {
	return &Server{
		providerService:      providerService,
		connectionPoolService: connectionPoolService,
		senderService:        senderService,
	}
}

// CreateProvider создает нового провайдера
func (s *Server) CreateProvider(ctx context.Context, req *providerv1.CreateProviderRequest) (*providerv1.CreateProviderResponse, error) {
	// Валидация
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Host == "" {
		return nil, status.Error(codes.InvalidArgument, "host is required")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return nil, status.Error(codes.InvalidArgument, "port must be between 1 and 65535")
	}
	if req.SystemId == "" {
		return nil, status.Error(codes.InvalidArgument, "system_id is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	// Преобразуем bind_type из int в строку
	bindType := domain.BindTypeTransceiver // по умолчанию
	switch req.BindType {
	case 1:
		bindType = domain.BindTypeTransmitter
	case 2:
		bindType = domain.BindTypeReceiver
	case 3:
		bindType = domain.BindTypeTransceiver
	}

	// Создаем доменную модель
	provider := &domain.Provider{
		Name:             req.Name,
		Host:             req.Host,
		Port:             int(req.Port),
		SystemID:         req.SystemId,
		Password:         req.Password,
		SystemType:       req.SystemType,
		BindType:         bindType,
		BindTON:          int(req.AddrTon),
		BindNPI:          int(req.AddrNpi),
		AddrTON:          int(req.AddrTon),
		AddrNPI:          int(req.AddrNpi),
		MaxConnections:   int(req.MaxConnections),
		WindowSize:       int(req.WindowSize),
		Active:           req.Active,
		Priority:         0, // По умолчанию
		ThroughputPerSec: 10, // По умолчанию
	}

	if err := s.providerService.CreateProvider(ctx, provider); err != nil {
		log.Error().Err(err).Msg("ошибка создания провайдера")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create provider: %v", err))
	}

	// Инициализируем соединения если провайдер активен
	if provider.Active && provider.MaxConnections > 0 {
		if err := s.connectionPoolService.Connect(ctx, provider); err != nil {
			log.Warn().Err(err).Str("provider_id", provider.ID.String()).Msg("ошибка подключения к провайдеру при создании")
		}
	}

	return &providerv1.CreateProviderResponse{
		ProviderId: provider.ID.String(),
		CreatedAt:  timestamppb.New(provider.CreatedAt),
	}, nil
}

// UpdateProvider обновляет провайдера
func (s *Server) UpdateProvider(ctx context.Context, req *providerv1.UpdateProviderRequest) (*providerv1.UpdateProviderResponse, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}

	// Создаем обновления
	updates := &domain.Provider{
		Name:             req.Name,
		Host:             req.Host,
		Port:             int(req.Port),
		SystemID:         req.SystemId,
		Password:         req.Password,
		MaxConnections:   int(req.MaxConnections),
		Active:           req.Active,
	}

	// Запоминаем состояние до обновления для определения изменения active
	existingProvider, err := s.providerService.GetProvider(ctx, providerID)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get provider: %v", err))
	}
	wasActive := existingProvider.Active

	if err := s.providerService.UpdateProvider(ctx, providerID, updates); err != nil {
		log.Error().Err(err).Msg("ошибка обновления провайдера")
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update provider: %v", err))
	}

	// Если провайдер стал активным, инициализируем соединения
	if updates.Active && !wasActive {
		provider, err := s.providerService.GetProvider(ctx, providerID)
		if err == nil && provider.MaxConnections > 0 {
			if err := s.connectionPoolService.Connect(ctx, provider); err != nil {
				log.Warn().Err(err).Str("provider_id", providerID.String()).Msg("ошибка подключения к провайдеру при обновлении")
			}
		}
	}

	// Если провайдер деактивирован, закрываем соединения
	if !updates.Active && wasActive {
		if err := s.connectionPoolService.Disconnect(providerID); err != nil {
			log.Warn().Err(err).Str("provider_id", providerID.String()).Msg("ошибка закрытия соединений при деактивации провайдера")
		}
		log.Info().Str("provider_id", providerID.String()).Msg("провайдер деактивирован, соединения закрыты, маршруты деактивированы")
	}

	return &providerv1.UpdateProviderResponse{
		Success: true,
	}, nil
}

// GetProvider получает провайдера по ID
func (s *Server) GetProvider(ctx context.Context, req *providerv1.GetProviderRequest) (*providerv1.GetProviderResponse, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}

	provider, err := s.providerService.GetProvider(ctx, providerID)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "provider not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get provider: %v", err))
	}

	return &providerv1.GetProviderResponse{
		Provider: domainToProto(provider),
	}, nil
}

// ListProviders получает список провайдеров
func (s *Server) ListProviders(ctx context.Context, req *providerv1.ListProvidersRequest) (*providerv1.ListProvidersResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	providers, total, err := s.providerService.ListProviders(ctx, req.ActiveOnly, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list providers: %v", err))
	}

	protoProviders := make([]*providerv1.ProviderInfo, len(providers))
	for i, p := range providers {
		protoProviders[i] = domainToProto(p)
	}

	return &providerv1.ListProvidersResponse{
		Providers: protoProviders,
		Total:     int32(total),
	}, nil
}

// DeleteProvider удаляет провайдера
func (s *Server) DeleteProvider(ctx context.Context, req *providerv1.DeleteProviderRequest) (*providerv1.DeleteProviderRequest, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}

	if err := s.providerService.DeleteProvider(ctx, providerID); err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "provider not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete provider: %v", err))
	}

	// Закрываем соединения
	if err := s.connectionPoolService.Disconnect(providerID); err != nil {
		log.Warn().Err(err).Str("provider_id", providerID.String()).Msg("ошибка закрытия соединений провайдера")
	}

	return req, nil
}

// GetProviderHealth получает статус здоровья провайдера
func (s *Server) GetProviderHealth(ctx context.Context, req *providerv1.GetProviderHealthRequest) (*providerv1.GetProviderHealthResponse, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}

	_, err = s.providerService.GetProvider(ctx, providerID)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "provider not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get provider: %v", err))
	}

	activeConnections, totalConnections, err := s.connectionPoolService.HealthCheck(providerID)
	if err != nil {
		log.Warn().Err(err).Str("provider_id", providerID.String()).Msg("ошибка проверки здоровья соединений")
		// При ошибке считаем, что соединений нет
		activeConnections = 0
		totalConnections = 0
	}

	// Вычисляем success rate на основе активных соединений
	successRate := int32(100)
	if totalConnections > 0 {
		successRate = int32((activeConnections * 100) / totalConnections)
	}

	// Используем доменную модель Health для расчета статуса
	health := &domain.Health{
		ProviderID:        providerID.String(),
		ActiveConnections: activeConnections,
		TotalConnections:  totalConnections,
		SuccessRate:       int(successRate),
		MessagesSent24h:   0, // TODO: получить из метрик мониторинга
		MessagesFailed24h: 0, // TODO: получить из метрик мониторинга
	}

	healthStatus := health.CalculateStatus()

	resp := &providerv1.GetProviderHealthResponse{
		ProviderId:         providerID.String(),
		Status:             string(healthStatus),
		ActiveConnections:  int32(activeConnections),
		TotalConnections:   int32(totalConnections),
		SuccessRate:        successRate,
		MessagesSent_24H:   health.MessagesSent24h,
		MessagesFailed_24H: health.MessagesFailed24h,
	}

	// Добавляем LastSuccess и LastFailure если они есть
	if health.LastSuccess != nil {
		resp.LastSuccess = timestamppb.New(*health.LastSuccess)
	}
	if health.LastFailure != nil {
		resp.LastFailure = timestamppb.New(*health.LastFailure)
	}
	if health.LastError != "" {
		resp.LastError = health.LastError
	}

	return resp, nil
}

// SendToProvider отправляет сообщение через провайдера
func (s *Server) SendToProvider(ctx context.Context, req *providerv1.SendToProviderRequest) (*providerv1.SendToProviderResponse, error) {
	providerID, err := uuid.Parse(req.ProviderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid provider_id")
	}

	// Получаем провайдера
	provider, err := s.providerService.GetProvider(ctx, providerID)
	if err != nil {
		if errors.Is(err, domain.ErrProviderNotFound) {
			return nil, status.Error(codes.NotFound, "provider not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get provider: %v", err))
	}

	// Подготавливаем параметры отправки
	params := &application.SendMessageParams{
		Source:             req.Source,
		Destination:        req.Destination,
		Text:               req.Text,
		SourceAddrTON:      0, // По умолчанию
		SourceAddrNPI:      0, // По умолчанию
		DestAddrTON:        0, // По умолчанию
		DestAddrNPI:        0, // По умолчанию
		RegisteredDelivery: 1, // По умолчанию запрашиваем DLR
		DataCoding:         0, // По умолчанию
	}

	// Отправляем сообщение
	smppMessageID, err := s.senderService.SendMessage(ctx, provider, params)
	if err != nil {
		log.Error().Err(err).Str("provider_id", providerID.String()).Msg("ошибка отправки сообщения")
		return &providerv1.SendToProviderResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &providerv1.SendToProviderResponse{
		Success:        true,
		SmppMessageId: smppMessageID,
	}, nil
}

// domainToProto преобразует domain.Provider в providerv1.ProviderInfo
func domainToProto(p *domain.Provider) *providerv1.ProviderInfo {
	return &providerv1.ProviderInfo{
		ProviderId:     p.ID.String(),
		Name:           p.Name,
		Host:           p.Host,
		Port:           int32(p.Port),
		SystemId:       p.SystemID,
		SystemType:     p.SystemType,
		BindType:       int32(getBindTypeInt(string(p.BindType))),
		MaxConnections: int32(p.MaxConnections),
		WindowSize:     int32(p.WindowSize),
		Active:         p.Active,
		Settings:       make(map[string]string), // TODO: добавить настройки если нужно
		CreatedAt:      timestamppb.New(p.CreatedAt),
		UpdatedAt:      timestamppb.New(p.UpdatedAt),
	}
}

// getBindTypeInt преобразует строковый bind type в int (для совместимости с proto)
func getBindTypeInt(bindType string) int {
	switch bindType {
	case "transceiver":
		return 3
	case "transmitter":
		return 1
	case "receiver":
		return 2
	default:
		return 3 // transceiver по умолчанию
	}
}
