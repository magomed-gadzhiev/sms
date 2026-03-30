package persist

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/queue"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// makeKafkaConsumerMessage creates a sarama.ConsumerMessage from a KafkaMessage.
func makeKafkaConsumerMessage(t *testing.T, km *queue.KafkaMessage) *sarama.ConsumerMessage {
	t.Helper()
	data, err := km.Serialize()
	require.NoError(t, err)
	return &sarama.ConsumerMessage{
		Value:     data,
		Topic:     "sms.outgoing",
		Partition: 0,
		Offset:    1,
	}
}

func newTestKafkaMessage() *queue.KafkaMessage {
	clientID := uuid.New()
	providerID := uuid.New()
	routeID := uuid.New()

	return &queue.KafkaMessage{
		ID:          "ext-123",
		MessageID:   uuid.New(),
		Source:      "TestApp",
		Destination: "+79001234567",
		Text:        "Hello, world!",
		ProviderID:  &providerID,
		RouteID:     &routeID,
		ClientID:    &clientID,
		Priority:    2,
		RetryCount:  0,
		MaxRetries:  3,
		CreatedAt:   time.Now().Add(-time.Minute),
		Metadata:    map[string]interface{}{"campaign": "test"},
	}
}

// ---------------------------------------------------------------------------
// buildCopyRows
// ---------------------------------------------------------------------------

func TestBuildCopyRows_HappyPath(t *testing.T) {
	t.Parallel()

	km := newTestKafkaMessage()
	msgs := []*sarama.ConsumerMessage{makeKafkaConsumerMessage(t, km)}

	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, km.MessageID, row.id)
	assert.Equal(t, km.ID, row.messageID)
	assert.Equal(t, km.Source, row.source)
	assert.Equal(t, km.Destination, row.destination)
	assert.Equal(t, km.Text, row.text)
	assert.Equal(t, "GSM7", row.encoding) // ASCII text is GSM7
	assert.Equal(t, 1, row.segmentCount)
	assert.Equal(t, "pending", row.status)
	assert.Equal(t, km.Priority, row.priorityFlag)
	assert.Equal(t, km.ProviderID, row.providerID)
	assert.Equal(t, km.RouteID, row.routeID)
	assert.Equal(t, km.ClientID, row.clientID)
	assert.Equal(t, km.RetryCount, row.retryCount)
	assert.Equal(t, km.MaxRetries, row.maxRetries)
	assert.False(t, row.createdAt.IsZero())
	assert.False(t, row.updatedAt.IsZero())
}

func TestBuildCopyRows_UCS2Encoding(t *testing.T) {
	t.Parallel()

	km := newTestKafkaMessage()
	km.Text = "Привет, мир! 😀" // Non-GSM7 characters

	msgs := []*sarama.ConsumerMessage{makeKafkaConsumerMessage(t, km)}
	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	assert.Equal(t, "UCS2", rows[0].encoding)
}

func TestBuildCopyRows_NilMessageID_GeneratesNew(t *testing.T) {
	t.Parallel()

	km := newTestKafkaMessage()
	km.MessageID = uuid.Nil

	msgs := []*sarama.ConsumerMessage{makeKafkaConsumerMessage(t, km)}
	rows, errs := buildCopyRows(msgs)

	require.Empty(t, errs)
	require.Len(t, rows, 1)
	assert.NotEqual(t, uuid.Nil, rows[0].id, "should generate a new UUID when MessageID is Nil")
}

func TestBuildCopyRows_ZeroCreatedAt_UsesNow(t *testing.T) {
	t.Parallel()

	km := newTestKafkaMessage()
	km.CreatedAt = time.Time{} // zero value

	msgs := []*sarama.ConsumerMessage{makeKafkaConsumerMessage(t, km)}
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
		Topic:     "sms.outgoing",
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

	good := makeKafkaConsumerMessage(t, newTestKafkaMessage())
	bad := &sarama.ConsumerMessage{
		Value:     []byte("broken"),
		Partition: 1,
		Offset:    99,
	}
	good2 := makeKafkaConsumerMessage(t, newTestKafkaMessage())

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
	short := newTestKafkaMessage()
	short.Text = "Hi"

	// Long GSM7 text = multiple segments (>160 chars)
	long := newTestKafkaMessage()
	long.Text = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " // repeat to exceed 160
	for len(long.Text) < 200 {
		long.Text += "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	}

	msgs := []*sarama.ConsumerMessage{
		makeKafkaConsumerMessage(t, short),
		makeKafkaConsumerMessage(t, long),
	}

	rows, errs := buildCopyRows(msgs)
	require.Empty(t, errs)
	require.Len(t, rows, 2)

	assert.Equal(t, 1, rows[0].segmentCount)
	assert.Greater(t, rows[1].segmentCount, 1, "long text should have multiple segments")
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
	now := time.Now()

	rows := []messageRow{
		{
			id: uuid.New(), messageID: "msg-1", source: "A", destination: "+7900",
			text: "hello", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 0, providerID: &providerID, routeID: &routeID, clientID: &clientID,
			retryCount: 0, maxRetries: 3, createdAt: now, updatedAt: now,
		},
		{
			id: uuid.New(), messageID: "msg-2", source: "B", destination: "+7901",
			text: "world", encoding: "UCS2", segmentCount: 2, status: "pending",
			priorityFlag: 1, providerID: nil, routeID: nil, clientID: nil,
			retryCount: 1, maxRetries: 5, createdAt: now, updatedAt: now,
		},
	}

	cs := &copySource{rows: rows}

	// Iterate through all rows
	var count int
	for cs.Next() {
		vals, err := cs.Values()
		require.NoError(t, err)
		require.Len(t, vals, 16, "should have 16 columns")
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
	now := time.Now()
	rowID := uuid.New()

	rows := []messageRow{
		{
			id: rowID, messageID: "ext-1", source: "SRC", destination: "+79001234567",
			text: "test text", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 2, providerID: &providerID, routeID: &routeID, clientID: &clientID,
			retryCount: 0, maxRetries: 3, createdAt: now, updatedAt: now,
		},
	}

	cs := &copySource{rows: rows}
	require.True(t, cs.Next())

	vals, err := cs.Values()
	require.NoError(t, err)

	// Verify order matches copyColumns
	assert.Equal(t, rowID, vals[0])            // id
	assert.Equal(t, "ext-1", vals[1])          // message_id
	assert.Equal(t, "SRC", vals[2])            // source
	assert.Equal(t, "+79001234567", vals[3])   // destination
	assert.Equal(t, "test text", vals[4])      // text
	assert.Equal(t, "GSM7", vals[5])           // encoding
	assert.Equal(t, 1, vals[6])                // segment_count
	assert.Equal(t, "pending", vals[7])        // status
	assert.Equal(t, 2, vals[8])                // priority_flag
	assert.Equal(t, &providerID, vals[9])      // provider_id
	assert.Equal(t, &routeID, vals[10])        // route_id
	assert.Equal(t, &clientID, vals[11])       // client_id
	assert.Equal(t, 0, vals[12])               // retry_count
	assert.Equal(t, 3, vals[13])               // max_retries
	assert.Equal(t, now, vals[14])             // created_at
	assert.Equal(t, now, vals[15])             // updated_at
}

func TestCopySource_NilPointerFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rows := []messageRow{
		{
			id: uuid.New(), messageID: "msg-nil", source: "A", destination: "+7900",
			text: "x", encoding: "GSM7", segmentCount: 1, status: "pending",
			priorityFlag: 0, providerID: nil, routeID: nil, clientID: nil,
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
}

// ---------------------------------------------------------------------------
// copyColumns sanity check
// ---------------------------------------------------------------------------

func TestCopyColumns_Count(t *testing.T) {
	t.Parallel()
	assert.Len(t, copyColumns, 16, "copyColumns should have 16 entries matching the COPY INSERT")
}

func TestCopyColumns_ExpectedNames(t *testing.T) {
	t.Parallel()

	expected := []string{
		"id", "message_id", "source", "destination", "text", "encoding",
		"segment_count", "status", "priority_flag", "provider_id",
		"route_id", "client_id", "retry_count", "max_retries",
		"created_at", "updated_at",
	}
	assert.Equal(t, expected, copyColumns)
}
