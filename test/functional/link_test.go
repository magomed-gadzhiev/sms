//go:build functional

package functional_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/link/application"
	"github.com/smpp-server/smpp-server/internal/services/link/domain"
	linkRepo "github.com/smpp-server/smpp-server/internal/services/link/infrastructure/repository"
)

// skipIfNoRedis skips the test when TEST_REDIS_ADDR is not set.
func skipIfNoRedis(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_REDIS_ADDR") == "" {
		t.Skip("TEST_REDIS_ADDR not set, skipping link functional test")
	}
}

// testPgxPool creates a *pgxpool.Pool from TEST_DB_* env vars (pgx native pool
// used by the link service repositories).
func testPgxPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := testDSN()
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err, "parsing pgx pool config")

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err, "creating pgx pool")

	t.Cleanup(func() { pool.Close() })
	return pool
}

// testRedisClient creates a redis.Client from TEST_REDIS_* env vars.
func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := envOr("TEST_REDIS_ADDR", "localhost:6379")
	password := envOr("TEST_REDIS_PASSWORD", "")
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       15, // Use DB 15 for tests to avoid collision with prod data.
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(ctx).Err(), "pinging test Redis")

	t.Cleanup(func() {
		_ = rdb.FlushDB(context.Background()).Err()
		_ = rdb.Close()
	})

	return rdb
}

func TestLinkShortening(t *testing.T) {
	skipIfNoDB(t)
	skipIfNoRedis(t)

	pool := testPgxPool(t)
	rdb := testRedisClient(t)
	db := setupTestDB(t)

	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-link', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM click_events WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM short_links WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM client_domains WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	linkRepository := linkRepo.NewLinkRepository(pool)
	domainRepository := linkRepo.NewDomainRepository(pool)
	clickRepository := linkRepo.NewClickRepository(pool)
	svc := application.NewLinkService(linkRepository, domainRepository, clickRepository, rdb)

	t.Run("ShortenAndResolve", func(t *testing.T) {
		originalURL := "https://example.com/long-url-for-sms"

		shortURL, code, err := svc.ShortenURL(ctx, clientID, originalURL, nil, nil, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, shortURL)
		assert.NotEmpty(t, code)
		assert.Contains(t, shortURL, domain.DefaultDomain)
		assert.Contains(t, shortURL, code)

		// Resolve the short code.
		resolved, err := svc.ResolveCode(ctx, code)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, originalURL, resolved.OriginalURL)
	})

	t.Run("ShortenWithMessageAndCampaignIDs", func(t *testing.T) {
		messageID := uuid.New()
		campaignID := uuid.New()

		shortURL, code, err := svc.ShortenURL(ctx, clientID, "https://example.com/tracked", &messageID, &campaignID, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, shortURL)
		assert.NotEmpty(t, code)

		resolved, err := svc.ResolveCode(ctx, code)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/tracked", resolved.OriginalURL)
	})

	t.Run("ResolveNonExistentCodeFails", func(t *testing.T) {
		_, err := svc.ResolveCode(ctx, "nonexistent999")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrLinkNotFound)
	})

	t.Run("RecordClick", func(t *testing.T) {
		_, code, err := svc.ShortenURL(ctx, clientID, "https://example.com/click-test", nil, nil, nil)
		require.NoError(t, err)

		resolved, err := svc.ResolveCode(ctx, code)
		require.NoError(t, err)

		event := &domain.ClickEvent{
			ID:          uuid.New(),
			ShortLinkID: resolved.ID,
			ClientID:    clientID,
			Phone:       "+79001234567",
			ClickedAt:   time.Now(),
			IPAddress:   "192.168.1.1",
			UserAgent:   "Mozilla/5.0",
			Referer:     "",
			CountryCode: "RU",
		}

		err = svc.RecordClick(ctx, event)
		require.NoError(t, err)
		assert.True(t, event.IsUnique, "first click should be unique")

		// Second click from same phone should not be unique.
		event2 := &domain.ClickEvent{
			ID:          uuid.New(),
			ShortLinkID: resolved.ID,
			ClientID:    clientID,
			Phone:       "+79001234567",
			ClickedAt:   time.Now(),
			IPAddress:   "192.168.1.2",
			UserAgent:   "Chrome/100",
			Referer:     "",
			CountryCode: "RU",
		}

		err = svc.RecordClick(ctx, event2)
		require.NoError(t, err)
		assert.False(t, event2.IsUnique, "second click from same phone should not be unique")
	})

	t.Run("MultipleShortLinksHaveUniqueCodes", func(t *testing.T) {
		codes := make(map[string]bool)
		for i := 0; i < 10; i++ {
			_, code, err := svc.ShortenURL(ctx, clientID, "https://example.com/unique-test-"+uuid.New().String(), nil, nil, nil)
			require.NoError(t, err)
			assert.False(t, codes[code], "short code must be unique")
			codes[code] = true
		}
	})
}

func TestLinkDomainManagement(t *testing.T) {
	skipIfNoDB(t)
	skipIfNoRedis(t)

	pool := testPgxPool(t)
	rdb := testRedisClient(t)
	db := setupTestDB(t)

	clientID := uuid.New()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-domain', $1, 'secret')
		 ON CONFLICT DO NOTHING`, clientID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		bgCtx := context.Background()
		_, _ = db.ExecContext(bgCtx, `DELETE FROM short_links WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM client_domains WHERE client_id = $1`, clientID)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM clients WHERE id = $1`, clientID)
	})

	linkRepository := linkRepo.NewLinkRepository(pool)
	domainRepository := linkRepo.NewDomainRepository(pool)
	clickRepository := linkRepo.NewClickRepository(pool)
	svc := application.NewLinkService(linkRepository, domainRepository, clickRepository, rdb)

	t.Run("AddAndListDomains", func(t *testing.T) {
		d, err := svc.AddDomain(ctx, clientID, "links.mycompany.com")
		require.NoError(t, err)
		require.NotNil(t, d)

		assert.Equal(t, clientID, d.ClientID)
		assert.Equal(t, "links.mycompany.com", d.Domain)
		assert.Equal(t, "pending_dns", d.Status)
		assert.NotEmpty(t, d.DNSTxtRecord)
		assert.Contains(t, d.DNSTxtRecord, "sms-verify=")

		domains, err := svc.ListDomains(ctx, clientID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(domains), 1)

		found := false
		for _, dom := range domains {
			if dom.Domain == "links.mycompany.com" {
				found = true
				break
			}
		}
		assert.True(t, found, "created domain must appear in list")
	})

	t.Run("DeleteDomain", func(t *testing.T) {
		d, err := svc.AddDomain(ctx, clientID, "delete-me.example.com")
		require.NoError(t, err)

		err = svc.DeleteDomain(ctx, d.ID)
		require.NoError(t, err)

		domains, err := svc.ListDomains(ctx, clientID)
		require.NoError(t, err)
		for _, dom := range domains {
			assert.NotEqual(t, "delete-me.example.com", dom.Domain)
		}
	})
}
