package domain

import (
	"time"

	"github.com/google/uuid"
)

// OperatorPrefix представляет привязку номерного префикса к оператору
type OperatorPrefix struct {
	ID         uuid.UUID
	OperatorID uuid.UUID
	Prefix     string // e.g. "+7900"
	Priority   int
	CreatedAt  time.Time
}

// NewOperatorPrefix создает новую привязку префикса
func NewOperatorPrefix(operatorID uuid.UUID, prefix string, priority int) *OperatorPrefix {
	return &OperatorPrefix{
		ID:         uuid.New(),
		OperatorID: operatorID,
		Prefix:     prefix,
		Priority:   priority,
		CreatedAt:  time.Now(),
	}
}
