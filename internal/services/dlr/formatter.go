package dlr

import (
	"fmt"
	"time"

	"github.com/smpp-server/smpp-server/internal/shared/messagestatus"
)

type ReceiptParams struct {
	MessageID  string
	Status     string
	SubmitDate time.Time
	DoneDate   time.Time
	ErrorCode  int
	Text       string
}

func FormatReceipt(p ReceiptParams) string {
	stat, dlvrd := MapStatusToSMSC(p.Status)
	text := p.Text
	if len(text) > 20 {
		text = text[:20]
	}
	return fmt.Sprintf("id:%s sub:001 dlvrd:%s submit date:%s done date:%s stat:%s err:%03d text:%s",
		p.MessageID, dlvrd, formatSMPPDate(p.SubmitDate), formatSMPPDate(p.DoneDate), stat, p.ErrorCode, text)
}

// MapStatusToSMSC maps a Message status to the receipt (stat, dlvrd) pair.
// Словарь принадлежит messagestatus — wrapper сохраняет сигнатуру для
// существующих вызывающих (dispatcher, тесты).
func MapStatusToSMSC(status string) (stat string, dlvrd string) {
	return messagestatus.ToSMSC(messagestatus.Status(status))
}

func formatSMPPDate(t time.Time) string {
	return t.Format("0601021504")
}
