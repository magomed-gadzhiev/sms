package network

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

type stubProviderApplier struct{ err error }

func (s stubProviderApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.err
}

type stubRouteApplier struct{ err error }

func (s stubRouteApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.err
}

func TestApplyAssignmentMaterializers_BothSucceed_ClearsError(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-success")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-success")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "old failure")

	w := ApplyAssignmentMaterializers(context.Background(), pool, stubProviderApplier{}, stubRouteApplier{}, clientID, &psID, &rsID)
	assert.Empty(t, w)

	var errText *string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&errText))
	assert.Nil(t, errText, "retry-state must be cleared on success")
}

func TestApplyAssignmentMaterializers_ProviderFails_RecordsError(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-pfail")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-pfail")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "")

	w := ApplyAssignmentMaterializers(context.Background(), pool,
		stubProviderApplier{err: errors.New("boom")}, stubRouteApplier{},
		clientID, &psID, &rsID)
	require.Len(t, w, 1)
	assert.Equal(t, "provider_materialize", w[0]["step"])
	assert.Contains(t, w[0]["error"], "boom")

	var errText *string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&errText))
	require.NotNil(t, errText)
	assert.Contains(t, *errText, "boom")
}

func TestApplyAssignmentMaterializers_BothFail_RecordsBoth(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-bothfail")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-bothfail")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "")

	w := ApplyAssignmentMaterializers(context.Background(), pool,
		stubProviderApplier{err: errors.New("p")},
		stubRouteApplier{err: errors.New("r")},
		clientID, &psID, &rsID)
	assert.Len(t, w, 2)

	var retryCount int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT materialize_retry_count FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&retryCount))
	assert.GreaterOrEqual(t, retryCount, 2, "retry_count должен быть инкрементирован")
}
