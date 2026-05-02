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
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

type DeliveryClient struct {
	httpClient *http.Client
}

// DeliveryOption configures a DeliveryClient.
type DeliveryOption func(*deliveryConfig)

type deliveryConfig struct {
	allowPrivateIPs bool
}

// WithAllowPrivateIPs disables the private-IP guard in DialContext. ONLY for
// tests that hit httptest.NewServer (which always binds to 127.0.0.1). Never
// pass this in production: it re-opens the SSRF / DNS-rebinding hole.
func WithAllowPrivateIPs() DeliveryOption {
	return func(c *deliveryConfig) { c.allowPrivateIPs = true }
}

func NewDeliveryClient(timeout time.Duration, opts ...DeliveryOption) *DeliveryClient {
	cfg := deliveryConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	var dialFn func(ctx context.Context, network, addr string) (net.Conn, error)
	if cfg.allowPrivateIPs {
		dialFn = dialer.DialContext
	} else {
		dialFn = safeDialContext(dialer)
	}

	transport := &http.Transport{
		DialContext:         dialFn,
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

	// Replay-protected signature: timestamp + "." + payload, HMAC-SHA256.
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signPayload(timestamp, payload, sub.Secret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SMS-Platform-Webhook/1.0")
	req.Header.Set("X-Webhook-Signature", "sha256="+signature)
	req.Header.Set("X-Webhook-Timestamp", timestamp)
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

// signPayload вычисляет HMAC-SHA256(timestamp + "." + payload, secret) и возвращает hex.
//
// Replay-protected: timestamp включён в подпись, поэтому одинаковый payload в
// разное время даёт разную подпись. Атакующий, перехвативший delivery, не может
// бесконечно реплеить — после истечения окна на receiver'е его запрос отклоняется.
//
// Receiver verification — реализация на стороне receiver'а (этот пакет
// unexported, импортировать нельзя; reference показывает только идею):
//
//	ts := r.Header.Get("X-Webhook-Timestamp")
//	sig := strings.TrimPrefix(r.Header.Get("X-Webhook-Signature"), "sha256=")
//	now := time.Now().Unix()
//	tsInt, _ := strconv.ParseInt(ts, 10, 64)
//	if math.Abs(float64(now-tsInt)) > 300 { // ±5 минут
//	    return errStaleTimestamp
//	}
//	body, _ := io.ReadAll(r.Body) // ВАЖНО: raw bytes, без re-marshal'а JSON
//	mac := hmac.New(sha256.New, []byte(secret))
//	mac.Write([]byte(ts)) // ВАЖНО: raw header-string, без re-format'а
//	mac.Write([]byte("."))
//	mac.Write(body)
//	expected := hex.EncodeToString(mac.Sum(nil))
//	if !hmac.Equal([]byte(sig), []byte(expected)) {
//	    return errBadSignature
//	}
//
// Защита через X-Webhook-ID dedup на receiver'е остаётся валидной добавкой:
// X-Webhook-Timestamp окно ловит late replay, X-Webhook-ID ловит дубли в окне
// (например при retry).
func signPayload(timestamp string, payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// isUnsafeIP reports whether ip points at a private/internal/loopback/link-local
// address that webhook delivery must never reach.
func isUnsafeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}

// safeDialContext returns a DialContext that resolves the hostname at the moment
// of dialling, rejects any private/internal address (closing the DNS-rebinding
// gap between ValidateURL and the actual TCP connection), and then dials by
// IP literal. The original hostname remains in req.URL, so HTTPS SNI and the
// HTTP Host header are unaffected.
func safeDialContext(base *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// addr already an IP literal — validate and dial directly.
		if ip := net.ParseIP(host); ip != nil {
			if isUnsafeIP(ip) {
				return nil, domain.ErrPrivateURL
			}
			return base.DialContext(ctx, network, addr)
		}

		// Hostname: re-resolve here (separate call from ValidateURL to defend
		// against DNS rebinding) and reject if any returned IP is private.
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("dns lookup failed: %w", err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no IPs resolved for %s", host)
		}
		for _, ipa := range ips {
			if isUnsafeIP(ipa.IP) {
				return nil, domain.ErrPrivateURL
			}
		}

		ipAddr := net.JoinHostPort(ips[0].IP.String(), port)
		return base.DialContext(ctx, network, ipAddr)
	}
}

// ValidateURL checks that a URL is syntactically a public HTTPS endpoint.
// It does NOT resolve DNS — that responsibility belongs to safeDialContext
// at delivery time (single source of truth for IP-level checks, robust
// against DNS rebinding between create and deliver).
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
	// Block obvious private/internal hostnames.
	privateHosts := []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}
	for _, ph := range privateHosts {
		if strings.EqualFold(host, ph) {
			return domain.ErrPrivateURL
		}
	}
	// IP literal — block if private/loopback/link-local.
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return domain.ErrPrivateURL
		}
	}
	return nil
}
