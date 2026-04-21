package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
)

// HLRProviderAdapterFactory creates HLR provider adapters based on adapter type
type HLRProviderAdapterFactory struct{}

// NewHLRProviderAdapterFactory creates a new adapter factory
func NewHLRProviderAdapterFactory() *HLRProviderAdapterFactory {
	return &HLRProviderAdapterFactory{}
}

// CreateAdapter creates an adapter for the given provider configuration
func (f *HLRProviderAdapterFactory) CreateAdapter(provider *domain.HLRProvider) (domain.HLRProviderAdapter, error) {
	switch provider.AdapterType {
	// "http_rest" is the canonical value per spec 004-hlr-smart-routing/data-model.md.
	// "http" is accepted as a defensive alias for legacy rows: migration 000113 normalises
	// existing data, but this guard prevents regressions if older seeds/fixtures resurface
	// or an operator hand-edits the table.
	case "http_rest", "http":
		return NewHTTPHLRAdapter(provider)
	default:
		return nil, fmt.Errorf("неподдерживаемый тип адаптера: %s", provider.AdapterType)
	}
}

// HTTPHLRAdapter implements HLR lookup via HTTP REST API
type HTTPHLRAdapter struct {
	name       string
	baseURL    string
	apiKey     string
	timeoutMs  int
	httpClient *http.Client
}

// NewHTTPHLRAdapter creates a new HTTP HLR adapter from provider config
func NewHTTPHLRAdapter(provider *domain.HLRProvider) (*HTTPHLRAdapter, error) {
	baseURL, _ := provider.Config["base_url"].(string)
	apiKey, _ := provider.Config["api_key"].(string)
	timeoutMs := 3000
	if tm, ok := provider.Config["timeout_ms"].(float64); ok {
		timeoutMs = int(tm)
	}

	if baseURL == "" {
		return nil, fmt.Errorf("base_url не указан в конфигурации HLR провайдера %s", provider.Name)
	}

	return &HTTPHLRAdapter{
		name:      provider.Name,
		baseURL:   baseURL,
		apiKey:    apiKey,
		timeoutMs: timeoutMs,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
	}, nil
}

// Name returns the provider name
func (a *HTTPHLRAdapter) Name() string {
	return a.name
}

// Lookup performs an HLR query for a phone number
func (a *HTTPHLRAdapter) Lookup(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	url := fmt.Sprintf("%s/%s", a.baseURL, msisdn)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HLR запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HLR провайдер вернул статус %d: %s", resp.StatusCode, string(body))
	}

	var hlrResp struct {
		MSISDN         string `json:"msisdn"`
		OperatorMCCMNC string `json:"mccmnc"`
		OperatorName   string `json:"operator_name"`
		Status         string `json:"status"`
		CountryCode    string `json:"country_code"`
		NumberType     string `json:"number_type"`
		IsPorted       bool   `json:"is_ported"`
		OriginalMCCMNC string `json:"original_mccmnc"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&hlrResp); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа HLR: %w", err)
	}

	result := &domain.LookupResult{
		MSISDN:                 hlrResp.MSISDN,
		OperatorMCCMNC:         hlrResp.OperatorMCCMNC,
		OperatorName:           hlrResp.OperatorName,
		NumberStatus:           domain.NumberStatus(hlrResp.Status),
		CountryCode:            hlrResp.CountryCode,
		NumberType:             domain.NumberType(hlrResp.NumberType),
		IsPorted:               hlrResp.IsPorted,
		OriginalOperatorMCCMNC: hlrResp.OriginalMCCMNC,
		Cached:                 false,
		QueriedAt:              time.Now(),
	}

	log.Debug().
		Str("msisdn", msisdn).
		Str("operator", hlrResp.OperatorMCCMNC).
		Str("status", hlrResp.Status).
		Str("provider", a.name).
		Msg("HLR lookup выполнен")

	return result, nil
}

// Ping checks if the HLR provider is reachable
func (a *HTTPHLRAdapter) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("HLR провайдер %s вернул статус %d", a.name, resp.StatusCode)
	}
	return nil
}
