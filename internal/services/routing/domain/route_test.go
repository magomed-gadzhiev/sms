package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoute(t *testing.T) {
	t.Run("creates route with correct defaults", func(t *testing.T) {
		providerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		route := NewRoute("Russia MTS", "+7900", PatternTypePrefix, providerIDs, 10, LoadBalanceRoundRobin)

		assert.NotEqual(t, uuid.Nil, route.ID)
		assert.Equal(t, "Russia MTS", route.Name)
		assert.Equal(t, "+7900", route.Pattern)
		assert.Equal(t, PatternTypePrefix, route.PatternType)
		assert.Equal(t, providerIDs, route.ProviderIDs)
		assert.Equal(t, 10, route.Priority)
		assert.True(t, route.Active)
		assert.False(t, route.FailoverEnabled)
		assert.Equal(t, LoadBalanceRoundRobin, route.LoadBalanceStrategy)
		require.NotNil(t, route.Metadata)
		assert.Empty(t, route.Metadata)
		assert.False(t, route.CreatedAt.IsZero())
		assert.False(t, route.UpdatedAt.IsZero())
	})
}

func TestRoute_Matches(t *testing.T) {
	t.Run("prefix pattern matches destination starting with pattern", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		assert.True(t, route.Matches("+79001234567"))
	})

	t.Run("prefix pattern does not match different prefix", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		assert.False(t, route.Matches("+79101234567"))
	})

	t.Run("prefix pattern does not match shorter destination", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		assert.False(t, route.Matches("+79"))
	})

	t.Run("prefix pattern matches exact length destination", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		assert.True(t, route.Matches("+7900"))
	})

	t.Run("exact pattern matches exact destination", func(t *testing.T) {
		route := NewRoute("R", "+79001234567", PatternTypeExact, nil, 1, LoadBalanceRoundRobin)
		assert.True(t, route.Matches("+79001234567"))
	})

	t.Run("exact pattern does not match partial destination", func(t *testing.T) {
		route := NewRoute("R", "+79001234567", PatternTypeExact, nil, 1, LoadBalanceRoundRobin)
		assert.False(t, route.Matches("+7900123456"))
	})

	t.Run("exact pattern does not match longer destination", func(t *testing.T) {
		route := NewRoute("R", "+79001234567", PatternTypeExact, nil, 1, LoadBalanceRoundRobin)
		assert.False(t, route.Matches("+790012345670"))
	})

	t.Run("regex pattern always returns true (validated elsewhere)", func(t *testing.T) {
		route := NewRoute("R", "^\\+7\\d{10}$", PatternTypeRegex, nil, 1, LoadBalanceRoundRobin)
		assert.True(t, route.Matches("+79001234567"))
		assert.True(t, route.Matches("anything"))
	})

	t.Run("unknown pattern type returns false", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternType("custom"), nil, 1, LoadBalanceRoundRobin)
		assert.False(t, route.Matches("+79001234567"))
	})
}

func TestRoute_Activate(t *testing.T) {
	t.Run("sets route as active", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		route.Active = false

		route.Activate()

		assert.True(t, route.Active)
	})
}

func TestRoute_Deactivate(t *testing.T) {
	t.Run("sets route as inactive", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		require.True(t, route.Active)

		route.Deactivate()

		assert.False(t, route.Active)
	})
}

func TestRoute_EnableFailover(t *testing.T) {
	t.Run("enables failover", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		require.False(t, route.FailoverEnabled)

		route.EnableFailover()

		assert.True(t, route.FailoverEnabled)
	})
}

func TestRoute_DisableFailover(t *testing.T) {
	t.Run("disables failover", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)
		route.EnableFailover()

		route.DisableFailover()

		assert.False(t, route.FailoverEnabled)
	})
}

func TestRoute_UpdatePattern(t *testing.T) {
	t.Run("updates pattern and pattern type", func(t *testing.T) {
		route := NewRoute("R", "+7900", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)

		route.UpdatePattern("+79001234567", PatternTypeExact)

		assert.Equal(t, "+79001234567", route.Pattern)
		assert.Equal(t, PatternTypeExact, route.PatternType)
	})
}

func TestRoute_UpdateProviders(t *testing.T) {
	t.Run("replaces provider list", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, []uuid.UUID{uuid.New()}, 1, LoadBalanceRoundRobin)
		newProviders := []uuid.UUID{uuid.New(), uuid.New()}

		route.UpdateProviders(newProviders)

		assert.Equal(t, newProviders, route.ProviderIDs)
	})
}

func TestRoute_UpdateStrategy(t *testing.T) {
	t.Run("updates load balance strategy", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, nil, 1, LoadBalanceRoundRobin)

		route.UpdateStrategy(LoadBalanceCheapest)

		assert.Equal(t, LoadBalanceCheapest, route.LoadBalanceStrategy)
	})
}

func TestRoute_ToShared(t *testing.T) {
	t.Run("converts to shared route with first provider", func(t *testing.T) {
		p1 := uuid.New()
		p2 := uuid.New()
		route := NewRoute("R", "+7", PatternTypePrefix, []uuid.UUID{p1, p2}, 5, LoadBalanceRoundRobin)
		route.EnableFailover()

		sharedRoute := route.ToShared()

		assert.Equal(t, route.ID, sharedRoute.ID)
		assert.Equal(t, route.Name, sharedRoute.Name)
		assert.Equal(t, route.Pattern, sharedRoute.Pattern)
		assert.Equal(t, "prefix", sharedRoute.PatternType)
		assert.Equal(t, p1, sharedRoute.ProviderID)
		require.NotNil(t, sharedRoute.FailoverProviderID)
		assert.Equal(t, p2, *sharedRoute.FailoverProviderID)
		assert.Equal(t, 5, sharedRoute.Priority)
		assert.True(t, sharedRoute.Active)
	})

	t.Run("failover is nil when failover disabled", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, []uuid.UUID{uuid.New(), uuid.New()}, 1, LoadBalanceRoundRobin)
		// FailoverEnabled is false by default

		sharedRoute := route.ToShared()

		assert.Nil(t, sharedRoute.FailoverProviderID)
	})

	t.Run("handles empty provider list", func(t *testing.T) {
		route := NewRoute("R", "+7", PatternTypePrefix, []uuid.UUID{}, 1, LoadBalanceRoundRobin)

		sharedRoute := route.ToShared()

		assert.Equal(t, uuid.Nil, sharedRoute.ProviderID)
		assert.Nil(t, sharedRoute.FailoverProviderID)
	})
}

func TestRouteFromShared(t *testing.T) {
	t.Run("converts shared route to domain route", func(t *testing.T) {
		p1 := uuid.New()
		p2 := uuid.New()
		sharedRoute := &shared.Route{
			ID:                 uuid.New(),
			Name:               "TestRoute",
			Pattern:            "+7",
			PatternType:        "prefix",
			ProviderID:         p1,
			Priority:           5,
			Active:             true,
			FailoverProviderID: &p2,
		}

		route := RouteFromShared(sharedRoute)

		assert.Equal(t, sharedRoute.ID, route.ID)
		assert.Equal(t, "TestRoute", route.Name)
		assert.Equal(t, "+7", route.Pattern)
		assert.Equal(t, PatternTypePrefix, route.PatternType)
		assert.Len(t, route.ProviderIDs, 2)
		assert.Equal(t, p1, route.ProviderIDs[0])
		assert.Equal(t, p2, route.ProviderIDs[1])
		assert.True(t, route.FailoverEnabled)
		assert.Equal(t, LoadBalanceRoundRobin, route.LoadBalanceStrategy)
	})

	t.Run("nil failover provider sets failover disabled", func(t *testing.T) {
		sharedRoute := &shared.Route{
			ID:          uuid.New(),
			Name:        "TestRoute",
			Pattern:     "+7",
			PatternType: "prefix",
			ProviderID:  uuid.New(),
		}

		route := RouteFromShared(sharedRoute)

		assert.Len(t, route.ProviderIDs, 1)
		assert.False(t, route.FailoverEnabled)
	})
}
