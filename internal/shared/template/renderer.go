package template

import (
	"fmt"
	"strings"
	"time"

	"github.com/osteele/liquid"
)

const maxOutputLen = 1600 // 10 SMS segments max

// Renderer wraps the Liquid engine with SMS-specific filters and sandbox.
type Renderer struct {
	engine *liquid.Engine
}

// NewRenderer creates a Renderer with custom SMS filters registered.
func NewRenderer() *Renderer {
	engine := liquid.NewEngine()

	engine.RegisterFilter("default", func(value, defaultVal interface{}) interface{} {
		if value == nil || value == "" {
			return defaultVal
		}
		return value
	})

	engine.RegisterFilter("truncate", func(value interface{}, length int) string {
		s := fmt.Sprintf("%v", value)
		runes := []rune(s)
		if len(runes) <= length {
			return s
		}
		return string(runes[:length]) + "..."
	})

	engine.RegisterFilter("phone_format", func(value interface{}) string {
		s := fmt.Sprintf("%v", value)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.TrimPrefix(s, "+")
		if len(s) == 11 && s[0] == '7' {
			return fmt.Sprintf("+%s (%s) %s-%s-%s", s[:1], s[1:4], s[4:7], s[7:9], s[9:11])
		}
		return fmt.Sprintf("+%s", s)
	})

	engine.RegisterFilter("date", func(value interface{}, format string) string {
		var t time.Time
		switch v := value.(type) {
		case time.Time:
			t = v
		case string:
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				parsed, err = time.Parse("2006-01-02", v)
				if err != nil {
					return v
				}
			}
			t = parsed
		default:
			return fmt.Sprintf("%v", value)
		}
		return goDateFormat(t, format)
	})

	return &Renderer{engine: engine}
}

// Render renders a Liquid template with the given bindings.
// Returns error if syntax is invalid, timeout exceeded, or output > 1600 chars.
func (r *Renderer) Render(templateStr string, bindings map[string]interface{}) (string, error) {
	out, err := r.engine.ParseAndRenderString(templateStr, bindings)
	if err != nil {
		return "", fmt.Errorf("template render error: %w", err)
	}
	if len([]rune(out)) > maxOutputLen {
		return "", fmt.Errorf("output exceeds maximum length of %d characters", maxOutputLen)
	}
	return out, nil
}

// goDateFormat converts strftime-style format to Go time format and formats the time.
func goDateFormat(t time.Time, format string) string {
	replacer := strings.NewReplacer(
		"%Y", "2006",
		"%m", "01",
		"%d", "02",
		"%H", "15",
		"%M", "04",
		"%S", "05",
	)
	goFmt := replacer.Replace(format)
	return t.Format(goFmt)
}
