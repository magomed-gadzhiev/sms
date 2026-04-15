package server

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/api/proto/smppv1"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGRPCServer() (*GRPCServer, map[string]*smppsession.Session, *sync.RWMutex) {
	sessions := make(map[string]*smppsession.Session)
	mu := &sync.RWMutex{}
	logger := zerolog.Nop()
	srv := NewGRPCServer(sessions, mu, logger)
	return srv, sessions, mu
}

func TestDeliverDLR_Success(t *testing.T) {
	srv, sessions, _ := newTestGRPCServer()

	// Create a pipe to simulate a TCP connection
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Create a session bound as transceiver
	sess := smppsession.NewSession(serverConn, zerolog.Nop())
	err := sess.Bind("transceiver", "test_client", nil, "user1", 100)
	require.NoError(t, err)
	sessions[sess.ID] = sess

	// Read the PDU written to the pipe in a goroutine
	type readResult struct {
		data []byte
		err  error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		// First read the 4-byte command_length
		header := make([]byte, 4)
		_, err := clientConn.Read(header)
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		cmdLen := binary.BigEndian.Uint32(header)
		// Read the rest of the PDU
		rest := make([]byte, cmdLen-4)
		_, err = clientConn.Read(rest)
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		full := append(header, rest...)
		resultCh <- readResult{data: full}
	}()

	// Call DeliverDLR
	req := &smppv1.DeliverDLRRequest{
		SystemId:        "test_client",
		SourceAddr:      "SenderName",
		DestinationAddr: "380501234567",
		ReceiptText:     "id:12345 sub:001 dlvrd:001 submit date:2604151200 done date:2604151201 stat:DELIVRD err:000 text:Hello",
	}

	resp, err := srv.DeliverDLR(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Delivered, "expected delivered=true")
	assert.Empty(t, resp.Error, "expected no error")

	// Verify the PDU was written correctly
	result := <-resultCh
	require.NoError(t, result.err, "failed to read PDU from pipe")
	require.True(t, len(result.data) >= protocol.PDUHeaderLength, "PDU too short")

	// Verify PDU header
	cmdLen := binary.BigEndian.Uint32(result.data[0:4])
	cmdID := binary.BigEndian.Uint32(result.data[4:8])
	cmdStatus := binary.BigEndian.Uint32(result.data[8:12])

	assert.Equal(t, uint32(len(result.data)), cmdLen, "command_length mismatch")
	assert.Equal(t, uint32(protocol.DeliverSM), cmdID, "command_id should be deliver_sm")
	assert.Equal(t, uint32(protocol.ESME_ROK), cmdStatus, "command_status should be ESME_ROK")
}

func TestDeliverDLR_Success_ReceiverBind(t *testing.T) {
	srv, sessions, _ := newTestGRPCServer()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	sess := smppsession.NewSession(serverConn, zerolog.Nop())
	err := sess.Bind("receiver", "rx_client", nil, "user2", 100)
	require.NoError(t, err)
	sessions[sess.ID] = sess

	// Drain the pipe so the write doesn't block
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	req := &smppv1.DeliverDLRRequest{
		SystemId:        "rx_client",
		SourceAddr:      "Sender",
		DestinationAddr: "380501234567",
		ReceiptText:     "id:99 sub:001 dlvrd:001 stat:DELIVRD err:000",
	}

	resp, err := srv.DeliverDLR(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Delivered)
	assert.Empty(t, resp.Error)
}

func TestDeliverDLR_SessionNotFound(t *testing.T) {
	srv, _, _ := newTestGRPCServer()

	req := &smppv1.DeliverDLRRequest{
		SystemId:        "unknown_client",
		SourceAddr:      "Sender",
		DestinationAddr: "380501234567",
		ReceiptText:     "id:1 sub:001 dlvrd:000 stat:UNDELIV err:001",
	}

	resp, err := srv.DeliverDLR(context.Background(), req)
	require.NoError(t, err, "DeliverDLR should not return a gRPC error")
	assert.False(t, resp.Delivered)
	assert.Contains(t, resp.Error, "session not found")
}

func TestDeliverDLR_SessionCannotReceive(t *testing.T) {
	srv, sessions, _ := newTestGRPCServer()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Bind as transmitter (cannot receive)
	sess := smppsession.NewSession(serverConn, zerolog.Nop())
	err := sess.Bind("transmitter", "tx_only", nil, "user3", 100)
	require.NoError(t, err)
	sessions[sess.ID] = sess

	req := &smppv1.DeliverDLRRequest{
		SystemId:        "tx_only",
		SourceAddr:      "Sender",
		DestinationAddr: "380501234567",
		ReceiptText:     "id:1 sub:001 dlvrd:000 stat:UNDELIV err:001",
	}

	resp, err := srv.DeliverDLR(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Delivered)
	assert.Contains(t, resp.Error, "cannot receive")
}

func TestDeliverDLR_ConnectionClosed(t *testing.T) {
	srv, sessions, _ := newTestGRPCServer()

	clientConn, serverConn := net.Pipe()
	// Close both ends immediately to simulate a broken connection
	clientConn.Close()
	serverConn.Close()

	sess := smppsession.NewSession(serverConn, zerolog.Nop())
	err := sess.Bind("transceiver", "dead_client", nil, "user4", 100)
	require.NoError(t, err)
	sessions[sess.ID] = sess

	req := &smppv1.DeliverDLRRequest{
		SystemId:        "dead_client",
		SourceAddr:      "Sender",
		DestinationAddr: "380501234567",
		ReceiptText:     "id:1 sub:001 dlvrd:000 stat:UNDELIV err:001",
	}

	resp, err := srv.DeliverDLR(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Delivered)
	assert.Contains(t, resp.Error, "failed to write deliver_sm")
}

func TestFindSessionBySystemID_SkipsUnboundSessions(t *testing.T) {
	srv, sessions, _ := newTestGRPCServer()

	_, serverConn := net.Pipe()
	defer serverConn.Close()

	// Create an unbound session with a matching SystemID
	sess := smppsession.NewSession(serverConn, zerolog.Nop())
	sess.SystemID = "unbound_client"
	// Don't bind — state remains Open
	sessions[sess.ID] = sess

	found := srv.findSessionBySystemID("unbound_client")
	assert.Nil(t, found, "should not find unbound session")
}
