package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SegmentRuleNode.IsGroup ---

func TestSegmentRuleNode_IsGroup_WithOperatorAndConditions(t *testing.T) {
	node := SegmentRuleNode{
		Operator: "AND",
		Conditions: []SegmentRuleNode{
			{Field: "phone", Op: "eq", Value: "123"},
		},
	}
	assert.True(t, node.IsGroup())
}

func TestSegmentRuleNode_IsGroup_LeafNode(t *testing.T) {
	node := SegmentRuleNode{Field: "phone", Op: "eq", Value: "123"}
	assert.False(t, node.IsGroup())
}

func TestSegmentRuleNode_IsGroup_OperatorWithoutConditions(t *testing.T) {
	node := SegmentRuleNode{Operator: "AND"}
	assert.False(t, node.IsGroup())
}

func TestSegmentRuleNode_IsGroup_ConditionsWithoutOperator(t *testing.T) {
	node := SegmentRuleNode{
		Conditions: []SegmentRuleNode{
			{Field: "phone", Op: "eq", Value: "123"},
		},
	}
	assert.False(t, node.IsGroup())
}

// --- MaxSegmentDepth constant ---

func TestMaxSegmentDepth(t *testing.T) {
	assert.Equal(t, 3, MaxSegmentDepth)
}

// --- ErrSegmentNotFound ---

func TestErrSegmentNotFound_NonNil(t *testing.T) {
	assert.NotNil(t, ErrSegmentNotFound)
}

// --- SegmentToSQL: nil rule and no tags → empty ---

func TestSegmentToSQL_NilRuleNoTags(t *testing.T) {
	sql, params, err := SegmentToSQL(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, params)
}

// --- SegmentToSQL: tags only ---

func TestSegmentToSQL_TagsOnly(t *testing.T) {
	sql, params, err := SegmentToSQL(nil, []string{"vip", "active"})
	require.NoError(t, err)
	assert.Equal(t, "tags && $2", sql)
	require.Len(t, params, 1)
	assert.Equal(t, []string{"vip", "active"}, params[0])
}

// --- SegmentToSQL: single eq condition on phone ---

func TestSegmentToSQL_EqOnPhone(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "phone", Op: "eq", Value: "+79001234567"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "phone = $2", sql)
	assert.Equal(t, []interface{}{"+79001234567"}, params)
}

// --- SegmentToSQL: neq ---

func TestSegmentToSQL_Neq(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "neq", Value: "Moscow"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'city' != $2", sql)
	assert.Equal(t, []interface{}{"Moscow"}, params)
}

// --- SegmentToSQL: contains ---

func TestSegmentToSQL_Contains(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "name", Op: "contains", Value: "Ivan"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'name' ILIKE $2", sql)
	assert.Equal(t, []interface{}{"%Ivan%"}, params)
}

// --- SegmentToSQL: starts_with ---

func TestSegmentToSQL_StartsWith(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "name", Op: "starts_with", Value: "Alex"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'name' ILIKE $2", sql)
	assert.Equal(t, []interface{}{"Alex%"}, params)
}

// --- SegmentToSQL: ends_with ---

func TestSegmentToSQL_EndsWith(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "name", Op: "ends_with", Value: "ov"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'name' ILIKE $2", sql)
	assert.Equal(t, []interface{}{"%ov"}, params)
}

// --- SegmentToSQL: gt / gte / lt / lte ---

func TestSegmentToSQL_ComparisonOps(t *testing.T) {
	tests := []struct {
		op      string
		sqlOp   string
	}{
		{"gt", ">"},
		{"gte", ">="},
		{"lt", "<"},
		{"lte", "<="},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.op, func(t *testing.T) {
			rule := &SegmentRule{
				Operator: "AND",
				Conditions: []SegmentCondition{
					{Field: "age", Op: tc.op, Value: "30"},
				},
			}
			sql, params, err := SegmentToSQL(rule, nil)
			require.NoError(t, err)
			assert.Equal(t, "attributes->>'age' "+tc.sqlOp+" $2", sql)
			assert.Equal(t, []interface{}{"30"}, params)
		})
	}
}

// --- SegmentToSQL: in ---

func TestSegmentToSQL_In(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "in", Value: "Moscow, SPb, Kazan"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'city' IN ($2, $3, $4)", sql)
	assert.Equal(t, []interface{}{"Moscow", "SPb", "Kazan"}, params)
}

// --- SegmentToSQL: not_in ---

func TestSegmentToSQL_NotIn(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "not_in", Value: "Moscow,SPb"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'city' NOT IN ($2, $3)", sql)
	assert.Equal(t, []interface{}{"Moscow", "SPb"}, params)
}

// --- SegmentToSQL: between ---

func TestSegmentToSQL_Between(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "age", Op: "between", Value: "18", Value2: "65"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'age' BETWEEN $2 AND $3", sql)
	assert.Equal(t, []interface{}{"18", "65"}, params)
}

// --- SegmentToSQL: is_empty / is_not_empty ---

func TestSegmentToSQL_IsEmpty(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "email", Op: "is_empty"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "(attributes->>'email' IS NULL OR attributes->>'email' = '')", sql)
	assert.Empty(t, params)
}

func TestSegmentToSQL_IsNotEmpty(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "email", Op: "is_not_empty"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "(attributes->>'email' IS NOT NULL AND attributes->>'email' != '')", sql)
	assert.Empty(t, params)
}

// --- SegmentToSQL: OR operator joins with OR ---

func TestSegmentToSQL_OrOperator(t *testing.T) {
	rule := &SegmentRule{
		Operator: "OR",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "eq", Value: "Moscow"},
			{Field: "city", Op: "eq", Value: "SPb"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "attributes->>'city' = $2 OR attributes->>'city' = $3", sql)
	assert.Equal(t, []interface{}{"Moscow", "SPb"}, params)
}

// --- SegmentToSQL: AND operator joins with AND ---

func TestSegmentToSQL_AndOperator(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "phone", Op: "eq", Value: "123"},
			{Field: "city", Op: "eq", Value: "Moscow"},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Equal(t, "phone = $2 AND attributes->>'city' = $3", sql)
	assert.Equal(t, []interface{}{"123", "Moscow"}, params)
}

// --- SegmentToSQL: nested group ---

func TestSegmentToSQL_NestedGroup(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "eq", Value: "Moscow"},
		},
		Nested: []SegmentRule{
			{
				Operator: "OR",
				Conditions: []SegmentCondition{
					{Field: "age", Op: "gte", Value: "18"},
					{Field: "age", Op: "lte", Value: "65"},
				},
			},
		},
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	// Expect: city = $2 AND (age >= $3 OR age <= $4)
	assert.Contains(t, sql, "attributes->>'city' = $2")
	assert.Contains(t, sql, "AND")
	assert.Contains(t, sql, "(")
	assert.Len(t, params, 3)
}

// --- SegmentToSQL: tags combined with rule ---

func TestSegmentToSQL_RuleAndTags(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "eq", Value: "Moscow"},
		},
	}
	sql, params, err := SegmentToSQL(rule, []string{"vip"})
	require.NoError(t, err)
	// Should have both rule SQL and tags SQL joined by AND
	assert.True(t, strings.Contains(sql, "AND"), "expected AND between rule and tags")
	assert.Contains(t, sql, "tags &&")
	assert.Len(t, params, 2) // "Moscow" + []string{"vip"}
}

// --- SegmentToSQL: depth exceeded ---

func TestSegmentToSQL_DepthExceeded(t *testing.T) {
	// Build 4 levels deep (MaxSegmentDepth == 3)
	innerMost := SegmentRule{
		Operator:   "AND",
		Conditions: []SegmentCondition{{Field: "city", Op: "eq", Value: "X"}},
	}
	level3 := SegmentRule{Operator: "AND", Nested: []SegmentRule{innerMost}}
	level2 := SegmentRule{Operator: "AND", Nested: []SegmentRule{level3}}
	level1 := SegmentRule{Operator: "AND", Nested: []SegmentRule{level2}}

	_, _, err := SegmentToSQL(&level1, nil)
	assert.ErrorIs(t, err, ErrSegmentDepthExceeded)
}

// --- SegmentToSQL: unsupported operator ---

func TestSegmentToSQL_UnsupportedOperator(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "like", Value: "Mos"},
		},
	}
	_, _, err := SegmentToSQL(rule, nil)
	assert.ErrorIs(t, err, ErrInvalidSegmentRules)
}

// --- SegmentToSQL: invalid rule operator ---

func TestSegmentToSQL_InvalidRuleOperator(t *testing.T) {
	rule := &SegmentRule{
		Operator: "XOR",
		Conditions: []SegmentCondition{
			{Field: "city", Op: "eq", Value: "Moscow"},
		},
	}
	_, _, err := SegmentToSQL(rule, nil)
	assert.ErrorIs(t, err, ErrInvalidSegmentRules)
}

// --- SegmentToSQL: empty field ---

func TestSegmentToSQL_EmptyField(t *testing.T) {
	rule := &SegmentRule{
		Operator: "AND",
		Conditions: []SegmentCondition{
			{Field: "", Op: "eq", Value: "value"},
		},
	}
	_, _, err := SegmentToSQL(rule, nil)
	assert.ErrorIs(t, err, ErrInvalidSegmentRules)
}

// --- SegmentToSQL: empty rule with empty conditions → empty ---

func TestSegmentToSQL_EmptyRuleNoConditions(t *testing.T) {
	rule := &SegmentRule{
		Operator:   "AND",
		Conditions: nil,
	}
	sql, params, err := SegmentToSQL(rule, nil)
	require.NoError(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, params)
}
