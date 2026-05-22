package router

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// mockPrefixRepo реализует OperatorPrefixRepository для тестов.
type mockPrefixRepo struct {
	mu       sync.Mutex
	prefixes []shared.OperatorPrefix
	err      error
	calls    int
}

func (m *mockPrefixRepo) GetAllActive(_ context.Context) ([]shared.OperatorPrefix, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	cp := make([]shared.OperatorPrefix, len(m.prefixes))
	copy(cp, m.prefixes)
	return cp, nil
}

func (m *mockPrefixRepo) setCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

var (
	opMTS      = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	opBeeline  = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	opMegafon  = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	opDefault  = uuid.MustParse("00000000-0000-0000-0000-0000000000ff")
)

func TestOperatorResolver_ExactPrefixMatch(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
			{OperatorID: opBeeline, Prefix: "7903"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	got := resolver.Resolve(context.Background(), "79101234567")
	assert.Equal(t, opMTS, got)

	got = resolver.Resolve(context.Background(), "79031234567")
	assert.Equal(t, opBeeline, got)

	got = resolver.Resolve(context.Background(), "+79101234567")
	assert.Equal(t, opMTS, got)
}

func TestOperatorResolver_LongestPrefixWins(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7"},
			{OperatorID: opBeeline, Prefix: "79"},
			{OperatorID: opMegafon, Prefix: "7910"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	got := resolver.Resolve(context.Background(), "79101234567")
	assert.Equal(t, opMegafon, got, "longest prefix 7910 should win")

	got = resolver.Resolve(context.Background(), "79201234567")
	assert.Equal(t, opBeeline, got, "prefix 79 should match when 7920 has no longer match")

	got = resolver.Resolve(context.Background(), "74951234567")
	assert.Equal(t, opMTS, got, "prefix 7 should match for landline")
}

func TestOperatorResolver_NoPrefixMatch_ReturnsDefault(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	got := resolver.Resolve(context.Background(), "38044123456")
	assert.Equal(t, opDefault, got)
}

func TestOperatorResolver_EmptyNumber_ReturnsDefault(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	got := resolver.Resolve(context.Background(), "")
	assert.Equal(t, opDefault, got)
}

func TestOperatorResolver_RefreshLoadsNewPrefixes(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	// Первый вызов загружает префиксы.
	got := resolver.Resolve(context.Background(), "79101234567")
	require.Equal(t, opMTS, got)

	// Обновляем данные в репозитории и сбрасываем TTL.
	repo.mu.Lock()
	repo.prefixes = []shared.OperatorPrefix{
		{OperatorID: opBeeline, Prefix: "7910"},
	}
	repo.mu.Unlock()

	// Принудительно делаем кеш устаревшим.
	resolver.mu.Lock()
	resolver.lastRefresh = time.Now().Add(-10 * time.Minute)
	resolver.mu.Unlock()

	got = resolver.Resolve(context.Background(), "79101234567")
	assert.Equal(t, opBeeline, got, "should return new operator after refresh")
}

func TestOperatorResolver_ConcurrentAccess(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
			{OperatorID: opBeeline, Prefix: "7903"},
			{OperatorID: opMegafon, Prefix: "7926"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	// Прогреваем кеш.
	resolver.Resolve(context.Background(), "79101234567")

	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctx := context.Background()
			switch n % 4 {
			case 0:
				got := resolver.Resolve(ctx, "79101234567")
				assert.Equal(t, opMTS, got)
			case 1:
				got := resolver.Resolve(ctx, "79031234567")
				assert.Equal(t, opBeeline, got)
			case 2:
				got := resolver.Resolve(ctx, "79261234567")
				assert.Equal(t, opMegafon, got)
			case 3:
				got := resolver.Resolve(ctx, "11111111111")
				assert.Equal(t, opDefault, got)
			}
		}(i)
	}
	wg.Wait()
}

func TestOperatorResolver_RepoError_KeepsOldPrefixes(t *testing.T) {
	repo := &mockPrefixRepo{
		prefixes: []shared.OperatorPrefix{
			{OperatorID: opMTS, Prefix: "7910"},
		},
	}
	resolver := NewOperatorResolver(repo, opDefault)

	// Загружаем начальные данные.
	got := resolver.Resolve(context.Background(), "79101234567")
	require.Equal(t, opMTS, got)

	// Устанавливаем ошибку и сбрасываем TTL.
	repo.mu.Lock()
	repo.err = assert.AnError
	repo.mu.Unlock()

	resolver.mu.Lock()
	resolver.lastRefresh = time.Now().Add(-10 * time.Minute)
	resolver.mu.Unlock()

	// Старые префиксы должны остаться при ошибке refresh.
	got = resolver.Resolve(context.Background(), "79101234567")
	assert.Equal(t, opMTS, got, "should keep old prefixes when refresh fails")
}
