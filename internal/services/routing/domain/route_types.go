package domain

import "time"

type TrafficType string

const (
	TrafficTypeAuthorization TrafficType = "authorization"
	TrafficTypeTransactional TrafficType = "transactional"
	TrafficTypeService       TrafficType = "service"
)

func ValidTrafficType(s string) bool {
	switch TrafficType(s) {
	case TrafficTypeAuthorization, TrafficTypeTransactional, TrafficTypeService:
		return true
	}
	return false
}

type RouteStatus string

const (
	RouteStatusActive RouteStatus = "active"
	RouteStatusDraft  RouteStatus = "draft"
)

func ValidRouteStatus(s string) bool {
	switch RouteStatus(s) {
	case RouteStatusActive, RouteStatusDraft:
		return true
	}
	return false
}

type ConditionType string

const (
	ConditionOperator    ConditionType = "operator"
	ConditionCountry     ConditionType = "country"
	ConditionTrafficType ConditionType = "traffic_type"
	ConditionPaidName    ConditionType = "paid_name"
	ConditionRegex       ConditionType = "regex"
)

func ValidConditionType(s string) bool {
	switch ConditionType(s) {
	case ConditionOperator, ConditionCountry, ConditionTrafficType, ConditionPaidName, ConditionRegex:
		return true
	}
	return false
}

type LogicOp string

const (
	LogicIf     LogicOp = "IF"
	LogicAnd    LogicOp = "AND"
	LogicAndNot LogicOp = "AND_NOT"
	LogicOr     LogicOp = "OR"
	LogicOrNot  LogicOp = "OR_NOT"
)

func ValidLogicOp(s string) bool {
	switch LogicOp(s) {
	case LogicIf, LogicAnd, LogicAndNot, LogicOr, LogicOrNot:
		return true
	}
	return false
}

type ConditionGroup struct {
	ID         int64
	GroupIndex int
	LogicOp    LogicOp
	Conditions []Condition
}

type Condition struct {
	ID    int64
	Type  ConditionType
	Value string
}

type Schedule struct {
	ID       int64
	DateFrom *time.Time
	DateTo   *time.Time
	TimeFrom *string
	TimeTo   *string
	Weekdays int
	Timezone string
}
