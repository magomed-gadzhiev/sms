package application

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// TestConnectionConfig — параметры для теста SMPP-соединения
type TestConnectionConfig struct {
	Host     string
	Port     int
	SystemID string
	Password string
	BindType string        // "transceiver", "transmitter", "receiver"
	Timeout  time.Duration // 0 = default 10s
}

// TestConnectionResult — результат теста
type TestConnectionResult struct {
	Success   bool
	LatencyMs int64
	Log       []string
	Error     string
}

// TestConnectionService выполняет SMPP bind-тест
type TestConnectionService struct{}

func NewTestConnectionService() *TestConnectionService {
	return &TestConnectionService{}
}

func (s *TestConnectionService) Test(ctx context.Context, cfg TestConnectionConfig) TestConnectionResult {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	var logs []string
	start := time.Now()

	addLog := func(msg string) {
		logs = append(logs, msg)
	}

	// Шаг 1: TCP connect
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	addLog(fmt.Sprintf("→ TCP connect %s ...", addr))

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		addLog(fmt.Sprintf("✗ TCP connect failed: %v", err))
		return TestConnectionResult{
			Success: false,
			Log:     logs,
			Error:   err.Error(),
		}
	}
	defer conn.Close()
	addLog("✓ TCP connect OK")

	// Шаг 2: SMPP bind
	bindCmd := bindCommandID(cfg.BindType)
	pdu := buildBindPDU(bindCmd, cfg.SystemID, cfg.Password)
	addLog(fmt.Sprintf("→ SMPP bind_%s (system_id=%s) ...", cfg.BindType, cfg.SystemID))

	conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(pdu); err != nil {
		addLog(fmt.Sprintf("✗ send bind failed: %v", err))
		return TestConnectionResult{Success: false, Log: logs, Error: err.Error()}
	}

	// Шаг 3: читаем ответ (минимум 16 байт — SMPP header)
	header := make([]byte, 16)
	if _, err := conn.Read(header); err != nil {
		addLog(fmt.Sprintf("✗ read bind_resp failed: %v", err))
		return TestConnectionResult{Success: false, Log: logs, Error: err.Error()}
	}

	cmdStatus := binary.BigEndian.Uint32(header[8:12])
	if cmdStatus != 0 {
		errMsg := fmt.Sprintf("SMPP bind_resp command_status=0x%08X", cmdStatus)
		if cmdStatus == 0x0000000D {
			errMsg += " (ESME_RBINDFAIL: неверный system_id или пароль)"
		}
		addLog(fmt.Sprintf("✗ %s", errMsg))
		return TestConnectionResult{Success: false, Log: logs, Error: errMsg}
	}
	addLog(fmt.Sprintf("✓ bind_%s_resp (command_status=0x00000000) OK", cfg.BindType))

	// Шаг 4: unbind
	addLog("→ unbind ...")
	unbind := buildUnbindPDU()
	conn.Write(unbind) // best-effort
	addLog("✓ unbind OK")

	latency := time.Since(start).Milliseconds()
	return TestConnectionResult{
		Success:   true,
		LatencyMs: latency,
		Log:       logs,
	}
}

func bindCommandID(bindType string) uint32 {
	switch bindType {
	case "transmitter":
		return 0x00000002
	case "receiver":
		return 0x00000001
	default: // transceiver
		return 0x00000009
	}
}

func buildBindPDU(commandID uint32, systemID, password string) []byte {
	body := []byte{}
	body = append(body, []byte(systemID)...)
	body = append(body, 0)
	body = append(body, []byte(password)...)
	body = append(body, 0)
	body = append(body, 0)    // system_type (empty)
	body = append(body, 0x34) // interface_version: SMPP 3.4
	body = append(body, 0)    // addr_ton
	body = append(body, 0)    // addr_npi
	body = append(body, 0)    // address_range (empty)

	length := uint32(16 + len(body))
	pdu := make([]byte, 16)
	binary.BigEndian.PutUint32(pdu[0:4], length)
	binary.BigEndian.PutUint32(pdu[4:8], commandID)
	binary.BigEndian.PutUint32(pdu[8:12], 0)
	binary.BigEndian.PutUint32(pdu[12:16], 1)
	pdu = append(pdu, body...)
	return pdu
}

func buildUnbindPDU() []byte {
	pdu := make([]byte, 16)
	binary.BigEndian.PutUint32(pdu[0:4], 16)
	binary.BigEndian.PutUint32(pdu[4:8], 0x00000006)
	binary.BigEndian.PutUint32(pdu[8:12], 0)
	binary.BigEndian.PutUint32(pdu[12:16], 2)
	return pdu
}
