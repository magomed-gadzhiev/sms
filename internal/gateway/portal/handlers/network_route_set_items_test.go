package handlers

// Интеграционные тесты handler'ов /reseller/network/route-sets/{id}/items.
// Используют реальную TEST_DATABASE_URL (через storagetest.SetupTestDB).
//
// Тестируем:
//  - Create + List (happy path),
//  - Create 409 при provider_id вне provider-set'а подписанного суб-аккаунта,
//  - Reorder (bulk-update priority),
//  - Duplicate (счёт строк в БД).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// uniqRSItemsName — уникальное имя route-set'а (UNIQUE (reseller_id, name)).
func uniqRSItemsName(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8]
}

func TestRouteSetItems_CreateAndList(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRSItemsName("RS"))

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	body := fmt.Sprintf(`{
		"name":"r1","provider_id":"%s","priority":10,"share":100,"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"IF","conditions":[{"type":"country","value":"RU"}]}],
		"schedules":[{"weekdays":127,"timezone":"Europe/Moscow"}]
	}`, provA.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	w = httptest.NewRecorder()
	listReq := httptest.NewRequest("GET", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", nil)
	listReq = mux.SetURLVars(listReq, map[string]string{"id": rsID.String()})
	listReq = withReseller(listReq, resellerID)
	h.List(w, listReq)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp struct {
		Items []map[string]interface{} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	require.Equal(t, "r1", resp.Items[0]["name"])
	require.Equal(t, provA.String(), resp.Items[0]["provider_id"])
	// condition_groups + schedules должны быть в ответе.
	groups, _ := resp.Items[0]["condition_groups"].([]interface{})
	require.Len(t, groups, 1)
	scheds, _ := resp.Items[0]["schedules"].([]interface{})
	require.Len(t, scheds, 1)
}

func TestRouteSetItems_Create_409_ProviderNotInSet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(ctx, resellerID, uniqRSItemsName("PS"), false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(ctx, ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, ExposeProviderName: true},
	}))

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRSItemsName("RS"))
	_, err = pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO UPDATE SET provider_set_id=EXCLUDED.provider_set_id, route_set_id=EXCLUDED.route_set_id`,
		subID, ps.ID, rsID)
	require.NoError(t, err)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	body := fmt.Sprintf(`{"name":"x","provider_id":"%s","priority":1,"share":100,"route_type":"sms","status":"active"}`, provB.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "route_uses_unavailable_provider")

	// Item не должен был сохраниться.
	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM reseller_route_set_items WHERE set_id=$1`, rsID).Scan(&cnt))
	require.Equal(t, 0, cnt)
}

func TestRouteSetItems_Reorder(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRSItemsName("RS"))
	itemA := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 1, nil)
	itemB := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 2, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	body := fmt.Sprintf(`{"items":[{"item_id":"%s","priority":100},{"item_id":"%s","priority":50}]}`,
		itemA.String(), itemB.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/route-sets/"+rsID.String()+"/items/reorder",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Reorder(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Проверить, что приоритеты применились.
	var prA, prB int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT priority FROM reseller_route_set_items WHERE id=$1`, itemA).Scan(&prA))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT priority FROM reseller_route_set_items WHERE id=$1`, itemB).Scan(&prB))
	require.Equal(t, 100, prA)
	require.Equal(t, 50, prB)
}

func TestRouteSetItems_Duplicate(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRSItemsName("RS"))
	src := storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkRouteSetItemsHandlers(pool, mat, validator)

	req := httptest.NewRequest("POST",
		fmt.Sprintf("/portal/v1/reseller/network/route-sets/%s/items/%s/duplicate", rsID.String(), src.String()),
		nil)
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String(), "item_id": src.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Duplicate(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM reseller_route_set_items WHERE set_id=$1`, rsID).Scan(&count))
	require.Equal(t, 2, count)

	// Дубликат должен сохранить группу условий.
	var groupCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM route_set_condition_groups g
		 JOIN reseller_route_set_items i ON i.id = g.item_id
		 WHERE i.set_id=$1`, rsID).Scan(&groupCount))
	require.Equal(t, 2, groupCount)
}
