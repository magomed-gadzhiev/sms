package network_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteMaterializer_Apply_CreatesTemplateRows(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")

	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, [][2]string{{"country", "RU"}})

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	var routesCount, groupsCount, condsCount int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`,
		subID).Scan(&routesCount))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM route_condition_groups rcg
		 JOIN client_routes cr ON cr.id = rcg.route_id
		 WHERE cr.client_id=$1`, subID).Scan(&groupsCount))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM route_conditions rc
		 JOIN route_condition_groups rcg ON rcg.id = rc.group_id
		 JOIN client_routes cr ON cr.id = rcg.route_id
		 WHERE cr.client_id=$1`, subID).Scan(&condsCount))
	require.Equal(t, 1, routesCount)
	require.Equal(t, 1, groupsCount)
	require.Equal(t, 1, condsCount)
}

func TestRouteMaterializer_Apply_PreservesOverrides(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	// Manually insert an override route
	_, err := pool.Exec(context.Background(),
		`INSERT INTO client_routes (client_id, provider_id, priority, weight, active, name, status, share, route_type, source, owner_type, owner_id)
		 VALUES ($1, $2, 99, 1, true, 'override-route', 'active', 100, 'sms', 'override', 'subaccount', $1)`,
		subID, provB)
	require.NoError(t, err)

	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	var template, override int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID).Scan(&template))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='override'`, subID).Scan(&override))
	require.Equal(t, 1, template)
	require.Equal(t, 1, override)
}

func TestRouteMaterializer_Apply_NilSet_ClearsTemplate(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	require.NoError(t, mat.ApplyToClient(context.Background(), subID, nil))

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID).Scan(&count))
	require.Equal(t, 0, count)
}

func TestRouteMaterializer_Apply_CopiesSchedules(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")

	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	itemID := storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)

	// Schedule: будни 09:00–18:00 Europe/Moscow. weekdays=31 (Mon..Fri = bits 0..4).
	_, err := pool.Exec(context.Background(),
		`INSERT INTO route_set_schedules (item_id, time_from, time_to, weekdays, timezone)
		 VALUES ($1, '09:00'::time, '18:00'::time, 31, 'Europe/Moscow')`, itemID)
	require.NoError(t, err)

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToClient(context.Background(), subID, &setID))

	var count int
	var timeFrom, timeTo, timezone string
	var weekdays int16
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM route_schedules rs
		 JOIN client_routes cr ON cr.id = rs.route_id
		 WHERE cr.client_id = $1`, subID).Scan(&count))
	require.Equal(t, 1, count)

	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT rs.time_from::text, rs.time_to::text, rs.weekdays, rs.timezone
		 FROM route_schedules rs
		 JOIN client_routes cr ON cr.id = rs.route_id
		 WHERE cr.client_id = $1`, subID).Scan(&timeFrom, &timeTo, &weekdays, &timezone))
	require.Equal(t, "09:00:00", timeFrom)
	require.Equal(t, "18:00:00", timeTo)
	require.Equal(t, int16(31), weekdays)
	require.Equal(t, "Europe/Moscow", timezone)
}

func TestRouteMaterializer_ApplyToAllSubscribers(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	setID := storagetest.SeedRouteSet(t, pool, resellerID, "RS")
	storagetest.SeedRouteSetItem(t, pool, setID, provA, 10, nil)

	// Subscribe both
	for _, c := range []uuid.UUID{sub1, sub2} {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO subaccount_routing_assignment (client_id, route_set_id) VALUES ($1, $2)
			 ON CONFLICT (client_id) DO UPDATE SET route_set_id = EXCLUDED.route_set_id`,
			c, setID)
		require.NoError(t, err)
	}

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)
	require.NoError(t, mat.ApplyToAllSubscribers(context.Background(), setID))

	var c1, c2 int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, sub1).Scan(&c1))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, sub2).Scan(&c2))
	require.Equal(t, 1, c1)
	require.Equal(t, 1, c2)
}
