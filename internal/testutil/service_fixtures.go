package testutil

import (
	"time"

	"github.com/google/uuid"

	analyticsdomain "github.com/smpp-server/smpp-server/internal/services/analytics/domain"
	authdomain "github.com/smpp-server/smpp-server/internal/services/auth/domain"
	billingdomain "github.com/smpp-server/smpp-server/internal/services/billing/domain"
	routingdomain "github.com/smpp-server/smpp-server/internal/services/routing/domain"
	tarifdomain "github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// NewTestUser returns a valid User with predictable defaults.
func NewTestUser() *authdomain.User {
	now := time.Now()
	return &authdomain.User{
		ID:           uuid.New(),
		Username:     "testuser",
		Email:        "testuser@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012",
		RoleID:       uuid.New(),
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// NewTestAPIKey returns a valid APIKey with predictable defaults.
func NewTestAPIKey(userID uuid.UUID) *authdomain.APIKey {
	now := time.Now()
	return &authdomain.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      "test-api-key",
		KeyHash:   "testhash_abc123def456",
		KeyPrefix: "sk_test_",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
		Scopes:    []string{"messages:send", "messages:read"},
	}
}

// NewTestAccount returns a valid billing Account with predictable defaults.
func NewTestAccount(clientID uuid.UUID) *billingdomain.Account {
	now := time.Now()
	return &billingdomain.Account{
		ID:        uuid.New(),
		ClientID:  clientID,
		Balance:   "1000.00",
		Currency:  "RUB",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewTestTransaction returns a valid billing Transaction with predictable defaults.
func NewTestTransaction(clientID uuid.UUID) *billingdomain.Transaction {
	return &billingdomain.Transaction{
		ID:            uuid.New(),
		ClientID:      clientID,
		Type:          billingdomain.TransactionTypeCharge,
		Amount:        "1.50",
		Currency:      "RUB",
		BalanceBefore: "1000.00",
		BalanceAfter:  "998.50",
		Description:   "SMS charge",
		Metadata:      make(map[string]interface{}),
		CreatedAt:     time.Now(),
	}
}

// NewTestTariffPlan returns a valid TariffPlan with predictable defaults.
func NewTestTariffPlan(operatorID uuid.UUID) *tarifdomain.TariffPlan {
	now := time.Now()
	return &tarifdomain.TariffPlan{
		ID:             uuid.New(),
		OperatorID:     operatorID,
		SenderCategory: tarifdomain.CategoryShared,
		Strategy:       tarifdomain.StrategyFixed,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewTestTariffTier returns a valid TariffTier with predictable defaults.
func NewTestTariffTier(tariffPeriodID uuid.UUID) *tarifdomain.TariffTier {
	return &tarifdomain.TariffTier{
		ID:              uuid.New(),
		TariffPeriodID:  tariffPeriodID,
		FromCount:       0,
		PricePerSegment: "1.50",
	}
}

// NewTestTariffPeriod returns a valid TariffPeriod with predictable defaults.
// The period spans the current month.
func NewTestTariffPeriod(tariffPlanID uuid.UUID) *tarifdomain.TariffPeriod {
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)
	return &tarifdomain.TariffPeriod{
		ID:           uuid.New(),
		TariffPlanID: tariffPlanID,
		StartDate:    startDate,
		EndDate:      &endDate,
		CreatedAt:    now,
	}
}

// NewTestLookupResult returns a valid LookupResult with predictable defaults.
func NewTestLookupResult() *routingdomain.LookupResult {
	return &routingdomain.LookupResult{
		MSISDN:                 "79001234567",
		OperatorMCCMNC:         "25001",
		OperatorName:           "MTS",
		NumberStatus:           routingdomain.NumberStatusActive,
		CountryCode:            "RU",
		NumberType:             routingdomain.NumberTypeMobile,
		IsPorted:               false,
		OriginalOperatorMCCMNC: "25001",
		Cached:                 false,
		QueriedAt:              time.Now(),
	}
}

// NewTestHLRProvider returns a valid HLRProvider with predictable defaults.
func NewTestHLRProvider() *routingdomain.HLRProvider {
	now := time.Now()
	return &routingdomain.HLRProvider{
		ID:               uuid.New(),
		Name:             "test-hlr-provider",
		AdapterType:      "http",
		Config:           map[string]interface{}{"api_key": "test-key", "base_url": "https://hlr.example.com"},
		Priority:         1,
		SupportedRegions: []string{"RU", "KZ", "BY"},
		CostPerLookup:    0.005,
		Status:           routingdomain.HLRProviderStatusHealthy,
		SuccessRate:      99.5,
		Active:           true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// NewTestOperator returns a valid Operator with predictable defaults.
func NewTestOperator(countryID uuid.UUID) *routingdomain.Operator {
	now := time.Now()
	return &routingdomain.Operator{
		ID:                 uuid.New(),
		CountryID:          countryID,
		Name:               "MTS",
		Code:               "mts-ru",
		SupportsPaidSender: true,
		SupportsFreeSender: true,
		Active:             true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// NewTestMetric returns a valid Metric with predictable defaults.
func NewTestMetric() *analyticsdomain.Metric {
	clientID := uuid.New()
	providerID := uuid.New()
	messageID := uuid.New()
	now := time.Now()
	return &analyticsdomain.Metric{
		ID:           uuid.New(),
		Type:         analyticsdomain.MetricTypeMessageSent,
		ClientID:     &clientID,
		ProviderID:   &providerID,
		MessageID:    &messageID,
		Status:       "delivered",
		Value:        1,
		SegmentCount: 1,
		Timestamp:    now,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    now,
	}
}

// NewTestSenderRegistration returns a valid SenderRegistration with predictable defaults.
func NewTestSenderRegistration(clientID, operatorID uuid.UUID) *tarifdomain.SenderRegistration {
	now := time.Now()
	return &tarifdomain.SenderRegistration{
		ID:         uuid.New(),
		ClientID:   clientID,
		OperatorID: operatorID,
		SenderName: "TestBrand",
		Type:       tarifdomain.SenderTypePaid,
		Status:     tarifdomain.SenderStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewTestUsageCounter returns a valid UsageCounter with predictable defaults.
func NewTestUsageCounter(clientID, tariffPlanID, tariffPeriodID uuid.UUID) *tarifdomain.UsageCounter {
	return &tarifdomain.UsageCounter{
		ID:             uuid.New(),
		ClientID:       clientID,
		TariffPlanID:   tariffPlanID,
		TariffPeriodID: tariffPeriodID,
		SegmentCount:   0,
		UpdatedAt:      time.Now(),
	}
}
