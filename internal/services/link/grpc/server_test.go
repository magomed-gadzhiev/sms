package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	linkv1 "github.com/smpp-server/smpp-server/api/proto/linkv1"
	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
	linkmocks "github.com/smpp-server/smpp-server/internal/services/link/mocks"
)

// helpers ────────────────────────────────────────────────────────────────────

func setupLinkGrpcServer(t *testing.T) (
	*LinkGrpcServer,
	*linkmocks.MockLinkRepo,
	*linkmocks.MockDomainRepo,
	*linkmocks.MockClickRepo,
) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	linkRepo := &linkmocks.MockLinkRepo{}
	domainRepo := &linkmocks.MockDomainRepo{}
	clickRepo := &linkmocks.MockClickRepo{}
	svc := application.NewLinkService(linkRepo, domainRepo, clickRepo, rdb)
	srv := NewLinkGrpcServer(svc)
	return srv, linkRepo, domainRepo, clickRepo
}

func setupDomainGrpcServer(t *testing.T) (
	*DomainGrpcServer,
	*linkmocks.MockDomainRepo,
	*linkmocks.MockLinkRepo,
) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	linkRepo := &linkmocks.MockLinkRepo{}
	domainRepo := &linkmocks.MockDomainRepo{}
	clickRepo := &linkmocks.MockClickRepo{}
	svc := application.NewLinkService(linkRepo, domainRepo, clickRepo, rdb)
	srv := NewDomainGrpcServer(svc)
	return srv, domainRepo, linkRepo
}

// ─── LinkGrpcServer.ShortenURL ──────────────────────────────────────────────

func TestShortenURL_Success(t *testing.T) {
	srv, linkRepo, _, _ := setupLinkGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(nil)

	resp, err := srv.ShortenURL(ctx, &linkv1.ShortenRequest{
		ClientId:    clientID.String(),
		OriginalUrl: "https://example.com",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ShortUrl)
	assert.NotEmpty(t, resp.Code)
	assert.Contains(t, resp.ShortUrl, resp.Code)

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_InvalidClientID(t *testing.T) {
	srv, _, _, _ := setupLinkGrpcServer(t)

	_, err := srv.ShortenURL(context.Background(), &linkv1.ShortenRequest{
		ClientId:    "not-a-uuid",
		OriginalUrl: "https://example.com",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestShortenURL_WithOptionalIDs(t *testing.T) {
	srv, linkRepo, _, _ := setupLinkGrpcServer(t)
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

	resp, err := srv.ShortenURL(ctx, &linkv1.ShortenRequest{
		ClientId:    clientID.String(),
		OriginalUrl: "https://example.com",
		MessageId:   messageID.String(),
		CampaignId:  campaignID.String(),
		RecipientId: recipientID.String(),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Code)

	linkRepo.AssertExpectations(t)
}

func TestShortenURL_ServiceError(t *testing.T) {
	srv, linkRepo, _, _ := setupLinkGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(errors.New("db down"))

	_, err := srv.ShortenURL(ctx, &linkv1.ShortenRequest{
		ClientId:    clientID.String(),
		OriginalUrl: "https://example.com",
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())

	linkRepo.AssertExpectations(t)
}

// ─── LinkGrpcServer.ShortenBatch ────────────────────────────────────────────

func TestShortenBatch_Success(t *testing.T) {
	srv, linkRepo, _, _ := setupLinkGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, clientID).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(nil)

	resp, err := srv.ShortenBatch(ctx, &linkv1.ShortenBatchRequest{
		Urls: []*linkv1.ShortenRequest{
			{ClientId: clientID.String(), OriginalUrl: "https://a.com"},
			{ClientId: clientID.String(), OriginalUrl: "https://b.com"},
		},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	assert.NotEqual(t, resp.Results[0].Code, resp.Results[1].Code)

	linkRepo.AssertExpectations(t)
}

func TestShortenBatch_EmptyRequest(t *testing.T) {
	srv, _, _, _ := setupLinkGrpcServer(t)

	resp, err := srv.ShortenBatch(context.Background(), &linkv1.ShortenBatchRequest{
		Urls: nil,
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Results)
}

func TestShortenBatch_PartialFailure(t *testing.T) {
	srv, linkRepo, _, _ := setupLinkGrpcServer(t)
	ctx := context.Background()
	goodClient := uuid.New()

	linkRepo.On("GetClientActiveDomain", ctx, goodClient).Return(nil, nil)
	linkRepo.On("GetByCode", ctx, mock.AnythingOfType("string")).Return(nil, domain.ErrLinkNotFound)
	linkRepo.On("CreateShortLink", ctx, mock.Anything).Return(nil)

	// First URL valid, second has invalid client_id
	_, err := srv.ShortenBatch(ctx, &linkv1.ShortenBatchRequest{
		Urls: []*linkv1.ShortenRequest{
			{ClientId: goodClient.String(), OriginalUrl: "https://a.com"},
			{ClientId: "invalid", OriginalUrl: "https://b.com"},
		},
	})
	// Should fail because the batch is all-or-nothing (error on second item propagates)
	require.Error(t, err)
}

// ─── LinkGrpcServer.GetLinkStats ────────────────────────────────────────────

func TestGetLinkStats_ReturnsEmpty(t *testing.T) {
	srv, _, _, _ := setupLinkGrpcServer(t)

	resp, err := srv.GetLinkStats(context.Background(), &linkv1.LinkStatsRequest{
		ShortLinkId: uuid.New().String(),
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.TotalClicks)
}

// ─── DomainGrpcServer.AddDomain ─────────────────────────────────────────────

func TestAddDomain_Success(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("Create", ctx, mock.MatchedBy(func(d *domain.ClientDomain) bool {
		return d.ClientID == clientID && d.Domain == "links.example.com"
	})).Return(nil)

	resp, err := srv.AddDomain(ctx, &linkv1.AddDomainRequest{
		ClientId: clientID.String(),
		Domain:   "links.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "links.example.com", resp.Domain)
	assert.Equal(t, "pending_dns", resp.Status)
	assert.NotEmpty(t, resp.DnsTxtRecord)

	domainRepo.AssertExpectations(t)
}

func TestAddDomain_InvalidClientID(t *testing.T) {
	srv, _, _ := setupDomainGrpcServer(t)

	_, err := srv.AddDomain(context.Background(), &linkv1.AddDomainRequest{
		ClientId: "bad",
		Domain:   "links.example.com",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestAddDomain_RepoError(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("Create", ctx, mock.Anything).Return(errors.New("duplicate"))

	_, err := srv.AddDomain(ctx, &linkv1.AddDomainRequest{
		ClientId: clientID.String(),
		Domain:   "dup.example.com",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())

	domainRepo.AssertExpectations(t)
}

// ─── DomainGrpcServer.ListDomains ───────────────────────────────────────────

func TestListDomains_Success(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()
	now := time.Now()

	domainRepo.On("ListByClient", ctx, clientID).Return([]*domain.ClientDomain{
		{ID: uuid.New(), ClientID: clientID, Domain: "a.com", Status: "active", CreatedAt: now},
		{ID: uuid.New(), ClientID: clientID, Domain: "b.com", Status: "pending_dns", CreatedAt: now},
	}, nil)

	resp, err := srv.ListDomains(ctx, &linkv1.ListDomainsRequest{
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.Len(t, resp.Domains, 2)
	assert.Equal(t, "a.com", resp.Domains[0].Domain)
	assert.Equal(t, "b.com", resp.Domains[1].Domain)

	domainRepo.AssertExpectations(t)
}

func TestListDomains_InvalidClientID(t *testing.T) {
	srv, _, _ := setupDomainGrpcServer(t)

	_, err := srv.ListDomains(context.Background(), &linkv1.ListDomainsRequest{
		ClientId: "xxx",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestListDomains_Empty(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("ListByClient", ctx, clientID).Return(nil, nil)

	resp, err := srv.ListDomains(ctx, &linkv1.ListDomainsRequest{
		ClientId: clientID.String(),
	})
	require.NoError(t, err)
	assert.Nil(t, resp.Domains)

	domainRepo.AssertExpectations(t)
}

func TestListDomains_RepoError(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	clientID := uuid.New()

	domainRepo.On("ListByClient", ctx, clientID).Return(nil, errors.New("db error"))

	_, err := srv.ListDomains(ctx, &linkv1.ListDomainsRequest{
		ClientId: clientID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())

	domainRepo.AssertExpectations(t)
}

// ─── DomainGrpcServer.DeleteDomain ──────────────────────────────────────────

func TestDeleteDomain_Success(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	domainID := uuid.New()

	domainRepo.On("Delete", ctx, domainID).Return(nil)

	resp, err := srv.DeleteDomain(ctx, &linkv1.DeleteDomainRequest{
		DomainId: domainID.String(),
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	domainRepo.AssertExpectations(t)
}

func TestDeleteDomain_InvalidID(t *testing.T) {
	srv, _, _ := setupDomainGrpcServer(t)

	_, err := srv.DeleteDomain(context.Background(), &linkv1.DeleteDomainRequest{
		DomainId: "not-uuid",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestDeleteDomain_RepoError(t *testing.T) {
	srv, domainRepo, _ := setupDomainGrpcServer(t)
	ctx := context.Background()
	domainID := uuid.New()

	domainRepo.On("Delete", ctx, domainID).Return(errors.New("not found"))

	_, err := srv.DeleteDomain(ctx, &linkv1.DeleteDomainRequest{
		DomainId: domainID.String(),
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())

	domainRepo.AssertExpectations(t)
}

// ─── domainToProto ──────────────────────────────────────────────────────────

func TestDomainToProto_AllFields(t *testing.T) {
	now := time.Now()
	verifiedAt := now.Add(-24 * time.Hour)
	sslExpires := now.Add(90 * 24 * time.Hour)

	d := &domain.ClientDomain{
		ID:            uuid.New(),
		ClientID:      uuid.New(),
		Domain:        "custom.example.com",
		Status:        "active",
		DNSTxtRecord:  "sms-verify=abc",
		DNSVerifiedAt: &verifiedAt,
		SSLExpiresAt:  &sslExpires,
		CreatedAt:     now,
	}

	proto := domainToProto(d)
	assert.Equal(t, d.ID.String(), proto.Id)
	assert.Equal(t, d.ClientID.String(), proto.ClientId)
	assert.Equal(t, "custom.example.com", proto.Domain)
	assert.Equal(t, "active", proto.Status)
	assert.Equal(t, "sms-verify=abc", proto.DnsTxtRecord)
	assert.NotNil(t, proto.DnsVerifiedAt)
	assert.NotNil(t, proto.SslExpiresAt)
	assert.NotNil(t, proto.CreatedAt)
}

func TestDomainToProto_NilOptionalFields(t *testing.T) {
	d := &domain.ClientDomain{
		ID:       uuid.New(),
		ClientID: uuid.New(),
		Domain:   "test.com",
		Status:   "pending_dns",
	}

	proto := domainToProto(d)
	assert.Nil(t, proto.DnsVerifiedAt)
	assert.Nil(t, proto.SslExpiresAt)
}
