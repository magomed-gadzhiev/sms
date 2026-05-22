package dlr

import (
	"fmt"
	"time"
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

func formatSMPPDate(t time.Time) string {
	return t.Format("0601021504")
}
