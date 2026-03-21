package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/stretchr/testify/mock"
)

type MockBillingStrategy struct {
	mock.Mock
}

func (m *MockBillingStrategy) Calculate(ctx context.Context, params application.CalculationParams) (*application.CalculationResult, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*application.CalculationResult), args.Error(1)
}
