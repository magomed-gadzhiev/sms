package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// uniqRouteSetName генерирует уникальное имя — UNIQUE (reseller_id, name) в
// reseller_route_sets конфликтует с повторными прогонами теста.
func uniqRouteSetName(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8]
}

// TestRouteSets_CRUD — полный цикл Create/List/Update/Delete.
func TestRouteSets_CRUD(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	h := NewNetworkRouteSetsHandlers(pool, nil)

	// Create
	createName := uniqRouteSetName("RS1")
	body := `{"name":"` + createName + `","is_default":false}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/route-sets", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var created routeSetOut
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, createName, created.Name)
	require.False(t, created.IsDefault)
	setID, err := uuid.Parse(created.ID)
	require.NoError(t, err)

	// List должен содержать созданный set.
	req = httptest.NewRequest("GET", "/portal/v1/reseller/network/route-sets", nil)
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		RouteSets []routeSetOut `json:"route_sets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	var found *routeSetOut
	for i := range listResp.RouteSets {
		if listResp.RouteSets[i].ID == created.ID {
			found = &listResp.RouteSets[i]
			break
		}
	}
	require.NotNil(t, found, "созданный set должен быть в списке")
	require.Equal(t, 0, found.ItemCount)
	require.Equal(t, 0, found.AssignedCount)

	// Update
	newName := uniqRouteSetName("RS1-renamed")
	updBody := `{"name":"` + newName + `","is_default":true}`
	req = httptest.NewRequest("PUT", "/portal/v1/reseller/network/route-sets/"+setID.String(), strings.NewReader(updBody))
	req = mux.SetURLVars(req, map[string]string{"id": setID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var dbName string
	var dbDefault bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT name, is_default FROM reseller_route_sets WHERE id=$1`, setID,
	).Scan(&dbName, &dbDefault))
	require.Equal(t, newName, dbName)
	require.True(t, dbDefault)

	// Delete
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/network/route-sets/"+setID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": setID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())

	var cnt int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reseller_route_sets WHERE id=$1`, setID,
	).Scan(&cnt))
	require.Equal(t, 0, cnt)
}

// TestRouteSets_Delete_409IfAssigned — нельзя удалить set, назначенный
// хотя бы одному суб-аккаунту через subaccount_routing_assignment.route_set_id.
func TestRouteSets_Delete_409IfAssigned(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS-assigned"))

	_, err := pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, route_set_id) VALUES ($1, $2)
		 ON CONFLICT (client_id) DO UPDATE SET route_set_id=EXCLUDED.route_set_id`,
		subID, rsID)
	require.NoError(t, err)

	h := NewNetworkRouteSetsHandlers(pool, nil)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/route-sets/"+rsID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "route_set_assigned")
	require.Contains(t, w.Body.String(), `\"count\":1`)
}

// TestRouteSets_OwnershipCheck_404 — Update от чужого reseller'а возвращает
// 404 (не 403, не утечка факта существования).
func TestRouteSets_OwnershipCheck_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerA := storagetest.SeedReseller(t, pool)
	resellerB := storagetest.SeedReseller(t, pool)
	rsA := storagetest.SeedRouteSet(t, pool, resellerA, uniqRouteSetName("Foreign"))

	h := NewNetworkRouteSetsHandlers(pool, nil)

	body := `{"name":"hijack","is_default":false}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/route-sets/"+rsA.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsA.String()})
	req = withReseller(req, resellerB)
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	// Delete тоже 404
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/network/route-sets/"+rsA.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": rsA.String()})
	req = withReseller(req, resellerB)
	w = httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	// Foreign set всё ещё принадлежит resellerA.
	var ownerID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT reseller_id FROM reseller_route_sets WHERE id=$1`, rsA,
	).Scan(&ownerID))
	require.Equal(t, resellerA, ownerID)
}
