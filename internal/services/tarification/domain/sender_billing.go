package domain

import (
	"time"

	"github.com/google/uuid"
)

// SenderNameBillingRecord представляет запись ежемесячного начисления за платное имя отправителя.
// Сумма хранится как строка для точности (аналогично другим денежным полям платформы).
type SenderNameBillingRecord struct {
	ID                   uuid.UUID
	SenderRegistrationID uuid.UUID
	ClientID             uuid.UUID
	OperatorID           uuid.UUID
	BillingMonth         time.Time // первое число месяца, UTC
	Amount               string    // сумма в RUB, например "1500.000000"
	CreatedAt            time.Time
}

// BillingMonthKey возвращает первое число месяца для t в UTC.
func BillingMonthKey(t time.Time) time.Time {
	return time.Date(t.UTC().Year(), t.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
}
