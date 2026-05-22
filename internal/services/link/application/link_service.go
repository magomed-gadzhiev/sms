package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

type LinkService struct {
	linkRepo   LinkRepo
	domainRepo DomainRepo
	clickRepo  ClickRepo
	rdb        *redis.Client
	logger     zerolog.Logger
}

func NewLinkService(
	linkRepo LinkRepo,
	domainRepo DomainRepo,
	clickRepo ClickRepo,
	rdb *redis.Client,
) *LinkService {
	return &LinkService{
		linkRepo:   linkRepo,
		domainRepo: domainRepo,
		clickRepo:  clickRepo,
		rdb:        rdb,
		logger:     log.With().Str("component", "link-service").Logger(),
	}
}

// ShortenURL creates a short link for the given URL.
func (s *LinkService) ShortenURL(ctx context.Context, clientID uuid.UUID, originalURL string, messageID, campaignID, recipientID *uuid.UUID) (string, string, error) {
	// Get client's active custom domain (or default)
	activeDomain, _ := s.linkRepo.GetClientActiveDomain(ctx, clientID)
	baseDomain := domain.DefaultDomain
	var domainID *uuid.UUID
	if activeDomain != nil {
		baseDomain = activeDomain.Domain
		domainID = &activeDomain.ID
	}

	// Generate unique short code (retry on collision)
	var code string
	for i := 0; i < 5; i++ {
		code = domain.GenerateCode(7)
		existing, _ := s.linkRepo.GetByCode(ctx, code)
		if existing == nil {
			break
		}
	}

	link := &domain.ShortLink{
		ClientID:    clientID,
		DomainID:    domainID,
		Code:        code,
		OriginalURL: originalURL,
		MessageID:   messageID,
		CampaignID:  campaignID,
		RecipientID: recipientID,
	}

	if err := s.linkRepo.CreateShortLink(ctx, link); err != nil {
		return "", "", fmt.Errorf("create short link: %w", err)
	}

	// Cache in Redis for fast redirect lookup
	cacheKey := fmt.Sprintf("link:%s", code)
	s.rdb.Set(ctx, cacheKey, originalURL, 0)

	shortURL := fmt.Sprintf("https://%s/%s", baseDomain, code)
	return shortURL, code, nil
}

// ResolveCode looks up a short code and returns the original URL.
func (s *LinkService) ResolveCode(ctx context.Context, code string) (*domain.ShortLink, error) {
	// Try Redis first
	cacheKey := fmt.Sprintf("link:%s", code)
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return &domain.ShortLink{Code: code, OriginalURL: cached}, nil
	}

	// Fallback to DB
	link, err := s.linkRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Warm cache
	s.rdb.Set(ctx, cacheKey, link.OriginalURL, 0)
	return link, nil
}

// RecordClick checks uniqueness and records a click event.
func (s *LinkService) RecordClick(ctx context.Context, event *domain.ClickEvent) error {
	// Check uniqueness using Redis SET
	uniqueKey := fmt.Sprintf("clicked:%s", event.ShortLinkID.String())
	added, _ := s.rdb.SAdd(ctx, uniqueKey, event.Phone).Result()
	event.IsUnique = added > 0

	return s.clickRepo.Insert(ctx, event)
}

// --- Domain management ---

func (s *LinkService) AddDomain(ctx context.Context, clientID uuid.UUID, domainName string) (*domain.ClientDomain, error) {
	txtRecord := fmt.Sprintf("sms-verify=%s", uuid.New().String()[:8])
	d := &domain.ClientDomain{
		ClientID:     clientID,
		Domain:       domainName,
		Status:       "pending_dns",
		DNSTxtRecord: txtRecord,
	}
	if err := s.domainRepo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *LinkService) ListDomains(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientDomain, error) {
	return s.domainRepo.ListByClient(ctx, clientID)
}

func (s *LinkService) DeleteDomain(ctx context.Context, id uuid.UUID) error {
	return s.domainRepo.Delete(ctx, id)
}
