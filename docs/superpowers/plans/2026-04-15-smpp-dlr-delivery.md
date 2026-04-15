# SMPP DLR Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver DLR (delivery receipts) to SMPP clients via a dedicated dlr-delivery microservice that consumes `sms.status` Kafka topic, looks up session bindings in Redis, and dispatches deliver_sm through smpp-gateway's new internal gRPC endpoint.

**Architecture:** New `dlr-delivery-service` consumes `sms.status`, resolves `message_id → system_id → gateway_addr` via Redis, formats SMSC receipt, calls smpp-gateway gRPC `DeliverDLR`. smpp-gateway writes session/message bindings to Redis and exposes internal gRPC for DLR dispatch. Also fixes `readPDU` TCP fragmentation bug.

**Tech Stack:** Go 1.24.0, sarama (Kafka), go-redis/v9 (Redis), gRPC + protobuf, zerolog, prometheus, testify

**Spec:** `docs/superpowers/specs/2026-04-15-smpp-dlr-delivery-design.md`

---

## File Structure

### New files
```
api/proto/smppv1/smpp.proto                      — gRPC service definition for DeliverDLR
internal/services/dlr/consumer.go                 — Kafka consumer for sms.status topic
internal/services/dlr/consumer_test.go            — tests for consumer
internal/services/dlr/formatter.go                — SMSC receipt string formatter
internal/services/dlr/formatter_test.go           — tests for formatter
internal/services/dlr/dispatcher.go               — gRPC client to smpp-gateway
internal/services/dlr/dispatcher_test.go          — tests for dispatcher
internal/gateway/smpp/server/grpc_server.go       — internal gRPC server for DeliverDLR
internal/gateway/smpp/server/grpc_server_test.go  — tests for gRPC server
internal/gateway/smpp/server/redis_store.go       — Redis session/message mapping
internal/gateway/smpp/server/redis_store_test.go  — tests for Redis store
cmd/dlr-delivery/main.go                          — dlr-delivery service entry point
deployments/docker/dlr-delivery.Dockerfile         — Docker build for dlr-delivery
```

### Modified files
```
internal/gateway/smpp/server/server.go            — add Redis + gRPC, fix readPDU
internal/gateway/smpp/server/handler.go           — write Redis mappings on submit_sm
internal/monitoring/metrics.go                     — add DLR metrics
internal/config/config.go                          — add DLR config fields
cmd/smpp-gateway/main.go                          — init Redis, start gRPC server
deployments/docker-compose.yml                    — add dlr-delivery service
deployments/configs/prometheus.yml                — add dlr-delivery scrape target
```

---

### Task 1: Fix readPDU TCP fragmentation bug

**Files:**
- Modify: `internal/gateway/smpp/server/server.go:247-280`

- [ ] **Step 1: Write failing test for partial TCP read**

Create `internal/gateway/smpp/server/server_readpdu_test.go`:

```go
package server

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowReader simulates TCP fragmentation by returning one byte at a time
type slowReader struct {
	data []byte
	pos  int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	// Return only 1 byte at a time to simulate fragmentation
	p[0] = s.data[s.pos]
	s.pos++
	return 1, nil
}

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
	assert.Equal(t, protocol.EnquireLink, result.CommandID)
	assert.Equal(t, uint32(1), result.SequenceNumber)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestReadPDU_HandlesFragmentedTCPRead -v`
Expected: FAIL — `conn.Read` returns partial data, decoder fails

- [ ] **Step 3: Fix readPDU to use io.ReadFull**

In `internal/gateway/smpp/server/server.go`, replace the `readPDU` method:

```go
// readPDU читает один PDU из соединения
func (s *Server) readPDU(conn net.Conn) (*protocol.PDU, error) {
	// Читаем заголовок (16 байт) — io.ReadFull гарантирует полное чтение
	header := make([]byte, protocol.PDUHeaderLength)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовка: %w", err)
	}

	// Декодируем длину команды
	commandLength := binary.BigEndian.Uint32(header[0:4])

	if commandLength < protocol.PDUHeaderLength {
		return nil, fmt.Errorf("неверная длина команды: %d", commandLength)
	}

	if commandLength > 65536 { // Максимальный разумный размер
		return nil, fmt.Errorf("слишком большая длина команды: %d", commandLength)
	}

	// Читаем тело — io.ReadFull гарантирует полное чтение
	bodyLength := int(commandLength) - protocol.PDUHeaderLength
	body := make([]byte, bodyLength)
	if bodyLength > 0 {
		if _, err := io.ReadFull(conn, body); err != nil {
			return nil, fmt.Errorf("ошибка чтения тела: %w", err)
		}
	}

	// Декодируем PDU
	decoder := protocol.NewDecoder(append(header, body...))
	pdu, err := decoder.DecodePDU()
	if err != nil {
		return nil, fmt.Errorf("ошибка декодирования PDU: %w", err)
	}

	return pdu, nil
}
```

Add `"io"` to imports in `server.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestReadPDU_HandlesFragmentedTCPRead -v`
Expected: PASS

- [ ] **Step 5: Run all existing server tests to check for regressions**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/smpp/server/server.go internal/gateway/smpp/server/server_readpdu_test.go
git commit -m "fix(smpp): use io.ReadFull in readPDU to handle TCP fragmentation"
```

---

### Task 2: SMSC Receipt Formatter

**Files:**
- Create: `internal/services/dlr/formatter.go`
- Create: `internal/services/dlr/formatter_test.go`

- [ ] **Step 1: Write failing tests for SMSC receipt formatting**

Create `internal/services/dlr/formatter_test.go`:

```go
package dlr

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatReceipt_Delivered(t *testing.T) {
	submitDate := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC)
	doneDate := time.Date(2026, 4, 15, 10, 30, 5, 0, time.UTC)
	msgID := uuid.New().String()

	receipt := FormatReceipt(ReceiptParams{
		MessageID:  msgID,
		Status:     "delivered",
		SubmitDate: submitDate,
		DoneDate:   doneDate,
		ErrorCode:  0,
		Text:       "Hello world test message",
	})

	assert.Contains(t, receipt, "id:"+msgID)
	assert.Contains(t, receipt, "stat:DELIVRD")
	assert.Contains(t, receipt, "dlvrd:001")
	assert.Contains(t, receipt, "err:000")
	assert.Contains(t, receipt, "submit date:2604151030")
	assert.Contains(t, receipt, "done date:2604151030")
	assert.Contains(t, receipt, "text:Hello world test mes")
}

func TestFormatReceipt_Failed(t *testing.T) {
	submitDate := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC)
	doneDate := time.Date(2026, 4, 15, 10, 31, 0, 0, time.UTC)

	receipt := FormatReceipt(ReceiptParams{
		MessageID:  "test-msg-1",
		Status:     "failed",
		SubmitDate: submitDate,
		DoneDate:   doneDate,
		ErrorCode:  5,
		Text:       "Hi",
	})

	assert.Contains(t, receipt, "stat:UNDELIV")
	assert.Contains(t, receipt, "dlvrd:000")
	assert.Contains(t, receipt, "err:005")
}

func TestFormatReceipt_Expired(t *testing.T) {
	now := time.Now()
	receipt := FormatReceipt(ReceiptParams{
		MessageID:  "test-msg-2",
		Status:     "expired",
		SubmitDate: now,
		DoneDate:   now,
		ErrorCode:  0,
		Text:       "",
	})

	assert.Contains(t, receipt, "stat:EXPIRED")
	assert.Contains(t, receipt, "dlvrd:000")
	assert.Contains(t, receipt, "text:")
}

func TestFormatReceipt_Rejected(t *testing.T) {
	now := time.Now()
	receipt := FormatReceipt(ReceiptParams{
		MessageID:  "test-msg-3",
		Status:     "rejected",
		SubmitDate: now,
		DoneDate:   now,
		ErrorCode:  0,
		Text:       "",
	})

	assert.Contains(t, receipt, "stat:REJECTD")
}

func TestFormatReceipt_TextTruncation(t *testing.T) {
	now := time.Now()
	longText := "This is a very long message that should be truncated to 20 characters"

	receipt := FormatReceipt(ReceiptParams{
		MessageID:  "test-msg-4",
		Status:     "delivered",
		SubmitDate: now,
		DoneDate:   now,
		ErrorCode:  0,
		Text:       longText,
	})

	assert.Contains(t, receipt, "text:This is a very long ")
}

func TestMapStatusToSMSC(t *testing.T) {
	tests := []struct {
		status string
		stat   string
		dlvrd  string
	}{
		{"delivered", "DELIVRD", "001"},
		{"failed", "UNDELIV", "000"},
		{"expired", "EXPIRED", "000"},
		{"rejected", "REJECTD", "000"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			stat, dlvrd := MapStatusToSMSC(tt.status)
			require.Equal(t, tt.stat, stat)
			require.Equal(t, tt.dlvrd, dlvrd)
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -run TestFormat -v`
Expected: FAIL — package/functions don't exist

- [ ] **Step 3: Implement formatter**

Create `internal/services/dlr/formatter.go`:

```go
package dlr

import (
	"fmt"
	"time"
)

// ReceiptParams содержит параметры для формирования SMSC receipt
type ReceiptParams struct {
	MessageID  string
	Status     string
	SubmitDate time.Time
	DoneDate   time.Time
	ErrorCode  int
	Text       string
}

// FormatReceipt формирует стандартный SMSC delivery receipt string
func FormatReceipt(p ReceiptParams) string {
	stat, dlvrd := MapStatusToSMSC(p.Status)
	text := p.Text
	if len(text) > 20 {
		text = text[:20]
	}

	return fmt.Sprintf("id:%s sub:001 dlvrd:%s submit date:%s done date:%s stat:%s err:%03d text:%s",
		p.MessageID,
		dlvrd,
		formatSMPPDate(p.SubmitDate),
		formatSMPPDate(p.DoneDate),
		stat,
		p.ErrorCode,
		text,
	)
}

// MapStatusToSMSC maps pipeline status to SMSC receipt stat and dlvrd fields
func MapStatusToSMSC(status string) (stat string, dlvrd string) {
	switch status {
	case "delivered":
		return "DELIVRD", "001"
	case "failed":
		return "UNDELIV", "000"
	case "expired":
		return "EXPIRED", "000"
	case "rejected":
		return "REJECTD", "000"
	default:
		return "UNKNOWN", "000"
	}
}

// formatSMPPDate formats time as YYMMDDHHmm (SMPP date format)
func formatSMPPDate(t time.Time) string {
	return t.Format("0601021504")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -run TestFormat -v && go test ./internal/services/dlr/ -run TestMapStatus -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/dlr/formatter.go internal/services/dlr/formatter_test.go
git commit -m "feat(dlr): add SMSC receipt formatter with status mapping"
```

---

### Task 3: Redis Store for session/message mappings

**Files:**
- Create: `internal/gateway/smpp/server/redis_store.go`
- Create: `internal/gateway/smpp/server/redis_store_test.go`

- [ ] **Step 1: Write failing tests for Redis store**

Create `internal/gateway/smpp/server/redis_store_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRedisClient implements a minimal Redis interface for testing
type mockRedisClient struct {
	store map[string]string
}

func newMockRedis() *mockRedisClient {
	return &mockRedisClient{store: make(map[string]string)}
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.store[key] = value.(string)
	return nil
}

func (m *mockRedisClient) Get(ctx context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", ErrRedisKeyNotFound
	}
	return v, nil
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

func TestRedisStore_SaveAndGetMessageMapping(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	mapping := &MessageMapping{
		SystemID:   "aggregator1",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC),
	}

	err := store.SaveMessageMapping(ctx, "msg-123", mapping)
	require.NoError(t, err)

	got, err := store.GetMessageMapping(ctx, "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "aggregator1", got.SystemID)
	assert.Equal(t, "MyBrand", got.SourceAddr)
	assert.Equal(t, "380501234567", got.DestAddr)
}

func TestRedisStore_GetMessageMapping_NotFound(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	_, err := store.GetMessageMapping(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrRedisKeyNotFound)
}

func TestRedisStore_SaveAndGetSessionBinding(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{
		GatewayAddr: "smpp-gateway:9095",
	}

	err := store.SaveSessionBinding(ctx, "aggregator1", binding)
	require.NoError(t, err)

	got, err := store.GetSessionBinding(ctx, "aggregator1")
	require.NoError(t, err)
	assert.Equal(t, "smpp-gateway:9095", got.GatewayAddr)
}

func TestRedisStore_DeleteSessionBinding(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
	store.SaveSessionBinding(ctx, "aggregator1", binding)

	err := store.DeleteSessionBinding(ctx, "aggregator1")
	require.NoError(t, err)

	_, err = store.GetSessionBinding(ctx, "aggregator1")
	assert.ErrorIs(t, err, ErrRedisKeyNotFound)
}

func TestRedisStore_RefreshSessionTTL(t *testing.T) {
	mock := newMockRedis()
	store := NewRedisStore(mock, 24*time.Hour, 5*time.Minute)
	ctx := context.Background()

	binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
	store.SaveSessionBinding(ctx, "aggregator1", binding)

	// RefreshSessionTTL should not error for existing key
	err := store.RefreshSessionTTL(ctx, "aggregator1")
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestRedisStore -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement Redis store**

Create `internal/gateway/smpp/server/redis_store.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrRedisKeyNotFound = errors.New("redis key not found")

// RedisClient interface for Redis operations (allows mocking)
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// MessageMapping stores submit_sm metadata for DLR correlation
type MessageMapping struct {
	SystemID   string    `json:"system_id"`
	SourceAddr string    `json:"source_addr"`
	DestAddr   string    `json:"dest_addr"`
	SubmitDate time.Time `json:"submit_date"`
}

// SessionBinding stores active SMPP session location
type SessionBinding struct {
	GatewayAddr string `json:"gateway_addr"`
}

// RedisStore manages Redis mappings for SMPP DLR delivery
type RedisStore struct {
	client     RedisClient
	messageTTL time.Duration
	sessionTTL time.Duration
}

// NewRedisStore creates a new RedisStore
func NewRedisStore(client RedisClient, messageTTL, sessionTTL time.Duration) *RedisStore {
	return &RedisStore{
		client:     client,
		messageTTL: messageTTL,
		sessionTTL: sessionTTL,
	}
}

// SaveMessageMapping saves message_id → {system_id, source, dest, submit_date}
func (s *RedisStore) SaveMessageMapping(ctx context.Context, messageID string, mapping *MessageMapping) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("ошибка сериализации message mapping: %w", err)
	}
	key := fmt.Sprintf("smpp:msg:%s", messageID)
	return s.client.Set(ctx, key, string(data), s.messageTTL)
}

// GetMessageMapping retrieves message mapping by message_id
func (s *RedisStore) GetMessageMapping(ctx context.Context, messageID string) (*MessageMapping, error) {
	key := fmt.Sprintf("smpp:msg:%s", messageID)
	val, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var mapping MessageMapping
	if err := json.Unmarshal([]byte(val), &mapping); err != nil {
		return nil, fmt.Errorf("ошибка десериализации message mapping: %w", err)
	}
	return &mapping, nil
}

// SaveSessionBinding saves system_id → {gateway_addr}
func (s *RedisStore) SaveSessionBinding(ctx context.Context, systemID string, binding *SessionBinding) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("ошибка сериализации session binding: %w", err)
	}
	key := fmt.Sprintf("smpp:session:%s", systemID)
	return s.client.Set(ctx, key, string(data), s.sessionTTL)
}

// GetSessionBinding retrieves session binding by system_id
func (s *RedisStore) GetSessionBinding(ctx context.Context, systemID string) (*SessionBinding, error) {
	key := fmt.Sprintf("smpp:session:%s", systemID)
	val, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var binding SessionBinding
	if err := json.Unmarshal([]byte(val), &binding); err != nil {
		return nil, fmt.Errorf("ошибка десериализации session binding: %w", err)
	}
	return &binding, nil
}

// DeleteSessionBinding removes session binding on unbind/disconnect
func (s *RedisStore) DeleteSessionBinding(ctx context.Context, systemID string) error {
	key := fmt.Sprintf("smpp:session:%s", systemID)
	return s.client.Del(ctx, key)
}

// RefreshSessionTTL refreshes TTL for session binding (called on enquire_link)
func (s *RedisStore) RefreshSessionTTL(ctx context.Context, systemID string) error {
	key := fmt.Sprintf("smpp:session:%s", systemID)
	return s.client.Expire(ctx, key, s.sessionTTL)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestRedisStore -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/smpp/server/redis_store.go internal/gateway/smpp/server/redis_store_test.go
git commit -m "feat(smpp): add Redis store for session/message DLR mappings"
```

---

### Task 4: Add DLR metrics to monitoring

**Files:**
- Modify: `internal/monitoring/metrics.go`

- [ ] **Step 1: Add DLR metrics**

Add to `internal/monitoring/metrics.go` alongside existing SMPP metrics:

```go
// DLR delivery metrics
var DLREventsConsumed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "dlr_events_consumed_total",
		Help: "Общее количество DLR событий из Kafka",
	},
	[]string{"status"},
)

var DLREventsDispatched = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "dlr_events_dispatched_total",
		Help: "Общее количество DLR событий отправленных клиентам",
	},
	[]string{"status", "result"},
)

var DLRDispatchDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "dlr_dispatch_duration_seconds",
		Help:    "Длительность отправки DLR клиенту",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	},
	[]string{},
)

var DLRRedisLookupMiss = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "dlr_redis_lookup_miss_total",
		Help: "Промахи при Redis lookup для DLR",
	},
	[]string{"key_type"},
)

var SMPPDLRDelivered = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "smpp_dlr_delivered_total",
		Help: "Количество DLR успешно доставленных SMPP клиентам",
	},
	[]string{"system_id"},
)

var SMPPDLRDeliveryFailed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "smpp_dlr_delivery_failed_total",
		Help: "Количество неудачных доставок DLR SMPP клиентам",
	},
	[]string{"system_id", "reason"},
)
```

- [ ] **Step 2: Verify compilation**

Run: `cd c:/projects/sms && go build ./internal/monitoring/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/monitoring/metrics.go
git commit -m "feat(monitoring): add DLR delivery metrics"
```

---

### Task 5: gRPC proto definition for DeliverDLR

**Files:**
- Create: `api/proto/smppv1/smpp.proto`

- [ ] **Step 1: Create proto file**

Create `api/proto/smppv1/smpp.proto`:

```protobuf
syntax = "proto3";
package smppv1;
option go_package = "github.com/smpp-server/smpp-server/api/proto/smppv1";

service SMPPGateway {
  // DeliverDLR dispatches a delivery receipt to a connected SMPP client
  rpc DeliverDLR(DeliverDLRRequest) returns (DeliverDLRResponse);
}

message DeliverDLRRequest {
  string system_id = 1;
  string source_addr = 2;
  string destination_addr = 3;
  string receipt_text = 4;
}

message DeliverDLRResponse {
  bool delivered = 1;
  string error = 2;
}
```

- [ ] **Step 2: Generate Go code**

Run: `cd c:/projects/sms && protoc --go_out=. --go-grpc_out=. api/proto/smppv1/smpp.proto`
Expected: Generated files in `api/proto/smppv1/`

If `protoc` is not available locally, check for a Makefile target or generate script:
Run: `cd c:/projects/sms && ls Makefile scripts/proto* scripts/generate* 2>/dev/null`

- [ ] **Step 3: Verify generated code compiles**

Run: `cd c:/projects/sms && go build ./api/proto/smppv1/`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add api/proto/smppv1/
git commit -m "feat(proto): add smppv1 proto with DeliverDLR RPC"
```

---

### Task 6: Internal gRPC server in smpp-gateway for DeliverDLR

**Files:**
- Create: `internal/gateway/smpp/server/grpc_server.go`
- Create: `internal/gateway/smpp/server/grpc_server_test.go`

- [ ] **Step 1: Write failing test for gRPC DeliverDLR**

Create `internal/gateway/smpp/server/grpc_server_test.go`:

```go
package server

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRPCServer_DeliverDLR_Success(t *testing.T) {
	// Create a mock session that captures written PDUs
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	session := smppsession.NewSession(serverConn, zerolog.Nop())
	clientID := uuid.New()
	session.Bind("transceiver", "aggregator1", &clientID, "user-1", 100)

	sessions := make(map[string]*smppsession.Session)
	sessions[session.ID] = session

	// Read the deliver_sm PDU in background
	var receivedPDU *protocol.PDU
	var readErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		header := make([]byte, protocol.PDUHeaderLength)
		_, readErr = clientConn.Read(header)
		if readErr != nil {
			return
		}
		// Just verify we received something
		receivedPDU = &protocol.PDU{
			CommandID: protocol.DeliverSM,
		}
	}()

	grpcSrv := NewGRPCServer(sessions, &sync.RWMutex{}, zerolog.Nop())

	resp, err := grpcSrv.DeliverDLR(context.Background(), &DeliverDLRRequest{
		SystemId:        "aggregator1",
		SourceAddr:      "380501234567",
		DestinationAddr: "MyBrand",
		ReceiptText:     "id:msg-1 sub:001 dlvrd:001 submit date:2604151030 done date:2604151030 stat:DELIVRD err:000 text:Hello",
	})

	require.NoError(t, err)
	assert.True(t, resp.Delivered)
	assert.Empty(t, resp.Error)

	// Wait for PDU to be received
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for deliver_sm PDU")
	}
}

func TestGRPCServer_DeliverDLR_SessionNotFound(t *testing.T) {
	sessions := make(map[string]*smppsession.Session)
	grpcSrv := NewGRPCServer(sessions, &sync.RWMutex{}, zerolog.Nop())

	resp, err := grpcSrv.DeliverDLR(context.Background(), &DeliverDLRRequest{
		SystemId:        "unknown_system",
		SourceAddr:      "380501234567",
		DestinationAddr: "MyBrand",
		ReceiptText:     "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.False(t, resp.Delivered)
	assert.Contains(t, resp.Error, "session not found")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestGRPCServer -v`
Expected: FAIL — GRPCServer not defined

- [ ] **Step 3: Implement gRPC server**

Create `internal/gateway/smpp/server/grpc_server.go`:

```go
package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	smppsession "github.com/smpp-server/smpp-server/internal/gateway/smpp/session"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/smpp/protocol"
)

// DeliverDLRRequest mirrors the protobuf message for in-package use and testing.
// When proto-generated code is available, this will be replaced by smppv1.DeliverDLRRequest.
type DeliverDLRRequest struct {
	SystemId        string
	SourceAddr      string
	DestinationAddr string
	ReceiptText     string
}

// DeliverDLRResponse mirrors the protobuf message.
type DeliverDLRResponse struct {
	Delivered bool
	Error     string
}

// GRPCServer handles internal gRPC calls for DLR delivery
type GRPCServer struct {
	sessions   map[string]*smppsession.Session
	sessionsMu *sync.RWMutex
	logger     zerolog.Logger
}

// NewGRPCServer creates a new GRPCServer
func NewGRPCServer(
	sessions map[string]*smppsession.Session,
	sessionsMu *sync.RWMutex,
	logger zerolog.Logger,
) *GRPCServer {
	return &GRPCServer{
		sessions:   sessions,
		sessionsMu: sessionsMu,
		logger:     logger.With().Str("component", "smpp_grpc_server").Logger(),
	}
}

// DeliverDLR sends a deliver_sm PDU to the SMPP client identified by system_id
func (s *GRPCServer) DeliverDLR(ctx context.Context, req *DeliverDLRRequest) (*DeliverDLRResponse, error) {
	startTime := time.Now()
	defer func() {
		monitoring.SMPPProcessingDuration.WithLabelValues("deliver_dlr").Observe(time.Since(startTime).Seconds())
	}()

	// Find session by system_id
	session := s.findSessionBySystemID(req.SystemId)
	if session == nil {
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(req.SystemId, "session_not_found").Inc()
		return &DeliverDLRResponse{
			Delivered: false,
			Error:     fmt.Sprintf("session not found for system_id: %s", req.SystemId),
		}, nil
	}

	// Verify session can receive (receiver or transceiver)
	if !session.CanReceive() {
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(req.SystemId, "invalid_bind_type").Inc()
		return &DeliverDLRResponse{
			Delivered: false,
			Error:     fmt.Sprintf("session %s cannot receive (bind type: %s)", req.SystemId, session.BindType),
		}, nil
	}

	// Build deliver_sm PDU
	err := s.sendDeliverSM(session, req)
	if err != nil {
		monitoring.SMPPDLRDeliveryFailed.WithLabelValues(req.SystemId, "send_error").Inc()
		s.logger.Error().Err(err).
			Str("system_id", req.SystemId).
			Msg("ошибка отправки deliver_sm")
		return &DeliverDLRResponse{
			Delivered: false,
			Error:     err.Error(),
		}, nil
	}

	monitoring.SMPPDLRDelivered.WithLabelValues(req.SystemId).Inc()
	s.logger.Info().
		Str("system_id", req.SystemId).
		Msg("DLR deliver_sm отправлен клиенту")

	return &DeliverDLRResponse{Delivered: true}, nil
}

// findSessionBySystemID looks up a bound session by system_id
func (s *GRPCServer) findSessionBySystemID(systemID string) *smppsession.Session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for _, session := range s.sessions {
		if session.IsBound() && session.SystemID == systemID {
			return session
		}
	}
	return nil
}

// sendDeliverSM builds and sends a deliver_sm PDU with DLR receipt
func (s *GRPCServer) sendDeliverSM(session *smppsession.Session, req *DeliverDLRRequest) error {
	encoder := protocol.NewEncoder()

	deliverSM := &protocol.DeliverSMPDU{
		SourceAddr:      req.SourceAddr,
		DestinationAddr: req.DestinationAddr,
		ESMClass:        0x04, // Delivery receipt
		ShortMessage:    []byte(req.ReceiptText),
	}

	body, err := encoder.EncodeDeliverSM(deliverSM)
	if err != nil {
		return fmt.Errorf("ошибка кодирования deliver_sm: %w", err)
	}

	seqNum := session.NextSequenceNumber()
	pdu := &protocol.PDU{
		CommandLength:  uint32(protocol.PDUHeaderLength + len(body)),
		CommandID:      protocol.DeliverSM,
		CommandStatus:  protocol.ESME_ROK,
		SequenceNumber: seqNum,
		Body:           body,
	}

	data, err := encoder.EncodePDU(pdu)
	if err != nil {
		return fmt.Errorf("ошибка кодирования PDU: %w", err)
	}

	conn := session.GetConn()
	if conn == nil {
		return smppsession.ErrConnectionClosed
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("ошибка записи deliver_sm: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Check that session.CanReceive() exists; if not, add it**

Run: `cd c:/projects/sms && grep -n "CanReceive\|CanSend" internal/gateway/smpp/session/session.go`

If `CanReceive` doesn't exist, add to `internal/gateway/smpp/session/session.go`:

```go
// CanReceive returns true if the session can receive deliver_sm (receiver or transceiver)
func (s *Session) CanReceive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State == SessionStateBoundRX || s.State == SessionStateBoundTRX
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -run TestGRPCServer -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/smpp/server/grpc_server.go internal/gateway/smpp/server/grpc_server_test.go internal/gateway/smpp/session/session.go
git commit -m "feat(smpp): add internal gRPC server for DLR delivery to SMPP clients"
```

---

### Task 7: Integrate Redis store into smpp-gateway handler

**Files:**
- Modify: `internal/gateway/smpp/server/handler.go`
- Modify: `internal/gateway/smpp/server/server.go`

- [ ] **Step 1: Add RedisStore to Handler struct**

In `internal/gateway/smpp/server/handler.go`, add `redisStore` field to Handler:

```go
type Handler struct {
	session      *smppsession.Session
	decoder      *protocol.Decoder
	encoder      *protocol.Encoder
	validator    *protocol.Validator
	authAdapter  *AuthAdapter
	messageRepo  *storage.MessageRepository
	optOutRepo   *storage.OptOutRepository
	producer     *queue.Producer
	redisStore   *RedisStore
	logger       zerolog.Logger
}
```

Update `NewHandler` to accept `redisStore *RedisStore`:

```go
func NewHandler(
	session *smppsession.Session,
	authAdapter *AuthAdapter,
	messageRepo *storage.MessageRepository,
	optOutRepo *storage.OptOutRepository,
	producer *queue.Producer,
	redisStore *RedisStore,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		session:     session,
		decoder:     protocol.NewDecoder(nil),
		encoder:     protocol.NewEncoder(),
		validator:   protocol.NewValidator(),
		authAdapter: authAdapter,
		messageRepo: messageRepo,
		optOutRepo:  optOutRepo,
		producer:    producer,
		redisStore:  redisStore,
		logger:      logger,
	}
}
```

- [ ] **Step 2: Add Redis write to handleSubmitSM**

In `handleSubmitSM`, after the successful `h.producer.PublishOutgoing(ctx, kafkaMsg)` call and before the monitoring increment, add:

```go
	// Сохраняем маппинг message_id → session для DLR delivery
	if h.redisStore != nil {
		mapping := &MessageMapping{
			SystemID:   h.session.SystemID,
			SourceAddr: submit.SourceAddr,
			DestAddr:   submit.DestinationAddr,
			SubmitDate: time.Now(),
		}
		if err := h.redisStore.SaveMessageMapping(ctx, msgID.String(), mapping); err != nil {
			h.logger.Error().Err(err).Str("message_id", msgID.String()).Msg("ошибка сохранения message mapping в Redis")
			// Non-fatal: message still sent, DLR just won't be delivered
		}
	}
```

- [ ] **Step 3: Add Redis session binding to bind handlers**

In `handleBindTransceiver` (and similarly for `handleBindTransmitter`, `handleBindReceiver`), after `h.session.Bind(...)` succeeds and before sending response, add:

```go
	// Сохраняем session binding в Redis для DLR delivery
	if h.redisStore != nil {
		binding := &SessionBinding{GatewayAddr: "smpp-gateway:9095"}
		if err := h.redisStore.SaveSessionBinding(ctx, bind.SystemID, binding); err != nil {
			h.logger.Error().Err(err).Str("system_id", bind.SystemID).Msg("ошибка сохранения session binding в Redis")
		}
	}
```

Add `ctx := context.Background()` at the top of each bind handler where it's not already present.

- [ ] **Step 4: Add Redis session cleanup to handleUnbind**

In `handleUnbind`, after `h.session.Unbind()` succeeds:

```go
	// Удаляем session binding из Redis
	if h.redisStore != nil {
		if err := h.redisStore.DeleteSessionBinding(context.Background(), h.session.SystemID); err != nil {
			h.logger.Error().Err(err).Msg("ошибка удаления session binding из Redis")
		}
	}
```

- [ ] **Step 5: Add Redis TTL refresh to handleEnquireLink**

In `handleEnquireLink`, after `session.UpdateEnquireLinkSent()`:

```go
	// Обновляем TTL session binding в Redis
	if h.redisStore != nil && h.session.SystemID != "" {
		if err := h.redisStore.RefreshSessionTTL(context.Background(), h.session.SystemID); err != nil {
			h.logger.Error().Err(err).Msg("ошибка обновления TTL session binding")
		}
	}
```

- [ ] **Step 6: Update server.go handleConnection to pass redisStore**

In `server.go`, add `redisStore *RedisStore` field to Server struct. Update `handleConnection` to pass it:

```go
handler := NewHandler(session, authAdapter, s.messageRepo, s.optOutRepo, s.producer, s.redisStore, s.logger)
```

Add cleanup of session binding on disconnect in the defer block:

```go
defer func() {
	// Удаляем session binding из Redis при disconnect
	if s.redisStore != nil && session.SystemID != "" {
		if err := s.redisStore.DeleteSessionBinding(context.Background(), session.SystemID); err != nil {
			s.logger.Error().Err(err).Str("system_id", session.SystemID).Msg("ошибка удаления session binding при disconnect")
		}
	}

	s.sessionsMu.Lock()
	delete(s.sessions, session.ID)
	s.sessionsMu.Unlock()
	monitoring.SMPPConnectionsActive.WithLabelValues("total").Set(float64(len(s.sessions)))
	session.Close()
}()
```

- [ ] **Step 7: Update NewServer to accept redisStore**

In `server.go`, update `NewServer`:

```go
func NewServer(
	cfg *config.SMSPConfig,
	authClient authv1.AuthServiceClient,
	messageRepo *storage.MessageRepository,
	optOutRepo *storage.OptOutRepository,
	producer *queue.Producer,
	redisStore *RedisStore,
	logger zerolog.Logger,
) *Server {
```

And assign: `redisStore: redisStore,` in the struct literal.

- [ ] **Step 8: Verify compilation**

Run: `cd c:/projects/sms && go build ./internal/gateway/smpp/server/`
Expected: Compilation errors in main.go (expected — will fix in Task 10)

- [ ] **Step 9: Run existing handler tests**

Run: `cd c:/projects/sms && go test ./internal/gateway/smpp/server/ -v`

Fix any test compilation issues (NewHandler calls need updated to include `nil` for redisStore).

- [ ] **Step 10: Commit**

```bash
git add internal/gateway/smpp/server/handler.go internal/gateway/smpp/server/server.go
git commit -m "feat(smpp): integrate Redis store for DLR mappings in handler and server"
```

---

### Task 8: DLR Dispatcher (gRPC client)

**Files:**
- Create: `internal/services/dlr/dispatcher.go`
- Create: `internal/services/dlr/dispatcher_test.go`

- [ ] **Step 1: Write failing test for dispatcher**

Create `internal/services/dlr/dispatcher_test.go`:

```go
package dlr

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGRPCClient simulates the gRPC call to smpp-gateway
type mockGRPCClient struct {
	delivered    bool
	err          string
	callCount    int
	lastSystemID string
	lastReceipt  string
	failUntil    int // fail first N calls
}

func (m *mockGRPCClient) DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (bool, string, error) {
	m.callCount++
	m.lastSystemID = systemID
	m.lastReceipt = receipt
	if m.callCount <= m.failUntil {
		return false, "unavailable", context.DeadlineExceeded
	}
	return m.delivered, m.err, nil
}

func TestDispatcher_Dispatch_Success(t *testing.T) {
	mock := &mockGRPCClient{delivered: true}
	d := NewDispatcher(mock, zerolog.Nop())

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, "aggregator1", mock.lastSystemID)
	assert.Equal(t, "id:msg-1 stat:DELIVRD", mock.lastReceipt)
}

func TestDispatcher_Dispatch_SessionNotFound(t *testing.T) {
	mock := &mockGRPCClient{delivered: false, err: "session not found"}
	d := NewDispatcher(mock, zerolog.Nop())

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "unknown",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.False(t, result)
}

func TestDispatcher_Dispatch_RetryOnFailure(t *testing.T) {
	mock := &mockGRPCClient{delivered: true, failUntil: 2}
	d := NewDispatcher(mock, zerolog.Nop())
	d.maxRetries = 3
	d.retryBackoff = 10 * time.Millisecond

	result, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, 3, mock.callCount) // 2 failures + 1 success
}

func TestDispatcher_Dispatch_ExhaustedRetries(t *testing.T) {
	mock := &mockGRPCClient{delivered: false, failUntil: 10}
	d := NewDispatcher(mock, zerolog.Nop())
	d.maxRetries = 3
	d.retryBackoff = 10 * time.Millisecond

	_, err := d.Dispatch(context.Background(), DispatchRequest{
		SystemID:   "aggregator1",
		SourceAddr: "380501234567",
		DestAddr:   "MyBrand",
		Receipt:    "id:msg-1 stat:DELIVRD",
	})

	require.Error(t, err)
	assert.Equal(t, 3, mock.callCount)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -run TestDispatcher -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement dispatcher**

Create `internal/services/dlr/dispatcher.go`:

```go
package dlr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/monitoring"
)

// GRPCClient interface for calling smpp-gateway DeliverDLR
type GRPCClient interface {
	DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (delivered bool, errMsg string, err error)
}

// DispatchRequest contains DLR dispatch parameters
type DispatchRequest struct {
	SystemID   string
	SourceAddr string
	DestAddr   string
	Receipt    string
}

// Dispatcher sends DLR to smpp-gateway via gRPC
type Dispatcher struct {
	client       GRPCClient
	logger       zerolog.Logger
	maxRetries   int
	retryBackoff time.Duration
}

// NewDispatcher creates a new Dispatcher
func NewDispatcher(client GRPCClient, logger zerolog.Logger) *Dispatcher {
	return &Dispatcher{
		client:       client,
		logger:       logger.With().Str("component", "dlr_dispatcher").Logger(),
		maxRetries:   3,
		retryBackoff: 1 * time.Second,
	}
}

// Dispatch sends DLR to smpp-gateway with retry logic
func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (bool, error) {
	startTime := time.Now()
	defer func() {
		monitoring.DLRDispatchDuration.WithLabelValues().Observe(time.Since(startTime).Seconds())
	}()

	var lastErr error
	for attempt := 1; attempt <= d.maxRetries; attempt++ {
		delivered, errMsg, err := d.client.DeliverDLR(ctx, req.SystemID, req.SourceAddr, req.DestAddr, req.Receipt)
		if err != nil {
			lastErr = err
			d.logger.Warn().
				Err(err).
				Int("attempt", attempt).
				Str("system_id", req.SystemID).
				Msg("gRPC call failed, retrying")

			if attempt < d.maxRetries {
				backoff := d.retryBackoff * time.Duration(1<<(attempt-1))
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				case <-time.After(backoff):
				}
			}
			continue
		}

		if !delivered {
			d.logger.Warn().
				Str("system_id", req.SystemID).
				Str("error", errMsg).
				Msg("DLR not delivered")
			monitoring.DLREventsDispatched.WithLabelValues("", "not_delivered").Inc()
			return false, nil
		}

		monitoring.DLREventsDispatched.WithLabelValues("", "success").Inc()
		return true, nil
	}

	monitoring.DLREventsDispatched.WithLabelValues("", "exhausted_retries").Inc()
	return false, fmt.Errorf("все %d попыток исчерпаны: %w", d.maxRetries, lastErr)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -run TestDispatcher -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/dlr/dispatcher.go internal/services/dlr/dispatcher_test.go
git commit -m "feat(dlr): add gRPC dispatcher with retry logic"
```

---

### Task 9: DLR Kafka Consumer

**Files:**
- Create: `internal/services/dlr/consumer.go`
- Create: `internal/services/dlr/consumer_test.go`

- [ ] **Step 1: Write failing test for consumer processing logic**

Create `internal/services/dlr/consumer_test.go`:

```go
package dlr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRedisStore implements Redis lookups for testing
type mockRedisStore struct {
	messages map[string]*server.MessageMapping
	sessions map[string]*server.SessionBinding
}

func newMockRedisStore() *mockRedisStore {
	return &mockRedisStore{
		messages: make(map[string]*server.MessageMapping),
		sessions: make(map[string]*server.SessionBinding),
	}
}

func (m *mockRedisStore) GetMessageMapping(ctx context.Context, messageID string) (*server.MessageMapping, error) {
	v, ok := m.messages[messageID]
	if !ok {
		return nil, server.ErrRedisKeyNotFound
	}
	return v, nil
}

func (m *mockRedisStore) GetSessionBinding(ctx context.Context, systemID string) (*server.SessionBinding, error) {
	v, ok := m.sessions[systemID]
	if !ok {
		return nil, server.ErrRedisKeyNotFound
	}
	return v, nil
}

// collectingDispatcher captures dispatch calls
type collectingDispatcher struct {
	calls []DispatchRequest
}

func (c *collectingDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (bool, error) {
	c.calls = append(c.calls, req)
	return true, nil
}

func TestProcessStatusUpdate_Delivered(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	msgID := uuid.New()
	submitDate := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC)

	store.messages[msgID.String()] = &server.MessageMapping{
		SystemID:   "aggregator1",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: submitDate,
	}
	store.sessions["aggregator1"] = &server.SessionBinding{
		GatewayAddr: "smpp-gateway:9095",
	}

	update := &pipeline.StatusUpdate{
		MessageID: msgID,
		Status:    "delivered",
	}

	err := processor.ProcessStatusUpdate(context.Background(), update)
	require.NoError(t, err)

	require.Len(t, dispatcher.calls, 1)
	assert.Equal(t, "aggregator1", dispatcher.calls[0].SystemID)
	assert.Contains(t, dispatcher.calls[0].Receipt, "stat:DELIVRD")
	assert.Contains(t, dispatcher.calls[0].Receipt, "id:"+msgID.String())
	// source and dest are swapped for DLR: source=destination, dest=source
	assert.Equal(t, "380501234567", dispatcher.calls[0].SourceAddr)
	assert.Equal(t, "MyBrand", dispatcher.calls[0].DestAddr)
}

func TestProcessStatusUpdate_SkipsNonFinalStatus(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	for _, status := range []string{"sent", "queued", "routed"} {
		update := &pipeline.StatusUpdate{
			MessageID: uuid.New(),
			Status:    status,
		}
		err := processor.ProcessStatusUpdate(context.Background(), update)
		require.NoError(t, err)
	}

	assert.Empty(t, dispatcher.calls, "non-final statuses should not dispatch DLR")
}

func TestProcessStatusUpdate_MessageMappingMiss(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	update := &pipeline.StatusUpdate{
		MessageID: uuid.New(),
		Status:    "delivered",
	}

	err := processor.ProcessStatusUpdate(context.Background(), update)
	require.NoError(t, err) // Not an error — message wasn't from SMPP client
	assert.Empty(t, dispatcher.calls)
}

func TestProcessStatusUpdate_SessionMiss(t *testing.T) {
	store := newMockRedisStore()
	dispatcher := &collectingDispatcher{}
	processor := NewProcessor(store, dispatcher, zerolog.Nop())

	msgID := uuid.New()
	store.messages[msgID.String()] = &server.MessageMapping{
		SystemID:   "disconnected_client",
		SourceAddr: "MyBrand",
		DestAddr:   "380501234567",
		SubmitDate: time.Now(),
	}
	// No session binding — client disconnected

	update := &pipeline.StatusUpdate{
		MessageID: msgID,
		Status:    "delivered",
	}

	err := processor.ProcessStatusUpdate(context.Background(), update)
	require.NoError(t, err) // Not an error — client disconnected, DLR lost
	assert.Empty(t, dispatcher.calls)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -run TestProcess -v`
Expected: FAIL — Processor not defined

- [ ] **Step 3: Implement consumer/processor**

Create `internal/services/dlr/consumer.go`:

```go
package dlr

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
)

// finalStatuses are the only statuses that trigger DLR delivery
var finalStatuses = map[string]bool{
	"delivered": true,
	"failed":    true,
	"expired":   true,
	"rejected":  true,
}

// MessageMappingReader reads message mappings from Redis
type MessageMappingReader interface {
	GetMessageMapping(ctx context.Context, messageID string) (*server.MessageMapping, error)
	GetSessionBinding(ctx context.Context, systemID string) (*server.SessionBinding, error)
}

// DLRDispatcher dispatches DLR to smpp-gateway
type DLRDispatcher interface {
	Dispatch(ctx context.Context, req DispatchRequest) (bool, error)
}

// Processor processes StatusUpdate events and dispatches DLRs
type Processor struct {
	store      MessageMappingReader
	dispatcher DLRDispatcher
	logger     zerolog.Logger
}

// NewProcessor creates a new Processor
func NewProcessor(store MessageMappingReader, dispatcher DLRDispatcher, logger zerolog.Logger) *Processor {
	return &Processor{
		store:      store,
		dispatcher: dispatcher,
		logger:     logger.With().Str("component", "dlr_processor").Logger(),
	}
}

// ProcessStatusUpdate handles a single StatusUpdate from Kafka
func (p *Processor) ProcessStatusUpdate(ctx context.Context, update *pipeline.StatusUpdate) error {
	// Skip non-final statuses
	if !finalStatuses[update.Status] {
		return nil
	}

	monitoring.DLREventsConsumed.WithLabelValues(update.Status).Inc()

	// 1. Lookup message mapping
	msgID := update.MessageID.String()
	mapping, err := p.store.GetMessageMapping(ctx, msgID)
	if err != nil {
		if errors.Is(err, server.ErrRedisKeyNotFound) {
			// Message not from SMPP client — skip silently
			monitoring.DLRRedisLookupMiss.WithLabelValues("msg").Inc()
			return nil
		}
		return err
	}

	// 2. Lookup session binding
	_, err = p.store.GetSessionBinding(ctx, mapping.SystemID)
	if err != nil {
		if errors.Is(err, server.ErrRedisKeyNotFound) {
			// Client disconnected — DLR lost
			monitoring.DLRRedisLookupMiss.WithLabelValues("session").Inc()
			p.logger.Warn().
				Str("system_id", mapping.SystemID).
				Str("message_id", msgID).
				Msg("клиент отключён, DLR потерян")
			return nil
		}
		return err
	}

	// 3. Format SMSC receipt
	doneDate := time.Now()
	if update.DoneDate != nil {
		doneDate = *update.DoneDate
	}
	submitDate := mapping.SubmitDate
	if update.SubmitDate != nil {
		submitDate = *update.SubmitDate
	}
	errorCode := 0
	if update.ErrorCode != nil {
		errorCode = *update.ErrorCode
	}

	receipt := FormatReceipt(ReceiptParams{
		MessageID:  msgID,
		Status:     update.Status,
		SubmitDate: submitDate,
		DoneDate:   doneDate,
		ErrorCode:  errorCode,
		Text:       "",
	})

	// 4. Dispatch DLR — source/dest swapped for delivery receipt
	_, err = p.dispatcher.Dispatch(ctx, DispatchRequest{
		SystemID:   mapping.SystemID,
		SourceAddr: mapping.DestAddr,
		DestAddr:   mapping.SourceAddr,
		Receipt:    receipt,
	})

	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/dlr/consumer.go internal/services/dlr/consumer_test.go
git commit -m "feat(dlr): add Kafka consumer processor with Redis lookup and dispatch"
```

---

### Task 10: dlr-delivery service entry point

**Files:**
- Create: `cmd/dlr-delivery/main.go`

- [ ] **Step 1: Create main.go**

Create `cmd/dlr-delivery/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/gateway/smpp/server"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/queue"
	"github.com/smpp-server/smpp-server/internal/services/dlr"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	smppv1 "github.com/smpp-server/smpp-server/api/proto/smppv1"
)

// grpcGatewayClient wraps the generated gRPC client to implement dlr.GRPCClient
type grpcGatewayClient struct {
	client smppv1.SMPPGatewayClient
}

func (g *grpcGatewayClient) DeliverDLR(ctx context.Context, systemID, sourceAddr, destAddr, receipt string) (bool, string, error) {
	resp, err := g.client.DeliverDLR(ctx, &smppv1.DeliverDLRRequest{
		SystemId:        systemID,
		SourceAddr:      sourceAddr,
		DestinationAddr: destAddr,
		ReceiptText:     receipt,
	})
	if err != nil {
		return false, "", err
	}
	return resp.Delivered, resp.Error, nil
}

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("dlr-delivery")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Msg("запуск DLR Delivery Service")

	// Redis
	redisCache, err := cache.NewCache(&cache.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к Redis")
	}

	redisStore := server.NewRedisStore(redisCache, 24*time.Hour, 5*time.Minute)

	// gRPC client to smpp-gateway
	gatewayAddr := config.EnvOrDefault("SMPP_GATEWAY_GRPC_ADDR", "smpp-gateway:9095")
	grpcConn, err := grpc.NewClient(gatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Str("addr", gatewayAddr).Msg("ошибка подключения к smpp-gateway gRPC")
	}
	defer grpcConn.Close()

	gatewayClient := &grpcGatewayClient{client: smppv1.NewSMPPGatewayClient(grpcConn)}
	dispatcher := dlr.NewDispatcher(gatewayClient, logger)

	// Processor
	processor := dlr.NewProcessor(redisStore, dispatcher, logger)

	// Kafka consumer
	logger.Info().Msg("ожидание готовности Kafka брокеров")
	if err := queue.WaitForKafka(&cfg.Kafka, 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("Kafka брокеры недоступны")
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Version = sarama.V2_6_0_0

	consumerGroup, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, "dlr-delivery", saramaConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания Kafka consumer group")
	}
	defer consumerGroup.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &statusConsumerHandler{processor: processor, logger: logger}

	// Start consuming in background
	go func() {
		topic := cfg.Kafka.TopicStatus
		for {
			if err := consumerGroup.Consume(ctx, []string{topic}, handler); err != nil {
				logger.Error().Err(err).Msg("ошибка Kafka consumer")
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Error logging
	go func() {
		for err := range consumerGroup.Errors() {
			logger.Error().Err(err).Msg("Kafka consumer error")
		}
	}()

	logger.Info().Str("topic", cfg.Kafka.TopicStatus).Msg("DLR Delivery consumer запущен")

	// Health + metrics HTTP server
	healthChecker := monitoring.NewHealthChecker("dlr-delivery", cfg.Service.Version)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Monitoring.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("metrics server запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("ошибка metrics server")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал остановки")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	metricsServer.Shutdown(shutdownCtx)

	logger.Info().Msg("DLR Delivery Service остановлен")
}

// statusConsumerHandler implements sarama.ConsumerGroupHandler
type statusConsumerHandler struct {
	processor *dlr.Processor
	logger    zerolog.Logger
}

func (h *statusConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *statusConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *statusConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var update pipeline.StatusUpdate
		if err := json.Unmarshal(msg.Value, &update); err != nil {
			h.logger.Error().Err(err).Msg("ошибка десериализации StatusUpdate")
			session.MarkMessage(msg, "")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := h.processor.ProcessStatusUpdate(ctx, &update)
		cancel()

		if err != nil {
			h.logger.Error().Err(err).
				Str("message_id", update.MessageID.String()).
				Msg("ошибка обработки StatusUpdate")
			// Don't mark — will be reprocessed
			continue
		}

		session.MarkMessage(msg, "")
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd c:/projects/sms && go build ./cmd/dlr-delivery/`

Fix any import issues. If proto-generated code is not yet available, temporarily comment out the smppv1 import and grpcGatewayClient — the proto will be generated in Task 5.

- [ ] **Step 3: Commit**

```bash
git add cmd/dlr-delivery/main.go
git commit -m "feat(dlr): add dlr-delivery service entry point"
```

---

### Task 11: Update smpp-gateway main.go

**Files:**
- Modify: `cmd/smpp-gateway/main.go`

- [ ] **Step 1: Add Redis and gRPC server initialization**

Update `cmd/smpp-gateway/main.go` to:

1. Initialize Redis cache:
```go
	// Redis для DLR маппинга
	redisCache, err := cache.NewCache(&cache.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к Redis")
	}

	redisStore := smppserver.NewRedisStore(redisCache, 24*time.Hour, 5*time.Minute)
```

2. Pass redisStore to NewServer:
```go
	smppGateway := smppserver.NewServer(
		&cfg.SMSP,
		serviceClients.AuthClient,
		messageRepo,
		optOutRepo,
		producer,
		redisStore,
		logger,
	)
```

3. Start internal gRPC server after smppGateway.Start():
```go
	// Internal gRPC сервер для DLR delivery
	grpcInternalPort := config.EnvOrDefault("GRPC_INTERNAL_PORT", "9095")
	grpcListener, err := net.Listen("tcp", ":"+grpcInternalPort)
	if err != nil {
		logger.Fatal().Err(err).Str("port", grpcInternalPort).Msg("ошибка запуска internal gRPC listener")
	}

	grpcServer := grpc.NewServer()
	dlrGRPCServer := smppserver.NewGRPCServer(smppGateway.GetSessions(), smppGateway.GetSessionsMu(), logger)
	// Register with proto-generated code:
	// smppv1.RegisterSMPPGatewayServer(grpcServer, dlrGRPCServer)
	reflection.Register(grpcServer)

	go func() {
		logger.Info().Str("port", grpcInternalPort).Msg("internal gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Error().Err(err).Msg("ошибка internal gRPC сервера")
		}
	}()
```

4. Add accessor methods to Server:
```go
// GetSessions returns the sessions map (for gRPC server)
func (s *Server) GetSessions() map[string]*smppsession.Session {
	return s.sessions
}

// GetSessionsMu returns the sessions mutex (for gRPC server)
func (s *Server) GetSessionsMu() *sync.RWMutex {
	return &s.sessionsMu
}
```

5. Add graceful shutdown for gRPC:
```go
	grpcServer.GracefulStop()
```

6. Add imports for `cache`, `net`, `grpc`, `reflection`.

- [ ] **Step 2: Verify compilation**

Run: `cd c:/projects/sms && go build ./cmd/smpp-gateway/`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cmd/smpp-gateway/main.go internal/gateway/smpp/server/server.go
git commit -m "feat(smpp): integrate Redis and internal gRPC into smpp-gateway"
```

---

### Task 12: Docker and deployment config

**Files:**
- Create: `deployments/docker/dlr-delivery.Dockerfile`
- Modify: `deployments/docker-compose.yml`
- Modify: `deployments/configs/prometheus.yml`

- [ ] **Step 1: Create Dockerfile**

Create `deployments/docker/dlr-delivery.Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dlr-delivery ./cmd/dlr-delivery

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

COPY --from=builder /app/dlr-delivery .
COPY --from=builder /app/configs ./configs

EXPOSE 2112

CMD ["./dlr-delivery"]
```

- [ ] **Step 2: Add dlr-delivery to docker-compose.yml**

Add after the smpp-gateway service block:

```yaml
  dlr-delivery:
    build:
      context: ..
      dockerfile: deployments/docker/dlr-delivery.Dockerfile
    container_name: dlr-delivery
    ports:
      - "2113:2112"
    environment:
      - SERVICE_NAME=dlr-delivery
      - KAFKA_BROKERS=kafka:9092
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - SMPP_GATEWAY_GRPC_ADDR=smpp-gateway:9095
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=smpp
      - POSTGRES_PASSWORD=smpp_password
      - POSTGRES_DB=smpp_db
    networks:
      - smpp-network
    depends_on:
      smpp-gateway:
        condition: service_healthy
      kafka:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O- http://localhost:2112/health >/dev/null || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 3
```

Also add `REDIS_HOST=redis` and `REDIS_PORT=6379` to smpp-gateway environment if not already there, and expose port 9095:

```yaml
    ports:
      - "2775:2775"
      - "2110:2112"
      - "9095:9095"
```

- [ ] **Step 3: Add Prometheus scrape config**

Add to `deployments/configs/prometheus.yml`:

```yaml
  - job_name: 'dlr-delivery'
    static_configs:
      - targets: ['dlr-delivery:2112']
        labels:
          service: 'dlr-delivery'
```

- [ ] **Step 4: Verify docker-compose syntax**

Run: `cd c:/projects/sms/deployments && docker compose config --quiet 2>&1 | head -5`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add deployments/docker/dlr-delivery.Dockerfile deployments/docker-compose.yml deployments/configs/prometheus.yml
git commit -m "feat(deploy): add dlr-delivery Dockerfile, docker-compose and Prometheus config"
```

---

### Task 13: Integration smoke test

**Files:**
- No new files — manual verification

- [ ] **Step 1: Verify all packages compile**

Run: `cd c:/projects/sms && go build ./...`
Expected: All packages compile successfully

- [ ] **Step 2: Run all tests**

Run: `cd c:/projects/sms && go test ./internal/services/dlr/ ./internal/gateway/smpp/server/ -v`
Expected: All PASS

- [ ] **Step 3: Commit any remaining fixes**

If any compilation or test fixes were needed, commit them:

```bash
git add -A
git commit -m "fix: resolve compilation and test issues for DLR delivery"
```
