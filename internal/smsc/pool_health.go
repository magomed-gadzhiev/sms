package smsc

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	smppprotocol "github.com/smpp-server/smpp-server/internal/smpp/protocol"
)

// enquireLinkLoop запускает цикл enquire link для поддержания соединения
func (p *Pool) enquireLinkLoop(conn *Connection, provider *shared.Provider) {
	ticker := time.NewTicker(60 * time.Second) // enquire link каждые 60 секунд
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.enquireLink(conn); err != nil {
				conn.logger.Error().Err(err).Msg("ошибка enquire link")
				// Переподключаемся
				p.reconnect(conn, provider)
				return
			}
		}
	}
}

// enquireLink отправляет enquire_link для проверки соединения
func (p *Pool) enquireLink(conn *Connection) error {
	seqNum := atomic.AddUint32(&p.seqNum, 1)
	conn.SequenceNum = seqNum

	encoder := smppprotocol.NewEncoder()
	body, err := encoder.EncodeEnquireLink(&smppprotocol.EnquireLinkPDU{})
	if err != nil {
		return err
	}

	pdu := &smppprotocol.PDU{
		CommandLength:  uint32(smppprotocol.PDUHeaderLength + len(body)),
		CommandID:      smppprotocol.EnquireLink,
		CommandStatus:  0,
		SequenceNumber: seqNum,
		Body:           body,
	}

	pduBytes, err := encoder.EncodePDU(pdu)
	if err != nil {
		return err
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Conn.Write(pduBytes); err != nil {
		return err
	}

	// Читаем ответ
	conn.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	header := make([]byte, smppprotocol.PDUHeaderLength)
	if _, err := conn.Conn.Read(header); err != nil {
		return err
	}

	var respCommandStatus uint32
	respCommandStatus = uint32(header[8])<<24 | uint32(header[9])<<16 | uint32(header[10])<<8 | uint32(header[11])

	if respCommandStatus != smppprotocol.ESME_ROK {
		return fmt.Errorf("enquire_link failed: статус 0x%08X", respCommandStatus)
	}

	return nil
}

// HealthCheck проверяет здоровье соединений
func (p *Pool) HealthCheck() map[uuid.UUID]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := make(map[uuid.UUID]int)
	for providerID, conns := range p.connections {
		healthyCount := 0
		for _, conn := range conns {
			conn.mu.RLock()
			if conn.Bound {
				healthyCount++
			}
			conn.mu.RUnlock()
		}
		health[providerID] = healthyCount
	}

	return health
}
