package network

import (
	"testing"

	"github.com/google/uuid"

	"github.com/smpp-server/smpp-server/internal/storage"
)

func TestRouteSignature_StableForReorderedConditions(t *testing.T) {
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{
		ProviderID: provID,
		RouteType:  "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{
				{Type: "country", Value: "RU"},
				{Type: "operator", Value: "megafon"},
			}},
		},
	}
	b := storage.RouteSetItemFull{
		ProviderID: provID,
		RouteType:  "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{
				{Type: "operator", Value: "megafon"},
				{Type: "country", Value: "RU"},
			}},
		},
	}
	if RouteSignature(a) != RouteSignature(b) {
		t.Fatalf("signature must be order-independent within group: %s vs %s",
			RouteSignature(a), RouteSignature(b))
	}
}

func TestRouteSignature_DifferentProvidersDiffer(t *testing.T) {
	a := storage.RouteSetItemFull{
		ProviderID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		RouteType:  "sms",
	}
	b := storage.RouteSetItemFull{
		ProviderID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		RouteType:  "sms",
	}
	if RouteSignature(a) == RouteSignature(b) {
		t.Fatal("different provider_id must produce different signature")
	}
}

func TestRouteSignature_GroupOrderMatters(t *testing.T) {
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{
		ProviderID: provID, RouteType: "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}},
			{LogicOp: "OR", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "BY"}}},
		},
	}
	b := storage.RouteSetItemFull{
		ProviderID: provID, RouteType: "sms",
		ConditionGroups: []storage.RouteSetConditionGroup{
			{LogicOp: "OR", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "BY"}}},
			{LogicOp: "AND", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}},
		},
	}
	if RouteSignature(a) == RouteSignature(b) {
		t.Fatal("different group order must produce different signature")
	}
}

func TestRouteSignature_EmptyConditionsStable(t *testing.T) {
	provID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := storage.RouteSetItemFull{ProviderID: provID, RouteType: "sms"}
	b := storage.RouteSetItemFull{ProviderID: provID, RouteType: "sms"}
	if RouteSignature(a) != RouteSignature(b) {
		t.Fatal("two empty-condition routes must share signature")
	}
}
