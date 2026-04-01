package dlr

// DLRStatus представляет значение поля stat в DLR receipt.
type DLRStatus string

const (
	DLRStatusDelivered DLRStatus = "DELIVRD"
	DLRStatusExpired   DLRStatus = "EXPIRED"
	DLRStatusRejected  DLRStatus = "REJECTD"
	DLRStatusUndeliv   DLRStatus = "UNDELIV"
)

// IsTerminal возвращает true, если статус является финальным.
func IsTerminal(s DLRStatus) bool {
	switch s {
	case DLRStatusDelivered, DLRStatusExpired, DLRStatusRejected, DLRStatusUndeliv:
		return true
	}
	return false
}
