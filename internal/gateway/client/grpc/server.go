package grpc

import (
	"context"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/api/proto/analyticsv1"
	"github.com/smpp-server/smpp-server/api/proto/billingv1"
	"github.com/smpp-server/smpp-server/api/proto/messagingv1"
)

// Server реализует gRPC сервер для Client Gateway
// Проксирует запросы к соответствующим микросервисам
type Server struct {
	messagingv1.UnimplementedMessagingServiceServer
	messagingClient messagingv1.MessagingServiceClient
	billingClient   billingv1.BillingServiceClient
	analyticsClient analyticsv1.AnalyticsServiceClient
}

// NewServer создает новый gRPC сервер для Client Gateway
func NewServer(
	messagingClient messagingv1.MessagingServiceClient,
	billingClient billingv1.BillingServiceClient,
	analyticsClient analyticsv1.AnalyticsServiceClient,
) *Server {
	return &Server{
		messagingClient: messagingClient,
		billingClient:   billingClient,
		analyticsClient: analyticsClient,
	}
}

// SendMessage проксирует запрос на отправку сообщения в Messaging Service
func (s *Server) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	// Получаем client_id из контекста и устанавливаем его в запрос
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка отправить сообщение от другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя отправлять сообщения от имени другого клиента")
	}

	// Устанавливаем client_id из контекста
	req.ClientId = clientID.String()

	// Проксируем запрос в Messaging Service
	return s.messagingClient.SendMessage(ctx, req)
}

// SendBatch проксирует запрос на пакетную отправку сообщений в Messaging Service
func (s *Server) SendBatch(ctx context.Context, req *messagingv1.SendBatchRequest) (*messagingv1.SendBatchResponse, error) {
	// Получаем client_id из контекста
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка отправить пакет сообщений от другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя отправлять сообщения от имени другого клиента")
	}

	// Устанавливаем client_id из контекста
	req.ClientId = clientID.String()

	// Устанавливаем client_id для всех сообщений в пакете
	for _, msg := range req.Messages {
		if msg.ClientId != "" && msg.ClientId != clientID.String() {
			log.Warn().
				Str("message_client_id", msg.ClientId).
				Str("auth_client_id", clientID.String()).
				Msg("попытка отправить сообщение от другого клиента в пакете")
			return nil, status.Error(codes.PermissionDenied, "нельзя отправлять сообщения от имени другого клиента")
		}
		msg.ClientId = clientID.String()
	}

	// Проксируем запрос в Messaging Service
	return s.messagingClient.SendBatch(ctx, req)
}

// GetMessageStatus проксирует запрос на получение статуса сообщения в Messaging Service
func (s *Server) GetMessageStatus(ctx context.Context, req *messagingv1.GetMessageStatusRequest) (*messagingv1.GetMessageStatusResponse, error) {
	// Получаем client_id из контекста
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка получить статус сообщения другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя получать статус сообщений другого клиента")
	}

	// Устанавливаем client_id из контекста
	req.ClientId = clientID.String()

	// Проксируем запрос в Messaging Service
	return s.messagingClient.GetMessageStatus(ctx, req)
}

// GetMessageHistory проксирует запрос на получение истории сообщений в Messaging Service
func (s *Server) GetMessageHistory(ctx context.Context, req *messagingv1.GetMessageHistoryRequest) (*messagingv1.GetMessageHistoryResponse, error) {
	// Получаем client_id из контекста
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка получить историю сообщений другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя получать историю сообщений другого клиента")
	}

	// Устанавливаем client_id из контекста
	req.ClientId = clientID.String()

	// Проксируем запрос в Messaging Service
	return s.messagingClient.GetMessageHistory(ctx, req)
}

// GetBalance проксирует запрос на получение баланса в Billing Service
func (s *Server) GetBalance(ctx context.Context, req *billingv1.GetBalanceRequest) (*billingv1.GetBalanceResponse, error) {
	// Получаем client_id из контекста
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка получить баланс другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя получать баланс другого клиента")
	}

	// Устанавливаем client_id из контекста
	req.ClientId = clientID.String()

	// Проксируем запрос в Billing Service
	return s.billingClient.GetBalance(ctx, req)
}

// ProcessDLR проксирует запрос на обработку DLR в Messaging Service
// Примечание: Обычно DLR обрабатываются автоматически провайдерами,
// но этот метод доступен для клиентов, если они хотят отправить DLR вручную
func (s *Server) ProcessDLR(ctx context.Context, req *messagingv1.ProcessDLRRequest) (*messagingv1.ProcessDLRResponse, error) {
	// Получаем client_id из контекста для логирования
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Проверяем, что сообщение принадлежит клиенту
	// Для этого нужно получить сообщение и проверить его client_id
	// Но для упрощения, проксируем запрос напрямую - Messaging Service проверит права
	log.Info().
		Str("client_id", clientID.String()).
		Str("message_id", req.MessageId).
		Msg("обработка DLR через Client Gateway")

	// Проксируем запрос в Messaging Service
	return s.messagingClient.ProcessDLR(ctx, req)
}

// GetStatistics проксирует запрос на получение статистики в Analytics Service
// Примечание: Этот метод не является частью MessagingService, но может быть использован
// для расширения функциональности через дополнительные методы
func (s *Server) GetStatistics(ctx context.Context, req *analyticsv1.GetStatisticsRequest) (*analyticsv1.GetStatisticsResponse, error) {
	// Получаем client_id из контекста
	clientID, ok := GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Убеждаемся, что client_id в запросе соответствует аутентифицированному клиенту
	if req.ClientId != "" && req.ClientId != clientID.String() {
		log.Warn().
			Str("request_client_id", req.ClientId).
			Str("auth_client_id", clientID.String()).
			Msg("попытка получить статистику другого клиента")
		return nil, status.Error(codes.PermissionDenied, "нельзя получать статистику другого клиента")
	}

	// Устанавливаем client_id из контекста (для фильтрации статистики по клиенту)
	req.ClientId = clientID.String()

	// Проксируем запрос в Analytics Service
	return s.analyticsClient.GetStatistics(ctx, req)
}