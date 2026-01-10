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

	// Подготавливаем submit_sm PDU
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

	// Кодируем submit_sm PDU
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

	// Увеличиваем счетчик успешных отправок
	monitoring.SMPPMessagesSent.WithLabelValues(provider.ID.String(), provider.Name, "success").Inc()
	
	// Обновляем throughput провайдера
	monitoring.SMPPProviderThroughput.WithLabelValues(provider.ID.String(), provider.Name).Set(float64(provider.ThroughputPerSec))

	s.logger.Debug().
		Str("message_id", msg.ID.String()).
		Str("smpp_message_id", resp.MessageID).
		Str("provider", provider.Name).
		Msg("сообщение отправлено успешно")

	return resp.MessageID, nil
}
