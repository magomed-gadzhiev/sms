package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

// SMSCSender интерфейс для smsc.Sender
type SMSCSender interface {
	SendMessage(ctx context.Context, msg *shared.Message, provider *shared.Provider) (string, error)
}

// SenderService предоставляет сервис для отправки сообщений через провайдеров
type SenderService struct {
	sender SMSCSender
	logger interface {
		Info() *log.Event
		Error() *log.Event
		Debug() *log.Event
	}
}

// NewSenderService создает новый сервис отправителя
func NewSenderService(sender SMSCSender) *SenderService {
	return &SenderService{
		sender: sender,
		logger: log.With().Str("component", "sender_service").Logger(),
	}
}

// SendMessage отправляет сообщение через провайдера
func (s *SenderService) SendMessage(
	ctx context.Context,
	provider *domain.Provider,
	params *SendMessageParams,
) (string, error) {
	if !provider.IsActive() {
		return "", domain.ErrProviderInactive
	}

	// Создаем shared.Message из параметров
	sharedMsg := &shared.Message{
		Source:             params.Source,
		Destination:        params.Destination,
		Text:               params.Text,
		ServiceType:        params.ServiceType,
		SourceAddrTON:      params.SourceAddrTON,
		SourceAddrNPI:      params.SourceAddrNPI,
		DestAddrTON:        params.DestAddrTON,
		DestAddrNPI:        params.DestAddrNPI,
		ESMClass:           params.ESMClass,
		ProtocolID:         params.ProtocolID,
		PriorityFlag:       params.PriorityFlag,
		RegisteredDelivery: params.RegisteredDelivery,
		ReplaceIfPresent:   params.ReplaceIfPresent,
		DataCoding:         params.DataCoding,
		ValidityPeriod:     params.ValidityPeriod,
	}

	// Создаем shared.Provider из domain.Provider
	sharedProvider := &shared.Provider{
		ID:               provider.ID,
		Name:             provider.Name,
		Host:             provider.Host,
		Port:             provider.Port,
		SystemID:         provider.SystemID,
		Password:         provider.Password,
		SystemType:       provider.SystemType,
		BindType:         string(provider.BindType),
		BindTON:          provider.BindTON,
		BindNPI:          provider.BindNPI,
		AddrTON:          provider.AddrTON,
		AddrNPI:          provider.AddrNPI,
		AddressRange:     provider.AddressRange,
		MaxConnections:   provider.MaxConnections,
		Active:           provider.Active,
		Priority:         provider.Priority,
		ThroughputPerSec: provider.ThroughputPerSec,
	}

	// Отправляем сообщение через smsc.Sender
	smppMessageID, err := s.sender.SendMessage(ctx, sharedMsg, sharedProvider)
	if err != nil {
		log.Error().
			Err(err).
			Str("provider_id", provider.ID.String()).
			Str("provider_name", provider.Name).
			Msg("ошибка отправки сообщения")
		return "", fmt.Errorf("отправка сообщения: %w", err)
	}

	log.Debug().
		Str("provider_id", provider.ID.String()).
		Str("provider_name", provider.Name).
		Str("smpp_message_id", smppMessageID).
		Msg("сообщение отправлено успешно")

	return smppMessageID, nil
}

// SMPPConnectionAdapter адаптирует существующее SMPP соединение к интерфейсу Connection
type SMPPConnectionAdapter struct {
	conn interface{} // *smsc.Connection
}

// SendMessage отправляет сообщение через адаптированное соединение
func (a *SMPPConnectionAdapter) SendMessage(ctx context.Context, params *SendMessageParams) (string, error) {
	// Используем существующий smsc.Sender для отправки
	// Это будет сделано через вызов существующего кода
	// Для полной реализации нужно будет интегрировать с smsc.Sender
	
	// Временная заглушка - в реальной реализации нужно использовать smsc.Sender
	return "", fmt.Errorf("not implemented: use smsc.Sender")
}

// IsBound проверяет, привязано ли соединение
func (a *SMPPConnectionAdapter) IsBound() bool {
	// Нужно получить доступ к полю Bound из smsc.Connection
	// Это требует рефлексии или изменения структуры
	return true // временно
}

// Close закрывает соединение
func (a *SMPPConnectionAdapter) Close() error {
	// Нужно вызвать Close на smsc.Connection
	return nil
}
