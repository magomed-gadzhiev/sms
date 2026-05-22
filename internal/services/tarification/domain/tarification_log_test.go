// internal/services/tarification/domain/tarification_log_test.go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewTarificationLog_LegacyFormPopulatesPlanAndPeriod(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()
	operatorID := uuid.New()
	planID := uuid.New()
	periodID := uuid.New()

	l := NewTarificationLog(
		clientID, messageID, operatorID, planID, periodID,
		CategoryShared, StrategyFixed,
		2, "1.5", "3.0",
		"idem-1",
	)

	require.NotNil(t, l.TariffPlanID)
	require.Equal(t, planID, *l.TariffPlanID)
	require.NotNil(t, l.TariffPeriodID)
	require.Equal(t, periodID, *l.TariffPeriodID)
	require.Nil(t, l.SourceRuleID)
	require.Equal(t, StrategyFixed, l.Strategy)
	require.Equal(t, "3.0", l.TotalAmount)
}

func TestNewUnifiedTarificationLog_LeavesPlanAndPeriodNil(t *testing.T) {
	clientID := uuid.New()
	messageID := uuid.New()
	operatorID := uuid.New()
	ruleID := uuid.New()

	l := NewUnifiedTarificationLog(
		clientID, messageID, operatorID, ruleID,
		CategoryShared,
		2, "1.5", "3.0",
		"idem-2",
	)

	require.Nil(t, l.TariffPlanID)
	require.Nil(t, l.TariffPeriodID)
	require.NotNil(t, l.SourceRuleID)
	require.Equal(t, ruleID, *l.SourceRuleID)
	require.Equal(t, StrategyUnified, l.Strategy)
	require.Equal(t, "3.0", l.TotalAmount)
	require.Equal(t, "idem-2", l.IdempotencyKey)
}
