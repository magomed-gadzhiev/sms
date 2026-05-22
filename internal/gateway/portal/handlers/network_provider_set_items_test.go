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

// TestProviderSetItems_GET_EmptyForNew — новый set, ListItems → 200 с {"items":[]}.
func TestProviderSetItems_GET_EmptyForNew(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Empty"), false)
	require.NoError(t, err)

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", nil)
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.ListItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Items []providerSetItemOut `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Items, "items не должен быть null")
	require.Len(t, resp.Items, 0, "items должен быть пустым массивом")
}

// TestProviderSetItems_PUT_AddsItems — PUT с 2 items → 200; GET → returns 2 items с provider_name.
func TestProviderSetItems_PUT_AddsItems(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Add"), false)
	require.NoError(t, err)

	provA := storagetest.SeedProvider(t, pool, "AddA")
	provB := storagetest.SeedProvider(t, pool, "AddB")

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[
		{"provider_id":"` + provA.String() + `","priority":10,"expose_cost":true,"expose_provider_name":true},
		{"provider_id":"` + provB.String() + `","priority":5,"expose_cost":false,"expose_provider_name":true}
	]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var putResp struct {
		SetID string `json:"set_id"`
		Count int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &putResp))
	require.Equal(t, set.ID.String(), putResp.SetID)
	require.Equal(t, 2, putResp.Count)

	// GET → returns these 2 items с provider_name.
	req = httptest.NewRequest("GET", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", nil)
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.ListItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var listResp struct {
		Items []providerSetItemOut `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 2)

	// Найти провайдер A и проверить поля.
	byProvID := map[string]providerSetItemOut{}
	for _, it := range listResp.Items {
		byProvID[it.ProviderID] = it
		require.NotEmpty(t, it.ProviderName, "provider_name должен быть из join'а")
	}
	require.Equal(t, 10, byProvID[provA.String()].Priority)
	require.True(t, byProvID[provA.String()].ExposeCost)
	require.Equal(t, 5, byProvID[provB.String()].Priority)
	require.False(t, byProvID[provB.String()].ExposeCost)

	// Order: priority DESC → A (10) перед B (5).
	require.Equal(t, provA.String(), listResp.Items[0].ProviderID)
	require.Equal(t, provB.String(), listResp.Items[1].ProviderID)
}

// TestProviderSetItems_PUT_ReplaceClearsOld — PUT [A], затем PUT [B] → GET вернёт только B.
func TestProviderSetItems_PUT_ReplaceClearsOld(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Repl"), false)
	require.NoError(t, err)

	provA := storagetest.SeedProvider(t, pool, "ReplA")
	provB := storagetest.SeedProvider(t, pool, "ReplB")

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	// PUT [A]
	body := `{"items":[{"provider_id":"` + provA.String() + `","priority":10,"expose_cost":true,"expose_provider_name":true}]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// PUT [B] — старый A должен быть удалён.
	body = `{"items":[{"provider_id":"` + provB.String() + `","priority":7,"expose_cost":false,"expose_provider_name":false}]}`
	req = httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// GET → только B.
	req = httptest.NewRequest("GET", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", nil)
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.ListItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var listResp struct {
		Items []providerSetItemOut `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	require.Equal(t, provB.String(), listResp.Items[0].ProviderID)
}

// TestProviderSetItems_PUT_DuplicateProviderID_400 — два одинаковых provider_id → 400.
func TestProviderSetItems_PUT_DuplicateProviderID_400(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Dup"), false)
	require.NoError(t, err)

	provA := storagetest.SeedProvider(t, pool, "DupA")

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[
		{"provider_id":"` + provA.String() + `","priority":10,"expose_cost":true,"expose_provider_name":true},
		{"provider_id":"` + provA.String() + `","priority":5,"expose_cost":false,"expose_provider_name":false}
	]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	require.Contains(t, strings.ToLower(w.Body.String()), "дубликат")
}

// TestProviderSetItems_PUT_ForeignPrivateProvider_403 — private-провайдер чужого reseller'а → 403.
func TestProviderSetItems_PUT_ForeignPrivateProvider_403(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Foreign"), false)
	require.NoError(t, err)

	foreignPriv := storagetest.SeedProviderPrivate(t, pool, "Foreign", otherReseller)

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[{"provider_id":"` + foreignPriv.String() + `","priority":10,"expose_cost":true,"expose_provider_name":true}]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "body: %s", w.Body.String())

	// Verify: items не записались.
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	items, err := itemsRepo.ListBySet(context.Background(), set.ID)
	require.NoError(t, err)
	require.Len(t, items, 0, "items не должны быть записаны при ошибке валидации")
}

// TestProviderSetItems_PUT_UnknownProvider_400 — несуществующий provider_id → 400.
func TestProviderSetItems_PUT_UnknownProvider_400(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("UnknownProv"), false)
	require.NoError(t, err)

	unknownID := uuid.New() // произвольный UUID, которого нет в providers

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[{"provider_id":"` + unknownID.String() + `","priority":10,"expose_cost":false,"expose_provider_name":false}]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), unknownID.String(), "ответ должен содержать неизвестный provider_id")
}

// TestProviderSetItems_PUT_TriggersMaterialization — назначаем set суб-аккаунту,
// потом PUT items → у суб-аккаунта появятся inherited записи в client_providers.
func TestProviderSetItems_PUT_TriggersMaterialization(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Mat"), false)
	require.NoError(t, err)

	// Назначаем set суб-аккаунту (через SubAccountRoutingAssignment.Upsert).
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	require.NoError(t, sraRepo.Upsert(context.Background(), subID, &set.ID, nil))

	provA := storagetest.SeedProvider(t, pool, "MatA")
	provB := storagetest.SeedProvider(t, pool, "MatB")

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	body := `{"items":[
		{"provider_id":"` + provA.String() + `","priority":10,"expose_cost":true,"expose_provider_name":true},
		{"provider_id":"` + provB.String() + `","priority":5,"expose_cost":false,"expose_provider_name":false}
	]}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+set.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Проверяем client_providers суб-аккаунта.
	rows, err := pool.Query(context.Background(),
		`SELECT provider_id, ownership, shared_priority, expose_cost, expose_provider_name, active
		   FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'
		  ORDER BY shared_priority DESC`, subID)
	require.NoError(t, err)
	defer rows.Close()

	type cpRow struct {
		ProviderID  uuid.UUID
		Ownership   string
		Priority    int
		ExposeCost  bool
		ExposeProvN bool
		Active      bool
	}
	var inherited []cpRow
	for rows.Next() {
		var r cpRow
		require.NoError(t, rows.Scan(&r.ProviderID, &r.Ownership, &r.Priority, &r.ExposeCost, &r.ExposeProvN, &r.Active))
		inherited = append(inherited, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, inherited, 2, "у суб-аккаунта должно быть 2 inherited записи")

	byID := map[uuid.UUID]cpRow{}
	for _, r := range inherited {
		byID[r.ProviderID] = r
		require.Equal(t, "inherited", r.Ownership)
		require.True(t, r.Active)
	}
	require.Equal(t, 10, byID[provA].Priority)
	require.True(t, byID[provA].ExposeCost)
	require.Equal(t, 5, byID[provB].Priority)
	require.False(t, byID[provB].ExposeCost)
}

// TestProviderSetItems_PUT_409IfRemovingProviderUsedInSubscriberRoutes —
// нельзя убрать провайдера из provider-set'а, если он используется в route-set'е,
// назначенном суб-аккаунту, подписанному на этот provider-set.
func TestProviderSetItems_PUT_409IfRemovingProviderUsedInSubscriberRoutes(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "DiffA")
	provB := storagetest.SeedProvider(t, pool, "DiffB")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	ps, err := psRepo.Create(context.Background(), resellerID, uniqSetName("DiffPS"), false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), ps.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10, ExposeCost: false, ExposeProviderName: true},
		{ProviderID: provB, Priority: 5, ExposeCost: false, ExposeProviderName: true},
	}))

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "DiffRS-"+uuid.New().String()[:8])
	storagetest.SeedRouteSetItem(t, pool, rsID, provB, 10, nil)

	_, err = pool.Exec(context.Background(),
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO UPDATE SET provider_set_id = EXCLUDED.provider_set_id,
		                                        route_set_id    = EXCLUDED.route_set_id`,
		subID, ps.ID, rsID)
	require.NoError(t, err)

	mat := network.NewProviderSetMaterializer(pool)
	h := NewNetworkProviderSetItemsHandlers(pool, mat)

	// PUT — оставляем только provA, убираем provB. provB используется в route-set'е,
	// назначенном subID (который подписан на этот provider-set) → 409.
	body := fmt.Sprintf(`{"items":[{"provider_id":"%s","priority":1,"expose_cost":false,"expose_provider_name":true}]}`, provA.String())
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+ps.ID.String()+"/items", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": ps.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.PutItems(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "provider_used_in_routes")

	// items не изменились (всё ещё A и B).
	items, err := itemsRepo.ListBySet(context.Background(), ps.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
}
