package smpp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/services/provider/application"
	"github.com/smpp-server/smpp-server/internal/services/provider/domain"
	"github.com/smpp-server/smpp-server/internal/smsc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Interface compliance ---

func TestPoolAdapter_ImplementsConnectionPoolService(t *testing.T) {
	var _ application.ConnectionPoolService = (*PoolAdapter)(nil)
}

func TestConnectionAdapter_ImplementsConnection(t *testing.T) {
	var _ application.Connection = (*ConnectionAdapter)(nil)
}

// --- domainToSharedProvider tests ---

func TestDomainToSharedProvider(t *testing.T) {
	providerID := uuid.New()
	now := time.Now()

	dp := &domain.Provider{
		ID:               providerID,
		Name:             "test-provider",
		Host:             "smsc.example.com",
		Port:             2775,
		SystemID:         "system1",
		Password:         "secret",
		SystemType:       "TRANSCEIVER",
		BindType:         domain.BindTypeTransceiver,
		BindTON:          1,
		BindNPI:          1,
		AddrTON:          1,
		AddrNPI:          1,
		AddressRange:     "^7",
		MaxConnections:   5,
		Active:           true,
		Priority:         10,
		ThroughputPerSec: 100,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	sp := domainToSharedProvider(dp)
	require.NotNil(t, sp)

	assert.Equal(t, providerID, sp.ID)
	assert.Equal(t, "test-provider", sp.Name)
	assert.Equal(t, "smsc.example.com", sp.Host)
	assert.Equal(t, 2775, sp.Port)
	assert.Equal(t, "system1", sp.SystemID)
	assert.Equal(t, "secret", sp.Password)
	assert.Equal(t, "TRANSCEIVER", sp.SystemType)
	assert.Equal(t, "transceiver", sp.BindType)
	assert.Equal(t, 1, sp.BindTON)
	assert.Equal(t, 1, sp.BindNPI)
	assert.Equal(t, 1, sp.AddrTON)
	assert.Equal(t, 1, sp.AddrNPI)
	assert.Equal(t, "^7", sp.AddressRange)
	assert.Equal(t, 5, sp.MaxConnections)
	assert.True(t, sp.Active)
	assert.Equal(t, 10, sp.Priority)
	assert.Equal(t, 100, sp.ThroughputPerSec)
	assert.Equal(t, now, sp.CreatedAt)
	assert.Equal(t, now, sp.UpdatedAt)
}

func TestDomainToSharedProvider_EmptyFields(t *testing.T) {
	dp := &domain.Provider{
		ID: uuid.New(),
	}

	sp := domainToSharedProvider(dp)
	require.NotNil(t, sp)

	assert.Equal(t, dp.ID, sp.ID)
	assert.Empty(t, sp.Name)
	assert.Empty(t, sp.Host)
	assert.Zero(t, sp.Port)
	assert.False(t, sp.Active)
	assert.Zero(t, sp.MaxConnections)
}

// --- ConnectionAdapter tests ---

func TestConnectionAdapter_SendMessage_ReturnsError(t *testing.T) {
	adapter := &ConnectionAdapter{
		conn: &smsc.Connection{},
		pool: nil,
	}

	result, err := adapter.SendMessage(context.Background(), &application.SendMessageParams{
		Source:      "sender",
		Destination: "79001234567",
		Text:        "test",
	})

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "SenderService.SendMessage")
}

func TestConnectionAdapter_IsBound_True(t *testing.T) {
	conn := &smsc.Connection{
		Bound: true,
	}
	adapter := &ConnectionAdapter{
		conn: conn,
	}

	assert.True(t, adapter.IsBound())
}

func TestConnectionAdapter_IsBound_False(t *testing.T) {
	conn := &smsc.Connection{
		Bound: false,
	}
	adapter := &ConnectionAdapter{
		conn: conn,
	}

	assert.False(t, adapter.IsBound())
}

// --- Disconnect tests ---

func TestPoolAdapter_Disconnect(t *testing.T) {
	providerID := uuid.New()

	adapter := &PoolAdapter{
		providerConnCount: map[uuid.UUID]int{
			providerID: 5,
		},
		logger: zerolog.Nop(),
	}

	err := adapter.Disconnect(providerID)
	assert.NoError(t, err)

	// Verify the provider was removed from the connection count map
	adapter.mu.RLock()
	count, exists := adapter.providerConnCount[providerID]
	adapter.mu.RUnlock()

	assert.False(t, exists, "provider should be removed from providerConnCount")
	assert.Zero(t, count)
}

func TestPoolAdapter_Disconnect_UnknownProvider(t *testing.T) {
	adapter := &PoolAdapter{
		providerConnCount: make(map[uuid.UUID]int),
		logger:            zerolog.Nop(),
	}

	err := adapter.Disconnect(uuid.New())
	assert.NoError(t, err, "disconnecting unknown provider should not error")
}

// --- Connect tests ---

func TestPoolAdapter_Connect_InactiveProvider(t *testing.T) {
	adapter := &PoolAdapter{
		providerConnCount: make(map[uuid.UUID]int),
		logger:            zerolog.Nop(),
	}

	provider := &domain.Provider{
		ID:     uuid.New(),
		Active: false,
	}

	err := adapter.Connect(context.Background(), provider)
	assert.Error(t, err)
	assert.Equal(t, domain.ErrProviderInactive, err)
}

func TestPoolAdapter_Connect_AlreadyEnoughConnections(t *testing.T) {
	providerID := uuid.New()

	adapter := &PoolAdapter{
		providerConnCount: map[uuid.UUID]int{
			providerID: 5,
		},
		mu:     sync.RWMutex{},
		logger: zerolog.Nop(),
	}

	provider := &domain.Provider{
		ID:             providerID,
		Active:         true,
		MaxConnections: 3, // less than current 5
	}

	// With no pool, this would panic if it tried to actually connect.
	// Since currentConnCount (5) >= desiredConnCount (3), it returns nil immediately.
	err := adapter.Connect(context.Background(), provider)
	assert.NoError(t, err)
}

// --- HealthCheck tests ---

func TestPoolAdapter_HealthCheck_WithTrackedConnections(t *testing.T) {
	providerID := uuid.New()

	// We need a real smsc.Pool for HealthCheck since it calls pool.HealthCheck().
	// Create a minimal pool with no connections.
	pool := smsc.NewPool(nil)

	adapter := &PoolAdapter{
		pool: pool,
		providerConnCount: map[uuid.UUID]int{
			providerID: 3,
		},
		logger: zerolog.Nop(),
	}

	active, total, err := adapter.HealthCheck(providerID)
	assert.NoError(t, err)
	assert.Equal(t, 0, active, "no real connections, so active should be 0")
	assert.Equal(t, 3, total, "total should come from providerConnCount")
}

func TestPoolAdapter_HealthCheck_NoTrackedConnections(t *testing.T) {
	providerID := uuid.New()
	pool := smsc.NewPool(nil)

	adapter := &PoolAdapter{
		pool:              pool,
		providerConnCount: make(map[uuid.UUID]int),
		logger:            zerolog.Nop(),
	}

	active, total, err := adapter.HealthCheck(providerID)
	assert.NoError(t, err)
	assert.Equal(t, 0, active)
	assert.Equal(t, 0, total)
}

// --- GetSender tests ---

func TestPoolAdapter_GetSender_NotNil(t *testing.T) {
	pool := smsc.NewPool(nil)
	adapter := &PoolAdapter{
		pool:   pool,
		sender: smsc.NewSender(pool),
		logger: zerolog.Nop(),
	}

	sender := adapter.GetSender()
	assert.NotNil(t, sender)
}
