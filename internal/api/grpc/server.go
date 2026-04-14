package grpc

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/smsv1"
	"github.com/smpp-server/smpp-server/internal/api/middleware"
	"github.com/smpp-server/smpp-server/internal/pipeline/trace"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Server реализует gRPC сервис для SMS
type Server struct {
	smsv1.UnimplementedSMSServiceServer
	producer       MessageProducer
	asyncProducer  BatchMessagePublisher
	topicOutgoing  string
	messageRepo    MessageRepository
	clientRepo     ClientRepository
}

// NewServer создает новый gRPC сервер
func NewServer(
	producer MessageProducer,
	messageRepo MessageRepository,
	clientRepo ClientRepository,
) *Server {
	return &Server{
		producer:    producer,
		messageRepo: messageRepo,
		clientRepo:  clientRepo,
	}
}

// SendSMS отправляет одно SMS сообщение
func (s *Server) SendSMS(ctx context.Context, req *smsv1.SendSMSRequest) (*smsv1.SendSMSResponse, error) {
	// Валидация
	if req.Source == "" {
		return nil, status.Error(codes.InvalidArgument, "поле source обязательно")
	}
	if req.Destination == "" {
		return nil, status.Error(codes.InvalidArgument, "поле destination обязательно")
	}
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "поле text обязательно")
	}
	if len(req.Text) > 1600 {
		return nil, status.Error(codes.InvalidArgument, "текст сообщения слишком длинный (максимум 1600 символов)")
	}
	if req.Priority < 0 || req.Priority > 3 {
		return nil, status.Error(codes.InvalidArgument, "приоритет должен быть в диапазоне 0-3")
	}

	// Получаем клиента из контекста (должен быть установлен interceptor)
	clientID, ok := middleware.GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	// Создаем сообщение
	msg := &shared.Message{
		ID:                uuid.New(),
		Source:            req.Source,
		Destination:       req.Destination,
		Text:              req.Text,
		ExternalID:        shared.NullString(req.ExternalId),
		PriorityFlag:     int(req.Priority),
		RegisteredDelivery: boolToInt(req.RegisteredDelivery),
		ServiceType:       req.ServiceType,
		SourceAddrTON:     int(req.SourceAddrTon),
		SourceAddrNPI:     int(req.SourceAddrNpi),
		DestAddrTON:       int(req.DestAddrTon),
		DestAddrNPI:       int(req.DestAddrNpi),
		DataCoding:        int(req.DataCoding),
		Status:            shared.MessageStatusPending,
		ClientID:          &clientID,
		MaxRetries:        5,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if req.ValidityPeriod != nil {
		validityPeriod := req.ValidityPeriod.AsTime()
		msg.ValidityPeriod = &validityPeriod
	}

	// Определяем кодировку
	msg.Encoding = detectEncoding(msg.Text)

	traceID := uuid.New().String()

	// Сохраняем в БД
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Error().Err(err).Msg("ошибка сохранения сообщения")
		return nil, status.Error(codes.Internal, "ошибка сохранения сообщения")
	}

	// Публикуем в Kafka
	kafkaMsg := queue.FromMessage(msg)
	kafkaMsg.TraceID = traceID

	trace.Log(log.Logger, traceID, msg.ID.String(), "api", "receive").
		Str("client_id", clientID.String()).
		Str("source", req.Source).
		Str("destination", req.Destination).
		Int("text_length", len(req.Text)).
		Msg("message received via gRPC")

	if err := s.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
		log.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
		return nil, status.Error(codes.Internal, "ошибка публикации сообщения в очередь")
	}

	// Обновляем статус на queued
	msg.Status = shared.MessageStatusQueued
	msg.UpdatedAt = time.Now()
	if err := s.messageRepo.UpdateStatus(ctx, msg.ID, msg.Status, ""); err != nil {
		log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
	}

	return &smsv1.SendSMSResponse{
		MessageId: msg.ID.String(),
		Status:    string(msg.Status),
	}, nil
}

// SendBatchSMS отправляет пакет SMS сообщений
func (s *Server) SendBatchSMS(ctx context.Context, req *smsv1.SendBatchRequest) (*smsv1.SendBatchResponse, error) {
	if len(req.Messages) == 0 {
		return nil, status.Error(codes.InvalidArgument, "список сообщений пуст")
	}

	// Получаем клиента из контекста
	clientID, ok := middleware.GetClientID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "клиент не найден")
	}

	results := make([]*smsv1.SendSMSResponse, 0, len(req.Messages))
	successCount := int32(0)
	failedCount := int32(0)

	for _, msgReq := range req.Messages {
		// Валидация
		if msgReq.Source == "" || msgReq.Destination == "" || msgReq.Text == "" {
			results = append(results, &smsv1.SendSMSResponse{
				Status: "failed",
				Error:  "неверный формат запроса",
			})
			failedCount++
			continue
		}

		// Создаем сообщение
		msg := &shared.Message{
			ID:                uuid.New(),
			Source:            msgReq.Source,
			Destination:       msgReq.Destination,
			Text:              msgReq.Text,
			ExternalID:        shared.NullString(msgReq.ExternalId),
			PriorityFlag:     int(msgReq.Priority),
			RegisteredDelivery: boolToInt(msgReq.RegisteredDelivery),
			ServiceType:       msgReq.ServiceType,
			SourceAddrTON:     int(msgReq.SourceAddrTon),
			SourceAddrNPI:     int(msgReq.SourceAddrNpi),
			DestAddrTON:       int(msgReq.DestAddrTon),
			DestAddrNPI:       int(msgReq.DestAddrNpi),
			DataCoding:        int(msgReq.DataCoding),
			Status:            shared.MessageStatusPending,
			ClientID:          &clientID,
			MaxRetries:        5,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		if msgReq.ValidityPeriod != nil {
			validityPeriod := msgReq.ValidityPeriod.AsTime()
			msg.ValidityPeriod = &validityPeriod
		}

		msg.Encoding = detectEncoding(msg.Text)

		// Сохраняем в БД
		if err := s.messageRepo.Create(ctx, msg); err != nil {
			log.Error().Err(err).Msg("ошибка сохранения сообщения")
			results = append(results, &smsv1.SendSMSResponse{
				Status: "failed",
				Error:  "ошибка сохранения сообщения",
			})
			failedCount++
			continue
		}

		// Публикуем в Kafka
		kafkaMsg := queue.FromMessage(msg)
		kafkaMsg.TraceID = uuid.New().String()

		if s.asyncProducer != nil {
			// Асинхронная пакетная публикация через AsyncProducer (T031)
			data, err := kafkaMsg.Serialize()
			if err != nil {
				log.Error().Err(err).Msg("ошибка сериализации сообщения для Kafka")
				results = append(results, &smsv1.SendSMSResponse{
					MessageId: msg.ID.String(),
					Status:    "failed",
					Error:     "ошибка сериализации сообщения",
				})
				failedCount++
				continue
			}

			headers := []sarama.RecordHeader{
				{Key: []byte("message_id"), Value: []byte(kafkaMsg.MessageID.String())},
				{Key: []byte("source"), Value: []byte(kafkaMsg.Source)},
				{Key: []byte("destination"), Value: []byte(kafkaMsg.Destination)},
			}
			s.asyncProducer.PublishAsync(s.topicOutgoing, kafkaMsg.MessageID.String(), data, headers)
		} else {
			// Синхронная публикация (fallback)
			if err := s.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
				log.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
				results = append(results, &smsv1.SendSMSResponse{
					MessageId: msg.ID.String(),
					Status:    "failed",
					Error:     "ошибка публикации в очередь",
				})
				failedCount++
				continue
			}
		}

		// Обновляем статус
		msg.Status = shared.MessageStatusQueued
		msg.UpdatedAt = time.Now()
		if err := s.messageRepo.UpdateStatus(ctx, msg.ID, msg.Status, ""); err != nil {
			log.Warn().Err(err).Msg("ошибка обновления статуса сообщения")
		}

		results = append(results, &smsv1.SendSMSResponse{
			MessageId: msg.ID.String(),
			Status:    string(msg.Status),
		})
		successCount++
	}

	log.Info().
		Str("client_id", clientID.String()).
		Int32("success_count", successCount).
		Int32("failed_count", failedCount).
		Int("total", len(req.Messages)).
		Msg("batch SMS received via gRPC")

	return &smsv1.SendBatchResponse{
		Results:     results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
	}, nil
}

// GetStatus получает статус сообщения по ID
func (s *Server) GetStatus(ctx context.Context, req *smsv1.GetStatusRequest) (*smsv1.GetStatusResponse, error) {
	if req.MessageId == "" {
		return nil, status.Error(codes.InvalidArgument, "поле message_id обязательно")
	}

	messageID, err := uuid.Parse(req.MessageId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "неверный формат ID сообщения")
	}

	// Получаем сообщение из БД
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, status.Error(codes.NotFound, "сообщение не найдено")
		}
		log.Error().Err(err).Msg("ошибка получения сообщения")
		return nil, status.Error(codes.Internal, "ошибка получения сообщения")
	}

	// Проверяем права доступа
	clientID, ok := middleware.GetClientID(ctx)
	if ok && msg.ClientID != nil && *msg.ClientID != clientID {
		return nil, status.Error(codes.PermissionDenied, "нет доступа к этому сообщению")
	}

	resp := &smsv1.GetStatusResponse{
		MessageId:     msg.ID.String(),
		Status:        string(msg.Status),
		StatusMessage: string(msg.StatusMessage),
		CreatedAt:     timestamppb.New(msg.CreatedAt),
		SmppMessageId: string(msg.SMPPMessageID),
	}

	if msg.SubmittedAt != nil {
		resp.SubmittedAt = timestamppb.New(*msg.SubmittedAt)
	}
	if msg.DeliveredAt != nil {
		resp.DeliveredAt = timestamppb.New(*msg.DeliveredAt)
	}
	if msg.FailedAt != nil {
		resp.FailedAt = timestamppb.New(*msg.FailedAt)
	}

	return resp, nil
}

// StreamDLR получает поток delivery receipts
func (s *Server) StreamDLR(req *smsv1.StreamDLRRequest, stream smsv1.SMSService_StreamDLRServer) error {
	// TODO: Реализовать стриминг DLR через Kafka consumer или WebSocket
	// Пока возвращаем ошибку "не реализовано"
	return status.Error(codes.Unimplemented, "метод StreamDLR пока не реализован")
}

// Вспомогательные функции

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func detectEncoding(text string) shared.MessageEncoding {
	for _, r := range text {
		if r > 127 {
			return shared.MessageEncodingUCS2
		}
	}
	return shared.MessageEncodingGSM7
}
