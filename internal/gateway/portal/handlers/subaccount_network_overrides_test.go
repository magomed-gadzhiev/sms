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
