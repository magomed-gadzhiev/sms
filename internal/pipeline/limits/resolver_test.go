package limits_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/pipeline/limits"
)

func TestDBLimitResolver_ResolveProviderTPS_UsesClientProviderLimit(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS: intPtr(30),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 30, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_FallsBackToTariffPlan(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:    nil,
		tariffPlanDefaultTPS: intPtr(20),
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 20, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_FallsBackToSystemDefault(t *testing.T) {
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:    nil,
		tariffPlanDefaultTPS: nil,
		systemDefaultTPS:     5,
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 5, tps)
}

func TestDBLimitResolver_ResolveProviderTPS_SubAccountBudgetCap(t *testing.T) {
	clientID := uuid.New()
	resolver := limits.NewDBLimitResolver(mockLimitQuerier{
		clientProviderTPS:  intPtr(30),
		systemDefaultTPS:   5,
		parentClientID:     &uuid.UUID{},
		allocatedTPSBudget: intPtr(40),
		allocatedTPSUsed:   20,
	})
	tps, err := resolver.ResolveProviderTPS(context.Background(), clientID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 20, tps)
}

// --- helpers ---

func intPtr(n int) *int { return &n }

type mockLimitQuerier struct {
	clientProviderTPS    *int
	tariffPlanDefaultTPS *int
	systemDefaultTPS     int
	parentClientID       *uuid.UUID
	allocatedTPSBudget   *int
	allocatedTPSUsed     int
}

func (m mockLimitQuerier) GetClientProviderTPS(_ context.Context, _, _ uuid.UUID) (*int, error) {
	return m.clientProviderTPS, nil
}
func (m mockLimitQuerier) GetTariffPlanDefaultTPS(_ context.Context, _ uuid.UUID) (*int, error) {
	return m.tariffPlanDefaultTPS, nil
}
func (m mockLimitQuerier) GetSystemDefaultTPS(_ context.Context) (int, error) {
	return m.systemDefaultTPS, nil
}
func (m mockLimitQuerier) GetClientParentAndBudget(_ context.Context, _ uuid.UUID) (*uuid.UUID, *int, error) {
	return m.parentClientID, m.allocatedTPSBudget, nil
}
func (m mockLimitQuerier) GetSubAccountAllocatedTPS(_ context.Context, _, _ uuid.UUID) (int, error) {
	return m.allocatedTPSUsed, nil
}
