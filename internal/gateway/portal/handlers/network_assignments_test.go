package handlers

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

// TestAssignments_List — у reseller'а 2 sub-аккаунта; одному назначен provider-set,
// другому нет. List возвращает обе строки (LEFT JOIN включает unassigned).
func TestAssignments_List(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("L"), false)
	require.NoError(t, err)

	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	require.NoError(t, sraRepo.Upsert(context.Background(), sub1, &set.ID, nil))

	h := NewNetworkAssignmentsHandlers(pool, nil, nil, nil) // материализаторы и validator не нужны для List без route-set
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/assignments", nil)
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Assignments []assignmentOut `json:"assignments"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Assignments, 2, "должно быть 2 строки (включая unassigned sub2)")

	byClient := map[string]assignmentOut{}
	for _, a := range resp.Assignments {
		byClient[a.ClientID] = a
		// route_set_* всегда null в Plan 1.
		require.Nil(t, a.RouteSetID, "route_set_id должен быть nil в Plan 1")
		require.Nil(t, a.RouteSetName, "route_set_name должен быть nil в Plan 1")
	}

	a1 := byClient[sub1.String()]
	require.NotNil(t, a1.ProviderSetID, "sub1 имеет назначение")
	require.Equal(t, set.ID.String(), *a1.ProviderSetID)
	require.NotNil(t, a1.ProviderSetName)
	require.Equal(t, "ok", a1.ValidationStatus)

	a2 := byClient[sub2.String()]
	require.Nil(t, a2.ProviderSetID, "sub2 без назначения")
	require.Nil(t, a2.ProviderSetName)
	require.Equal(t, "unassigned", a2.ValidationStatus)
}

// TestAssignments_PutOne_Materializes — создать provider-set с провайдером A;
// PUT assignment → 200; в client_providers появится inherited запись.
func TestAssignments_PutOne_Materializes(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "AssignA")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("PutOne"), false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeCost: true, ExposeProviderName: true},
	}))

	mat := network.NewProviderSetMaterializer(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, mat, routeMat, nil)

	body := `{"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID,
	).Scan(&count))
	require.Equal(t, 1, count, "у суб-аккаунта должна появиться 1 inherited запись")

	// Verify подзапись
	var providerID uuid.UUID
	var priority int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT provider_id, shared_priority FROM client_providers
		 WHERE client_id=$1 AND ownership='inherited'`, subID,
	).Scan(&providerID, &priority))
	require.Equal(t, provA, providerID)
	require.Equal(t, 10, priority)
}

// TestAssignments_Bulk_PartialSuccess — 2 свои + 1 чужой sub-account.
// Bulk отдаёт {results: 2 ok + 1 error}.
func TestAssignments_Bulk_PartialSuccess(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)
	otherReseller := storagetest.SeedReseller(t, pool)
	subOther := storagetest.SeedSubAccount(t, pool, otherReseller)

	provA := storagetest.SeedProvider(t, pool, "BulkA")
	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Bulk"), false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 7, ExposeCost: false, ExposeProviderName: false},
	}))

	mat := network.NewProviderSetMaterializer(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, mat, routeMat, nil)

	body := `{"client_ids":["` + sub1.String() + `","` + sub2.String() + `","` + subOther.String() + `"],"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/assignments/bulk", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Bulk(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Results []bulkResultItem `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 3)

	okCount, errCount := 0, 0
	byID := map[string]bulkResultItem{}
	for _, r := range resp.Results {
		byID[r.ClientID] = r
		if r.Status == "ok" {
			okCount++
		} else {
			errCount++
		}
	}
	require.Equal(t, 2, okCount, "2 свои → ok")
	require.Equal(t, 1, errCount, "1 чужой → error")
	require.Equal(t, "ok", byID[sub1.String()].Status)
	require.Equal(t, "ok", byID[sub2.String()].Status)
	require.Equal(t, "error", byID[subOther.String()].Status)
}

// TestAssignments_PutOne_NullProviderSet_ClearsInherited — assign set, потом PUT с null →
// inherited записи стерты.
func TestAssignments_PutOne_NullProviderSet_ClearsInherited(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "ClearA")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Clear"), false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeCost: true, ExposeProviderName: true},
	}))

	mat := network.NewProviderSetMaterializer(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, mat, routeMat, nil)

	// Шаг 1: assign set → inherited появилась.
	body := `{"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID,
	).Scan(&count))
	require.Equal(t, 1, count, "после assign — 1 inherited")

	// Шаг 2: PUT с null → inherited стёрта.
	body = `{"provider_set_id":null,"route_set_id":null}`
	req = httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID,
	).Scan(&count))
	require.Equal(t, 0, count, "после null — inherited стёрта")

	// SRA запись осталась с provider_set_id NULL.
	var hasRow bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM subaccount_routing_assignment WHERE client_id=$1 AND provider_set_id IS NULL)`, subID,
	).Scan(&hasRow))
	require.True(t, hasRow, "SRA-запись с NULL provider_set_id должна остаться")
}

// TestAssignments_PutOne_ForeignSet_404 — назначить чужой provider-set → 404.
func TestAssignments_PutOne_ForeignSet_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	foreignSet, err := setRepo.Create(context.Background(), otherReseller, uniqSetName("Foreign"), false)
	require.NoError(t, err)

	mat := network.NewProviderSetMaterializer(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, mat, routeMat, nil)

	body := `{"provider_set_id":"` + foreignSet.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "чужой set → 404, body: %s", w.Body.String())

	// Verify: SRA запись не появилась.
	var hasRow bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM subaccount_routing_assignment WHERE client_id=$1)`, subID,
	).Scan(&hasRow))
	require.False(t, hasRow, "SRA-запись не должна быть создана при отказе ownership-check")
}

// TestAssignments_PutOne_ForeignSubAccount_404 — назначить set чужому sub-account → 404.
func TestAssignments_PutOne_ForeignSubAccount_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)
	foreignSub := storagetest.SeedSubAccount(t, pool, otherReseller)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("MySet"), false)
	require.NoError(t, err)

	mat := network.NewProviderSetMaterializer(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, mat, routeMat, nil)

	body := `{"provider_set_id":"` + set.ID.String() + `","route_set_id":null}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+foreignSub.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": foreignSub.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "чужой sub-account → 404, body: %s", w.Body.String())
}

// TestAssignments_List_ValidationStatus_Conflict — у sub-аккаунта назначены
// provider-set с провайдером A и route-set с провайдером B → validation_status='conflict'.
func TestAssignments_List_ValidationStatus_Conflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "ConflA")
	provB := storagetest.SeedProvider(t, pool, "ConflB")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(context.Background(), resellerID, uniqSetName("ConflPS"), false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
	}))

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqSetName("ConflRS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil) // provB не в provider-set

	_, err = pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (client_id) DO UPDATE SET
		   provider_set_id=EXCLUDED.provider_set_id, route_set_id=EXCLUDED.route_set_id`,
		subID, ps.ID, rsID)
	require.NoError(t, err)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, nil, nil, validator)

	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/assignments", nil)
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Assignments []assignmentOut `json:"assignments"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Assignments, 1)
	require.Equal(t, "conflict", resp.Assignments[0].ValidationStatus)
	require.NotEmpty(t, resp.Assignments[0].ValidationError)
	require.NotNil(t, resp.Assignments[0].RouteSetID)
	require.NotNil(t, resp.Assignments[0].RouteSetName)
}

// TestAssignments_PutOne_409OnMissingProvider — pre-validation; route-set ссылается на провайдера
// вне provider-set → 409 + details.kind='route_uses_unavailable_provider'.
func TestAssignments_PutOne_409OnMissingProvider(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "Miss409A")
	provB := storagetest.SeedProvider(t, pool, "Miss409B")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(context.Background(), resellerID, uniqSetName("Miss409PS"), false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
	}))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqSetName("Miss409RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	provMat := network.NewProviderSetMaterializer(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, provMat, routeMat, validator)

	body := fmt.Sprintf(`{"provider_set_id":"%s","route_set_id":"%s"}`, ps.ID.String(), rsID.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "route_uses_unavailable_provider")

	// SRA не должна быть записана при pre-validation conflict.
	var hasRow bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM subaccount_routing_assignment WHERE client_id=$1)`, subID,
	).Scan(&hasRow))
	require.False(t, hasRow, "SRA не должна появиться при 409")
}

// TestAssignments_PutOne_BothMaterializersRun — provider-set с A + route-set с A (без конфликта)
// → после PUT появляется и client_providers (inherited), и client_routes (template).
func TestAssignments_PutOne_BothMaterializersRun(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "BothA")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(context.Background(), resellerID, uniqSetName("BothPS"), false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
	}))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqSetName("BothRS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	provMat := network.NewProviderSetMaterializer(pool)
	routeMat := network.NewRouteSetMaterializer(pool, rsItems)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, provMat, routeMat, validator)

	body := fmt.Sprintf(`{"provider_set_id":"%s","route_set_id":"%s"}`, ps.ID.String(), rsID.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/assignments/"+subID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutOne(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var providers, routes int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_providers WHERE client_id=$1 AND ownership='inherited'`, subID,
	).Scan(&providers))
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id=$1 AND source='template'`, subID,
	).Scan(&routes))
	require.Equal(t, 1, providers, "должна появиться 1 inherited provider-row")
	require.Equal(t, 1, routes, "должна появиться 1 template route-row")
}

// TestAssignments_BulkDryRun — 2 sub-аккаунта, route-set ссылается на провайдера вне provider-set →
// все возвращаются как status='conflict' без мутаций.
func TestAssignments_BulkDryRun(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	sub1 := storagetest.SeedSubAccount(t, pool, resellerID)
	sub2 := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "DryA")
	provB := storagetest.SeedProvider(t, pool, "DryB")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(context.Background(), resellerID, uniqSetName("DryPS"), false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeProviderName: true},
	}))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqSetName("DryRS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	validator := network.NewConflictValidator(psItems, rsItems)
	h := NewNetworkAssignmentsHandlers(pool, nil, nil, validator)

	body := fmt.Sprintf(`{"client_ids":["%s","%s"],"provider_set_id":"%s","route_set_id":"%s"}`,
		sub1.String(), sub2.String(), ps.ID.String(), rsID.String())
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/assignments/bulk/dry-run", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.BulkDryRun(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Results []bulkResultItem `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	for _, item := range resp.Results {
		require.Equal(t, "conflict", item.Status, "client_id=%s", item.ClientID)
		require.NotEmpty(t, item.Error)
	}

	// Дабл-проверка: dry-run не пишет в SRA.
	var sraCount int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM subaccount_routing_assignment WHERE client_id IN ($1, $2)`,
		sub1, sub2,
	).Scan(&sraCount))
	require.Equal(t, 0, sraCount, "dry-run не должен мутировать SRA")
}
