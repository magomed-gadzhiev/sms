package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
	"github.com/smpp-server/smpp-server/internal/services/messaging/application"
)

// Server реализует gRPC сервис для работы с сообщениями
type Server struct {
	messagingv1.UnimplementedMessagingServiceServer
	messageService *application.MessageService
	dlrService     *application.DLRService
}

// NewServer создает новый gRPC сервер для Messaging Service
func NewServer(
	messageService *application.MessageService,
	dlrService *application.DLRService,
) *Server {
	return &Server{
		messageService: messageService,
		dlrService:     dlrService,
	}
}

// SendMessage отправляет одно SMS сообщение
func (s *Server) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	// Валидация запроса
	if req.Source == "" {
		return nil, status.Error(codes.InvalidArgument, "source is required")
	}
	if req.Destination == "" {
		return nil, status.Error(codes.InvalidArgument, "destination is required")
	}
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	// Парсим client_id
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	// Создаем опции
	options := &application.SendMessageOptions{
		ExternalID: req.ExternalId,
		Priority:   int(req.Priority),
	}

	if req.RegisteredDelivery {
		options.RegisteredDelivery = &req.RegisteredDelivery
	}
	if req.ValidityPeriod != nil {
		validityPeriod := req.ValidityPeriod.AsTime()
		options.ValidityPeriod = &validityPeriod
	}
	if req.ServiceType != "" {
		options.ServiceType = req.ServiceType
	}
	if req.SourceAddrTon >= 0 {
		options.SourceAddrTON = int(req.SourceAddrTon)
	}
	if req.SourceAddrNpi >= 0 {
		options.SourceAddrNPI = int(req.SourceAddrNpi)
	}
	if req.DestAddrTon >= 0 {
		options.DestAddrTON = int(req.DestAddrTon)
	}
	if req.DestAddrNpi >= 0 {
		options.DestAddrNPI = int(req.DestAddrNpi)
	}
	if req.DataCoding >= 0 {
		options.DataCoding = int(req.DataCoding)
	}
	if req.ScheduledAt != nil {
		scheduledAt := req.ScheduledAt.AsTime()
		options.ScheduledAt = &scheduledAt
	}

	// Отправляем сообщение
	msg, err := s.messageService.SendMessage(ctx, clientID, req.Source, req.Destination, req.Text, options)
	if err != nil {
		log.Error().Err(err).Msg("ошибка отправки сообщения")
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &messagingv1.SendMessageResponse{
		MessageId:    msg.ID.String(),
		Status:       string(msg.Status),
		CreatedAt:    timestamppb.New(msg.CreatedAt),
		SegmentCount: int32(msg.SegmentCount),
	}
	if msg.ScheduledAt != nil {
		resp.ScheduledAt = timestamppb.New(*msg.ScheduledAt)
	}
	return resp, nil
}

// SendBatch отправляет пакет SMS сообщений
func (s *Server) SendBatch(ctx context.Context, req *messagingv1.SendBatchRequest) (*messagingv1.SendBatchResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if len(req.Messages) == 0 {
		return nil, status.Error(codes.InvalidArgument, "messages list is empty")
	}

	// Парсим client_id
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	// Преобразуем запросы
	requests := make([]*application.SendMessageRequest, len(req.Messages))
	for i, m := range req.Messages {
		req := &application.SendMessageRequest{
			Source:      m.Source,
			Destination: m.Destination,
			Text:        m.Text,
			ExternalID:  m.ExternalId,
			Priority:    int(m.Priority),
		}

		if m.RegisteredDelivery {
			req.RegisteredDelivery = &m.RegisteredDelivery
		}
		if m.ValidityPeriod != nil {
			validityPeriod := m.ValidityPeriod.AsTime()
			req.ValidityPeriod = &validityPeriod
		}
		if m.ServiceType != "" {
			req.ServiceType = m.ServiceType
		}
		if m.SourceAddrTon >= 0 {
			req.SourceAddrTON = int(m.SourceAddrTon)
		}
		if m.SourceAddrNpi >= 0 {
			req.SourceAddrNPI = int(m.SourceAddrNpi)
		}
		if m.DestAddrTon >= 0 {
			req.DestAddrTON = int(m.DestAddrTon)
		}
		if m.DestAddrNpi >= 0 {
			req.DestAddrNPI = int(m.DestAddrNpi)
		}
		if m.DataCoding >= 0 {
			req.DataCoding = int(m.DataCoding)
		}

		requests[i] = req
	}

	// Извлекаем batch-level scheduled_at
	var scheduledAt *time.Time
	if req.ScheduledAt != nil {
		t := req.ScheduledAt.AsTime()
		scheduledAt = &t
	}

	// Отправляем пакет
	results, err := s.messageService.SendBatch(ctx, clientID, requests, scheduledAt)
	if err != nil {
		log.Error().Err(err).Msg("ошибка пакетной отправки сообщений")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Преобразуем результаты
	protoResults := make([]*messagingv1.SendMessageResponse, len(results))
	successCount := int32(0)
	failedCount := int32(0)

	for i, result := range results {
		if result.Success {
			successCount++
			protoResults[i] = &messagingv1.SendMessageResponse{
				MessageId:    result.MessageID.String(),
				Status:       "queued",
				SegmentCount: int32(result.SegmentCount),
			}
		} else {
			failedCount++
			protoResults[i] = &messagingv1.SendMessageResponse{
				Status: "failed",
				Error:  result.Error,
			}
		}
	}

	return &messagingv1.SendBatchResponse{
		Results:     protoResults,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

// GetMessageStatus получает статус сообщения по ID
func (s *Server) GetMessageStatus(ctx context.Context, req *messagingv1.GetMessageStatusRequest) (*messagingv1.GetMessageStatusResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
	}

	var clientID *uuid.UUID
	if req.ClientId != "" {
		parsedClientID, err := uuid.Parse(req.ClientId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
		}
		clientID = &parsedClientID
	}

	// Получаем сообщение
	msg, err := s.messageService.GetMessageStatus(ctx, messageID, clientID)
	if err != nil {
		if err.Error() == "message not found" {
			return nil, status.Error(codes.NotFound, "message not found")
		}
		if err.Error() == "access denied" {
			return nil, status.Error(codes.PermissionDenied, "access denied")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	response := &messagingv1.GetMessageStatusResponse{
		MessageId:     msg.ID.String(),
		Status:        string(msg.Status),
		StatusMessage: msg.StatusMessage,
		CreatedAt:     timestamppb.New(msg.CreatedAt),
		SegmentCount:  int32(msg.SegmentCount),
	}

	if msg.SubmittedAt != nil {
		response.SubmittedAt = timestamppb.New(*msg.SubmittedAt)
	}
	if msg.DeliveredAt != nil {
		response.DeliveredAt = timestamppb.New(*msg.DeliveredAt)
	}
	if msg.FailedAt != nil {
		response.FailedAt = timestamppb.New(*msg.FailedAt)
	}
	if msg.SMPPMessageID != "" {
		response.SmppMessageId = msg.SMPPMessageID
	}
	if msg.ScheduledAt != nil {
		response.ScheduledAt = timestamppb.New(*msg.ScheduledAt)
	}
	if msg.ExpiredAt != nil {
		response.ExpiredAt = timestamppb.New(*msg.ExpiredAt)
	}

	return response, nil
}

// GetMessageHistory получает историю сообщений с фильтрацией
func (s *Server) GetMessageHistory(ctx context.Context, req *messagingv1.GetMessageHistoryRequest) (*messagingv1.GetMessageHistoryResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	filters := &application.MessageHistoryFilters{
		Limit: 100,
		Offset: 0,
	}

	if req.From != nil {
		from := req.From.AsTime()
		filters.From = &from
	}
	if req.To != nil {
		to := req.To.AsTime()
		filters.To = &to
	}
	if req.Status != "" {
		filters.Status = req.Status
	}
	if req.Destination != "" {
		filters.Destination = req.Destination
	}
	if req.Limit > 0 {
		filters.Limit = int(req.Limit)
	}
	if req.Offset > 0 {
		filters.Offset = int(req.Offset)
	}

	// Получаем историю
	messages, err := s.messageService.GetMessageHistory(ctx, clientID, filters)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения истории сообщений")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Преобразуем в proto формат
	protoMessages := make([]*messagingv1.MessageInfo, len(messages))
	for i, msg := range messages {
		protoMsg := &messagingv1.MessageInfo{
			MessageId:    msg.ID.String(),
			Source:       msg.Source,
			Destination:  msg.Destination,
			Text:         msg.Text,
			Status:       string(msg.Status),
			ExternalId:   msg.ExternalID,
			CreatedAt:    timestamppb.New(msg.CreatedAt),
			SegmentCount: int32(msg.SegmentCount),
		}

		if msg.ClientID != nil {
			protoMsg.ClientId = msg.ClientID.String()
		}
		if msg.SubmittedAt != nil {
			protoMsg.SubmittedAt = timestamppb.New(*msg.SubmittedAt)
		}
		if msg.DeliveredAt != nil {
			protoMsg.DeliveredAt = timestamppb.New(*msg.DeliveredAt)
		}
		if msg.FailedAt != nil {
			protoMsg.FailedAt = timestamppb.New(*msg.FailedAt)
		}
		if msg.ProviderID != nil {
			protoMsg.ProviderId = msg.ProviderID.String()
		}
		if msg.RouteID != nil {
			protoMsg.RouteId = msg.RouteID.String()
		}
		if msg.ScheduledAt != nil {
			protoMsg.ScheduledAt = timestamppb.New(*msg.ScheduledAt)
		}
		if msg.ExpiredAt != nil {
			protoMsg.ExpiredAt = timestamppb.New(*msg.ExpiredAt)
		}

		protoMessages[i] = protoMsg
	}

	return &messagingv1.GetMessageHistoryResponse{
		Messages: protoMessages,
		Total:    int32(len(protoMessages)),
		Limit:    int32(filters.Limit),
		Offset:   int32(filters.Offset),
	}, nil
}

// ProcessDLR обрабатывает delivery receipt
func (s *Server) ProcessDLR(ctx context.Context, req *messagingv1.ProcessDLRRequest) (*messagingv1.ProcessDLRResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.SmppMessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "smpp_message_id is required")
	}
	if req.Stat == "" {
		return nil, status.Error(codes.InvalidArgument, "stat is required")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
	}

	var doneDate *time.Time
	if req.DoneDate != nil {
		done := req.DoneDate.AsTime()
		doneDate = &done
	}

	var errCode *int
	if req.Err != 0 {
		errCodeVal := int(req.Err)
		errCode = &errCodeVal
	}

	var providerID *uuid.UUID
	if req.ProviderId != "" {
		parsedProviderID, err := uuid.Parse(req.ProviderId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid provider_id format")
		}
		providerID = &parsedProviderID
	}

	// Обрабатываем DLR
	err = s.dlrService.ProcessDLR(ctx, messageID, req.SmppMessageId, req.Stat, doneDate, errCode, req.Text, providerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обработки DLR")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Получаем обновленный статус сообщения
	msg, err := s.messageService.GetMessageStatus(ctx, messageID, nil)
	if err != nil {
		log.Warn().Err(err).Msg("не удалось получить статус сообщения после обработки DLR")
		// Возвращаем успешный ответ, т.к. DLR обработан
		return &messagingv1.ProcessDLRResponse{
			Success:       true,
			UpdatedStatus: "unknown",
		}, nil
	}

	return &messagingv1.ProcessDLRResponse{
		Success:       true,
		UpdatedStatus: string(msg.Status),
	}, nil
}

// CancelMessage отменяет запланированное сообщение
func (s *Server) CancelMessage(ctx context.Context, req *messagingv1.CancelMessageRequest) (*messagingv1.CancelMessageResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id is required")
	}
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message_id format")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id format")
	}

	if err := s.messageService.CancelMessage(ctx, messageID, clientID); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "message not found or not in scheduled status")
	}

	return &messagingv1.CancelMessageResponse{Success: true}, nil
}
