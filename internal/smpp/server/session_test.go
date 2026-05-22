package server

import (
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSession создаёт сессию без реального соединения для тестов.
func newTestSession(t *testing.T) *Session {
	t.Helper()
	logger := zerolog.Nop()
	s := NewSession(nil, logger)
	require.NotNil(t, s)
	return s
}

// newTestSessionWithConn создаёт сессию с реальным соединением через net.Pipe.
func newTestSessionWithConn(t *testing.T) (*Session, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		server.Close()
		client.Close()
	})
	logger := zerolog.Nop()
	s := NewSession(server, logger)
	require.NotNil(t, s)
	return s, client
}

// ─── NewSession ────────────────────────────────────────────────────────────────

func TestNewSession_InitialState(t *testing.T) {
	s := newTestSession(t)

	assert.NotEmpty(t, s.ID)
	assert.Equal(t, SessionStateOpen, s.State)
	assert.Equal(t, uint32(1), s.SequenceNumber)
	assert.WithinDuration(t, time.Now(), s.LastActivity, 2*time.Second)
	assert.NotNil(t, s.RateLimiter)
}

func TestNewSession_UniqueIDs(t *testing.T) {
	logger := zerolog.Nop()
	s1 := NewSession(nil, logger)
	s2 := NewSession(nil, logger)
	assert.NotEqual(t, s1.ID, s2.ID)
}

// ─── SessionState.String ───────────────────────────────────────────────────────

func TestSessionState_String(t *testing.T) {
	cases := []struct {
		state SessionState
		want  string
	}{
		{SessionStateOpen, "OPEN"},
		{SessionStateBoundRX, "BOUND_RX"},
		{SessionStateBoundTX, "BOUND_TX"},
		{SessionStateBoundTRX, "BOUND_TRX"},
		{SessionStateClosed, "CLOSED"},
		{SessionState(99), "UNKNOWN"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.state.String())
	}
}

// ─── Bind ──────────────────────────────────────────────────────────────────────

func TestSession_Bind_Receiver(t *testing.T) {
	s := newTestSession(t)
	id := uuid.New()

	err := s.Bind("receiver", "sys1", &id, 0)
	require.NoError(t, err)
	assert.Equal(t, SessionStateBoundRX, s.State)
	assert.Equal(t, "sys1", s.SystemID)
	assert.Equal(t, &id, s.ClientID)
	assert.Equal(t, "receiver", s.BindType)
}

func TestSession_Bind_Transmitter(t *testing.T) {
	s := newTestSession(t)
	err := s.Bind("transmitter", "sys2", nil, 0)
	require.NoError(t, err)
	assert.Equal(t, SessionStateBoundTX, s.State)
}

func TestSession_Bind_Transceiver(t *testing.T) {
	s := newTestSession(t)
	err := s.Bind("transceiver", "sys3", nil, 0)
	require.NoError(t, err)
	assert.Equal(t, SessionStateBoundTRX, s.State)
}

func TestSession_Bind_UnknownType(t *testing.T) {
	s := newTestSession(t)
	err := s.Bind("unknown", "sys", nil, 0)
	assert.Error(t, err)
	// Состояние не должно измениться
	assert.Equal(t, SessionStateOpen, s.State)
}

func TestSession_Bind_AlreadyBound(t *testing.T) {
	s := newTestSession(t)
	require.NoError(t, s.Bind("transmitter", "sys", nil, 0))

	// Повторный bind должен вернуть ошибку
	err := s.Bind("receiver", "sys", nil, 0)
	assert.Error(t, err)
	// Состояние не должно измениться обратно в RX
	assert.Equal(t, SessionStateBoundTX, s.State)
}

func TestSession_Bind_SetsCustomRateLimit(t *testing.T) {
	s := newTestSession(t)
	err := s.Bind("transceiver", "sys", nil, 42)
	require.NoError(t, err)
	assert.Equal(t, 42, s.RateLimiter.maxTokens)
}

func TestSession_Bind_ZeroRateLimitKeepsDefault(t *testing.T) {
	s := newTestSession(t)
	// RateLimiter по умолчанию — 100
	defaultMax := s.RateLimiter.maxTokens

	err := s.Bind("transceiver", "sys", nil, 0)
	require.NoError(t, err)
	assert.Equal(t, defaultMax, s.RateLimiter.maxTokens)
}

func TestSession_Bind_UpdatesLastActivity(t *testing.T) {
	s := newTestSession(t)
	before := s.LastActivity
	time.Sleep(5 * time.Millisecond)

	require.NoError(t, s.Bind("transmitter", "sys", nil, 0))
	assert.True(t, s.LastActivity.After(before))
}

// ─── Unbind ────────────────────────────────────────────────────────────────────

func TestSession_Unbind_AfterBind(t *testing.T) {
	s := newTestSession(t)
	require.NoError(t, s.Bind("transceiver", "sys", nil, 0))

	err := s.Unbind()
	require.NoError(t, err)
	assert.Equal(t, SessionStateClosed, s.State)
}

func TestSession_Unbind_WhenOpen(t *testing.T) {
	s := newTestSession(t)
	err := s.Unbind()
	assert.Error(t, err) // не привязана
}

func TestSession_Unbind_WhenClosed(t *testing.T) {
	s := newTestSession(t)
	require.NoError(t, s.Bind("transmitter", "sys", nil, 0))
	require.NoError(t, s.Unbind())

	// Повторный unbind закрытой сессии
	err := s.Unbind()
	assert.Error(t, err)
}

// ─── IsBound / CanSend / CanReceive ───────────────────────────────────────────

func TestSession_IsBound(t *testing.T) {
	s := newTestSession(t)
	assert.False(t, s.IsBound(), "открытая сессия не привязана")

	require.NoError(t, s.Bind("transceiver", "sys", nil, 0))
	assert.True(t, s.IsBound())
}

func TestSession_IsBound_AllBindTypes(t *testing.T) {
	for _, bt := range []string{"receiver", "transmitter", "transceiver"} {
		t.Run(bt, func(t *testing.T) {
			s := newTestSession(t)
			require.NoError(t, s.Bind(bt, "sys", nil, 0))
			assert.True(t, s.IsBound())
		})
	}
}

func TestSession_IsBound_AfterClose(t *testing.T) {
	s := newTestSession(t)
	require.NoError(t, s.Bind("transceiver", "sys", nil, 0))
	require.NoError(t, s.Unbind())
	assert.False(t, s.IsBound())
}

func TestSession_CanSend(t *testing.T) {
	cases := []struct {
		bindType string
		canSend  bool
	}{
		{"receiver", false},
		{"transmitter", true},
		{"transceiver", true},
	}
	for _, tc := range cases {
		t.Run(tc.bindType, func(t *testing.T) {
			s := newTestSession(t)
			require.NoError(t, s.Bind(tc.bindType, "sys", nil, 0))
			assert.Equal(t, tc.canSend, s.CanSend())
		})
	}
}

func TestSession_CanReceive(t *testing.T) {
	cases := []struct {
		bindType   string
		canReceive bool
	}{
		{"receiver", true},
		{"transmitter", false},
		{"transceiver", true},
	}
	for _, tc := range cases {
		t.Run(tc.bindType, func(t *testing.T) {
			s := newTestSession(t)
			require.NoError(t, s.Bind(tc.bindType, "sys", nil, 0))
			assert.Equal(t, tc.canReceive, s.CanReceive())
		})
	}
}

// ─── NextSequenceNumber ────────────────────────────────────────────────────────

func TestSession_NextSequenceNumber_Increments(t *testing.T) {
	s := newTestSession(t)

	prev := s.NextSequenceNumber()
	for i := 0; i < 9; i++ {
		cur := s.NextSequenceNumber()
		assert.Equal(t, prev+1, cur)
		prev = cur
	}
}

func TestSession_NextSequenceNumber_StartsAt1(t *testing.T) {
	s := newTestSession(t)
	first := s.NextSequenceNumber()
	assert.Equal(t, uint32(1), first)
}

func TestSession_NextSequenceNumber_WrapsAround(t *testing.T) {
	s := newTestSession(t)
	// Искусственно устанавливаем счётчик на максимум uint32
	s.mu.Lock()
	s.SequenceNumber = ^uint32(0) // 0xFFFFFFFF
	s.mu.Unlock()

	val := s.NextSequenceNumber()
	assert.Equal(t, uint32(0xFFFFFFFF), val)

	// После переполнения — должно стать 1, а не 0
	next := s.NextSequenceNumber()
	assert.Equal(t, uint32(1), next, "после wrap-around sequence number должен быть 1")
}

func TestSession_NextSequenceNumber_Concurrent(t *testing.T) {
	s := newTestSession(t)
	const goroutines = 100

	results := make(chan uint32, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			results <- s.NextSequenceNumber()
		}()
	}

	seen := make(map[uint32]bool, goroutines)
	for i := 0; i < goroutines; i++ {
		n := <-results
		assert.False(t, seen[n], "sequence number %d встретился дважды", n)
		seen[n] = true
	}
}

// ─── UpdateActivity ────────────────────────────────────────────────────────────

func TestSession_UpdateActivity(t *testing.T) {
	s := newTestSession(t)
	before := s.LastActivity
	time.Sleep(5 * time.Millisecond)

	s.UpdateActivity()

	s.mu.RLock()
	after := s.LastActivity
	s.mu.RUnlock()

	assert.True(t, after.After(before), "LastActivity должна обновиться")
	assert.WithinDuration(t, time.Now(), after, 2*time.Second)
}

// ─── CheckRateLimit ────────────────────────────────────────────────────────────

func TestSession_CheckRateLimit_Allows(t *testing.T) {
	s := newTestSession(t)
	// По умолчанию 100 токенов — первый запрос должен пройти
	err := s.CheckRateLimit()
	assert.NoError(t, err)
}

func TestSession_CheckRateLimit_Blocks(t *testing.T) {
	s := newTestSession(t)
	// Устанавливаем лимит = 1 и исчерпываем его
	s.RateLimiter = NewRateLimiter(1)
	require.NoError(t, s.CheckRateLimit())

	err := s.CheckRateLimit()
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

// ─── Close ─────────────────────────────────────────────────────────────────────

func TestSession_Close_SetsClosedState(t *testing.T) {
	s, _ := newTestSessionWithConn(t)
	err := s.Close()
	require.NoError(t, err)
	assert.Equal(t, SessionStateClosed, s.State)
}

func TestSession_Close_Idempotent(t *testing.T) {
	s, _ := newTestSessionWithConn(t)
	require.NoError(t, s.Close())
	// Второй вызов Close на уже закрытой сессии не должен паниковать/ошибаться
	err := s.Close()
	assert.NoError(t, err)
}

func TestSession_Close_NilConn(t *testing.T) {
	s := newTestSession(t) // conn == nil
	err := s.Close()
	assert.NoError(t, err)
	assert.Equal(t, SessionStateClosed, s.State)
}

// ─── RemoteAddr ────────────────────────────────────────────────────────────────

func TestSession_RemoteAddr_NilConn(t *testing.T) {
	s := newTestSession(t)
	assert.Nil(t, s.RemoteAddr())
}

func TestSession_RemoteAddr_WithConn(t *testing.T) {
	s, _ := newTestSessionWithConn(t)
	addr := s.RemoteAddr()
	assert.NotNil(t, addr)
}
