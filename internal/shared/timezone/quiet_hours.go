// internal/shared/timezone/quiet_hours.go
package timezone

import "time"

// QuietHoursConfig represents quiet hours settings for a client.
type QuietHoursConfig struct {
	Enabled   bool
	StartHour int    // 0-23
	EndHour   int    // 0-23
	Action    string // "postpone" or "skip"
}

// QuietHoursResult holds the result of a quiet hours check.
type QuietHoursResult struct {
	IsQuiet    bool
	Action     string
	PostponeTo time.Time // Only set if Action == "postpone"
}

// CheckQuietHours checks if the given local time falls within quiet hours.
func CheckQuietHours(localTime time.Time, config QuietHoursConfig) QuietHoursResult {
	if !config.Enabled {
		return QuietHoursResult{IsQuiet: false}
	}

	hour := localTime.Hour()
	isQuiet := false

	if config.StartHour > config.EndHour {
		// Overnight: e.g., 22-8
		isQuiet = hour >= config.StartHour || hour < config.EndHour
	} else if config.StartHour < config.EndHour {
		// Daytime: e.g., 13-15
		isQuiet = hour >= config.StartHour && hour < config.EndHour
	}

	if !isQuiet {
		return QuietHoursResult{IsQuiet: false}
	}

	result := QuietHoursResult{
		IsQuiet: true,
		Action:  config.Action,
	}

	if config.Action == "postpone" {
		// Calculate next end_hour
		postponeTo := time.Date(localTime.Year(), localTime.Month(), localTime.Day(),
			config.EndHour, 0, 0, 0, localTime.Location())
		if postponeTo.Before(localTime) || postponeTo.Equal(localTime) {
			postponeTo = postponeTo.Add(24 * time.Hour)
		}
		result.PostponeTo = postponeTo
	}

	return result
}
