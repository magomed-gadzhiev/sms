package testutil

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// MockProducer представляет мок для Kafka Producer
type MockProducer struct {
	PublishOutgoingFunc func(ctx context.Context, msg *queue.KafkaMessage) error
	PublishDLRFunc      func(ctx context.Context, dlr *queue.DLRMessage) error
	PublishFailedFunc   func(ctx context.Context, failed *queue.FailedMessage) error
	CloseFunc           func() error
	HealthFunc          func() error
}

func (m *MockProducer) PublishOutgoing(ctx context.Context, msg *queue.KafkaMessage) error {
	if m.PublishOutgoingFunc != nil {
		return m.PublishOutgoingFunc(ctx, msg)
	}
	return nil
}

func (m *MockProducer) PublishDLR(ctx context.Context, dlr *queue.DLRMessage) error {
	if m.PublishDLRFunc != nil {
		return m.PublishDLRFunc(ctx, dlr)
	}
	return nil
}

func (m *MockProducer) PublishFailed(ctx context.Context, failed *queue.FailedMessage) error {
	if m.PublishFailedFunc != nil {
		return m.PublishFailedFunc(ctx, failed)
	}
	return nil
}

func (m *MockProducer) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockProducer) Health() error {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return nil
}

// MockAsyncProducer представляет мок для BatchMessagePublisher (AsyncProducer)
type MockAsyncProducer struct {
	PublishAsyncFunc func(topic string, key string, value []byte, headers []sarama.RecordHeader)
	Calls            []MockAsyncPublishCall
}

// MockAsyncPublishCall записывает параметры вызова PublishAsync
type MockAsyncPublishCall struct {
	Topic   string
	Key     string
	Value   []byte
	Headers []sarama.RecordHeader
}

func (m *MockAsyncProducer) PublishAsync(topic string, key string, value []byte, headers []sarama.RecordHeader) {
	m.Calls = append(m.Calls, MockAsyncPublishCall{
		Topic:   topic,
		Key:     key,
		Value:   value,
		Headers: headers,
	})
	if m.PublishAsyncFunc != nil {
		m.PublishAsyncFunc(topic, key, value, headers)
	}
}

// MockMessageRepository представляет мок для MessageRepository
type MockMessageRepository struct {
	CreateFunc                func(ctx context.Context, msg *shared.Message) error
	GetByIDFunc               func(ctx context.Context, id uuid.UUID) (*shared.Message, error)
	GetByMessageIDFunc        func(ctx context.Context, messageID string) (*shared.Message, error)
	GetByExternalIDFunc       func(ctx context.Context, externalID string) (*shared.Message, error)
	GetBySMPPMessageIDFunc    func(ctx context.Context, smppMessageID string) (*shared.Message, error)
	UpdateFunc                func(ctx context.Context, msg *shared.Message) error
	UpdateStatusFunc          func(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error
	GetPendingForRetryFunc    func(ctx context.Context, limit int) ([]*shared.Message, error)
	GetByClientIDFunc         func(ctx context.Context, clientID uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error)
	GetAllFunc                func(ctx context.Context, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error)
	GetByDestinationFunc      func(ctx context.Context, destination string, limit, offset int) ([]*shared.Message, error)
	IncrementRetryCountFunc   func(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error
}

func (m *MockMessageRepository) Create(ctx context.Context, msg *shared.Message) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, msg)
	}
	return nil
}

func (m *MockMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Message, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *MockMessageRepository) GetByMessageID(ctx context.Context, messageID string) (*shared.Message, error) {
	if m.GetByMessageIDFunc != nil {
		return m.GetByMessageIDFunc(ctx, messageID)
	}
	return nil, storage.ErrNotFound
}

func (m *MockMessageRepository) GetByExternalID(ctx context.Context, externalID string) (*shared.Message, error) {
	if m.GetByExternalIDFunc != nil {
		return m.GetByExternalIDFunc(ctx, externalID)
	}
	return nil, storage.ErrNotFound
}

func (m *MockMessageRepository) GetBySMPPMessageID(ctx context.Context, smppMessageID string) (*shared.Message, error) {
	if m.GetBySMPPMessageIDFunc != nil {
		return m.GetBySMPPMessageIDFunc(ctx, smppMessageID)
	}
	return nil, storage.ErrNotFound
}

func (m *MockMessageRepository) Update(ctx context.Context, msg *shared.Message) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, msg)
	}
	return nil
}

func (m *MockMessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status shared.MessageStatus, statusMessage string) error {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, id, status, statusMessage)
	}
	return nil
}

func (m *MockMessageRepository) GetPendingForRetry(ctx context.Context, limit int) ([]*shared.Message, error) {
	if m.GetPendingForRetryFunc != nil {
		return m.GetPendingForRetryFunc(ctx, limit)
	}
	return []*shared.Message{}, nil
}

func (m *MockMessageRepository) GetByClientID(ctx context.Context, clientID uuid.UUID, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
	if m.GetByClientIDFunc != nil {
		return m.GetByClientIDFunc(ctx, clientID, limit, offset, status)
	}
	return []*shared.Message{}, nil
}

func (m *MockMessageRepository) GetAll(ctx context.Context, limit, offset int, status *shared.MessageStatus) ([]*shared.Message, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, limit, offset, status)
	}
	return []*shared.Message{}, nil
}

func (m *MockMessageRepository) GetByDestination(ctx context.Context, destination string, limit, offset int) ([]*shared.Message, error) {
	if m.GetByDestinationFunc != nil {
		return m.GetByDestinationFunc(ctx, destination, limit, offset)
	}
	return []*shared.Message{}, nil
}

func (m *MockMessageRepository) IncrementRetryCount(ctx context.Context, id uuid.UUID, nextRetryAt time.Time) error {
	if m.IncrementRetryCountFunc != nil {
		return m.IncrementRetryCountFunc(ctx, id, nextRetryAt)
	}
	return nil
}

// MockClientRepository представляет мок для ClientRepository
type MockClientRepository struct {
	CreateFunc      func(ctx context.Context, client *shared.Client) error
	GetByIDFunc     func(ctx context.Context, id uuid.UUID) (*shared.Client, error)
	GetByAPIKeyFunc func(ctx context.Context, apiKey string) (*shared.Client, error)
	GetAllActiveFunc func(ctx context.Context) ([]*shared.Client, error)
	GetAllFunc      func(ctx context.Context) ([]*shared.Client, error)
	UpdateFunc      func(ctx context.Context, client *shared.Client) error
	DeleteFunc      func(ctx context.Context, id uuid.UUID) error
}

func (m *MockClientRepository) Create(ctx context.Context, client *shared.Client) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, client)
	}
	return nil
}

func (m *MockClientRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Client, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *MockClientRepository) GetByAPIKey(ctx context.Context, apiKey string) (*shared.Client, error) {
	if m.GetByAPIKeyFunc != nil {
		return m.GetByAPIKeyFunc(ctx, apiKey)
	}
	return nil, storage.ErrNotFound
}

func (m *MockClientRepository) GetAllActive(ctx context.Context) ([]*shared.Client, error) {
	if m.GetAllActiveFunc != nil {
		return m.GetAllActiveFunc(ctx)
	}
	return []*shared.Client{}, nil
}

func (m *MockClientRepository) GetAll(ctx context.Context) ([]*shared.Client, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx)
	}
	return []*shared.Client{}, nil
}

func (m *MockClientRepository) Update(ctx context.Context, client *shared.Client) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, client)
	}
	return nil
}

func (m *MockClientRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

// MockProviderRepository представляет мок для ProviderRepository
type MockProviderRepository struct {
	CreateFunc    func(ctx context.Context, provider *shared.Provider) error
	GetByIDFunc   func(ctx context.Context, id uuid.UUID) (*shared.Provider, error)
	GetByNameFunc func(ctx context.Context, name string) (*shared.Provider, error)
	GetAllActiveFunc func(ctx context.Context) ([]*shared.Provider, error)
	GetAllFunc    func(ctx context.Context) ([]*shared.Provider, error)
	UpdateFunc    func(ctx context.Context, provider *shared.Provider) error
	DeleteFunc    func(ctx context.Context, id uuid.UUID) error
}

func (m *MockProviderRepository) Create(ctx context.Context, provider *shared.Provider) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, provider)
	}
	return nil
}

func (m *MockProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Provider, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *MockProviderRepository) GetByName(ctx context.Context, name string) (*shared.Provider, error) {
	if m.GetByNameFunc != nil {
		return m.GetByNameFunc(ctx, name)
	}
	return nil, storage.ErrNotFound
}

func (m *MockProviderRepository) GetAllActive(ctx context.Context) ([]*shared.Provider, error) {
	if m.GetAllActiveFunc != nil {
		return m.GetAllActiveFunc(ctx)
	}
	return []*shared.Provider{}, nil
}

func (m *MockProviderRepository) GetAll(ctx context.Context) ([]*shared.Provider, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx)
	}
	return []*shared.Provider{}, nil
}

func (m *MockProviderRepository) Update(ctx context.Context, provider *shared.Provider) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, provider)
	}
	return nil
}

func (m *MockProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

// MockRouteRepository представляет мок для RouteRepository
type MockRouteRepository struct {
	GetByIDFunc              func(ctx context.Context, id uuid.UUID) (*shared.Route, error)
	GetActiveByDestinationFunc func(ctx context.Context, destination string) ([]*shared.Route, error)
	GetAllActiveFunc         func(ctx context.Context) ([]*shared.Route, error)
}

func (m *MockRouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Route, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *MockRouteRepository) GetActiveByDestination(ctx context.Context, destination string) ([]*shared.Route, error) {
	if m.GetActiveByDestinationFunc != nil {
		return m.GetActiveByDestinationFunc(ctx, destination)
	}
	return []*shared.Route{}, nil
}

func (m *MockRouteRepository) GetAllActive(ctx context.Context) ([]*shared.Route, error) {
	if m.GetAllActiveFunc != nil {
		return m.GetAllActiveFunc(ctx)
	}
	return []*shared.Route{}, nil
}
