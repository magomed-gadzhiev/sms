package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSegmentNotFound = errors.New("segment not found")
)

// SavedSegment represents a cross-list saved segment with filter rules.
type SavedSegment struct {
	ID             uuid.UUID    `json:"id"`
	ClientID       uuid.UUID    `json:"client_id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	ContactListIDs []uuid.UUID  `json:"contact_list_ids"`
	Rules          SegmentRules `json:"rules"`
	TagRules       *TagRules    `json:"tag_rules,omitempty"`
	EstimatedCount int32        `json:"estimated_count"`
	EstimatedAt    *time.Time   `json:"estimated_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// SegmentRules represents the top-level rule group with nested conditions.
type SegmentRules struct {
	Operator   string            `json:"operator"` // "AND" | "OR"
	Conditions []SegmentRuleNode `json:"conditions"`
}

// SegmentRuleNode is either a leaf condition or a nested group.
type SegmentRuleNode struct {
	// Leaf condition fields
	Field string      `json:"field,omitempty"`
	Op    string      `json:"op,omitempty"`
	Value interface{} `json:"value,omitempty"`

	// Nested group fields
	Operator   string            `json:"operator,omitempty"`
	Conditions []SegmentRuleNode `json:"conditions,omitempty"`
}

// TagRules for tag-based filtering.
type TagRules struct {
	Op   string   `json:"op"`   // "contains_any" | "contains_all"
	Tags []string `json:"tags"`
}

// IsGroup returns true if this node is a nested group (has Operator and Conditions).
func (n SegmentRuleNode) IsGroup() bool {
	return n.Operator != "" && len(n.Conditions) > 0
}

// MaxSegmentDepth is the maximum nesting depth for segment rules
const MaxSegmentDepth = 3

// SegmentRule represents an AND/OR tree of conditions
type SegmentRule struct {
	Operator   string             // "AND" or "OR"
	Conditions []SegmentCondition // leaf conditions
	Nested     []SegmentRule      // nested sub-rules
}

// SegmentCondition represents a single filter condition
type SegmentCondition struct {
	Field  string // attribute name, "phone", or "tags"
	Op     string // operator
	Value  string // primary value
	Value2 string // secondary value (for "between")
}

// validOperators defines all supported filter operators
var validOperators = map[string]bool{
	"eq": true, "neq": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
	"contains": true, "starts_with": true, "ends_with": true,
	"in": true, "not_in": true,
	"between": true,
	"is_empty": true, "is_not_empty": true,
}

// SegmentToSQL translates segment rules into a parameterized SQL WHERE clause.
// Parameters start at $2 (since $1 = contact_list_id).
// Returns the WHERE clause fragment and the parameter values.
func SegmentToSQL(rule *SegmentRule, tags []string) (string, []interface{}, error) {
	if rule == nil && len(tags) == 0 {
		return "", nil, nil
	}

	var params []interface{}
	paramIdx := 2 // $1 is reserved for contact_list_id

	var parts []string

	// Process rule tree
	if rule != nil {
		ruleSQL, ruleParams, newIdx, err := ruleToSQL(rule, paramIdx, 1)
		if err != nil {
			return "", nil, err
		}
		if ruleSQL != "" {
			parts = append(parts, ruleSQL)
		}
		params = append(params, ruleParams...)
		paramIdx = newIdx
	}

	// Process tags filter (overlap operator)
	if len(tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags && $%d", paramIdx))
		params = append(params, tags)
		// paramIdx++  // not needed after last param
	}

	if len(parts) == 0 {
		return "", nil, nil
	}

	where := strings.Join(parts, " AND ")
	return where, params, nil
}

// ruleToSQL recursively converts a SegmentRule into SQL.
func ruleToSQL(rule *SegmentRule, paramIdx int, depth int) (string, []interface{}, int, error) {
	if depth > MaxSegmentDepth {
		return "", nil, paramIdx, ErrSegmentDepthExceeded
	}

	op := strings.ToUpper(rule.Operator)
	if op != "AND" && op != "OR" {
		return "", nil, paramIdx, fmt.Errorf("%w: operator must be AND or OR, got %q", ErrInvalidSegmentRules, rule.Operator)
	}

	var parts []string
	var params []interface{}

	// Process leaf conditions
	for _, cond := range rule.Conditions {
		condSQL, condParams, newIdx, err := conditionToSQL(&cond, paramIdx)
		if err != nil {
			return "", nil, paramIdx, err
		}
		parts = append(parts, condSQL)
		params = append(params, condParams...)
		paramIdx = newIdx
	}

	// Process nested sub-rules
	for _, nested := range rule.Nested {
		nestedSQL, nestedParams, newIdx, err := ruleToSQL(&nested, paramIdx, depth+1)
		if err != nil {
			return "", nil, paramIdx, err
		}
		if nestedSQL != "" {
			parts = append(parts, "("+nestedSQL+")")
		}
		params = append(params, nestedParams...)
		paramIdx = newIdx
	}

	if len(parts) == 0 {
		return "", nil, paramIdx, nil
	}

	joiner := " " + op + " "
	result := strings.Join(parts, joiner)
	return result, params, paramIdx, nil
}

// conditionToSQL converts a single SegmentCondition to SQL.
func conditionToSQL(cond *SegmentCondition, paramIdx int) (string, []interface{}, int, error) {
	if !validOperators[cond.Op] {
		return "", nil, paramIdx, fmt.Errorf("%w: unsupported operator %q", ErrInvalidSegmentRules, cond.Op)
	}

	if cond.Field == "" {
		return "", nil, paramIdx, fmt.Errorf("%w: field is required", ErrInvalidSegmentRules)
	}

	// Determine the SQL field expression
	var fieldExpr string
	if cond.Field == "phone" {
		fieldExpr = "phone"
	} else if cond.Field == "tags" {
		fieldExpr = "tags"
	} else {
		// JSONB attribute text extraction
		fieldExpr = fmt.Sprintf("attributes->>'%s'", sanitizeFieldName(cond.Field))
	}

	var sqlFrag string
	var params []interface{}

	switch cond.Op {
	case "eq":
		sqlFrag = fmt.Sprintf("%s = $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "neq":
		sqlFrag = fmt.Sprintf("%s != $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "gt":
		sqlFrag = fmt.Sprintf("%s > $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "gte":
		sqlFrag = fmt.Sprintf("%s >= $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "lt":
		sqlFrag = fmt.Sprintf("%s < $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "lte":
		sqlFrag = fmt.Sprintf("%s <= $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value)
		paramIdx++
	case "contains":
		sqlFrag = fmt.Sprintf("%s ILIKE $%d", fieldExpr, paramIdx)
		params = append(params, "%"+cond.Value+"%")
		paramIdx++
	case "starts_with":
		sqlFrag = fmt.Sprintf("%s ILIKE $%d", fieldExpr, paramIdx)
		params = append(params, cond.Value+"%")
		paramIdx++
	case "ends_with":
		sqlFrag = fmt.Sprintf("%s ILIKE $%d", fieldExpr, paramIdx)
		params = append(params, "%"+cond.Value)
		paramIdx++
	case "in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			params = append(params, strings.TrimSpace(v))
			paramIdx++
		}
		sqlFrag = fmt.Sprintf("%s IN (%s)", fieldExpr, strings.Join(placeholders, ", "))
	case "not_in":
		values := strings.Split(cond.Value, ",")
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = fmt.Sprintf("$%d", paramIdx)
			params = append(params, strings.TrimSpace(v))
			paramIdx++
		}
		sqlFrag = fmt.Sprintf("%s NOT IN (%s)", fieldExpr, strings.Join(placeholders, ", "))
	case "between":
		sqlFrag = fmt.Sprintf("%s BETWEEN $%d AND $%d", fieldExpr, paramIdx, paramIdx+1)
		params = append(params, cond.Value, cond.Value2)
		paramIdx += 2
	case "is_empty":
		sqlFrag = fmt.Sprintf("(%s IS NULL OR %s = '')", fieldExpr, fieldExpr)
	case "is_not_empty":
		sqlFrag = fmt.Sprintf("(%s IS NOT NULL AND %s != '')", fieldExpr, fieldExpr)
	}

	return sqlFrag, params, paramIdx, nil
}

// sanitizeFieldName removes any characters that could cause SQL injection in field names
func sanitizeFieldName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
