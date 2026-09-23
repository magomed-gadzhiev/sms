// Package messagestatus is the single owner of the Message Status
// vocabulary (CONTEXT.md): the status enum, terminality, and the mappers
// between Message status and the wire vocabularies (DLR stat codes, SMSC
// receipt formatting).
//
// Domain rule enforced here: `unknown` is NEVER a stored status — a receipt
// with an unrecognized stat leaves the Message in `sent` until the DLR
// timeout expires it (see FromDLRStat's ok=false).
//
// Raw status string literals are banned outside this package in
// Message-domain code (scripts/check-message-status.sh, wired into CI).
// Use shared.MessageStatus* aliases or this package directly.
package messagestatus

// Status is the Message lifecycle:
// pending → queued → sent → delivered | failed | expired | rejected,
// plus scheduled and cancelled.
type Status string

const (
	Pending   Status = "pending"
	Queued    Status = "queued"
	Sent      Status = "sent"
	Delivered Status = "delivered"
	Failed    Status = "failed"
	Expired   Status = "expired"
	Rejected  Status = "rejected"
	Scheduled Status = "scheduled"
	Cancelled Status = "cancelled"
)

// All lists every valid status. Order follows the lifecycle.
var All = []Status{Pending, Queued, Sent, Delivered, Failed, Expired, Rejected, Scheduled, Cancelled}

// Valid reports whether s is a member of the vocabulary.
func Valid(s Status) bool {
	switch s {
	case Pending, Queued, Sent, Delivered, Failed, Expired, Rejected, Scheduled, Cancelled:
		return true
	}
	return false
}

// IsTerminal reports whether no further transition is expected: the four
// terminal outcomes of the send lifecycle plus cancelled. pending, queued,
// scheduled and sent are non-terminal.
func IsTerminal(s Status) bool {
	switch s {
	case Delivered, Failed, Expired, Rejected, Cancelled:
		return true
	}
	return false
}

// FromDLRStat maps a DLR receipt `stat` code to a Message status.
//
// ok=false means the receipt must NOT change the stored status: unknown
// stat codes leave the Message in `sent` until the DLR timeout expires it
// (`unknown` is never a stored status — CONTEXT.md, Message Status).
func FromDLRStat(stat string) (Status, bool) {
	switch stat {
	case "DELIVRD":
		return Delivered, true
	case "EXPIRED":
		return Expired, true
	case "REJECTD", "UNDELIV":
		return Failed, true
	default:
		return "", false
	}
}

// ToSMSC maps a Message status to the receipt representation the platform
// emits when formatting a DLR for downstream consumers: the SMPP `stat`
// code and the `dlvrd` flag. Non-terminal statuses format as UNKNOWN
// receipts (callers only format receipts for terminal statuses).
func ToSMSC(s Status) (stat, dlvrd string) {
	switch s {
	case Delivered:
		return "DELIVRD", "001"
	case Failed:
		return "UNDELIV", "000"
	case Expired:
		return "EXPIRED", "000"
	case Rejected:
		return "REJECTD", "000"
	default:
		return "UNKNOWN", "000"
	}
}
