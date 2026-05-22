package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// Handler обрабатывает SMPP команды
type Handler struct {
	session      *Session
	decoder      *protocol.Decoder
	encoder      *protocol.Encoder
	validator    *protocol.Validator
	clientRepo   *storage.ClientRepository
	messageRepo  *storage.MessageRepository
	producer     *queue.Producer
	logger       zerolog.Logger
}

// NewHandler создает новый обработчик команд
func NewHandler(
	session *Session,
	clientRepo *storage.ClientRepository,
	messageRepo *storage.MessageRepository,
	producer *queue.Producer,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		session:     session,
		decoder:     protocol.NewDecoder(nil),
		encoder:     protocol.NewEncoder(),
		validator:   protocol.NewValidator(),
		clientRepo:  clientRepo,
		messageRepo: messageRepo,
		producer:    producer,
		logger:      logger,
	}
}

// HandlePDU обрабатывает входящий PDU
func (h *Handler) HandlePDU(pdu *protocol.PDU) error {
	h.session.UpdateActivity()
	
	// Валидация базового PDU
	if err := h.validator.ValidatePDU(pdu); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации PDU")
		return h.sendGenericNack(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN)
	}
	
	// Проверяем, является ли это ответом
	if protocol.IsResponse(pdu.CommandID) {
		return h.handleResponse(pdu)
	}
	
	// Обрабатываем команду в зависимости от типа
	switch pdu.CommandID {
	case protocol.BindReceiver:
		return h.handleBindReceiver(pdu)
	case protocol.BindTransmitter:
		return h.handleBindTransmitter(pdu)
	case protocol.BindTransceiver:
		return h.handleBindTransceiver(pdu)
	case protocol.Unbind:
		return h.handleUnbind(pdu)
	case protocol.SubmitSM:
		return h.handleSubmitSM(pdu)
	case protocol.EnquireLink:
		return h.handleEnquireLink(pdu)
	case protocol.QuerySM:
		return h.handleQuerySM(pdu)
	case protocol.CancelSM:
		return h.handleCancelSM(pdu)
	case protocol.ReplaceSM:
		return h.handleReplaceSM(pdu)
	default:
		h.logger.Warn().
			Uint32("command_id", pdu.CommandID).
			Str("command_name", protocol.GetCommandName(pdu.CommandID)).
			Msg("неподдерживаемая команда")
		return h.sendGenericNack(pdu.SequenceNumber, protocol.ESME_RINVCMDID)
	}
}

// handleResponse обрабатывает ответные PDU (обычно игнорируем)
func (h *Handler) handleResponse(pdu *protocol.PDU) error {
	h.logger.Debug().
		Uint32("command_id", pdu.CommandID).
		Str("command_name", protocol.GetCommandName(pdu.CommandID)).
		Msg("получен ответный PDU")
	return nil
}

// handleBindReceiver обрабатывает bind_receiver
func (h *Handler) handleBindReceiver(pdu *protocol.PDU) error {
	bind, err := h.decoder.DecodeBind(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования bind_receiver")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindReceiverResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	if err := h.validator.ValidateBind(bind); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации bind_receiver")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindReceiverResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	// Проверяем аутентификацию
	client, err := h.authenticate(bind.SystemID, bind.Password)
	if err != nil {
		h.logger.Warn().
			Str("system_id", bind.SystemID).
			Err(err).
			Msg("ошибка аутентификации")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindReceiverResp, protocol.ESME_RINVPASWD, "")
	}
	
	// Привязываем сессию
	var clientID *uuid.UUID
	rateLimit := 0
	if client != nil {
		clientID = &client.ID
		rateLimit = client.RateLimitPerSecond
	}

	if err := h.session.Bind("receiver", bind.SystemID, clientID, rateLimit); err != nil {
		h.logger.Error().Err(err).Msg("ошибка привязки сессии")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindReceiverResp, protocol.ESME_RINVBNDSTS, "")
	}
	
	// Отправляем успешный ответ
	return h.sendBindResp(pdu.SequenceNumber, protocol.BindReceiverResp, protocol.ESME_ROK, bind.SystemID)
}

// handleBindTransmitter обрабатывает bind_transmitter
func (h *Handler) handleBindTransmitter(pdu *protocol.PDU) error {
	bind, err := h.decoder.DecodeBind(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования bind_transmitter")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransmitterResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	if err := h.validator.ValidateBind(bind); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации bind_transmitter")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransmitterResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	// Проверяем аутентификацию
	client, err := h.authenticate(bind.SystemID, bind.Password)
	if err != nil {
		h.logger.Warn().
			Str("system_id", bind.SystemID).
			Err(err).
			Msg("ошибка аутентификации")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransmitterResp, protocol.ESME_RINVPASWD, "")
	}
	
	// Привязываем сессию
	var clientID *uuid.UUID
	rateLimit := 0
	if client != nil {
		clientID = &client.ID
		rateLimit = client.RateLimitPerSecond
	}

	if err := h.session.Bind("transmitter", bind.SystemID, clientID, rateLimit); err != nil {
		h.logger.Error().Err(err).Msg("ошибка привязки сессии")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransmitterResp, protocol.ESME_RINVBNDSTS, "")
	}
	
	// Отправляем успешный ответ
	return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransmitterResp, protocol.ESME_ROK, bind.SystemID)
}

// handleBindTransceiver обрабатывает bind_transceiver
func (h *Handler) handleBindTransceiver(pdu *protocol.PDU) error {
	bind, err := h.decoder.DecodeBind(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования bind_transceiver")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransceiverResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	if err := h.validator.ValidateBind(bind); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации bind_transceiver")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransceiverResp, protocol.ESME_RINVCMDLEN, "")
	}
	
	// Проверяем аутентификацию
	client, err := h.authenticate(bind.SystemID, bind.Password)
	if err != nil {
		h.logger.Warn().
			Str("system_id", bind.SystemID).
			Err(err).
			Msg("ошибка аутентификации")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransceiverResp, protocol.ESME_RINVPASWD, "")
	}
	
	// Привязываем сессию
	var clientID *uuid.UUID
	rateLimit := 0
	if client != nil {
		clientID = &client.ID
		rateLimit = client.RateLimitPerSecond
	}

	if err := h.session.Bind("transceiver", bind.SystemID, clientID, rateLimit); err != nil {
		h.logger.Error().Err(err).Msg("ошибка привязки сессии")
		return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransceiverResp, protocol.ESME_RINVBNDSTS, "")
	}
	
	// Отправляем успешный ответ
	return h.sendBindResp(pdu.SequenceNumber, protocol.BindTransceiverResp, protocol.ESME_ROK, bind.SystemID)
}

// handleUnbind обрабатывает unbind
func (h *Handler) handleUnbind(pdu *protocol.PDU) error {
	if !h.session.IsBound() {
		return h.sendUnbindResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS)
	}
	
	if err := h.session.Unbind(); err != nil {
		h.logger.Error().Err(err).Msg("ошибка отвязки сессии")
		return h.sendUnbindResp(pdu.SequenceNumber, protocol.ESME_RSYSERR)
	}
	
	return h.sendUnbindResp(pdu.SequenceNumber, protocol.ESME_ROK)
}

// handleSubmitSM обрабатывает submit_sm
func (h *Handler) handleSubmitSM(pdu *protocol.PDU) error {
	startTime := time.Now()
	defer func() {
		monitoring.SMPPProcessingDuration.WithLabelValues("submit_sm").Observe(time.Since(startTime).Seconds())
	}()

	// Проверяем, что сессия может отправлять сообщения
	if !h.session.CanSend() {
		monitoring.SMPPMessagesFailed.WithLabelValues("", "", "invalid_bind_status").Inc()
		return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS, "")
	}
	
	// Проверяем rate limit
	if err := h.session.CheckRateLimit(); err != nil {
		h.logger.Warn().Msg("превышен rate limit")
		monitoring.SMPPMessagesFailed.WithLabelValues("", "", "rate_limit_exceeded").Inc()
		return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_RTHROTTLED, "")
	}
	
	// Декодируем submit_sm
	submit, err := h.decoder.DecodeSubmitSM(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования submit_sm")
		monitoring.SMPPMessagesFailed.WithLabelValues("", "", "decode_error").Inc()
		return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN, "")
	}
	
	// Валидируем
	if err := h.validator.ValidateSubmitSM(submit); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации submit_sm")
		monitoring.SMPPMessagesFailed.WithLabelValues("", "", "validation_error").Inc()
		return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN, "")
	}
	
	// Создаем сообщение для Kafka
	msgID := uuid.New()
	messageID := fmt.Sprintf("%s-%d", h.session.ID, pdu.SequenceNumber)
	
	kafkaMsg := &queue.KafkaMessage{
		ID:          msgID.String(),
		MessageID:   msgID,
		Source:      submit.SourceAddr,
		Destination: submit.DestinationAddr,
		Text:        string(submit.ShortMessage),
		ClientID:    h.session.ClientID,
		Priority:    int(submit.PriorityFlag),
		RetryCount:  0,
		MaxRetries:  5,
		CreatedAt:   time.Now(),
		Metadata: map[string]interface{}{
			"smpp_session_id": h.session.ID,
			"smpp_sequence":    pdu.SequenceNumber,
			"source_ton":       submit.SourceAddrTON,
			"source_npi":       submit.SourceAddrNPI,
			"dest_ton":         submit.DestAddrTON,
			"dest_npi":         submit.DestAddrNPI,
			"data_coding":      submit.DataCoding,
			"esm_class":        submit.ESMClass,
			"registered_delivery": submit.RegisteredDelivery,
		},
	}
	
	// Публикуем в Kafka
	ctx := context.Background()
	if err := h.producer.PublishOutgoing(ctx, kafkaMsg); err != nil {
		h.logger.Error().Err(err).Msg("ошибка публикации сообщения в Kafka")
		monitoring.SMPPMessagesFailed.WithLabelValues("", "", "kafka_publish_error").Inc()
		return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_RSYSERR, "")
	}
	
	// Увеличиваем счетчик полученных сообщений
	clientIDStr := ""
	if h.session.ClientID != nil {
		clientIDStr = h.session.ClientID.String()
	}
	monitoring.SMPPMessagesReceived.WithLabelValues(clientIDStr, h.session.ID).Inc()
	
	h.logger.Info().
		Str("message_id", msgID.String()).
		Str("source", submit.SourceAddr).
		Str("destination", submit.DestinationAddr).
		Msg("сообщение принято и отправлено в очередь")
	
	// Отправляем успешный ответ
	return h.sendSubmitSMResp(pdu.SequenceNumber, protocol.ESME_ROK, messageID)
}

// handleEnquireLink обрабатывает enquire_link
func (h *Handler) handleEnquireLink(pdu *protocol.PDU) error {
	if !h.session.IsBound() {
		return h.sendEnquireLinkResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS)
	}
	
	h.session.mu.Lock()
	h.session.EnquireLinkSent = time.Now()
	h.session.mu.Unlock()
	
	return h.sendEnquireLinkResp(pdu.SequenceNumber, protocol.ESME_ROK)
}

// handleQuerySM обрабатывает query_sm
func (h *Handler) handleQuerySM(pdu *protocol.PDU) error {
	if !h.session.IsBound() {
		return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS, "", "", 0, 0)
	}

	query, err := h.decoder.DecodeQuerySM(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования query_sm")
		return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN, "", "", 0, 0)
	}

	if err := h.validator.ValidateQuerySM(query); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации query_sm")
		return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN, "", "", 0, 0)
	}

	if h.messageRepo == nil {
		h.logger.Warn().Msg("message repository не настроен, query_sm недоступен")
		return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_RQUERYFAIL, query.MessageID, "", 0, 0)
	}

	ctx := context.Background()
	msg, err := h.messageRepo.GetByMessageID(ctx, query.MessageID)
	if err != nil {
		h.logger.Warn().Err(err).Str("message_id", query.MessageID).Msg("сообщение не найдено для query_sm")
		return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_RQUERYFAIL, query.MessageID, "", 0, 0)
	}

	// Формируем final_date для завершенных сообщений
	finalDate := ""
	if msg.DeliveredAt != nil {
		finalDate = msg.DeliveredAt.Format("060102150405000") + "+"
	} else if msg.FailedAt != nil {
		finalDate = msg.FailedAt.Format("060102150405000") + "+"
	}

	smppState := mapMessageStatusToSMPP(msg.Status)
	h.logger.Info().
		Str("message_id", query.MessageID).
		Str("status", string(msg.Status)).
		Uint8("smpp_state", smppState).
		Msg("query_sm выполнен успешно")

	return h.sendQuerySMResp(pdu.SequenceNumber, protocol.ESME_ROK, string(msg.MessageID), finalDate, smppState, 0)
}

// handleCancelSM обрабатывает cancel_sm
func (h *Handler) handleCancelSM(pdu *protocol.PDU) error {
	if !h.session.IsBound() {
		return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS)
	}

	cancel, err := h.decoder.DecodeCancelSM(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования cancel_sm")
		return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN)
	}

	if err := h.validator.ValidateCancelSM(cancel); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации cancel_sm")
		return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN)
	}

	if h.messageRepo == nil {
		h.logger.Warn().Msg("message repository не настроен, cancel_sm недоступен")
		return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_RCANCELFAIL)
	}

	ctx := context.Background()
	err = h.messageRepo.UpdateStatusByMessageID(ctx, cancel.MessageID, shared.MessageStatusCancelled)
	if err != nil {
		h.logger.Warn().Err(err).Str("message_id", cancel.MessageID).Msg("не удалось отменить сообщение")
		return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_RCANCELFAIL)
	}

	h.logger.Info().Str("message_id", cancel.MessageID).Msg("сообщение отменено через cancel_sm")
	return h.sendCancelSMResp(pdu.SequenceNumber, protocol.ESME_ROK)
}

// handleReplaceSM обрабатывает replace_sm
func (h *Handler) handleReplaceSM(pdu *protocol.PDU) error {
	if !h.session.IsBound() {
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RINVBNDSTS)
	}

	replace, err := h.decoder.DecodeReplaceSM(pdu.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("ошибка декодирования replace_sm")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN)
	}

	if err := h.validator.ValidateReplaceSM(replace); err != nil {
		h.logger.Error().Err(err).Msg("ошибка валидации replace_sm")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RINVCMDLEN)
	}

	if h.messageRepo == nil {
		h.logger.Warn().Msg("message repository не настроен, replace_sm недоступен")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RREPLACEFAIL)
	}

	ctx := context.Background()
	msg, err := h.messageRepo.GetByMessageID(ctx, replace.MessageID)
	if err != nil {
		h.logger.Warn().Err(err).Str("message_id", replace.MessageID).Msg("сообщение не найдено для replace_sm")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RREPLACEFAIL)
	}

	// Замена допускается только для сообщений в статусе pending или queued
	if msg.Status != shared.MessageStatusPending && msg.Status != shared.MessageStatusQueued {
		h.logger.Warn().
			Str("message_id", replace.MessageID).
			Str("status", string(msg.Status)).
			Msg("замена невозможна: сообщение не в статусе pending/queued")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RREPLACEFAIL)
	}

	err = h.messageRepo.UpdateTextByMessageID(ctx, replace.MessageID, string(replace.ShortMessage))
	if err != nil {
		h.logger.Error().Err(err).Str("message_id", replace.MessageID).Msg("ошибка обновления текста сообщения")
		return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_RREPLACEFAIL)
	}

	h.logger.Info().Str("message_id", replace.MessageID).Msg("текст сообщения заменен через replace_sm")
	return h.sendReplaceSMResp(pdu.SequenceNumber, protocol.ESME_ROK)
}

// mapMessageStatusToSMPP преобразует внутренний статус сообщения в SMPP message state
func mapMessageStatusToSMPP(status shared.MessageStatus) byte {
	switch status {
	case shared.MessageStatusPending, shared.MessageStatusQueued:
		return protocol.MSG_STATE_ENROUTE
	case shared.MessageStatusSent:
		return protocol.MSG_STATE_ACCEPTED
	case shared.MessageStatusDelivered:
		return protocol.MSG_STATE_DELIVERED
	case shared.MessageStatusExpired:
		return protocol.MSG_STATE_EXPIRED
	case shared.MessageStatusFailed:
		return protocol.MSG_STATE_UNDELIVERABLE
	case shared.MessageStatusRejected:
		return protocol.MSG_STATE_REJECTED
	case shared.MessageStatusCancelled:
		return protocol.MSG_STATE_DELETED
	case shared.MessageStatusScheduled:
		return protocol.MSG_STATE_SCHEDULED
	default:
		return protocol.MSG_STATE_UNKNOWN
	}
}

// authenticate проверяет аутентификацию клиента
func (h *Handler) authenticate(systemID, password string) (*shared.Client, error) {
	if h.clientRepo == nil {
		// Если репозиторий не настроен, разрешаем подключение без проверки
		h.logger.Warn().Msg("client repository не настроен, пропускаем аутентификацию")
		return nil, nil
	}
	
	// Ищем клиента по system_id (предполагаем, что system_id = api_key или есть отдельная таблица)
	// Для упрощения используем system_id как идентификатор
	ctx := context.Background()
	client, err := h.clientRepo.GetByAPIKey(ctx, systemID)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("клиент не найден")
		}
		return nil, fmt.Errorf("ошибка поиска клиента: %w", err)
	}
	
	// Проверяем пароль (в реальной системе нужно использовать хеширование)
	if client.Secret != password {
		return nil, fmt.Errorf("неверный пароль")
	}
	
	// Проверяем активность
	if !client.Active {
		return nil, fmt.Errorf("клиент неактивен")
	}
	
	return client, nil
}

// sendBindResp отправляет bind response
func (h *Handler) sendBindResp(seqNum uint32, commandID uint32, status uint32, systemID string) error {
	resp := &protocol.BindRespPDU{
		SystemID: systemID,
		TLV:      make(map[uint16][]byte),
	}
	
	body, err := h.encoder.EncodeBindResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования bind_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      commandID,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendUnbindResp отправляет unbind_resp
func (h *Handler) sendUnbindResp(seqNum uint32, status uint32) error {
	resp := &protocol.UnbindRespPDU{}
	body, err := h.encoder.EncodeUnbindResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования unbind_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.UnbindResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendSubmitSMResp отправляет submit_sm_resp
func (h *Handler) sendSubmitSMResp(seqNum uint32, status uint32, messageID string) error {
	resp := &protocol.SubmitSMRespPDU{
		MessageID: messageID,
	}
	
	body, err := h.encoder.EncodeSubmitSMResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования submit_sm_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.SubmitSMResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendEnquireLinkResp отправляет enquire_link_resp
func (h *Handler) sendEnquireLinkResp(seqNum uint32, status uint32) error {
	resp := &protocol.EnquireLinkRespPDU{}
	body, err := h.encoder.EncodeEnquireLinkResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования enquire_link_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.EnquireLinkResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendQuerySMResp отправляет query_sm_resp
func (h *Handler) sendQuerySMResp(seqNum uint32, status uint32, messageID, finalDate string, messageState, errorCode byte) error {
	resp := &protocol.QuerySMRespPDU{
		MessageID:    messageID,
		FinalDate:    finalDate,
		MessageState: messageState,
		ErrorCode:    errorCode,
	}
	
	body, err := h.encoder.EncodeQuerySMResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования query_sm_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.QuerySMResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendCancelSMResp отправляет cancel_sm_resp
func (h *Handler) sendCancelSMResp(seqNum uint32, status uint32) error {
	resp := &protocol.CancelSMRespPDU{}
	body, err := h.encoder.EncodeCancelSMResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования cancel_sm_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.CancelSMResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendReplaceSMResp отправляет replace_sm_resp
func (h *Handler) sendReplaceSMResp(seqNum uint32, status uint32) error {
	resp := &protocol.ReplaceSMRespPDU{}
	body, err := h.encoder.EncodeReplaceSMResp(resp)
	if err != nil {
		return fmt.Errorf("ошибка кодирования replace_sm_resp: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.ReplaceSMResp,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendGenericNack отправляет generic_nack
func (h *Handler) sendGenericNack(seqNum uint32, status uint32) error {
	nack := &protocol.GenericNackPDU{}
	body, err := h.encoder.EncodeGenericNack(nack)
	if err != nil {
		return fmt.Errorf("ошибка кодирования generic_nack: %w", err)
	}
	
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.GenericNack,
		CommandStatus:  status,
		SequenceNumber: seqNum,
		Body:           body,
	}
	
	return h.sendPDU(pdu)
}

// sendPDU отправляет PDU через соединение
func (h *Handler) sendPDU(pdu *protocol.PDU) error {
	data, err := h.encoder.EncodePDU(pdu)
	if err != nil {
		return fmt.Errorf("ошибка кодирования PDU: %w", err)
	}
	
	if h.session.Conn == nil {
		return ErrConnectionClosed
	}
	
	_, err = h.session.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("ошибка записи PDU: %w", err)
	}
	
	return nil
}
