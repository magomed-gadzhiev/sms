package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	client := NewDeliveryClient(5 * time.Second)
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
	var signatureHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signatureHeader = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDeliveryClient(5 * time.Second)
	sub := testSubscription(server.URL)
	event := testEvent()

	err := client.Deliver(context.Background(), sub, event)
	require.NoError(t, err)

	// Verify signature format
	assert.True(t, len(signatureHeader) > 7)
	assert.Equal(t, "sha256=", signatureHeader[:7])

	// Verify signature content matches expected HMAC
	payload, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(sub.Secret))
	mac.Write(payload)
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

	client := NewDeliveryClient(5 * time.Second)
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

			client := NewDeliveryClient(5 * time.Second)
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

			client := NewDeliveryClient(5 * time.Second)
			err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
			require.NoError(t, err)
		})
	}
}

func TestDeliver_ConnectionRefused(t *testing.T) {
	client := NewDeliveryClient(2 * time.Second)
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

	client := NewDeliveryClient(10 * time.Second)
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

	client := NewDeliveryClient(5 * time.Second)
	err := client.Deliver(context.Background(), testSubscription(server.URL), testEvent())
	// 302 is not 2xx, so it should be an error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook delivery failed")
}

// ─── signPayload ────────────────────────────────────────────────────────────

func TestSignPayload(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "my-secret"

	sig := signPayload(payload, secret)

	// Verify manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestSignPayload_EmptySecret(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	sig := signPayload(payload, "")
	assert.NotEmpty(t, sig) // HMAC with empty key still produces output
}

func TestSignPayload_DifferentSecretsProduceDifferentSignatures(t *testing.T) {
	payload := []byte(`{"same":"payload"}`)
	sig1 := signPayload(payload, "secret-1")
	sig2 := signPayload(payload, "secret-2")
	assert.NotEqual(t, sig1, sig2)
}

func TestSignPayload_DifferentPayloadsProduceDifferentSignatures(t *testing.T) {
	secret := "shared-secret"
	sig1 := signPayload([]byte(`{"a":1}`), secret)
	sig2 := signPayload([]byte(`{"b":2}`), secret)
	assert.NotEqual(t, sig1, sig2)
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
	client := NewDeliveryClient(15 * time.Second)
	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 15*time.Second, client.httpClient.Timeout)
}
