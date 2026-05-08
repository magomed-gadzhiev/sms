package repository

import (
	"testing"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// findKPI returns the KPI with the given name, or nil if absent.
func findKPI(kpis []domain.KPI, name string) *domain.KPI {
	for i := range kpis {
		if kpis[i].Name == name {
			return &kpis[i]
		}
	}
	return nil
}

// TestComputeKPIs_NoMoneyDataRendersAsNil mirrors the application-layer test
// for buildStatKPIs: when revenue and cost are both zero, Прибыль's Value is
// nil so the UI renders "—" rather than a misleading "0 ₽".
func TestComputeKPIs_NoMoneyDataRendersAsNil(t *testing.T) {
	kpis := computeKPIs(100, 80, 10, 0, 0)

	profit := findKPI(kpis, "Прибыль")
	if profit == nil {
		t.Fatalf("Прибыль: KPI missing")
	}
	if profit.Value != nil {
		t.Errorf("Прибыль: expected Value == nil when no money data, got %v", *profit.Value)
	}
	if profit.Format != "currency" {
		t.Errorf("Прибыль: expected Format=\"currency\", got %q", profit.Format)
	}
	if profit.Currency != "RUB" {
		t.Errorf("Прибыль: expected Currency=\"RUB\", got %q", profit.Currency)
	}

	dlr := findKPI(kpis, "Доставляемость")
	if dlr == nil || dlr.Value == nil {
		t.Fatalf("Доставляемость: expected non-nil Value")
	}
	if dlr.Format != "percent" {
		t.Errorf("Доставляемость: expected Format=\"percent\", got %q", dlr.Format)
	}

	total := findKPI(kpis, "Всего")
	if total == nil || total.Value == nil {
		t.Fatalf("Всего: expected non-nil Value")
	}
	if total.Format != "count" {
		t.Errorf("Всего: expected Format=\"count\", got %q", total.Format)
	}
	if *total.Value != 100 {
		t.Errorf("Всего: expected Value=100, got %v", *total.Value)
	}
}

// TestComputeKPIs_BreakevenRendersAsZero is the regression guard against the
// previous moneyValue closure that suppressed any zero — including a genuine
// breakeven (revenue == cost > 0, profit == 0) — and incorrectly rendered it
// as "—". Profit must point to 0.0 here, not nil.
func TestComputeKPIs_BreakevenRendersAsZero(t *testing.T) {
	kpis := computeKPIs(100, 80, 10, 1500.0, 1500.0)

	profit := findKPI(kpis, "Прибыль")
	if profit == nil {
		t.Fatalf("Прибыль: KPI missing")
	}
	if profit.Value == nil {
		t.Fatalf("Прибыль: expected non-nil Value at breakeven (revenue == cost > 0), got nil")
	}
	if *profit.Value != 0.0 {
		t.Errorf("Прибыль: expected 0.0 at breakeven, got %v", *profit.Value)
	}
	if profit.Format != "currency" {
		t.Errorf("Прибыль: expected Format=\"currency\", got %q", profit.Format)
	}
	if profit.Currency != "RUB" {
		t.Errorf("Прибыль: expected Currency=\"RUB\", got %q", profit.Currency)
	}
}
