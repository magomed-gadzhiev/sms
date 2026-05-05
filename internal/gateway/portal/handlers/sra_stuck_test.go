package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestSRAStuck_List_Empty — пустая SRA таблица: handler возвращает {"rows": []}, 200.
func TestSRAStuck_List_Empty(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/admin/network/sra-stuck", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp struct {
		Rows []sraStuckRow `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Rows, "rows должен быть []  а не null")
	require.Empty(t, resp.Rows)
}

// TestSRAStuck_List_OnlyStuck — 3 строки с разными retry_count:
// A=50 (active), B=99 (active), C=120 (stuck). List должен вернуть только C.
// Проверяем также порядок ORDER BY last_materialize_error_at ASC.
func TestSRAStuck_List_OnlyStuck(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	subA := storagetest.SeedSubAccount(t, pool, resellerID)
	subB := storagetest.SeedSubAccount(t, pool, resellerID)
	subC := storagetest.SeedSubAccount(t, pool, resellerID)

	storagetest.SeedSRAErrorStateWithCount(t, pool, subA, nil, nil, "err-A", 50)
	storagetest.SeedSRAErrorStateWithCount(t, pool, subB, nil, nil, "err-B", 99)
	storagetest.SeedSRAErrorStateWithCount(t, pool, subC, nil, nil, "err-C", 120)

	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/admin/network/sra-stuck", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp struct {
		Rows []sraStuckRow `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Rows, 1, "только C (retry=120) должна быть в ответе")

	row := resp.Rows[0]
	require.Equal(t, subC.String(), row.ClientID)
	require.Equal(t, 120, row.MaterializeRetryCount)
	require.NotEmpty(t, row.ClientEmail)
	require.NotNil(t, row.LastMaterializeErrorAt)
}

// TestSRAStuck_List_OrderByErrorAt — 2 stuck-строки; проверяем ORDER BY last_materialize_error_at ASC.
// Старейшая ошибка должна идти первой. Тест фильтрует глобальный результат по двум известным
// client_id (глобальная БД может содержать stuck-строки от других тестов).
func TestSRAStuck_List_OrderByErrorAt(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	subFirst := storagetest.SeedSubAccount(t, pool, resellerID)
	subSecond := storagetest.SeedSubAccount(t, pool, resellerID)

	// subFirst получает error_at в прошлом, subSecond — свежую.
	// UPDATE напрямую — SeedSRAErrorStateWithCount использует now(), потом
	// корректируем subFirst назад.
	storagetest.SeedSRAErrorStateWithCount(t, pool, subFirst, nil, nil, "err-first", 100)
	storagetest.SeedSRAErrorStateWithCount(t, pool, subSecond, nil, nil, "err-second", 100)

	_, err := pool.Exec(t.Context(),
		`UPDATE subaccount_routing_assignment
		    SET last_materialize_error_at = NOW() - INTERVAL '2 hours'
		  WHERE client_id = $1`,
		subFirst,
	)
	require.NoError(t, err)

	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodGet, "/portal/v1/admin/network/sra-stuck", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var resp struct {
		Rows []sraStuckRow `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Отфильтровываем только строки данного теста из глобального результата.
	var filtered []sraStuckRow
	known := map[string]bool{subFirst.String(): true, subSecond.String(): true}
	for _, row := range resp.Rows {
		if known[row.ClientID] {
			filtered = append(filtered, row)
		}
	}
	require.Len(t, filtered, 2, "оба seeded row должны быть в ответе")
	require.Equal(t, subFirst.String(), filtered[0].ClientID, "subFirst (старый) должен быть первым")
	require.Equal(t, subSecond.String(), filtered[1].ClientID)
}

// TestSRAStuck_Reset_Success — stuck-строка (retry=120): POST reset → 200,
// в БД materialize_retry_count=0, error_at=NULL, error_text=NULL.
//
// ВАЖНО: SRA row засевается с ненулевым provider_set_id, чтобы триггер
// trg_sra_orphan_cleanup (миграция 000140) не удалил row при UPDATE:
// триггер удаляет row только когда ОБА set_id NULL после UPDATE.
func TestSRAStuck_Reset_Success(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	setID := storagetest.SeedProviderSet(t, pool, resellerID, "stuck-reset-set")
	storagetest.SeedSRAErrorStateWithCount(t, pool, subID, &setID, nil, "stuck-error", 120)

	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/admin/network/sra-stuck/"+subID.String()+"/reset", nil)
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	w := httptest.NewRecorder()
	h.Reset(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var result struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.True(t, result.OK)

	// Проверяем состояние в БД.
	var retryCount int
	var errorAt *time.Time
	var errorText *string
	err := pool.QueryRow(t.Context(),
		`SELECT materialize_retry_count, last_materialize_error_at, last_materialize_error_text
		   FROM subaccount_routing_assignment WHERE client_id = $1`,
		subID,
	).Scan(&retryCount, &errorAt, &errorText)
	require.NoError(t, err)
	require.Equal(t, 0, retryCount, "retry_count должен быть сброшен в 0")
	require.Nil(t, errorAt, "last_materialize_error_at должен быть NULL")
	require.Nil(t, errorText, "last_materialize_error_text должен быть NULL")
}

// TestSRAStuck_Reset_NotFound — случайный UUID: POST reset → 404.
func TestSRAStuck_Reset_NotFound(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	randomID := uuid.New()
	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/admin/network/sra-stuck/"+randomID.String()+"/reset", nil)
	req = mux.SetURLVars(req, map[string]string{"client_id": randomID.String()})
	w := httptest.NewRecorder()
	h.Reset(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

// TestSRAStuck_Reset_NotStuck — строка существует, но retry_count=50 (не stuck).
// POST reset → 409 Conflict. Сброс активного retry-state был бы ошибкой оператора
// и уничтожил бы диагностику.
func TestSRAStuck_Reset_NotStuck(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	storagetest.SeedSRAErrorStateWithCount(t, pool, subID, nil, nil, "active-retry", 50)

	h := NewSRAStuckHandlers(pool)
	req := httptest.NewRequest(http.MethodPost, "/portal/v1/admin/network/sra-stuck/"+subID.String()+"/reset", nil)
	req = mux.SetURLVars(req, map[string]string{"client_id": subID.String()})
	w := httptest.NewRecorder()
	h.Reset(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	errObj, ok := errResp["error"].(map[string]interface{})
	require.True(t, ok, "ответ должен содержать поле error")
	require.Contains(t, errObj["message"], "row is not in stuck state")
}
