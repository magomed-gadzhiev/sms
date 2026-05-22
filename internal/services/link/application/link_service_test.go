package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
	"github.com/smpp-server/smpp-server/internal/services/link/mocks"
)

// helpers ────────────────────────────────────────────────────────────────────

func setupService(t *testing.T) (
	*application.LinkService,
	*mocks.MockLinkRepo,
	*mocks.MockDomainRepo,
	*mocks.MockClickRepo,
	*miniredis.Miniredis,
) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	linkRepo := &mocks.MockLinkRepo{}
	domainRepo := &mocks.MockDomainRepo{}
	clickRepo := &mocks.MockClickRepo{}
	svc := application.NewLinkService(linkRepo, domainRepo, clickRepo, rdb)
	return svc, linkRepo, domainRepo, clickRepo, mr
}

// ─── ShortenURL ─────────────────────────────────────────────────────────────

func TestShortenURL_DefaultDomain(t *testing.T) {
	svc, linkRepo, _, _, mr := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.AnythingOfType("*domain.ShortLink")).Return(nil)

	shortURL, code, err := svc.ShortenURL(ctx, clientID, "https://example.com/page", nil, nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, code)
	assert.Contains(t, shortURL, domain.DefaultDomain)
	assert.Contains(t, shortURL, code)

	// Verify Redis cache was set
	cached, err := mr.Get("link:" + code)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", cached)

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_CustomDomain(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()
	domainID := uuid.New()

	customDomain := &domain.ClientDomain{
		ID:       domainID,
		ClientID: clientID,
		Domain:   "links.mycompany.com",
		Status:   "active",
	}

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(customDomain, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.MatchedBy(func(link *domain.ShortLink) bool {
		return link.DomainID != nil && *link.DomainID == domainID
	})).Return(nil)

	shortURL, code, err := svc.ShortenURL(ctx, clientID, "https://example.com", nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, shortURL, "links.mycompany.com")
	assert.NotEmpty(t, code)

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_WithOptionalIDs(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()
	messageID := uuid.New()
	campaignID := uuid.New()
	recipientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.MatchedBy(func(link *domain.ShortLink) bool {
		return link.MessageID != nil && *link.MessageID == messageID &&
			link.CampaignID != nil && *link.CampaignID == campaignID &&
			link.RecipientID != nil && *link.RecipientID == recipientID
	})).Return(nil)

	_, _, err := svc.ShortenURL(ctx, clientID, "https://example.com", &messageID, &campaignID, &recipientID)
	require.NoError(t, err)

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_CreateError(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(errors.New("db connection lost"))

	_, _, err := svc.ShortenURL(ctx, clientID, "https://example.com", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create short link")

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_CodeCollisionRetry(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)

	// First call to GetByCode returns existing link (collision), second returns nil (no collision)
	existingLink := &domain.ShortLink{Code: "exists1"}
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(existingLink, nil).Once()
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound).Once()
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(nil)

	_, _, err := svc.ShortenURL(ctx, clientID, "https://example.com", nil, nil, nil)
	require.NoError(t, err)

	linkRepo.AssertExpectations(t)
}

// ─── ResolveCode ────────────────────────────────────────────────────────────

func TestResolveCode_FromCache(t *testing.T) {
	svc, _, _, _, mr := setupService(t)
	ctx := context.Background()
	code := "abc1234"

	mr.Set("link:"+code, "https://cached-url.com")

	link, err := svc.ResolveCode(ctx, code)
	require.NoError(t, err)
	assert.Equal(t, code, link.Code)
	assert.Equal(t, "https://cached-url.com", link.OriginalURL)
}

func TestResolveCode_CacheMiss_FallbackToDB(t *testing.T) {
	svc, linkRepo, _, _, mr := setupService(t)
	ctx := context.Background()
	code := "xyz7890"

	dbLink := &domain.ShortLink{
		ID:          uuid.New(),
		Code:        code,
		OriginalURL: "https://db-url.com",
	}
	linkRepo.On("GetByCode", ctx, code).Return(dbLink, nil)

	link, err := svc.ResolveCode(ctx, code)
	require.NoError(t, err)
	assert.Equal(t, "https://db-url.com", link.OriginalURL)

	// Verify cache was warmed
	cached, err := mr.Get("link:" + code)
	require.NoError(t, err)
	assert.Equal(t, "https://db-url.com", cached)

	linkRepo.AssertExpectations(t)
}

func TestResolveCode_NotFound(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	code := "notfound"

	linkRepo.On("GetByCode", ctx, code).Return(nil, domain.ErrLinkNotFound)

	link, err := svc.ResolveCode(ctx, code)
	assert.Nil(t, link)
	assert.ErrorIs(t, err, domain.ErrLinkNotFound)

	linkRepo.AssertExpectations(t)
}

func TestResolveCode_DBError(t *testing.T) {
	svc, linkRepo, _, _, _ := setupService(t)
	ctx := context.Background()
	code := "dberr01"

	linkRepo.On("GetByCode", ctx, code).Return(nil, errors.New("connection refused"))

	_, err := svc.ResolveCode(ctx, code)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")

	linkRepo.AssertExpectations(t)
}

// ─── RecordClick ────────────────────────────────────────────────────────────

func TestRecordClick_UniqueClick(t *testing.T) {
	svc, _, _, clickRepo, _ := setupService(t)
	ctx := context.Background()

	linkID := uuid.New()
	event := &domain.ClickEvent{
		ShortLinkID: linkID,
		Phone:       "+1234567890",
		IPAddress:   "1.2.3.4",
	}

	clickRepo.On("Insert", ctx, mock.MatchedBy(func(e *domain.ClickEvent) bool {
		return e.IsUnique == true
	})).Return(nil)

	err := svc.RecordClick(ctx, event)
	require.NoError(t, err)
	assert.True(t, event.IsUnique)

	clickRepo.AssertExpectations(t)
}

func TestRecordClick_DuplicateClick(t *testing.T) {
	svc, _, _, clickRepo, mr := setupService(t)
	ctx := context.Background()

	linkID := uuid.New()

	// Pre-populate the set so second call is not unique
	mr.SAdd("clicked:"+linkID.String(), "+1234567890")

	event := &domain.ClickEvent{
		ShortLinkID: linkID,
		Phone:       "+1234567890",
	}

	clickRepo.On("Insert", ctx, mock.MatchedBy(func(e *domain.ClickEvent) bool {
		return e.IsUnique == false
	})).Return(nil)

	err := svc.RecordClick(ctx, event)
	require.NoError(t, err)
	assert.False(t, event.IsUnique)

	clickRepo.AssertExpectations(t)
}

func TestRecordClick_InsertError(t *testing.T) {
	svc, _, _, clickRepo, _ := setupService(t)
	ctx := context.Background()

	event := &domain.ClickEvent{
		ShortLinkID: uuid.New(),
		Phone:       "+9876543210",
	}

	clickRepo.On("Insert", ctx, mock.Anything).Return(errors.New("insert failed"))

	err := svc.RecordClick(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")

	clickRepo.AssertExpectations(t)
}

// ─── AddDomain ──────────────────────────────────────────────────────────────

func TestAddDomain_Success(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("Create", ctx, mock.MatchedBy(func(d *domain.ClientDomain) bool {
		return d.ClientID == clientID &&
			d.Domain == "links.example.com" &&
			d.Status == "pending_dns" &&
			d.DNSTxtRecord != ""
	})).Return(nil)

	d, err := svc.AddDomain(ctx, clientID, "links.example.com")
	require.NoError(t, err)
	assert.Equal(t, "pending_dns", d.Status)
	assert.Equal(t, "links.example.com", d.Domain)
	assert.Contains(t, d.DNSTxtRecord, "sms-verify=")

	domainRepo.AssertExpectations(t)
}

func TestAddDomain_RepoError(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("Create", ctx, mock.Anything).Return(domain.ErrDomainExists)

	_, err := svc.AddDomain(ctx, clientID, "dup.example.com")
	assert.ErrorIs(t, err, domain.ErrDomainExists)

	domainRepo.AssertExpectations(t)
}

// ─── ListDomains ────────────────────────────────────────────────────────────

func TestListDomains_Success(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	expected := []*domain.ClientDomain{
		{ID: uuid.New(), Domain: "a.com"},
		{ID: uuid.New(), Domain: "b.com"},
	}
	domainRepo.On("ListByClient", ctx, clientID).Return(expected, nil)

	result, err := svc.ListDomains(ctx, clientID)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	domainRepo.AssertExpectations(t)
}

func TestListDomains_Empty(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("ListByClient", ctx, clientID).Return(nil, nil)

	result, err := svc.ListDomains(ctx, clientID)
	require.NoError(t, err)
	assert.Nil(t, result)

	domainRepo.AssertExpectations(t)
}

func TestListDomains_Error(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("ListByClient", ctx, clientID).Return(nil, errors.New("query failed"))

	_, err := svc.ListDomains(ctx, clientID)
	require.Error(t, err)

	domainRepo.AssertExpectations(t)
}

// ─── DeleteDomain ───────────────────────────────────────────────────────────

func TestDeleteDomain_Success(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	domainID := uuid.New()

	domainRepo.On("Delete", ctx, domainID).Return(nil)

	err := svc.DeleteDomain(ctx, domainID)
	require.NoError(t, err)

	domainRepo.AssertExpectations(t)
}

func TestDeleteDomain_Error(t *testing.T) {
	svc, _, domainRepo, _, _ := setupService(t)
	ctx := context.Background()
	domainID := uuid.New()

	domainRepo.On("Delete", ctx, domainID).Return(errors.New("not found"))

	err := svc.DeleteDomain(ctx, domainID)
	require.Error(t, err)

	domainRepo.AssertExpectations(t)
}
