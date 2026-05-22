package flashcall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// Adapter — Flash Call адаптер, инициирует звонок через HTTP провайдера
type Adapter struct {
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewAdapter создаёт новый Flash Call адаптер
func NewAdapter(logger zerolog.Logger) *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger.With().Str("component", "flash_call_adapter").Logger(),
	}
}

// Type возвращает тип канала
func (a *Adapter) Type() domain.ChannelType {
	return domain.ChannelFlashCall
}

// Send инициирует flash call через HTTP провайдера.
// Результат доставки приходит асинхронно через webhook.
func (a *Adapter) Send(ctx context.Context, attempt *domain.DeliveryAttempt, cfg *domain.ChannelConfig) error {
	if cfg == nil {
		return fmt.Errorf("flash call channel config is required")
	}

	delivery, ok := ctx.Value(deliveryContextKey{}).(*domain.Delivery)
	if !ok || delivery == nil {
		return fmt.Errorf("delivery not found in context")
	}

	providerURL, _ := cfg.Config["provider_url"].(string)
	if providerURL == "" {
		return fmt.Errorf("flash_call provider_url not configured")
	}

	payload := map[string]interface{}{
		"attempt_id": attempt.ID.String(),
		"msisdn":     delivery.Recipient,
		"text":       delivery.Text,
		"request_id": delivery.RequestID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal flash call payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, providerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create flash call request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if apiKey, ok := cfg.Config["api_key"].(string); ok && apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("flash call provider request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("flash call provider error %d: %s", resp.StatusCode, string(respBody))
	}

	a.logger.Debug().
		Str("attempt_id", attempt.ID.String()).
		Str("delivery_id", attempt.DeliveryID.String()).
		Str("recipient", delivery.Recipient).
		Int("status_code", resp.StatusCode).
		Msg("flash call инициирован")

	return nil
}
