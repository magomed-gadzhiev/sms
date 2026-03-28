package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
	"github.com/stretchr/testify/mock"
)

// MockAuditLogRepository implements domain.AuditLogRepository.
type MockAuditLogRepository struct {
	mock.Mock
}

func (m *MockAuditLogRepository) QueryAuditLog(ctx context.Context, filters *domain.AuditLogFilters) ([]*domain.AuditLogEntry, int, error) {
	args := m.Called(ctx, filters)
	var entries []*domain.AuditLogEntry
	if v := args.Get(0); v != nil {
		entries = v.([]*domain.AuditLogEntry)
	}
	return entries, args.Int(1), args.Error(2)
}
