package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/webhook/domain"
)

// helpers ────────────────────────────────────────────────────────────────────

func testSubscription(url string) *domain.Subscription {
	return &domain.Subscription{
		URL:    url,
		Secret: "test-secret-key",
	}
}

func testEvent() *domain.WebhookEvent {
	return &domain.WebhookEvent{
		EventID:   "evt-123",
		EventType: "delivered",
		Timestamp: time.Now(),
		Data: domain.EventData{
			MessageID:   "msg-456",
			Source:      "MySender",
			Destination: "+1234567890",
			Status:      "delivered",
		},
	}
}

// ─── Deliver: happy path ────────────────────────────────────────────────────

func TestDeliver_Success(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
	sub := testSubscription(server.URL)
	event := testEvent()

	err := client.Deliver(context.Background(), sub, event)
	require.NoError(t, err)

	// Verify request body is valid JSON of the event
	var decoded domain.WebhookEvent
	require.NoError(t, json.Unmarshal(receivedBody, &decoded))
	assert.Equal(t, "evt-123", decoded.EventID)
	assert.Equal(t, "delivered", decoded.EventType)
	assert.Equal(t, "msg-456", decoded.Data.MessageID)

	// Verify headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "SMS-Platform-Webhook/1.0", receivedHeaders.Get("User-Agent"))
	assert.Equal(t, "delivered", receivedHeaders.Get("X-Webhook-Event"))
	assert.Equal(t, "evt-123", receivedHeaders.Get("X-Webhook-ID"))
}

func TestDeliver_HMACSignature(t *testing.T) {
	var signatureHeader, timestampHeader string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signatureHeader = r.Header.Get("X-Webhook-Signature")
		timestampHeader = r.Header.Get("X-Webhook-Timestamp")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
	sub := testSubscription(server.URL)
	event := testEvent()

	beforeUnix := time.Now().Unix()
	err := client.Deliver(context.Background(), sub, event)
	require.NoError(t, err)
	afterUnix := time.Now().Unix()

	// Timestamp header in request window.
	require.NotEmpty(t, timestampHeader)
	tsInt, err := strconv.ParseInt(timestampHeader, 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, tsInt, beforeUnix)
	assert.LessOrEqual(t, tsInt, afterUnix)

	// Signature header format and replay-protected content.
	require.True(t, len(signatureHeader) > 7)
	assert.Equal(t, "sha256=", signatureHeader[:7])

	mac := hmac.New(sha256.New, []byte(sub.Secret))
	mac.Write([]byte(timestampHeader))
	mac.Write([]byte("."))
	mac.Write(receivedBody)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expectedSig, signatureHeader)
}

func TestDeliver_UsesPostMethod(t *testing.T) {
	var method string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
	err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, method)
}

// ─── Deliver: error cases ───────────────────────────────────────────────────

func TestDeliver_Non2xxStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
			err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "webhook delivery failed")
		})
	}
}

func TestDeliver_2xxStatusCodes(t *testing.T) {
	// All 2xx codes should be treated as success
	for code := 200; code <= 204; code++ {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
			err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
			require.NoError(t, err)
		})
	}
}

func TestDeliver_ConnectionRefused(t *testing.T) {
	client := NewDeliveryClient(2*time.Second, WithAllowPrivateIPs())
	// Connect to a port that's certainly not listening
	sub := testSubscription("https://127.0.0.1:1")
	err := client.Deliver(context.Background(), sub, testEvent())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deliver webhook")
}

func TestDeliver_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // slow server
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(10*time.Second, WithAllowPrivateIPs())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.Deliver(ctx, testSubscription(server.URL), testEvent())
	require.Error(t, err)
}

func TestDeliver_DoesNotFollowRedirects(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("redirect should not be followed")
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer server.Close()

	client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
	err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
	// 302 is not 2xx, so it should be an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook delivery failed")
}

// ─── signPayload ────────────────────────────────────────────────────────────

func TestSignPayload(t *testing.T) {
	timestamp := "1700000000"
	payload := []byte(`{"event":"test"}`)
	secret := "my-secret"

	sig := signPayload(timestamp, payload, secret)

	// Verify manually: HMAC over (timestamp + "." + payload).
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestSignPayload_EmptySecret(t *testing.T) {
	sig := signPayload("1700000000", []byte(`{"event":"test"}`), "")
	assert.NotEmpty(t, sig) // HMAC with empty key still produces output
}

func TestSignPayload_DifferentSecretsProduceDifferentSignatures(t *testing.T) {
	ts := "1700000000"
	payload := []byte(`{"same":"payload"}`)
	sig1 := signPayload(ts, payload, "secret-1")
	sig2 := signPayload(ts, payload, "secret-2")
	assert.NotEqual(t, sig1, sig2)
}

func TestSignPayload_DifferentPayloadsProduceDifferentSignatures(t *testing.T) {
	ts := "1700000000"
	secret := "shared-secret"
	sig1 := signPayload(ts, []byte(`{"a":1}`), secret)
	sig2 := signPayload(ts, []byte(`{"b":2}`), secret)
	assert.NotEqual(t, sig1, sig2)
}

// TestSignPayload_DifferentTimestampsProduceDifferentSignatures —
// replay-protection regression-guard. Подпись от (ts1, payload, secret) НЕ
// должна совпадать с подписью от (ts2, payload, secret) для ts1 != ts2.
// Это гарантирует, что перехваченный delivery нельзя реплеить с подменённым
// timestamp (receiver проверит окно ±5мин и отклонит stale).
func TestSignPayload_DifferentTimestampsProduceDifferentSignatures(t *testing.T) {
	payload := []byte(`{"event":"sms.delivered"}`)
	secret := "shared-secret"
	sig1 := signPayload("1700000000", payload, secret)
	sig2 := signPayload("1700000060", payload, secret) // на 60 секунд позже
	assert.NotEqual(t, sig1, sig2, "timestamp обязан входить в подпись — иначе replay тривиален")
}

// TestDeliver_DistinctTimestampsAndSignatures — end-to-end replay-protection
// regression-guard: два Deliver-вызова с одним event дают РАЗНЫЕ подписи
// (потому что timestamps отличаются). До A2 fix'а оба вызова давали
// идентичную X-Webhook-Signature, что позволяло replay.
func TestDeliver_DistinctTimestampsAndSignatures(t *testing.T) {
	type capture struct {
		sig string
		ts  string
	}
	var captured []capture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, capture{
			sig: r.Header.Get("X-Webhook-Signature"),
			ts:  r.Header.Get("X-Webhook-Timestamp"),
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(5*time.Second, WithAllowPrivateIPs())
	sub := testSubscription(server.URL)
	event := testEvent()

	require.NoError(t, client.Deliver(context.Background(), sub, event))
	// 2-секундный sleep гарантирует пересечение unix-секундной границы даже с
	// учётом до 1с scheduler/GC-slip на нагруженном CI (1.1с было слишком тонко).
	// TODO: лучшее долгосрочное решение — инжекция clock-интерфейса в
	// DeliveryClient, чтобы тест мог детерминистично продвинуть время без sleep.
	time.Sleep(2 * time.Second)
	require.NoError(t, client.Deliver(context.Background(), sub, event))

	require.Len(t, captured, 2)
	require.NotEmpty(t, captured[0].ts)
	require.NotEmpty(t, captured[1].ts)
	assert.NotEqual(t, captured[0].ts, captured[1].ts, "timestamps должны различаться")
	assert.NotEqual(t, captured[0].sig, captured[1].sig, "подписи должны различаться (replay-protected)")
}

// ─── ValidateURL ────────────────────────────────────────────────────────────

func TestValidateURL_ValidHTTPS(t *testing.T) {
	assert.NoError(t, ValidateURL("https://example.com/webhook"))
	assert.NoError(t, ValidateURL("https://api.example.com:8443/path"))
	assert.NoError(t, ValidateURL("https://subdomain.example.com/a/b/c?q=1"))
}

func TestValidateURL_RejectsHTTP(t *testing.T) {
	err := ValidateURL("http://example.com/webhook")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidURL)
}

func TestValidateURL_RejectsNonHTTPSchemes(t *testing.T) {
	schemes := []string{
		"ftp://example.com",
		"ws://example.com",
		"file:///etc/passwd",
	}
	for _, u := range schemes {
		t.Run(u, func(t *testing.T) {
			err := ValidateURL(u)
			require.Error(t, err)
		})
	}
}

func TestValidateURL_RejectsPrivateHosts(t *testing.T) {
	hosts := []string{
		"https://localhost/webhook",
		"https://127.0.0.1/webhook",
		"https://0.0.0.0/webhook",
		"https://[::1]/webhook",
	}
	for _, u := range hosts {
		t.Run(u, func(t *testing.T) {
			err := ValidateURL(u)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "private")
		})
	}
}

func TestValidateURL_RejectsPrivateIPs(t *testing.T) {
	ips := []string{
		"https://10.0.0.1/webhook",
		"https://172.16.0.1/webhook",
		"https://192.168.1.1/webhook",
		"https://169.254.1.1/webhook", // link-local
	}
	for _, u := range ips {
		t.Run(u, func(t *testing.T) {
			err := ValidateURL(u)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "private")
		})
	}
}

func TestValidateURL_RejectsInvalidURL(t *testing.T) {
	err := ValidateURL("://not-a-url")
	require.Error(t, err)
}

func TestValidateURL_AcceptsPublicIPs(t *testing.T) {
	assert.NoError(t, ValidateURL("https://8.8.8.8/webhook"))
	assert.NoError(t, ValidateURL("https://1.1.1.1/path"))
}

// ─── NewDeliveryClient ──────────────────────────────────────────────────────

func TestNewDeliveryClient_SetsTimeout(t *testing.T) {
	client := NewDeliveryClient(15*time.Second, WithAllowPrivateIPs())
	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 15*time.Second, client.httpClient.Timeout)
}

// ─── safeDialContext (DNS-rebinding guard) ──────────────────────────────────

func TestSafeDialContext_RejectsLoopbackIPLiteral(t *testing.T) {
	dial := safeDialContext(&net.Dialer{Timeout: time.Second})
	_, err := dial(context.Background(), "tcp", "127.0.0.1:1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrPrivateURL),
		"expected ErrPrivateURL, got %v", err)
}

func TestSafeDialContext_RejectsRFC1918IPLiterals(t *testing.T) {
	dial := safeDialContext(&net.Dialer{Timeout: time.Second})
	addrs := []string{
		"10.0.0.1:443",
		"172.16.0.1:443",
		"192.168.1.1:443",
		"169.254.169.254:80", // AWS/GCP metadata
		"[::1]:443",
	}
	for _, addr := range addrs {
		t.Run(addr, func(t *testing.T) {
			_, err := dial(context.Background(), "tcp", addr)
			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrPrivateURL),
				"expected ErrPrivateURL, got %v", err)
		})
	}
}

func TestSafeDialContext_RejectsHostnameResolvingToLoopback(t *testing.T) {
	dial := safeDialContext(&net.Dialer{Timeout: time.Second})
	// localhost resolves to 127.0.0.1 / ::1 — must be rejected at dial time.
	_, err := dial(context.Background(), "tcp", "localhost:443")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrPrivateURL),
		"expected ErrPrivateURL, got %v", err)
}

func TestNewDeliveryClient_DefaultRejectsLoopback(t *testing.T) {
	// Without WithAllowPrivateIPs the default DialContext blocks 127.0.0.1.
	client := NewDeliveryClient(2 * time.Second)
	sub := testSubscription("https://127.0.0.1:1")
	err := client.Deliver(context.Background(), sub, testEvent())
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrPrivateURL),
		"expected ErrPrivateURL, got %v", err)
}
