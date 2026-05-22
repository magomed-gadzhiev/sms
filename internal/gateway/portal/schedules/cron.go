package schedules

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NextRunTime returns the next scheduled execution time after `from`
// for the given frequency and optional cron expression.
func NextRunTime(frequency, cronExpr string, from time.Time) time.Time {
	from = from.UTC()
	switch frequency {
	case "daily":
		return from.AddDate(0, 0, 1)
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "custom":
		if cronExpr != "" {
			return nextCronTime(cronExpr, from)
		}
		return from.AddDate(0, 0, 1)
	default:
		return from.AddDate(0, 0, 1)
	}
}

// ValidateCronExpression validates a standard 5-field cron expression.
// Fields: minute hour day-of-month month day-of-week
func ValidateCronExpression(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("должно быть 5 полей (мин час день месяц день-недели), получено %d", len(fields))
	}
	type fieldSpec struct {
		name     string
		min, max int
	}
	specs := []fieldSpec{
		{"минута", 0, 59},
		{"час", 0, 23},
		{"день месяца", 1, 31},
		{"месяц", 1, 12},
		{"день недели", 0, 7},
	}
	for i, sp := range specs {
		if err := validateCronField(fields[i], sp.min, sp.max); err != nil {
			return fmt.Errorf("поле '%s': %w", sp.name, err)
		}
	}
	return nil
}

// validateCronField checks that a single cron field string is valid for the given range.
func validateCronField(field string, min, max int) error {
	if field == "*" {
		return nil
	}
	// Step: expr/N
	if idx := strings.Index(field, "/"); idx != -1 {
		step, err := strconv.Atoi(field[idx+1:])
		if err != nil || step <= 0 {
			return fmt.Errorf("некорректный шаг")
		}
		base := field[:idx]
		if base != "*" {
			return validateCronField(base, min, max)
		}
		return nil
	}
	// List: N,M,...
	if strings.Contains(field, ",") {
		for _, p := range strings.Split(field, ",") {
			if err := validateCronField(strings.TrimSpace(p), min, max); err != nil {
				return err
			}
		}
		return nil
	}
	// Range: N-M
	if idx := strings.Index(field, "-"); idx != -1 {
		s, err1 := strconv.Atoi(field[:idx])
		e, err2 := strconv.Atoi(field[idx+1:])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("некорректный диапазон")
		}
		if s > e || s < min || e > max {
			return fmt.Errorf("диапазон %d-%d вне допустимых значений %d-%d", s, e, min, max)
		}
		return nil
	}
	// Exact value
	v, err := strconv.Atoi(field)
	if err != nil {
		return fmt.Errorf("некорректное значение: %s", field)
	}
	// day-of-week: 7 is also a valid alias for Sunday (0)
	if max == 7 && v == 7 {
		return nil
	}
	if v < min || v > max {
		return fmt.Errorf("значение %d вне допустимых %d-%d", v, min, max)
	}
	return nil
}

// nextCronTime returns the next time after `from` that matches the 5-field cron expression.
// Fields: minute hour day-of-month month day-of-week
// Uses an optimized search: advances by month/day/hour when possible.
func nextCronTime(expr string, from time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return from.AddDate(0, 0, 1)
	}
	t := from.Add(time.Minute).Truncate(time.Minute).UTC()
	limit := from.AddDate(1, 0, 0)
	for t.Before(limit) {
		// Check month
		if !matchCronField(fields[3], int(t.Month()), 1, 12) {
			// Advance to 1st of next month at midnight
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		// Check day-of-month and day-of-week (both must match if both are specific)
		dayMatch := matchCronField(fields[2], t.Day(), 1, 31)
		wdayMatch := matchCronField(fields[4], int(t.Weekday()), 0, 7)
		if !dayMatch || !wdayMatch {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		// Check hour
		if !matchCronField(fields[1], t.Hour(), 0, 23) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		// Check minute
		if !matchCronField(fields[0], t.Minute(), 0, 59) {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	// Fallback: daily if no match found within a year
	return from.AddDate(0, 0, 1)
}

// matchCronField reports whether value matches a cron field expression.
// Supports: *, N, N-M, */N, N-M/N, N,M,...
func matchCronField(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}
	// Step: base/N
	if idx := strings.Index(field, "/"); idx != -1 {
		step, err := strconv.Atoi(field[idx+1:])
		if err != nil || step <= 0 {
			return false
		}
		base := field[:idx]
		var start, end int
		if base == "*" {
			start, end = min, max
		} else if ridx := strings.Index(base, "-"); ridx != -1 {
			start, _ = strconv.Atoi(base[:ridx])
			end, _ = strconv.Atoi(base[ridx+1:])
		} else {
			start, _ = strconv.Atoi(base)
			end = max
		}
		return value >= start && value <= end && (value-start)%step == 0
	}
	// List: N,M,...
	if strings.Contains(field, ",") {
		for _, p := range strings.Split(field, ",") {
			v, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				continue
			}
			if v == value || (max == 7 && v == 7 && value == 0) {
				return true
			}
		}
		return false
	}
	// Range: N-M
	if idx := strings.Index(field, "-"); idx != -1 {
		s, _ := strconv.Atoi(field[:idx])
		e, _ := strconv.Atoi(field[idx+1:])
		return value >= s && value <= e
	}
	// Exact value
	v, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	// day-of-week: 7 == 0 (both mean Sunday)
	if max == 7 && v == 7 && value == 0 {
		return true
	}
	return v == value
}
