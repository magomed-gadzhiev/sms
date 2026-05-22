package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderer_SimpleVariable(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render("Привет, {{ name }}!", map[string]interface{}{"name": "Иван"})
	require.NoError(t, err)
	assert.Equal(t, "Привет, Иван!", result)
}

func TestRenderer_DefaultFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`Привет, {{ name | default: "друг" }}!`, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Привет, друг!", result)
}

func TestRenderer_TruncateFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ text | truncate: 10 }}`, map[string]interface{}{"text": "Очень длинный текст"})
	require.NoError(t, err)
	assert.Equal(t, "Очень длин...", result)
}

func TestRenderer_PhoneFormatFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ phone | phone_format }}`, map[string]interface{}{"phone": "79991234567"})
	require.NoError(t, err)
	assert.Equal(t, "+7 (999) 123-45-67", result)
}

func TestRenderer_DateFilter(t *testing.T) {
	r := NewRenderer()
	result, err := r.Render(`{{ order_date | date: "%d.%m.%Y" }}`, map[string]interface{}{"order_date": "2026-03-28T10:00:00Z"})
	require.NoError(t, err)
	assert.Equal(t, "28.03.2026", result)
}

func TestRenderer_ConditionalLogic(t *testing.T) {
	r := NewRenderer()

	result, err := r.Render(`{% if vip %}VIP: {{ discount }}%{% endif %}`, map[string]interface{}{"vip": true, "discount": 20})
	require.NoError(t, err)
	assert.Equal(t, "VIP: 20%", result)

	result, err = r.Render(`{% if vip %}VIP{% endif %}`, map[string]interface{}{"vip": false})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestRenderer_OutputLimitExceeded(t *testing.T) {
	r := NewRenderer()
	// Template that produces output > 1600 characters
	longValue := make([]byte, 2000)
	for i := range longValue {
		longValue[i] = 'A'
	}
	_, err := r.Render("{{ text }}", map[string]interface{}{"text": string(longValue)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output exceeds maximum")
}

func TestRenderer_InvalidSyntax(t *testing.T) {
	r := NewRenderer()
	_, err := r.Render("{% if true %}unclosed if", map[string]interface{}{})
	require.Error(t, err)
}
