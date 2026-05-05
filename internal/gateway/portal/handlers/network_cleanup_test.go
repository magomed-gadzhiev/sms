package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestRouteCleanup_RemovesOrphanItems — после удаления провайдера у reseller'а
// в route-set'е есть item с этим provider_id, у sub-account'а — override-route.
// Cleanup удаляет оба.
func TestRouteCleanup_RemovesOrphanItems(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "CleanupA")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS-cleanup")
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, nil)

	// Override-маршрут sub-account'а на тот же провайдер.
	_, err := pool.Exec(context.Background(),
		`INSERT INTO client_routes (client_id, provider_id, priority, weight, active, name, status, share, route_type, source, owner_type, owner_id)
		 VALUES ($1, $2, 10, 1, true, 'override-r', 'active', 100, 'sms', 'override', 'subaccount', $1)`,
		subID, provA)
	require.NoError(t, err)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkCleanupHandlers(pool, mat)

	body := fmt.Sprintf(`{"provider_id":"%s"}`, provA.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-cleanup", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.RouteCleanup(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		RemovedRouteSetItems int `json:"removed_route_set_items"`
		RemovedOverrides     int `json:"removed_overrides"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.RemovedRouteSetItems, "должен быть удалён 1 route-set item")
	require.Equal(t, 1, resp.RemovedOverrides, "должен быть удалён 1 override")

	// Verify: в БД действительно ничего не осталось с этим provider_id.
	var itemsLeft, overridesLeft int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM reseller_route_set_items i
		 JOIN reseller_route_sets s ON s.id = i.set_id
		 WHERE s.reseller_id = $1 AND i.provider_id = $2`,
		resellerID, provA).Scan(&itemsLeft))
	require.Equal(t, 0, itemsLeft)

	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM client_routes cr
		 JOIN clients c ON c.id = cr.client_id
		 WHERE c.parent_client_id = $1 AND cr.provider_id = $2 AND cr.source = 'override'`,
		resellerID, provA).Scan(&overridesLeft))
	require.Equal(t, 0, overridesLeft)
}
