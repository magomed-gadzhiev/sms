package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrors(t *testing.T) {
	assert.EqualError(t, ErrLinkNotFound, "short link not found")
	assert.EqualError(t, ErrDomainNotFound, "domain not found")
	assert.EqualError(t, ErrDomainExists, "domain already exists")
	assert.EqualError(t, ErrCodeExists, "short code already exists")
	assert.EqualError(t, ErrLinkExpired, "short link has expired")
}

func TestErrorsAreDistinct(t *testing.T) {
	errs := []error{ErrLinkNotFound, ErrDomainNotFound, ErrDomainExists, ErrCodeExists, ErrLinkExpired}
	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			assert.NotEqual(t, errs[i], errs[j], "errors %d and %d should be distinct", i, j)
		}
	}
}

func TestDefaultDomain(t *testing.T) {
	assert.Equal(t, "go.sms-platform.com", DefaultDomain)
}

func TestClientDomain_ZeroValue(t *testing.T) {
	var cd ClientDomain
	assert.Equal(t, uuid.Nil, cd.ID)
	assert.Equal(t, uuid.Nil, cd.ClientID)
	assert.Equal(t, "", cd.Domain)
	assert.Equal(t, "", cd.Status)
	assert.Equal(t, "", cd.DNSTxtRecord)
	assert.Nil(t, cd.DNSVerifiedAt)
	assert.Equal(t, "", cd.SSLCertPath)
	assert.Nil(t, cd.SSLExpiresAt)
	assert.True(t, cd.CreatedAt.IsZero())
	assert.True(t, cd.UpdatedAt.IsZero())
}

func TestClientDomain_FullyPopulated(t *testing.T) {
	now := time.Now()
	verifiedAt := now.Add(-24 * time.Hour)
	sslExpires := now.Add(365 * 24 * time.Hour)

	cd := ClientDomain{
		ID:            uuid.New(),
		ClientID:      uuid.New(),
		Domain:        "links.example.com",
		Status:        "verified",
		DNSTxtRecord:  "sms-verify=abc123",
		DNSVerifiedAt: &verifiedAt,
		SSLCertPath:   "/certs/example.com.pem",
		SSLExpiresAt:  &sslExpires,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.NotEqual(t, uuid.Nil, cd.ID)
	assert.Equal(t, "links.example.com", cd.Domain)
	assert.Equal(t, "verified", cd.Status)
	assert.Equal(t, "sms-verify=abc123", cd.DNSTxtRecord)
	require.NotNil(t, cd.DNSVerifiedAt)
	assert.Equal(t, verifiedAt, *cd.DNSVerifiedAt)
	assert.Equal(t, "/certs/example.com.pem", cd.SSLCertPath)
	require.NotNil(t, cd.SSLExpiresAt)
	assert.Equal(t, sslExpires, *cd.SSLExpiresAt)
}

func TestClientDomain_PendingStatus(t *testing.T) {
	cd := ClientDomain{
		Status:        "pending",
		DNSVerifiedAt: nil,
		SSLExpiresAt:  nil,
	}

	assert.Equal(t, "pending", cd.Status)
	assert.Nil(t, cd.DNSVerifiedAt)
	assert.Nil(t, cd.SSLExpiresAt)
}

func TestShortLink_ZeroValue(t *testing.T) {
	var sl ShortLink
	assert.Equal(t, uuid.Nil, sl.ID)
	assert.Equal(t, uuid.Nil, sl.ClientID)
	assert.Nil(t, sl.DomainID)
	assert.Equal(t, "", sl.Code)
	assert.Equal(t, "", sl.OriginalURL)
	assert.Nil(t, sl.MessageID)
	assert.Nil(t, sl.CampaignID)
	assert.Nil(t, sl.RecipientID)
	assert.Nil(t, sl.ExpiresAt)
	assert.True(t, sl.CreatedAt.IsZero())
}

func TestShortLink_FullyPopulated(t *testing.T) {
	now := time.Now()
	expires := now.Add(7 * 24 * time.Hour)
	domainID := uuid.New()
	messageID := uuid.New()
	campaignID := uuid.New()
	recipientID := uuid.New()

	sl := ShortLink{
		ID:          uuid.New(),
		ClientID:    uuid.New(),
		DomainID:    &domainID,
		Code:        "abc123",
		OriginalURL: "https://example.com/long-url?param=value",
		MessageID:   &messageID,
		CampaignID:  &campaignID,
		RecipientID: &recipientID,
		ExpiresAt:   &expires,
		CreatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, sl.ID)
	assert.Equal(t, "abc123", sl.Code)
	assert.Equal(t, "https://example.com/long-url?param=value", sl.OriginalURL)
	require.NotNil(t, sl.DomainID)
	assert.Equal(t, domainID, *sl.DomainID)
	require.NotNil(t, sl.MessageID)
	assert.Equal(t, messageID, *sl.MessageID)
	require.NotNil(t, sl.CampaignID)
	assert.Equal(t, campaignID, *sl.CampaignID)
	require.NotNil(t, sl.RecipientID)
	assert.Equal(t, recipientID, *sl.RecipientID)
	require.NotNil(t, sl.ExpiresAt)
}

func TestShortLink_NoOptionalFields(t *testing.T) {
	sl := ShortLink{
		ID:          uuid.New(),
		ClientID:    uuid.New(),
		Code:        "xyz789",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now(),
	}

	assert.Nil(t, sl.DomainID)
	assert.Nil(t, sl.MessageID)
	assert.Nil(t, sl.CampaignID)
	assert.Nil(t, sl.RecipientID)
	assert.Nil(t, sl.ExpiresAt)
}

func TestClickEvent_ZeroValue(t *testing.T) {
	var ce ClickEvent
	assert.Equal(t, uuid.Nil, ce.ID)
	assert.Equal(t, uuid.Nil, ce.ShortLinkID)
	assert.Equal(t, uuid.Nil, ce.ClientID)
	assert.Nil(t, ce.CampaignID)
	assert.Equal(t, "", ce.Phone)
	assert.True(t, ce.ClickedAt.IsZero())
	assert.Equal(t, "", ce.IPAddress)
	assert.Equal(t, "", ce.UserAgent)
	assert.Equal(t, "", ce.Referer)
	assert.Equal(t, "", ce.CountryCode)
	assert.False(t, ce.IsUnique)
}

func TestClickEvent_FullyPopulated(t *testing.T) {
	now := time.Now()
	campaignID := uuid.New()

	ce := ClickEvent{
		ID:          uuid.New(),
		ShortLinkID: uuid.New(),
		ClientID:    uuid.New(),
		CampaignID:  &campaignID,
		Phone:       "+79001234567",
		ClickedAt:   now,
		IPAddress:   "203.0.113.42",
		UserAgent:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)",
		Referer:     "https://referrer.com",
		CountryCode: "RU",
		IsUnique:    true,
	}

	assert.NotEqual(t, uuid.Nil, ce.ID)
	assert.Equal(t, "+79001234567", ce.Phone)
	assert.Equal(t, "203.0.113.42", ce.IPAddress)
	assert.Equal(t, "RU", ce.CountryCode)
	assert.True(t, ce.IsUnique)
	require.NotNil(t, ce.CampaignID)
	assert.Equal(t, campaignID, *ce.CampaignID)
}

func TestClickEvent_NoCampaign(t *testing.T) {
	ce := ClickEvent{
		ID:          uuid.New(),
		ShortLinkID: uuid.New(),
		ClientID:    uuid.New(),
		CampaignID:  nil,
		IsUnique:    false,
	}

	assert.Nil(t, ce.CampaignID)
	assert.False(t, ce.IsUnique)
}
