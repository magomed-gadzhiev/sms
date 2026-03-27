package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Plan struct {
	ID                 uuid.UUID
	Name               string
	DisplayName        string
	MonthlyPriceRub    float64
	MaxSMSPerMonth     int
	MaxSMPPConnections int
	MaxUsers           int
	RateLimitPerSecond int
	RateLimitPerMinute int
	RateLimitPerHour   int
	RateLimitPerDay    int
	Features           PlanFeatures
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PlanFeatures struct {
	Analytics    bool `json:"analytics"`
	Webhooks     bool `json:"webhooks"`
	HLR          bool `json:"hlr"`
	SmartRouting bool `json:"smart_routing"`
	SubAccounts  bool `json:"sub_accounts"`
	WhiteLabel   bool `json:"white_label"`
}

func (p *Plan) HasFeature(feature string) bool {
	switch feature {
	case "analytics":
		return p.Features.Analytics
	case "webhooks":
		return p.Features.Webhooks
	case "hlr":
		return p.Features.HLR
	case "smart_routing":
		return p.Features.SmartRouting
	case "sub_accounts":
		return p.Features.SubAccounts
	case "white_label":
		return p.Features.WhiteLabel
	default:
		return false
	}
}

func (p *Plan) UnmarshalFeatures(data []byte) error {
	return json.Unmarshal(data, &p.Features)
}

func (p *Plan) MarshalFeatures() ([]byte, error) {
	return json.Marshal(p.Features)
}
