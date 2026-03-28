package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLinkNotFound   = errors.New("short link not found")
	ErrDomainNotFound = errors.New("domain not found")
	ErrDomainExists   = errors.New("domain already exists")
	ErrCodeExists     = errors.New("short code already exists")
	ErrLinkExpired    = errors.New("short link has expired")
)

const DefaultDomain = "go.sms-platform.com"

type ClientDomain struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Domain        string
	Status        string
	DNSTxtRecord  string
	DNSVerifiedAt *time.Time
	SSLCertPath   string
	SSLExpiresAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ShortLink struct {
	ID          uuid.UUID
	ClientID    uuid.UUID
	DomainID    *uuid.UUID
	Code        string
	OriginalURL string
	MessageID   *uuid.UUID
	CampaignID  *uuid.UUID
	RecipientID *uuid.UUID
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

type ClickEvent struct {
	ID          uuid.UUID
	ShortLinkID uuid.UUID
	ClientID    uuid.UUID
	CampaignID  *uuid.UUID
	Phone       string
	ClickedAt   time.Time
	IPAddress   string
	UserAgent   string
	Referer     string
	CountryCode string
	IsUnique    bool
}
