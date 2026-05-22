package smsc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

// mockStubConfigRepo реализует smsc.StubConfigRepository для тестов.
type mockStubConfigRepo struct {
	configs map[uuid.UUID]*smsc.StubProviderConfig
	err     error
}

func (m *mockStubConfigRepo) GetByProviderID(_ context.Context, providerID uuid.UUID) (*smsc.StubProviderConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	cfg, ok := m.configs[providerID]
	if !ok {
		return nil, nil
	}
	return cfg, nil
}

func newTestMessage() *shared.Message {
	clientID := uuid.New()
	return &shared.Message{
		ID:          uuid.New(),
		Source:      "TestSender",
		Destination: "79001234567",
		Text:        "test message",
		Encoding:    shared.MessageEncodingGSM7,
		Status:      shared.MessageStatusPending,
		ClientID:    &clientID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func newTestProvider() *shared.Provider {
	return &shared.Provider{
		ID:         uuid.New(),
		Name:       "test-stub",
		SystemType: "SIMULATOR",
		Active:     true,
	}
}

func TestStubSender_SendMessageAsync_ReturnsStubPrefixedID(t *testing.T) {
	sender := smsc.NewStubSender(nil, nil, "")
	msg := newTestMessage()
	provider := newTestProvider()

	msgID, err := sender.SendMessageAsync(context.Background(), msg, provider, nil)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(msgID, "stub-"), "message ID should have stub- prefix, got: %s", msgID)
}

func TestStubSender_SendMessageAsync_FailureRate100_AlwaysFails(t *testing.T) {
	providerID := uuid.New()
	repo := &mockStubConfigRepo{
		configs: map[uuid.UUID]*smsc.StubProviderConfig{
			providerID: {
				ProviderID:     providerID,
				MinDelayMs:     0,
				MaxDelayMs:     1,
				FailureRatePct: 100,
				DLRDelayMs:     0,
				DLRSuccessRate: 100,
			},
		},
	}
	sender := smsc.NewStubSender(repo, nil, "")
	provider := &shared.Provider{ID: providerID, Name: "fail-stub", SystemType: "SIMULATOR"}

	for i := 0; i < 20; i++ {
		_, err := sender.SendMessageAsync(context.Background(), newTestMessage(), provider, nil)
		require.Error(t, err, "attempt %d should fail with 100%% failure rate", i)
		assert.Contains(t, err.Error(), "simulated failure")
	}
}

func TestStubSender_SendMessageAsync_FailureRate0_AlwaysSucceeds(t *testing.T) {
	providerID := uuid.New()
	repo := &mockStubConfigRepo{
		configs: map[uuid.UUID]*smsc.StubProviderConfig{
			providerID: {
				ProviderID:     providerID,
				MinDelayMs:     0,
				MaxDelayMs:     1,
				FailureRatePct: 0,
				DLRDelayMs:     0,
				DLRSuccessRate: 100,
			},
		},
	}
	sender := smsc.NewStubSender(repo, nil, "")
	provider := &shared.Provider{ID: providerID, Name: "ok-stub", SystemType: "SIMULATOR"}

	for i := 0; i < 20; i++ {
		msgID, err := sender.SendMessageAsync(context.Background(), newTestMessage(), provider, nil)
		require.NoError(t, err, "attempt %d should succeed with 0%% failure rate", i)
		assert.True(t, strings.HasPrefix(msgID, "stub-"))
	}
}

func TestStubSender_DefaultConfig_WhenConfigRepoNil(t *testing.T) {
	// configRepo=nil => используется дефолтный конфиг (FailureRatePct=0).
	// Дефолтный конфиг имеет задержку 100-500ms, поэтому делаем один вызов.
	sender := smsc.NewStubSender(nil, nil, "")
	provider := newTestProvider()

	msgID, err := sender.SendMessageAsync(context.Background(), newTestMessage(), provider, nil)
	require.NoError(t, err, "default config has 0%% failure rate, should succeed")
	assert.True(t, strings.HasPrefix(msgID, "stub-"))
}

func TestStubSender_DefaultConfig_WhenRepoReturnsNil(t *testing.T) {
	// Repo существует, но возвращает nil для провайдера => fallback на дефолтный конфиг.
	// Используем repo с пустой картой — GetByProviderID вернёт nil.
	repo := &mockStubConfigRepo{configs: map[uuid.UUID]*smsc.StubProviderConfig{}}
	sender := smsc.NewStubSender(repo, nil, "")
	provider := newTestProvider()

	// Дефолтный конфиг имеет задержку 100-500ms, один вызов.
	msgID, err := sender.SendMessageAsync(context.Background(), newTestMessage(), provider, nil)
	require.NoError(t, err, "fallback to default config should succeed")
	assert.True(t, strings.HasPrefix(msgID, "stub-"))
}

func TestStubSender_CustomConfig_UsedWhenAvailable(t *testing.T) {
	providerID := uuid.New()
	repo := &mockStubConfigRepo{
		configs: map[uuid.UUID]*smsc.StubProviderConfig{
			providerID: {
				ProviderID:     providerID,
				MinDelayMs:     0,
				MaxDelayMs:     1,
				FailureRatePct: 100, // всегда ошибка
				DLRDelayMs:     0,
				DLRSuccessRate: 100,
			},
		},
	}
	sender := smsc.NewStubSender(repo, nil, "")
	provider := &shared.Provider{ID: providerID, Name: "custom-stub", SystemType: "SIMULATOR"}

	// Если кастомный конфиг с FailureRatePct=100 применяется, все вызовы должны упасть.
	_, err := sender.SendMessageAsync(context.Background(), newTestMessage(), provider, nil)
	require.Error(t, err, "custom config with 100%% failure rate should be used")
	assert.Contains(t, err.Error(), "simulated failure")
}

func TestStubSender_ContextCancellation(t *testing.T) {
	providerID := uuid.New()
	// Большая задержка чтобы гарантировать срабатывание отмены контекста.
	repo := &mockStubConfigRepo{
		configs: map[uuid.UUID]*smsc.StubProviderConfig{
			providerID: {
				ProviderID:     providerID,
				MinDelayMs:     5000,
				MaxDelayMs:     5001,
				FailureRatePct: 0,
				DLRDelayMs:     0,
				DLRSuccessRate: 100,
			},
		},
	}
	sender := smsc.NewStubSender(repo, nil, "")
	provider := &shared.Provider{ID: providerID, Name: "slow-stub", SystemType: "SIMULATOR"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу

	start := time.Now()
	_, err := sender.SendMessageAsync(ctx, newTestMessage(), provider, nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 500*time.Millisecond, "should return immediately on cancelled context")
}
