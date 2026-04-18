package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PeriodType — тип периода агрегации для usage counter.
// Извлекается из tiers_json.period в моделях (tiered, prepaid_threshold).
// Для fixed модели периода нет, используется calendar_month как дефолт (см.
// ComputePeriodKey с пустым period).
type PeriodType string

const (
	PeriodCalendarMonth PeriodType = "calendar_month"
	PeriodCalendarDay   PeriodType = "calendar_day"
)

// SubaccountUsageCounter — единый per-subaccount-per-period счётчик потребления.
// Именно per-subaccount (а не per-source_rule): при смене тарифа в середине
// периода счётчик продолжается, тир клиента не обнуляется.
type SubaccountUsageCounter struct {
	SubaccountID  uuid.UUID
	PeriodKey     string
	SegmentsUsed  int64
	AmountCharged string // NUMERIC as string
	LastUpdated   time.Time
}

// ComputePeriodKey возвращает строковый ключ периода по UTC.
// Пустой period трактуется как calendar_month (дефолт для fixed модели,
// где в tiers_json нет поля period).
func ComputePeriodKey(period PeriodType, at time.Time) string {
	utc := at.UTC()
	switch period {
	case PeriodCalendarDay:
		return utc.Format("2006-01-02")
	case PeriodCalendarMonth, "":
		return utc.Format("2006-01")
	default:
		return fmt.Sprintf("unknown-%s-%s", period, utc.Format("2006-01"))
	}
}
