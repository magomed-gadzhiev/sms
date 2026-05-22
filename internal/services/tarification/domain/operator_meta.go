// operator_meta.go
package domain

// OperatorMeta — метаданные оператора, необходимые tarification hot path.
// Currency источник — countries.currency по operator.country_id.
// Value-object без поведения; живёт в domain, чтобы избежать infrastructure→application
// cross-layer import.
type OperatorMeta struct {
	Code     string
	Currency string
}
