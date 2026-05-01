package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type DeliveryClient struct {
	httpClient *http.Client
}

func NewDeliveryClient(timeout time.Duration) *DeliveryClient {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &DeliveryClient{
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // do not follow redirects (SSRF prevention)
			},
		},
	}
}

// Deliver sends a webhook event to the subscription URL
func (c *DeliveryClient) Deliver(ctx context.Context, sub *domain.Subscription, event *domain.WebhookEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Sign payload with HMAC-SHA256
	signature := signPayload(payload, sub.Secret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SMS-Platform-Webhook/1.0")
	req.Header.Set("X-Webhook-Signature", "sha256="+signature)
	req.Header.Set("X-Webhook-Event", event.EventType)
	req.Header.Set("X-Webhook-ID", event.EventID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deliver webhook: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		log.Debug().Err(err).Msg("webhook response body drain error")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook delivery failed: HTTP %d", resp.StatusCode)
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateURL checks that a URL is HTTPS and not targeting private IPs
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return domain.ErrInvalidURL
	}
	if len(rawURL) > 2048 {
		return domain.ErrURLTooLong
	}

	host := u.Hostname()
	// Block private/internal ranges
	privateHosts := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, ph := range privateHosts {
		if strings.EqualFold(host, ph) {
			return domain.ErrPrivateURL
		}
	}
	// Block 10.x, 172.16-31.x, 192.168.x, 169.254.x
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return domain.ErrPrivateURL
	}
	return nil
}
