package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/smpp-server/smpp-server/api/proto/auditv1"
	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
	"github.com/smpp-server/smpp-server/internal/services/audit/mocks"
)

func newTestAuditServer(repo *mocks.MockAuditLogRepository) *Server {
	return NewServer(repo)
}

func TestAuditServer_QueryAuditLog_WithResults(t *testing.T) {
	repo := new(mocks.MockAuditLogRepository)
	srv := newTestAuditServer(repo)

	now := time.Now().UTC().Truncate(time.Second)
	entries := []*domain.AuditLogEntry{
		{
			ID:           "entry-1",
			TenantID:     "tenant-abc",
			UserID:       "user-1",
			Action:       "login",
			ResourceType: "session",
			ResourceID:   "sess-1",
			Details:      `{"ip":"1.2.3.4"}`,
			IPAddress:    "1.2.3.4",
			CreatedAt:    now,
		},
		{
			ID:           "entry-2",
			TenantID:     "tenant-abc",
			UserID:       "user-2",
			Action:       "logout",
			ResourceType: "session",
			ResourceID:   "sess-2",
			Details:      `{}`,
			IPAddress:    "5.6.7.8",
			CreatedAt:    now,
		},
	}

	expectedFilters := &domain.AuditLogFilters{
		TenantID: "tenant-abc",
		Page:     1,
		PerPage:  10,
	}
	repo.On("QueryAuditLog", mock.Anything, expectedFilters).Return(entries, 2, nil)

	resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
		TenantId: "tenant-abc",
		Page:     1,
		PerPage:  10,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(2), resp.Total)
	assert.Equal(t, int32(1), resp.Page)
	assert.Equal(t, int32(1), resp.TotalPages) // ceil(2/10) = 1
	require.Len(t, resp.Entries, 2)

	e0 := resp.Entries[0]
	assert.Equal(t, "entry-1", e0.Id)
	assert.Equal(t, "tenant-abc", e0.TenantId)
	assert.Equal(t, "user-1", e0.UserId)
	assert.Equal(t, "login", e0.Action)
	assert.Equal(t, "session", e0.ResourceType)
	assert.Equal(t, "sess-1", e0.ResourceId)
	assert.Equal(t, `{"ip":"1.2.3.4"}`, e0.Details)
	assert.Equal(t, "1.2.3.4", e0.IpAddress)
	assert.Equal(t, now.Unix(), e0.CreatedAt.AsTime().Unix())

	e1 := resp.Entries[1]
	assert.Equal(t, "entry-2", e1.Id)
	assert.Equal(t, "logout", e1.Action)

	repo.AssertExpectations(t)
}

func TestAuditServer_QueryAuditLog_WithFilters(t *testing.T) {
	repo := new(mocks.MockAuditLogRepository)
	srv := newTestAuditServer(repo)

	dateFrom := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	entries := []*domain.AuditLogEntry{
		{
			ID:        "entry-filtered",
			TenantID:  "tenant-xyz",
			UserID:    "user-42",
			Action:    "create_campaign",
			CreatedAt: dateFrom,
		},
	}

	repo.On("QueryAuditLog", mock.Anything, mock.MatchedBy(func(f *domain.AuditLogFilters) bool {
		return f.TenantID == "tenant-xyz" &&
			f.Action == "create_campaign" &&
			f.UserID == "user-42" &&
			f.DateFrom != nil && f.DateFrom.Equal(dateFrom) &&
			f.DateTo != nil && f.DateTo.Equal(dateTo)
	})).Return(entries, 1, nil)

	resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
		TenantId: "tenant-xyz",
		Action:   "create_campaign",
		UserId:   "user-42",
		DateFrom: timestamppb.New(dateFrom),
		DateTo:   timestamppb.New(dateTo),
		Page:     1,
		PerPage:  20,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.Total)
	require.Len(t, resp.Entries, 1)
	assert.Equal(t, "entry-filtered", resp.Entries[0].Id)
	assert.Equal(t, "user-42", resp.Entries[0].UserId)
	assert.Equal(t, "create_campaign", resp.Entries[0].Action)

	repo.AssertExpectations(t)
}

func TestAuditServer_QueryAuditLog_EmptyTenantID(t *testing.T) {
	repo := new(mocks.MockAuditLogRepository)
	srv := newTestAuditServer(repo)

	resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
		TenantId: "",
		Page:     1,
		PerPage:  20,
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "tenant_id is required")

	// repository should never be called
	repo.AssertNotCalled(t, "QueryAuditLog")
}

func TestAuditServer_QueryAuditLog_RepositoryError(t *testing.T) {
	repo := new(mocks.MockAuditLogRepository)
	srv := newTestAuditServer(repo)

	repo.On("QueryAuditLog", mock.Anything, mock.Anything).
		Return(nil, 0, errors.New("db connection lost"))

	resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
		TenantId: "tenant-abc",
		Page:     1,
		PerPage:  20,
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to query audit log")

	repo.AssertExpectations(t)
}

func TestAuditServer_QueryAuditLog_DefaultPagination(t *testing.T) {
	// When PerPage is 0, the server defaults to 20 for TotalPages calculation.
	repo := new(mocks.MockAuditLogRepository)
	srv := newTestAuditServer(repo)

	entries := make([]*domain.AuditLogEntry, 0)
	// 45 total entries, perPage defaults to 20 → totalPages = ceil(45/20) = 3
	repo.On("QueryAuditLog", mock.Anything, mock.MatchedBy(func(f *domain.AuditLogFilters) bool {
		return f.TenantID == "tenant-abc" && f.PerPage == 0
	})).Return(entries, 45, nil)

	resp, err := srv.QueryAuditLog(context.Background(), &auditv1.QueryAuditLogRequest{
		TenantId: "tenant-abc",
		// Page and PerPage are zero-value (0)
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(45), resp.Total)
	assert.Equal(t, int32(3), resp.TotalPages) // ceil(45/20) = 3

	repo.AssertExpectations(t)
}
