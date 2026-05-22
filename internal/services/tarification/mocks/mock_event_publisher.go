package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishTarificationResult(ctx context.Context, log *domain.TarificationLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishRecalcEvent(ctx context.Context, clientID string, tariffPlanID, tariffPeriodID string, oldPrice, newPrice string, affectedSegments int, recalcAmount string, recalcType string) error {
	args := m.Called(ctx, clientID, tariffPlanID, tariffPeriodID, oldPrice, newPrice, affectedSegments, recalcAmount, recalcType)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishPrepaidEvent(ctx context.Context, clientID string, tariffPlanID, tariffPeriodID string, amount, currency string) error {
	args := m.Called(ctx, clientID, tariffPlanID, tariffPeriodID, amount, currency)
	return args.Error(0)
}

func (m *MockEventPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}
