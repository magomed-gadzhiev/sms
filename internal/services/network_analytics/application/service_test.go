package application

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

// TestBuildStatKPIs_NoMoneyDataRendersAsNil verifies that money KPIs (Выручка,
// Себестоимость, Прибыль) get nil Value when revenue and cost are both zero,
// while non-money KPIs (Всего, Доставляемость, Ошибки) still render with a
// concrete *float64 and the right format hint.
func TestBuildStatKPIs_NoMoneyDataRendersAsNil(t *testing.T) {
	rows := []domain.StatRow{
		{Total: 100, Delivered: 90, Failed: 5, Pending: 0, Timeout: 0, Revenue: 0, Cost: 0},
	}

	kpis := buildStatKPIs(rows)

	moneyNames := []string{"Выручка", "Себестоимость", "Прибыль"}
	for _, name := range moneyNames {
		k := findKPI(kpis, name)
		if k == nil {
			t.Fatalf("KPI %q missing", name)
		}
		if k.Value != nil {
			t.Errorf("KPI %q: expected Value == nil when no money data, got %v", name, *k.Value)
		}
		if k.Format != "currency" {
			t.Errorf("KPI %q: expected Format=\"currency\", got %q", name, k.Format)
		}
		if k.Currency != "RUB" {
			t.Errorf("KPI %q: expected Currency=\"RUB\", got %q", name, k.Currency)
		}
	}

	dlr := findKPI(kpis, "Доставляемость")
	if dlr == nil || dlr.Value == nil {
		t.Fatalf("Доставляемость: expected non-nil Value")
	}
	if dlr.Format != "percent" {
		t.Errorf("Доставляемость: expected Format=\"percent\", got %q", dlr.Format)
	}

	errs := findKPI(kpis, "Ошибки")
	if errs == nil || errs.Value == nil {
		t.Fatalf("Ошибки: expected non-nil Value")
	}
	if errs.Format != "percent" {
		t.Errorf("Ошибки: expected Format=\"percent\", got %q", errs.Format)
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

// TestBuildStatKPIs_WithMoneyDataRendersValues verifies money KPIs render real
// values when revenue/cost are present, including derived profit = revenue - cost.
func TestBuildStatKPIs_WithMoneyDataRendersValues(t *testing.T) {
	rows := []domain.StatRow{
		{Total: 100, Delivered: 90, Failed: 5, Revenue: 1500.0, Cost: 600.0},
	}

	kpis := buildStatKPIs(rows)

	revenue := findKPI(kpis, "Выручка")
	if revenue == nil || revenue.Value == nil {
		t.Fatalf("Выручка: expected non-nil Value")
	}
	if *revenue.Value != 1500.0 {
		t.Errorf("Выручка: expected 1500.0, got %v", *revenue.Value)
	}

	profit := findKPI(kpis, "Прибыль")
	if profit == nil || profit.Value == nil {
		t.Fatalf("Прибыль: expected non-nil Value")
	}
	if *profit.Value != 900.0 {
		t.Errorf("Прибыль: expected 900.0, got %v", *profit.Value)
	}

	cost := findKPI(kpis, "Себестоимость")
	if cost == nil || cost.Value == nil {
		t.Fatalf("Себестоимость: expected non-nil Value")
	}
	if *cost.Value != 600.0 {
		t.Errorf("Себестоимость: expected 600.0, got %v", *cost.Value)
	}
}
