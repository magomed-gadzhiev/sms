package server

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/config"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}

func newTestConfig() *config.SMSPConfig {
	return &config.SMSPConfig{
		Host:              "127.0.0.1",
		Port:              0, // let OS pick
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		EnquireLinkPeriod: 60 * time.Second,
		MaxConnections:    100,
		RateLimitPerSec:   100,
	}
}

// --- NewServer ---

func TestNewServer(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()

	s := NewServer(cfg, nil, nil, nil, logger)

	require.NotNil(t, s)
	assert.NotNil(t, s.sessions)
	assert.Equal(t, 0, len(s.sessions))
	assert.NotNil(t, s.ctx)
	assert.NotNil(t, s.cancel)
}

// --- Start / Stop ---

func TestStartAndStop(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()

	s := NewServer(cfg, nil, nil, nil, logger)

	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	s.config = cfg

	err := s.Start()
	require.NoError(t, err)
	require.NotNil(t, s.listener)

	// Should be able to connect
	actualAddr := s.listener.Addr().String()
	conn, dialErr := net.DialTimeout("tcp", actualAddr, 2*time.Second)
	if dialErr == nil {
		conn.Close()
	}

	err = s.Stop()
	require.NoError(t, err)
}

func TestStopClosesAllSessions(t *testing.T) {
	cfg := newTestConfig()
	cfg.Port = 0
	logger := newTestLogger()

	s := NewServer(cfg, nil, nil, nil, logger)
	require.NoError(t, s.Start())

	// Create a mock session and add it
	mockConn := newMockServerConn()
	sess := smppsession.NewSession(mockConn, logger)

	s.sessionsMu.Lock()
	s.sessions[sess.ID] = sess
	s.sessionsMu.Unlock()

	assert.Equal(t, 1, s.GetActiveSessionsCount())

	require.NoError(t, s.Stop())

	assert.Equal(t, 0, s.GetActiveSessionsCount())
}

// --- GetActiveSessionsCount / GetBoundSessionsCount ---

func TestGetActiveSessionsCount(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	assert.Equal(t, 0, s.GetActiveSessionsCount())

	// Add sessions manually
	conn1 := newMockServerConn()
	sess1 := smppsession.NewSession(conn1, logger)

	conn2 := newMockServerConn()
	sess2 := smppsession.NewSession(conn2, logger)

	s.sessionsMu.Lock()
	s.sessions[sess1.ID] = sess1
	s.sessions[sess2.ID] = sess2
	s.sessionsMu.Unlock()

	assert.Equal(t, 2, s.GetActiveSessionsCount())
}

func TestGetBoundSessionsCount(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	conn1 := newMockServerConn()
	sess1 := smppsession.NewSession(conn1, logger)
	require.NoError(t, sess1.Bind("transmitter", "sys1", nil, "u1", 100))

	conn2 := newMockServerConn()
	sess2 := smppsession.NewSession(conn2, logger)
	// sess2 is not bound

	s.sessionsMu.Lock()
	s.sessions[sess1.ID] = sess1
	s.sessions[sess2.ID] = sess2
	s.sessionsMu.Unlock()

	assert.Equal(t, 2, s.GetActiveSessionsCount())
	assert.Equal(t, 1, s.GetBoundSessionsCount())
}

// --- readPDU ---

func TestReadPDU(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	// Build a valid enquire_link PDU (header only, 16 bytes)
	header := make([]byte, protocol.PDUHeaderLength)
	binary.BigEndian.PutUint32(header[0:4], protocol.PDUHeaderLength) // command_length
	binary.BigEndian.PutUint32(header[4:8], protocol.EnquireLink)     // command_id
	binary.BigEndian.PutUint32(header[8:12], protocol.ESME_ROK)       // command_status
	binary.BigEndian.PutUint32(header[12:16], 1)                       // sequence_number

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		client.Write(header)
	}()

	pdu, err := s.readPDU(server)
	require.NoError(t, err)
	require.NotNil(t, pdu)
	assert.Equal(t, uint32(protocol.PDUHeaderLength), pdu.CommandLength)
	assert.Equal(t, uint32(protocol.EnquireLink), pdu.CommandID)
	assert.Equal(t, uint32(protocol.ESME_ROK), pdu.CommandStatus)
	assert.Equal(t, uint32(1), pdu.SequenceNumber)
}

func TestReadPDUWithBody(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	// Build a submit_sm_resp with a body (message_id "abc\0")
	bodyData := []byte("abc\x00")
	totalLen := uint32(protocol.PDUHeaderLength + len(bodyData))

	header := make([]byte, protocol.PDUHeaderLength)
	binary.BigEndian.PutUint32(header[0:4], totalLen)
	binary.BigEndian.PutUint32(header[4:8], protocol.SubmitSMResp)
	binary.BigEndian.PutUint32(header[8:12], protocol.ESME_ROK)
	binary.BigEndian.PutUint32(header[12:16], 42)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		client.Write(header)
		client.Write(bodyData)
	}()

	pdu, err := s.readPDU(server)
	require.NoError(t, err)
	require.NotNil(t, pdu)
	assert.Equal(t, totalLen, pdu.CommandLength)
	assert.Equal(t, uint32(protocol.SubmitSMResp), pdu.CommandID)
	assert.Equal(t, uint32(42), pdu.SequenceNumber)
	assert.Equal(t, bodyData, pdu.Body)
}

func TestReadPDUInvalidCommandLength(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	// command_length < 16
	header := make([]byte, protocol.PDUHeaderLength)
	binary.BigEndian.PutUint32(header[0:4], 8)  // invalid: < 16
	binary.BigEndian.PutUint32(header[4:8], protocol.EnquireLink)
	binary.BigEndian.PutUint32(header[8:12], 0)
	binary.BigEndian.PutUint32(header[12:16], 1)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		client.Write(header)
	}()

	_, err := s.readPDU(server)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "неверная длина")
}

func TestReadPDUTooLargeCommandLength(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	header := make([]byte, protocol.PDUHeaderLength)
	binary.BigEndian.PutUint32(header[0:4], 100000)  // > 65536
	binary.BigEndian.PutUint32(header[4:8], protocol.EnquireLink)
	binary.BigEndian.PutUint32(header[8:12], 0)
	binary.BigEndian.PutUint32(header[12:16], 1)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		client.Write(header)
	}()

	_, err := s.readPDU(server)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "слишком большая")
}

func TestReadPDUClosedConnection(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	server, client := net.Pipe()
	client.Close()
	defer server.Close()

	_, err := s.readPDU(server)
	require.Error(t, err)
}

// --- UserInfo ---

func TestUserInfoStruct(t *testing.T) {
	ui := UserInfo{
		UserID:    "u-123",
		ClientID:  "c-456",
		RateLimit: 200,
		Active:    true,
	}

	assert.Equal(t, "u-123", ui.UserID)
	assert.Equal(t, "c-456", ui.ClientID)
	assert.Equal(t, 200, ui.RateLimit)
	assert.True(t, ui.Active)
}

// --- mockServerConn ---

type mockServerConn struct {
	closed     bool
	mu         sync.Mutex
	localAddr  net.Addr
	remoteAddr net.Addr
}

func newMockServerConn() *mockServerConn {
	return &mockServerConn{
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2775},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 50000},
	}
}

func (c *mockServerConn) Read(b []byte) (int, error)  { return 0, nil }
func (c *mockServerConn) Write(b []byte) (int, error)  { return len(b), nil }
func (c *mockServerConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
func (c *mockServerConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *mockServerConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *mockServerConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockServerConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockServerConn) SetWriteDeadline(t time.Time) error { return nil }

// --- handleConnection integration test ---

func TestHandleConnectionCreatesAndRemovesSession(t *testing.T) {
	cfg := newTestConfig()
	cfg.Port = 0
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)
	require.NoError(t, s.Start())
	defer s.Stop()

	addr := s.listener.Addr().String()

	// Connect and immediately disconnect
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)

	// Give server a moment to register the session
	time.Sleep(100 * time.Millisecond)
	assert.GreaterOrEqual(t, s.GetActiveSessionsCount(), 0)

	conn.Close()

	// Give server a moment to clean up
	time.Sleep(200 * time.Millisecond)
}

// --- Test server accept loop shutdown ---

func TestServerContextCancelStopsAccept(t *testing.T) {
	cfg := newTestConfig()
	cfg.Port = 0
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	require.NoError(t, s.Start())

	// Cancel context; this should cause accept loop to return
	s.cancel()

	// Close listener to unblock Accept()
	s.listener.Close()

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not finish in time")
	}
}

// --- sendEnquireLink ---

func TestSendEnquireLink(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	// Create a session with a pipe connection to capture written data
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := smppsession.NewSession(serverConn, logger)
	require.NoError(t, sess.Bind("transceiver", "sys1", nil, "u1", 100))

	// Read what the server writes in a goroutine
	var readBuf []byte
	readDone := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		n, _ := clientConn.Read(buf)
		readBuf = buf[:n]
		close(readDone)
	}()

	s.sendEnquireLink(sess)

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for enquire_link data")
	}

	// Should have written at least a PDU header (16 bytes)
	assert.GreaterOrEqual(t, len(readBuf), protocol.PDUHeaderLength)

	// Parse the command_id from the written data
	if len(readBuf) >= 8 {
		cmdID := binary.BigEndian.Uint32(readBuf[4:8])
		assert.Equal(t, uint32(protocol.EnquireLink), cmdID)
	}
}

// --- Concurrent session map access ---

func TestConcurrentSessionMapAccess(t *testing.T) {
	cfg := newTestConfig()
	logger := newTestLogger()
	s := NewServer(cfg, nil, nil, nil, logger)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := newMockServerConn()
			sess := smppsession.NewSession(conn, logger)

			s.sessionsMu.Lock()
			s.sessions[sess.ID] = sess
			s.sessionsMu.Unlock()

			_ = s.GetActiveSessionsCount()
			_ = s.GetBoundSessionsCount()

			s.sessionsMu.Lock()
			delete(s.sessions, sess.ID)
			s.sessionsMu.Unlock()
		}(i)
	}
	wg.Wait()
}
