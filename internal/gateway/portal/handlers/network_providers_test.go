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

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// withReseller кладёт client_id в context (заменяет session middleware в тесте).
func withReseller(r *http.Request, clientID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClientIDKey, clientID)
	return r.WithContext(ctx)
}

// TestNetworkProviders_List_ReturnsPlatformAndPrivate — реселлер видит и
// platform, и свой private; чужой private — не видит.
func TestNetworkProviders_List_ReturnsPlatformAndPrivate(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	otherReseller := storage.SeedTestReseller(t, pool)

	platformID := storage.SeedTestProvider(t, pool, "PlatformA")
	privateID := storage.SeedTestProviderPrivate(t, pool, "PrivateB", resellerID)
	foreignPrivate := storage.SeedTestProviderPrivate(t, pool, "ForeignC", otherReseller)

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/providers", nil)
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()

	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Providers []providerOut `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	ids := map[string]string{}
	for _, p := range resp.Providers {
		ids[p.ID] = p.Ownership
	}
	require.Equal(t, "platform", ids[platformID.String()], "platform-провайдер должен быть в списке")
	require.Equal(t, "private", ids[privateID.String()], "свой private — должен быть")
	_, foreignSeen := ids[foreignPrivate.String()]
	require.False(t, foreignSeen, "чужой private не должен быть виден")
}

// TestNetworkProviders_List_Unauthorized — без client_id в контексте → 401.
func TestNetworkProviders_List_Unauthorized(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("GET", "/portal/v1/reseller/network/providers", nil)
	w := httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestNetworkProviders_Create_PrivateOnly — POST создаёт private-провайдера.
func TestNetworkProviders_Create_PrivateOnly(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)

	h := NewNetworkProvidersHandlers(pool)

	uniqueName := "MyProvider-" + uuid.New().String()[:8]
	body := `{"name":"` + uniqueName + `","smpp_host":"smpp.example.com","smpp_port":2775,"system_id":"sys","password":"pwd","system_type":"SMPP"}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/providers", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	var resp providerOut
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "private", resp.Ownership)
	require.Equal(t, uniqueName, resp.Name)

	// Проверяем что в БД действительно private + source_client_id = resellerID.
	var ownership string
	var sourceClientID *uuid.UUID
	err := pool.QueryRow(context.Background(),
		`SELECT ownership, source_client_id FROM providers WHERE id=$1`, resp.ID,
	).Scan(&ownership, &sourceClientID)
	require.NoError(t, err)
	require.Equal(t, "private", ownership)
	require.NotNil(t, sourceClientID)
	require.Equal(t, resellerID, *sourceClientID)

	// Cleanup — у нас нет t.Cleanup от seed, удаляем вручную.
	_, _ = pool.Exec(context.Background(), `DELETE FROM providers WHERE id=$1`, resp.ID)
}

// TestNetworkProviders_Create_InvalidInput — без обязательных полей → 400.
func TestNetworkProviders_Create_InvalidInput(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)

	h := NewNetworkProvidersHandlers(pool)
	body := `{"name":"","smpp_host":"","system_id":""}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/providers", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestNetworkProviders_Update_RejectPlatform — PUT на platform-провайдера → 403.
func TestNetworkProviders_Update_RejectPlatform(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	platformID := storage.SeedTestProvider(t, pool, "Plat")

	h := NewNetworkProvidersHandlers(pool)
	body := `{"name":"hacked","smpp_host":"x","system_id":"y"}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/providers/"+platformID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": platformID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestNetworkProviders_Update_OwnPrivate — PUT на свой private → 200.
func TestNetworkProviders_Update_OwnPrivate(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	priv := storage.SeedTestProviderPrivate(t, pool, "Own", resellerID)

	h := NewNetworkProvidersHandlers(pool)
	uniqueName := "Renamed-" + uuid.New().String()[:8]
	body := `{"name":"` + uniqueName + `","smpp_host":"newhost","smpp_port":2776,"system_id":"newsys","password":"","system_type":"SMPP"}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/providers/"+priv.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Проверяем что обновилось.
	var name, host string
	var port int
	err := pool.QueryRow(context.Background(),
		`SELECT name, host, port FROM providers WHERE id=$1`, priv,
	).Scan(&name, &host, &port)
	require.NoError(t, err)
	require.Equal(t, uniqueName, name)
	require.Equal(t, "newhost", host)
	require.Equal(t, 2776, port)
}

// TestNetworkProviders_Update_RejectForeignPrivate — PUT на чужой private → 404.
func TestNetworkProviders_Update_RejectForeignPrivate(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	otherReseller := storage.SeedTestReseller(t, pool)
	foreign := storage.SeedTestProviderPrivate(t, pool, "Foreign", otherReseller)

	h := NewNetworkProvidersHandlers(pool)
	body := `{"name":"steal","smpp_host":"x","system_id":"y"}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/providers/"+foreign.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": foreign.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestNetworkProviders_Delete_409IfUsedInSet — DELETE private, который в set'е → 409.
func TestNetworkProviders_Delete_409IfUsedInSet(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	priv := storage.SeedTestProviderPrivate(t, pool, "P", resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, "S-"+uuid.New().String()[:8], false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(context.Background(), set.ID, []storage.ProviderSetItemInput{
		{ProviderID: priv, Priority: 10, ExposeCost: false, ExposeProviderName: true},
	}))

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+priv.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "used_in_provider_sets")
}

// TestNetworkProviders_Delete_RejectPlatform — DELETE platform → 403.
func TestNetworkProviders_Delete_RejectPlatform(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	platformID := storage.SeedTestProvider(t, pool, "Plat")

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+platformID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": platformID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestNetworkProviders_Delete_NotInUse — DELETE private не-использованного → 204.
func TestNetworkProviders_Delete_NotInUse(t *testing.T) {
	pool, cleanup := storage.SetupTestDBExposed(t)
	defer cleanup()
	resellerID := storage.SeedTestReseller(t, pool)
	priv := storage.SeedTestProviderPrivate(t, pool, "Free", resellerID)

	h := NewNetworkProvidersHandlers(pool)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/providers/"+priv.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": priv.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())

	// Проверяем что в БД действительно удалён.
	var cnt int
	err := pool.QueryRow(context.Background(), `SELECT count(*) FROM providers WHERE id=$1`, priv).Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 0, cnt)
}
