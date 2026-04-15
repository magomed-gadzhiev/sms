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
	})
	assert.Contains(t, receipt, "stat:EXPIRED")
	assert.Contains(t, receipt, "dlvrd:000")
}

func TestFormatReceipt_Rejected(t *testing.T) {
	now := time.Now()
	receipt := FormatReceipt(ReceiptParams{
		MessageID:  "test-msg-3",
		Status:     "rejected",
		SubmitDate: now,
		DoneDate:   now,
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
