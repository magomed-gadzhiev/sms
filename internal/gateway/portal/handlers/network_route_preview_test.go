package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestPreview_MatchesByCountry — item с условием country=RU матчит +7-номер.
func TestPreview_MatchesByCountry(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A-"+uniqRouteSetName(""))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	h := NewNetworkRoutePreviewHandlers(pool)
	body := `{"phone":"79991234567","sender_id":"TestSender","traffic_type":"transactional"}`
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Preview(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Matches []struct {
			ItemID string `json:"matched_item_id"`
		} `json:"matches"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Matches, 1)
}

// TestPreview_NoMatch_DifferentCountry — DE-номер не матчит item с country=RU.
func TestPreview_NoMatch_DifferentCountry(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A-"+uniqRouteSetName(""))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "RU"}})

	h := NewNetworkRoutePreviewHandlers(pool)
	body := `{"phone":"491234567890","sender_id":"X","traffic_type":"transactional"}`
	req := httptest.NewRequest("POST",
		"/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview",
		strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": rsID.String()})
	req = withReseller(req, resellerID)
	w := httptest.NewRecorder()
	h.Preview(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Matches []interface{} `json:"matches"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Empty(t, resp.Matches)
}
