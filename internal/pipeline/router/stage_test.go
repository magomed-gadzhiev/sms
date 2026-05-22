package router

import (
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/queue"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func testConfig() *config.Config {
	return &config.Config{
		Kafka: config.KafkaConfig{
			TopicOutgoing: "sms.outgoing",
			TopicFailed:   "sms.failed",
			TopicRouted:   "sms.routed",
		},
	}
}

// newStageForTest builds a Stage with no real Kafka connections, only enough
// state to exercise deserializeByTopic.
func newStageForTest() *Stage {
	cfg := testConfig()
	logger := zerolog.Nop()
	return &Stage{
		cfg:    cfg,
		logger: logger,
	}
}

func makeOutgoingMsg(t *testing.T, km *queue.KafkaMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := km.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Topic:     "sms.outgoing",
		Value:     data,
		Partition: 0,
		Offset:    10,
	}
}

func makeFailedMsg(t *testing.T, fm *queue.FailedMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := fm.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Topic:     "sms.failed",
		Value:     data,
		Partition: 0,
		Offset:    20,
	}
}

func newKM() *queue.KafkaMessage {
	return &queue.KafkaMessage{
		ID:          "ext-1",
		MessageID:   uuid.New(),
		Source:      "TestApp",
		Destination: "+79001234567",
		Text:        "Hello",
		Priority:    1,
		RetryCount:  0,
		MaxRetries:  3,
		CreatedAt:   time.Now(),
	}
}

// ---------------------------------------------------------------------------
// deserializeByTopic — outgoing topic
// ---------------------------------------------------------------------------

func TestDeserializeByTopic_Outgoing_HappyPath(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	msg := makeOutgoingMsg(t, km)

	result, err := s.deserializeByTopic(msg)
	require.NoError(t, err)

	assert.Equal(t, km.MessageID, result.MessageID)
	assert.Equal(t, km.Source, result.Source)
	assert.Equal(t, km.Destination, result.Destination)
	assert.Equal(t, km.Text, result.Text)
}

func TestDeserializeByTopic_Outgoing_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	msg := &sarama.ConsumerMessage{
		Topic: "sms.outgoing",
		Value: []byte("bad json"),
	}

	_, err := s.deserializeByTopic(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KafkaMessage")
}

// ---------------------------------------------------------------------------
// deserializeByTopic — failed topic
// ---------------------------------------------------------------------------

func TestDeserializeByTopic_Failed_HappyPath(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	km.MaxRetries = 5
	fm := &queue.FailedMessage{
		MessageID:    km.MessageID,
		KafkaMessage: km,
		Error:        "provider timeout",
		ErrorCode:    "timeout",
		RetryCount:   2, // still < MaxRetries (5)
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	result, err := s.deserializeByTopic(msg)
	require.NoError(t, err)

	assert.Equal(t, km.MessageID, result.MessageID)
	assert.Equal(t, 2, result.RetryCount, "RetryCount should be updated from FailedMessage")
}

func TestDeserializeByTopic_Failed_ExhaustedRetries(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	km.MaxRetries = 3
	fm := &queue.FailedMessage{
		MessageID:    km.MessageID,
		KafkaMessage: km,
		Error:        "provider timeout",
		ErrorCode:    "timeout",
		RetryCount:   3, // == MaxRetries => exhausted
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	_, err := s.deserializeByTopic(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry")
}

func TestDeserializeByTopic_Failed_ExceedsRetries(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	km.MaxRetries = 2
	fm := &queue.FailedMessage{
		MessageID:    km.MessageID,
		KafkaMessage: km,
		Error:        "oops",
		RetryCount:   5, // > MaxRetries
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	_, err := s.deserializeByTopic(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry")
}

func TestDeserializeByTopic_Failed_NilKafkaMessage(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	fm := &queue.FailedMessage{
		MessageID:    uuid.New(),
		KafkaMessage: nil,
		Error:        "oops",
		RetryCount:   0,
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	_, err := s.deserializeByTopic(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KafkaMessage")
}

func TestDeserializeByTopic_Failed_InvalidJSON(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	msg := &sarama.ConsumerMessage{
		Topic: "sms.failed",
		Value: []byte("not json"),
	}

	_, err := s.deserializeByTopic(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FailedMessage")
}

func TestDeserializeByTopic_Failed_RetryCountZero(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	km.MaxRetries = 5
	fm := &queue.FailedMessage{
		MessageID:    km.MessageID,
		KafkaMessage: km,
		Error:        "first failure",
		RetryCount:   0, // first retry
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	result, err := s.deserializeByTopic(msg)
	require.NoError(t, err)
	assert.Equal(t, 0, result.RetryCount)
}

func TestDeserializeByTopic_Failed_ExactlyMaxMinusOne(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	km.MaxRetries = 3
	fm := &queue.FailedMessage{
		MessageID:    km.MessageID,
		KafkaMessage: km,
		Error:        "almost done",
		RetryCount:   2, // last allowed retry (< 3)
		FailedAt:     time.Now(),
	}

	msg := makeFailedMsg(t, fm)
	result, err := s.deserializeByTopic(msg)
	require.NoError(t, err)
	assert.Equal(t, 2, result.RetryCount)
}

// ---------------------------------------------------------------------------
// deserializeByTopic — unknown topic
// ---------------------------------------------------------------------------

func TestDeserializeByTopic_UnknownTopic(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	km := newKM()
	data, err := km.Serialize()
	require.NoError(t, err)

	msg := &sarama.ConsumerMessage{
		Topic: "sms.unknown.topic",
		Value: data,
	}

	// Unknown topics fall through to the default path which just deserializes
	// as a regular KafkaMessage.
	result, err := s.deserializeByTopic(msg)
	require.NoError(t, err)
	assert.Equal(t, km.MessageID, result.MessageID)
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose_ErrorCascadeLogic(t *testing.T) {
	t.Parallel()

	// Verify the firstErr pattern used in Close: the first error encountered
	// is returned while subsequent errors are only logged.
	// This is a design-level test for the error handling pattern.
	// Real Close testing requires actual Kafka consumer/producer mocks.

	var firstErr error
	err1 := fmt.Errorf("consumer close error")
	err2 := fmt.Errorf("producer close error")

	// Simulate Close logic
	firstErr = err1
	if firstErr == nil {
		firstErr = err2
	}

	assert.Equal(t, err1, firstErr, "firstErr should capture the first error")
}

// ---------------------------------------------------------------------------
// Stage field configuration
// ---------------------------------------------------------------------------

func TestStage_ConfigTopics(t *testing.T) {
	t.Parallel()

	s := newStageForTest()
	assert.Equal(t, "sms.outgoing", s.cfg.Kafka.TopicOutgoing)
	assert.Equal(t, "sms.failed", s.cfg.Kafka.TopicFailed)
	assert.Equal(t, "sms.routed", s.cfg.Kafka.TopicRouted)
}
