package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
)

// TestMaterializer_Apply_CreatesInheritedRows проверяет, что ApplyToClient
// создаёт inherited-записи в client_providers по items provider-set'а.
func TestMaterializer_Apply_CreatesInheritedRows(t *testing.T) {
	pool, cleanup := setupNetworkTestDB(t)
	defer cleanup()

	resellerID := seedNetworkReseller(t, pool)
	subID := seedNetworkSubAccount(t, pool, resellerID)
	provA := seedNetworkProvider(t, pool, "A")
	provB := seedNetworkProvider(t, pool, "B")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)

	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
		{ProviderID: provB, Priority: 5, ExposeProviderName: true},
	})
	require.NoError(t, err)

	require.NoError(t, mat.ApplyToClient(ctx, subID, &set.ID))

	var count int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

// TestMaterializer_Apply_PreservesPrivateOverrides проверяет, что ApplyToClient
// не затирает записи ownership='private' при материализации.
func TestMaterializer_Apply_PreservesPrivateOverrides(t *testing.T) {
	pool, cleanup := setupNetworkTestDB(t)
	defer cleanup()

	resellerID := seedNetworkReseller(t, pool)
	subID := seedNetworkSubAccount(t, pool, resellerID)
	provA := seedNetworkProvider(t, pool, "A")
	provB := seedNetworkProvider(t, pool, "B")
	provC := seedNetworkProvider(t, pool, "C")

	ctx := context.Background()

	// Вручную вставляем private override для provC.
	_, err := pool.Exec(ctx,
		`INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, active)
		 VALUES ($1, $2, 'private', 99, true)`,
		subID, provC,
	)
	require.NoError(t, err)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)

	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10},
		{ProviderID: provB, Priority: 5},
	})
	require.NoError(t, err)

	require.NoError(t, mat.ApplyToClient(ctx, subID, &set.ID))

	var inheritedCount, privateCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID,
	).Scan(&inheritedCount)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'private'`,
		subID,
	).Scan(&privateCount)
	require.NoError(t, err)

	require.Equal(t, 2, inheritedCount)
	require.Equal(t, 1, privateCount)
}

// TestMaterializer_Apply_NilSet_ClearsInherited проверяет, что ApplyToClient(_, _, nil)
// удаляет все inherited-записи клиента, не трогая private.
func TestMaterializer_Apply_NilSet_ClearsInherited(t *testing.T) {
	pool, cleanup := setupNetworkTestDB(t)
	defer cleanup()

	resellerID := seedNetworkReseller(t, pool)
	subID := seedNetworkSubAccount(t, pool, resellerID)
	provA := seedNetworkProvider(t, pool, "A")

	ctx := context.Background()

	// Вручную вставляем inherited-запись.
	_, err := pool.Exec(ctx,
		`INSERT INTO client_providers (client_id, provider_id, ownership, shared_priority, active)
		 VALUES ($1, $2, 'inherited', 10, true)`,
		subID, provA,
	)
	require.NoError(t, err)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	mat := NewProviderSetMaterializer(pool, setRepo, itemsRepo)

	require.NoError(t, mat.ApplyToClient(ctx, subID, nil))

	var count int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
