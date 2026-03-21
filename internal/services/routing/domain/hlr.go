package domain

import (
	"time"

	"github.com/google/uuid"
)

// NumberStatus represents the status of a phone number from HLR lookup
type NumberStatus string

const (
	NumberStatusActive  NumberStatus = "active"
	NumberStatusAbsent  NumberStatus = "absent"
	NumberStatusInvalid NumberStatus = "invalid"
	NumberStatusUnknown NumberStatus = "unknown"
)

// NumberType represents the type of phone number
type NumberType string

const (
	NumberTypeMobile NumberType = "mobile"
	NumberTypeFixed  NumberType = "fixed"
	NumberTypeVoip   NumberType = "voip"
)

// HLRProviderStatus represents the health status of an HLR provider
type HLRProviderStatus string

const (
	HLRProviderStatusHealthy   HLRProviderStatus = "healthy"
	HLRProviderStatusDegraded  HLRProviderStatus = "degraded"
	HLRProviderStatusUnhealthy HLRProviderStatus = "unhealthy"
	HLRProviderStatusDisabled  HLRProviderStatus = "disabled"
)

// LookupResult represents the result of an HLR lookup
type LookupResult struct {
	MSISDN                 string
	OperatorMCCMNC         string
	OperatorName           string
	NumberStatus           NumberStatus
	CountryCode            string
	NumberType             NumberType
	IsPorted               bool
	OriginalOperatorMCCMNC string
	HLRProviderID          *uuid.UUID
	Cached                 bool
	QueriedAt              time.Time
}

// IsDeliverable returns true if the number can receive SMS
func (r *LookupResult) IsDeliverable() bool {
	return r.NumberStatus == NumberStatusActive || r.NumberStatus == NumberStatusAbsent || r.NumberStatus == NumberStatusUnknown
}

// IsInvalid returns true if the number is invalid and should not be sent to
func (r *LookupResult) IsInvalid() bool {
	return r.NumberStatus == NumberStatusInvalid
}

// HLRProvider represents an external HLR lookup provider configuration
type HLRProvider struct {
	ID               uuid.UUID
	Name             string
	AdapterType      string
	Config           map[string]interface{}
	Priority         int
	SupportedRegions []string
	CostPerLookup    float64
	Status           HLRProviderStatus
	SuccessRate      float64
	LastSuccessAt    *time.Time
	LastFailureAt    *time.Time
	Active           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewHLRProvider creates a new HLR provider
func NewHLRProvider(name, adapterType string, config map[string]interface{}, priority int, supportedRegions []string, costPerLookup float64) *HLRProvider {
	now := time.Now()
	return &HLRProvider{
		ID:               uuid.New(),
		Name:             name,
		AdapterType:      adapterType,
		Config:           config,
		Priority:         priority,
		SupportedRegions: supportedRegions,
		CostPerLookup:    costPerLookup,
		Status:           HLRProviderStatusHealthy,
		SuccessRate:      100.0,
		Active:           true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// IsAvailable returns true if provider can accept lookup requests
func (p *HLRProvider) IsAvailable() bool {
	return p.Active && p.Status != HLRProviderStatusUnhealthy && p.Status != HLRProviderStatusDisabled
}

// SupportsRegion checks if the provider supports a given country code
func (p *HLRProvider) SupportsRegion(countryCode string) bool {
	for _, region := range p.SupportedRegions {
		if region == countryCode {
			return true
		}
	}
	return false
}

// UpdateStatus updates provider status based on success rate
func (p *HLRProvider) UpdateStatus(successRate float64) {
	p.SuccessRate = successRate
	switch {
	case successRate >= 95.0:
		p.Status = HLRProviderStatusHealthy
	case successRate >= 80.0:
		p.Status = HLRProviderStatusDegraded
	default:
		p.Status = HLRProviderStatusUnhealthy
	}
	p.UpdatedAt = time.Now()
}

// RecordSuccess records a successful HLR query
func (p *HLRProvider) RecordSuccess() {
	now := time.Now()
	p.LastSuccessAt = &now
	p.UpdatedAt = now
}

// RecordFailure records a failed HLR query
func (p *HLRProvider) RecordFailure() {
	now := time.Now()
	p.LastFailureAt = &now
	p.UpdatedAt = now
}

// Disable disables the provider (admin action)
func (p *HLRProvider) Disable() {
	p.Active = false
	p.Status = HLRProviderStatusDisabled
	p.UpdatedAt = time.Now()
}

// Enable enables the provider and resets metrics (admin action)
func (p *HLRProvider) Enable() {
	p.Active = true
	p.Status = HLRProviderStatusHealthy
	p.SuccessRate = 100.0
	p.UpdatedAt = time.Now()
}

// MaskedConfig возвращает конфигурацию с замаскированными чувствительными полями
// (API keys, пароли) для безопасного логирования (Constitution V: Data Safety)
func (p *HLRProvider) MaskedConfig() map[string]interface{} {
	sensitiveKeys := map[string]bool{
		"api_key": true, "password": true, "secret": true, "token": true,
		"apikey": true, "api_secret": true, "access_key": true,
	}
	masked := make(map[string]interface{}, len(p.Config))
	for k, v := range p.Config {
		if sensitiveKeys[k] {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}
	return masked
}

// LookupSource represents the source of a lookup request
type LookupSource string

const (
	LookupSourceSMSRouting LookupSource = "sms_routing"
	LookupSourceAPILookup  LookupSource = "api_lookup"
)

// LookupLogEntry represents an audit log entry for an HLR lookup
type LookupLogEntry struct {
	ID              uuid.UUID
	MSISDN          string
	OperatorMCCMNC  string
	OperatorName    string
	NumberStatus    string
	CountryCode     string
	NumberType      string
	IsPorted        bool
	HLRProviderID   *uuid.UUID
	Source          LookupSource
	ClientID        uuid.UUID
	Cached          bool
	LatencyMs       int
	RequestID       string
	MessageID       *uuid.UUID
	CreatedAt       time.Time
}

// NewLookupLogEntry creates a new lookup log entry from a LookupResult
func NewLookupLogEntry(result *LookupResult, source LookupSource, clientID uuid.UUID, requestID string, messageID *uuid.UUID, latencyMs int) *LookupLogEntry {
	return &LookupLogEntry{
		ID:             uuid.New(),
		MSISDN:         result.MSISDN,
		OperatorMCCMNC: result.OperatorMCCMNC,
		OperatorName:   result.OperatorName,
		NumberStatus:   string(result.NumberStatus),
		CountryCode:    result.CountryCode,
		NumberType:     string(result.NumberType),
		IsPorted:       result.IsPorted,
		HLRProviderID:  result.HLRProviderID,
		Source:         source,
		ClientID:       clientID,
		Cached:         result.Cached,
		LatencyMs:      latencyMs,
		RequestID:      requestID,
		MessageID:      messageID,
		CreatedAt:      time.Now(),
	}
}

