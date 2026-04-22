//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
)

// Reuses test helpers from network_tariff_editor_test.go (same package,
// integration build tag).

type clientTariffsBody struct {
	Plan *struct {
		ID       string `json:"id"`
		Strategy string `json:"strategy"`
		Currency string `json:"currency"`
	} `json:"plan"`
	Period *struct {
		ID   string  `json:"id"`
		From string  `json:"from"`
		To   *string `json:"to"`
	} `json:"period"`
	Operators []struct {
		ID   string  `json:"id"`
		Name string  `json:"name"`
		Icon *string `json:"icon"`
	} `json:"operators"`
	Tiers []struct {
		ID           string `json:"id"`
		FromQuantity int    `json:"from_quantity"`
	} `json:"tiers"`
	Cells []struct {
		OperatorID string   `json:"operator_id"`
		TierID     string   `json:"tier_id"`
		Effective  *float64 `json:"effective"`
	} `json:"cells"`
}

func doClientTariffsGet(
	t *testing.T, h *ClientTariffsEffectiveHandler, clientID uuid.UUID, query string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/portal/v1/client/tariffs/effective?" + query
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ClientIDKey, clientID))
	w := httptest.NewRecorder()
	h.Get(w, req)
	return w
}

func TestClientTariffs_ReturnsEffective(t *testing.T) {
	pool := getEditorTestPool(t)
	ctx := context.Background()

	reseller := seedEditorReseller(t, pool)
	sub := seedEditorSubAccount(t, pool, reseller)
	ruID := seedEditorCountryRU(t, pool)
	mts, bln := seedEditorOperators(t, pool, ruID)

	// Template with tier0=3.20, tier1k=2.80.
	seed := seedEditorTemplatePlan(t, pool, reseller, ruID,
		fmt.Sprintf("tpl-cli-%s", uuid.NewString()[:6]), 3.20, 2.80)

	// Bind template to sub.
	_, err := pool.Exec(ctx, `
		INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		VALUES ($1, $2)`, sub, seed.TemplateID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1`, sub)
	})

	// Override plan + period aligned; override only tier0 @ 3.00 (on MTS only).
	_, ovrPeriod := seedEditorOverridePlan(t, pool, reseller, sub, ruID)

	// The override plan in seedEditorOverridePlan has operator_id NULL (wildcard).
	// Set a wildcard override tier at from_count=0 → affects all operators.
	_, err = pool.Exec(ctx, `
		INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
		VALUES ($1, 0, 3.00)`, ovrPeriod)
	require.NoError(t, err)

	h := NewClientTariffsEffectiveHandler(pool)
	w := doClientTariffsGet(t, h, sub,
		"channel=sms&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var b clientTariffsBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b),
		"unmarshal failed; raw=%s", w.Body.String())

	require.NotNil(t, b.Plan)
	assert.Equal(t, seed.PlanID.String(), b.Plan.ID)
	assert.Equal(t, "RUB", b.Plan.Currency)
	require.NotNil(t, b.Period)
	assert.Equal(t, seed.PeriodID.String(), b.Period.ID)
	require.Len(t, b.Tiers, 2)

	// Find tier0 and tier1k ids.
	var tier0ID, tier1kID string
	for _, t := range b.Tiers {
		switch t.FromQuantity {
		case 0:
			tier0ID = t.ID
		case 1000:
			tier1kID = t.ID
		}
	}
	require.NotEmpty(t, tier0ID)
	require.NotEmpty(t, tier1kID)

	// Build index and assert effective values for MTS + Beeline.
	type key struct{ op, tier string }
	eff := map[key]*float64{}
	for _, c := range b.Cells {
		eff[key{c.OperatorID, c.TierID}] = c.Effective
	}

	for _, opID := range []uuid.UUID{mts, bln} {
		t0 := eff[key{opID.String(), tier0ID}]
		t1 := eff[key{opID.String(), tier1kID}]
		require.NotNil(t, t0, "tier0 cell missing for op=%s", opID)
		require.NotNil(t, t1, "tier1k cell missing for op=%s", opID)
		// tier0 overridden via wildcard → 3.00
		assert.InDelta(t, 3.00, *t0, 0.0001, "op=%s tier0", opID)
		// tier1k falls through to template → 2.80
		assert.InDelta(t, 2.80, *t1, 0.0001, "op=%s tier1k", opID)
	}
}

func TestClientTariffs_Unauthorized(t *testing.T) {
	pool := getEditorTestPool(t)
	h := NewClientTariffsEffectiveHandler(pool)

	req := httptest.NewRequest(http.MethodGet,
		"/portal/v1/client/tariffs/effective?country=RU", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestClientTariffs_ResellerReturnsEmpty(t *testing.T) {
	// Reseller has no parent_client_id → not a sub-account → empty cells, 200.
	// This is consistent with "no plan found" handling in the editor.
	pool := getEditorTestPool(t)
	reseller := seedEditorReseller(t, pool)

	h := NewClientTariffsEffectiveHandler(pool)
	w := doClientTariffsGet(t, h, reseller,
		"channel=sms&country=RU&sender_category=paid_registered&traffic_type=any")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var b clientTariffsBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Nil(t, b.Plan)
	assert.Nil(t, b.Period)
	assert.Empty(t, b.Cells)
}
