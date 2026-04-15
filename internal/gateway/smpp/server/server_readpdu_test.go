package server

import (
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPDU_HandlesFragmentedTCPRead(t *testing.T) {
	// Build a valid enquire_link PDU (header only, 16 bytes)
	encoder := protocol.NewEncoder()
	pdu := &protocol.PDU{
		CommandLength:  16,
		CommandID:      protocol.EnquireLink,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: 1,
	}
	data, err := encoder.EncodePDU(pdu)
	require.NoError(t, err)

	// Create a pipe: write full PDU, but read side will fragment
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		// Write one byte at a time to simulate fragmentation
		for i := 0; i < len(data); i++ {
			clientConn.Write(data[i : i+1])
			time.Sleep(1 * time.Millisecond)
		}
	}()

	srv := &Server{
		config: &config.SMSPConfig{ReadTimeout: 5 * time.Second},
		logger: zerolog.Nop(),
	}

	result, err := srv.readPDU(serverConn)
	require.NoError(t, err)
	assert.Equal(t, uint32(protocol.EnquireLink), result.CommandID)
	assert.Equal(t, uint32(1), result.SequenceNumber)
}
