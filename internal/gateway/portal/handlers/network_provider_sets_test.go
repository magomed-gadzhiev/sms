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

// uniqSetName генерирует уникальное имя set'а — UNIQUE (reseller_id, name)
// в reseller_provider_sets конфликтует с повторными прогонами теста.
func uniqSetName(prefix string) string {
	return prefix + "-" + uuid.New().String()[:8]
}

// TestProviderSets_CRUD — полный цикл Create/List/Update/Delete.
func TestProviderSets_CRUD(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	h := NewNetworkProviderSetsHandlers(pool, nil)

	// Create
	createName := uniqSetName("S1")
	body := `{"name":"` + createName + `","is_default":false}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/provider-sets", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var created providerSetOut
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, createName, created.Name)
	require.False(t, created.IsDefault)
	setID, err := uuid.Parse(created.ID)
	require.NoError(t, err)

	// List — должен содержать созданный set.
	req = httptest.NewRequest("GET", "/portal/v1/reseller/network/provider-sets", nil)
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.List(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var listResp struct {
		ProviderSets []providerSetOut `json:"provider_sets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	var found *providerSetOut
	for i := range listResp.ProviderSets {
		if listResp.ProviderSets[i].ID == created.ID {
			found = &listResp.ProviderSets[i]
			break
		}
	}
	require.NotNil(t, found, "созданный set должен быть в списке")
	require.Equal(t, 0, found.ItemCount)
	require.Equal(t, 0, found.AssignedCount)

	// Update
	newName := uniqSetName("S1-renamed")
	updBody := `{"name":"` + newName + `","is_default":true}`
	req = httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+setID.String(), strings.NewReader(updBody))
	req = mux.SetURLVars(req, map[string]string{"id": setID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Verify in DB
	var dbName string
	var dbDefault bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT name, is_default FROM reseller_provider_sets WHERE id=$1`, setID,
	).Scan(&dbName, &dbDefault))
	require.Equal(t, newName, dbName)
	require.True(t, dbDefault)

	// Delete
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/network/provider-sets/"+setID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": setID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, "body: %s", w.Body.String())

	// Verify gone
	var cnt int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reseller_provider_sets WHERE id=$1`, setID,
	).Scan(&cnt))
	require.Equal(t, 0, cnt)
}

// TestProviderSets_Delete_409IfAssigned — нельзя удалить set, назначенный
// хотя бы одному суб-аккаунту.
func TestProviderSets_Delete_409IfAssigned(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	set, err := setRepo.Create(context.Background(), resellerID, uniqSetName("Assigned"), false)
	require.NoError(t, err)

	// Назначаем set суб-аккаунту.
	sraRepo := storage.NewSubAccountRoutingAssignmentRepository(pool)
	require.NoError(t, sraRepo.Upsert(context.Background(), subID, &set.ID, nil))

	h := NewNetworkProviderSetsHandlers(pool, nil)
	req := httptest.NewRequest("DELETE", "/portal/v1/reseller/network/provider-sets/"+set.ID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": set.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "set_assigned")
	// details — строка с JSON-телом, поэтому в outer-JSON оно эскейпится.
	require.Contains(t, w.Body.String(), `\"count\":1`)
}

// TestProviderSets_OwnershipCheck_404 — Update от чужого reseller'а возвращает
// 404 (не 403, не утечка факта существования).
func TestProviderSets_OwnershipCheck_404(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	otherReseller := storagetest.SeedReseller(t, pool)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	foreign, err := setRepo.Create(context.Background(), otherReseller, uniqSetName("Foreign"), false)
	require.NoError(t, err)

	h := NewNetworkProviderSetsHandlers(pool, nil)

	// Update — должен 404
	body := `{"name":"hijack","is_default":false}`
	req := httptest.NewRequest("PUT", "/portal/v1/reseller/network/provider-sets/"+foreign.ID.String(), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": foreign.ID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Update(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	// Delete — тоже 404
	req = httptest.NewRequest("DELETE", "/portal/v1/reseller/network/provider-sets/"+foreign.ID.String(), nil)
	req = mux.SetURLVars(req, map[string]string{"id": foreign.ID.String()})
	req = withReseller(req, resellerID)
	w = httptest.NewRecorder()
	h.Delete(w, req)
	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	// Verify foreign set всё ещё существует и принадлежит otherReseller
	var ownerID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT reseller_id FROM reseller_provider_sets WHERE id=$1`, foreign.ID,
	).Scan(&ownerID))
	require.Equal(t, otherReseller, ownerID)
}

// TestProviderSets_Create_DefaultUniqueness — Create с is_default=true когда
// уже есть default → старый default снимается транзакционно, новый становится
// default. Уникальность гарантируется uq_reseller_provider_sets_default.
func TestProviderSets_Create_DefaultUniqueness(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	// Создаём первый default напрямую через repo.
	setRepo := storage.NewResellerProviderSetRepository(pool)
	first, err := setRepo.Create(context.Background(), resellerID, uniqSetName("First"), true)
	require.NoError(t, err)
	require.True(t, first.IsDefault)

	// Создаём второй с is_default=true через handler.
	h := NewNetworkProviderSetsHandlers(pool, nil)
	secondName := uniqSetName("Second")
	body := `{"name":"` + secondName + `","is_default":true}`
	req := httptest.NewRequest("POST", "/portal/v1/reseller/network/provider-sets", strings.NewReader(body))
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var created providerSetOut
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.True(t, created.IsDefault)

	// Старый default должен быть снят.
	var firstDefault bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT is_default FROM reseller_provider_sets WHERE id=$1`, first.ID,
	).Scan(&firstDefault))
	require.False(t, firstDefault, "старый default должен быть снят")

	// И только один default у этого reseller'а.
	var defaultCount int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reseller_provider_sets WHERE reseller_id=$1 AND is_default=true`, resellerID,
	).Scan(&defaultCount))
	require.Equal(t, 1, defaultCount)
}
