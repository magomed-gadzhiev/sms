package application

import (
	"testing"

	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWhereClause_SimpleEq(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "eq", Value: "Москва"},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "attributes->>'city'")
	assert.Contains(t, args, "Москва")
}

func TestBuildWhereClause_NestedGroup(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "eq", Value: "Москва"},
			{
				Operator: "OR",
				Conditions: []domain.SegmentRuleNode{
					{Field: "attributes.age", Op: "gte", Value: float64(25)},
					{Field: "attributes.spending", Op: "gte", Value: float64(10000)},
				},
			},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "AND")
	assert.Contains(t, clause, "OR")
	assert.Len(t, args, 3)
}

func TestBuildWhereClause_InOperator(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.city", Op: "in", Value: []interface{}{"Москва", "Санкт-Петербург"}},
		},
	}
	clause, args, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "IN")
	assert.True(t, len(args) >= 2)
}

func TestBuildWhereClause_IsEmpty(t *testing.T) {
	rules := domain.SegmentRules{
		Operator: "AND",
		Conditions: []domain.SegmentRuleNode{
			{Field: "attributes.email", Op: "is_empty"},
		},
	}
	clause, _, err := BuildWhereClause(rules, 1)
	require.NoError(t, err)
	assert.Contains(t, clause, "IS NULL")
}
