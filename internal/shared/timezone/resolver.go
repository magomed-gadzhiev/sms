// internal/shared/timezone/resolver.go
package timezone

import (
	"time"

	"github.com/nyaruka/phonenumbers"
)

// Resolver resolves phone numbers to time zones.
type Resolver interface {
	GetTimezone(phone string) (*time.Location, error)
}

// PhoneTimezoneResolver resolves timezone from phone number using libphonenumber.
type PhoneTimezoneResolver struct{}

// NewPhoneTimezoneResolver creates a new resolver.
func NewPhoneTimezoneResolver() *PhoneTimezoneResolver {
	return &PhoneTimezoneResolver{}
}

// GetTimezone parses the phone number, determines the region and timezone.
// Falls back to UTC if unable to determine.
func (r *PhoneTimezoneResolver) GetTimezone(phone string) (*time.Location, error) {
	num, err := phonenumbers.Parse(phone, "")
	if err != nil {
		return time.UTC, nil // Fallback
	}

	regionCode := phonenumbers.GetRegionCodeForNumber(num)
	if regionCode == "" {
		return time.UTC, nil
	}

	// Get timezone(s) for this number
	timezones, err := phonenumbers.GetTimezonesForNumber(num)
	if err != nil || len(timezones) == 0 {
		return time.UTC, nil
	}

	// Use the first timezone
	loc, err := time.LoadLocation(timezones[0])
	if err != nil {
		return time.UTC, nil
	}
	return loc, nil
}
