package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/api/proto/smppv1"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
)

// GRPCServer implements the smppv1.SMPPGatewayServer interface.
// It handles DeliverDLR requests from the dlr-delivery service by finding
// the target SMPP session and writing a deliver_sm PDU to its TCP connection.
type GRPCServer struct {
	smppv1.UnimplementedSMPPGatewayServer
	sessions   map[string]*smppsession.Session
	sessionsMu *sync.RWMutex
	logger     zerolog.Logger
}

// NewGRPCServer creates a new GRPCServer that shares the sessions map
// and mutex with the main SMPP Server.
func NewGRPCServer(sessions map[string]*smppsession.Session, sessionsMu *sync.RWMutex, logger zerolog.Logger) *GRPCServer {
	return &GRPCServer{
		sessions:   sessions,
		sessionsMu: sessionsMu,
		logger:     logger.With().Str("component", "smpp_grpc_server").Logger(),
	}
}

// NewGRPCServerFromServer creates a GRPCServer from an existing SMPP Server,
// sharing its sessions map and mutex.
func NewGRPCServerFromServer(s *Server) *GRPCServer {
	return NewGRPCServer(s.sessions, &s.sessionsMu, s.logger)
}

// DeliverDLR delivers a DLR (Delivery Receipt) to a connected SMPP client.
// It finds the session by system_id, verifies it can receive, builds a
// deliver_sm PDU with esm_class=0x04, and writes it to the client's TCP connection.
func (g *GRPCServer) DeliverDLR(ctx context.Context, req *smppv1.DeliverDLRRequest) (*smppv1.DeliverDLRResponse, error) {
	start := time.Now()
	systemID := req.GetSystemId()

	g.logger.Debug().
		Str("system_id", systemID).
		Str("source_addr", req.GetSourceAddr()).
		Str("destination_addr", req.GetDestinationAddr()).
		Msg("DeliverDLR request received")

	// Find a bound session for this system_id
	sess := g.findSessionBySystemID(systemID)
	if sess == nil {
		reason := "session_not_found"
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(systemID, reason).Inc()
		g.logger.Warn().
			Str("system_id", systemID).
			Msg("no bound session found for DLR delivery")
		return &smppv1.DeliverDLRResponse{
			Delivered: false,
			Error:     fmt.Sprintf("session not found for system_id %s", systemID),
		}, nil
	}

	// Verify session can receive (bound as receiver or transceiver)
	if !sess.CanReceive() {
		reason := "session_cannot_receive"
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(systemID, reason).Inc()
		g.logger.Warn().
			Str("system_id", systemID).
			Str("session_id", sess.ID).
			Str("bind_type", sess.BindType).
			Msg("session cannot receive DLR (not receiver/transceiver)")
		return &smppv1.DeliverDLRResponse{
			Delivered: false,
			Error:     fmt.Sprintf("session %s cannot receive (bind_type: %s)", sess.ID, sess.BindType),
		}, nil
	}

	// Build and send the deliver_sm PDU
	if err := g.sendDeliverSM(sess, req); err != nil {
		reason := "write_failed"
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(systemID, reason).Inc()
		g.logger.Error().Err(err).
			Str("system_id", systemID).
			Str("session_id", sess.ID).
			Msg("failed to send deliver_sm PDU")
		return &smppv1.DeliverDLRResponse{
			Delivered: false,
			Error:     fmt.Sprintf("failed to write deliver_sm: %v", err),
		}, nil
	}

	duration := time.Since(start).Seconds()
	monitoring.SMPPProcessingDuration.WithLabelValues("deliver_dlr").Observe(duration)
	monitoring.SMPPDLRDelivered.WithLabelValues(systemID).Inc()

	g.logger.Info().
		Str("system_id", systemID).
		Str("session_id", sess.ID).
		Float64("duration_s", duration).
		Msg("DLR delivered successfully")

	return &smppv1.DeliverDLRResponse{
		Delivered: true,
	}, nil
}

// findSessionBySystemID iterates over sessions looking for a bound session
// matching the given system_id. Returns nil if no matching session is found.
func (g *GRPCServer) findSessionBySystemID(systemID string) *smppsession.Session {
	g.sessionsMu.RLock()
	defer g.sessionsMu.RUnlock()

	for _, sess := range g.sessions {
		if sess.SystemID == systemID && sess.IsBound() {
			return sess
		}
	}
	return nil
}

// sendDeliverSM builds a deliver_sm PDU with esm_class=0x04 (delivery receipt)
// and writes it to the session's TCP connection.
func (g *GRPCServer) sendDeliverSM(sess *smppsession.Session, req *smppv1.DeliverDLRRequest) error {
	conn := sess.GetConn()
	if conn == nil {
		return smppsession.ErrConnectionClosed
	}

	receiptBytes := []byte(req.GetReceiptText())

	// Build the deliver_sm body
	deliverPDU := &protocol.DeliverSMPDU{
		SourceAddrTON:   protocol.TON_UNKNOWN,
		SourceAddrNPI:   protocol.NPI_UNKNOWN,
		SourceAddr:      req.GetSourceAddr(),
		DestAddrTON:     protocol.TON_UNKNOWN,
		DestAddrNPI:     protocol.NPI_UNKNOWN,
		DestinationAddr: req.GetDestinationAddr(),
		ESMClass:        0x04, // Delivery receipt
		SMLength:        byte(len(receiptBytes)),
		ShortMessage:    receiptBytes,
	}

	encoder := protocol.NewEncoder()
	body, err := encoder.EncodeDeliverSM(deliverPDU)
	if err != nil {
		return fmt.Errorf("encode deliver_sm body: %w", err)
	}

	seqNum := sess.NextSequenceNumber()

	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.DeliverSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: seqNum,
		Body:           body,
	}

	data, err := encoder.EncodePDU(pdu)
	if err != nil {
		return fmt.Errorf("encode deliver_sm PDU: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write deliver_sm to connection: %w", err)
	}

	return nil
}
