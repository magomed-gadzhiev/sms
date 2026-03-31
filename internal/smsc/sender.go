package smsc

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/shared"
	smppprotocol "github.com/smpp-server/smpp-server/internal/smpp/protocol"
)

// Sender отправляет SMS сообщения через SMPP соединения
type Sender struct {
	pool   *Pool
	logger zerolog.Logger
}

// NewSender создает новый отправитель
func NewSender(pool *Pool) *Sender {
	logger := log.With().Str("component", "smsc_sender").Logger()
	return &Sender{
		pool:   pool,
		logger: logger,
	}
}

// sendSinglePDU отправляет один submit_sm PDU и возвращает SMPP message ID
func (s *Sender) sendSinglePDU(conn *Connection, submitSM *smppprotocol.SubmitSMPDU, provider *shared.Provider) (string, error) {
	encoder := smppprotocol.NewEncoder()
	body, err := encoder.EncodeSubmitSM(submitSM)
	if err != nil {
		return "", fmt.Errorf("ошибка кодирования submit_sm: %w", err)
	}

	// Генерируем sequence number
	conn.mu.Lock()
	conn.SequenceNum++
	seqNum := conn.SequenceNum
	conn.mu.Unlock()

	// Создаем полный PDU
	pdu := &smppprotocol.PDU{
		CommandLength:  uint32(smppprotocol.PDUHeaderLength + len(body)),
		CommandID:      smppprotocol.SubmitSM,
		CommandStatus:  0,
		SequenceNumber: seqNum,
		Body:           body,
	}

	pduBytes, err := encoder.EncodePDU(pdu)
	if err != nil {
		return "", fmt.Errorf("ошибка кодирования PDU: %w", err)
	}

	// Отправляем запрос
	conn.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := conn.Conn.Write(pduBytes); err != nil {
		return "", fmt.Errorf("ошибка отправки submit_sm: %w", err)
	}

	// Читаем ответ
	conn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	header := make([]byte, smppprotocol.PDUHeaderLength)
	if _, err := conn.Conn.Read(header); err != nil {
		return "", fmt.Errorf("ошибка чтения ответа submit_sm: %w", err)
	}

	// Декодируем заголовок
	var respCommandLength uint32
	respCommandLength = uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	var respCommandStatus uint32
	respCommandStatus = uint32(header[8])<<24 | uint32(header[9])<<16 | uint32(header[10])<<8 | uint32(header[11])

	// Читаем тело ответа
	bodyLength := int(respCommandLength) - smppprotocol.PDUHeaderLength
	bodyData := make([]byte, bodyLength)
	if bodyLength > 0 {
		if _, err := conn.Conn.Read(bodyData); err != nil {
			return "", fmt.Errorf("ошибка чтения тела ответа submit_sm: %w", err)
		}
	}

	// Проверяем статус
	if respCommandStatus != smppprotocol.ESME_ROK {
		statusName := smppprotocol.GetStatusName(respCommandStatus)
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, statusName).Inc()
		return "", fmt.Errorf("submit_sm отклонен: статус %s (0x%08X)", statusName, respCommandStatus)
	}

	// Декодируем ответ
	decoder := smppprotocol.NewDecoder(bodyData)
	resp, err := decoder.DecodeSubmitSMResp(bodyData)
	if err != nil {
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "decode_error").Inc()
		return "", fmt.Errorf("ошибка декодирования ответа submit_sm: %w", err)
	}

	return resp.MessageID, nil
}

// SendMessage отправляет SMS сообщение через провайдера
func (s *Sender) SendMessage(ctx context.Context, msg *shared.Message, provider *shared.Provider) (string, error) {
	startTime := time.Now()
	defer func() {
		monitoring.SMPPProcessingDuration.WithLabelValues("send_to_provider").Observe(time.Since(startTime).Seconds())
	}()

	// Получаем соединение
	conn, err := s.pool.GetConnection(provider.ID)
	if err != nil {
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "connection_error").Inc()
		return "", fmt.Errorf("ошибка получения соединения: %w", err)
	}

	// Проверяем throttling
	if !conn.throttler.Allow() {
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "throttle_exceeded").Inc()
		return "", fmt.Errorf("превышен лимит скорости для провайдера %s", provider.Name)
	}

	// Проверяем, нужно ли разделять сообщение на сегменты
	if msg.SegmentCount > 1 {
		return s.sendMultipart(ctx, conn, msg, provider)
	}

	// Одиночное сообщение — отправляем как раньше
	submitSM := &smppprotocol.SubmitSMPDU{
		ServiceType:          msg.ServiceType,
		SourceAddrTON:        byte(msg.SourceAddrTON),
		SourceAddrNPI:        byte(msg.SourceAddrNPI),
		SourceAddr:           msg.Source,
		DestAddrTON:          byte(msg.DestAddrTON),
		DestAddrNPI:          byte(msg.DestAddrNPI),
		DestinationAddr:      msg.Destination,
		ESMClass:             byte(msg.ESMClass),
		ProtocolID:           byte(msg.ProtocolID),
		PriorityFlag:         byte(msg.PriorityFlag),
		ScheduleDeliveryTime: "",
		ValidityPeriod:       "",
		RegisteredDelivery:   byte(msg.RegisteredDelivery),
		ReplaceIfPresent:     byte(msg.ReplaceIfPresent),
		DataCoding:           byte(msg.DataCoding),
		SMDefaultMsgID:       0,
		SMLength:             byte(len(msg.Text)),
		ShortMessage:         []byte(msg.Text),
		TLV:                  make(map[uint16][]byte),
	}

	smppMsgID, err := s.sendSinglePDU(conn, submitSM, provider)
	if err != nil {
		return "", err
	}

	// Увеличиваем счетчик успешных отправок
	monitoring.SMPPMessagesSent.WithLabelValues(provider.ID.String(), provider.Name, "success").Inc()

	// Обновляем throughput провайдера
	monitoring.SMPPProviderThroughput.WithLabelValues(provider.ID.String(), provider.Name).Set(float64(provider.ThroughputPerSec))

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("smpp_message_id", smppMsgID).
		Str("provider", provider.Name).
		Msg("сообщение отправлено успешно")

	return smppMsgID, nil
}

// sendMultipart отправляет многочастное SMS с UDH заголовками
func (s *Sender) sendMultipart(ctx context.Context, conn *Connection, msg *shared.Message, provider *shared.Provider) (string, error) {
	segments := shared.SplitMessage(msg.Text)
	var firstMessageID string

	for i, segment := range segments {
		// Формируем ShortMessage с UDH-заголовком
		var shortMessage []byte
		if segment.UDH != nil {
			shortMessage = append(segment.UDH, []byte(segment.Text)...)
		} else {
			shortMessage = []byte(segment.Text)
		}

		// ESMClass = 0x40 указывает на наличие UDH в ShortMessage
		esmClass := byte(msg.ESMClass) | 0x40

		submitSM := &smppprotocol.SubmitSMPDU{
			ServiceType:          msg.ServiceType,
			SourceAddrTON:        byte(msg.SourceAddrTON),
			SourceAddrNPI:        byte(msg.SourceAddrNPI),
			SourceAddr:           msg.Source,
			DestAddrTON:          byte(msg.DestAddrTON),
			DestAddrNPI:          byte(msg.DestAddrNPI),
			DestinationAddr:      msg.Destination,
			ESMClass:             esmClass,
			ProtocolID:           byte(msg.ProtocolID),
			PriorityFlag:         byte(msg.PriorityFlag),
			ScheduleDeliveryTime: "",
			ValidityPeriod:       "",
			RegisteredDelivery:   byte(msg.RegisteredDelivery),
			ReplaceIfPresent:     byte(msg.ReplaceIfPresent),
			DataCoding:           byte(msg.DataCoding),
			SMDefaultMsgID:       0,
			SMLength:             byte(len(shortMessage)),
			ShortMessage:         shortMessage,
			TLV:                  make(map[uint16][]byte),
		}

		smppMsgID, err := s.sendSinglePDU(conn, submitSM, provider)
		if err != nil {
			monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "multipart_segment_error").Inc()
			return firstMessageID, fmt.Errorf("ошибка отправки сегмента %d/%d: %w", i+1, len(segments), err)
		}

		// Сохраняем ID первого сегмента как основной ID сообщения
		if i == 0 {
			firstMessageID = smppMsgID
		}

		monitoring.SMPPMessagesSent.WithLabelValues(provider.ID.String(), provider.Name, "success").Inc()

		s.logger.Debug().
			Str("message_id", msg.ID.String()).
			Str("smpp_message_id", smppMsgID).
			Int("segment", i+1).
			Int("total_segments", len(segments)).
			Str("provider", provider.Name).
			Msg("сегмент multipart SMS отправлен")
	}

	// Обновляем throughput провайдера
	monitoring.SMPPProviderThroughput.WithLabelValues(provider.ID.String(), provider.Name).Set(float64(provider.ThroughputPerSec))

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("smpp_message_id", firstMessageID).
		Int("segments", len(segments)).
		Str("provider", provider.Name).
		Msg("multipart SMS отправлено успешно")

	return firstMessageID, nil
}

// ---------------------------------------------------------------------------
// Async SMPP sending with sliding window (R-005 / T010)
// ---------------------------------------------------------------------------

// defaultAsyncTimeout — таймаут ожидания ответа на async PDU.
const defaultAsyncTimeout = 30 * time.Second

// SendMessageAsync отправляет SubmitSM PDU через асинхронное соединение
// со sliding window. Метод НЕ блокирует TCP-соединение напрямую:
//   - PDU кодируется и отправляется в WriterCh (writer-горутина пишет в TCP)
//   - Ответ приходит через per-sequence канал из PendingResponses
//     (reader-горутина читает TCP и маршрутизирует ответы)
//
// Возвращает SMPP message_id из submit_sm_resp.
func (s *Sender) SendMessageAsync(
	ctx context.Context,
	msg *shared.Message,
	provider *shared.Provider,
	conn *AsyncConnection,
) (string, error) {
	startTime := time.Now()
	defer func() {
		monitoring.SMPPProcessingDuration.WithLabelValues("async_send_to_provider").Observe(time.Since(startTime).Seconds())
	}()

	// Строим SubmitSM PDU (идентично sendSinglePDU)
	submitSM := &smppprotocol.SubmitSMPDU{
		ServiceType:          msg.ServiceType,
		SourceAddrTON:        byte(msg.SourceAddrTON),
		SourceAddrNPI:        byte(msg.SourceAddrNPI),
		SourceAddr:           msg.Source,
		DestAddrTON:          byte(msg.DestAddrTON),
		DestAddrNPI:          byte(msg.DestAddrNPI),
		DestinationAddr:      msg.Destination,
		ESMClass:             byte(msg.ESMClass),
		ProtocolID:           byte(msg.ProtocolID),
		PriorityFlag:         byte(msg.PriorityFlag),
		ScheduleDeliveryTime: "",
		ValidityPeriod:       "",
		RegisteredDelivery:   byte(msg.RegisteredDelivery),
		ReplaceIfPresent:     byte(msg.ReplaceIfPresent),
		DataCoding:           byte(msg.DataCoding),
		SMDefaultMsgID:       0,
		SMLength:             byte(len(msg.Text)),
		ShortMessage:         []byte(msg.Text),
		TLV:                  make(map[uint16][]byte),
	}

	// Кодируем тело SubmitSM
	encoder := smppprotocol.NewEncoder()
	body, err := encoder.EncodeSubmitSM(submitSM)
	if err != nil {
		return "", fmt.Errorf("async: ошибка кодирования submit_sm: %w", err)
	}

	// 1. Получаем слот в sliding window (blocking с таймаутом из ctx)
	select {
	case conn.WindowSem <- struct{}{}:
		// Слот получен
	case <-ctx.Done():
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "window_timeout").Inc()
		return "", fmt.Errorf("async: таймаут ожидания слота в window: %w", ctx.Err())
	}

	// Гарантируем освобождение слота при любом исходе
	windowReleased := false
	releaseWindow := func() {
		if !windowReleased {
			<-conn.WindowSem
			windowReleased = true
		}
	}
	defer releaseWindow()

	// 2. Получаем sequence number
	seqNum := conn.NextSequence()

	// 3. Создаем полный PDU с sequence number
	pdu := &smppprotocol.PDU{
		CommandLength:  uint32(smppprotocol.PDUHeaderLength + len(body)),
		CommandID:      smppprotocol.SubmitSM,
		CommandStatus:  0,
		SequenceNumber: seqNum,
		Body:           body,
	}

	pduBytes, err := encoder.EncodePDU(pdu)
	if err != nil {
		return "", fmt.Errorf("async: ошибка кодирования PDU: %w", err)
	}

	// 4. Создаём канал для ответа и регистрируем в PendingResponses
	respCh := make(chan *SubmitSMResponse, 1)
	conn.PendingResponses.Store(seqNum, respCh)

	// Гарантируем очистку при любом исходе
	defer conn.PendingResponses.Delete(seqNum)

	// 5. Отправляем закодированные байты в writer-горутину
	select {
	case conn.WriterCh <- pduBytes:
		// PDU отправлен в writer channel
	case <-ctx.Done():
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "writer_ch_timeout").Inc()
		return "", fmt.Errorf("async: таймаут отправки PDU в writer channel: %w", ctx.Err())
	}

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Uint32("sequence_num", seqNum).
		Str("provider", provider.Name).
		Msg("async: PDU отправлен в writer channel, ожидание ответа")

	// 6. Ожидаем ответ с таймаутом
	var responseTimeout <-chan time.Time
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		responseTimeout = time.After(time.Until(deadline))
	} else {
		responseTimeout = time.After(defaultAsyncTimeout)
	}

	select {
	case resp := <-respCh:
		// Освобождаем window слот сразу после получения ответа
		releaseWindow()

		if resp == nil {
			monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "nil_response").Inc()
			return "", fmt.Errorf("async: получен nil ответ для sequence %d", seqNum)
		}

		monitoring.SMPPMessagesSent.WithLabelValues(provider.ID.String(), provider.Name, "success").Inc()
		monitoring.SMPPProviderThroughput.WithLabelValues(provider.ID.String(), provider.Name).Set(float64(provider.ThroughputPerSec))

		s.logger.Debug().
			Str("message_id", msg.ID.String()).
			Str("smpp_message_id", resp.MessageID).
			Uint32("sequence_num", seqNum).
			Str("provider", provider.Name).
			Msg("async: ответ получен успешно")

		return resp.MessageID, nil

	case <-responseTimeout:
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "response_timeout").Inc()
		return "", fmt.Errorf("async: таймаут ожидания ответа для sequence %d", seqNum)

	case <-ctx.Done():
		monitoring.SMPPMessagesFailed.WithLabelValues(provider.ID.String(), provider.Name, "ctx_cancelled").Inc()
		return "", fmt.Errorf("async: контекст отменён при ожидании ответа: %w", ctx.Err())
	}
}

