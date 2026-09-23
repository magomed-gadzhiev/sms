package sender

import (
	"context"
	"errors"
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
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/smsc"
)

// ---------------------------------------------------------------------------
// buildDLRMessage — регрессия на F-D1/F-D2 (docs/.../2026-04-21-fix-dlr-webhook-delivery.md)
// ---------------------------------------------------------------------------

// stubDLRLookup реализует dlrMessageLookup для юнит-теста без реального DB.
type stubDLRLookup struct {
	msg *shared.Message
	err error
}

func (s *stubDLRLookup) GetBySMPPMessageID(_ context.Context, _ string) (*shared.Message, error) {
	return s.msg, s.err
}

func TestBuildDLRMessage_PopulatesMessageIDAndClientID(t *testing.T) {
	t.Parallel()

	msgID := uuid.New()
	clientID := uuid.New()
	providerID := uuid.New()
	submittedAt := time.Now().Add(-5 * time.Minute)
	now := time.Now()

	repo := &stubDLRLookup{
		msg: &shared.Message{
			ID:          msgID,
			ClientID:    &clientID,
			SubmittedAt: &submittedAt,
		},
	}
	data := &smsc.DeliverSMData{
		SMPPMessageID: "provider-msg-xyz",
		ProviderID:    providerID,
		Stat:          "DELIVRD",
		Source:        "Sender",
		Destination:   "+79001234567",
		Text:          "id:provider-msg-xyz stat:DELIVRD",
	}

	dlrMsg, ok := buildDLRMessage(context.Background(), repo, data, now)

	require.True(t, ok, "должны успешно построить DLR когда message найден")
	require.NotNil(t, dlrMsg)
	assert.Equal(t, msgID, dlrMsg.MessageID, "MessageID должен быть populated из lookup")
	require.NotNil(t, dlrMsg.ClientID)
	assert.Equal(t, clientID, *dlrMsg.ClientID, "ClientID должен скопироваться из найденного message")
	assert.Equal(t, "provider-msg-xyz", dlrMsg.SMPPMessageID)
	assert.Equal(t, &providerID, dlrMsg.ProviderID)
	assert.Equal(t, "DELIVRD", dlrMsg.Stat)
	require.NotNil(t, dlrMsg.SubmitDate)
	assert.Equal(t, submittedAt, *dlrMsg.SubmitDate)
	require.NotNil(t, dlrMsg.DoneDate)
	assert.Equal(t, now, *dlrMsg.DoneDate)
}

func TestBuildDLRMessage_SkipsWhenMessageNotFound(t *testing.T) {
	t.Parallel()

	repo := &stubDLRLookup{msg: nil, err: nil}
	data := &smsc.DeliverSMData{
		SMPPMessageID: "unknown-smpp-id",
		ProviderID:    uuid.New(),
		Stat:          "DELIVRD",
	}

	dlrMsg, ok := buildDLRMessage(context.Background(), repo, data, time.Now())

	assert.False(t, ok, "должны skip publish когда сообщение не найдено")
	assert.Nil(t, dlrMsg)
}

func TestBuildDLRMessage_SkipsOnLookupError(t *testing.T) {
	t.Parallel()

	repo := &stubDLRLookup{msg: nil, err: errors.New("db connection lost")}
	data := &smsc.DeliverSMData{
		SMPPMessageID: "some-smpp-id",
		ProviderID:    uuid.New(),
		Stat:          "UNDELIV",
	}

	dlrMsg, ok := buildDLRMessage(context.Background(), repo, data, time.Now())

	assert.False(t, ok, "должны skip publish на lookup error")
	assert.Nil(t, dlrMsg)
}

func TestBuildDLRMessage_NilSubmittedAt(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	repo := &stubDLRLookup{
		msg: &shared.Message{
			ID:          uuid.New(),
			ClientID:    &clientID,
			SubmittedAt: nil, // message ещё не submitted / поле пустое
		},
	}
	data := &smsc.DeliverSMData{
		SMPPMessageID: "some-id",
		ProviderID:    uuid.New(),
		Stat:          "ENROUTE",
	}

	dlrMsg, ok := buildDLRMessage(context.Background(), repo, data, time.Now())

	require.True(t, ok)
	require.NotNil(t, dlrMsg)
	assert.Nil(t, dlrMsg.SubmitDate, "SubmitDate должен остаться nil если message.SubmittedAt nil")
}

// ---------------------------------------------------------------------------
// routedToSharedMessage
// ---------------------------------------------------------------------------

func TestRoutedToSharedMessage_AllFields(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	providerID := uuid.New()
	fallbackID := uuid.New()
	routeID := uuid.New()
	createdAt := time.Now().Add(-time.Hour)

	rm := &pipeline.RoutedMessage{
		SchemaVersion:      1,
		MessageID:          uuid.New(),
		Source:             "TestSender",
		Destination:        "+79001234567",
		Text:               "Hello from sender",
		ClientID:           &clientID,
		ProviderID:         providerID,
		FallbackProviderID: &fallbackID,
		RouteID:            &routeID,
		Priority:           3,
		RetryCount:         1,
		MaxRetries:         5,
		RoutedAt:           time.Now(),
		CreatedAt:          createdAt,
		Metadata:           map[string]interface{}{"key": "val"},
	}

	msg := routedToSharedMessage(rm)

	assert.Equal(t, rm.MessageID, msg.ID)
	assert.Equal(t, rm.Source, msg.Source)
	assert.Equal(t, rm.Destination, msg.Destination)
	assert.Equal(t, rm.Text, msg.Text)
	assert.Equal(t, rm.ClientID, msg.ClientID)
	assert.Equal(t, &rm.ProviderID, msg.ProviderID)
	assert.Equal(t, rm.RouteID, msg.RouteID)
	assert.Equal(t, rm.Priority, msg.PriorityFlag)
	assert.Equal(t, rm.RetryCount, msg.RetryCount)
	assert.Equal(t, rm.MaxRetries, msg.MaxRetries)
	assert.Equal(t, rm.CreatedAt, msg.CreatedAt)
}

func TestRoutedToSharedMessage_NilOptionalFields(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Source:        "Src",
		Destination:   "+79001234567",
		Text:          "Hi",
		ProviderID:    uuid.New(),
		// ClientID, FallbackProviderID, RouteID are nil
	}

	msg := routedToSharedMessage(rm)

	assert.Nil(t, msg.ClientID)
	assert.Nil(t, msg.RouteID)
	// ProviderID is always set from rm.ProviderID
	require.NotNil(t, msg.ProviderID)
	assert.Equal(t, rm.ProviderID, *msg.ProviderID)
}

func TestRoutedToSharedMessage_ZeroValues(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		// All other fields are zero values
	}

	msg := routedToSharedMessage(rm)

	assert.Empty(t, msg.Source)
	assert.Empty(t, msg.Destination)
	assert.Empty(t, msg.Text)
	assert.Equal(t, 0, msg.PriorityFlag)
	assert.Equal(t, 0, msg.RetryCount)
	assert.Equal(t, 0, msg.MaxRetries)
	assert.True(t, msg.CreatedAt.IsZero())
}

func TestRoutedToSharedMessage_ProviderIDIsPointer(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		ProviderID: providerID,
	}

	msg := routedToSharedMessage(rm)

	// shared.Message has ProviderID as *uuid.UUID
	require.NotNil(t, msg.ProviderID)
	assert.Equal(t, providerID, *msg.ProviderID)
}

// ---------------------------------------------------------------------------
// RoutedMessage deserialization for processMessage
// ---------------------------------------------------------------------------

func TestRoutedMessage_RoundTrip(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	routeID := uuid.New()
	fallback := uuid.New()

	rm := &pipeline.RoutedMessage{
		SchemaVersion:      1,
		MessageID:          uuid.New(),
		Source:             "App",
		Destination:        "+79009990000",
		Text:               "Test message",
		ClientID:           &clientID,
		ProviderID:         uuid.New(),
		FallbackProviderID: &fallback,
		RouteID:            &routeID,
		Priority:           2,
		RetryCount:         0,
		MaxRetries:         3,
		RoutedAt:           time.Now().Truncate(time.Millisecond),
		CreatedAt:          time.Now().Add(-time.Minute).Truncate(time.Millisecond),
		Metadata:           map[string]interface{}{"campaign": "test"},
	}

	data, err := rm.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeRoutedMessage(data)
	require.NoError(t, err)

	// Verify the deserialized message matches the original
	assert.Equal(t, rm.SchemaVersion, decoded.SchemaVersion)
	assert.Equal(t, rm.MessageID, decoded.MessageID)
	assert.Equal(t, rm.Source, decoded.Source)
	assert.Equal(t, rm.Destination, decoded.Destination)
	assert.Equal(t, rm.Text, decoded.Text)
	assert.Equal(t, rm.ClientID, decoded.ClientID)
	assert.Equal(t, rm.ProviderID, decoded.ProviderID)
	assert.Equal(t, rm.FallbackProviderID, decoded.FallbackProviderID)
	assert.Equal(t, rm.RouteID, decoded.RouteID)
	assert.Equal(t, rm.Priority, decoded.Priority)
	assert.Equal(t, rm.RetryCount, decoded.RetryCount)
	assert.Equal(t, rm.MaxRetries, decoded.MaxRetries)

	// Verify routedToSharedMessage produces correct output
	msg := routedToSharedMessage(decoded)
	assert.Equal(t, decoded.MessageID, msg.ID)
	assert.Equal(t, decoded.ProviderID, *msg.ProviderID)
}

func TestRoutedMessage_DeserializeInvalid(t *testing.T) {
	t.Parallel()

	_, err := pipeline.DeserializeRoutedMessage([]byte("not json"))
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// SentMessage construction pattern (used in processMessage)
// ---------------------------------------------------------------------------

func TestSentMessage_SuccessCase(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    providerID,
		SMPPMessageID: "SMPP-12345",
		Status:        "sent",
		SentAt:        time.Now(),
		SegmentsCount: 1,
		ConnectionID:  "conn-1",
	}

	data, err := sentMsg.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeSentMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "sent", decoded.Status)
	assert.Equal(t, "SMPP-12345", decoded.SMPPMessageID)
	assert.Nil(t, decoded.ErrorMessage)
	assert.Nil(t, decoded.ErrorCode)
}

func TestSentMessage_FailureCase(t *testing.T) {
	t.Parallel()

	errMsg := "connection refused"
	sentMsg := &pipeline.SentMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		ProviderID:    uuid.New(),
		Status:        "failed",
		ErrorMessage:  &errMsg,
		SentAt:        time.Now(),
		SegmentsCount: 1,
		ConnectionID:  "conn-2",
	}

	data, err := sentMsg.Serialize()
	require.NoError(t, err)

	decoded, err := pipeline.DeserializeSentMessage(data)
	require.NoError(t, err)

	assert.Equal(t, "failed", decoded.Status)
	require.NotNil(t, decoded.ErrorMessage)
	assert.Equal(t, errMsg, *decoded.ErrorMessage)
	assert.Empty(t, decoded.SMPPMessageID)
}

// ---------------------------------------------------------------------------
// FailedMessage construction (used in retry path)
// ---------------------------------------------------------------------------

func TestFailedMessage_ForRetry(t *testing.T) {
	t.Parallel()

	msgID := uuid.New()
	clientID := uuid.New()

	rm := &pipeline.RoutedMessage{
		MessageID:   msgID,
		Source:      "App",
		Destination: "+79001234567",
		Text:        "Retry me",
		ClientID:    &clientID,
		ProviderID:  uuid.New(),
		Priority:    1,
		RetryCount:  1,
		MaxRetries:  3,
		CreatedAt:   time.Now(),
		Metadata:    map[string]interface{}{"key": "val"},
	}

	// Simulate what processMessage does when both providers fail
	failedMsg := &queue.FailedMessage{
		MessageID:  rm.MessageID,
		Error:      "send_failed",
		ErrorCode:  "send_failed",
		RetryCount: rm.RetryCount + 1,
		FailedAt:   time.Now(),
		KafkaMessage: &queue.KafkaMessage{
			ID:          rm.MessageID.String(),
			MessageID:   rm.MessageID,
			Source:      rm.Source,
			Destination: rm.Destination,
			Text:        rm.Text,
			ClientID:    rm.ClientID,
			Priority:    rm.Priority,
			RetryCount:  rm.RetryCount + 1,
			MaxRetries:  rm.MaxRetries,
			CreatedAt:   rm.CreatedAt,
			Metadata:    rm.Metadata,
		},
	}

	data, err := failedMsg.Serialize()
	require.NoError(t, err)

	decoded, err := queue.DeserializeFailed(data)
	require.NoError(t, err)

	assert.Equal(t, msgID, decoded.MessageID)
	assert.Equal(t, 2, decoded.RetryCount)
	require.NotNil(t, decoded.KafkaMessage)
	assert.Equal(t, rm.Source, decoded.KafkaMessage.Source)
	assert.Equal(t, rm.Destination, decoded.KafkaMessage.Destination)
	assert.Equal(t, 2, decoded.KafkaMessage.RetryCount)
	assert.Equal(t, 3, decoded.KafkaMessage.MaxRetries)
}

func TestFailedMessage_NoRetryWhenExhausted(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		RetryCount: 3,
		MaxRetries: 3,
	}

	// In processMessage, this condition prevents publishing to sms.failed:
	// sendErr != nil && routedMsg.RetryCount < routedMsg.MaxRetries
	assert.False(t, rm.RetryCount < rm.MaxRetries,
		"should not retry when RetryCount >= MaxRetries")
}

func TestFailedMessage_RetryAllowed(t *testing.T) {
	t.Parallel()

	rm := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		RetryCount: 2,
		MaxRetries: 3,
	}

	assert.True(t, rm.RetryCount < rm.MaxRetries,
		"should allow retry when RetryCount < MaxRetries")
}

// ---------------------------------------------------------------------------
// handleSendOutcome — регрессия на bug #13 (iter 2)
// (docs/.../2026-04-21-network-stats-*... see review feedback):
// при unreachable провайдере sendErr проходит через общий failed-path:
// SentMessage{status=failed} публикуется, failover или refund — по условиям.
// ---------------------------------------------------------------------------

// publishedMsg — захваченное обращение к producer.PublishAsync.
type publishedMsg struct {
	Topic   string
	Key     string
	Value   []byte
	Headers []sarama.RecordHeader
}

// capturingPublisher реализует messagePublisher и собирает все publish'ы.
type capturingPublisher struct {
	msgs []publishedMsg
}

func (p *capturingPublisher) PublishAsync(topic, key string, value []byte, headers []sarama.RecordHeader) {
	p.msgs = append(p.msgs, publishedMsg{
		Topic:   topic,
		Key:     key,
		Value:   append([]byte(nil), value...),
		Headers: headers,
	})
}

func (p *capturingPublisher) byTopic(topic string) []publishedMsg {
	var out []publishedMsg
	for _, m := range p.msgs {
		if m.Topic == topic {
			out = append(out, m)
		}
	}
	return out
}

func newTestStage(pub *capturingPublisher) *Stage {
	return &Stage{
		cfg: &config.Config{
			Kafka: config.KafkaConfig{
				TopicSent:   "sms.sent",
				TopicFailed: "sms.failed",
			},
		},
		logger:        zerolog.Nop(),
		testPublisher: pub,
	}
}

// TestHandleSendOutcome_UnreachableProvider_RetryAvailable проверяет что при
// sendErr (неважно — unreachable или submit failure) и RetryCount < MaxRetries:
//   - SentMessage{status=failed} публикуется в sms.sent
//   - FailedMessage публикуется в sms.failed для failover
func TestHandleSendOutcome_UnreachableProvider_RetryAvailable(t *testing.T) {
	t.Parallel()

	pub := &capturingPublisher{}
	stage := newTestStage(pub)

	providerID := uuid.New()
	clientID := uuid.New()
	routedMsg := &pipeline.RoutedMessage{
		SchemaVersion: 1,
		MessageID:     uuid.New(),
		Source:        "Sender",
		Destination:   "+79001234567",
		Text:          "Hello",
		ClientID:      &clientID,
		ProviderID:    providerID,
		RetryCount:    0,
		MaxRetries:    3,
		CreatedAt:     time.Now(),
	}

	sentMsg, err := stage.handleSendOutcome(context.Background(), sendOutcomeInput{
		routedMsg:      routedMsg,
		traceID:        "trace-1",
		sendErr:        errors.New("провайдер X недоступен: connection refused"),
		usedProviderID: providerID,
	})
	require.NoError(t, err)
	require.NotNil(t, sentMsg)
	assert.Equal(t, "failed", sentMsg.Status)
	assert.Equal(t, providerID, sentMsg.ProviderID)
	require.NotNil(t, sentMsg.ErrorMessage)
	assert.Contains(t, *sentMsg.ErrorMessage, "недоступен")

	// SentMessage{failed} published
	sent := pub.byTopic("sms.sent")
	require.Len(t, sent, 1, "должен быть один publish в sms.sent")
	assert.Equal(t, providerID.String(), sent[0].Key)

	// FailedMessage published для failover
	failed := pub.byTopic("sms.failed")
	require.Len(t, failed, 1, "должен быть один publish в sms.failed для failover retry")
	decoded, derr := queue.DeserializeFailed(failed[0].Value)
	require.NoError(t, derr)
	assert.Equal(t, routedMsg.MessageID, decoded.MessageID)
	assert.Equal(t, 1, decoded.RetryCount, "retry_count инкрементирован")
}

// TestHandleSendOutcome_UnreachableProvider_RetriesExhausted проверяет что при
// sendErr + RetryCount >= MaxRetries:
//   - FailedMessage НЕ публикуется (retries исчерпаны, failover бесполезен)
//   - SentMessage{status=failed} публикуется
//
// Refund при провале не нужен: TarifyMessage read-only, CommitCharge при
// sendErr != nil не вызывался — деньги не списаны.
func TestHandleSendOutcome_UnreachableProvider_RetriesExhausted(t *testing.T) {
	t.Parallel()

	pub := &capturingPublisher{}
	stage := newTestStage(pub)

	providerID := uuid.New()
	clientID := uuid.New()
	routedMsg := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		ClientID:   &clientID,
		ProviderID: providerID,
		RetryCount: 3,
		MaxRetries: 3, // исчерпаны
	}

	sentMsg, err := stage.handleSendOutcome(context.Background(), sendOutcomeInput{
		routedMsg:      routedMsg,
		traceID:        "trace-2",
		sendErr:        errors.New("провайдер Y недоступен"),
		usedProviderID: providerID,
	})
	require.NoError(t, err)
	assert.Equal(t, "failed", sentMsg.Status)

	// FailedMessage НЕ опубликован — retries исчерпаны
	assert.Empty(t, pub.byTopic("sms.failed"), "sms.failed не должен публиковаться при исчерпанных retries")
	// SentMessage{failed} публикуется
	require.Len(t, pub.byTopic("sms.sent"), 1)
}

// ---------------------------------------------------------------------------
// Permanent-error classification (порт из legacy RetryManager cmd/worker):
// перманентные ESME-ошибки не должны попадать в retry-цикл sms.failed.
// ---------------------------------------------------------------------------

func TestIsPermanentSendError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"permanent dst", errors.New("submit failed: ESME_RINVDSTADR (0x0000000B)"), true},
		{"permanent lowercase", errors.New("smpp error: esme_rinvmsglen"), true},
		{"transient timeout", errors.New("context deadline exceeded"), false},
		{"transient refused", errors.New("провайдер X недоступен: connection refused"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isPermanentSendError(tc.err))
		})
	}
}

// TestHandleSendOutcome_PermanentError_NoRetryPublish: при перманентной
// ESME-ошибке и неисчерпанном retry-бюджете sms.failed НЕ публикуется —
// повторная отправка того же PDU бессмысленна; финальный failed уходит
// в sms.sent как обычно.
func TestHandleSendOutcome_PermanentError_NoRetryPublish(t *testing.T) {
	t.Parallel()

	pub := &capturingPublisher{}
	stage := newTestStage(pub)

	providerID := uuid.New()
	clientID := uuid.New()
	routedMsg := &pipeline.RoutedMessage{
		MessageID:  uuid.New(),
		ClientID:   &clientID,
		ProviderID: providerID,
		RetryCount: 0,
		MaxRetries: 3, // бюджет есть, но ошибка перманентная
	}

	sentMsg, err := stage.handleSendOutcome(context.Background(), sendOutcomeInput{
		routedMsg:      routedMsg,
		traceID:        "trace-3",
		sendErr:        errors.New("submit failed: ESME_RINVDSTADR"),
		usedProviderID: providerID,
	})
	require.NoError(t, err)
	assert.Equal(t, "failed", sentMsg.Status)

	assert.Empty(t, pub.byTopic("sms.failed"),
		"перманентная ошибка не должна публиковаться в sms.failed")
	require.Len(t, pub.byTopic("sms.sent"), 1,
		"финальный failed должен публиковаться в sms.sent")
}
