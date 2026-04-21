package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	"github.com/stretchr/testify/mock"
)

// MockReachabilityChecker is a mock implementation of application.ReachabilityChecker
type MockReachabilityChecker struct {
	mock.Mock
}

func (m *MockReachabilityChecker) CheckReachability(ctx context.Context, msisdn string, channelType domain.ChannelType) (bool, error) {
	args := m.Called(ctx, msisdn, channelType)
	return args.Bool(0), args.Error(1)
}
