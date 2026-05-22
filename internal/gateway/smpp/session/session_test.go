package session

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn implements net.Conn for testing.
type mockConn struct {
	readData  []byte
	readPos   int
	written   []byte
	closed    bool
	readErr   error
	writeErr  error
	closeErr  error
	localAddr net.Addr
	remoteAddr net.Addr
	mu        sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2775},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 54321},
	}
}

func (c *mockConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readErr != nil {
		return 0, c.readErr
	}
	if c.readPos >= len(c.readData) {
		return 0, nil
	}
	n := copy(b, c.readData[c.readPos:])
	c.readPos += n
	return n, nil
}

func (c *mockConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	c.written = append(c.written, b...)
	return len(b), nil
}

func (c *mockConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.closeErr
}

func (c *mockConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *mockConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}

// --- Session creation ---

func TestNewSession(t *testing.T) {
	conn := newMockConn()
	logger := newTestLogger()

	s := NewSession(conn, logger)

	require.NotNil(t, s)
	assert.NotEmpty(t, s.ID, "session ID must be generated")
	assert.Equal(t, SessionStateOpen, s.State)
	assert.Equal(t, uint32(1), s.SequenceNumber)
	assert.Equal(t, conn, s.Conn)
	assert.NotNil(t, s.RateLimiter)
	assert.NotNil(t, s.GetContext())
	assert.False(t, s.IsBound())
	assert.False(t, s.CanSend())
	assert.False(t, s.CanReceive())
	assert.WithinDuration(t, time.Now(), s.LastActivity, 2*time.Second)
}

func TestNewSessionUniqueIDs(t *testing.T) {
	conn := newMockConn()
	logger := newTestLogger()

	ids := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		s := NewSession(conn, logger)
		_, exists := ids[s.ID]
		assert.False(t, exists, "session ID must be unique, got duplicate: %s", s.ID)
		ids[s.ID] = struct{}{}
	}
}

// --- SessionState.String ---

func TestSessionStateString(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{SessionStateOpen, "OPEN"},
		{SessionStateBoundRX, "BOUND_RX"},
		{SessionStateBoundTX, "BOUND_TX"},
		{SessionStateBoundTRX, "BOUND_TRX"},
		{SessionStateClosed, "CLOSED"},
		{SessionState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// --- Bind ---

func TestBindReceiver(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	clientID := uuid.New()

	err := s.Bind("receiver", "test_sys", &clientID, "user123", 50)
	require.NoError(t, err)

	assert.Equal(t, SessionStateBoundRX, s.State)
	assert.Equal(t, "receiver", s.BindType)
	assert.Equal(t, "test_sys", s.SystemID)
	assert.Equal(t, &clientID, s.ClientID)
	assert.Equal(t, "user123", s.UserID)
	assert.True(t, s.IsBound())
	assert.False(t, s.CanSend())
	assert.True(t, s.CanReceive())
}

func TestBindTransmitter(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	clientID := uuid.New()

	err := s.Bind("transmitter", "test_sys", &clientID, "user123", 50)
	require.NoError(t, err)

	assert.Equal(t, SessionStateBoundTX, s.State)
	assert.True(t, s.IsBound())
	assert.True(t, s.CanSend())
	assert.False(t, s.CanReceive())
}

func TestBindTransceiver(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	clientID := uuid.New()

	err := s.Bind("transceiver", "test_sys", &clientID, "user123", 50)
	require.NoError(t, err)

	assert.Equal(t, SessionStateBoundTRX, s.State)
	assert.True(t, s.IsBound())
	assert.True(t, s.CanSend())
	assert.True(t, s.CanReceive())
}

func TestBindNilClientID(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	err := s.Bind("transceiver", "test_sys", nil, "user123", 200)
	require.NoError(t, err)

	assert.Nil(t, s.ClientID)
	assert.Equal(t, SessionStateBoundTRX, s.State)
}

func TestBindInvalidType(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	err := s.Bind("unknown_type", "sys", nil, "u", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "неизвестный тип bind")
	assert.Equal(t, SessionStateOpen, s.State)
}

func TestBindAlreadyBound(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	err := s.Bind("receiver", "sys1", nil, "u1", 100)
	require.NoError(t, err)

	err = s.Bind("transmitter", "sys2", nil, "u2", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "уже привязана")
}

func TestBindAfterClose(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	s.Close()

	err := s.Bind("receiver", "sys1", nil, "u1", 100)
	require.Error(t, err)
}

func TestBindSetsRateLimit(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	err := s.Bind("transmitter", "sys1", nil, "u1", 500)
	require.NoError(t, err)

	assert.NotNil(t, s.RateLimiter)
	assert.Equal(t, 500, s.RateLimiter.maxTokens)
}

func TestBindZeroRateLimitKeepsDefault(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	originalRL := s.RateLimiter

	err := s.Bind("transmitter", "sys1", nil, "u1", 0)
	require.NoError(t, err)

	// When rateLimit <= 0, the original RateLimiter is kept
	assert.Equal(t, originalRL, s.RateLimiter)
}

// --- Unbind ---

func TestUnbind(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	require.NoError(t, s.Bind("transmitter", "sys1", nil, "u1", 100))

	err := s.Unbind()
	require.NoError(t, err)
	assert.Equal(t, SessionStateClosed, s.State)
	assert.False(t, s.IsBound())
}

func TestUnbindWhenOpen(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	err := s.Unbind()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не привязана")
}

func TestUnbindWhenAlreadyClosed(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	require.NoError(t, s.Bind("transmitter", "sys1", nil, "u1", 100))
	require.NoError(t, s.Unbind())

	err := s.Unbind()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не привязана")
}

// --- NextSequenceNumber ---

func TestNextSequenceNumber(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	assert.Equal(t, uint32(1), s.NextSequenceNumber())
	assert.Equal(t, uint32(2), s.NextSequenceNumber())
	assert.Equal(t, uint32(3), s.NextSequenceNumber())
}

func TestNextSequenceNumberWraparound(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	s.SequenceNumber = 0xFFFFFFFF

	seq := s.NextSequenceNumber()
	assert.Equal(t, uint32(0xFFFFFFFF), seq)

	// After overflow, SequenceNumber wraps to 0, but the code resets 0 to 1
	nextSeq := s.NextSequenceNumber()
	assert.Equal(t, uint32(1), nextSeq)
}

func TestNextSequenceNumberConcurrent(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	var wg sync.WaitGroup
	seen := sync.Map{}
	count := 1000

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seq := s.NextSequenceNumber()
			_, loaded := seen.LoadOrStore(seq, true)
			assert.False(t, loaded, "duplicate sequence number: %d", seq)
		}()
	}
	wg.Wait()
}

// --- UpdateActivity ---

func TestUpdateActivity(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	before := s.GetLastActivity()
	time.Sleep(10 * time.Millisecond)
	s.UpdateActivity()
	after := s.GetLastActivity()

	assert.True(t, after.After(before))
}

// --- Close ---

func TestClose(t *testing.T) {
	conn := newMockConn()
	s := NewSession(conn, newTestLogger())

	err := s.Close()
	require.NoError(t, err)

	assert.Equal(t, SessionStateClosed, s.State)
	assert.True(t, conn.closed)
}

func TestCloseAlreadyClosed(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	require.NoError(t, s.Close())

	// Second close should be no-op
	err := s.Close()
	require.NoError(t, err)
}

func TestCloseCancelsContext(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	ctx := s.GetContext()
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done before close")
	default:
	}

	s.Close()

	select {
	case <-ctx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be done after close")
	}
}

// --- RemoteAddr ---

func TestRemoteAddr(t *testing.T) {
	conn := newMockConn()
	s := NewSession(conn, newTestLogger())

	addr := s.RemoteAddr()
	require.NotNil(t, addr)
	assert.Equal(t, "192.168.1.100:54321", addr.String())
}

func TestRemoteAddrNilConn(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	s.Conn = nil

	addr := s.RemoteAddr()
	assert.Nil(t, addr)
}

// --- GetConn ---

func TestGetConn(t *testing.T) {
	conn := newMockConn()
	s := NewSession(conn, newTestLogger())

	assert.Equal(t, conn, s.GetConn())
}

// --- EnquireLink ---

func TestUpdateEnquireLinkSent(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())

	before := s.GetEnquireLinkSent()
	time.Sleep(10 * time.Millisecond)
	s.UpdateEnquireLinkSent()
	after := s.GetEnquireLinkSent()

	assert.True(t, after.After(before))
}

// --- CheckRateLimit ---

func TestCheckRateLimit(t *testing.T) {
	s := NewSession(newMockConn(), newTestLogger())
	// Default is 100 tokens

	// First 100 should succeed
	for i := 0; i < 100; i++ {
		err := s.CheckRateLimit()
		require.NoError(t, err, "call %d should succeed", i)
	}

	// Next should fail (tokens exhausted within the same instant)
	err := s.CheckRateLimit()
	assert.ErrorIs(t, err, ErrRateLimitExceeded)
}

// --- Concurrent bind/unbind safety ---

func TestConcurrentBindUnbind(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := NewSession(newMockConn(), newTestLogger())

			_ = s.Bind("transmitter", "sys", nil, "u", 100)
			_ = s.Unbind()
			_ = s.Close()
		}()
	}
	wg.Wait()
}

// --- generateSessionID ---

func TestGenerateSessionID(t *testing.T) {
	ids := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		id := generateSessionID()
		assert.NotEmpty(t, id)
		_, exists := ids[id]
		assert.False(t, exists, "duplicate ID: %s", id)
		ids[id] = struct{}{}
	}
}
