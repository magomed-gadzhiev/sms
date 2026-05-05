package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// seedCountryWithPhoneCode вставляет country с заданным phone_code, регистрирует cleanup.
func seedCountryWithPhoneCode(t *testing.T, pool *pgxpool.Pool, iso, phoneCode string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO countries (id, name, iso_code, phone_code, currency)
		 VALUES ($1, $2, $3, $4, 'USD')`,
		id, "TestCountry-"+iso, iso, phoneCode,
	)
	require.NoError(t, err, "seed country failed")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM countries WHERE id = $1`, id)
	})
	return id
}

// seedOperatorWithPrefix вставляет operator + prefix, регистрирует cleanup.
func seedOperatorWithPrefix(t *testing.T, pool *pgxpool.Pool, countryID uuid.UUID, code, prefix string) uuid.UUID {
	t.Helper()
	opID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO operators (id, country_id, name, code, active)
		 VALUES ($1, $2, $3, $4, true)`,
		opID, countryID, "TestOp-"+code, code,
	)
	require.NoError(t, err, "seed operator failed")
	prefixID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO operator_prefixes (id, operator_id, prefix, priority, active)
		 VALUES ($1, $2, $3, 0, true)`,
		prefixID, opID, prefix,
	)
	require.NoError(t, err, "seed operator_prefix failed")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operator_prefixes WHERE id = $1`, prefixID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, opID)
	})
	return opID
}

// uniqOperatorCode — UNIQUE INDEX на operators.code; сделать тест-стабильным.
func uniqOperatorCode(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// uniqISO — UNIQUE INDEX на countries.iso_code (VARCHAR(2)).
// 26*26=676 комбинаций; используем хвост наносекунд → 2 буквы A-Z.
var isoSeq int64

func uniqISO() string {
	n := time.Now().UnixNano() + isoSeq
	isoSeq++
	a := byte('A' + (n/26)%26)
	b := byte('A' + n%26)
	return string([]byte{a, b})
}

// TestPreview_MatchesByCountry — item с условием country=KZ матчит +7-номер
// (KZ — единственная country с phone_code='7' в seed; см. resolve_country_iso_by_phone).
func TestPreview_MatchesByCountry(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A-"+uniqRouteSetName(""))
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10, [][2]string{{"country", "KZ"}})

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

// TestPreview_OperatorCondition_MatchesByPhonePrefix — Plan 5 D8.
// item с условием operator=<uuid> матчит phone, чей prefix принадлежит этому operator'у;
// чужой phone — нет.
func TestPreview_OperatorCondition_MatchesByPhonePrefix(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A-"+uniqRouteSetName(""))

	// Уникальный prefix на основе времени — UNIQUE INDEX на operator_prefixes.prefix.
	// Должен быть достаточно длинным (>1 символа), чтобы не конфликтовать с seed
	// и быть выбранным longest-prefix-match'ем.
	uniqPrefix := fmt.Sprintf("79%d", time.Now().UnixNano()%100000)
	uniqPhoneCode := uniqPrefix[:5] // VARCHAR(5)
	countryID := seedCountryWithPhoneCode(t, pool, uniqISO(), uniqPhoneCode)
	opID := seedOperatorWithPrefix(t, pool, countryID, uniqOperatorCode("OP"), uniqPrefix)

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10,
		[][2]string{{"operator", opID.String()}})

	h := NewNetworkRoutePreviewHandlers(pool)

	// Phone matching prefix → operator-condition match.
	body := fmt.Sprintf(`{"phone":"%s4567890","sender_id":"X","traffic_type":"transactional"}`, uniqPrefix)
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
	require.Len(t, resp.Matches, 1, "expected match for phone with operator prefix")

	// Phone NOT matching prefix → no match.
	body2 := `{"phone":"19999999999","sender_id":"X","traffic_type":"transactional"}`
	req2 := httptest.NewRequest("POST",
		"/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview",
		strings.NewReader(body2))
	req2 = mux.SetURLVars(req2, map[string]string{"id": rsID.String()})
	req2 = withReseller(req2, resellerID)
	w2 := httptest.NewRecorder()
	h.Preview(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, "body: %s", w2.Body.String())

	var resp2 struct {
		Matches []interface{} `json:"matches"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	require.Empty(t, resp2.Matches, "phone without matching operator prefix should not match")
}

// TestPreview_CountryCondition_LookupsCountriesTable — Plan 5 D9.
// country-condition resolved через countries.phone_code lookup, не in-memory map.
func TestPreview_CountryCondition_LookupsCountriesTable(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A-"+uniqRouteSetName(""))

	// Уникальный 5-значный phone_code (VARCHAR(5)). 88-prefix не конфликтует
	// с seed countries (KZ='7', UA='380'). Длиннее всего seed → longest-match.
	uniqPhoneCode := fmt.Sprintf("88%03d", time.Now().UnixNano()%1000)
	iso := uniqISO()
	seedCountryWithPhoneCode(t, pool, iso, uniqPhoneCode)

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, uniqRouteSetName("RS"))
	storagetest.SeedRouteSetItem(t, pool, rsID, provA, 10,
		[][2]string{{"country", iso}})

	h := NewNetworkRoutePreviewHandlers(pool)

	// Phone с этим prefix — match.
	body := fmt.Sprintf(`{"phone":"+%s34567","sender_id":"X","traffic_type":"transactional"}`, uniqPhoneCode)
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
	require.Len(t, resp.Matches, 1, "expected DB-resolved country match for +88812")

	// +99912 — другая страна, no match.
	body2 := `{"phone":"+9991234567","sender_id":"X","traffic_type":"transactional"}`
	req2 := httptest.NewRequest("POST",
		"/portal/v1/reseller/network/route-sets/"+rsID.String()+"/preview",
		strings.NewReader(body2))
	req2 = mux.SetURLVars(req2, map[string]string{"id": rsID.String()})
	req2 = withReseller(req2, resellerID)
	w2 := httptest.NewRecorder()
	h.Preview(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, "body: %s", w2.Body.String())

	var resp2 struct {
		Matches []interface{} `json:"matches"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	require.Empty(t, resp2.Matches)
}
