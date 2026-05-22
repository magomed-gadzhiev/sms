package status

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/pipeline"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// ---------------------------------------------------------------------------
// mockPublisher — test double for asyncPublisher (Bug #2)
// ---------------------------------------------------------------------------

type publishedMsg struct {
	topic   string
	key     string
	value   []byte
	headers []sarama.RecordHeader
}

type mockPublisher struct {
	mu        sync.Mutex
	published []publishedMsg
}

func (m *mockPublisher) PublishAsync(topic, key string, value []byte, headers []sarama.RecordHeader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, publishedMsg{topic: topic, key: key, value: value, headers: headers})
}

func (m *mockPublisher) Close() error { return nil }

func (m *mockPublisher) messages() []publishedMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]publishedMsg, len(m.published))
	copy(out, m.published)
	return out
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func testConfig() *config.Config {
	return &config.Config{
		Kafka: config.KafkaConfig{
			TopicSent: "sms.sent",
			TopicDLR:  "sms.dlr",
		},
	}
}

func newStageForTest() *Stage {
	cfg := testConfig()
	return &Stage{
		cfg:    cfg,
		logger: zerolog.Nop(),
	}
}

func makeSentConsumerMsg(t *testing.T, sent *pipeline.SentMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := sent.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Topic:     "sms.sent",
		Value:     data,
		Partition: 0,
		Offset:    1,
	}
}

func makeDLRConsumerMsg(t *testing.T, dlr *queue.DLRMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := dlr.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Topic:     "sms.dlr",
		Value:     data,
		Partition: 0,
		Offset:    2,
	}
}

// ---------------------------------------------------------------------------
// mapStatus
// ---------------------------------------------------------------------------

func TestMapStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"sent", "sent"},
		{"failed", "failed"},
		{"unknown_status", "unknown_status"},
		{"", ""},
		{"retry", "retry"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapStatus(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// mapDLRStat
// ---------------------------------------------------------------------------

func TestMapDLRStat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"DELIVRD", "delivered"},
		{"UNDELIV", "failed"},
		{"EXPIRED", "expired"},
		{"UNKNOWN", "unknown"},
		{"REJECTD", "unknown"},
		{"ACCEPTD", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapDLRStat(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// deserializeMessage — sms.sent topic
// ---------------------------------------------------------------------------

func TestDeserializeMessage_Sent_HappyPath(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	providerID := uuid.New()
	sent := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    providerID,
		SMPPMessageID: "SMPP-001",
		Status:        "sent",
		SentAt:        time.Now().Truncate(time.Millisecond),
		SegmentsCount: 2,
		ConnectionID:  "conn-1",
	}

	msg := makeSentConsumerMsg(t, sent)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, sent.MessageID, rec.MessageID)
	assert.Equal(t, "sent", rec.Status) // mapStatus("sent") => "sent"
	assert.Equal(t, "SMPP-001", rec.SMPPMessageID)
	require.NotNil(t, rec.ProviderID)
	assert.Equal(t, providerID, *rec.ProviderID)
	require.NotNil(t, rec.SubmittedAt)
	assert.Equal(t, 2, rec.SegmentCount)
	assert.False(t, rec.UpdatedAt.IsZero())
}

func TestDeserializeMessage_Sent_FailedStatus(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	sent := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Status:        "failed",
		SentAt:        time.Now(),
	}

	msg := makeSentConsumerMsg(t, sent)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, "failed", rec.Status)
}

func TestDeserializeMessage_Sent_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	msg := &sarama.ConsumerMessage{
		Topic: "sms.sent",
		Value: []byte("not json"),
	}

	_, err := s.deserializeMessage(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SentMessage")
}

// ---------------------------------------------------------------------------
// deserializeMessage — sms.dlr topic
// ---------------------------------------------------------------------------

func TestDeserializeMessage_DLR_HappyPath_DELIVRD(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	providerID := uuid.New()
	// Use Round(0) to strip monotonic clock reading which is lost through JSON serialization.
	submitDate := time.Now().UTC().Add(-time.Minute).Round(0)
	doneDate := time.Now().UTC().Round(0)

	dlr := &queue.DLRMessage{
		MessageID:     uuid.New(),
		SMPPMessageID: "SMPP-DLR-001",
		ProviderID:    &providerID,
		Stat:          "DELIVRD",
		SubmitDate:    &submitDate,
		DoneDate:      &doneDate,
		Source:        "Src",
		Destination:   "+79001234567",
		CreatedAt:     time.Now().UTC(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, dlr.MessageID, rec.MessageID)
	assert.Equal(t, "delivered", rec.Status) // mapDLRStat("DELIVRD") => "delivered"
	assert.Equal(t, "SMPP-DLR-001", rec.SMPPMessageID)
	require.NotNil(t, rec.ProviderID)
	assert.Equal(t, providerID, *rec.ProviderID)
	require.NotNil(t, rec.SubmittedAt)
	// UpdatedAt should use DoneDate
	assert.Equal(t, doneDate, rec.UpdatedAt)
}

func TestDeserializeMessage_DLR_UNDELIV(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	dlr := &queue.DLRMessage{
		MessageID: uuid.New(),
		Stat:      "UNDELIV",
		CreatedAt: time.Now(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, "failed", rec.Status)
}

func TestDeserializeMessage_DLR_EXPIRED(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	dlr := &queue.DLRMessage{
		MessageID: uuid.New(),
		Stat:      "EXPIRED",
		CreatedAt: time.Now(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, "expired", rec.Status)
}

func TestDeserializeMessage_DLR_NilDoneDate_UsesNow(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	dlr := &queue.DLRMessage{
		MessageID: uuid.New(),
		Stat:      "DELIVRD",
		DoneDate:  nil, // nil -> should use time.Now()
		CreatedAt: time.Now(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	// UpdatedAt should be approximately now
	assert.WithinDuration(t, time.Now(), rec.UpdatedAt, 5*time.Second)
}

func TestDeserializeMessage_DLR_NilProviderID(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	dlr := &queue.DLRMessage{
		MessageID:  uuid.New(),
		Stat:       "DELIVRD",
		ProviderID: nil,
		CreatedAt:  time.Now(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Nil(t, rec.ProviderID)
}

func TestDeserializeMessage_DLR_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	msg := &sarama.ConsumerMessage{
		Topic: "sms.dlr",
		Value: []byte("{broken"),
	}

	_, err := s.deserializeMessage(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DLRMessage")
}

// ---------------------------------------------------------------------------
// deserializeMessage — unknown topic
// ---------------------------------------------------------------------------

func TestDeserializeMessage_UnknownTopic(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	msg := &sarama.ConsumerMessage{
		Topic: "sms.unknown",
		Value: []byte(`{}`),
	}

	_, err := s.deserializeMessage(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sms.unknown")
}

// ---------------------------------------------------------------------------
// statusRecord fields
// ---------------------------------------------------------------------------

func TestStatusRecord_SentMessageFields(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	providerID := uuid.New()
	sentAt := time.Now().UTC().Truncate(time.Millisecond)

	sent := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    providerID,
		SMPPMessageID: "SMPP-123",
		Status:        "sent",
		SentAt:        sentAt,
		SegmentsCount: 3,
		ConnectionID:  "conn-x",
	}

	msg := makeSentConsumerMsg(t, sent)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, sent.MessageID, rec.MessageID)
	assert.Equal(t, "sent", rec.Status)
	assert.Equal(t, "SMPP-123", rec.SMPPMessageID)
	require.NotNil(t, rec.ProviderID)
	assert.Equal(t, providerID, *rec.ProviderID)
	require.NotNil(t, rec.SubmittedAt)
	assert.Equal(t, sentAt, *rec.SubmittedAt)
	assert.Equal(t, 3, rec.SegmentCount)
}

func TestStatusRecord_DLRFields(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	providerID := uuid.New()
	submitDate := time.Now().UTC().Add(-time.Minute).Round(0)
	doneDate := time.Now().UTC().Round(0)

	dlr := &queue.DLRMessage{
		MessageID:     uuid.New(),
		SMPPMessageID: "SMPP-DLR-777",
		ProviderID:    &providerID,
		Stat:          "DELIVRD",
		SubmitDate:    &submitDate,
		DoneDate:      &doneDate,
		CreatedAt:     time.Now().UTC(),
	}

	msg := makeDLRConsumerMsg(t, dlr)
	rec, err := s.deserializeMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, dlr.MessageID, rec.MessageID)
	assert.Equal(t, "delivered", rec.Status)
	assert.Equal(t, "SMPP-DLR-777", rec.SMPPMessageID)
	require.NotNil(t, rec.ProviderID)
	assert.Equal(t, providerID, *rec.ProviderID)
	require.NotNil(t, rec.SubmittedAt)
	assert.Equal(t, doneDate, rec.UpdatedAt)
	assert.Equal(t, 0, rec.SegmentCount) // DLR has 0 segments
}

// ---------------------------------------------------------------------------
// failedBuffer concurrency safety
// ---------------------------------------------------------------------------

func TestFailedBuffer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	s.failedBuffer = nil

	const goroutines = 10
	const recordsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				rec := &statusRecord{
					MessageID: uuid.New(),
					Status:    "sent",
					UpdatedAt: time.Now(),
				}
				s.failedMu.Lock()
				s.failedBuffer = append(s.failedBuffer, rec)
				s.failedMu.Unlock()
			}
		}()
	}

	wg.Wait()

	s.failedMu.Lock()
	totalRecords := len(s.failedBuffer)
	s.failedMu.Unlock()

	assert.Equal(t, goroutines*recordsPerGoroutine, totalRecords)
}

func TestFailedBuffer_DrainAndRefill(t *testing.T) {
	t.Parallel()

	s := newStageForTest()

	// Add some records
	for i := 0; i < 5; i++ {
		s.failedBuffer = append(s.failedBuffer, &statusRecord{
			MessageID: uuid.New(),
			Status:    "sent",
			UpdatedAt: time.Now(),
		})
	}

	// Drain (like retryLoop does)
	s.failedMu.Lock()
	batch := s.failedBuffer
	s.failedBuffer = nil
	s.failedMu.Unlock()

	assert.Len(t, batch, 5)

	// Simulate failed retry: return to buffer
	s.failedMu.Lock()
	s.failedBuffer = append(batch, s.failedBuffer...)
	s.failedMu.Unlock()

	s.failedMu.Lock()
	assert.Len(t, s.failedBuffer, 5)
	s.failedMu.Unlock()
}

func TestFailedBuffer_EmptyDrain(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	s.failedBuffer = nil

	s.failedMu.Lock()
	assert.Len(t, s.failedBuffer, 0)
	s.failedMu.Unlock()
}

// ---------------------------------------------------------------------------
// DLR stat edge cases
// ---------------------------------------------------------------------------

func TestMapDLRStat_AllKnownStats(t *testing.T) {
	t.Parallel()

	// SMPP 3.4 standard DLR stat values
	statsMap := map[string]string{
		"DELIVRD": "delivered",
		"UNDELIV": "failed",
		"EXPIRED": "expired",
	}

	for stat, expected := range statsMap {
		result := mapDLRStat(stat)
		assert.Equal(t, expected, result, "stat=%s", stat)
	}
}

func TestMapDLRStat_UnknownReturnsUnknown(t *testing.T) {
	t.Parallel()

	unknownStats := []string{"REJECTD", "ACCEPTD", "DELETED", "ENROUTE", "SKIPPED", ""}
	for _, stat := range unknownStats {
		result := mapDLRStat(stat)
		assert.Equal(t, "unknown", result, "stat=%s should return 'unknown'", stat)
	}
}

// ---------------------------------------------------------------------------
// mapStatus edge cases
// ---------------------------------------------------------------------------

func TestMapStatus_PassthroughForCustomStatuses(t *testing.T) {
	t.Parallel()

	customStatuses := []string{"retry", "queued", "pending", "delivered"}
	for _, status := range customStatuses {
		result := mapStatus(status)
		assert.Equal(t, status, result,
			"mapStatus should pass through status=%s as-is (no mapping for custom statuses)", status)
	}
}

// ---------------------------------------------------------------------------
// publishStatusUpdates — Bug #2: status stage must publish to sms.status
// so campaign-service can update sent/delivered/failed counters.
// ---------------------------------------------------------------------------

func newStageWithMockPublisher(pub *mockPublisher) *Stage {
	cfg := &config.Config{
		Kafka: config.KafkaConfig{
			TopicSent:   "sms.sent",
			TopicDLR:    "sms.dlr",
			TopicStatus: "sms.status",
		},
	}
	return &Stage{
		cfg:           cfg,
		logger:        zerolog.Nop(),
		asyncProducer: pub,
	}
}

func TestPublishStatusUpdates_PublishesToStatusTopic(t *testing.T) {
	// Regression: before the fix nothing was published to sms.status,
	// so campaign-service never received events and counters stayed at 0.
	t.Parallel()

	pub := &mockPublisher{}
	s := newStageWithMockPublisher(pub)

	providerID := uuid.New()
	records := []*statusRecord{
		{MessageID: uuid.New(), Status: "sent", SMPPMessageID: "S1", ProviderID: &providerID, UpdatedAt: time.Now()},
		{MessageID: uuid.New(), Status: "delivered", SMPPMessageID: "S2", UpdatedAt: time.Now()},
		{MessageID: uuid.New(), Status: "failed", SMPPMessageID: "S3", UpdatedAt: time.Now()},
	}

	s.publishStatusUpdates(records)

	msgs := pub.messages()
	require.Len(t, msgs, 3, "one message per record must be published")

	for i, msg := range msgs {
		assert.Equal(t, "sms.status", msg.topic, "must publish to sms.status topic")
		assert.Equal(t, records[i].MessageID.String(), msg.key, "partition key must be message_id")

		var update pipeline.StatusUpdate
		require.NoError(t, json.Unmarshal(msg.value, &update))
		assert.Equal(t, records[i].MessageID, update.MessageID)
		assert.Equal(t, records[i].Status, update.Status)
		assert.Equal(t, records[i].SMPPMessageID, update.SMPPMessageID)
	}
}

func TestPublishStatusUpdates_ProviderIDPropagated(t *testing.T) {
	t.Parallel()

	pub := &mockPublisher{}
	s := newStageWithMockPublisher(pub)

	providerID := uuid.New()
	rec := &statusRecord{MessageID: uuid.New(), Status: "sent", ProviderID: &providerID, UpdatedAt: time.Now()}
	s.publishStatusUpdates([]*statusRecord{rec})

	msgs := pub.messages()
	require.Len(t, msgs, 1)

	var update pipeline.StatusUpdate
	require.NoError(t, json.Unmarshal(msgs[0].value, &update))
	require.NotNil(t, update.ProviderID)
	assert.Equal(t, providerID, *update.ProviderID)
}

func TestPublishStatusUpdates_EmptyRecords(t *testing.T) {
	t.Parallel()

	pub := &mockPublisher{}
	s := newStageWithMockPublisher(pub)
	s.publishStatusUpdates([]*statusRecord{})

	assert.Empty(t, pub.messages(), "no messages must be published for empty input")
}

func TestPublishStatusUpdates_MessageIDHeader(t *testing.T) {
	t.Parallel()

	pub := &mockPublisher{}
	s := newStageWithMockPublisher(pub)

	msgID := uuid.New()
	s.publishStatusUpdates([]*statusRecord{{MessageID: msgID, Status: "sent", UpdatedAt: time.Now()}})

	msgs := pub.messages()
	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].headers, 1)
	assert.Equal(t, "message_id", string(msgs[0].headers[0].Key))
	assert.Equal(t, msgID.String(), string(msgs[0].headers[0].Value))
}
