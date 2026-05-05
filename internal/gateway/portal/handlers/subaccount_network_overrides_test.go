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

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestProviderOverride_Add_OK — добавляем private-override → 201; в client_providers
// появилась запись с ownership='private'.
func TestProviderOverride_Add_OK(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provB := storagetest.SeedProvider(t, pool, "OverrideAddOK")

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provB.String() + `","priority":50,"expose_cost":true,"expose_provider_name":true}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	// Verify: client_providers row создан с ownership='private'.
	var ownership string
	var priority int
	var exposeCost, exposeName, active bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT ownership, shared_priority, expose_cost, expose_provider_name, active
		 FROM client_providers WHERE client_id=$1 AND provider_id=$2`,
		subID, provB,
	).Scan(&ownership, &priority, &exposeCost, &exposeName, &active))
	require.Equal(t, "private", ownership)
	require.Equal(t, 50, priority)
	require.True(t, exposeCost)
	require.True(t, exposeName)
	require.True(t, active)
}

// TestProviderOverride_Add_409IfInherited — провайдер уже есть как inherited → 409.
func TestProviderOverride_Add_409IfInherited(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "OverrideInherited")

	// Эмулируем inherited (как будто материализатор уже отработал).
	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provA.String() + `","priority":50,"expose_provider_name":true}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "уже доступен через шаблон")
	require.Contains(t, w.Body.String(), "already_present")
	require.Contains(t, w.Body.String(), "inherited")
}

// TestProviderOverride_Add_NonExistentProvider_404 — POST с UUID провайдера,
// которого нет в БД → 404, message содержит "провайдер".
func TestProviderOverride_Add_NonExistentProvider_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	bogusProv := uuid.New()

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + bogusProv.String() + `","priority":50}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "несуществующий provider → 404, body: %s", w.Body.String())
	require.Contains(t, strings.ToLower(w.Body.String()), "провайдер")
}

// TestProviderOverride_Add_AfterDelete_201 — Add → Delete → Add: последний должен
// вернуть 201 (lifecycle: после удаления private override можно добавить заново).
func TestProviderOverride_Add_AfterDelete_201(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provB := storagetest.SeedProvider(t, pool, "OverrideAfterDelete")

	h := NewSubAccountNetworkOverridesHandlers(pool)

	// 1) Add.
	body := `{"provider_id":"` + provB.String() + `","priority":50}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "первый Add: %s", w.Body.String())

	// 2) Delete.
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides/"+provB.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "provider_id": provB.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.DeleteProviderOverride(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "Delete: %s", w.Body.String())

	// 3) Add повторно — должен вернуть 201 (не 409).
	req = httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "повторный Add после Delete должен быть 201, body: %s", w.Body.String())
}

// TestProviderOverride_Add_ForeignSubAccount_404 — попытка добавить override
// к чужому суб-аккаунту → 404 (не 403).
func TestProviderOverride_Add_ForeignSubAccount_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)
	foreignSub := storagetest.SeedSubAccount(t, pool, otherReseller)
	provB := storagetest.SeedProvider(t, pool, "OverrideForeignSub")

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provB.String() + `","priority":50}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+foreignSub.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": foreignSub.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "чужой sub → 404, body: %s", w.Body.String())

	// Verify: запись не создана.
	var has bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM client_providers WHERE client_id=$1 AND provider_id=$2)`,
		foreignSub, provB,
	).Scan(&has))
	require.False(t, has, "override не должен быть создан для чужого sub")
}

// TestProviderOverride_Add_ForeignPrivateProvider_403 — попытка использовать
// private провайдера, принадлежащего другому reseller'у → 403.
func TestProviderOverride_Add_ForeignPrivateProvider_403(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	foreignPriv := storagetest.SeedProviderPrivate(t, pool, "ForeignPriv", otherReseller)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + foreignPriv.String() + `","priority":50}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "чужой private → 403, body: %s", w.Body.String())
}

// TestProviderOverride_Delete — DELETE existing private override → 204; row gone.
func TestProviderOverride_Delete(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provB := storagetest.SeedProvider(t, pool, "OverrideDel")

	// Сначала создаём override через handler.
	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provB.String() + `","priority":50}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// DELETE.
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides/"+provB.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "provider_id": provB.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.DeleteProviderOverride(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())

	var has bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM client_providers WHERE client_id=$1 AND provider_id=$2)`,
		subID, provB,
	).Scan(&has))
	require.False(t, has, "private override должен быть удалён")

	// Повторный DELETE → 404 (override не существует).
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides/"+provB.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "provider_id": provB.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.DeleteProviderOverride(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "повторный delete → 404")
}

// TestProviderOverride_Delete_DoesNotTouchInherited — DELETE не трогает inherited записи.
func TestProviderOverride_Delete_DoesNotTouchInherited(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "OverrideDelInherited")

	// Эмулируем inherited.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides/"+provA.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "provider_id": provA.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.DeleteProviderOverride(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "inherited не должен удаляться через override-delete; ожидаем 404")

	// Verify: inherited запись жива.
	var ownership string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT ownership FROM client_providers WHERE client_id=$1 AND provider_id=$2`,
		subID, provA,
	).Scan(&ownership))
	require.Equal(t, "inherited", ownership)
}

// overviewResp описывает ответ GET /sub-accounts/{id}/network/overview.
type overviewResp struct {
	ProviderSet *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"provider_set"`
	RouteSet          *struct{}             `json:"route_set"`
	ProviderOverrides []providerOverrideOut `json:"provider_overrides"`
	RouteOverrides    []interface{}         `json:"route_overrides"`
}

// TestProviderOverview — GET overview возвращает provider_set + private override.
func TestProviderOverview(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provInSet := storagetest.SeedProvider(t, pool, "InSet")
	provOverride := storagetest.SeedProvider(t, pool, "OverrideOnly")

	// Создаём provider-set с одним provider'ом и assign'им суб-аккаунту (без материализации —
	// нам важно только то, что overview JOIN-ит через subaccount_routing_assignment).
	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Overview"), false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: provInSet, Priority: 10, ExposeCost: true, ExposeProviderName: true},
	}))
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	require.NoError(t, sraRepo.Upsert(context.Background(), subID, &set.ID, nil))

	// Добавляем private-override через handler.
	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := `{"provider_id":"` + provOverride.String() + `","priority":77,"expose_provider_name":true}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/provider-overrides", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddProviderOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	// GET overview.
	req = httptest.NewRequest("GET", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/overview", nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Overview(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp overviewResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.NotNil(t, resp.ProviderSet, "provider_set должен быть заполнен")
	require.Equal(t, set.ID.String(), resp.ProviderSet.ID)
	require.Equal(t, set.Name, resp.ProviderSet.Name)
	require.Nil(t, resp.RouteSet, "route_set всегда null в Plan 1")
	require.Empty(t, resp.RouteOverrides, "route_overrides пустой в Plan 1")

	require.Len(t, resp.ProviderOverrides, 1, "ровно один private override")
	o := resp.ProviderOverrides[0]
	require.Equal(t, provOverride.String(), o.ProviderID)
	require.Equal(t, 77, o.Priority)
	require.Equal(t, "private", o.Ownership)
}

// TestProviderOverview_ForeignSubAccount_404 — overview чужого sub → 404.
func TestProviderOverview_ForeignSubAccount_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)
	foreignSub := storagetest.SeedSubAccount(t, pool, otherReseller)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/sub-accounts/"+foreignSub.String()+"/network/overview", nil)
	req = mux.SetURLVars(req, map[string]string{"id": foreignSub.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Overview(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "чужой sub → 404, body: %s", w.Body.String())
}

// TestProviderOverview_NoAssignment — overview суб-аккаунта без assignment →
// provider_set=null, provider_overrides=[].
func TestProviderOverview_NoAssignment(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/overview", nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Overview(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp overviewResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.ProviderSet)
	require.Empty(t, resp.ProviderOverrides)
}

// overviewRespV2 — расширенная форма ответа Overview с route_set/route_overrides
// (Plan 2 Task 14). Отдельный тип, чтобы не ломать assertions старых тестов.
type overviewRespV2 struct {
	ProviderSet *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"provider_set"`
	RouteSet *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"route_set"`
	ProviderOverrides []providerOverrideOut    `json:"provider_overrides"`
	RouteOverrides    []map[string]interface{} `json:"route_overrides"`
}

// TestRouteOverride_Add_OK — POST route-override с доступным провайдером → 201;
// в client_routes появилась запись с source='override' и owner_type='subaccount'.
func TestRouteOverride_Add_OK(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideAddOK")

	// Делаем provider доступным суб-аккаунту: client_providers row (любой ownership).
	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"override-1","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"IF","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	// Verify: client_routes row создан с source='override', owner_type='subaccount'.
	var count int
	var ownerType string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes
		 WHERE client_id = $1 AND source = 'override'`, subID,
	).Scan(&count))
	require.Equal(t, 1, count)

	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT owner_type::text FROM client_routes
		 WHERE client_id = $1 AND source = 'override' LIMIT 1`, subID,
	).Scan(&ownerType))
	require.Equal(t, "subaccount", ownerType)
}

// TestRouteOverride_Add_409IfProviderUnavailable — провайдер не привязан к
// суб-аккаунту → 409 с kind=route_provider_not_available.
func TestRouteOverride_Add_409IfProviderUnavailable(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provB := storagetest.SeedProvider(t, pool, "RouteOverrideUnavail") // не в client_providers

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"x","provider_id":"%s","priority":1,"share":100,
		"route_type":"sms","status":"active"
	}`, provB.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "route_provider_not_available")

	// Verify: route не создан.
	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id = $1`, subID,
	).Scan(&count))
	require.Equal(t, 0, count)
}

// TestOverview_IncludesRouteSetAndOverrides — Overview возвращает route_set
// (по subaccount_routing_assignment) и список route_overrides.
func TestOverview_IncludesRouteSetAndOverrides(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqSetName("OverviewRS"))

	// Назначаем route-set суб-аккаунту.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO subaccount_routing_assignment (client_id, route_set_id)
		VALUES ($1, $2)
		ON CONFLICT (client_id) DO UPDATE SET route_set_id = EXCLUDED.route_set_id`,
		subID, rsID)
	require.NoError(t, err)

	// Создаём один override-маршрут (через handler — заодно проверяем, что Add
	// корректно создаёт строку, видимую через Overview).
	provA := storagetest.SeedProvider(t, pool, "OverviewOverrideProv")
	_, err = pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"ovrd","provider_id":"%s","priority":42,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "AddRouteOverride: %s", w.Body.String())

	// GET overview.
	req = httptest.NewRequest("GET",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/overview", nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Overview(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp overviewRespV2
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.RouteSet, "route_set должен быть заполнен")
	require.Equal(t, rsID.String(), resp.RouteSet.ID)
	require.Len(t, resp.RouteOverrides, 1, "ровно один override-маршрут")
	require.Equal(t, provA.String(), resp.RouteOverrides[0]["provider_id"])
	require.Equal(t, "ovrd", resp.RouteOverrides[0]["name"])
}

// TestRouteOverride_Update_OK — PUT /route-overrides/{route_id} меняет priority,
// БД-строка обновлена, status 200.
func TestRouteOverride_Update_OK(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideUpdOK")

	// Provider должен быть доступен суб-аккаунту.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	// Создаём override через handler.
	addBody := fmt.Sprintf(`{
		"name":"orig","provider_id":"%s","priority":10,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(addBody))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add: %s", w.Body.String())

	var addResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &addResp))
	routeID := addResp.ID

	// PUT — меняем priority на 99.
	updBody := fmt.Sprintf(`{
		"name":"orig","provider_id":"%s","priority":99,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req = httptest.NewRequest("PUT",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides/"+routeID,
		strings.NewReader(updBody))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "route_id": routeID})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.UpdateRouteOverride(w, req)
	require.Equal(t, http.StatusOK, w.Code, "Update: %s", w.Body.String())

	// Verify: priority обновлён в БД.
	var priority int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT priority FROM client_routes WHERE id = $1`, routeID,
	).Scan(&priority))
	require.Equal(t, 99, priority)
}

// TestRouteOverride_Update_404IfWrongSubAccount — reseller B пытается обновить
// override-маршрут суб-аккаунта reseller'а A → 404 (cross-account).
func TestRouteOverride_Update_404IfWrongSubAccount(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerA := storagetest.SeedReseller(t, pool)
	resellerB := storagetest.SeedReseller(t, pool)
	subA := storagetest.SeedSubAccount(t, pool, resellerA)
	subB := storagetest.SeedSubAccount(t, pool, resellerB)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideCross")

	// Provider доступен subA.
	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subA, provA, resellerA)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	// resellerA создаёт override на subA.
	addBody := fmt.Sprintf(`{
		"name":"a-route","provider_id":"%s","priority":10,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subA.String()+"/network/route-overrides",
		strings.NewReader(addBody))
	req = mux.SetURLVars(req, map[string]string{"id": subA.String()})
	req = withReseller(req, resellerA)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add: %s", w.Body.String())

	var addResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &addResp))
	routeID := addResp.ID

	// resellerB пытается PUT на subB, передавая routeID от subA.
	// verifyOwnership пройдёт (subB принадлежит resellerB), но scoped WHERE
	// вернёт 0 rows → handler должен вернуть 404 (через pre-flight lookup
	// или через RowsAffected==0).
	updBody := fmt.Sprintf(`{
		"name":"hijacked","provider_id":"%s","priority":99,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req = httptest.NewRequest("PUT",
		"/portal/v1/reseller/sub-accounts/"+subB.String()+"/network/route-overrides/"+routeID,
		strings.NewReader(updBody))
	req = mux.SetURLVars(req, map[string]string{"id": subB.String(), "route_id": routeID})
	req = withReseller(req, resellerB)
	w = httptest.NewRecorder()
	h.UpdateRouteOverride(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "cross-account → 404, body: %s", w.Body.String())

	// Verify: оригинальная строка не тронута (priority остался 10, name 'a-route').
	var priority int
	var name string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT priority, COALESCE(name,'') FROM client_routes WHERE id = $1`, routeID,
	).Scan(&priority, &name))
	require.Equal(t, 10, priority)
	require.Equal(t, "a-route", name)
}

// TestAddRouteOverride_DuplicateSignature_Returns409 — Plan 3 Task 2.
// После drop'а uq_cell_provider handler-уровень pre-check проверяет canonical
// signature (provider_id, route_type, condition_groups). Идентичный второй POST → 409
// с details kind=duplicate_route_signature и existing_id первого override'а.
func TestAddRouteOverride_DuplicateSignature_Returns409(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideDupSig")

	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"dup-1","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())

	// Первый POST → 201.
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "первый Add: %s", w.Body.String())

	var firstResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &firstResp))
	require.NotEmpty(t, firstResp.ID)

	// Второй идентичный POST → 409 duplicate_route_signature.
	req = httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "повторный Add → 409, body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "duplicate_route_signature")
	require.Contains(t, w.Body.String(), firstResp.ID, "details должен содержать existing_id первого override'а")

	// Verify: только одна override-строка в БД.
	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id = $1 AND source = 'override'`, subID,
	).Scan(&count))
	require.Equal(t, 1, count, "дубликат не должен быть вставлен")
}

// TestAddRouteOverride_DifferentConditionsSameProvider_Returns201 — Plan 3 Task 2.
// Два POST'а с одним provider_id, но разными значениями условий — оба 201
// (signature различается из-за condition values).
func TestAddRouteOverride_DifferentConditionsSameProvider_Returns201(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideDiffCond")

	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	bodyRU := fmt.Sprintf(`{
		"name":"diff-RU","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(bodyRU))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "RU Add: %s", w.Body.String())

	bodyBY := fmt.Sprintf(`{
		"name":"diff-BY","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"BY"}]}]
	}`, provA.String())
	req = httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(bodyBY))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "BY Add: %s", w.Body.String())

	// Verify: две override-строки.
	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE client_id = $1 AND source = 'override'`, subID,
	).Scan(&count))
	require.Equal(t, 2, count)
}

// TestUpdateRouteOverride_SameSignature_Returns200 — Plan 3 Task 2 review.
// PUT override с теми же conditions, что и существующая → 200. Валидирует, что
// excludeID в findDuplicateOverrideSignature корректно исключает обновляемую
// строку из поиска дубликатов (иначе self-conflict → 409).
func TestUpdateRouteOverride_SameSignature_Returns200(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideUpdSameSig")

	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)
	body := fmt.Sprintf(`{
		"name":"same-sig","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())

	// Создаём override.
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add: %s", w.Body.String())

	var addResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &addResp))
	routeID := addResp.ID

	// PUT с теми же conditions → должен быть 200, а не 409.
	req = httptest.NewRequest("PUT",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides/"+routeID,
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "route_id": routeID})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.UpdateRouteOverride(w, req)
	require.Equal(t, http.StatusOK, w.Code, "PUT с теми же conditions должен быть 200 (excludeID исключает self), body: %s", w.Body.String())
}

// TestUpdateRouteOverride_ConflictsWithSibling_Returns409 — Plan 3 Task 2 review.
// PUT override меняющий conditions на conditions ДРУГОГО существующего override → 409
// duplicate_route_signature. Валидирует, что excludeID не исключает чужие строки.
func TestUpdateRouteOverride_ConflictsWithSibling_Returns409(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideUpdSibling")

	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	// Override A: country=RU.
	bodyRU := fmt.Sprintf(`{
		"name":"sib-RU","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(bodyRU))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add A (RU): %s", w.Body.String())

	// Override B: country=BY.
	bodyBY := fmt.Sprintf(`{
		"name":"sib-BY","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"BY"}]}]
	}`, provA.String())
	req = httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(bodyBY))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add B (BY): %s", w.Body.String())

	var bResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bResp))
	routeBID := bResp.ID

	// PUT B с conditions=RU → должен конфликтовать с A → 409.
	updBody := fmt.Sprintf(`{
		"name":"sib-BY","provider_id":"%s","priority":50,"share":100,
		"route_type":"sms","status":"active",
		"condition_groups":[{"logic_op":"AND","conditions":[{"type":"country","value":"RU"}]}]
	}`, provA.String())
	req = httptest.NewRequest("PUT",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides/"+routeBID,
		strings.NewReader(updBody))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "route_id": routeBID})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.UpdateRouteOverride(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "PUT B → conditions A должен быть 409, body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "duplicate_route_signature")
}

// TestRouteOverride_Delete_OK — DELETE /route-overrides/{route_id} → 204; row gone.
func TestRouteOverride_Delete_OK(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "RouteOverrideDelOK")

	_, err := pool.Exec(context.Background(), `
		INSERT INTO client_providers (client_id, provider_id, ownership, source_client_id, shared_priority, active)
		VALUES ($1, $2, 'inherited', $3, 10, true)`, subID, provA, resellerID)
	require.NoError(t, err)

	h := NewSubAccountNetworkOverridesHandlers(pool)

	// Создаём override.
	addBody := fmt.Sprintf(`{
		"name":"to-delete","provider_id":"%s","priority":10,"share":100,
		"route_type":"sms","status":"active"
	}`, provA.String())
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides",
		strings.NewReader(addBody))
	req = mux.SetURLVars(req, map[string]string{"id": subID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.AddRouteOverride(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Add: %s", w.Body.String())

	var addResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &addResp))
	routeID := addResp.ID

	// DELETE.
	req = httptest.NewRequest("DELETE",
		"/portal/v1/reseller/sub-accounts/"+subID.String()+"/network/route-overrides/"+routeID, nil)
	req = mux.SetURLVars(req, map[string]string{"id": subID.String(), "route_id": routeID})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.DeleteRouteOverride(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "Delete: %s", w.Body.String())

	// Verify: 0 rows.
	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM client_routes WHERE id = $1`, routeID,
	).Scan(&count))
	require.Equal(t, 0, count)
}
