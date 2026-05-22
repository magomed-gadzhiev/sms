package maxmessenger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
)

// deliveryContextKey — ключ для хранения Delivery в контексте
type deliveryContextKey struct{}

// WithDelivery добавляет Delivery в контекст
func WithDelivery(ctx context.Context, d *domain.Delivery) context.Context {
	return context.WithValue(ctx, deliveryContextKey{}, d)
}

const (
	maxRetries     = 3
	baseRetryDelay = 1 * time.Second
)

// Adapter — Max Messenger адаптер, отправляет сообщения через Max Bot API
type Adapter struct {
	httpClient *http.Client
	logger     zerolog.Logger
	metrics    *MaxMessengerMetrics
}

// NewAdapter создаёт новый Max Messenger адаптер
func NewAdapter(logger zerolog.Logger, metrics *MaxMessengerMetrics) *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger.With().Str("component", "max_messenger_adapter").Logger(),
		metrics:    metrics,
	}
}

// Type возвращает тип канала
func (a *Adapter) Type() domain.ChannelType {
	return domain.ChannelMaxMessenger
}

// Send отправляет сообщение через Max Bot API.
// Результат доставки приходит асинхронно через webhook.
func (a *Adapter) Send(ctx context.Context, attempt *domain.DeliveryAttempt, cfg *domain.ChannelConfig) error {
	if cfg == nil {
		return fmt.Errorf("max messenger channel config is required")
	}

	delivery, ok := ctx.Value(deliveryContextKey{}).(*domain.Delivery)
	if !ok || delivery == nil {
		return fmt.Errorf("delivery not found in context")
	}

	providerURL, _ := cfg.Config["provider_url"].(string)
	if providerURL == "" {
		return fmt.Errorf("max_messenger provider_url not configured")
	}

	apiKey, _ := cfg.Config["api_key"].(string)
	if apiKey == "" {
		return fmt.Errorf("max_messenger api_key not configured")
	}

	callbackURL, _ := cfg.Config["callback_url"].(string)

	payload := SendRequest{
		RecipientMSISDN: delivery.Recipient,
		Text:            delivery.Text,
		CallbackURL:     callbackURL,
		ExternalID:      attempt.ID.String(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal max messenger payload: %w", err)
	}

	a.metrics.ActiveAttempts.Inc()
	defer a.metrics.ActiveAttempts.Dec()

	start := time.Now()
	err = a.sendWithRetry(ctx, providerURL, apiKey, body, attempt)
	elapsed := time.Since(start).Seconds()

	if err != nil {
		a.metrics.SendLatency.WithLabelValues("error").Observe(elapsed)
		return err
	}

	a.metrics.SendLatency.WithLabelValues("success").Observe(elapsed)

	a.logger.Debug().
		Str("attempt_id", attempt.ID.String()).
		Str("delivery_id", attempt.DeliveryID.String()).
		Str("recipient", maskMSISDN(delivery.Recipient)).
		Str("request_id", delivery.RequestID).
		Msg("max messenger сообщение отправлено")

	return nil
}

func (a *Adapter) sendWithRetry(ctx context.Context, providerURL, apiKey string, body []byte, attempt *domain.DeliveryAttempt) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			delay := baseRetryDelay * time.Duration(math.Pow(2, float64(i-1)))
			a.logger.Warn().
				Int("retry", i).
				Dur("delay", delay).
				Str("attempt_id", attempt.ID.String()).
				Msg("max messenger retry после 429")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, providerURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create max messenger request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := a.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("max messenger provider request: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			a.metrics.SendLatency.WithLabelValues("rate_limited").Observe(0)
			lastErr = fmt.Errorf("max messenger rate limited (429)")
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("max messenger provider error %d: %s", resp.StatusCode, string(respBody))
		}

		return nil
	}

	return fmt.Errorf("max messenger retries exhausted: %w", lastErr)
}

// ValidateConfig проверяет обязательные поля конфигурации канала Max Messenger
func ValidateConfig(cfg map[string]interface{}) error {
	required := []string{"provider_url", "api_key", "bot_id", "webhook_secret", "check_registration_url"}
	for _, key := range required {
		val, ok := cfg[key].(string)
		if !ok || val == "" {
			return fmt.Errorf("missing required field: %s", key)
		}
	}

	// Validate HTTPS URLs
	for _, key := range []string{"provider_url", "check_registration_url"} {
		raw := cfg[key].(string)
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid URL for %s: %w", key, err)
		}
		if u.Scheme != "https" {
			return fmt.Errorf("%s must use HTTPS", key)
		}
	}

	// webhook_secret минимум 32 символа
	secret := cfg["webhook_secret"].(string)
	if len(secret) < 32 {
		return fmt.Errorf("webhook_secret must be at least 32 characters")
	}

	return nil
}

func maskMSISDN(msisdn string) string {
	if len(msisdn) <= 4 {
		return strings.Repeat("*", len(msisdn))
	}
	return msisdn[:2] + strings.Repeat("*", len(msisdn)-4) + msisdn[len(msisdn)-2:]
}
