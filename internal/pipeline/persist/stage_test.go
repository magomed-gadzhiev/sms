package persist

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/pipeline"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// makeRoutedConsumerMessage creates a sarama.ConsumerMessage from a RoutedMessage.
func makeRoutedConsumerMessage(t *testing.T, rm *pipeline.RoutedMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := rm.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Value:     data,
		Topic:     "sms.routed",
		Partition: 0,
		Offset:    1,
	}
}

func newTestRoutedMessage() *pipeline.RoutedMessage {
	clientID := uuid.New()
	operatorID := uuid.New()
	countryID := uuid.New()
	routeID := uuid.New()
	templateID := uuid.New()
	senderNameID := uuid.New()

	return &pipeline.RoutedMessage{
		SchemaVersion: 2,
		MessageID:     uuid.New(),
		TraceID:       "trace-xyz",
		Source:        "TestApp",
		Destination:   "+79001234567",
		Text:          "Hello, world!",
		ClientID:      &clientID,
		OperatorID:    &operatorID,
		CountryID:     &countryID,
		Channel:       "sms",
		TemplateID:    &templateID,
		SenderNameID:  &senderNameID,
		ProviderID:    uuid.New(),
		RouteID:       &routeID,
		Priority:      2,
		RetryCount:    0,
		MaxRetries:    3,
		RoutedAt:      time.Now(),
		CreatedAt:     time.Now().Add(-time.Minute),
		Metadata:      map[string]interface{}{"campaign": "test"},
	}
}

// ---------------------------------------------------------------------------
// buildCopyRows
// ---------------------------------------------------------------------------

func TestBuildCopyRows_HappyPath(t *testing.T) {
	t.Parallel()

	rm := newTestRoutedMessage()
	msgs := []*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)}

	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, rm.MessageID, row.id)
	assert.Equal(t, rm.MessageID.String(), row.messageID)
	assert.Equal(t, rm.Source, row.source)
	assert.Equal(t, rm.Destination, row.destination)
	assert.Equal(t, rm.Text, row.text)
	assert.Equal(t, "GSM7", row.encoding) // ASCII text is GSM7
	assert.Equal(t, 1, row.segmentCount)
	assert.Equal(t, "pending", row.status)
	assert.Equal(t, rm.Priority, row.priorityFlag)
	require.NotNil(t, row.providerID)
	assert.Equal(t, rm.ProviderID, *row.providerID)
	assert.Equal(t, rm.RouteID, row.routeID)
	assert.Equal(t, rm.ClientID, row.clientID)
	assert.Equal(t, rm.OperatorID, row.operatorID)
	assert.Equal(t, rm.CountryID, row.countryID)
	assert.Equal(t, rm.TemplateID, row.templateID)
	assert.Equal(t, rm.SenderNameID, row.senderNameID)
	require.NotNil(t, row.channel)
	assert.Equal(t, "sms", *row.channel)
	assert.Equal(t, rm.RetryCount, row.retryCount)
	assert.Equal(t, rm.MaxRetries, row.maxRetries)
	assert.False(t, row.createdAt.IsZero())
	assert.False(t, row.updatedAt.IsZero())
}

func TestBuildCopyRows_EnrichmentColumnsPopulated(t *testing.T) {
	t.Parallel()

	// Regression for bug #15 — all four enrichment columns must be present
	// in the row produced from a RoutedMessage.
	rm := newTestRoutedMessage()
	rows, errs := buildCopyRows([]*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)})

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	row := rows[0]

	assert.NotNil(t, row.operatorID, "operator_id must land at INSERT time")
	assert.NotNil(t, row.providerID, "provider_id must land at INSERT time")
	assert.NotNil(t, row.countryID, "country_id must land at INSERT time")
	assert.NotNil(t, row.channel, "channel must land at INSERT time")
}

func TestBuildCopyRows_UCS2Encoding(t *testing.T) {
	t.Parallel()

	rm := newTestRoutedMessage()
	rm.Text = "Привет, мир! 😀" // Non-GSM7 characters

	msgs := []*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)}
	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	assert.Equal(t, "UCS2", rows[0].encoding)
}

func TestBuildCopyRows_NilMessageID_GeneratesNew(t *testing.T) {
	t.Parallel()

	rm := newTestRoutedMessage()
	rm.MessageID = uuid.Nil

	msgs := []*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)}
	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	assert.NotEqual(t, uuid.Nil, rows[0].id, "should generate a new UUID when MessageID is Nil")
}

func TestBuildCopyRows_ZeroCreatedAt_UsesNow(t *testing.T) {
	t.Parallel()

	rm := newTestRoutedMessage()
	rm.CreatedAt = time.Time{} // zero value

	msgs := []*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)}
	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)

	// createdAt should be approximately now
	assert.WithinDuration(t, time.Now(), rows[0].createdAt, 5*time.Second)
}

func TestBuildCopyRows_DeserializationError(t *testing.T) {
	t.Parallel()

	badMsg := &sarama.ConsumerMessage{
		Value:     []byte("not valid json"),
		Topic:     "sms.routed",
		Partition: 0,
		Offset:    42,
	}

	rows, errs := buildCopyRows([]*sarama.ConsumerMessage{badMsg})

	assert.Empty(t, rows)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "offset 42")
	assert.Contains(t, errs[0].Error(), "partition 0")
}

func TestBuildCopyRows_MixedValidAndInvalid(t *testing.T) {
	t.Parallel()

	good := makeRoutedConsumerMessage(t, newTestRoutedMessage())
	bad := &sarama.ConsumerMessage{
		Value:     []byte("broken"),
		Partition: 1,
		Offset:    99,
	}
	good2 := makeRoutedConsumerMessage(t, newTestRoutedMessage())

	rows, errs := buildCopyRows([]*sarama.ConsumerMessage{good, bad, good2})

	assert.Len(t, rows, 2, "two valid rows")
	assert.Len(t, errs, 1, "one deserialization error")
}

func TestBuildCopyRows_EmptyBatch(t *testing.T) {
	t.Parallel()

	rows, errs := buildCopyRows([]*sarama.ConsumerMessage{})

	assert.Empty(t, rows)
	assert.Empty(t, errs)
}

func TestBuildCopyRows_MultipleMsgsSegmentCount(t *testing.T) {
	t.Parallel()

	// Short text = 1 segment
	short := newTestRoutedMessage()
	short.Text = "Hi"

	// Long GSM7 text = multiple segments (>160 chars)
	long := newTestRoutedMessage()
	long.Text = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	for len(long.Text) < 200 {
		long.Text += "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	}

	msgs := []*sarama.ConsumerMessage{
		makeRoutedConsumerMessage(t, short),
		makeRoutedConsumerMessage(t, long),
	}

	rows, errs := buildCopyRows(msgs)
	require.Empty(t, errs)
	require.Len(t, rows, 2)

	assert.Equal(t, 1, rows[0].segmentCount)
	assert.Greater(t, rows[1].segmentCount, 1, "long text should have multiple segments")
}

func TestBuildCopyRows_MissingChannelAndCountry_NilPointers(t *testing.T) {
	t.Parallel()

	// Backward compatibility: a RoutedMessage from an old router pod (pre
	// enrichment fix) has no Channel, no CountryID. persist must still insert
	// the row (with NULL columns) rather than dropping the message.
	rm := newTestRoutedMessage()
	rm.Channel = ""
	rm.CountryID = nil

	rows, errs := buildCopyRows([]*sarama.ConsumerMessage{makeRoutedConsumerMessage(t, rm)})

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].channel)
	assert.Nil(t, rows[0].countryID)
	// operator_id + provider_id are still present from the router on v1 messages.
}

// ---------------------------------------------------------------------------
// copySource
// ---------------------------------------------------------------------------

func TestCopySource_EmptyRows(t *testing.T) {
	t.Parallel()

	cs := &copySource{rows: []messageRow{}}
	assert.False(t, cs.Next())
	assert.NoError(t, cs.Err())
}

func TestCopySource_IteratesAllRows(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	routeID := uuid.New()
	clientID := uuid.New()
	operatorID := uuid.New()
	countryID := uuid.New()
	channel := "sms"
	now := time.Now()

	rows := []messageRow{
		{
			id: uuid.New(), messageID: "msg-1", source: "A", destination: "+7900",
			text: "hello", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 0, providerID: &providerID, routeID: &routeID, clientID: &clientID,
			operatorID: &operatorID, countryID: &countryID, channel: &channel,
			retryCount: 0, maxRetries: 3, createdAt: now, updatedAt: now,
		},
		{
			id: uuid.New(), messageID: "msg-2", source: "B", destination: "+7901",
			text: "world", encoding: "UCS2", segmentCount: 2, status: "pending",
			priorityFlag: 1, providerID: nil, routeID: nil, clientID: nil,
			operatorID: nil, countryID: nil, channel: nil,
			retryCount: 1, maxRetries: 5, createdAt: now, updatedAt: now,
		},
	}

	cs := &copySource{rows: rows}

	var count int
	for cs.Next() {
		vals, err := cs.Values()
		require.NoError(t, err)
		require.Len(t, vals, len(copyColumns), "should have %d columns", len(copyColumns))
		count++
	}

	assert.Equal(t, 2, count)
	assert.False(t, cs.Next(), "should return false after last row")
	assert.NoError(t, cs.Err())
}

func TestCopySource_ValuesOrder(t *testing.T) {
	t.Parallel()

	providerID := uuid.New()
	routeID := uuid.New()
	clientID := uuid.New()
	operatorID := uuid.New()
	countryID := uuid.New()
	templateID := uuid.New()
	senderNameID := uuid.New()
	channel := "sms"
	now := time.Now()
	rowID := uuid.New()

	rows := []messageRow{
		{
			id: rowID, messageID: "ext-1", source: "SRC", destination: "+79001234567",
			text: "test text", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 2, providerID: &providerID, routeID: &routeID, clientID: &clientID,
			templateID: &templateID, senderNameID: &senderNameID,
			operatorID: &operatorID, countryID: &countryID, channel: &channel,
			retryCount: 0, maxRetries: 3, createdAt: now, updatedAt: now,
		},
	}

	cs := &copySource{rows: rows}
	require.True(t, cs.Next())

	vals, err := cs.Values()
	require.NoError(t, err)

	// Verify order matches copyColumns
	assert.Equal(t, rowID, vals[0])          // id
	assert.Equal(t, "ext-1", vals[1])        // message_id
	assert.Equal(t, "SRC", vals[2])          // source
	assert.Equal(t, "+79001234567", vals[3]) // destination
	assert.Equal(t, "test text", vals[4])    // text
	assert.Equal(t, "GSM7", vals[5])         // encoding
	assert.Equal(t, 1, vals[6])              // segment_count
	assert.Equal(t, "pending", vals[7])      // status
	assert.Equal(t, 2, vals[8])              // priority_flag
	assert.Equal(t, &providerID, vals[9])    // provider_id
	assert.Equal(t, &routeID, vals[10])      // route_id
	assert.Equal(t, &clientID, vals[11])     // client_id
	assert.Equal(t, &templateID, vals[12])   // template_id
	assert.Equal(t, &senderNameID, vals[13]) // sender_name_id
	assert.Equal(t, &operatorID, vals[14])   // operator_id
	assert.Equal(t, &countryID, vals[15])    // country_id
	assert.Equal(t, "sms", vals[16])         // channel
	assert.Equal(t, 0, vals[17])             // retry_count
	assert.Equal(t, 3, vals[18])             // max_retries
	assert.Equal(t, now, vals[19])           // created_at
	assert.Equal(t, now, vals[20])           // updated_at
}

func TestCopySource_NilPointerFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rows := []messageRow{
		{
			id: uuid.New(), messageID: "msg-nil", source: "A", destination: "+7900",
			text: "x", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 0, providerID: nil, routeID: nil, clientID: nil,
			operatorID: nil, countryID: nil, channel: nil,
			retryCount: 0, maxRetries: 5, createdAt: now, updatedAt: now,
		},
	}

	cs := &copySource{rows: rows}
	require.True(t, cs.Next())

	vals, err := cs.Values()
	require.NoError(t, err)

	assert.Nil(t, vals[9])  // provider_id
	assert.Nil(t, vals[10]) // route_id
	assert.Nil(t, vals[11]) // client_id
	assert.Nil(t, vals[14]) // operator_id
	assert.Nil(t, vals[15]) // country_id
	assert.Nil(t, vals[16]) // channel
}

// ---------------------------------------------------------------------------
// copyColumns sanity check
// ---------------------------------------------------------------------------

func TestCopyColumns_Count(t *testing.T) {
	t.Parallel()
	// 14 original + operator_id + country_id + channel + 4 legacy (retry/max/created/updated) = 21.
	assert.Len(t, copyColumns, 21, "copyColumns should list all persisted columns")
}

func TestCopyColumns_ExpectedNames(t *testing.T) {
	t.Parallel()

	expected := []string{
		"id", "message_id", "source", "destination", "text", "encoding",
		"segment_count", "status", "priority_flag", "provider_id",
		"route_id", "client_id", "template_id", "sender_name_id",
		"operator_id", "country_id", "channel",
		"retry_count", "max_retries",
		"created_at", "updated_at",
	}
	assert.Equal(t, expected, copyColumns)
}
